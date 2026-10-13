import Foundation
import os

enum AnalyticsCommand: Sendable {
    case record(QueuedEvent)
    case consent(Bool)
    case optOut
    case optIn
    case flush((@Sendable () -> Void)?)
    case width(Int)
}

actor AnalyticsCore {
    typealias Sleep = @Sendable (Duration) async throws -> Void

    nonisolated let endpoint: URL
    nonisolated let processKey: String
    private let requireConsent: Bool
    private let session: URLSession
    private let autoFlush: Duration?
    private let now: @Sendable () -> Date
    private let sleep: Sleep
    private let logger: Logger?
    private var app: AppInfo
    private var queue: AnalyticsQueue
    private var consented: Bool
    private var optedOut: Bool
    private var generation = 0
    private var failures = 0
    private var sender: Task<Void, Never>?
    private var senderID = 0
    private var senderWaiting = false

    init(
        endpoint: URL, processKey: String, app: AppInfo, requireConsent: Bool, optedOut: Bool,
        queueFile: URL?, session: URLSession, autoFlush: Duration? = .seconds(5), debug: Bool = false,
        now: @escaping @Sendable () -> Date = Date.init,
        sleep: @escaping Sleep = { try await Task.sleep(for: $0) }
    ) {
        self.endpoint = endpoint
        self.processKey = processKey
        self.app = app
        self.requireConsent = requireConsent
        self.optedOut = optedOut
        self.consented = !requireConsent
        self.session = session
        self.autoFlush = autoFlush
        self.now = now
        self.sleep = sleep
        self.logger = debug ? Logger(subsystem: "dev.yuva.analytics", category: "YuvaAnalytics") : nil
        var queue = AnalyticsQueue(file: queueFile)
        if optedOut { queue.clear() }
        queue.prune(now: Int64(now().timeIntervalSince1970 * 1000))
        queue.save()
        self.queue = queue
    }

    var pending: [EventBatch] { queue.batches }

    func start() {
        if !queue.batches.isEmpty, !optedOut { schedule(after: .zero) }
    }

    func handle(_ command: AnalyticsCommand) {
        switch command {
        case .record(var event):
            guard !optedOut, consented else {
                let reason = optedOut ? "opted out" : "no consent"
                logger?.debug("dropped \(event.n, privacy: .public): \(reason, privacy: .public)")
                return
            }
            if requireConsent { event.c = ConsentBody(a: 1) }
            guard queue.append(event, key: processKey, app: app) else {
                logger?.error("dropped \(event.n, privacy: .public): larger than 64 KiB")
                return
            }
            queue.save()
            logger?.debug("queued \(event.n, privacy: .public) \(event.u, privacy: .public)")
            if let autoFlush { schedule(after: autoFlush) }
        case .consent(let granted):
            consented = granted
            if !granted { clear() }
        case .optOut:
            optedOut = true
            clear()
        case .optIn:
            optedOut = false
        case .flush(let done):
            if !optedOut { schedule(after: .zero) }
            if let done {
                Task {
                    await self.idle()
                    done()
                }
            }
        case .width(let width):
            app.width = width
            queue.updateApp(key: processKey) { if $0.width == nil { $0.width = width } }
        }
    }

    func idle() async {
        while let sender {
            await sender.value
            if self.sender == sender { break }
        }
    }

    private func clear() {
        generation += 1
        queue.clear()
        queue.save()
        sender?.cancel()
        sender = nil
    }

    private func schedule(after delay: Duration) {
        if sender != nil, !(senderWaiting && delay == .zero) { return }
        sender?.cancel()
        senderID += 1
        let id = senderID
        senderWaiting = delay > .zero
        let sleep = self.sleep
        sender = Task {
            if delay > .zero {
                do { try await sleep(delay) } catch { return }
            }
            await self.run(id)
        }
    }

    private func run(_ id: Int) async {
        guard id == senderID else { return }
        senderWaiting = false
        await drain()
        if id == senderID { sender = nil }
    }

    private func drain() async {
        while !optedOut, !Task.isCancelled {
            guard let delay = await sendNext() else { return }
            guard delay > .zero else { continue }
            logger?.debug("retrying in \(delay.components.seconds, privacy: .public) s")
            do { try await sleep(delay) } catch { return }
        }
    }

    func sendNext() async -> Duration? {
        queue.prune(now: Int64(now().timeIntervalSince1970 * 1000))
        guard let batch = queue.next() else {
            queue.save()
            return nil
        }
        let started = generation
        var request = URLRequest(url: endpoint)
        request.httpMethod = "POST"
        request.httpBody = batch.body
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        if app.language == nil, let language = Locale.preferredLanguages.first {
            request.setValue(language, forHTTPHeaderField: "Accept-Language")
        }
        let status: Int
        let retryAfter: String?
        do {
            let (_, response) = try await session.data(for: request)
            let http = response as? HTTPURLResponse
            status = http?.statusCode ?? 0
            retryAfter = http?.value(forHTTPHeaderField: "Retry-After")
        } catch {
            guard started == generation else { return .zero }
            logger?.debug("send failed: \(error.localizedDescription, privacy: .public)")
            return backoff()
        }
        guard started == generation else { return .zero }
        logger?.debug("sent \(batch.count, privacy: .public) events: \(status, privacy: .public)")
        switch status {
        case 429, 503:
            return Self.retryDelay(retryAfter)
        case 200..<300, 400..<500:
            failures = 0
            queue.remove(batch)
            queue.save()
            return .zero
        default:
            return backoff()
        }
    }

    private func backoff() -> Duration {
        failures += 1
        return .seconds(min(300, 5 << min(failures - 1, 6)))
    }

    static func retryDelay(_ header: String?) -> Duration {
        if let header, let seconds = Int(header.trimmingCharacters(in: .whitespaces)), seconds >= 0 {
            return .seconds(max(1, seconds))
        }
        return .seconds(60)
    }
}

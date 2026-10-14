import Foundation
import os

enum AnalyticsCommand: Sendable {
    case record(QueuedEvent)
    case consent(Bool, persistent: Bool)
    case identify(String)
    case optOut
    case optIn
    case flush((@Sendable () -> Void)?)
    case width(Int)
    case request(RequestRecord)
    case queuedRequests(@Sendable (Int) -> Void)
}

actor AnalyticsCore {
    typealias Sleep = @Sendable (Duration) async throws -> Void
    static let consentEvent = "yuva_consent"

    nonisolated let endpoint: URL
    nonisolated let processKey: String
    private let home: String
    private let ids: PersistentIDStore
    private var persistentID: PersistentID?
    private var posts: [Task<Void, Never>] = []
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
        endpoint: URL, processKey: String, app: AppInfo, home: String, requireConsent: Bool, optedOut: Bool,
        queueFile: URL?, ids: PersistentIDStore, session: URLSession, autoFlush: Duration? = .seconds(5), debug: Bool = false,
        now: @escaping @Sendable () -> Date = Date.init,
        sleep: @escaping Sleep = { try await Task.sleep(for: $0) }
    ) {
        self.endpoint = endpoint
        self.processKey = processKey
        self.app = app
        self.home = home
        self.ids = ids
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
        var stored = ids.load()
        if let value = stored, optedOut || value.expired(at: now()) {
            ids.delete()
            stored = nil
        }
        self.persistentID = stored
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
            let id = currentID()
            if requireConsent || id != nil { event.c = ConsentBody(a: 1, p: id) }
            guard queue.append(event, key: processKey, app: app) else {
                logger?.error("dropped \(event.n, privacy: .public): larger than 64 KiB")
                return
            }
            queue.save()
            logger?.debug("queued \(event.n, privacy: .public) \(event.u, privacy: .public)")
            if let autoFlush { schedule(after: autoFlush) }
        case .consent(let granted, let persistent):
            consented = granted
            if granted, persistent, !optedOut {
                if currentID() == nil {
                    let fresh = PersistentID.make(now: now())
                    ids.save(fresh)
                    persistentID = fresh
                }
            } else {
                withdraw()
            }
            if !granted { clear() }
        case .identify(let token):
            guard !optedOut, consented, !token.isEmpty, let id = currentID() else {
                logger?.debug("identify ignored: no persistent consent")
                return
            }
            post(QueuedEvent(n: Self.consentEvent, u: home, t: timestamp(), c: ConsentBody(a: 1, p: id, j: token)))
        case .optOut:
            optedOut = true
            withdraw()
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
        case .request, .queuedRequests:
            break
        }
    }

    func idle() async {
        while !posts.isEmpty {
            let pending = posts
            posts.removeAll()
            for post in pending { await post.value }
        }
        while let sender {
            await sender.value
            if self.sender == sender { break }
        }
    }

    private func timestamp() -> Int64 {
        Int64(now().timeIntervalSince1970 * 1000)
    }

    private func currentID() -> String? {
        guard let value = persistentID else { return nil }
        guard value.expired(at: now()) else { return value.id }
        let fresh = PersistentID.make(now: now())
        ids.save(fresh)
        persistentID = fresh
        return fresh.id
    }

    private func withdraw() {
        guard let value = persistentID else { return }
        post(QueuedEvent(n: Self.consentEvent, u: home, t: timestamp(), c: ConsentBody(a: 0, p: value.id)))
        ids.delete()
        persistentID = nil
        queue.stripPersistent()
        queue.save()
    }

    private func post(_ event: QueuedEvent) {
        guard let body = try? JSONEncoder().encode(EventBatch(k: processKey, a: app, e: [event])) else { return }
        var request = URLRequest(url: endpoint)
        request.httpMethod = "POST"
        request.httpBody = body
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        let session = self.session
        let logger = self.logger
        posts.append(
            Task {
                do {
                    let (_, response) = try await session.data(for: request)
                    let status = (response as? HTTPURLResponse)?.statusCode ?? 0
                    logger?.debug("sent \(Self.consentEvent, privacy: .public): \(status, privacy: .public)")
                } catch {
                    logger?.debug("dropped \(Self.consentEvent, privacy: .public): \(error.localizedDescription, privacy: .public)")
                }
            })
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

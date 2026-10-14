import Foundation
import os

struct RequestMatcher: Sendable {
    enum Target: Equatable, Sendable {
        case route(String)
        case path(String)
    }

    struct Pattern: Sendable {
        let text: String
        let segments: [String]
        let literals: Int

        init(_ text: String) {
            self.text = text.hasPrefix("/") ? text : "/" + text
            segments = text.split(separator: "/").map(String.init)
            literals = segments.filter { !Self.isParameter($0) }.count
        }

        static func isParameter(_ segment: String) -> Bool {
            segment.count > 2 && segment.hasPrefix("{") && segment.hasSuffix("}")
        }

        func matches(_ parts: [String]) -> Bool {
            guard parts.count == segments.count else { return false }
            return zip(segments, parts).allSatisfy { Self.isParameter($0) || $0 == $1 }
        }
    }

    let hosts: Set<String>
    let patterns: [Pattern]
    let route: (@Sendable (URLRequest) -> String?)?
    let sendPaths: Bool
    let apiVersion: String?
    let environment: String?

    init(_ monitoring: YuvaRequestMonitoring) {
        hosts = Set(monitoring.hosts.map { $0.lowercased() })
        patterns = monitoring.routes.map(Pattern.init)
        route = monitoring.route
        sendPaths = monitoring.sendUnmatchedPaths
        apiVersion = monitoring.apiVersion
        environment = monitoring.environment
    }

    func target(for request: URLRequest) -> Target? {
        guard let url = request.url, let host = url.host?.lowercased(), hosts.contains(host) else { return nil }
        if let custom = route?(request), !custom.isEmpty, custom.count <= 300 { return .route(custom) }
        let path = URLComponents(url: url, resolvingAgainstBaseURL: false)?.percentEncodedPath ?? ""
        if let pattern = match(path) { return .route(pattern) }
        guard sendPaths else { return nil }
        let value = path.isEmpty ? "/" : path
        return value.count <= 2000 ? .path(value) : nil
    }

    func match(_ path: String) -> String? {
        let parts = path.split(separator: "/").map(String.init)
        var best: Pattern?
        for pattern in patterns where pattern.matches(parts) {
            if let current = best, current.literals >= pattern.literals { continue }
            best = pattern
        }
        return best?.text
    }

    func record(
        _ request: URLRequest, status: Int, start: Date, duration: TimeInterval, requestSize: Int64?, responseSize: Int64?
    ) -> RequestRecord? {
        guard let target = target(for: request) else { return nil }
        var record = RequestRecord(
            method: (request.httpMethod ?? "GET").uppercased(), status: status,
            durationUs: Int64((max(0, duration) * 1_000_000).rounded()), requestSize: requestSize, responseSize: responseSize,
            time: RequestRecord.timestamp(start), apiVersion: apiVersion, environment: environment)
        switch target {
        case .route(let route): record.route = route
        case .path(let path): record.path = path
        }
        return record
    }

    func record(_ task: URLSessionTask, metrics: URLSessionTaskMetrics) -> RequestRecord? {
        if task.state == .canceling || (task.error as? URLError)?.code == .cancelled { return nil }
        guard let request = task.originalRequest else { return nil }
        let transactions = metrics.transactionMetrics
        let start = transactions.first?.fetchStartDate ?? metrics.taskInterval.start
        let end = transactions.last?.responseEndDate ?? metrics.taskInterval.end
        let last = transactions.last
        let response = task.response as? HTTPURLResponse
        return record(
            request, status: response?.statusCode ?? 0, start: start, duration: end.timeIntervalSince(start),
            requestSize: last.map(\.countOfRequestBodyBytesSent), responseSize: response == nil ? nil : last.map(\.countOfResponseBodyBytesReceived))
    }
}

actor RequestCore {
    typealias Sleep = AnalyticsCore.Sleep

    nonisolated let endpoint: URL
    private let requireConsent: Bool
    private let session: URLSession
    private let autoFlush: Duration?
    private let now: @Sendable () -> Date
    private let sleep: Sleep
    private let logger: Logger?
    private var app: AppInfo
    private var queue: RequestQueue
    private var consented: Bool
    private var optedOut: Bool
    private var generation = 0
    private var failures = 0
    private var sender: Task<Void, Never>?
    private var senderID = 0
    private var senderWaiting = false

    init(
        endpoint: URL, app: AppInfo, requireConsent: Bool, optedOut: Bool, queueFile: URL?, session: URLSession,
        autoFlush: Duration? = .seconds(5), debug: Bool = false, now: @escaping @Sendable () -> Date = Date.init,
        sleep: @escaping Sleep = { try await Task.sleep(for: $0) }
    ) {
        self.endpoint = endpoint
        self.app = app
        self.requireConsent = requireConsent
        self.optedOut = optedOut
        self.consented = !requireConsent
        self.session = session
        self.autoFlush = autoFlush
        self.now = now
        self.sleep = sleep
        self.logger = debug ? Logger(subsystem: "dev.yuva.analytics", category: "YuvaAnalytics") : nil
        var queue = RequestQueue(file: queueFile)
        if optedOut { queue.clear() }
        queue.prune(now: Int64(now().timeIntervalSince1970 * 1000))
        queue.save()
        self.queue = queue
    }

    var pending: [RequestBatch] { queue.batches }

    func start() {
        if !queue.batches.isEmpty, !optedOut { schedule(after: .zero) }
    }

    func handle(_ command: AnalyticsCommand) {
        switch command {
        case .request(var record):
            let name = "\(record.method) \(record.route ?? record.path ?? "")"
            guard !optedOut, consented else {
                let reason = optedOut ? "opted out" : "no consent"
                logger?.debug("dropped request \(name, privacy: .public): \(reason, privacy: .public)")
                return
            }
            if requireConsent { record.c = ConsentBody(a: 1) }
            guard queue.append(record, app: app) else { return }
            queue.save()
            logger?.debug(
                "queued request \(name, privacy: .public) \(record.status, privacy: .public) in \(record.durationUs / 1000, privacy: .public) ms")
            if let autoFlush { schedule(after: autoFlush) }
        case .consent(let granted, _):
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
            queue.updateApp { if $0.width == nil { $0.width = width } }
        case .queuedRequests(let reply):
            reply(queue.count)
        case .record, .identify:
            break
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
        while !optedOut, !Task.isCancelled {
            guard let delay = await sendNext() else { break }
            guard delay > .zero else { continue }
            logger?.debug("retrying requests in \(delay.components.seconds, privacy: .public) s")
            do { try await sleep(delay) } catch { break }
        }
        if id == senderID { sender = nil }
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
        let status: Int
        let retryAfter: String?
        do {
            let (_, response) = try await session.data(for: request)
            let http = response as? HTTPURLResponse
            status = http?.statusCode ?? 0
            retryAfter = http?.value(forHTTPHeaderField: "Retry-After")
        } catch {
            guard started == generation else { return .zero }
            logger?.debug("sending requests failed: \(error.localizedDescription, privacy: .public)")
            return backoff()
        }
        guard started == generation else { return .zero }
        logger?.debug("sent \(batch.count, privacy: .public) requests: \(status, privacy: .public)")
        switch status {
        case 429, 503:
            return AnalyticsCore.retryDelay(retryAfter)
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
}

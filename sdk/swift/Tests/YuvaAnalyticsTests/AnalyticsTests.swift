import Foundation
import Testing
@testable import YuvaAnalytics

final class FakeCollector: @unchecked Sendable {
    private let lock = NSLock()
    private var answers: [(Int, [String: String])]
    private var bodies: [Data] = []
    private var headers: [[String: String]] = []
    let offline: Bool

    init(answers: [(Int, [String: String])] = [], offline: Bool = false) {
        self.answers = answers
        self.offline = offline
    }

    var requests: [Data] { lock.withLock { bodies } }
    var requestHeaders: [[String: String]] { lock.withLock { headers } }

    var batches: [EventBatch] {
        requests.map { try! JSONDecoder().decode(EventBatch.self, from: $0) }
    }

    func respond(_ body: Data, _ header: [String: String]) throws -> (Int, [String: String]) {
        try lock.withLock {
            bodies.append(body)
            headers.append(header)
            if offline { throw URLError(.notConnectedToInternet) }
            return answers.isEmpty ? (202, [:]) : answers.removeFirst()
        }
    }
}

final class CollectorProtocol: URLProtocol, @unchecked Sendable {
    private static let lock = NSLock()
    nonisolated(unsafe) private static var collectors: [String: FakeCollector] = [:]

    static func session(for collector: FakeCollector) -> URLSession {
        let id = UUID().uuidString
        lock.withLock { collectors[id] = collector }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [CollectorProtocol.self]
        configuration.httpAdditionalHeaders = ["X-Collector": id]
        return URLSession(configuration: configuration)
    }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func stopLoading() {}

    override func startLoading() {
        let id = request.value(forHTTPHeaderField: "X-Collector") ?? ""
        guard let collector = Self.lock.withLock({ Self.collectors[id] }), let url = request.url else {
            client?.urlProtocol(self, didFailWithError: URLError(.badServerResponse))
            return
        }
        do {
            let (status, headers) = try collector.respond(body(), request.allHTTPHeaderFields ?? [:])
            let response = HTTPURLResponse(url: url, statusCode: status, httpVersion: nil, headerFields: headers)!
            client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: Data())
            client?.urlProtocolDidFinishLoading(self)
        } catch {
            client?.urlProtocol(self, didFailWithError: error)
        }
    }

    private func body() -> Data {
        if let data = request.httpBody { return data }
        guard let stream = request.httpBodyStream else { return Data() }
        stream.open()
        defer { stream.close() }
        var data = Data()
        var buffer = [UInt8](repeating: 0, count: 16 * 1024)
        while stream.hasBytesAvailable {
            let read = stream.read(&buffer, maxLength: buffer.count)
            if read <= 0 { break }
            data.append(buffer, count: read)
        }
        return data
    }
}

final class Sleeps: @unchecked Sendable {
    private let lock = NSLock()
    private var log: [Duration] = []
    var recorded: [Duration] { lock.withLock { log } }
    func record(_ duration: Duration) { lock.withLock { log.append(duration) } }
}

let now = Date(timeIntervalSince1970: 1_760_263_200)
let nowMs = Int64(now.timeIntervalSince1970 * 1000)
let app = AppInfo(platform: "ios", version: "2.3.1", osVersion: "18.1", device: "mobile", width: 390, language: "tr")

func tempFile() -> URL {
    FileManager.default.temporaryDirectory.appendingPathComponent("yuva-analytics-\(UUID().uuidString).json")
}

func tempStore() -> PersistentIDStore {
    PersistentIDStore(defaults: UserDefaults(suiteName: "yuva-analytics-\(UUID().uuidString)")!)
}

func makeCore(
    key: String = "0f8fad5b-d9cb-469f-a165-70867728950e", file: URL? = nil, collector: FakeCollector,
    requireConsent: Bool = false, optedOut: Bool = false, sleeps: Sleeps = Sleeps(), ids: PersistentIDStore = tempStore()
) -> AnalyticsCore {
    AnalyticsCore(
        endpoint: URL(string: "https://yuva.test/client/v1/analytics/yuva_ak_test")!, processKey: key, app: app,
        home: "app://dev.yuva.example/", requireConsent: requireConsent, optedOut: optedOut, queueFile: file, ids: ids,
        session: CollectorProtocol.session(for: collector), autoFlush: nil, now: { now },
        sleep: { sleeps.record($0) })
}

func screen(_ name: String, at t: Int64 = nowMs) -> AnalyticsCommand {
    .record(QueuedEvent(n: "screen", u: "app://dev.yuva.example/\(name)", t: t))
}

func send(_ core: AnalyticsCore) async {
    await core.handle(.flush(nil))
    await core.idle()
}

@Test func queuedEventsKeepTheirTimesAndProcessKeyAcrossALaunch() async throws {
    let file = tempFile()
    defer { try? FileManager.default.removeItem(at: file) }
    let first = makeCore(key: "first-process-key-0001", file: file, collector: FakeCollector(offline: true))
    await first.handle(screen("Home", at: nowMs - 3_600_000))
    await first.handle(screen("Cart", at: nowMs - 1_800_000))

    let collector = FakeCollector()
    let second = makeCore(key: "second-process-key-0002", file: file, collector: collector)
    await second.handle(screen("Home"))
    await send(second)

    let batches = collector.batches
    #expect(batches.map(\.k) == ["first-process-key-0001", "second-process-key-0002"])
    #expect(batches[0].e.map(\.t) == [nowMs - 3_600_000, nowMs - 1_800_000])
    #expect(batches[0].e.map(\.u) == ["app://dev.yuva.example/Home", "app://dev.yuva.example/Cart"])
    #expect(batches[1].e.map(\.t) == [nowMs])
    #expect(batches.allSatisfy { $0.a == app })
    #expect(await second.pending.isEmpty)
    #expect(!FileManager.default.fileExists(atPath: file.path))
}

@Test func eventsOlderThan72HoursAreDroppedAtLaunch() async throws {
    let file = tempFile()
    defer { try? FileManager.default.removeItem(at: file) }
    let first = makeCore(key: "first-process-key-0001", file: file, collector: FakeCollector(offline: true))
    await first.handle(screen("Old", at: nowMs - 73 * 3_600_000))
    await first.handle(screen("Recent", at: nowMs - 71 * 3_600_000))

    let second = makeCore(key: "second-process-key-0002", file: file, collector: FakeCollector())
    #expect(await second.pending.flatMap(\.e).map(\.u) == ["app://dev.yuva.example/Recent"])
}

@Test func optOutStopsSendingAndClearsTheQueue() async throws {
    let file = tempFile()
    defer { try? FileManager.default.removeItem(at: file) }
    let collector = FakeCollector()
    let core = makeCore(file: file, collector: collector)
    await core.handle(screen("Home"))
    await core.handle(screen("Cart"))
    #expect(FileManager.default.fileExists(atPath: file.path))

    await core.handle(.optOut)
    #expect(await core.pending.isEmpty)
    #expect(!FileManager.default.fileExists(atPath: file.path))

    await core.handle(screen("Checkout"))
    await send(core)
    #expect(collector.requests.isEmpty)

    await core.handle(.optIn)
    await core.handle(screen("Checkout"))
    await send(core)
    #expect(collector.batches.flatMap(\.e).map(\.u) == ["app://dev.yuva.example/Checkout"])
}

@Test func anOptedOutLaunchDeletesTheQueueItFinds() async throws {
    let file = tempFile()
    defer { try? FileManager.default.removeItem(at: file) }
    let first = makeCore(file: file, collector: FakeCollector(offline: true))
    await first.handle(screen("Home"))

    let second = makeCore(file: file, collector: FakeCollector(), optedOut: true)
    #expect(await second.pending.isEmpty)
    #expect(!FileManager.default.fileExists(atPath: file.path))
}

@Test func rateLimitedBatchIsRetriedAfterRetryAfter() async throws {
    let collector = FakeCollector(answers: [(429, ["Retry-After": "7"]), (503, ["Retry-After": "2"])])
    let sleeps = Sleeps()
    let core = makeCore(collector: collector, sleeps: sleeps)
    await core.handle(screen("Home"))
    await send(core)

    #expect(sleeps.recorded == [.seconds(7), .seconds(2)])
    #expect(collector.requests.count == 3)
    #expect(collector.batches.allSatisfy { $0 == collector.batches[0] })
    #expect(await core.pending.isEmpty)
}

@Test func refusedBatchIsDropped() async throws {
    let collector = FakeCollector(answers: [(403, [:])])
    let core = makeCore(collector: collector)
    await core.handle(screen("Home"))
    await send(core)

    #expect(collector.requests.count == 1)
    #expect(await core.pending.isEmpty)
}

@Test func networkFailureKeepsTheBatchAndBacksOff() async throws {
    let collector = FakeCollector(answers: [(500, [:])])
    let sleeps = Sleeps()
    let core = makeCore(collector: collector, sleeps: sleeps)
    await core.handle(screen("Home"))
    await send(core)

    #expect(sleeps.recorded == [.seconds(5)])
    #expect(collector.requests.count == 2)
    #expect(await core.pending.isEmpty)
}

@Test func batchesHoldAtMost100EventsAnd64KiB() async throws {
    let collector = FakeCollector()
    let core = makeCore(collector: collector)
    for index in 0..<250 { await core.handle(screen("Screen\(index)", at: nowMs - Int64(250 - index))) }
    await send(core)
    #expect(collector.batches.map(\.e.count) == [100, 100, 50])

    let large = FakeCollector()
    let heavy = makeCore(collector: large)
    let text = String(repeating: "x", count: 1_900)
    for index in 0..<90 {
        let props: [String: YuvaAnalyticsValue] = ["a": .string(text), "b": .string(text), "c": .string(text)]
        await heavy.handle(.record(QueuedEvent(n: "Viewed", u: "app://dev.yuva.example/Item", t: nowMs - Int64(index), p: props)))
    }
    await send(heavy)
    #expect(large.requests.allSatisfy { $0.count <= 64 * 1024 })
    #expect(large.requests.count > 1)
    #expect(large.batches.map(\.e.count).reduce(0, +) == 90)
}

@Test func consentPropertySendsNothingUntilConsentAndMarksEvents() async throws {
    let collector = FakeCollector()
    let core = makeCore(collector: collector, requireConsent: true)
    await core.handle(screen("Before"))
    await core.handle(.consent(true, persistent: false))
    await core.handle(screen("After"))
    await send(core)

    let events = collector.batches.flatMap(\.e)
    #expect(events.map(\.u) == ["app://dev.yuva.example/After"])
    #expect(events.first?.c == ConsentBody(a: 1))
}

@Test func customEventsCarryPropsRevenueAndInteractivity() async throws {
    let collector = FakeCollector()
    let core = makeCore(collector: collector)
    await core.handle(
        .record(
            QueuedEvent(
                n: "Purchase", u: "app://dev.yuva.example/Cart", t: nowMs, p: ["plan": "pro", "seats": 3],
                revenue: RevenueBody(a: "9.99", c: "EUR"), i: false)))
    await send(core)

    let json = try #require(collector.requests.first.flatMap { try JSONSerialization.jsonObject(with: $0) as? [String: Any] })
    #expect(json["k"] as? String == "0f8fad5b-d9cb-469f-a165-70867728950e")
    let a = try #require(json["a"] as? [String: Any])
    #expect(a["platform"] as? String == "ios")
    #expect(a["os_version"] as? String == "18.1")
    let event = try #require((json["e"] as? [[String: Any]])?.first)
    #expect(event["n"] as? String == "Purchase")
    #expect(event["i"] as? Bool == false)
    #expect((event["$"] as? [String: String]) == ["a": "9.99", "c": "EUR"])
    #expect((event["p"] as? [String: Any])?["seats"] as? Int == 3)
    #expect(collector.requestHeaders.first?["Content-Type"] == "application/json")
}

@Test func screenNamesArePercentEncoded() {
    #expect(YuvaAnalytics.screenURL(appId: "com.example.shop", name: "Product Detail") == "app://com.example.shop/Product%20Detail")
    #expect(YuvaAnalytics.screenURL(appId: "com.example.shop", name: "/Settings/Profile") == "app://com.example.shop/Settings/Profile")
    #expect(YuvaAnalytics.screenURL(appId: "com.example.shop", name: "Ürün?#") == "app://com.example.shop/%C3%9Cr%C3%BCn%3F%23")
}

@Test func defaultModeKeepsNoIdAndSendsNoPersistentConsent() async throws {
    let ids = tempStore()
    let collector = FakeCollector()
    let core = makeCore(collector: collector, ids: ids)
    await core.handle(screen("Home"))
    await core.handle(.consent(true, persistent: false))
    await core.handle(screen("Cart"))
    await core.handle(.identify("token"))
    await send(core)

    #expect(ids.load() == nil)
    #expect(collector.batches.flatMap(\.e).map(\.n) == ["screen", "screen"])
    #expect(collector.batches.flatMap(\.e).allSatisfy { $0.c == nil })
}

@Test func persistentConsentKeepsOneIdAndSendsItOnEveryEvent() async throws {
    let ids = tempStore()
    let collector = FakeCollector()
    let core = makeCore(collector: collector, requireConsent: true, ids: ids)
    await core.handle(.consent(true, persistent: true))
    let stored = try #require(ids.load())
    #expect(stored.createdAt == nowMs)
    #expect(UUID(uuidString: stored.id) != nil)
    await core.handle(screen("Home"))
    await send(core)

    let next = FakeCollector()
    let relaunched = makeCore(collector: next, requireConsent: true, ids: ids)
    await relaunched.handle(.consent(true, persistent: true))
    await relaunched.handle(screen("Cart"))
    await send(relaunched)

    #expect(ids.load() == stored)
    #expect((collector.batches + next.batches).flatMap(\.e).map(\.c) == [
        ConsentBody(a: 1, p: stored.id), ConsentBody(a: 1, p: stored.id),
    ])
}

@Test func anIdOlderThan13MonthsIsReplaced() async throws {
    let ids = tempStore()
    var calendar = Calendar(identifier: .gregorian)
    calendar.timeZone = TimeZone(identifier: "UTC")!
    let old = calendar.date(byAdding: .month, value: -13, to: now)!
    ids.save(PersistentID(id: "11111111-2222-4333-8444-555555555555", createdAt: Int64(old.timeIntervalSince1970 * 1000)))
    let collector = FakeCollector()
    let core = makeCore(collector: collector, ids: ids)
    #expect(ids.load() == nil)

    await core.handle(screen("Home"))
    await core.handle(.consent(true, persistent: true))
    await core.handle(screen("Cart"))
    await send(core)

    let fresh = try #require(ids.load())
    #expect(fresh.id != "11111111-2222-4333-8444-555555555555")
    #expect(collector.batches.flatMap(\.e).map(\.c) == [nil, ConsentBody(a: 1, p: fresh.id)])

    let young = tempStore()
    let recent = calendar.date(byAdding: .day, value: 1, to: old)!
    let kept = PersistentID(id: "11111111-2222-4333-8444-555555555555", createdAt: Int64(recent.timeIntervalSince1970 * 1000))
    young.save(kept)
    _ = makeCore(collector: FakeCollector(), ids: young)
    #expect(young.load() == kept)
}

@Test func identifySendsTheTokenRightAwayOnlyOnceAnIdExists() async throws {
    let ids = tempStore()
    let collector = FakeCollector()
    let core = makeCore(collector: collector, ids: ids)
    await core.handle(.identify("before"))
    await core.idle()
    #expect(collector.requests.isEmpty)

    await core.handle(.consent(true, persistent: true))
    await core.handle(screen("Home"))
    await core.handle(.identify("signed.jwt.token"))
    await core.idle()

    let id = try #require(ids.load()?.id)
    let json = try #require(collector.requests.first.flatMap { try JSONSerialization.jsonObject(with: $0) as? [String: Any] })
    let event = try #require((json["e"] as? [[String: Any]])?.first)
    #expect(event["n"] as? String == "yuva_consent")
    #expect(event["u"] as? String == "app://dev.yuva.example/")
    let consent = try #require(event["c"] as? [String: Any])
    #expect(consent["a"] as? Int == 1)
    #expect(consent["p"] as? String == id)
    #expect(consent["j"] as? String == "signed.jwt.token")
    #expect(collector.requests.count == 1)
    #expect(await core.pending.flatMap(\.e).map(\.c) == [ConsentBody(a: 1, p: id)])
}

@Test func identifyWhileOfflineIsDroppedNotQueued() async throws {
    let ids = tempStore()
    let collector = FakeCollector(offline: true)
    let core = makeCore(collector: collector, ids: ids)
    await core.handle(.consent(true, persistent: true))
    await core.handle(.identify("signed.jwt.token"))
    await core.idle()

    #expect(collector.requests.count == 1)
    #expect(await core.pending.isEmpty)
    #expect(ids.load() != nil)
}

@Test func withdrawingPersistentSendsTheIdThenForgetsIt() async throws {
    let ids = tempStore()
    let collector = FakeCollector()
    let core = makeCore(collector: collector, ids: ids)
    await core.handle(.consent(true, persistent: true))
    let id = try #require(ids.load()?.id)
    await core.handle(screen("Home"))
    await core.handle(.consent(true, persistent: false))
    await core.idle()

    #expect(ids.load() == nil)
    let withdrawal = try #require(collector.batches.first?.e.first)
    #expect(withdrawal.n == "yuva_consent")
    #expect(withdrawal.c == ConsentBody(a: 0, p: id))
    #expect(await core.pending.flatMap(\.e).map(\.c) == [ConsentBody(a: 1)])

    await core.handle(.identify("signed.jwt.token"))
    await send(core)
    #expect(collector.batches.flatMap(\.e).map(\.n) == ["yuva_consent", "screen"])
    #expect(collector.batches.flatMap(\.e).allSatisfy { $0.c?.j == nil })
}

@Test func consentFalseAndOptOutWithdrawAndDeleteTheId() async throws {
    for command in [AnalyticsCommand.consent(false, persistent: true), .optOut] {
        let ids = tempStore()
        let collector = FakeCollector()
        let core = makeCore(collector: collector, ids: ids)
        await core.handle(.consent(true, persistent: true))
        let id = try #require(ids.load()?.id)
        await core.handle(screen("Home"))
        await core.handle(command)
        await core.idle()

        #expect(ids.load() == nil)
        #expect(await core.pending.isEmpty)
        #expect(collector.batches.map(\.e) == [[QueuedEvent(n: "yuva_consent", u: "app://dev.yuva.example/", t: nowMs, c: ConsentBody(a: 0, p: id))]])
    }
}

@Test func optedOutDevicesMakeNoId() async throws {
    let ids = tempStore()
    let core = makeCore(collector: FakeCollector(), optedOut: true, ids: ids)
    await core.handle(.consent(true, persistent: true))
    #expect(ids.load() == nil)
}

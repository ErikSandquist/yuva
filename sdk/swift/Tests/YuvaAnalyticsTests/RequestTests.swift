import Foundation
import Testing
@testable import YuvaAnalytics

extension FakeCollector {
    var requestBatches: [RequestBatch] {
        requests.map { try! JSONDecoder().decode(RequestBatch.self, from: $0) }
    }
}

let apiHost = "api.shop.test"

func matcher(
    routes: [String] = ["/users/{id}", "/users/me", "/orders/{id}/items/{item}", "/health"],
    route: (@Sendable (URLRequest) -> String?)? = nil, sendPaths: Bool = false
) -> RequestMatcher {
    RequestMatcher(
        YuvaRequestMonitoring(
            key: "yuva_ak_api", hosts: ["API.shop.test"], routes: routes, route: route, sendUnmatchedPaths: sendPaths,
            apiVersion: "v2", environment: "staging"))
}

func get(_ path: String, host: String = apiHost, method: String = "GET") -> URLRequest {
    var request = URLRequest(url: URL(string: "https://\(host)\(path)")!)
    request.httpMethod = method
    return request
}

func makeRequests(
    file: URL? = nil, collector: FakeCollector, requireConsent: Bool = false, optedOut: Bool = false,
    sleeps: Sleeps = Sleeps()
) -> RequestCore {
    RequestCore(
        endpoint: URL(string: "https://yuva.test/client/v1/analytics/yuva_ak_api/requests")!, app: app,
        requireConsent: requireConsent, optedOut: optedOut, queueFile: file,
        session: CollectorProtocol.session(for: collector), autoFlush: nil, now: { now }, sleep: { sleeps.record($0) })
}

func call(_ path: String = "/users/42", status: Int = 200, at date: Date = now) -> AnalyticsCommand {
    .request(
        matcher(sendPaths: true).record(
            get(path), status: status, start: date, duration: 0.0125, requestSize: 0, responseSize: 512)!)
}

func sendRequests(_ core: RequestCore) async {
    await core.handle(.flush(nil))
    await core.idle()
}

@Test func routesComeFromTheHostsPatternsWithTheMostSpecificWinning() {
    let m = matcher()
    #expect(m.target(for: get("/users/42")) == .route("/users/{id}"))
    #expect(m.target(for: get("/users/me")) == .route("/users/me"))
    #expect(m.target(for: get("/users/42/")) == .route("/users/{id}"))
    #expect(m.target(for: get("/orders/9/items/3?expand=1#top")) == .route("/orders/{id}/items/{item}"))
    #expect(m.target(for: get("/health")) == .route("/health"))
}

@Test func unmatchedRequestsAndOtherHostsAreSkipped() {
    let m = matcher()
    #expect(m.target(for: get("/users")) == nil)
    #expect(m.target(for: get("/users/42/orders")) == nil)
    #expect(m.target(for: get("/users/42", host: "cdn.shop.test")) == nil)
    #expect(m.target(for: get("/users/42", host: "api.shop.test.evil.example")) == nil)
    #expect(m.record(get("/unknown/1"), status: 200, start: now, duration: 0.1, requestSize: nil, responseSize: nil) == nil)
}

@Test func unmatchedPathsAreSentOnlyWhenTheHostAsksWithoutQueryOrFragment() {
    let m = matcher(sendPaths: true)
    #expect(m.target(for: get("/search/abc?q=secret#frag")) == .path("/search/abc"))
    #expect(m.target(for: get("")) == .path("/"))
    #expect(m.target(for: get("/users/42")) == .route("/users/{id}"))
    #expect(m.target(for: get("/x", host: "other.test")) == nil)
}

@Test func theHostsClosureComesFirstAndFallsBackToPatterns() {
    let m = matcher(route: { $0.url?.path.hasPrefix("/v1/carts/") == true ? "/v1/carts/{cart}" : nil })
    #expect(m.target(for: get("/v1/carts/abc")) == .route("/v1/carts/{cart}"))
    #expect(m.target(for: get("/users/42")) == .route("/users/{id}"))
    #expect(m.target(for: get("/v1/other")) == nil)
}

@Test func aRecordCarriesMethodRouteTimingSizesAndTheHostsLabels() throws {
    let record = try #require(
        matcher().record(
            get("/users/42?token=x", method: "post"), status: 201, start: now, duration: 0.0125, requestSize: 64,
            responseSize: 2048))
    #expect(record == RequestRecord(
        method: "POST", route: "/users/{id}", status: 201, durationUs: 12_500, requestSize: 64, responseSize: 2048,
        time: "2025-10-12T10:00:00.000Z", apiVersion: "v2", environment: "staging"))
    #expect(record.millis == nowMs)
}

@Test func urlSessionMetricsBecomeARecord() async throws {
    final class Capture: NSObject, URLSessionTaskDelegate, @unchecked Sendable {
        var record: RequestRecord?
        let matcher: RequestMatcher
        init(_ matcher: RequestMatcher) { self.matcher = matcher }
        func urlSession(_ session: URLSession, task: URLSessionTask, didFinishCollecting metrics: URLSessionTaskMetrics) {
            record = matcher.record(task, metrics: metrics)
        }
    }
    let collector = FakeCollector(answers: [(404, [:])])
    let capture = Capture(matcher())
    _ = try await CollectorProtocol.session(for: collector).data(for: get("/users/7"), delegate: capture)
    let record = try #require(capture.record)
    #expect(record.method == "GET")
    #expect(record.route == "/users/{id}")
    #expect(record.path == nil)
    #expect(record.status == 404)
    #expect(record.durationUs >= 0)
    #expect(record.millis != nil)
}

@Test func requestsAreBatchedAt100AndKept64KiB() async throws {
    let collector = FakeCollector()
    let core = makeRequests(collector: collector)
    for index in 0..<250 { await core.handle(call("/users/\(index)")) }
    #expect(await queued(core) == 250)
    await sendRequests(core)
    #expect(collector.requestBatches.map(\.records.count) == [100, 100, 50])
    #expect(collector.requests.allSatisfy { $0.count <= 64 * 1024 })
    #expect(collector.requestBatches.allSatisfy { $0.a == app })
    #expect(await core.pending.isEmpty)

    let large = FakeCollector()
    let heavy = makeRequests(collector: large)
    let long = "/" + String(repeating: "p", count: 1_990)
    for _ in 0..<60 { await heavy.handle(call(long)) }
    await sendRequests(heavy)
    #expect(large.requests.count > 1)
    #expect(large.requests.allSatisfy { $0.count <= 64 * 1024 })
    #expect(large.requestBatches.map(\.records.count).reduce(0, +) == 60)
}

@Test func aConsentPropertySendsNoRequestBeforeConsentAndMarksEachAfter() async throws {
    let collector = FakeCollector()
    let core = makeRequests(collector: collector, requireConsent: true)
    await core.handle(call("/users/1"))
    #expect(await core.pending.isEmpty)
    await core.handle(.consent(true, persistent: false))
    await core.handle(call("/users/2"))
    await core.handle(call("/users/3"))
    await core.handle(.consent(false, persistent: false))
    #expect(await core.pending.isEmpty)
    await core.handle(call("/users/4"))
    await core.handle(.consent(true, persistent: false))
    await core.handle(call("/users/5"))
    await sendRequests(core)

    let records = collector.requestBatches.flatMap(\.records)
    #expect(records.count == 1)
    #expect(records.allSatisfy { $0.c == ConsentBody(a: 1) })
}

@Test func withoutAConsentModeRequestsCarryNoConsentBlock() async throws {
    let collector = FakeCollector()
    let core = makeRequests(collector: collector)
    await core.handle(call())
    await sendRequests(core)
    #expect(collector.requestBatches.flatMap(\.records).map(\.c) == [nil])
}

@Test func optOutClearsTheRequestQueueAndStopsRecording() async throws {
    let file = tempFile()
    defer { try? FileManager.default.removeItem(at: file) }
    let collector = FakeCollector()
    let core = makeRequests(file: file, collector: collector)
    await core.handle(call("/users/1"))
    #expect(FileManager.default.fileExists(atPath: file.path))
    await core.handle(.optOut)
    #expect(await core.pending.isEmpty)
    #expect(!FileManager.default.fileExists(atPath: file.path))
    await core.handle(call("/users/2"))
    await sendRequests(core)
    #expect(collector.requests.isEmpty)

    await core.handle(.optIn)
    await core.handle(call("/users/3"))
    await sendRequests(core)
    #expect(collector.requestBatches.flatMap(\.records).map(\.route) == ["/users/{id}"])

    let relaunch = makeRequests(file: file, collector: FakeCollector(), optedOut: true)
    #expect(await relaunch.pending.isEmpty)
}

@Test func queuedRequestsSurviveARelaunchAndExpireAfter72Hours() async throws {
    let file = tempFile()
    defer { try? FileManager.default.removeItem(at: file) }
    let first = makeRequests(file: file, collector: FakeCollector(offline: true))
    await first.handle(call("/users/1", at: now.addingTimeInterval(-73 * 3600)))
    await first.handle(call("/users/2", at: now.addingTimeInterval(-71 * 3600)))

    let collector = FakeCollector()
    let second = makeRequests(file: file, collector: collector)
    #expect(await second.pending.flatMap(\.records).count == 1)
    await sendRequests(second)
    #expect(collector.requestBatches.flatMap(\.records).map(\.time) == [RequestRecord.timestamp(now.addingTimeInterval(-71 * 3600))])
    #expect(!FileManager.default.fileExists(atPath: file.path))
}

@Test func rateLimitedAndFailedRequestBatchesAreRetriedAndRefusedOnesDropped() async throws {
    let collector = FakeCollector(answers: [(429, ["Retry-After": "7"]), (500, [:]), (202, [:]), (403, [:])])
    let sleeps = Sleeps()
    let core = makeRequests(collector: collector, sleeps: sleeps)
    await core.handle(call())
    await sendRequests(core)
    #expect(sleeps.recorded == [.seconds(7), .seconds(5)])
    #expect(collector.requests.count == 3)
    await core.handle(call())
    await sendRequests(core)
    #expect(collector.requests.count == 4)
    #expect(await core.pending.isEmpty)
}

@Test func theWireFormatMatchesTheOpenAPISchema() async throws {
    let collector = FakeCollector()
    let core = makeRequests(collector: collector, requireConsent: true)
    await core.handle(.consent(true, persistent: false))
    await core.handle(call("/users/42", status: 0))
    await core.handle(call("/nowhere/1"))
    await sendRequests(core)

    #expect(collector.requestHeaders.first?["Content-Type"] == "application/json")
    let json = try #require(collector.requests.first.flatMap { try JSONSerialization.jsonObject(with: $0) as? [String: Any] })
    let batch = try schema("AnalyticsAppRequests")
    #expect(Set(json.keys) == batch.properties)
    #expect(batch.required.isSubset(of: Set(json.keys)))
    let appSchema = try schema("AnalyticsApp")
    let a = try #require(json["a"] as? [String: Any])
    #expect(Set(a.keys).isSubset(of: appSchema.properties))
    #expect(a["platform"] as? String == "ios")
    #expect(a["version"] as? String == "2.3.1")

    let recordSchema = try schema("AnalyticsAppRequestRecord")
    let records = try #require(json["records"] as? [[String: Any]])
    #expect(records.count == 2)
    for record in records {
        #expect(Set(record.keys).isSubset(of: recordSchema.properties))
        #expect(recordSchema.required.isSubset(of: Set(record.keys)))
        #expect((record["route"] == nil) != (record["path"] == nil))
        #expect(record["duration_us"] as? Int == 12_500)
        #expect(try ISO8601DateFormatter.fractional.date(from: #require(record["time"] as? String)) != nil)
        #expect((record["c"] as? [String: Int]) == ["a": 1])
    }
    #expect(records[0]["status"] as? Int == 0)
    #expect(records[0]["route"] as? String == "/users/{id}")
    #expect(records[1]["path"] as? String == "/nowhere/1")
}

extension ISO8601DateFormatter {
    nonisolated(unsafe) static let fractional: ISO8601DateFormatter = {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter
    }()
}

func queued(_ core: RequestCore) async -> Int {
    await withCheckedContinuation { continuation in
        Task { await core.handle(.queuedRequests { continuation.resume(returning: $0) }) }
    }
}

func schema(_ name: String) throws -> (properties: Set<String>, required: Set<String>) {
    let spec = URL(fileURLWithPath: #filePath).deletingLastPathComponent().appendingPathComponent("../../../../openapi/openapi.yaml")
    let lines = try String(contentsOf: spec.standardizedFileURL, encoding: .utf8).components(separatedBy: "\n")
    let start = try #require(lines.firstIndex(of: "    \(name):"))
    var properties = Set<String>()
    var required = Set<String>()
    var inProperties = false
    for line in lines[(start + 1)...] {
        let indent = line.prefix { $0 == " " }.count
        let text = line.trimmingCharacters(in: .whitespaces)
        if text.isEmpty { continue }
        if indent <= 4 { break }
        if indent == 6 {
            inProperties = text == "properties:"
            if text.hasPrefix("required: [") {
                required = Set(text.dropFirst(11).dropLast().split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) })
            }
        } else if indent == 8, inProperties, text.hasSuffix(":") {
            properties.insert(String(text.dropLast()))
        }
    }
    return (properties, required)
}

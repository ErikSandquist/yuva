import Foundation

struct RequestRecord: Codable, Sendable, Equatable {
    var method: String
    var route: String?
    var path: String?
    var status: Int
    var durationUs: Int64
    var requestSize: Int64?
    var responseSize: Int64?
    var time: String
    var apiVersion: String?
    var environment: String?
    var c: ConsentBody?

    enum CodingKeys: String, CodingKey {
        case method, route, path, status, time, environment, c
        case durationUs = "duration_us"
        case requestSize = "request_size"
        case responseSize = "response_size"
        case apiVersion = "api_version"
    }

    static let timeFormat = Date.ISO8601FormatStyle(includingFractionalSeconds: true)

    static func timestamp(_ date: Date) -> String {
        date.formatted(timeFormat)
    }

    var millis: Int64? {
        (try? Self.timeFormat.parse(time)).map { Int64($0.timeIntervalSince1970 * 1000) }
    }
}

struct RequestBatch: Codable, Sendable, Equatable {
    var a: AppInfo
    var records: [RequestRecord]
}

struct PendingRequests: Sendable {
    let count: Int
    let body: Data
}

struct RequestQueue: Sendable {
    static let maxRecords = 100
    static let maxBytes = 64 * 1024
    static let maxAge: Int64 = 72 * 3600 * 1000
    static let capacity = 1000

    private(set) var batches: [RequestBatch] = []
    let file: URL?

    init(file: URL?) {
        self.file = file
        if let file, let data = try? Data(contentsOf: file),
           let stored = try? JSONDecoder().decode([RequestBatch].self, from: data) {
            batches = stored
        }
    }

    var count: Int { batches.reduce(0) { $0 + $1.records.count } }

    @discardableResult
    mutating func append(_ record: RequestRecord, app: AppInfo) -> Bool {
        let probe = RequestBatch(a: app, records: [record])
        guard let data = try? JSONEncoder().encode(probe), data.count <= Self.maxBytes else { return false }
        if let last = batches.indices.last, batches[last].a == app {
            batches[last].records.append(record)
        } else {
            batches.append(probe)
        }
        while count > Self.capacity { dropOldest() }
        return true
    }

    mutating func updateApp(_ change: (inout AppInfo) -> Void) {
        for index in batches.indices { change(&batches[index].a) }
    }

    mutating func prune(now: Int64) {
        for index in batches.indices {
            batches[index].records.removeAll { record in
                guard let time = record.millis else { return true }
                return now - time > Self.maxAge
            }
        }
        batches.removeAll { $0.records.isEmpty }
    }

    func next() -> PendingRequests? {
        guard let first = batches.first else { return nil }
        let encoder = JSONEncoder()
        var take = min(first.records.count, Self.maxRecords)
        while true {
            let body = (try? encoder.encode(RequestBatch(a: first.a, records: Array(first.records.prefix(take))))) ?? Data()
            if body.count <= Self.maxBytes || take == 1 { return PendingRequests(count: take, body: body) }
            take = (take + 1) / 2
        }
    }

    mutating func remove(_ pending: PendingRequests) {
        guard !batches.isEmpty else { return }
        batches[0].records.removeFirst(min(pending.count, batches[0].records.count))
        if batches[0].records.isEmpty { batches.removeFirst() }
    }

    mutating func clear() {
        batches.removeAll()
    }

    func save() {
        guard let file else { return }
        if batches.isEmpty {
            try? FileManager.default.removeItem(at: file)
            return
        }
        guard let data = try? JSONEncoder().encode(batches) else { return }
        try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        try? data.write(to: file, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
    }

    private mutating func dropOldest() {
        guard !batches.isEmpty else { return }
        batches[0].records.removeFirst()
        if batches[0].records.isEmpty { batches.removeFirst() }
    }
}

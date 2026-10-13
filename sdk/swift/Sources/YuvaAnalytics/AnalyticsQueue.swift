import Foundation

struct AppInfo: Codable, Sendable, Equatable {
    var platform: String
    var version: String?
    var osVersion: String?
    var device: String?
    var width: Int?
    var language: String?

    enum CodingKeys: String, CodingKey {
        case platform, version, device, width, language
        case osVersion = "os_version"
    }
}

struct RevenueBody: Codable, Sendable, Equatable {
    var a: String
    var c: String
}

struct ConsentBody: Codable, Sendable, Equatable {
    var a: Int
}

struct QueuedEvent: Codable, Sendable, Equatable {
    var n: String
    var u: String
    var t: Int64
    var p: [String: YuvaAnalyticsValue]?
    var revenue: RevenueBody?
    var i: Bool?
    var c: ConsentBody?

    enum CodingKeys: String, CodingKey {
        case n, u, t, p, i, c
        case revenue = "$"
    }
}

struct EventBatch: Codable, Sendable, Equatable {
    var k: String
    var a: AppInfo
    var e: [QueuedEvent]
}

struct PendingBatch: Sendable {
    let key: String
    let count: Int
    let body: Data
}

struct AnalyticsQueue: Sendable {
    static let maxEvents = 100
    static let maxBytes = 64 * 1024
    static let maxAge: Int64 = 72 * 3600 * 1000
    static let capacity = 1000

    private(set) var batches: [EventBatch] = []
    let file: URL?

    init(file: URL?) {
        self.file = file
        if let file, let data = try? Data(contentsOf: file),
           let stored = try? JSONDecoder().decode([EventBatch].self, from: data) {
            batches = stored
        }
    }

    var count: Int { batches.reduce(0) { $0 + $1.e.count } }

    @discardableResult
    mutating func append(_ event: QueuedEvent, key: String, app: AppInfo) -> Bool {
        let probe = EventBatch(k: key, a: app, e: [event])
        guard let data = try? JSONEncoder().encode(probe), data.count <= Self.maxBytes else { return false }
        if let index = batches.lastIndex(where: { $0.k == key }) {
            batches[index].e.append(event)
        } else {
            batches.append(probe)
        }
        while count > Self.capacity { dropOldest() }
        return true
    }

    mutating func updateApp(key: String, _ change: (inout AppInfo) -> Void) {
        for index in batches.indices where batches[index].k == key { change(&batches[index].a) }
    }

    mutating func prune(now: Int64) {
        for index in batches.indices {
            batches[index].e.removeAll { now - $0.t > Self.maxAge }
        }
        batches.removeAll { $0.e.isEmpty }
    }

    func next() -> PendingBatch? {
        guard let first = batches.first else { return nil }
        let encoder = JSONEncoder()
        var take = min(first.e.count, Self.maxEvents)
        while true {
            let batch = EventBatch(k: first.k, a: first.a, e: Array(first.e.prefix(take)))
            let body = (try? encoder.encode(batch)) ?? Data()
            if body.count <= Self.maxBytes || take == 1 {
                return PendingBatch(key: first.k, count: take, body: body)
            }
            take = (take + 1) / 2
        }
    }

    mutating func remove(_ pending: PendingBatch) {
        guard let index = batches.firstIndex(where: { $0.k == pending.key }) else { return }
        batches[index].e.removeFirst(min(pending.count, batches[index].e.count))
        if batches[index].e.isEmpty { batches.remove(at: index) }
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
        batches[0].e.removeFirst()
        if batches[0].e.isEmpty { batches.removeFirst() }
    }
}

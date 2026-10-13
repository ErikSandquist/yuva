import Foundation

struct PersistentID: Sendable, Equatable {
    static let lifetimeMonths = 13

    var id: String
    var createdAt: Int64

    static func make(now: Date) -> PersistentID {
        PersistentID(id: UUID().uuidString.lowercased(), createdAt: Int64(now.timeIntervalSince1970 * 1000))
    }

    func expired(at now: Date) -> Bool {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "UTC")!
        let created = Date(timeIntervalSince1970: Double(createdAt) / 1000)
        guard let end = calendar.date(byAdding: .month, value: Self.lifetimeMonths, to: created) else { return true }
        return now >= end
    }
}

final class PersistentIDStore: @unchecked Sendable {
    static let key = "yuva_analytics_persistent"

    private let defaults: UserDefaults

    init(defaults: UserDefaults) {
        self.defaults = defaults
    }

    func load() -> PersistentID? {
        guard let stored = defaults.dictionary(forKey: Self.key), let id = stored["id"] as? String,
              let created = (stored["created_at"] as? NSNumber)?.int64Value
        else { return nil }
        return PersistentID(id: id, createdAt: created)
    }

    func save(_ value: PersistentID) {
        defaults.set(["id": value.id, "created_at": NSNumber(value: value.createdAt)], forKey: Self.key)
    }

    func delete() {
        defaults.removeObject(forKey: Self.key)
    }
}

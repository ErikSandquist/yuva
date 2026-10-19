import Foundation

struct StatusNotice: Equatable {
    let incidents: [YuvaStatusIncident]

    var lead: YuvaStatusIncident { incidents[0] }
    var more: Int { incidents.count - 1 }

    init?(_ incidents: [YuvaStatusIncident]) {
        guard !incidents.isEmpty else { return nil }
        self.incidents = incidents.sorted {
            $0.impact != $1.impact ? $0.impact > $1.impact : $0.updatedAt > $1.updatedAt
        }
    }
}

@MainActor
@Observable
final class StatusDismissals {
    static let shared = StatusDismissals()

    private let defaults: UserDefaults
    private let key: String
    private(set) var entries: Set<String>

    init(defaults: UserDefaults = .standard, key: String = "yuva.status.dismissed") {
        self.defaults = defaults
        self.key = key
        entries = Set(defaults.stringArray(forKey: key) ?? [])
    }

    static func entry(_ incident: YuvaStatusIncident) -> String {
        "\(incident.id)|\(Int64(incident.updatedAt.timeIntervalSince1970 * 1000))"
    }

    func notice(for status: YuvaInboxStatus?) -> StatusNotice? {
        StatusNotice((status?.open ?? []).filter { !entries.contains(Self.entry($0)) })
    }

    func dismiss(_ notice: StatusNotice) {
        entries.formUnion(notice.incidents.map(Self.entry))
        defaults.set(Array(entries), forKey: key)
    }

    func prune(keeping status: YuvaInboxStatus?) {
        let open = Set((status?.open ?? []).map(Self.entry))
        let kept = entries.intersection(open)
        guard kept != entries else { return }
        entries = kept
        defaults.set(Array(kept), forKey: key)
    }
}

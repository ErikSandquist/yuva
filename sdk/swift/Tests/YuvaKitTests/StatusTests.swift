import Foundation
import Testing
@testable import YuvaKit

@Suite struct StatusTests {
    private static let inbox = """
        {"id":"i1","name":"Acme","branding":{},"default_locale":"en","timezone":"UTC","mode":"live",\
        "open_now":true,"feedback_categories":[],\
        "status":{"url":"https://status.example/acme","state":"partial_outage","open":[\
        {"id":"a","kind":"incident","title":"API errors","impact":"major","state":"identified",\
        "updated_at":"2026-10-01T10:00:00Z","url":"https://status.example/acme/incidents/a"},\
        {"id":"b","kind":"maintenance","title":"Database upgrade","impact":"none","state":"in_progress",\
        "updated_at":"2026-10-01T09:00:00.250Z","url":"https://status.example/acme/incidents/b"},\
        {"id":"c","kind":"weather","title":"Odd","impact":"galactic","state":"brewing",\
        "updated_at":"2026-10-01T11:00:00Z","url":"https://status.example/acme/incidents/c"},\
        {"title":"broken"}]}}
        """

    private func incident(_ id: String, _ impact: YuvaStatusImpact, at seconds: TimeInterval) -> YuvaStatusIncident {
        YuvaStatusIncident(
            id: id, title: id, impact: impact, updatedAt: Date(timeIntervalSince1970: seconds), url: "https://x.example/\(id)")
    }

    private func status(_ incidents: [YuvaStatusIncident]) -> YuvaInboxStatus {
        YuvaInboxStatus(url: "https://x.example", open: incidents)
    }

    @MainActor private func dismissals() -> StatusDismissals {
        let suite = "yuva.test.\(UUID().uuidString)"
        return StatusDismissals(defaults: UserDefaults(suiteName: suite)!, key: "d")
    }

    @Test func decodesTheStatusAndFallsBackOnUnknownValues() throws {
        let inbox = try YuvaJSON.decoder.decode(YuvaInbox.self, from: Data(Self.inbox.utf8))

        let open = try #require(inbox.status?.open)
        #expect(inbox.status?.url == "https://status.example/acme")
        #expect(open.map(\.id) == ["a", "b", "c"])
        #expect(open[0].impact == .major)
        #expect(open[0].state == .identified)
        #expect(open[1].kind == .maintenance)
        #expect(open[1].state == .inProgress)
        #expect(open[2].kind == .incident)
        #expect(open[2].impact == .none)
        #expect(open[2].state == .unknown)
    }

    @Test func anInboxWithoutAStatusDecodes() throws {
        let json = """
            {"id":"i1","name":"Acme","branding":{},"default_locale":"en","timezone":"UTC","mode":"live",\
            "open_now":true,"feedback_categories":[]}
            """
        #expect(try YuvaJSON.decoder.decode(YuvaInbox.self, from: Data(json.utf8)).status == nil)
    }

    @Test @MainActor func theWorstImpactLeadsAndTheNewestBreaksTies() throws {
        let store = dismissals()
        let notice = try #require(
            store.notice(
                for: status([incident("old", .major, at: 1), incident("minor", .minor, at: 9), incident("new", .major, at: 5)])))

        #expect(notice.incidents.map(\.id) == ["new", "old", "minor"])
        #expect(notice.lead.id == "new")
        #expect(notice.more == 2)
    }

    @Test @MainActor func aDismissedIncidentStaysHiddenUntilItIsUpdated() throws {
        let store = dismissals()
        let first = status([incident("a", .major, at: 1)])
        store.dismiss(try #require(store.notice(for: first)))

        #expect(store.notice(for: first) == nil)
        #expect(store.notice(for: status([incident("a", .major, at: 2)]))?.lead.id == "a")
        #expect(store.notice(for: status([incident("a", .major, at: 1), incident("b", .minor, at: 1)]))?.incidents.map(\.id) == ["b"])
    }

    @Test @MainActor func dismissalsSurviveARestartAndAreDroppedOnceResolved() throws {
        let suite = "yuva.test.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        let open = status([incident("a", .major, at: 1)])
        let store = StatusDismissals(defaults: defaults, key: "d")
        store.dismiss(try #require(store.notice(for: open)))

        let restarted = StatusDismissals(defaults: defaults, key: "d")
        #expect(restarted.notice(for: open) == nil)

        restarted.prune(keeping: nil)
        #expect(restarted.notice(for: open) == nil)

        restarted.prune(keeping: status([]))
        #expect(restarted.entries.isEmpty)
        #expect(StatusDismissals(defaults: defaults, key: "d").entries.isEmpty)
        #expect(restarted.notice(for: open)?.lead.id == "a")
    }

    @Test @MainActor func nothingOpenMeansNoLine() {
        let store = dismissals()
        #expect(store.notice(for: nil) == nil)
        #expect(store.notice(for: status([])) == nil)
    }
}

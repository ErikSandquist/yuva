import Foundation
import Testing
@testable import YuvaKit

@Suite struct SubscriptionTests {
    @Test func listsEveryPublicListWithTheContactsState() async throws {
        let client = makeClient(FakeServer(), store: MemoryStore(), identity: { identityToken })

        let subscriptions = try await client.subscriptions()

        #expect(subscriptions.map(\.id) == ["news", "double"])
        #expect(subscriptions[0].state == .subscribed)
        #expect(subscriptions[0].list.description == "Monthly")
        #expect(subscriptions[1].state == nil)
        #expect(subscriptions[1].list.doubleOptIn)
    }

    @Test func subscribesAtOnceOrWaitsForTheConfirmationMail() async throws {
        let server = FakeServer()
        let client = makeClient(server, store: MemoryStore(), identity: { identityToken })

        #expect(try await client.subscribe(listId: "news", consentText: "Send me the newsletter") == .subscribed)
        #expect(try await client.subscribe(listId: "double", consentText: "Send me the beta news") == .pending)

        let puts = server.requests.filter { $0.method == "PUT" }
        #expect(puts.map(\.path) == ["/client/v1/subscriptions/news", "/client/v1/subscriptions/double"])
        #expect(puts.first?.body["consent_text"] == "Send me the newsletter")
        #expect(puts.allSatisfy { $0.bearer == "t1" })
    }

    @Test func anAnonymousSessionGetsIdentityRequired() async {
        let client = makeClient(FakeServer(), store: MemoryStore())

        await #expect {
            try await client.subscribe(listId: "news", consentText: "Send me the newsletter")
        } throws: { error in
            (error as? YuvaError)?.status == 403 && (error as? YuvaError)?.code == "identity_required"
        }
    }

    @Test func unsubscribesWithADelete() async throws {
        let server = FakeServer()
        let client = makeClient(server, store: MemoryStore(), identity: { identityToken })

        try await client.unsubscribe(listId: "news")

        #expect(server.requests.last?.method == "DELETE")
        #expect(server.requests.last?.path == "/client/v1/subscriptions/news")
        #expect(server.requests.last?.bearer == "t1")
    }
}

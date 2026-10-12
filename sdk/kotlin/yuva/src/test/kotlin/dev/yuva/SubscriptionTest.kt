package dev.yuva

import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test

class SubscriptionTest {
    @Test
    fun listsEveryPublicListWithTheContactsState() = runBlocking {
        val subscriptions = client(FakeServer(), MemoryStore()) { IDENTITY_TOKEN }.subscriptions()

        assertEquals(listOf("news", "double"), subscriptions.map { it.list.id })
        assertEquals(YuvaSubscriptionState.SUBSCRIBED, subscriptions[0].state)
        assertEquals("Monthly", subscriptions[0].list.description)
        assertNull(subscriptions[1].state)
        assertTrue(subscriptions[1].list.doubleOptIn)
    }

    @Test
    fun subscribesAtOnceOrWaitsForTheConfirmationMail() = runBlocking {
        val server = FakeServer()
        val yuva = client(server, MemoryStore()) { IDENTITY_TOKEN }

        assertEquals(YuvaSubscriptionState.SUBSCRIBED, yuva.subscribe("news", "Send me the newsletter"))
        assertEquals(YuvaSubscriptionState.PENDING, yuva.subscribe("double", "Send me the beta news"))

        val puts = server.requests.filter { it.method == "PUT" }
        assertEquals(listOf("/client/v1/subscriptions/news", "/client/v1/subscriptions/double"), puts.map { it.path })
        assertEquals("Send me the newsletter", puts.first().consentText)
        assertTrue(puts.all { it.bearer == "t1" })
    }

    @Test
    fun anAnonymousSessionGetsIdentityRequired() = runBlocking {
        try {
            client(FakeServer(), MemoryStore()).subscribe("news", "Send me the newsletter")
            fail("expected identity_required")
        } catch (error: YuvaException) {
            assertEquals(403, error.status)
            assertEquals("identity_required", error.code)
        }
    }

    @Test
    fun unsubscribesWithADelete() = runBlocking {
        val server = FakeServer()

        client(server, MemoryStore()) { IDENTITY_TOKEN }.unsubscribe("news")

        val last = server.requests.last()
        assertEquals("DELETE", last.method)
        assertEquals("/client/v1/subscriptions/news", last.path)
        assertEquals("t1", last.bearer)
    }
}

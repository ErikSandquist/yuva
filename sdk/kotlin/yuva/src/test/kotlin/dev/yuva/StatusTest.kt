package dev.yuva

import java.time.Instant
import kotlinx.serialization.decodeFromString
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class StatusTest {
    private val inboxJson = """
        {"id":"i1","name":"Acme","branding":{},"default_locale":"en","timezone":"UTC","mode":"live",
        "open_now":true,"feedback_categories":[],
        "status":{"url":"https://status.example/acme","state":"partial_outage","open":[
        {"id":"a","kind":"incident","title":"API errors","impact":"major","state":"identified",
        "updated_at":"2026-10-01T10:00:00Z","url":"https://status.example/acme/incidents/a"},
        {"id":"b","kind":"maintenance","title":"Database upgrade","impact":"none","state":"in_progress",
        "updated_at":"2026-10-01T09:00:00.250Z","url":"https://status.example/acme/incidents/b"},
        {"id":"c","kind":"weather","title":"Odd","impact":"galactic","state":"brewing",
        "updated_at":"2026-10-01T11:00:00Z","url":"https://status.example/acme/incidents/c"},
        {"title":"broken"}]}}
    """.trimIndent()

    private fun incident(id: String, impact: YuvaStatusImpact, at: Long) =
        YuvaStatusIncident(id = id, title = id, impact = impact, updatedAt = Instant.ofEpochSecond(at), url = "https://x.example/$id")

    private fun status(vararg incidents: YuvaStatusIncident) = YuvaInboxStatus("https://x.example", incidents.toList())

    private fun dismissals(saved: MutableList<Set<String>> = mutableListOf(), initial: Set<String> = emptySet()) =
        StatusDismissals({ saved.add(it) }, initial)

    @Test
    fun decodesTheStatusAndFallsBackOnUnknownValues() {
        val inbox = YuvaClient.json.decodeFromString<YuvaInbox>(inboxJson)

        val status = inbox.status!!
        assertEquals("https://status.example/acme", status.url)
        assertEquals(listOf("a", "b", "c"), status.open.map { it.id })
        assertEquals(YuvaStatusImpact.MAJOR, status.open[0].impact)
        assertEquals(YuvaStatusState.IDENTIFIED, status.open[0].state)
        assertEquals(YuvaStatusKind.MAINTENANCE, status.open[1].kind)
        assertEquals(YuvaStatusState.IN_PROGRESS, status.open[1].state)
        assertEquals(YuvaStatusKind.INCIDENT, status.open[2].kind)
        assertEquals(YuvaStatusImpact.NONE, status.open[2].impact)
        assertEquals(YuvaStatusState.UNKNOWN, status.open[2].state)
    }

    @Test
    fun anInboxWithoutAStatusDecodes() {
        val json = """{"id":"i1","name":"Acme","branding":{},"default_locale":"en","timezone":"UTC","mode":"live","open_now":true,"feedback_categories":[]}"""

        assertNull(YuvaClient.json.decodeFromString<YuvaInbox>(json).status)
    }

    @Test
    fun theWorstImpactLeadsAndTheNewestBreaksTies() {
        val notice = dismissals().notice(
            status(incident("old", YuvaStatusImpact.MAJOR, 1), incident("minor", YuvaStatusImpact.MINOR, 9), incident("new", YuvaStatusImpact.MAJOR, 5)),
        )

        assertEquals(listOf("new", "old", "minor"), notice!!.incidents.map { it.id })
        assertEquals("new", notice.lead.id)
        assertEquals(2, notice.more)
    }

    @Test
    fun aDismissedIncidentStaysHiddenUntilItIsUpdated() {
        val store = dismissals()
        val first = status(incident("a", YuvaStatusImpact.MAJOR, 1))
        store.dismiss(store.notice(first)!!)

        assertNull(store.notice(first))
        assertEquals("a", store.notice(status(incident("a", YuvaStatusImpact.MAJOR, 2)))!!.lead.id)
        val both = status(incident("a", YuvaStatusImpact.MAJOR, 1), incident("b", YuvaStatusImpact.MINOR, 1))
        assertEquals(listOf("b"), store.notice(both)!!.incidents.map { it.id })
    }

    @Test
    fun dismissalsAreSavedAndDroppedOnceResolved() {
        val saved = mutableListOf<Set<String>>()
        val open = status(incident("a", YuvaStatusImpact.MAJOR, 1))
        val store = dismissals(saved)
        store.dismiss(store.notice(open)!!)

        assertNull(dismissals(initial = saved.last()).notice(open))

        store.prune(status())
        assertTrue(store.entries.isEmpty())
        assertTrue(saved.last().isEmpty())
        assertEquals("a", store.notice(open)!!.lead.id)
    }

    @Test
    fun nothingOpenMeansNoLine() {
        val store = dismissals()
        assertNull(store.notice(null))
        assertNull(store.notice(status()))
    }
}

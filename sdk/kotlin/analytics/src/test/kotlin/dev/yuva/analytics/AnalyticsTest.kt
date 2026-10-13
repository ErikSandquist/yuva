package dev.yuva.analytics

import java.io.File
import java.io.IOException
import java.nio.file.Files
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.boolean
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

private const val NOW = 1_760_263_200_000L
private val APP = AppInfo("android", "2.3.1", "16", "mobile", 412, "tr-TR")

private class FakeCollector(answers: List<Answer> = emptyList(), private val offline: Boolean = false) : AnalyticsTransport {
    private val queued = answers.toMutableList()
    val bodies = mutableListOf<ByteArray>()
    val languages = mutableListOf<String?>()

    val batches: List<EventBatch>
        get() = synchronized(this) { bodies.map { AnalyticsQueue.json.decodeFromString(EventBatch.serializer(), String(it)) } }

    @Synchronized override fun post(url: String, body: ByteArray, acceptLanguage: String?): Answer {
        bodies += body
        languages += acceptLanguage
        if (offline) throw IOException("offline")
        return if (queued.isEmpty()) Answer(202, null) else queued.removeAt(0)
    }
}

private fun tempFile(): File = File(Files.createTempDirectory("yuva-analytics").toFile(), "queue.json")

private fun CoroutineScope.core(
    collector: FakeCollector,
    key: String = "0f8fad5b-d9cb-469f-a165-70867728950e",
    file: File? = null,
    requireConsent: Boolean = false,
    optedOut: Boolean = false,
    sleeps: MutableList<Long> = mutableListOf(),
) = AnalyticsCore(
    endpoint = "https://yuva.test/client/v1/analytics/yuva_ak_test",
    processKey = key,
    app = APP,
    requireConsent = requireConsent,
    optedOut = optedOut,
    queueFile = file,
    transport = collector,
    scope = this,
    autoFlushMillis = null,
    now = { NOW },
    sleep = { sleeps += it },
)

private fun screen(name: String, at: Long = NOW) =
    AnalyticsCommand.Record(QueuedEvent(n = "screen", u = "app://dev.yuva.sample/$name", t = at))

private suspend fun AnalyticsCore.send() {
    handle(AnalyticsCommand.Flush())
    idle()
}

class AnalyticsTest {
    @Test
    fun queuedEventsKeepTheirTimesAndProcessKeyAcrossALaunch() = runBlocking {
        val file = tempFile()
        val first = core(FakeCollector(offline = true), key = "first-process-key-0001", file = file)
        first.handle(screen("Home", NOW - 3_600_000))
        first.handle(screen("Cart", NOW - 1_800_000))

        val collector = FakeCollector()
        val second = core(collector, key = "second-process-key-0002", file = file)
        second.handle(screen("Home"))
        second.send()

        val batches = collector.batches
        assertEquals(listOf("first-process-key-0001", "second-process-key-0002"), batches.map { it.k })
        assertEquals(listOf(NOW - 3_600_000, NOW - 1_800_000), batches[0].e.map { it.t })
        assertEquals(listOf("app://dev.yuva.sample/Home", "app://dev.yuva.sample/Cart"), batches[0].e.map { it.u })
        assertEquals(listOf(NOW), batches[1].e.map { it.t })
        assertTrue(batches.all { it.a == APP })
        assertTrue(second.pending.isEmpty())
        assertFalse(file.exists())
    }

    @Test
    fun eventsOlderThan72HoursAreDroppedAtLaunch() = runBlocking {
        val file = tempFile()
        val first = core(FakeCollector(offline = true), key = "first-process-key-0001", file = file)
        first.handle(screen("Old", NOW - 73 * 3_600_000L))
        first.handle(screen("Recent", NOW - 71 * 3_600_000L))

        val second = core(FakeCollector(), key = "second-process-key-0002", file = file)
        assertEquals(listOf("app://dev.yuva.sample/Recent"), second.pending.flatMap { it.e }.map { it.u })
    }

    @Test
    fun optOutStopsSendingAndClearsTheQueue() = runBlocking {
        val file = tempFile()
        val collector = FakeCollector()
        val core = core(collector, file = file)
        core.handle(screen("Home"))
        core.handle(screen("Cart"))
        assertTrue(file.exists())

        core.handle(AnalyticsCommand.OptOut)
        assertTrue(core.pending.isEmpty())
        assertFalse(file.exists())

        core.handle(screen("Checkout"))
        core.send()
        assertTrue(collector.bodies.isEmpty())

        core.handle(AnalyticsCommand.OptIn)
        core.handle(screen("Checkout"))
        core.send()
        assertEquals(listOf("app://dev.yuva.sample/Checkout"), collector.batches.flatMap { it.e }.map { it.u })
    }

    @Test
    fun anOptedOutLaunchDeletesTheQueueItFinds() = runBlocking {
        val file = tempFile()
        core(FakeCollector(offline = true), file = file).handle(screen("Home"))

        val second = core(FakeCollector(), file = file, optedOut = true)
        assertTrue(second.pending.isEmpty())
        assertFalse(file.exists())
    }

    @Test
    fun rateLimitedBatchIsRetriedAfterRetryAfter() = runBlocking {
        val collector = FakeCollector(listOf(Answer(429, "7"), Answer(503, "2")))
        val sleeps = mutableListOf<Long>()
        val core = core(collector, sleeps = sleeps)
        core.handle(screen("Home"))
        core.send()

        assertEquals(listOf(7_000L, 2_000L), sleeps)
        assertEquals(3, collector.bodies.size)
        assertTrue(collector.batches.all { it == collector.batches[0] })
        assertTrue(core.pending.isEmpty())
    }

    @Test
    fun refusedBatchIsDropped() = runBlocking {
        val collector = FakeCollector(listOf(Answer(403, null)))
        val core = core(collector)
        core.handle(screen("Home"))
        core.send()

        assertEquals(1, collector.bodies.size)
        assertTrue(core.pending.isEmpty())
    }

    @Test
    fun serverErrorKeepsTheBatchAndBacksOff() = runBlocking {
        val collector = FakeCollector(listOf(Answer(500, null)))
        val sleeps = mutableListOf<Long>()
        val core = core(collector, sleeps = sleeps)
        core.handle(screen("Home"))
        core.send()

        assertEquals(listOf(5_000L), sleeps)
        assertEquals(2, collector.bodies.size)
        assertTrue(core.pending.isEmpty())
    }

    @Test
    fun batchesHoldAtMost100EventsAnd64KiB() = runBlocking {
        val collector = FakeCollector()
        val core = core(collector)
        repeat(250) { core.handle(screen("Screen$it", NOW - 250 + it)) }
        core.send()
        assertEquals(listOf(100, 100, 50), collector.batches.map { it.e.size })

        val large = FakeCollector()
        val heavy = core(large)
        val text = "x".repeat(1_900)
        repeat(90) {
            val props = mapOf("a" to JsonPrimitive(text), "b" to JsonPrimitive(text), "c" to JsonPrimitive(text))
            heavy.handle(AnalyticsCommand.Record(QueuedEvent(n = "Viewed", u = "app://dev.yuva.sample/Item", t = NOW - it, p = props)))
        }
        heavy.send()
        assertTrue(large.bodies.all { it.size <= 64 * 1024 })
        assertTrue(large.bodies.size > 1)
        assertEquals(90, large.batches.sumOf { it.e.size })
    }

    @Test
    fun consentPropertySendsNothingUntilConsentAndMarksEvents() = runBlocking {
        val collector = FakeCollector()
        val core = core(collector, requireConsent = true)
        core.handle(screen("Before"))
        core.handle(AnalyticsCommand.Consent(true))
        core.handle(screen("After"))
        core.send()

        val events = collector.batches.flatMap { it.e }
        assertEquals(listOf("app://dev.yuva.sample/After"), events.map { it.u })
        assertEquals(ConsentBody(1), events.first().c)
    }

    @Test
    fun customEventsCarryPropsRevenueAndInteractivity() = runBlocking {
        val collector = FakeCollector()
        val core = core(collector)
        core.handle(
            AnalyticsCommand.Record(
                QueuedEvent(
                    n = "Purchase",
                    u = "app://dev.yuva.sample/Cart",
                    t = NOW,
                    p = mapOf("plan" to JsonPrimitive("pro"), "seats" to JsonPrimitive(3)),
                    revenue = RevenueBody("9.99", "EUR"),
                    i = false,
                ),
            ),
        )
        core.send()

        val body = AnalyticsQueue.json.parseToJsonElement(String(collector.bodies.single())).jsonObject
        assertEquals("0f8fad5b-d9cb-469f-a165-70867728950e", body["k"]!!.jsonPrimitive.content)
        assertEquals("android", body["a"]!!.jsonObject["platform"]!!.jsonPrimitive.content)
        assertEquals("16", body["a"]!!.jsonObject["os_version"]!!.jsonPrimitive.content)
        val event = body["e"]!!.jsonArray.single().jsonObject
        assertEquals("Purchase", event["n"]!!.jsonPrimitive.content)
        assertFalse(event["i"]!!.jsonPrimitive.boolean)
        assertEquals("9.99", event["$"]!!.jsonObject["a"]!!.jsonPrimitive.content)
        assertEquals("EUR", event["$"]!!.jsonObject["c"]!!.jsonPrimitive.content)
        assertEquals(3, event["p"]!!.jsonObject["seats"]!!.jsonPrimitive.int)
        assertFalse(event.containsKey("c"))
        assertEquals(listOf<String?>(null), collector.languages)
    }

    @Test
    fun screenNamesArePercentEncoded() {
        assertEquals("app://com.example.shop/Product%20Detail", YuvaAnalytics.screenUrl("com.example.shop", "Product Detail"))
        assertEquals("app://com.example.shop/Settings/Profile", YuvaAnalytics.screenUrl("com.example.shop", "/Settings/Profile"))
        assertEquals("app://com.example.shop/%C3%9Cr%C3%BCn%3F%23", YuvaAnalytics.screenUrl("com.example.shop", "Ürün?#"))
    }
}

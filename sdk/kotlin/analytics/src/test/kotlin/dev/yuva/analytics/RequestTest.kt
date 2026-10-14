package dev.yuva.analytics

import java.io.File
import java.io.IOException
import java.nio.file.Files
import java.time.Instant
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
import okhttp3.Interceptor
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Protocol
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

private const val REQUEST_NOW = 1_760_263_200_000L
private val REQUEST_APP = AppInfo("android", "2.3.1", "16", "mobile", 412, "tr-TR")
private const val API_HOST = "api.shop.test"

private class RequestCollector(answers: List<Answer> = emptyList(), private val offline: Boolean = false) : AnalyticsTransport {
    private val queued = answers.toMutableList()
    val bodies = mutableListOf<ByteArray>()
    val urls = mutableListOf<String>()

    val batches: List<RequestBatch>
        get() = synchronized(this) { bodies.map { AnalyticsQueue.json.decodeFromString(RequestBatch.serializer(), String(it)) } }

    @Synchronized override fun post(url: String, body: ByteArray, acceptLanguage: String?): Answer {
        bodies += body
        urls += url
        if (offline) throw IOException("offline")
        return if (queued.isEmpty()) Answer(202, null) else queued.removeAt(0)
    }
}

private fun matcher(
    routes: List<String> = listOf("/users/{id}", "/users/me", "/orders/{id}/items/{item}", "/health"),
    sendPaths: Boolean = false,
) = RequestMatcher(listOf("API.shop.test"), routes, sendPaths, apiVersion = "v2", environment = "staging")

private fun RequestMatcher.at(path: String, host: String = API_HOST, custom: String? = null) = target(host, path) { custom }

private fun CoroutineScope.requests(
    collector: RequestCollector,
    file: File? = null,
    requireConsent: Boolean = false,
    optedOut: Boolean = false,
    sleeps: MutableList<Long> = mutableListOf(),
) = RequestCore(
    endpoint = "https://yuva.test/client/v1/analytics/yuva_ak_api/requests",
    app = REQUEST_APP,
    requireConsent = requireConsent,
    optedOut = optedOut,
    queueFile = file,
    transport = collector,
    scope = this,
    autoFlushMillis = null,
    now = { REQUEST_NOW },
    sleep = { sleeps += it },
)

private fun call(path: String = "/users/42", status: Int = 200, at: Long = REQUEST_NOW): AnalyticsCommand {
    val m = matcher(sendPaths = true)
    return AnalyticsCommand.Request(m.record("GET", m.at(path)!!, status, at, 12_500_000, 0, 512))
}

private suspend fun RequestCore.send() {
    handle(AnalyticsCommand.Flush())
    idle()
}

private fun RequestCore.queued(): Int {
    var count = -1
    handle(AnalyticsCommand.QueuedRequests { count = it })
    return count
}

private fun schema(name: String): Pair<Set<String>, Set<String>> {
    val lines = File("../../../openapi/openapi.yaml").readLines()
    val start = lines.indexOf("    $name:")
    assertTrue("$name not in openapi.yaml", start >= 0)
    val properties = mutableSetOf<String>()
    var required = emptySet<String>()
    var inProperties = false
    for (line in lines.drop(start + 1)) {
        val text = line.trim()
        if (text.isEmpty()) continue
        val indent = line.takeWhile { it == ' ' }.length
        if (indent <= 4) break
        if (indent == 6) {
            inProperties = text == "properties:"
            if (text.startsWith("required: [")) required = text.removePrefix("required: [").removeSuffix("]").split(",").map { it.trim() }.toSet()
        } else if (indent == 8 && inProperties && text.endsWith(":")) {
            properties += text.removeSuffix(":")
        }
    }
    return properties to required
}

class RequestTest {
    @Test
    fun routesComeFromTheHostsPatternsWithTheMostSpecificWinning() {
        val m = matcher()
        assertEquals(RequestMatcher.Target.Route("/users/{id}"), m.at("/users/42"))
        assertEquals(RequestMatcher.Target.Route("/users/me"), m.at("/users/me"))
        assertEquals(RequestMatcher.Target.Route("/users/{id}"), m.at("/users/42/"))
        assertEquals(RequestMatcher.Target.Route("/orders/{id}/items/{item}"), m.at("/orders/9/items/3"))
        assertEquals(RequestMatcher.Target.Route("/health"), m.at("/health"))
    }

    @Test
    fun unmatchedRequestsAndOtherHostsAreSkipped() {
        val m = matcher()
        assertNull(m.at("/users"))
        assertNull(m.at("/users/42/orders"))
        assertNull(m.at("/users/42", host = "cdn.shop.test"))
        assertNull(m.at("/users/42", host = "api.shop.test.evil.example"))
    }

    @Test
    fun unmatchedPathsAreSentOnlyWhenTheHostAsks() {
        val m = matcher(sendPaths = true)
        assertEquals(RequestMatcher.Target.Path("/search/abc"), m.at("/search/abc"))
        assertEquals(RequestMatcher.Target.Path("/"), m.at(""))
        assertEquals(RequestMatcher.Target.Route("/users/{id}"), m.at("/users/42"))
        assertNull(m.at("/x", host = "other.test"))
    }

    @Test
    fun theHostsRouteComesFirstAndFallsBackToPatterns() {
        val m = matcher()
        assertEquals(RequestMatcher.Target.Route("/v1/carts/{cart}"), m.at("/v1/carts/abc", custom = "/v1/carts/{cart}"))
        assertEquals(RequestMatcher.Target.Route("/users/{id}"), m.at("/users/42", custom = ""))
        var asked = false
        m.target("cdn.shop.test", "/x") { "/x".also { asked = true } }
        assertFalse(asked)
    }

    @Test
    fun theInterceptorTimesCallsWithoutQueryHeadersOrBodies() {
        val records = mutableListOf<RequestRecord>()
        val monitor = RequestMonitor(matcher(), { if (it.url.encodedPath.startsWith("/v1/carts/")) "/v1/carts/{cart}" else null }) { records += it }
        var tick = 0L
        val answers = mutableMapOf("/users/7" to 404, "/v1/carts/abc" to 201)
        val client = OkHttpClient.Builder()
            .addInterceptor(YuvaRequestInterceptor({ monitor }, { REQUEST_NOW }, { tick.also { tick += 3_000_000 } }))
            .addInterceptor(
                Interceptor { chain ->
                    val path = chain.request().url.encodedPath
                    if (chain.call().isCanceled()) throw IOException("Canceled")
                    if (path == "/users/down") throw IOException("offline")
                    Response.Builder()
                        .request(chain.request())
                        .protocol(Protocol.HTTP_1_1)
                        .code(answers[path] ?: 200)
                        .message("")
                        .body("{\"ok\":true}".toResponseBody("application/json".toMediaType()))
                        .build()
                },
            )
            .build()

        client.newCall(Request.Builder().url("https://$API_HOST/users/7?token=secret").header("Authorization", "Bearer x").build())
            .execute().use { it.body.string() }
        client.newCall(
            Request.Builder().url("https://$API_HOST/v1/carts/abc").post("{\"sku\":1}".toRequestBody("application/json".toMediaType())).build(),
        ).execute().close()
        client.newCall(Request.Builder().url("https://$API_HOST/unmapped/1").build()).execute().close()
        client.newCall(Request.Builder().url("https://cdn.shop.test/users/7").build()).execute().close()
        runCatching { client.newCall(Request.Builder().url("https://$API_HOST/users/down").build()).execute() }
        val canceled = client.newCall(Request.Builder().url("https://$API_HOST/health").build())
        canceled.cancel()
        runCatching { canceled.execute() }

        assertEquals(
            listOf(
                RequestRecord("GET", route = "/users/{id}", status = 404, durationUs = 3_000, responseSize = 11, time = "2025-10-12T10:00:00Z", apiVersion = "v2", environment = "staging"),
                RequestRecord("POST", route = "/v1/carts/{cart}", status = 201, durationUs = 3_000, requestSize = 9, responseSize = 11, time = "2025-10-12T10:00:00Z", apiVersion = "v2", environment = "staging"),
            ),
            records.take(2),
        )
        assertEquals(3, records.size)
        assertEquals(0, records[2].status)
        assertNull(records[2].responseSize)
    }

    @Test
    fun requestsAreBatchedAt100AndKept64KiB() = runBlocking {
        val collector = RequestCollector()
        val core = requests(collector)
        repeat(250) { core.handle(call("/users/$it")) }
        assertEquals(250, core.queued())
        core.send()
        assertEquals(listOf(100, 100, 50), collector.batches.map { it.records.size })
        assertTrue(collector.bodies.all { it.size <= 64 * 1024 })
        assertTrue(collector.batches.all { it.a == REQUEST_APP })
        assertEquals(listOf("https://yuva.test/client/v1/analytics/yuva_ak_api/requests"), collector.urls.distinct())
        assertTrue(core.pending.isEmpty())

        val large = RequestCollector()
        val heavy = requests(large)
        val long = "/" + "p".repeat(1_990)
        repeat(60) { heavy.handle(call(long)) }
        heavy.send()
        assertTrue(large.bodies.size > 1)
        assertTrue(large.bodies.all { it.size <= 64 * 1024 })
        assertEquals(60, large.batches.sumOf { it.records.size })
    }

    @Test
    fun aConsentPropertySendsNoRequestBeforeConsentAndMarksEachAfter() = runBlocking {
        val collector = RequestCollector()
        val core = requests(collector, requireConsent = true)
        core.handle(call("/users/1"))
        assertTrue(core.pending.isEmpty())
        core.handle(AnalyticsCommand.Consent(true, false))
        core.handle(call("/users/2"))
        core.handle(call("/users/3"))
        core.handle(AnalyticsCommand.Consent(false, false))
        assertTrue(core.pending.isEmpty())
        core.handle(call("/users/4"))
        core.handle(AnalyticsCommand.Consent(true, false))
        core.handle(call("/users/5"))
        core.send()

        val records = collector.batches.flatMap { it.records }
        assertEquals(1, records.size)
        assertEquals(ConsentBody(1), records[0].c)
    }

    @Test
    fun withoutAConsentModeRequestsCarryNoConsentBlock() = runBlocking {
        val collector = RequestCollector()
        val core = requests(collector)
        core.handle(call())
        core.send()
        assertEquals(listOf(null), collector.batches.flatMap { it.records }.map { it.c })
    }

    @Test
    fun optOutClearsTheRequestQueueAndStopsRecording() = runBlocking {
        val file = File(Files.createTempDirectory("yuva-requests").toFile(), "requests.json")
        val collector = RequestCollector()
        val core = requests(collector, file = file)
        core.handle(call("/users/1"))
        assertTrue(file.exists())
        core.handle(AnalyticsCommand.OptOut)
        assertTrue(core.pending.isEmpty())
        assertFalse(file.exists())
        core.handle(call("/users/2"))
        core.send()
        assertTrue(collector.bodies.isEmpty())

        core.handle(AnalyticsCommand.OptIn)
        core.handle(call("/users/3"))
        core.send()
        assertEquals(listOf("/users/{id}"), collector.batches.flatMap { it.records }.map { it.route })

        core.handle(call("/users/4"))
        assertTrue(requests(RequestCollector(), file = file, optedOut = true).pending.isEmpty())
        assertFalse(file.exists())
    }

    @Test
    fun queuedRequestsSurviveARelaunchAndExpireAfter72Hours() = runBlocking {
        val file = File(Files.createTempDirectory("yuva-requests").toFile(), "requests.json")
        val first = requests(RequestCollector(offline = true), file = file)
        first.handle(call("/users/1", at = REQUEST_NOW - 73 * 3_600_000L))
        first.handle(call("/users/2", at = REQUEST_NOW - 71 * 3_600_000L))

        val collector = RequestCollector()
        val second = requests(collector, file = file)
        assertEquals(1, second.pending.sumOf { it.records.size })
        second.send()
        assertEquals(
            listOf(RequestRecord.timestamp(REQUEST_NOW - 71 * 3_600_000L)),
            collector.batches.flatMap { it.records }.map { it.time },
        )
        assertFalse(file.exists())
    }

    @Test
    fun rateLimitedAndFailedBatchesAreRetriedAndRefusedOnesDropped() = runBlocking {
        val collector = RequestCollector(listOf(Answer(429, "7"), Answer(500, null), Answer(202, null), Answer(403, null)))
        val sleeps = mutableListOf<Long>()
        val core = requests(collector, sleeps = sleeps)
        core.handle(call())
        core.send()
        assertEquals(listOf(7_000L, 5_000L), sleeps)
        assertEquals(3, collector.bodies.size)
        core.handle(call())
        core.send()
        assertEquals(4, collector.bodies.size)
        assertTrue(core.pending.isEmpty())
    }

    @Test
    fun theWireFormatMatchesTheOpenApiSchema() = runBlocking {
        val collector = RequestCollector()
        val core = requests(collector, requireConsent = true)
        core.handle(AnalyticsCommand.Consent(true, false))
        core.handle(call("/users/42", status = 0))
        core.handle(call("/nowhere/1"))
        core.send()

        val json = AnalyticsQueue.json.parseToJsonElement(String(collector.bodies.single())).jsonObject
        val (batchProperties, batchRequired) = schema("AnalyticsAppRequests")
        assertEquals(batchProperties, json.keys)
        assertTrue(json.keys.containsAll(batchRequired))
        val a = json.getValue("a").jsonObject
        assertTrue(schema("AnalyticsApp").first.containsAll(a.keys))
        assertEquals("android", a.getValue("platform").jsonPrimitive.content)
        assertEquals("2.3.1", a.getValue("version").jsonPrimitive.content)

        val (recordProperties, recordRequired) = schema("AnalyticsAppRequestRecord")
        val records = json.getValue("records").jsonArray.map { it.jsonObject }
        assertEquals(2, records.size)
        for (record in records) {
            assertTrue(record.keys.toString(), recordProperties.containsAll(record.keys))
            assertTrue(record.keys.containsAll(recordRequired))
            assertTrue(("route" in record) != ("path" in record))
            assertEquals(12_500L, record.getValue("duration_us").jsonPrimitive.long)
            assertNotNull(Instant.parse(record.getValue("time").jsonPrimitive.content))
            assertEquals(1, (record.getValue("c") as JsonObject).getValue("a").jsonPrimitive.int)
        }
        assertEquals(0, records[0].getValue("status").jsonPrimitive.int)
        assertEquals("/users/{id}", records[0].getValue("route").jsonPrimitive.content)
        assertEquals("/nowhere/1", records[1].getValue("path").jsonPrimitive.content)
    }
}

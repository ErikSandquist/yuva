package dev.yuva.analytics

import java.io.File
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

internal class RequestMatcher(
    hosts: List<String>,
    routes: List<String>,
    private val sendPaths: Boolean,
    private val apiVersion: String? = null,
    private val environment: String? = null,
) {
    sealed interface Target {
        data class Route(val route: String) : Target
        data class Path(val path: String) : Target
    }

    private class Pattern(text: String) {
        val text = if (text.startsWith("/")) text else "/$text"
        val segments = text.split('/').filter { it.isNotEmpty() }
        val literals = segments.count { !isParameter(it) }

        fun matches(parts: List<String>) =
            parts.size == segments.size && segments.indices.all { isParameter(segments[it]) || segments[it] == parts[it] }
    }

    private val hosts = hosts.map { it.lowercase() }.toSet()
    private val patterns = routes.map(::Pattern)

    fun target(host: String, encodedPath: String, custom: () -> String?): Target? {
        if (host.lowercase() !in hosts) return null
        custom()?.takeIf { it.isNotEmpty() && it.length <= 300 }?.let { return Target.Route(it) }
        match(encodedPath)?.let { return Target.Route(it) }
        if (!sendPaths) return null
        val path = encodedPath.ifEmpty { "/" }
        return if (path.length <= 2000) Target.Path(path) else null
    }

    fun match(encodedPath: String): String? {
        val parts = encodedPath.split('/').filter { it.isNotEmpty() }
        var best: Pattern? = null
        for (pattern in patterns) {
            if (!pattern.matches(parts)) continue
            if (best != null && best.literals >= pattern.literals) continue
            best = pattern
        }
        return best?.text
    }

    fun record(
        method: String,
        target: Target,
        status: Int,
        startedAt: Long,
        durationNanos: Long,
        requestSize: Long?,
        responseSize: Long?,
    ) = RequestRecord(
        method = method.uppercase(),
        route = (target as? Target.Route)?.route,
        path = (target as? Target.Path)?.path,
        status = status,
        durationUs = maxOf(0, (durationNanos + 500) / 1000),
        requestSize = requestSize,
        responseSize = responseSize,
        time = RequestRecord.timestamp(startedAt),
        apiVersion = apiVersion,
        environment = environment,
    )

    companion object {
        fun isParameter(segment: String) = segment.length > 2 && segment.startsWith("{") && segment.endsWith("}")
    }
}

internal class RequestCore(
    val endpoint: String,
    private var app: AppInfo,
    private val requireConsent: Boolean,
    optedOut: Boolean,
    queueFile: File?,
    private val transport: AnalyticsTransport,
    private val scope: CoroutineScope,
    private val autoFlushMillis: Long? = 5_000,
    private val now: () -> Long = System::currentTimeMillis,
    private val sleep: suspend (Long) -> Unit = { delay(it) },
    private val log: ((String) -> Unit)? = null,
) {
    private val queue = RequestQueue(queueFile)
    private var optedOut = optedOut
    private var consented = !requireConsent
    private var generation = 0
    private var failures = 0
    private var sender: Job? = null
    private var senderWaiting = false

    init {
        if (optedOut) queue.clear()
        queue.prune(now())
        queue.save()
    }

    val pending: List<RequestBatch> get() = queue.pending

    fun start() {
        if (queue.count > 0 && !optedOut) schedule(0)
    }

    fun handle(command: AnalyticsCommand) {
        when (command) {
            is AnalyticsCommand.Request -> record(command.record)
            is AnalyticsCommand.Consent -> {
                consented = command.granted
                if (!command.granted) clear()
            }
            AnalyticsCommand.OptOut -> {
                optedOut = true
                clear()
            }
            AnalyticsCommand.OptIn -> optedOut = false
            is AnalyticsCommand.Flush -> {
                if (!optedOut) schedule(0)
                command.done?.let { done ->
                    scope.launch {
                        idle()
                        done()
                    }
                }
            }
            is AnalyticsCommand.Width -> {
                app = app.copy(width = command.width)
                queue.updateApp { if (it.width == null) it.copy(width = command.width) else it }
            }
            is AnalyticsCommand.QueuedRequests -> command.reply(queue.count)
            is AnalyticsCommand.Record, is AnalyticsCommand.Identify -> Unit
        }
    }

    suspend fun idle() {
        while (true) {
            val current = sender ?: return
            current.join()
            if (sender === current || sender?.isActive != true) return
        }
    }

    private fun record(record: RequestRecord) {
        val name = "${record.method} ${record.route ?: record.path}"
        if (optedOut || !consented) {
            log?.invoke("dropped request $name: ${if (optedOut) "opted out" else "no consent"}")
            return
        }
        val stamped = if (requireConsent) record.copy(c = ConsentBody(1)) else record
        if (!queue.append(stamped, app)) return
        queue.save()
        log?.invoke("queued request $name ${record.status} in ${record.durationUs / 1000} ms")
        autoFlushMillis?.let(::schedule)
    }

    private fun clear() {
        generation++
        queue.clear()
        queue.save()
        sender?.cancel()
        sender = null
    }

    private fun schedule(delayMillis: Long) {
        if (sender?.isActive == true && !(senderWaiting && delayMillis == 0L)) return
        sender?.cancel()
        senderWaiting = delayMillis > 0
        sender = scope.launch {
            if (delayMillis > 0) sleep(delayMillis)
            senderWaiting = false
            drain()
        }
    }

    private suspend fun drain() {
        while (!optedOut && currentCoroutineContext().isActive) {
            val wait = sendNext() ?: return
            if (wait > 0) {
                log?.invoke("retrying requests in ${wait / 1000} s")
                sleep(wait)
            }
        }
    }

    suspend fun sendNext(): Long? {
        queue.prune(now())
        val batch = queue.next() ?: run {
            queue.save()
            return null
        }
        val started = generation
        val answer = try {
            withContext(Dispatchers.IO) { transport.post(endpoint, batch.body, null) }
        } catch (error: CancellationException) {
            throw error
        } catch (error: Exception) {
            if (started != generation) return 0
            log?.invoke("sending requests failed: ${error.message}")
            return backoff()
        }
        if (started != generation) return 0
        log?.invoke("sent ${batch.count} requests: ${answer.status}")
        return when (answer.status) {
            429, 503 -> AnalyticsCore.retryDelay(answer.retryAfter)
            in 200..299, in 400..499 -> {
                failures = 0
                queue.remove(batch)
                queue.save()
                0
            }
            else -> backoff()
        }
    }

    private fun backoff(): Long {
        failures++
        return minOf(300_000L, 5_000L shl minOf(failures - 1, 6))
    }
}

package dev.yuva.analytics

import java.io.File
import java.net.HttpURLConnection
import java.net.URL
import java.util.Locale
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

internal sealed interface AnalyticsCommand {
    data class Record(val event: QueuedEvent) : AnalyticsCommand
    data class Consent(val granted: Boolean, val persistent: Boolean) : AnalyticsCommand
    data class Identify(val token: String) : AnalyticsCommand
    data object OptOut : AnalyticsCommand
    data object OptIn : AnalyticsCommand
    class Flush(val done: (() -> Unit)? = null) : AnalyticsCommand
    data class Width(val width: Int) : AnalyticsCommand
}

internal class Answer(val status: Int, val retryAfter: String?)

internal fun interface AnalyticsTransport {
    fun post(url: String, body: ByteArray, acceptLanguage: String?): Answer
}

internal object HttpTransport : AnalyticsTransport {
    override fun post(url: String, body: ByteArray, acceptLanguage: String?): Answer {
        val connection = URL(url).openConnection() as HttpURLConnection
        try {
            connection.requestMethod = "POST"
            connection.doOutput = true
            connection.connectTimeout = 15_000
            connection.readTimeout = 15_000
            connection.useCaches = false
            connection.setRequestProperty("Content-Type", "application/json")
            acceptLanguage?.let { connection.setRequestProperty("Accept-Language", it) }
            connection.setFixedLengthStreamingMode(body.size)
            connection.outputStream.use { it.write(body) }
            val status = connection.responseCode
            runCatching { (if (status >= 400) connection.errorStream else connection.inputStream)?.use { it.readBytes() } }
            return Answer(status, connection.getHeaderField("Retry-After"))
        } finally {
            connection.disconnect()
        }
    }
}

internal class AnalyticsCore(
    val endpoint: String,
    val processKey: String,
    private var app: AppInfo,
    private val home: String,
    private val requireConsent: Boolean,
    optedOut: Boolean,
    queueFile: File?,
    private val ids: PersistentIdStore,
    private val transport: AnalyticsTransport,
    private val scope: CoroutineScope,
    private val autoFlushMillis: Long? = 5_000,
    private val now: () -> Long = System::currentTimeMillis,
    private val sleep: suspend (Long) -> Unit = { delay(it) },
    private val log: ((String) -> Unit)? = null,
) {
    private val queue = AnalyticsQueue(queueFile)
    private var optedOut = optedOut
    private var consented = !requireConsent
    private var generation = 0
    private var failures = 0
    private var sender: Job? = null
    private var senderWaiting = false
    private var persistentId: PersistentId? = null
    private val posts = mutableListOf<Job>()

    init {
        if (optedOut) queue.clear()
        queue.prune(now())
        queue.save()
        val stored = ids.load()
        if (stored != null && (optedOut || stored.expired(now()))) ids.delete() else persistentId = stored
    }

    val pending: List<EventBatch> get() = queue.pending

    fun start() {
        if (queue.count > 0 && !optedOut) schedule(0)
    }

    fun handle(command: AnalyticsCommand) {
        when (command) {
            is AnalyticsCommand.Record -> record(command.event)
            is AnalyticsCommand.Consent -> {
                consented = command.granted
                if (command.granted && command.persistent && !optedOut) {
                    if (currentId() == null) persistentId = PersistentId.make(now()).also(ids::save)
                } else {
                    withdraw()
                }
                if (!command.granted) clear()
            }
            is AnalyticsCommand.Identify -> {
                val id = currentId()
                if (optedOut || !consented || command.token.isEmpty() || id == null) {
                    log?.invoke("identify ignored: no persistent consent")
                    return
                }
                post(QueuedEvent(n = CONSENT_EVENT, u = home, t = now(), c = ConsentBody(1, id, command.token)))
            }
            AnalyticsCommand.OptOut -> {
                optedOut = true
                withdraw()
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
                queue.updateApp(processKey) { if (it.width == null) it.copy(width = command.width) else it }
            }
        }
    }

    suspend fun idle() {
        while (true) {
            val pending = synchronized(posts) { posts.toList().also { posts.clear() } }
            if (pending.isEmpty()) break
            pending.forEach { it.join() }
        }
        while (true) {
            val current = sender ?: return
            current.join()
            if (sender === current || sender?.isActive != true) return
        }
    }

    private fun record(event: QueuedEvent) {
        if (optedOut || !consented) {
            log?.invoke("dropped ${event.n}: ${if (optedOut) "opted out" else "no consent"}")
            return
        }
        val id = currentId()
        val stamped = if (requireConsent || id != null) event.copy(c = ConsentBody(1, id)) else event
        if (!queue.append(stamped, processKey, app)) {
            log?.invoke("dropped ${event.n}: larger than 64 KiB")
            return
        }
        queue.save()
        log?.invoke("queued ${event.n} ${event.u}")
        autoFlushMillis?.let(::schedule)
    }

    private fun currentId(): String? {
        val value = persistentId ?: return null
        if (!value.expired(now())) return value.id
        return PersistentId.make(now()).also(ids::save).also { persistentId = it }.id
    }

    private fun withdraw() {
        val value = persistentId ?: return
        post(QueuedEvent(n = CONSENT_EVENT, u = home, t = now(), c = ConsentBody(0, value.id)))
        ids.delete()
        persistentId = null
        queue.stripPersistent()
        queue.save()
    }

    private fun post(event: QueuedEvent) {
        val body = AnalyticsQueue.encode(EventBatch(processKey, app, listOf(event)))
        val job = scope.launch {
            try {
                val answer = withContext(Dispatchers.IO) { transport.post(endpoint, body, null) }
                log?.invoke("sent $CONSENT_EVENT: ${answer.status}")
            } catch (error: CancellationException) {
                throw error
            } catch (error: Exception) {
                log?.invoke("dropped $CONSENT_EVENT: ${error.message}")
            }
        }
        synchronized(posts) { posts += job }
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
                log?.invoke("retrying in ${wait / 1000} s")
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
        val language = if (app.language == null) Locale.getDefault().toLanguageTag() else null
        val answer = try {
            withContext(Dispatchers.IO) { transport.post(endpoint, batch.body, language) }
        } catch (error: CancellationException) {
            throw error
        } catch (error: Exception) {
            if (started != generation) return 0
            log?.invoke("send failed: ${error.message}")
            return backoff()
        }
        if (started != generation) return 0
        log?.invoke("sent ${batch.count} events: ${answer.status}")
        return when (answer.status) {
            429, 503 -> retryDelay(answer.retryAfter)
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

    companion object {
        const val CONSENT_EVENT = "yuva_consent"

        fun retryDelay(header: String?): Long {
            val seconds = header?.trim()?.toLongOrNull()
            return if (seconds != null && seconds >= 0) maxOf(1, seconds) * 1000 else 60_000
        }
    }
}

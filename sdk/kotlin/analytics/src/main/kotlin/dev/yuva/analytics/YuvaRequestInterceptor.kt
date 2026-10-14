package dev.yuva.analytics

import java.io.IOException
import java.util.concurrent.atomic.AtomicBoolean
import okhttp3.Interceptor
import okhttp3.MediaType
import okhttp3.Request
import okhttp3.Response
import okhttp3.ResponseBody
import okio.Buffer
import okio.BufferedSource
import okio.ForwardingSource
import okio.buffer

internal class RequestMonitor(
    val matcher: RequestMatcher,
    val route: ((Request) -> String?)?,
    val record: (RequestRecord) -> Unit,
)

class YuvaRequestInterceptor internal constructor(
    private val monitor: () -> RequestMonitor?,
    private val clock: () -> Long,
    private val ticker: () -> Long,
) : Interceptor {
    constructor() : this({ YuvaAnalytics.requestMonitor }, System::currentTimeMillis, System::nanoTime)

    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        val monitor = monitor() ?: return chain.proceed(request)
        val target = monitor.matcher.target(request.url.host, request.url.encodedPath) { monitor.route?.invoke(request) }
            ?: return chain.proceed(request)
        val startedAt = clock()
        val started = ticker()
        val requestSize = request.body?.contentLength()?.takeIf { it >= 0 }
        val done = AtomicBoolean(false)
        fun finish(status: Int, responseSize: Long?) {
            if (!done.compareAndSet(false, true)) return
            monitor.record(
                monitor.matcher.record(request.method, target, status, startedAt, ticker() - started, requestSize, responseSize),
            )
        }
        val response = try {
            chain.proceed(request)
        } catch (error: IOException) {
            if (!chain.call().isCanceled()) finish(0, null)
            throw error
        }
        return response.newBuilder().body(TimedBody(response.body) { finish(response.code, it) }).build()
    }

    private class TimedBody(private val body: ResponseBody, private val done: (Long) -> Unit) : ResponseBody() {
        private var read = 0L

        private fun finish() = done(body.contentLength().takeIf { it >= 0 } ?: read)

        private val source: BufferedSource by lazy {
            object : ForwardingSource(body.source()) {
                override fun read(sink: Buffer, byteCount: Long): Long {
                    val count = try {
                        super.read(sink, byteCount)
                    } catch (error: IOException) {
                        finish()
                        throw error
                    }
                    if (count == -1L) finish() else read += count
                    return count
                }

                override fun close() {
                    super.close()
                    finish()
                }
            }.buffer()
        }

        override fun contentType(): MediaType? = body.contentType()

        override fun contentLength(): Long = body.contentLength()

        override fun source(): BufferedSource = source
    }
}

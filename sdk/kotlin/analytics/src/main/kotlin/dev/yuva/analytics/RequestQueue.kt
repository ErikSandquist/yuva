package dev.yuva.analytics

import java.io.File
import java.time.Instant
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.builtins.ListSerializer

@Serializable
internal data class RequestRecord(
    val method: String,
    val route: String? = null,
    val path: String? = null,
    val status: Int,
    @SerialName("duration_us") val durationUs: Long,
    @SerialName("request_size") val requestSize: Long? = null,
    @SerialName("response_size") val responseSize: Long? = null,
    val time: String,
    @SerialName("api_version") val apiVersion: String? = null,
    val environment: String? = null,
    val c: ConsentBody? = null,
) {
    val millis: Long? get() = runCatching { Instant.parse(time).toEpochMilli() }.getOrNull()

    companion object {
        fun timestamp(millis: Long): String = Instant.ofEpochMilli(millis).toString()
    }
}

@Serializable
internal data class RequestBatch(val a: AppInfo, val records: List<RequestRecord>)

internal class PendingRequests(val count: Int, val body: ByteArray)

internal class RequestQueue(private val file: File?) {
    private val batches = mutableListOf<RequestBatch>()

    init {
        val stored = file
        if (stored != null && stored.isFile) {
            runCatching { AnalyticsQueue.json.decodeFromString(listSerializer, stored.readText()) }.onSuccess { batches += it }
        }
    }

    val pending: List<RequestBatch> get() = batches.toList()

    val count: Int get() = batches.sumOf { it.records.size }

    fun append(record: RequestRecord, app: AppInfo): Boolean {
        val probe = RequestBatch(app, listOf(record))
        if (encode(probe).size > MAX_BYTES) return false
        val last = batches.lastOrNull()
        if (last != null && last.a == app) batches[batches.lastIndex] = last.copy(records = last.records + record) else batches += probe
        while (count > CAPACITY) dropOldest()
        return true
    }

    fun updateApp(change: (AppInfo) -> AppInfo) {
        for (index in batches.indices) batches[index] = batches[index].copy(a = change(batches[index].a))
    }

    fun prune(now: Long) {
        for (index in batches.indices) {
            batches[index] = batches[index].copy(
                records = batches[index].records.filter { record -> record.millis?.let { now - it <= MAX_AGE } ?: false },
            )
        }
        batches.removeAll { it.records.isEmpty() }
    }

    fun next(): PendingRequests? {
        val first = batches.firstOrNull() ?: return null
        var take = minOf(first.records.size, MAX_RECORDS)
        while (true) {
            val body = encode(RequestBatch(first.a, first.records.take(take)))
            if (body.size <= MAX_BYTES || take == 1) return PendingRequests(take, body)
            take = (take + 1) / 2
        }
    }

    fun remove(pending: PendingRequests) {
        val first = batches.firstOrNull() ?: return
        val rest = first.records.drop(pending.count)
        if (rest.isEmpty()) batches.removeAt(0) else batches[0] = first.copy(records = rest)
    }

    fun clear() {
        batches.clear()
    }

    fun save() {
        val file = file ?: return
        if (batches.isEmpty()) {
            file.delete()
            return
        }
        runCatching {
            file.parentFile?.mkdirs()
            val temp = File(file.parentFile, file.name + ".tmp")
            temp.writeText(AnalyticsQueue.json.encodeToString(listSerializer, batches))
            if (!temp.renameTo(file)) {
                file.delete()
                temp.renameTo(file)
            }
        }
    }

    private fun dropOldest() {
        val first = batches.firstOrNull() ?: return
        if (first.records.size <= 1) batches.removeAt(0) else batches[0] = first.copy(records = first.records.drop(1))
    }

    companion object {
        const val MAX_RECORDS = 100
        const val MAX_BYTES = 64 * 1024
        const val MAX_AGE = 72L * 3600 * 1000
        const val CAPACITY = 1000

        private val listSerializer = ListSerializer(RequestBatch.serializer())

        fun encode(batch: RequestBatch): ByteArray =
            AnalyticsQueue.json.encodeToString(RequestBatch.serializer(), batch).toByteArray()
    }
}

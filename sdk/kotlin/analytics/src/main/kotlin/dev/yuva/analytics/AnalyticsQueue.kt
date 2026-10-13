package dev.yuva.analytics

import java.io.File
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonPrimitive

@Serializable
internal data class AppInfo(
    val platform: String,
    val version: String? = null,
    @SerialName("os_version") val osVersion: String? = null,
    val device: String? = null,
    val width: Int? = null,
    val language: String? = null,
)

@Serializable
internal data class RevenueBody(val a: String, val c: String)

@Serializable
internal data class ConsentBody(val a: Int, val p: String? = null, val j: String? = null)

@Serializable
internal data class QueuedEvent(
    val n: String,
    val u: String,
    val t: Long,
    val p: Map<String, JsonPrimitive>? = null,
    @SerialName("$") val revenue: RevenueBody? = null,
    val i: Boolean? = null,
    val c: ConsentBody? = null,
)

@Serializable
internal data class EventBatch(val k: String, val a: AppInfo, val e: List<QueuedEvent>)

internal class PendingBatch(val key: String, val count: Int, val body: ByteArray)

internal class AnalyticsQueue(private val file: File?) {
    private val batches = mutableListOf<EventBatch>()

    init {
        val stored = file
        if (stored != null && stored.isFile) {
            runCatching { json.decodeFromString(listSerializer, stored.readText()) }.onSuccess { batches += it }
        }
    }

    val pending: List<EventBatch> get() = batches.toList()

    val count: Int get() = batches.sumOf { it.e.size }

    fun append(event: QueuedEvent, key: String, app: AppInfo): Boolean {
        val probe = EventBatch(key, app, listOf(event))
        if (encode(probe).size > MAX_BYTES) return false
        val index = batches.indexOfLast { it.k == key }
        if (index >= 0) batches[index] = batches[index].copy(e = batches[index].e + event) else batches += probe
        while (count > CAPACITY) dropOldest()
        return true
    }

    fun updateApp(key: String, change: (AppInfo) -> AppInfo) {
        for (index in batches.indices) {
            if (batches[index].k == key) batches[index] = batches[index].copy(a = change(batches[index].a))
        }
    }

    fun stripPersistent() {
        for (index in batches.indices) {
            batches[index] = batches[index].copy(e = batches[index].e.map { event -> event.copy(c = event.c?.copy(p = null, j = null)) })
        }
    }

    fun prune(now: Long) {
        for (index in batches.indices) {
            batches[index] = batches[index].copy(e = batches[index].e.filter { now - it.t <= MAX_AGE })
        }
        batches.removeAll { it.e.isEmpty() }
    }

    fun next(): PendingBatch? {
        val first = batches.firstOrNull() ?: return null
        var take = minOf(first.e.size, MAX_EVENTS)
        while (true) {
            val body = encode(EventBatch(first.k, first.a, first.e.take(take)))
            if (body.size <= MAX_BYTES || take == 1) return PendingBatch(first.k, take, body)
            take = (take + 1) / 2
        }
    }

    fun remove(pending: PendingBatch) {
        val index = batches.indexOfFirst { it.k == pending.key }
        if (index < 0) return
        val rest = batches[index].e.drop(pending.count)
        if (rest.isEmpty()) batches.removeAt(index) else batches[index] = batches[index].copy(e = rest)
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
            temp.writeText(json.encodeToString(listSerializer, batches))
            if (!temp.renameTo(file)) {
                file.delete()
                temp.renameTo(file)
            }
        }
    }

    private fun dropOldest() {
        val first = batches.firstOrNull() ?: return
        if (first.e.size <= 1) batches.removeAt(0) else batches[0] = first.copy(e = first.e.drop(1))
    }

    companion object {
        const val MAX_EVENTS = 100
        const val MAX_BYTES = 64 * 1024
        const val MAX_AGE = 72L * 3600 * 1000
        const val CAPACITY = 1000

        val json = Json {
            explicitNulls = false
            ignoreUnknownKeys = true
        }
        private val listSerializer = ListSerializer(EventBatch.serializer())

        fun encode(batch: EventBatch): ByteArray = json.encodeToString(EventBatch.serializer(), batch).toByteArray()
    }
}

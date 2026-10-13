package dev.yuva.analytics

import java.time.Instant
import java.time.ZoneOffset
import java.util.UUID

internal data class PersistentId(val id: String, val createdAt: Long) {
    fun expired(now: Long): Boolean =
        Instant.ofEpochMilli(createdAt).atZone(ZoneOffset.UTC).plusMonths(LIFETIME_MONTHS).toInstant().toEpochMilli() <= now

    companion object {
        const val LIFETIME_MONTHS = 13L

        fun make(now: Long) = PersistentId(UUID.randomUUID().toString(), now)
    }
}

internal interface PersistentIdStore {
    fun load(): PersistentId?
    fun save(value: PersistentId)
    fun delete()
}

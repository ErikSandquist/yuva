package dev.yuva

import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue

internal class StatusNotice private constructor(val incidents: List<YuvaStatusIncident>) {
    val lead: YuvaStatusIncident get() = incidents.first()
    val more: Int get() = incidents.size - 1

    override fun equals(other: Any?) = other is StatusNotice && other.incidents == incidents
    override fun hashCode() = incidents.hashCode()

    companion object {
        fun of(incidents: List<YuvaStatusIncident>): StatusNotice? =
            if (incidents.isEmpty()) {
                null
            } else {
                StatusNotice(incidents.sortedWith(compareByDescending<YuvaStatusIncident> { it.impact.ordinal }.thenByDescending { it.updatedAt }))
            }
    }
}

internal class StatusDismissals(private val save: (Set<String>) -> Unit, initial: Set<String>) {
    var entries by mutableStateOf(initial)
        private set

    fun notice(status: YuvaInboxStatus?): StatusNotice? =
        StatusNotice.of(status?.open.orEmpty().filter { entry(it) !in entries })

    fun dismiss(notice: StatusNotice) {
        entries = entries + notice.incidents.map(::entry)
        save(entries)
    }

    fun prune(status: YuvaInboxStatus?) {
        val kept = entries intersect status?.open.orEmpty().map(::entry).toSet()
        if (kept == entries) return
        entries = kept
        save(kept)
    }

    companion object {
        fun entry(incident: YuvaStatusIncident) = "${incident.id}|${incident.updatedAt.toEpochMilli()}"

        @Volatile private var shared: StatusDismissals? = null

        fun of(context: Context): StatusDismissals = shared ?: synchronized(this) {
            shared ?: run {
                val prefs = context.applicationContext.getSharedPreferences("yuva.status", Context.MODE_PRIVATE)
                StatusDismissals({ prefs.edit().putStringSet("dismissed", it).apply() }, prefs.getStringSet("dismissed", null).orEmpty().toSet())
            }.also { shared = it }
        }
    }
}

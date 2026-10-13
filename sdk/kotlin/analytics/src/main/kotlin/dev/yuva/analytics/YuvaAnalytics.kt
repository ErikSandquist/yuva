package dev.yuva.analytics

import android.content.Context
import android.content.SharedPreferences
import android.content.pm.PackageManager
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.util.Log
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.lifecycle.DefaultLifecycleObserver
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.ProcessLifecycleOwner
import java.io.File
import java.math.BigDecimal
import java.util.Locale
import java.util.UUID
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.launch
import kotlinx.serialization.json.JsonPrimitive

class YuvaAnalyticsConfiguration(
    val serverUrl: String,
    val key: String,
    val appId: String? = null,
    val requireConsent: Boolean = false,
    val debug: Boolean = false,
)

class YuvaRevenue(val amount: BigDecimal, val currency: String)

object YuvaAnalytics {
    private const val PREFERENCES = "yuva_analytics"
    private const val OPT_OUT = "optout"
    private const val PERSISTENT_ID = "persistent_id"
    private const val PERSISTENT_CREATED = "persistent_created_at"
    private const val TAG = "YuvaAnalytics"
    internal val processKey: String = UUID.randomUUID().toString()

    private val lock = Any()
    private var commands: Channel<AnalyticsCommand>? = null
    private var scope: CoroutineScope? = null
    private var appId = ""
    private var current: String? = null
    private var preferences: SharedPreferences? = null
    private var observing = false

    fun configure(context: Context, configuration: YuvaAnalyticsConfiguration) {
        val app = context.applicationContext
        val prefs = app.getSharedPreferences(PREFERENCES, Context.MODE_PRIVATE)
        val newScope = CoroutineScope(SupervisorJob() + Dispatchers.IO.limitedParallelism(1))
        val core = AnalyticsCore(
            endpoint = configuration.serverUrl.trimEnd('/') + "/client/v1/analytics/" + configuration.key,
            processKey = processKey,
            app = appInfo(app),
            home = "app://${configuration.appId ?: app.packageName}/",
            requireConsent = configuration.requireConsent,
            optedOut = prefs.getBoolean(OPT_OUT, false),
            queueFile = File(File(app.noBackupFilesDir, "yuva-analytics"), fileName(configuration.key)),
            ids = PreferencesIdStore(prefs),
            transport = HttpTransport,
            scope = newScope,
            log = if (configuration.debug) { message -> Log.d(TAG, message) } else null,
        )
        val channel = Channel<AnalyticsCommand>(Channel.UNLIMITED)
        val previous = synchronized(lock) {
            val old = commands to scope
            commands = channel
            scope = newScope
            appId = configuration.appId ?: app.packageName
            preferences = prefs
            old
        }
        previous.first?.close()
        previous.second?.cancel()
        newScope.launch {
            core.start()
            for (command in channel) core.handle(command)
        }
        observeLifecycle()
    }

    fun screen(name: String) {
        val url = synchronized(lock) {
            if (commands == null) return
            screenUrl(appId, name).also { current = it }
        }
        send(AnalyticsCommand.Record(QueuedEvent(n = "screen", u = url, t = System.currentTimeMillis())))
    }

    fun track(
        name: String,
        props: Map<String, Any> = emptyMap(),
        revenue: YuvaRevenue? = null,
        interactive: Boolean = true,
    ) {
        val url = synchronized(lock) {
            if (commands == null) return
            current ?: "app://$appId/"
        }
        send(
            AnalyticsCommand.Record(
                QueuedEvent(
                    n = name,
                    u = url,
                    t = System.currentTimeMillis(),
                    p = props.takeIf { it.isNotEmpty() }?.mapValues { (_, value) -> primitive(value) },
                    revenue = revenue?.let { RevenueBody(it.amount.toPlainString(), it.currency.uppercase(Locale.ROOT)) },
                    i = if (interactive) null else false,
                ),
            ),
        )
    }

    fun consent(granted: Boolean, persistent: Boolean = false) {
        send(AnalyticsCommand.Consent(granted, persistent))
    }

    fun identify(token: String) {
        send(AnalyticsCommand.Identify(token))
    }

    val persistentId: String?
        get() {
            val prefs = synchronized(lock) { preferences } ?: return null
            return PreferencesIdStore(prefs).load()?.takeUnless { it.expired(System.currentTimeMillis()) }?.id
        }

    fun optOut() {
        synchronized(lock) { preferences }?.edit()?.putBoolean(OPT_OUT, true)?.apply()
        send(AnalyticsCommand.OptOut)
    }

    fun optIn() {
        synchronized(lock) { preferences }?.edit()?.remove(OPT_OUT)?.apply()
        send(AnalyticsCommand.OptIn)
    }

    val isOptedOut: Boolean
        get() = synchronized(lock) { preferences }?.getBoolean(OPT_OUT, false) ?: false

    fun flush() {
        send(AnalyticsCommand.Flush())
    }

    private class PreferencesIdStore(private val prefs: SharedPreferences) : PersistentIdStore {
        override fun load(): PersistentId? {
            val id = prefs.getString(PERSISTENT_ID, null) ?: return null
            val created = prefs.getLong(PERSISTENT_CREATED, -1).takeIf { it >= 0 } ?: return null
            return PersistentId(id, created)
        }

        override fun save(value: PersistentId) {
            prefs.edit().putString(PERSISTENT_ID, value.id).putLong(PERSISTENT_CREATED, value.createdAt).apply()
        }

        override fun delete() {
            prefs.edit().remove(PERSISTENT_ID).remove(PERSISTENT_CREATED).apply()
        }
    }

    private fun send(command: AnalyticsCommand) {
        synchronized(lock) { commands }?.trySend(command)
    }

    private fun observeLifecycle() {
        val register = Runnable {
            synchronized(lock) {
                if (observing) return@Runnable
                observing = true
            }
            ProcessLifecycleOwner.get().lifecycle.addObserver(object : DefaultLifecycleObserver {
                override fun onStart(owner: LifecycleOwner) = flush()
                override fun onStop(owner: LifecycleOwner) = flush()
            })
        }
        if (Looper.myLooper() == Looper.getMainLooper()) register.run() else Handler(Looper.getMainLooper()).post(register)
    }

    private fun appInfo(context: Context): AppInfo {
        val configuration = context.resources.configuration
        val packages = context.packageManager
        val version = runCatching {
            if (Build.VERSION.SDK_INT >= 33) {
                packages.getPackageInfo(context.packageName, PackageManager.PackageInfoFlags.of(0)).versionName
            } else {
                @Suppress("DEPRECATION")
                packages.getPackageInfo(context.packageName, 0).versionName
            }
        }.getOrNull()
        val device = when {
            packages.hasSystemFeature("org.chromium.arc") || packages.hasSystemFeature("org.chromium.arc.device_management") -> "desktop"
            configuration.smallestScreenWidthDp >= 600 -> "tablet"
            else -> "mobile"
        }
        return AppInfo(
            platform = "android",
            version = version,
            osVersion = Build.VERSION.RELEASE,
            device = device,
            width = configuration.screenWidthDp.takeIf { it > 0 },
            language = configuration.locales[0]?.toLanguageTag(),
        )
    }

    private fun fileName(key: String) = key.map { if (it.isLetterOrDigit() || it == '_') it else '-' }.joinToString("") + ".json"

    private fun primitive(value: Any): JsonPrimitive = when (value) {
        is String -> JsonPrimitive(value)
        is Number -> JsonPrimitive(value)
        is Boolean -> JsonPrimitive(value)
        else -> JsonPrimitive(value.toString())
    }

    internal fun screenUrl(appId: String, name: String): String {
        val out = StringBuilder("app://").append(appId).append('/')
        for (byte in name.trim('/').toByteArray(Charsets.UTF_8)) {
            val code = byte.toInt() and 0xff
            val char = code.toChar()
            if (code < 0x80 && (char in 'a'..'z' || char in 'A'..'Z' || char in '0'..'9' || char in PATH_SAFE)) {
                out.append(char)
            } else {
                out.append('%').append(HEX[code shr 4]).append(HEX[code and 0xf])
            }
        }
        return out.toString()
    }

    private const val PATH_SAFE = "!$&'()*+,-./:;=@_~"
    private const val HEX = "0123456789ABCDEF"
}

@Composable
fun TrackScreen(name: String) {
    LaunchedEffect(name) { YuvaAnalytics.screen(name) }
}

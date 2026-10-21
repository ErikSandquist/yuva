package dev.yuva

import android.app.Activity
import android.app.Application
import android.content.ContentProvider
import android.content.ContentValues
import android.content.Intent
import android.database.Cursor
import android.graphics.Bitmap
import android.hardware.Sensor
import android.hardware.SensorEvent
import android.hardware.SensorEventListener
import android.hardware.SensorManager
import android.net.Uri
import android.os.Bundle
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.drawToBitmap
import dev.yuva.ui.YuvaShakeActivity
import java.lang.ref.WeakReference
import kotlin.math.sqrt

internal class ShakeSession(
    val client: YuvaClient,
    val screenshot: Bitmap,
    val screen: String,
    val theme: (@Composable (@Composable () -> Unit) -> Unit)?,
)

internal object ShakeFeedback {
    private const val THRESHOLD_G = 2.7f
    private const val MIN_GAP_MS = 120L
    private const val WINDOW_MS = 1000L
    private const val SHAKES = 3
    private const val QUIET_MS = 2000L

    private var application: Application? = null
    private var client: YuvaClient? = null
    private var screen: (() -> String?)? = null
    private var isEnabled: () -> Boolean = { true }
    private var theme: (@Composable (@Composable () -> Unit) -> Unit)? = null
    private var resumed: WeakReference<Activity>? = null
    private var listening = false
    private val peaks = ArrayDeque<Long>()
    private var lastTrigger = 0L

    var open = 0
    var pending: ShakeSession? = null

    fun attach(application: Application) {
        if (this.application != null) return
        this.application = application
        application.registerActivityLifecycleCallbacks(object : Application.ActivityLifecycleCallbacks {
            override fun onActivityResumed(activity: Activity) {
                if (activity is YuvaShakeActivity) return
                resumed = WeakReference(activity)
                listen()
            }

            override fun onActivityPaused(activity: Activity) {
                if (resumed?.get() === activity) resumed = null
                stopListening()
            }

            override fun onActivityCreated(activity: Activity, savedInstanceState: Bundle?) = Unit
            override fun onActivityStarted(activity: Activity) = Unit
            override fun onActivityStopped(activity: Activity) = Unit
            override fun onActivitySaveInstanceState(activity: Activity, outState: Bundle) = Unit
            override fun onActivityDestroyed(activity: Activity) = Unit
        })
    }

    fun enable(
        application: Application,
        client: YuvaClient,
        screen: (() -> String?)?,
        isEnabled: () -> Boolean,
        theme: (@Composable (@Composable () -> Unit) -> Unit)?,
    ) {
        attach(application)
        this.client = client
        this.screen = screen
        this.isEnabled = isEnabled
        this.theme = theme
        listen()
    }

    fun disable() {
        client = null
        stopListening()
    }

    private val listener = object : SensorEventListener {
        override fun onSensorChanged(event: SensorEvent) {
            val g = sqrt(event.values[0] * event.values[0] + event.values[1] * event.values[1] + event.values[2] * event.values[2]) /
                SensorManager.GRAVITY_EARTH
            if (g < THRESHOLD_G) return
            val now = event.timestamp / 1_000_000
            if (now - lastTrigger < QUIET_MS) return
            if (peaks.isNotEmpty() && now - peaks.last() < MIN_GAP_MS) return
            peaks.addLast(now)
            while (peaks.isNotEmpty() && now - peaks.first() > WINDOW_MS) peaks.removeFirst()
            if (peaks.size >= SHAKES) {
                peaks.clear()
                lastTrigger = now
                shaken()
            }
        }

        override fun onAccuracyChanged(sensor: Sensor?, accuracy: Int) = Unit
    }

    private fun listen() {
        val application = application ?: return
        if (listening || client == null || resumed?.get() == null) return
        val manager = application.getSystemService(SensorManager::class.java) ?: return
        val accelerometer = manager.getDefaultSensor(Sensor.TYPE_ACCELEROMETER) ?: return
        peaks.clear()
        listening = manager.registerListener(listener, accelerometer, SensorManager.SENSOR_DELAY_GAME)
    }

    private fun stopListening() {
        if (!listening) return
        listening = false
        application?.getSystemService(SensorManager::class.java)?.unregisterListener(listener)
    }

    private fun shaken() {
        val client = client ?: return
        val activity = resumed?.get() ?: return
        if (activity.isFinishing || open > 0 || !isEnabled() || isTyping(activity)) return
        val bitmap = runCatching { activity.window.decorView.drawToBitmap() }.getOrNull() ?: return
        pending = ShakeSession(client, bitmap, screen?.invoke() ?: activity.javaClass.simpleName, theme)
        activity.startActivity(Intent(activity, YuvaShakeActivity::class.java))
    }

    private fun isTyping(activity: Activity): Boolean {
        val decor = activity.window.decorView
        return activity.currentFocus?.onCheckIsTextEditor() == true ||
            ViewCompat.getRootWindowInsets(decor)?.isVisible(WindowInsetsCompat.Type.ime()) == true
    }
}

@Composable
internal fun TrackOpen() {
    DisposableEffect(Unit) {
        ShakeFeedback.open++
        onDispose { ShakeFeedback.open = maxOf(0, ShakeFeedback.open - 1) }
    }
}

internal class YuvaInitProvider : ContentProvider() {
    override fun onCreate(): Boolean {
        (context?.applicationContext as? Application)?.let(ShakeFeedback::attach)
        return true
    }

    override fun query(uri: Uri, projection: Array<String>?, selection: String?, selectionArgs: Array<String>?, sortOrder: String?): Cursor? = null
    override fun getType(uri: Uri): String? = null
    override fun insert(uri: Uri, values: ContentValues?): Uri? = null
    override fun delete(uri: Uri, selection: String?, selectionArgs: Array<String>?): Int = 0
    override fun update(uri: Uri, values: ContentValues?, selection: String?, selectionArgs: Array<String>?): Int = 0
}

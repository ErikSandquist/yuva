package dev.yuva.ui

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import android.graphics.Bitmap
import android.os.Build
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import dev.yuva.R
import dev.yuva.ShakeFeedback
import dev.yuva.ShakeSession
import dev.yuva.YuvaUpload
import java.io.ByteArrayOutputStream
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

internal class YuvaShakeActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val session = ShakeFeedback.pending
        if (session == null) {
            finish()
            return
        }
        enableEdgeToEdge()
        setContent {
            val theme = session.theme
            if (theme != null) theme { ShakeContent(session, ::finish) } else DefaultTheme { ShakeContent(session, ::finish) }
        }
    }

    override fun onDestroy() {
        super.onDestroy()
        if (isFinishing) {
            ShakeFeedback.pending?.screenshot?.recycle()
            ShakeFeedback.pending = null
        }
    }
}

@Composable
private fun DefaultTheme(content: @Composable () -> Unit) {
    val dark = isSystemInDarkTheme()
    val context = LocalContext.current
    val colors = when {
        Build.VERSION.SDK_INT >= Build.VERSION_CODES.S -> if (dark) dynamicDarkColorScheme(context) else dynamicLightColorScheme(context)
        dark -> darkColorScheme()
        else -> lightColorScheme()
    }
    MaterialTheme(colorScheme = colors, content = content)
}

@Composable
private fun ShakeContent(session: ShakeSession, onDone: () -> Unit) {
    var confirmed by rememberSaveable { mutableStateOf(false) }
    var upload by remember { mutableStateOf<YuvaUpload?>(null) }
    var encoded by remember { mutableStateOf(false) }
    LaunchedEffect(confirmed) {
        if (!confirmed) return@LaunchedEffect
        upload = withContext(Dispatchers.Default) {
            val out = ByteArrayOutputStream()
            if (session.screenshot.compress(Bitmap.CompressFormat.JPEG, 80, out)) {
                YuvaUpload("screenshot.jpg", "image/jpeg", out.toByteArray())
            } else {
                null
            }
        }
        encoded = true
    }
    if (!confirmed) {
        AlertDialog(
            onDismissRequest = onDone,
            title = { Text(stringResource(R.string.yuva_shake_title)) },
            text = { Text(stringResource(R.string.yuva_shake_message)) },
            confirmButton = { TextButton(onClick = { confirmed = true }) { Text(stringResource(R.string.yuva_feedback_send)) } },
            dismissButton = { TextButton(onClick = onDone) { Text(stringResource(R.string.yuva_feedback_cancel)) } },
        )
    } else if (encoded) {
        YuvaFeedbackView(session.client, screenshot = upload, screen = session.screen, onDone = onDone)
    }
}

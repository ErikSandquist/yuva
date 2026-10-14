package dev.yuva.sample

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import dev.yuva.analytics.YuvaAnalytics
import dev.yuva.analytics.YuvaRequestInterceptor
import java.util.UUID
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request

private val client by lazy { OkHttpClient.Builder().addInterceptor(YuvaRequestInterceptor()).build() }

@Composable
internal fun ApiCalls() {
    val calls = remember { mutableStateListOf<String>() }
    var queued by remember { mutableStateOf<Int?>(null) }
    var running by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text("API calls", style = MaterialTheme.typography.titleSmall)
        if (BuildConfig.API_KEY.isEmpty()) {
            Text("Add apiKey to sample/yuva.properties to time API calls.", style = MaterialTheme.typography.bodySmall)
        }
        Button(
            onClick = {
                running = true
                calls.clear()
                scope.launch {
                    val paths = listOf("/healthz", "/readyz", "/client/v1/conversations/${UUID.randomUUID()}", "/not-mapped/42")
                    for (path in paths) {
                        val line = withContext(Dispatchers.IO) {
                            runCatching {
                                client.newCall(Request.Builder().url(BuildConfig.SERVER_URL.trimEnd('/') + path).build()).execute()
                                    .use { response ->
                                        response.body.string()
                                        "GET ${path.take(32)}  ${response.code}"
                                    }
                            }.getOrElse { "GET ${path.take(32)}  ${it.message}" }
                        }
                        calls += line
                    }
                    delay(300)
                    queued = YuvaAnalytics.queuedRequests()
                    running = false
                }
            },
            enabled = !running && BuildConfig.API_KEY.isNotEmpty(),
            modifier = Modifier.fillMaxWidth().testTag("sample.requests.call"),
        ) { Text("Call the API") }
        calls.forEach { Text(it, style = MaterialTheme.typography.bodySmall, fontFamily = FontFamily.Monospace) }
        queued?.let { Text("Queued: $it", Modifier.testTag("sample.requests.queued")) }
        Text(
            "Timed by YuvaRequestInterceptor; calls with no matching route are skipped.",
            style = MaterialTheme.typography.bodySmall,
        )
    }
}

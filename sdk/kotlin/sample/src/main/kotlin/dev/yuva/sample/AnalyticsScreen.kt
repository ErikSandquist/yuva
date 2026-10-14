package dev.yuva.sample

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import dev.yuva.analytics.TrackScreen
import dev.yuva.analytics.YuvaAnalytics
import dev.yuva.analytics.YuvaRevenue
import java.math.BigDecimal
import java.time.LocalTime
import java.time.format.DateTimeFormatter
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun AnalyticsScreen(onClose: () -> Unit) {
    var detail by remember { mutableStateOf(false) }
    var optedOut by remember { mutableStateOf(YuvaAnalytics.isOptedOut) }
    var consent by remember { mutableStateOf(false) }
    var persistent by remember { mutableStateOf(YuvaAnalytics.persistentId != null) }
    var persistentId by remember { mutableStateOf(YuvaAnalytics.persistentId) }
    var token by remember { mutableStateOf("") }
    val scope = rememberCoroutineScope()
    fun refreshId() {
        scope.launch {
            delay(200)
            persistentId = YuvaAnalytics.persistentId
        }
    }
    val log = remember { mutableStateListOf<String>() }
    fun note(text: String) = log.add(0, LocalTime.now().format(DateTimeFormatter.ofPattern("HH:mm:ss")) + "  " + text)

    if (detail) {
        TrackScreen("Product Detail")
        Scaffold(topBar = {
            TopAppBar(
                title = { Text("Product Detail") },
                navigationIcon = {
                    IconButton(onClick = { detail = false }) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back") }
                },
            )
        }) { padding ->
            Text("Product Detail", Modifier.padding(padding).padding(16.dp), style = MaterialTheme.typography.titleLarge)
        }
        return
    }
    TrackScreen("Analytics")
    Scaffold(topBar = {
        TopAppBar(
            title = { Text("Analytics") },
            navigationIcon = { IconButton(onClick = onClose) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back") } },
        )
    }) { padding ->
        Column(
            Modifier.fillMaxSize().padding(padding).padding(16.dp).verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            if (BuildConfig.ANALYTICS_KEY.isEmpty()) {
                Text("Add analyticsKey to sample/yuva.properties to send analytics.", style = MaterialTheme.typography.bodySmall)
            }
            Button(onClick = {
                YuvaAnalytics.track(
                    "Purchase",
                    props = mapOf("plan" to "pro", "seats" to 3),
                    revenue = YuvaRevenue(BigDecimal("9.99"), "EUR"),
                )
                note("Purchase")
            }, Modifier.fillMaxWidth().testTag("sample.analytics.purchase")) { Text("Track Purchase (9.99 EUR)") }
            Button(onClick = {
                YuvaAnalytics.track("Signup", props = mapOf("source" to "sample"))
                note("Signup")
            }, Modifier.fillMaxWidth().testTag("sample.analytics.signup")) { Text("Track Signup") }
            OutlinedButton(onClick = { detail = true }, Modifier.fillMaxWidth().testTag("sample.analytics.product")) {
                Text("Open Product Detail")
            }
            OutlinedButton(onClick = {
                YuvaAnalytics.flush()
                note("Send now")
            }, Modifier.fillMaxWidth().testTag("sample.analytics.flush")) { Text("Send now") }
            ApiCalls()
            if (BuildConfig.ANALYTICS_CONSENT == "required") {
                ToggleRow("Analytics consent", consent, "sample.analytics.consent") {
                    consent = it
                    if (!it) persistent = false
                    YuvaAnalytics.consent(it, persistent)
                    refreshId()
                }
            }
            ToggleRow("Remember me (persistent)", persistent, "sample.analytics.persistent") {
                persistent = it
                if (it && BuildConfig.ANALYTICS_CONSENT == "required") consent = true
                YuvaAnalytics.consent(it || consent || BuildConfig.ANALYTICS_CONSENT != "required", it)
                note(if (it) "Persistent consent" else "Persistent withdrawn")
                refreshId()
            }
            persistentId?.let {
                Text(
                    it,
                    Modifier.testTag("sample.analytics.persistentId"),
                    style = MaterialTheme.typography.bodySmall,
                    fontFamily = FontFamily.Monospace,
                )
            }
            OutlinedTextField(
                value = token,
                onValueChange = { token = it },
                label = { Text("Identity token") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth().testTag("sample.analytics.token"),
            )
            OutlinedButton(
                onClick = {
                    YuvaAnalytics.identify(token)
                    note("Identify")
                    token = ""
                },
                enabled = persistentId != null && token.isNotEmpty(),
                modifier = Modifier.fillMaxWidth().testTag("sample.analytics.identify"),
            ) { Text("Identify") }
            ToggleRow("Opt out", optedOut, "sample.analytics.optout") {
                optedOut = it
                if (it) {
                    YuvaAnalytics.optOut()
                    persistent = false
                } else {
                    YuvaAnalytics.optIn()
                }
                note(if (it) "Opted out" else "Opted in")
                refreshId()
            }
            log.forEach { Text(it, style = MaterialTheme.typography.bodySmall, fontFamily = FontFamily.Monospace) }
        }
    }
}

@Composable
private fun ToggleRow(label: String, checked: Boolean, tag: String, onChange: (Boolean) -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .toggleable(value = checked, role = Role.Switch, onValueChange = onChange)
            .testTag(tag)
            .padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, Modifier.weight(1f))
        Switch(checked = checked, onCheckedChange = null)
    }
}

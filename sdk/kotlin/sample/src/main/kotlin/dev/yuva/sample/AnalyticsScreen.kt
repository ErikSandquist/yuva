package dev.yuva.sample

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.ShoppingCart
import androidx.compose.material.icons.filled.Star
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
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

@Composable
internal fun AnalyticsScreen(onClose: () -> Unit) {
    var detail by remember { mutableStateOf(false) }
    var optedOut by remember { mutableStateOf(YuvaAnalytics.isOptedOut) }
    var consent by remember { mutableStateOf(false) }
    var persistent by remember { mutableStateOf(YuvaAnalytics.persistentId != null) }
    var persistentId by remember { mutableStateOf(YuvaAnalytics.persistentId) }
    var token by remember { mutableStateOf("") }
    val log = remember { mutableStateListOf<String>() }
    val scope = rememberCoroutineScope()
    val consentRequired = BuildConfig.ANALYTICS_CONSENT == "required"
    val colors = MaterialTheme.colorScheme

    fun refreshId() {
        scope.launch {
            delay(200)
            persistentId = YuvaAnalytics.persistentId
        }
    }

    fun note(text: String) = log.add(0, LocalTime.now().format(DateTimeFormatter.ofPattern("HH:mm:ss")) + "  " + text)

    if (detail) {
        TrackScreen("Product Detail")
        SamplePage("Product Detail", onBack = { detail = false }) {}
        return
    }
    TrackScreen("Analytics")
    SamplePage("Analytics", onBack = onClose) {
        if (BuildConfig.ANALYTICS_KEY.isEmpty()) {
            item {
                Tile(
                    "Analytics is off",
                    groupShape(0, 1),
                    supporting = "Add analyticsKey to sample/yuva.properties to send analytics.",
                    icon = Icons.Default.Info,
                    tint = colors.onSurfaceVariant,
                )
            }
        }
        item { SectionLabel("Events") }
        item {
            Tile(
                "Track Purchase",
                groupShape(0, 4),
                supporting = "9.99 EUR, plan pro, 3 seats",
                icon = Icons.Default.ShoppingCart,
                tint = colors.primary,
                onClick = {
                    YuvaAnalytics.track(
                        "Purchase",
                        props = mapOf("plan" to "pro", "seats" to 3),
                        revenue = YuvaRevenue(BigDecimal("9.99"), "EUR"),
                    )
                    note("Purchase")
                },
                modifier = Modifier.testTag("sample.analytics.purchase"),
            )
        }
        item {
            Tile(
                "Track Signup",
                groupShape(1, 4),
                supporting = "source sample",
                icon = Icons.Default.Person,
                tint = colors.secondary,
                onClick = {
                    YuvaAnalytics.track("Signup", props = mapOf("source" to "sample"))
                    note("Signup")
                },
                modifier = Modifier.testTag("sample.analytics.signup"),
            )
        }
        item {
            Tile(
                "Open Product Detail",
                groupShape(2, 4),
                supporting = "Records a screen view",
                icon = Icons.Default.Star,
                tint = colors.tertiary,
                opensScreen = true,
                onClick = { detail = true },
                modifier = Modifier.testTag("sample.analytics.product"),
            )
        }
        item {
            Tile(
                "Send now",
                groupShape(3, 4),
                supporting = "Flush the queue",
                icon = Icons.AutoMirrored.Filled.Send,
                tint = colors.primary,
                onClick = {
                    YuvaAnalytics.flush()
                    note("Send now")
                },
                modifier = Modifier.testTag("sample.analytics.flush"),
            )
        }
        item { SectionLabel("Privacy") }
        val toggles = if (consentRequired) 3 else 2
        if (consentRequired) {
            item {
                SwitchTile("Analytics consent", consent, groupShape(0, toggles), "sample.analytics.consent") {
                    consent = it
                    if (!it) persistent = false
                    YuvaAnalytics.consent(it, persistent)
                    refreshId()
                }
            }
        }
        item {
            SwitchTile(
                "Remember me",
                persistent,
                groupShape(toggles - 2, toggles),
                "sample.analytics.persistent",
                supporting = "Keep one id across launches",
            ) {
                persistent = it
                if (it && consentRequired) consent = true
                YuvaAnalytics.consent(it || consent || !consentRequired, it)
                note(if (it) "Persistent consent" else "Persistent withdrawn")
                refreshId()
            }
        }
        item {
            SwitchTile("Opt out", optedOut, groupShape(toggles - 1, toggles), "sample.analytics.optout") {
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
        }
        persistentId?.let {
            item {
                Text(
                    it,
                    Modifier.padding(horizontal = 16.dp, vertical = 4.dp).testTag("sample.analytics.persistentId"),
                    style = MaterialTheme.typography.bodySmall,
                    color = colors.onSurfaceVariant,
                    fontFamily = FontFamily.Monospace,
                )
            }
        }
        item { SectionLabel("Identify") }
        item {
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                TextField(
                    value = token,
                    onValueChange = { token = it },
                    label = { Text("Identity token") },
                    singleLine = true,
                    shape = groupShape(0, 1),
                    colors = TextFieldDefaults.colors(
                        focusedContainerColor = tileColor,
                        unfocusedContainerColor = tileColor,
                        disabledContainerColor = tileColor,
                        focusedIndicatorColor = Color.Transparent,
                        unfocusedIndicatorColor = Color.Transparent,
                        disabledIndicatorColor = Color.Transparent,
                    ),
                    modifier = Modifier.weight(1f).testTag("sample.analytics.token"),
                )
                FilledTonalButton(
                    onClick = {
                        YuvaAnalytics.identify(token)
                        note("Identify")
                        token = ""
                    },
                    enabled = persistentId != null && token.isNotEmpty(),
                    modifier = Modifier.testTag("sample.analytics.identify"),
                ) { Text("Identify") }
            }
        }
        if (log.isNotEmpty()) {
            item { SectionLabel("Recorded") }
            log.forEachIndexed { index, line ->
                item {
                    Tile(line, groupShape(index, log.size))
                }
            }
        }
    }
}

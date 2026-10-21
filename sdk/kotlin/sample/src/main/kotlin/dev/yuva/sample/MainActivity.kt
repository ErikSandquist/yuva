package dev.yuva.sample

import android.app.Application
import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import android.os.Bundle
import android.view.View
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.spring
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.List
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material.icons.filled.Email
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Star
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.core.view.drawToBitmap
import dev.yuva.Yuva
import dev.yuva.YuvaClient
import dev.yuva.YuvaConfiguration
import dev.yuva.YuvaSubscription
import dev.yuva.YuvaSubscriptionState
import dev.yuva.YuvaUpload
import dev.yuva.analytics.TrackScreen
import dev.yuva.ui.YuvaConversationsSection
import dev.yuva.ui.YuvaConversationsView
import dev.yuva.ui.YuvaFeedbackView
import java.io.ByteArrayOutputStream
import java.net.URL
import java.net.URLEncoder
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext

class MainActivity : ComponentActivity() {
    private val requested = mutableStateOf<Request?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        requested.value = request(intent)
        setContent {
            SampleTheme { Sample(this, requested.value) }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        requested.value = request(intent)
    }

    private fun request(intent: Intent): Request {
        val extras = intent.extras?.keySet()?.associateWith { intent.extras?.getString(it).orEmpty() }.orEmpty()
        return Request(Yuva.handleNotification(extras), intent.getStringExtra("yuva_open"))
    }
}

private class Request(val conversation: String?, val screen: String?)

private fun client(context: Context, identified: Boolean, userId: String): YuvaClient {
    val tokenUrl = BuildConfig.IDENTITY_TOKEN_URL
    val provider: (suspend () -> String?)? = if (identified && userId.isNotBlank() && tokenUrl.isNotEmpty()) {
        {
            withContext(Dispatchers.IO) {
                URL(tokenUrl + "?sub=" + URLEncoder.encode(userId, "UTF-8")).readText().trim()
            }
        }
    } else {
        null
    }
    return YuvaClient(context, YuvaConfiguration(BuildConfig.SERVER_URL, BuildConfig.CHANNEL_KEY, provider))
}

@Composable
private fun Sample(context: Context, request: Request?) {
    val openConversation = request?.conversation
    val prefs = remember { context.getSharedPreferences("sample", Context.MODE_PRIVATE) }
    var identified by remember { mutableStateOf(prefs.getBoolean("identified", false)) }
    var userId by remember { mutableStateOf(prefs.getString("user_id", "").orEmpty()) }
    fun target(request: Request?) = if (request?.conversation != null) "messages" else request?.screen
    var screen by remember { mutableStateOf(target(request)) }
    LaunchedEffect(request) {
        if (request != null) screen = target(request)
    }
    var yuva by remember { mutableStateOf<YuvaClient?>(client(context, identified, userId)) }
    var screenshot by remember { mutableStateOf<YuvaUpload?>(null) }
    val current = yuva
    val view = LocalView.current
    val scope = rememberCoroutineScope()
    val swap = remember { Mutex() }

    fun replaceClient(signOut: Boolean) {
        val previous = yuva
        yuva = null
        scope.launch {
            swap.withLock {
                if (signOut) (yuva ?: previous)?.signOut()
                yuva = client(context, identified, userId)
            }
        }
    }

    val currentScreen by rememberUpdatedState(screen)
    DisposableEffect(current) {
        if (current != null) {
            Yuva.enableShakeToFeedback(
                context.applicationContext as Application,
                current,
                screen = { "sample." + (currentScreen ?: "home") },
                theme = { content -> SampleTheme(content) },
            )
        }
        onDispose { Yuva.disableShakeToFeedback() }
    }

    LaunchedEffect(identified, userId) {
        prefs.edit().putBoolean("identified", identified).putString("user_id", userId).apply()
    }

    if (BuildConfig.SERVER_URL.isEmpty()) {
        Text("Copy sample/yuva.properties.example to sample/yuva.properties and rebuild.", Modifier.padding(32.dp))
        return
    }
    val shown = screen.takeIf { it == "analytics" || current != null }
    AnimatedContent(
        targetState = shown,
        transitionSpec = {
            val forward = initialState == null
            val motion = spring<IntOffset>(dampingRatio = 0.9f, stiffness = Spring.StiffnessMediumLow)
            (slideInHorizontally(motion) { if (forward) it / 4 else -it / 4 } + fadeIn())
                .togetherWith(slideOutHorizontally(motion) { if (forward) -it / 4 else it / 4 } + fadeOut())
        },
        label = "screen",
    ) { target ->
        if (target != null) BackHandler { screen = null }
        when (target) {
            "analytics" -> AnalyticsScreen(onClose = { screen = null })
            "messages", "messages-only", "feedback-list" -> if (current != null) YuvaConversationsView(
                current,
                openConversationId = openConversation,
                onClose = { screen = null },
                section = when (target) {
                    "messages-only" -> YuvaConversationsSection.MESSAGES
                    "feedback-list" -> YuvaConversationsSection.FEEDBACK
                    else -> null
                },
            )
            "feedback" -> if (current != null) YuvaFeedbackView(current, screenshot = screenshot, screen = "sample.home", onDone = { screen = null })
            else -> Home(
                identified = identified,
                userId = userId,
                client = current,
                onIdentified = { value ->
                    val signOut = identified && !value
                    identified = value
                    replaceClient(signOut)
                },
                onUserId = {
                    userId = it
                    replaceClient(false)
                },
                onOpen = { next ->
                    if (next == "feedback") screenshot = capture(view)
                    screen = next
                },
            )
        }
    }
}

@Composable
private fun Home(
    identified: Boolean,
    userId: String,
    client: YuvaClient?,
    onIdentified: (Boolean) -> Unit,
    onUserId: (String) -> Unit,
    onOpen: (String) -> Unit,
) {
    TrackScreen("Home")
    val colors = MaterialTheme.colorScheme
    SamplePage("Yuva Sample") {
        item { SectionLabel("Mode") }
        item {
            ConnectedToggle(
                options = listOf(false to "Anonymous", true to "Identified"),
                selected = identified,
                onSelect = onIdentified,
            )
        }
        if (identified) {
            item {
                TextField(
                    value = userId,
                    onValueChange = onUserId,
                    label = { Text("Host user id") },
                    singleLine = true,
                    keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
                    shape = groupShape(0, 1),
                    colors = TextFieldDefaults.colors(
                        focusedContainerColor = tileColor,
                        unfocusedContainerColor = tileColor,
                        disabledContainerColor = tileColor,
                        focusedIndicatorColor = Color.Transparent,
                        unfocusedIndicatorColor = Color.Transparent,
                    ),
                    modifier = Modifier.fillParentMaxWidth().padding(top = 8.dp),
                )
            }
        }
        item { SectionLabel("Support") }
        val screens = listOf(
            HomeRow("messages", "Messages and feedback", "Everything in one place", Icons.Default.Home, colors.primary),
            HomeRow("messages-only", "Messages only", "Conversations with the team", Icons.Default.Email, colors.secondary),
            HomeRow("feedback-list", "Feedback only", "Bugs, ideas and praise you sent", Icons.AutoMirrored.Filled.List, colors.tertiary),
            HomeRow("feedback", "Send feedback", "With a screenshot of this screen", Icons.AutoMirrored.Filled.Send, colors.error),
        )
        screens.forEachIndexed { index, row ->
            item(key = row.screen) {
                Tile(
                    row.title,
                    groupShape(index, screens.size),
                    supporting = row.supporting,
                    icon = row.icon,
                    tint = row.tint,
                    enabled = client != null,
                    opensScreen = true,
                    onClick = { onOpen(row.screen) },
                    modifier = Modifier.testTag("sample.open.${row.screen}"),
                )
            }
        }
        item { SectionLabel("Analytics") }
        item {
            Tile(
                "Analytics",
                groupShape(0, 1),
                supporting = "Events, consent and identify",
                icon = Icons.Default.Star,
                tint = colors.primary,
                opensScreen = true,
                onClick = { onOpen("analytics") },
                modifier = Modifier.testTag("sample.analytics"),
            )
        }
        if (identified && userId.isNotBlank() && client != null) {
            item { SectionLabel("Newsletter") }
            item(key = "newsletter") { key(client) { Newsletter(client) } }
        }
        item { SectionLabel("Server") }
        item {
            Tile(
                BuildConfig.SERVER_URL,
                groupShape(0, 1),
                supporting = "Yuva SDK ${Yuva.VERSION}",
                icon = Icons.Default.Info,
                tint = colors.onSurfaceVariant,
            )
        }
    }
}

private class HomeRow(
    val screen: String,
    val title: String,
    val supporting: String,
    val icon: ImageVector,
    val tint: Color,
)

@Composable
private fun Newsletter(yuva: YuvaClient) {
    var subscriptions by remember { mutableStateOf<List<YuvaSubscription>>(emptyList()) }
    val busy = remember { mutableStateListOf<String>() }
    var notice by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    LaunchedEffect(yuva) {
        runCatching { yuva.subscriptions() }
            .onSuccess { subscriptions = it }
            .onFailure { notice = it.message }
    }

    fun toggle(subscription: YuvaSubscription, on: Boolean) {
        val list = subscription.list
        busy += list.id
        scope.launch {
            notice = runCatching {
                if (on) {
                    val consent = if (list.description.isEmpty()) list.name else "${list.name}: ${list.description}"
                    when (yuva.subscribe(list.id, consent)) {
                        YuvaSubscriptionState.PENDING -> "We sent a confirmation e-mail for ${list.name}."
                        else -> "Subscribed to ${list.name}."
                    }
                } else {
                    yuva.unsubscribe(list.id)
                    "Unsubscribed from ${list.name}."
                }
            }.getOrElse { it.message }
            runCatching { yuva.subscriptions() }.onSuccess { subscriptions = it }
            busy -= list.id
        }
    }

    Column(verticalArrangement = Arrangement.spacedBy(3.dp)) {
        subscriptions.forEachIndexed { index, subscription ->
            val state = subscription.state
            SwitchTile(
                title = subscription.list.name,
                checked = state == YuvaSubscriptionState.SUBSCRIBED || state == YuvaSubscriptionState.PENDING,
                shape = groupShape(index, subscriptions.size),
                tag = "sample.newsletter.${subscription.list.name}",
                supporting = if (state == YuvaSubscriptionState.PENDING) {
                    "Waiting for the confirmation e-mail"
                } else {
                    subscription.list.description.ifEmpty { null }
                },
                enabled = subscription.list.id !in busy,
                onChange = { toggle(subscription, it) },
            )
        }
        if (subscriptions.isEmpty() && notice == null) Tile("No public lists", groupShape(0, 1))
        notice?.let {
            Text(
                it,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
            )
        }
    }
}

private fun capture(view: View): YuvaUpload? = runCatching {
    val out = ByteArrayOutputStream()
    view.rootView.drawToBitmap().compress(Bitmap.CompressFormat.JPEG, 80, out)
    YuvaUpload("screenshot.jpg", "image/jpeg", out.toByteArray())
}.getOrNull()

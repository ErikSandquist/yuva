package dev.yuva.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Create
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.background
import android.text.format.DateUtils
import dev.yuva.R
import dev.yuva.YuvaAuthorType
import dev.yuva.YuvaClient
import dev.yuva.YuvaConversation
import dev.yuva.YuvaConversationKind
import dev.yuva.YuvaConversationStatus
import dev.yuva.YuvaFeedbackState
import dev.yuva.feedbackState
import dev.yuva.isFeedback
import dev.yuva.YuvaEvent
import dev.yuva.YuvaInbox
import dev.yuva.YuvaInboxMode
import dev.yuva.YuvaSession
import kotlinx.coroutines.launch

internal enum class ConversationsTab { MESSAGES, FEEDBACK }

internal class ConversationsState(private val client: YuvaClient) {
    val messages = mutableStateListOf<YuvaConversation>()
    val feedback = mutableStateListOf<YuvaConversation>()
    var session by mutableStateOf<YuvaSession?>(null)
    var loading by mutableStateOf(true)
    var failed by mutableStateOf(false)
    private var messagesCursor: String? = null
    private var feedbackCursor: String? = null

    val hasFeedback: Boolean get() = feedback.isNotEmpty()

    fun items(tab: ConversationsTab): List<YuvaConversation> =
        if (tab == ConversationsTab.MESSAGES) messages else feedback

    suspend fun load() {
        loading = messages.isEmpty() && feedback.isEmpty()
        try {
            session = client.start()
            val sent = client.conversations(kind = listOf(YuvaConversationKind.FEEDBACK))
            val page = client.conversations(kind = listOf(YuvaConversationKind.CONVERSATION))
            messages.clear()
            messages.addAll(page.items)
            messagesCursor = page.nextCursor
            feedback.clear()
            feedback.addAll(sent.items)
            feedbackCursor = sent.nextCursor
            failed = false
        } catch (_: Exception) {
            failed = messages.isEmpty() && feedback.isEmpty()
        } finally {
            loading = false
        }
    }

    suspend fun loadMore(tab: ConversationsTab) {
        val messagesTab = tab == ConversationsTab.MESSAGES
        val cursor = (if (messagesTab) messagesCursor else feedbackCursor) ?: return
        if (messagesTab) messagesCursor = null else feedbackCursor = null
        val kind = if (messagesTab) YuvaConversationKind.CONVERSATION else YuvaConversationKind.FEEDBACK
        val page = runCatching { client.conversations(kind = listOf(kind), cursor = cursor) }.getOrNull() ?: run {
            if (messagesTab) messagesCursor = cursor else feedbackCursor = cursor
            return
        }
        val list = if (messagesTab) messages else feedback
        val known = list.map { it.id }.toSet()
        list.addAll(page.items.filter { it.id !in known })
        if (messagesTab) messagesCursor = page.nextCursor else feedbackCursor = page.nextCursor
    }

    suspend fun listen() {
        client.events().collect { event ->
            when (event) {
                is YuvaEvent.ConversationCreated -> upsert(event.conversation)
                is YuvaEvent.ConversationUpdated -> refresh(event.conversationId)
                is YuvaEvent.Read -> refresh(event.conversationId)
                is YuvaEvent.MessageCreated -> refresh(event.message.conversationId)
                is YuvaEvent.Presence -> runCatching { session = client.refreshSession() }
                is YuvaEvent.InboxUpdated -> session = session?.copy(inbox = event.inbox)
                is YuvaEvent.ResyncRequired -> load()
                else -> Unit
            }
        }
    }

    private suspend fun refresh(id: String) {
        runCatching { client.conversation(id) }.getOrNull()?.let(::upsert)
    }

    private fun upsert(conversation: YuvaConversation) {
        messages.removeAll { it.id == conversation.id }
        feedback.removeAll { it.id == conversation.id }
        val list = if (conversation.isFeedback) feedback else messages
        list.add(conversation)
        list.sortByDescending { it.lastMessageAt ?: it.createdAt }
    }
}

private sealed interface Route {
    data class Thread(val id: String?) : Route
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun YuvaConversationsView(
    client: YuvaClient,
    modifier: Modifier = Modifier,
    openConversationId: String? = null,
    onClose: (() -> Unit)? = null,
) {
    val state = remember(client) { ConversationsState(client) }
    var route by remember { mutableStateOf<Route?>(null) }
    var tab by remember { mutableStateOf(ConversationsTab.MESSAGES) }
    val scope = rememberCoroutineScope()
    LaunchedEffect(client) { state.load() }
    LaunchedEffect(client) { state.listen() }
    LaunchedEffect(openConversationId) { if (openConversationId != null) route = Route.Thread(openConversationId) }

    val current = route
    if (current is Route.Thread) {
        BackHandler { route = null }
        YuvaThreadView(client, current.id, modifier, onBack = { route = null })
        return
    }
    Scaffold(
        modifier = modifier,
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.yuva_conversations_title)) },
                navigationIcon = {
                    if (onClose != null) {
                        IconButton(onClick = onClose) { Icon(Icons.Default.Close, contentDescription = null) }
                    }
                },
                actions = {
                    IconButton(onClick = { route = Route.Thread(null) }, modifier = Modifier.testTag("yuva.conversations.new")) {
                        Icon(Icons.Default.Create, contentDescription = stringResource(R.string.yuva_conversations_new))
                    }
                },
            )
        },
    ) { padding ->
        Box(Modifier.fillMaxSize().padding(padding)) {
            when {
                state.loading -> CircularProgressIndicator(Modifier.align(Alignment.Center))
                state.failed -> Placeholder(
                    title = stringResource(R.string.yuva_error_generic),
                    action = stringResource(R.string.yuva_error_retry),
                    onAction = { scope.launch { state.load() } },
                )
                state.messages.isEmpty() && !state.hasFeedback -> Placeholder(
                    title = stringResource(R.string.yuva_conversations_empty),
                    hint = stringResource(R.string.yuva_conversations_empty_hint),
                    action = stringResource(R.string.yuva_conversations_new),
                    onAction = { route = Route.Thread(null) },
                )
                else -> Column(Modifier.fillMaxSize()) {
                    if (state.hasFeedback) {
                        SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp)) {
                            ConversationsTab.entries.forEachIndexed { index, option ->
                                SegmentedButton(
                                    selected = option == tab,
                                    onClick = { tab = option },
                                    shape = SegmentedButtonDefaults.itemShape(index, ConversationsTab.entries.size),
                                    modifier = Modifier.testTag("yuva.conversations.tab." + option.name.lowercase()),
                                ) {
                                    Text(
                                        stringResource(
                                            if (option == ConversationsTab.MESSAGES) R.string.yuva_conversations_messages
                                            else R.string.yuva_feedback_kind,
                                        ),
                                    )
                                }
                            }
                        }
                    }
                    val items = state.items(tab)
                    LazyColumn(Modifier.fillMaxSize()) {
                        if (tab == ConversationsTab.MESSAGES) {
                            state.session?.inbox?.let { inbox ->
                                item { PresenceHeader(inbox, Modifier.padding(16.dp)) }
                            }
                        }
                        items(items, key = { it.id }) { conversation ->
                            if (conversation.isFeedback) {
                                FeedbackRow(conversation) { route = Route.Thread(conversation.id) }
                            } else {
                                ConversationRow(conversation) { route = Route.Thread(conversation.id) }
                            }
                            HorizontalDivider()
                            if (conversation.id == items.lastOrNull()?.id) {
                                LaunchedEffect(conversation.id, tab) { state.loadMore(tab) }
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
internal fun Placeholder(
    title: String,
    hint: String? = null,
    action: String? = null,
    onAction: () -> Unit = {},
    secondaryAction: String? = null,
    onSecondaryAction: () -> Unit = {},
) {
    Column(
        Modifier.fillMaxSize().padding(32.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(title, style = MaterialTheme.typography.titleMedium, textAlign = TextAlign.Center)
        if (hint != null) {
            Spacer(Modifier.height(8.dp))
            Text(
                hint,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center,
            )
        }
        if (action != null) {
            Spacer(Modifier.height(16.dp))
            Button(onClick = onAction) { Text(action) }
        }
        if (secondaryAction != null) {
            TextButton(onClick = onSecondaryAction, modifier = Modifier.testTag("yuva.feedback.view")) { Text(secondaryAction) }
        }
    }
}

@Composable
internal fun PresenceHeader(inbox: YuvaInbox, modifier: Modifier = Modifier) {
    Row(modifier, verticalAlignment = Alignment.CenterVertically) {
        val members = inbox.presence?.members.orEmpty().take(3)
        members.forEach { member ->
            Avatar(member.initials, 26.dp)
            Spacer(Modifier.width(4.dp))
        }
        if (members.isNotEmpty()) Spacer(Modifier.width(6.dp))
        Column {
            Text(inbox.name, style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
            Text(
                presenceText(inbox),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
internal fun presenceText(inbox: YuvaInbox): String {
    if (inbox.mode == YuvaInboxMode.LIVE) {
        return stringResource(if (inbox.presence?.available == true) R.string.yuva_presence_online else R.string.yuva_presence_away)
    }
    val minutes = inbox.expectedReplyMinutes ?: return stringResource(R.string.yuva_presence_away)
    return stringResource(R.string.yuva_presence_reply_in, minutes)
}

@Composable
private fun ConversationRow(conversation: YuvaConversation, onClick: () -> Unit) {
    val title = when {
        conversation.subject.isNotEmpty() -> conversation.subject
        conversation.feedback != null -> stringResource(R.string.yuva_feedback_kind) + " · " +
            stringResource(categoryLabel(conversation.feedback.category))
        else -> stringResource(R.string.yuva_thread_title)
    }
    val last = conversation.lastMessage
    val preview = when {
        last == null -> ""
        last.authorType == YuvaAuthorType.CONTACT -> stringResource(R.string.yuva_thread_you) + ": " + last.text
        else -> last.text
    }
    Column(Modifier.fillMaxWidth().clickable(onClick = onClick).padding(horizontal = 16.dp, vertical = 12.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                title,
                style = MaterialTheme.typography.titleMedium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f),
            )
            Text(
                DateUtils.getRelativeTimeSpanString((conversation.lastMessageAt ?: conversation.createdAt).toEpochMilli()).toString(),
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                preview,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f),
            )
            if (conversation.unread) {
                Box(Modifier.padding(start = 8.dp).size(10.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primary))
            }
        }
        if (conversation.status == YuvaConversationStatus.CLOSED) {
            Text(
                stringResource(R.string.yuva_status_closed),
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
internal fun BackButton(onBack: () -> Unit) {
    IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = null) }
}

internal fun categoryLabel(category: dev.yuva.YuvaFeedbackCategory): Int = when (category) {
    dev.yuva.YuvaFeedbackCategory.BUG -> R.string.yuva_feedback_category_bug
    dev.yuva.YuvaFeedbackCategory.IDEA -> R.string.yuva_feedback_category_idea
    dev.yuva.YuvaFeedbackCategory.PRAISE -> R.string.yuva_feedback_category_praise
    dev.yuva.YuvaFeedbackCategory.OTHER -> R.string.yuva_feedback_category_other
}


internal fun stateLabel(state: YuvaFeedbackState): Int = when (state) {
    YuvaFeedbackState.SENT -> R.string.yuva_feedback_state_sent
    YuvaFeedbackState.ANSWERED -> R.string.yuva_feedback_state_answered
    YuvaFeedbackState.RESOLVED -> R.string.yuva_feedback_state_resolved
}

@Composable
private fun FeedbackRow(conversation: YuvaConversation, onClick: () -> Unit) {
    val title = conversation.feedback?.let { stringResource(categoryLabel(it.category)) }
        ?: stringResource(R.string.yuva_feedback_kind)
    val text = conversation.feedback?.text?.takeIf { it.isNotEmpty() }
        ?: conversation.subject.ifEmpty { conversation.lastMessage?.text.orEmpty() }
    val firstLine = text.lineSequence().firstOrNull().orEmpty()
    Column(Modifier.fillMaxWidth().clickable(onClick = onClick).padding(horizontal = 16.dp, vertical = 12.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                title,
                style = MaterialTheme.typography.titleMedium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f),
            )
            Text(
                DateUtils.getRelativeTimeSpanString((conversation.lastMessageAt ?: conversation.createdAt).toEpochMilli()).toString(),
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                firstLine,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f),
            )
            if ((conversation.feedback?.attachmentCount ?: 0) > 0) {
                val label = stringResource(R.string.yuva_feedback_attachments)
                Text(
                    "\uD83D\uDCCE",
                    style = MaterialTheme.typography.labelMedium,
                    modifier = Modifier.padding(start = 8.dp).semantics { contentDescription = label },
                )
            }
            if (conversation.unread) {
                Box(Modifier.padding(start = 8.dp).size(10.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primary))
            }
        }
        conversation.feedbackState?.let { state ->
            Text(
                stringResource(stateLabel(state)),
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.testTag("yuva.feedback.state." + state.name.lowercase()),
            )
        }
    }
}

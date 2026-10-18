import SwiftUI

enum ConversationsTab: Hashable {
    case messages, feedback
}

@MainActor
@Observable
final class ConversationsModel {
    let client: YuvaClient
    var messages: [YuvaConversation] = []
    var feedback: [YuvaConversation] = []
    var session: YuvaSession?
    var loading = false
    var failed = false
    private var messagesCursor: String?
    private var feedbackCursor: String?

    init(client: YuvaClient) {
        self.client = client
    }

    var hasFeedback: Bool { !feedback.isEmpty }

    func items(_ tab: ConversationsTab) -> [YuvaConversation] {
        tab == .messages ? messages : feedback
    }

    func load() async {
        loading = messages.isEmpty && feedback.isEmpty
        defer { loading = false }
        do {
            session = try await client.start()
            async let sent = client.conversations(kind: [.feedback])
            let page = try await client.conversations(kind: [.conversation])
            let sentPage = try await sent
            messages = page.items
            messagesCursor = page.nextCursor
            feedback = sentPage.items
            feedbackCursor = sentPage.nextCursor
            failed = false
        } catch {
            failed = messages.isEmpty && feedback.isEmpty
        }
    }

    func loadMore(_ tab: ConversationsTab) async {
        guard let cursor = tab == .messages ? messagesCursor : feedbackCursor else { return }
        setCursor(nil, tab)
        let kind: YuvaConversationKind = tab == .messages ? .conversation : .feedback
        guard let page = try? await client.conversations(kind: [kind], cursor: cursor) else {
            setCursor(cursor, tab)
            return
        }
        let known = Set(items(tab).map(\.id))
        let added = page.items.filter { !known.contains($0.id) }
        if tab == .messages { messages += added } else { feedback += added }
        setCursor(page.nextCursor, tab)
    }

    private func setCursor(_ cursor: String?, _ tab: ConversationsTab) {
        if tab == .messages { messagesCursor = cursor } else { feedbackCursor = cursor }
    }

    func listen() async {
        for await event in client.events() {
            switch event {
            case .conversationCreated(let conversation):
                upsert(conversation)
            case .conversationUpdated(let id, _, _, _), .read(let id, _):
                await refresh(id)
            case .messageCreated(let message):
                await refresh(message.conversationId)
            case .presence:
                session = try? await client.refreshSession()
            case .inboxUpdated:
                session = await client.session
            case .resyncRequired:
                await load()
            default:
                break
            }
        }
    }

    private func refresh(_ id: String) async {
        if let conversation = try? await client.conversation(id: id) { upsert(conversation) }
    }

    private func upsert(_ conversation: YuvaConversation) {
        messages.removeAll { $0.id == conversation.id }
        feedback.removeAll { $0.id == conversation.id }
        if conversation.isFeedback {
            feedback.append(conversation)
            feedback.sort { ($0.lastMessageAt ?? $0.createdAt) > ($1.lastMessageAt ?? $1.createdAt) }
        } else {
            messages.append(conversation)
            messages.sort { ($0.lastMessageAt ?? $0.createdAt) > ($1.lastMessageAt ?? $1.createdAt) }
        }
    }
}

enum ThreadRoute: Hashable {
    case new
    case conversation(String)

    var id: String? {
        if case .conversation(let id) = self { return id }
        return nil
    }
}

public struct YuvaConversationsView: View {
    private let client: YuvaClient
    @State private var model: ConversationsModel
    @State private var path: [ThreadRoute] = []
    @State private var tab: ConversationsTab = .messages
    @Binding private var openConversationId: String?

    public init(client: YuvaClient, openConversationId: Binding<String?> = .constant(nil)) {
        self.client = client
        _model = State(initialValue: ConversationsModel(client: client))
        _openConversationId = openConversationId
    }

    public var body: some View {
        NavigationStack(path: $path) {
            content
                .navigationTitle(L.t("conversations.title"))
                .toolbar {
                    ToolbarItem(placement: .primaryAction) {
                        Button {
                            path.append(.new)
                        } label: {
                            Label(L.t("conversations.new"), systemImage: "square.and.pencil")
                        }
                        .accessibilityIdentifier("yuva.conversations.new")
                    }
                }
                .navigationDestination(for: ThreadRoute.self) { route in
                    YuvaThreadView(client: client, conversationId: route.id)
                }
        }
        .task { await model.load() }
        .task { await model.listen() }
        .onChange(of: openConversationId, initial: true) { _, id in
            guard let id else { return }
            path = [.conversation(id)]
            openConversationId = nil
        }
    }

    @ViewBuilder
    private var content: some View {
        if model.loading {
            ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
        } else if model.failed {
            ContentUnavailableView {
                Label(L.t("error.generic"), systemImage: "wifi.exclamationmark")
            } actions: {
                Button(L.t("error.retry")) { Task { await model.load() } }
            }
        } else if model.messages.isEmpty && !model.hasFeedback {
            ContentUnavailableView {
                Label(L.t("conversations.empty"), systemImage: "bubble.left.and.bubble.right")
            } description: {
                Text(L.t("conversations.emptyHint"))
            } actions: {
                Button(L.t("conversations.new")) { path.append(.new) }
                    .buttonStyle(.borderedProminent)
            }
        } else {
            let items = model.items(tab)
            List {
                if model.hasFeedback {
                    Picker("", selection: $tab) {
                        Text(L.t("conversations.messages")).tag(ConversationsTab.messages)
                        Text(L.t("feedback.kind")).tag(ConversationsTab.feedback)
                    }
                    .pickerStyle(.segmented)
                    .labelsHidden()
                    .listRowSeparator(.hidden)
                    .accessibilityIdentifier("yuva.conversations.tabs")
                }
                if tab == .messages, let inbox = model.session?.inbox {
                    PresenceHeader(inbox: inbox)
                }
                ForEach(items) { conversation in
                    NavigationLink(value: ThreadRoute.conversation(conversation.id)) {
                        if conversation.isFeedback {
                            FeedbackRow(conversation: conversation)
                        } else {
                            ConversationRow(conversation: conversation)
                        }
                    }
                    .onAppear {
                        if conversation.id == items.last?.id { Task { await model.loadMore(tab) } }
                    }
                }
            }
            .refreshable { await model.load() }
        }
    }
}

struct PresenceHeader: View {
    let inbox: YuvaInbox

    var body: some View {
        HStack(spacing: 10) {
            if let members = inbox.presence?.members, !members.isEmpty {
                HStack(spacing: -8) {
                    ForEach(Array(members.prefix(3).enumerated()), id: \.offset) { _, member in
                        AvatarView(initials: member.initials, size: 26)
                            .overlay(Circle().stroke(.background, lineWidth: 2))
                    }
                }
            }
            VStack(alignment: .leading, spacing: 2) {
                Text(inbox.name).font(.subheadline.weight(.semibold))
                Text(PresenceHeader.status(inbox)).font(.caption).foregroundStyle(.secondary)
            }
        }
    }

    static func status(_ inbox: YuvaInbox) -> String {
        if inbox.mode == .live {
            return inbox.presence?.available == true ? L.t("presence.online") : L.t("presence.away")
        }
        if let minutes = inbox.expectedReplyMinutes { return L.f("presence.replyIn", Int32(minutes)) }
        return L.t("presence.away")
    }
}

struct ConversationRow: View {
    let conversation: YuvaConversation

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(title).font(.headline).lineLimit(1)
                Spacer()
                if let date = conversation.lastMessageAt ?? Optional(conversation.createdAt) {
                    Text(date, format: .relative(presentation: .named)).font(.caption).foregroundStyle(.secondary)
                }
            }
            HStack {
                Text(preview).font(.subheadline).foregroundStyle(.secondary).lineLimit(2)
                Spacer()
                if conversation.unread {
                    Circle().fill(Color.accentColor).frame(width: 10, height: 10)
                        .accessibilityLabel(L.t("conversations.unread"))
                }
            }
            if conversation.status == .closed {
                Text(L.t("status.closed")).font(.caption2).foregroundStyle(.secondary)
            }
        }
        .padding(.vertical, 2)
    }

    private var title: String {
        if !conversation.subject.isEmpty { return conversation.subject }
        if let feedback = conversation.feedback {
            return L.t("feedback.kind") + " · " + L.t("feedback.category." + feedback.category.rawValue)
        }
        return L.t("thread.title")
    }

    private var preview: String {
        guard let last = conversation.lastMessage else { return "" }
        return last.authorType == .contact ? L.t("thread.you") + ": " + last.text : last.text
    }
}

extension YuvaFeedbackState {
    var label: String { L.t("feedback.state." + rawValue) }
}

struct FeedbackRow: View {
    let conversation: YuvaConversation

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                if let feedback = conversation.feedback {
                    Text(L.t("feedback.category." + feedback.category.rawValue)).font(.headline).lineLimit(1)
                } else {
                    Text(L.t("feedback.kind")).font(.headline).lineLimit(1)
                }
                Spacer()
                Text(conversation.lastMessageAt ?? conversation.createdAt, format: .relative(presentation: .named))
                    .font(.caption).foregroundStyle(.secondary)
            }
            HStack {
                Text(firstLine).font(.subheadline).foregroundStyle(.secondary).lineLimit(1)
                Spacer()
                if (conversation.feedback?.attachmentCount ?? 0) > 0 {
                    Image(systemName: "paperclip").font(.caption).foregroundStyle(.secondary)
                        .accessibilityLabel(L.t("feedback.attachments"))
                }
                if conversation.unread {
                    Circle().fill(Color.accentColor).frame(width: 10, height: 10)
                        .accessibilityLabel(L.t("conversations.unread"))
                }
            }
            if let state = conversation.feedbackState {
                Text(state.label).font(.caption2.weight(.medium)).foregroundStyle(.secondary)
                    .accessibilityIdentifier("yuva.feedback.state." + state.rawValue)
            }
        }
        .padding(.vertical, 2)
    }

    private var firstLine: String {
        let own = conversation.feedback?.text ?? ""
        let text = !own.isEmpty ? own : (conversation.subject.isEmpty ? (conversation.lastMessage?.text ?? "") : conversation.subject)
        return text.split(whereSeparator: \.isNewline).first.map(String.init) ?? text
    }
}

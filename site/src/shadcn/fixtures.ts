import type {
  ClientAttachment,
  ClientContact,
  ClientConversation,
  ClientInbox,
  ClientMailingList,
  ClientRoadmap,
  IdeaStage,
  ClientMessage,
  ClientRealtimeMessage,
  Discussion,
  DiscussionAuthor,
  DiscussionPost,
  YuvaClient,
} from "useyuva"

const minutes = (n: number) => new Date(Date.now() - n * 60_000).toISOString()
let counter = 0
const nextId = (prefix: string) => `${prefix}_${++counter}`
const wait = <T>(value: T, ms = 120) => new Promise<T>((resolve) => setTimeout(() => resolve(value), ms))

const inbox: ClientInbox = {
  id: "inb_demo",
  name: "Acme Support",
  branding: {},
  default_locale: "en",
  timezone: "UTC",
  mode: "live",
  business_hours: { enabled: false, intervals: [] },
  open_now: true,
  expected_reply_minutes: 5,
  presence: { available: true, members: [{ name: "Maya Chen", initials: "MC" }] },
  chat: {
    greeting: "Hi, ask us anything about Acme.",
    launcher_position: "bottom-right",
    ask_email_offline: false,
    allow_anonymous: true,
    discussions: "open",
  },
  feedback_categories: [],
  ask_for_rating: true,
} as ClientInbox

const contact: ClientContact = { id: "con_demo", name: "You", identified: false }
const maya = { type: "member", name: "Maya Chen", initials: "MC" } as const
const you = { type: "contact", name: "You", initials: "Y" } as const

type Listener = (value: never) => void

class Emitter {
  #listeners = new Map<string, Set<Listener>>()
  on(type: string, handler: Listener) {
    const set = this.#listeners.get(type) ?? new Set()
    set.add(handler)
    this.#listeners.set(type, set)
    return () => void set.delete(handler)
  }
  emit(type: string, value: unknown) {
    for (const handler of this.#listeners.get(type) ?? []) handler(value as never)
  }
}

function message(
  conversation: string,
  direction: "in" | "out",
  body: string,
  ago: number,
  attachments: ClientAttachment[] = []
): ClientMessage {
  return {
    id: nextId("msg"),
    conversation_id: conversation,
    direction,
    author: direction === "in" ? you : maya,
    body,
    attachments,
    created_at: minutes(ago),
  }
}

function previewOf(m: ClientMessage): ClientConversation["last_message"] {
  return { id: m.id, author_type: m.author.type, text: m.body, created_at: m.created_at }
}

export function createChatFixture(): YuvaClient {
  const events = new Emitter()
  const invoice: ClientAttachment = {
    id: "att_demo",
    filename: "invoice-0412.pdf",
    content_type: "application/pdf",
    size: 48213,
    inline: false,
  }
  const threads = new Map<string, ClientMessage[]>([
    [
      "conv_billing",
      [
        message("conv_billing", "in", "Hi, I was charged twice this month.", 42),
        message("conv_billing", "out", "Sorry about that. Could you send me the invoice number?", 40),
        message("conv_billing", "in", "Here it is.", 38, [invoice]),
        message("conv_billing", "out", "Found it. I refunded the duplicate charge, it shows up in 3 to 5 days.", 35),
      ],
    ],
    [
      "conv_export",
      [
        message("conv_export", "in", "Can I export my contacts as CSV?", 3000),
        message("conv_export", "out", "Yes: Contacts, then Export. It is mailed to you.", 2990),
      ],
    ],
  ])
  const conversations: ClientConversation[] = [
    {
      id: "conv_billing",
      kind: "conversation",
      subject: "Charged twice this month",
      status: "open",
      last_message: previewOf(threads.get("conv_billing")!.at(-1)!),
      last_message_at: minutes(35),
      unread: true,
      can_rate: false,
      created_at: minutes(42),
    },
    {
      id: "conv_export",
      kind: "conversation",
      subject: "Exporting contacts",
      status: "closed",
      last_message: previewOf(threads.get("conv_export")!.at(-1)!),
      last_message_at: minutes(2990),
      unread: false,
      can_rate: true,
      created_at: minutes(3000),
    },
  ]
  const sorted = () =>
    [...conversations].sort((a, b) => Date.parse(b.last_message_at!) - Date.parse(a.last_message_at!))
  const push = (m: ClientMessage) => {
    threads.get(m.conversation_id)!.push(m)
    const at = conversations.find((c) => c.id === m.conversation_id)!
    at.last_message = previewOf(m)
    at.last_message_at = m.created_at
    events.emit("event", { type: "message.created", data: m } as ClientRealtimeMessage)
  }
  const replyLater = (conversationId: string) => {
    setTimeout(() => {
      events.emit("event", {
        type: "typing",
        data: { conversation_id: conversationId, typing: true, author: maya },
      } as ClientRealtimeMessage)
    }, 600)
    setTimeout(() => {
      push(message(conversationId, "out", "Thanks, a teammate is looking at this now.", 0))
    }, 2200)
  }

  const client = {
    server: "https://fixture.invalid",
    channel: "fixture",
    on: (type: string, handler: Listener) => events.on(type, handler),
    start: () => wait({ expires_at: minutes(-60), contact, inbox } as Awaited<ReturnType<YuvaClient["start"]>>),
    connect: async () => {
      events.emit("connection", { state: "open", attempt: 0 })
    },
    disconnect: () => undefined,
    listConversations: () => wait({ items: sorted() }),
    getConversation: (id: string) => wait(conversations.find((c) => c.id === id)!),
    listMessages: (id: string) => wait({ items: [...threads.get(id)!].reverse() }),
    sendMessage: async (id: string, input: { body?: string; client_id?: string }) => {
      const sent = { ...message(id, "in", input.body ?? "", 0), client_id: input.client_id }
      await wait(null, 250)
      push(sent)
      replyLater(id)
      return sent
    },
    startConversation: async (input: { body?: string; subject?: string; client_id?: string }) => {
      const id = nextId("conv")
      threads.set(id, [])
      const first = { ...message(id, "in", input.body ?? "", 0), client_id: input.client_id }
      const created: ClientConversation = {
        id,
        kind: "conversation",
        subject: input.subject ?? (input.body ?? "").slice(0, 40),
        status: "open",
        unread: false,
        can_rate: false,
        created_at: first.created_at,
        last_message_at: first.created_at,
      }
      conversations.push(created)
      await wait(null, 250)
      push(first)
      replyLater(id)
      return { conversation: created, message: first }
    },
    markRead: (id: string) => wait({ conversation_id: id, unread: false }),
    rate: async (id: string, rating: "good" | "bad") => {
      const at = conversations.find((c) => c.id === id)!
      Object.assign(at, { rating, can_rate: false })
      return wait(at)
    },
    setEmail: async (email: string) =>
      wait({
        ...contact,
        typed_email: email,
        email_confirmation: { state: "sent", email, expires_at: new Date(Date.now() + 86_400_000).toISOString() },
      }),
    setTyping: async () => undefined,
    attachment: async () => new Blob(["fixture"], { type: "application/pdf" }),
  }
  return client as unknown as YuvaClient
}

const authors: Record<string, DiscussionAuthor> = {
  sam: { kind: "reader", name: "Sam Rivera", initials: "SR", deleted: false },
  maya: { kind: "member", name: "Maya Chen", initials: "MC", deleted: false },
  ada: { kind: "reader", name: "Ada Lovelace", initials: "AL", deleted: false },
}

function post(
  discussion: string,
  id: string,
  body: string,
  author: DiscussionAuthor,
  ago: number,
  extra: Partial<DiscussionPost> = {}
): DiscussionPost {
  return {
    id,
    discussion_id: discussion,
    opening: false,
    body,
    body_html: `<p>${body}</p>`,
    author,
    state: "published",
    votes: 0,
    accepted: false,
    deleted: false,
    created_at: minutes(ago),
    ...extra,
  }
}

function seedDiscussions() {
  const list: Discussion[] = []
  const posts = new Map<string, DiscussionPost[]>()
  const add = (
    id: string,
    title: string,
    body: string,
    author: DiscussionAuthor,
    ago: number,
    votes: number,
    replies: DiscussionPost[],
    extra: Partial<Discussion> = {}
  ) => {
    const opening = post(id, `${id}_open`, body, author, ago, { opening: true, votes })
    list.push({
      id,
      page: "/docs/getting-started",
      page_title: "Getting started",
      title,
      opening_post_id: opening.id,
      body,
      body_html: opening.body_html,
      author,
      state: "published",
      locked: false,
      votes,
      reply_count: replies.length,
      created_at: opening.created_at,
      last_activity_at: replies.at(-1)?.created_at ?? opening.created_at,
      ...extra,
    })
    posts.set(id, [opening, ...replies])
  }
  add(
    "d_key",
    "How do I rotate the channel key?",
    "I pasted the key into a public repo by mistake. Can I rotate it without breaking the live site?",
    authors.sam!,
    300,
    12,
    [
      post(
        "d_key",
        "d_key_1",
        "The key is public by design, but you can create a second channel and swap it in.",
        authors.ada!,
        280,
        { votes: 2 }
      ),
      post(
        "d_key",
        "d_key_2",
        "Create the new channel, deploy with its key, then delete the old channel. Nothing else changes.",
        authors.maya!,
        240,
        { votes: 7, accepted: true }
      ),
    ],
    { answer_post_id: "d_key_2", answered_at: minutes(240) }
  )
  add(
    "d_fonts",
    "Can the composer use my own font?",
    "The preview tab looks different from my site. Is there a way to match the typography?",
    authors.ada!,
    1500,
    4,
    [
      post(
        "d_fonts",
        "d_fonts_1",
        "It reads your theme tokens, so the font comes from your own CSS.",
        authors.maya!,
        1400,
        { votes: 1 }
      ),
    ]
  )
  add("d_lock", "Dark mode in the preview", "Does the Markdown preview follow dark mode?", authors.sam!, 4000, 1, [], {
    locked: true,
  })
  return { list, posts }
}

export function createDiscussionsFixture(): YuvaClient {
  const events = new Emitter()
  const { list, posts } = seedDiscussions()
  const reader = {
    token: "fixture",
    expires_at: minutes(-60),
    reader: { name: "Ada Lovelace", initials: "AL" },
  }
  const find = (id: string) => list.find((d) => d.id === id)!
  const sorters = {
    new: (a: Discussion, b: Discussion) => Date.parse(b.created_at) - Date.parse(a.created_at),
    top: (a: Discussion, b: Discussion) => b.votes - a.votes,
    unanswered: (a: Discussion, b: Discussion) => Date.parse(b.created_at) - Date.parse(a.created_at),
  }
  const client = {
    server: "https://fixture.invalid",
    channel: "fixture",
    reader,
    on: (type: string, handler: Listener) => events.on(type, handler),
    channelSettings: () => wait(inbox),
    discussionWriter: () => wait("reader" as const),
    readerSignIn: async () => reader,
    readerSignOut: async () => undefined,
    ratePage: () => wait(undefined, 300),
    listDiscussions: (_page: string, options: { sort?: keyof typeof sorters } = {}) => {
      const sort = options.sort ?? "new"
      const items = list.filter((d) => sort !== "unanswered" || !d.answer_post_id).sort(sorters[sort])
      return wait({ items: items.map((d) => ({ ...d })) })
    },
    getDiscussion: (id: string) => wait({ discussion: { ...find(id) }, posts: posts.get(id)!.map((p) => ({ ...p })) }),
    startDiscussion: async (input: { title: string; body: string; page_url: string; page_title?: string }) => {
      const id = nextId("d")
      const opening = post(id, `${id}_open`, input.body, authors.ada!, 0, { opening: true, mine: true })
      const created: Discussion = {
        id,
        page: input.page_url,
        page_title: input.page_title,
        title: input.title,
        opening_post_id: opening.id,
        body: input.body,
        body_html: opening.body_html,
        author: authors.ada!,
        state: "published",
        locked: false,
        votes: 0,
        reply_count: 0,
        mine: true,
        created_at: opening.created_at,
        last_activity_at: opening.created_at,
      }
      list.push(created)
      posts.set(id, [opening])
      return wait({ ...created })
    },
    replyToDiscussion: async (id: string, input: { body: string; parent_post_id?: string }) => {
      const reply = post(id, nextId("post"), input.body, authors.ada!, 0, {
        mine: true,
        parent_post_id: input.parent_post_id,
      })
      posts.get(id)!.push(reply)
      find(id).reply_count++
      return wait({ ...reply })
    },
    editDiscussionPost: async (postId: string, input: { body: string }) => {
      const at = [...posts.values()].flat().find((p) => p.id === postId)!
      Object.assign(at, { body: input.body, body_html: `<p>${input.body}</p>`, edited_at: minutes(0) })
      return wait({ ...at })
    },
    deleteDiscussionPost: async (postId: string) => {
      const at = [...posts.values()].flat().find((p) => p.id === postId)!
      at.deleted = true
    },
    voteDiscussionPost: async (postId: string, voted = true) => {
      const at = [...posts.values()].flat().find((p) => p.id === postId)!
      if (!!at.voted !== voted) at.votes += voted ? 1 : -1
      at.voted = voted
      if (at.opening) Object.assign(find(at.discussion_id), { votes: at.votes, voted })
      return wait({ ...at })
    },
    followDiscussion: async (id: string, following = true) => {
      find(id).following = following
    },
    acceptDiscussionAnswer: async (id: string, postId: string | null) => {
      for (const p of posts.get(id)!) p.accepted = p.id === postId
      Object.assign(find(id), { answer_post_id: postId ?? undefined })
      return wait({ ...find(id) })
    },
  }
  return client as unknown as YuvaClient
}

const lists: ClientMailingList[] = [
  {
    id: "list_updates",
    name: "Product updates",
    description: "New features and fixes, about once a month.",
    double_opt_in: true,
  },
  { id: "list_blog", name: "Engineering blog", description: "Long reads from the team.", double_opt_in: true },
]

export function createSubscribeFixture(): YuvaClient {
  const client = {
    server: "https://fixture.invalid",
    channel: "fixture",
    on: () => () => undefined,
    channelSettings: () => wait(inbox),
    lists: () => wait({ lists, form_token: "fixture" }),
    subscribe: () => wait({ confirmation: true }, 400),
    subscriptions: () => wait({ items: [] }),
    subscribeList: () => wait({ state: "subscribed" as const }, 300),
    unsubscribeList: () => wait(undefined, 300),
  }
  return client as unknown as YuvaClient
}

const helpCollections = [
  {
    id: "hc_start",
    number: 1,
    icon: "rocket",
    name: "Getting started",
    description: "Set up your account in a few minutes.",
    article_count: 2,
    url: "https://fixture.invalid/help/acme/en/collections/1-getting-started",
  },
  {
    id: "hc_billing",
    number: 2,
    icon: "billing",
    name: "Billing",
    description: "Invoices, plans and payment methods.",
    article_count: 1,
    url: "https://fixture.invalid/help/acme/en/collections/2-billing",
  },
]

const helpArticles = [
  {
    id: "ha_invite",
    number: 1,
    locale: "en",
    title: "Invite your team",
    excerpt: "Add teammates from Settings, then pick the inboxes each of them may see.",
    url: "https://fixture.invalid/help/acme/en/articles/1-invite-your-team",
    collection: { id: "hc_start", name: "Getting started" },
    body_html:
      "<p>Add teammates from <strong>Settings</strong>, then pick the inboxes each of them may see.</p><h2>Roles</h2><ul><li>Owners and admins change settings.</li><li>Agents answer conversations.</li></ul>",
  },
  {
    id: "ha_export",
    number: 2,
    locale: "en",
    title: "Export your contacts",
    excerpt: "Open Contacts, choose Export, and the file is mailed to you.",
    url: "https://fixture.invalid/help/acme/en/articles/2-export-your-contacts",
    collection: { id: "hc_start", name: "Getting started" },
    body_html: "<p>Open <strong>Contacts</strong>, choose <strong>Export</strong>, and the file is mailed to you.</p>",
  },
  {
    id: "ha_invoice",
    number: 3,
    locale: "en",
    title: "Download an invoice",
    excerpt: "Invoices are under Billing. Pick a month and download the PDF.",
    url: "https://fixture.invalid/help/acme/en/articles/3-download-an-invoice",
    collection: { id: "hc_billing", name: "Billing" },
    body_html: "<p>Invoices are under <strong>Billing</strong>. Pick a month and download the PDF.</p>",
  },
]

const helpSummary = ({ body_html: _, ...summary }: (typeof helpArticles)[number]) => summary

export function createHelpFixture(): YuvaClient {
  const help = {
    get: () =>
      wait({
        center: {
          name: "Acme help",
          description: "Answers to common questions.",
          locale: "en",
          locales: ["en"],
          default_locale: "en",
          url: "https://fixture.invalid/help/acme/en/",
        },
        collections: helpCollections,
        suggested: [],
        popular: helpArticles.map(helpSummary),
      }),
    search: (q: string) => {
      const words = q.toLowerCase().split(/\s+/).filter(Boolean)
      const items = helpArticles
        .filter((a) => words.some((w) => `${a.title} ${a.excerpt}`.toLowerCase().includes(w)))
        .map((a) => ({
          ...helpSummary(a),
          highlights: words.flatMap((w) => {
            const start = a.title.toLowerCase().indexOf(w)
            return start < 0 ? [] : [{ field: "title", start, end: start + w.length }]
          }),
        }))
      return wait({ items, partial: false }, 200)
    },
    collection: (id: string) => {
      const collection = helpCollections.find((c) => c.id === id)!
      return wait({
        collection,
        collections: [],
        articles: helpArticles.filter((a) => a.collection.id === id).map(helpSummary),
      })
    },
    article: (id: string) => {
      const found = helpArticles.find((a) => a.id === id)!
      return wait({
        ...found,
        fallback: false,
        updated_at: minutes(60 * 24 * 3),
        related: helpArticles.filter((a) => a.id !== id).map(helpSummary),
      })
    },
    react: () => wait(undefined, 200),
    click: () => wait(undefined, 50),
  }
  const client = {
    server: "https://fixture.invalid",
    channel: "fixture",
    on: () => () => undefined,
    help,
  }
  return client as unknown as YuvaClient
}

const ideaSeed: { id: string; title: string; body: string; stage: IdeaStage; votes: number; ago: number }[] = [
  {
    id: "i_dark",
    title: "Dark mode for the panel",
    body: "The panel is bright at night. A dark theme that follows the system setting would help.",
    stage: "in_progress",
    votes: 42,
    ago: 9000,
  },
  {
    id: "i_csv",
    title: "Export conversations as CSV",
    body: "We need a monthly export of conversations with their labels for our own reports.",
    stage: "planned",
    votes: 27,
    ago: 7000,
  },
  {
    id: "i_slack",
    title: "Slack notifications",
    body: "Post new conversations of an inbox into a Slack channel.",
    stage: "ideas",
    votes: 18,
    ago: 5000,
  },
  {
    id: "i_sso",
    title: "Single sign-on for the team",
    body: "Let the team sign in with our identity provider.",
    stage: "ideas",
    votes: 9,
    ago: 2000,
  },
  {
    id: "i_tags",
    title: "Bulk edit labels",
    body: "Select conversations and add or remove a label for all of them at once.",
    stage: "shipped",
    votes: 33,
    ago: 20000,
  },
]

export function createRoadmapFixture(): YuvaClient {
  const base = createDiscussionsFixture() as unknown as Record<string, unknown>
  const ideas: Discussion[] = []
  const posts = new Map<string, DiscussionPost[]>()
  const add = (seed: (typeof ideaSeed)[number], author: DiscussionAuthor) => {
    const opening = post(seed.id, `${seed.id}_open`, seed.body, author, seed.ago, { opening: true, votes: seed.votes })
    ideas.push({
      id: seed.id,
      page: "",
      title: seed.title,
      opening_post_id: opening.id,
      body: seed.body,
      body_html: opening.body_html,
      author,
      state: "published",
      locked: false,
      votes: seed.votes,
      reply_count: 0,
      kind: "idea",
      stage: seed.stage,
      created_at: opening.created_at,
      last_activity_at: opening.created_at,
    } as Discussion)
    posts.set(seed.id, [opening])
  }
  ideaSeed.forEach((seed, index) => add(seed, index % 2 ? authors.sam! : authors.ada!))
  posts
    .get("i_dark")!
    .push(
      post("i_dark", "i_dark_1", "We start with the inbox and the settings. Conversations follow.", authors.maya!, 4000)
    )
  ideas.find((i) => i.id === "i_dark")!.reply_count = 1
  const count = (stage: IdeaStage) => ideas.filter((i) => i.stage === stage).length
  const board = (): ClientRoadmap =>
    ({
      name: "Acme roadmap",
      description: "What we are building, and what you asked for.",
      url: "https://fixture.invalid/roadmap/acme",
      suggestions: "open",
      stages: {
        ideas: count("ideas"),
        planned: count("planned"),
        in_progress: count("in_progress"),
        shipped: count("shipped"),
      },
    }) as ClientRoadmap
  const find = (id: string) => ideas.find((i) => i.id === id)
  const client = {
    ...base,
    roadmap: () => wait(board()),
    listIdeas: (options: { stage?: IdeaStage; sort?: "top" | "new"; q?: string } = {}) => {
      const words = (options.q ?? "").toLowerCase().split(/\s+/).filter(Boolean)
      const items = ideas
        .filter((i) => !options.stage || i.stage === options.stage)
        .filter((i) => words.every((w) => i.title.toLowerCase().includes(w)))
        .sort((a, b) =>
          options.sort === "new" ? Date.parse(b.created_at) - Date.parse(a.created_at) : b.votes - a.votes
        )
      return wait({ items: items.map((i) => ({ ...i })) })
    },
    suggestIdea: async (input: { title: string; body: string }) => {
      const id = nextId("i")
      const opening = post(id, `${id}_open`, input.body, authors.ada!, 0, { opening: true, mine: true })
      const created = {
        id,
        page: "",
        title: input.title,
        opening_post_id: opening.id,
        body: input.body,
        body_html: opening.body_html,
        author: authors.ada!,
        state: "published",
        locked: false,
        votes: 0,
        reply_count: 0,
        kind: "idea",
        stage: "ideas",
        mine: true,
        created_at: opening.created_at,
        last_activity_at: opening.created_at,
      } as Discussion
      ideas.unshift(created)
      posts.set(id, [opening])
      return wait({ ...created })
    },
    getDiscussion: (id: string) => {
      const idea = find(id)
      if (!idea) return (base.getDiscussion as (id: string) => Promise<unknown>)(id)
      return wait({ discussion: { ...idea }, posts: posts.get(id)!.map((p) => ({ ...p })) })
    },
    voteDiscussionPost: async (postId: string, voted = true) => {
      const at = [...posts.values()].flat().find((p) => p.id === postId)
      if (!at) return (base.voteDiscussionPost as (id: string, voted: boolean) => Promise<unknown>)(postId, voted)
      if (!!at.voted !== voted) at.votes += voted ? 1 : -1
      at.voted = voted
      Object.assign(find(at.discussion_id)!, { votes: at.votes, voted })
      return wait({ ...at })
    },
    replyToDiscussion: async (id: string, input: { body: string; parent_post_id?: string }) => {
      const idea = find(id)
      if (!idea) return (base.replyToDiscussion as (id: string, input: unknown) => Promise<unknown>)(id, input)
      const reply = post(id, nextId("post"), input.body, authors.ada!, 0, {
        mine: true,
        parent_post_id: input.parent_post_id,
      })
      posts.get(id)!.push(reply)
      idea.reply_count++
      return wait({ ...reply })
    },
  }
  return client as unknown as YuvaClient
}

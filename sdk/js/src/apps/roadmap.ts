import { msg } from "@lingui/core/macro"
import { ApiError, type ClientRoadmap, type Discussion, type DiscussionDetail, type IdeaStage } from "../client/api"
import { el, textButton } from "../panel/helpers"
import { icons } from "../panel/icons"
import type { AppContext } from "../types"
import type { Translate } from "./apps"
import { article } from "./help"

type Screen = { kind: "list" } | { kind: "idea"; id: string } | { kind: "suggest" }
type Tab = "ideas" | "planned" | "in_progress" | "shipped"

interface Loaded<T> {
  data?: T
  failed?: boolean
}

interface Column {
  items: Discussion[]
  cursor?: string
  loading: boolean
  failed: boolean
}

const tabs: Tab[] = ["ideas", "planned", "in_progress", "shipped"]
const pageSize = 20
const minTitle = 3
const voteIcon = icons.vote.replace('width="20" height="20"', 'width="14" height="14"')

function excerptOf(html: string): string {
  const template = document.createElement("template")
  template.innerHTML = html
  const text = (template.content.textContent ?? "").replace(/\s+/g, " ").trim()
  return text.length > 140 ? `${text.slice(0, 139)}…` : text
}

export function roadmapApp(context: AppContext, t: Translate, root: HTMLElement): () => void {
  const { client, locale } = context
  const columns = new Map<Tab, Column>()
  const details = new Map<string, Loaded<DiscussionDetail>>()
  const stack: Screen[] = []
  let screen: Screen = context.params.idea ? { kind: "idea", id: context.params.idea } : { kind: "list" }
  if (context.params.idea) stack.push({ kind: "list" })
  let tab: Tab = "ideas"
  let board: ClientRoadmap | null = null
  let writer: "reader" | "contact" | null = null
  const busy = new Set<string>()
  let acting = 0
  let error = ""
  let notice = ""
  let similar: Discussion[] = []
  let suggested: Discussion | null = null
  let sending = false
  let destroyed = false
  let timer: ReturnType<typeof setTimeout> | undefined
  let sequence = 0
  const suggestions = context.inbox.roadmap?.suggestions ?? "off"

  const titleField = el("input", "field")
  titleField.maxLength = 200
  titleField.dir = "auto"
  const bodyField = el("textarea", "field")
  bodyField.rows = 5
  bodyField.maxLength = 20000
  bodyField.dir = "auto"
  const replyField = el("textarea", "field")
  replyField.rows = 3
  replyField.maxLength = 20000
  replyField.dir = "auto"
  const similarBox = el("div", "ideas")
  const similarSection = el("section", "sec")

  const submit = el("button", "primary")
  submit.type = "button"
  submit.addEventListener("click", () => void send())
  const replyButton = textButton(t(msg`Reply`), "primary", () => void reply())

  titleField.addEventListener("input", () => {
    syncSend()
    clearTimeout(timer)
    const q = titleField.value.trim()
    const mine = ++sequence
    if (q.length < minTitle) {
      similar = []
      renderSimilar()
      return
    }
    timer = setTimeout(() => {
      client
        .listIdeas({ q, sort: "top", limit: 3 })
        .then((page) => {
          if (mine !== sequence || destroyed) return
          similar = page.items
          renderSimilar()
        })
        .catch(() => undefined)
    }, 300)
  })
  bodyField.addEventListener("input", () => syncSend())
  replyField.addEventListener("input", () => syncSend())

  function section(label: string): HTMLElement {
    const sec = el("section", "sec")
    if (label) sec.append(el("h2", "label", label))
    return sec
  }

  function stageLabel(stage: IdeaStage | undefined): string {
    return stage === "planned"
      ? t(msg`Planned`)
      : stage === "in_progress"
        ? t(msg`In progress`)
        : stage === "shipped"
          ? t(msg`Shipped`)
          : t(msg`Ideas`)
  }

  function syncSend(): void {
    submit.disabled = sending || titleField.value.trim().length < minTitle || bodyField.value.trim() === ""
    replyButton.disabled = busy.has("reply") || replyField.value.trim() === ""
  }

  function failureText(failure: unknown): string {
    if (!(failure instanceof ApiError)) return t(msg`Could not send. Try again.`)
    if (failure.code === "popup_blocked") return t(msg`Allow pop-ups for this site to sign in.`)
    if (failure.code === "sign_in_cancelled") return ""
    if (failure.status === 429) return t(msg`Too many attempts. Wait a moment and try again.`)
    if (failure.code === "network") return t(msg`You seem to be offline.`)
    return t(msg`Could not send. Try again.`)
  }

  // The pop-up has to open inside the click that asked for it, so callers reach this before any await.
  async function ensureWriter(): Promise<void> {
    if (writer) return
    await client.readerSignIn()
    writer = "reader"
  }

  function column(): Column {
    let value = columns.get(tab)
    if (!value) columns.set(tab, (value = { items: [], loading: false, failed: false }))
    return value
  }

  async function load(more: boolean): Promise<void> {
    const which = tab
    const current = column()
    if (current.loading) return
    current.loading = true
    current.failed = false
    if (!more) current.items = []
    render()
    try {
      const page = await client.listIdeas({
        stage: which,
        sort: "top",
        limit: pageSize,
        cursor: more ? current.cursor : undefined,
      })
      const known = new Set(current.items.map((item) => item.id))
      current.items = [...current.items, ...page.items.filter((item) => !known.has(item.id))]
      current.cursor = page.next_cursor
    } catch {
      current.failed = true
    }
    current.loading = false
    if (!destroyed) render()
  }

  async function loadBoard(): Promise<void> {
    try {
      board = await client.roadmap()
    } catch {
      board = null
    }
    if (!destroyed && screen.kind === "list") render()
  }

  async function loadIdea(id: string, force = false): Promise<void> {
    if (!force && details.get(id)?.data) return
    if (!details.get(id)?.data) details.set(id, {})
    render()
    try {
      details.set(id, { data: await client.getDiscussion(id) })
    } catch {
      details.set(id, { failed: true })
    }
    if (!destroyed) render()
  }

  function refresh(): void {
    columns.clear()
    details.clear()
    if (screen.kind === "list") void load(false)
    else if (screen.kind === "idea") void loadIdea(screen.id, true)
  }

  function patch(post: { id: string; votes: number; voted?: boolean }): void {
    for (const value of columns.values())
      value.items = value.items.map((item) =>
        item.opening_post_id === post.id ? { ...item, votes: post.votes, voted: post.voted } : item
      )
    for (const [id, state] of details) {
      const detail = state.data
      if (detail?.discussion.opening_post_id === post.id)
        details.set(id, {
          data: { ...detail, discussion: { ...detail.discussion, votes: post.votes, voted: post.voted } },
        })
    }
  }

  function vote(idea: Discussion): void {
    if (busy.has(idea.id)) return
    busy.add(idea.id)
    acting++
    error = ""
    const wasSignedIn = !!writer
    ensureWriter()
      .then(async () => {
        patch(await client.voteDiscussionPost(idea.opening_post_id, !idea.voted))
        if (!wasSignedIn) refresh()
      })
      .catch((failure: unknown) => {
        error = failureText(failure)
      })
      .finally(() => {
        busy.delete(idea.id)
        acting--
        render()
      })
    render()
  }

  async function send(): Promise<void> {
    const title = titleField.value.trim()
    const body = bodyField.value.trim()
    if (sending || title.length < minTitle || !body) return
    sending = true
    acting++
    error = ""
    syncSend()
    try {
      await ensureWriter()
      suggested = await client.suggestIdea({ title, body })
      titleField.value = ""
      bodyField.value = ""
      similar = []
      columns.clear()
      void loadBoard()
    } catch (failure) {
      error = failureText(failure)
    }
    sending = false
    acting--
    render()
  }

  async function reply(): Promise<void> {
    const current = screen
    const body = replyField.value.trim()
    if (current.kind !== "idea" || !body || busy.has("reply")) return
    busy.add("reply")
    acting++
    error = ""
    notice = ""
    syncSend()
    try {
      await ensureWriter()
      const post = await client.replyToDiscussion(current.id, { body })
      replyField.value = ""
      if (post.state === "pending") notice = t(msg`Waiting for approval`)
      await loadIdea(current.id, true)
    } catch (failure) {
      error = failureText(failure)
    }
    busy.delete("reply")
    acting--
    render()
  }

  function voteButton(idea: Discussion): HTMLButtonElement {
    const button = el("button", "vote")
    button.type = "button"
    button.disabled = idea.stage === "shipped" || idea.stage === "closed" || busy.has(idea.id)
    button.setAttribute("aria-pressed", String(!!idea.voted))
    button.setAttribute(
      "aria-label",
      idea.voted ? t(msg`Remove your vote for ${idea.title}`) : t(msg`Vote for ${idea.title}`)
    )
    button.insertAdjacentHTML("beforeend", voteIcon)
    button.append(el("span", "", new Intl.NumberFormat(locale).format(idea.votes)))
    button.addEventListener("click", () => vote(idea))
    return button
  }

  function ideaRow(idea: Discussion): HTMLElement {
    const row = el("div", "idea")
    const text = el("button", "idea-text")
    text.type = "button"
    const title = el("b", "", idea.title)
    title.dir = "auto"
    text.append(title)
    const lede = excerptOf(idea.body_html)
    if (lede) {
      const line = el("span", "lede", lede)
      line.dir = "auto"
      text.append(line)
    }
    const meta = el("span", "meta")
    if (idea.state === "pending") meta.append(el("span", "badge-line", t(msg`Waiting for approval`)))
    else meta.append(el("span", "", t(msg`Replies: ${idea.reply_count}`)))
    text.append(meta)
    text.addEventListener("click", () => go({ kind: "idea", id: idea.id }))
    row.append(voteButton(idea), text)
    return row
  }

  function failure(retry: () => void): HTMLElement {
    const sec = section("")
    sec.append(
      el("p", "empty", t(msg`Could not load this. Try again.`)),
      textButton(t(msg`Try again`), "primary", retry)
    )
    return sec
  }

  function loading(): HTMLElement {
    const sec = section("")
    sec.append(el("p", "hint", t(msg`Loading…`)))
    return sec
  }

  function backRow(): HTMLElement {
    const sec = el("div", "hback")
    const button = textButton("", "link soft", back)
    button.insertAdjacentHTML("beforeend", icons.back)
    button.append(t(msg`Back`))
    sec.append(button)
    return sec
  }

  function listNodes(): Node[] {
    const top = el("section", "sec rm-top")
    if (suggestions !== "off" || board?.description) {
      const head = el("div", "rm-head")
      const about = el("p", "hint", board?.description ?? "")
      about.dir = "auto"
      head.append(about)
      if (suggestions !== "off") {
        const button = textButton("", "pill suggest", () => go({ kind: "suggest" }))
        button.insertAdjacentHTML("beforeend", icons.plus)
        button.append(t(msg`Suggest`))
        head.append(button)
      }
      top.append(head)
    }
    const strip = el("div", "tabs")
    strip.setAttribute("role", "tablist")
    for (const value of tabs) {
      const button = el("button", "", stageLabel(value))
      button.type = "button"
      button.setAttribute("role", "tab")
      button.setAttribute("aria-selected", String(value === tab))
      button.addEventListener("click", () => {
        if (value === tab) return
        tab = value
        error = ""
        if (!columns.has(value)) void load(false)
        render()
      })
      strip.append(button)
    }
    top.append(strip)

    const current = column()
    if (current.failed && current.items.length === 0) return [top, failure(() => void load(false))]
    const body = el("section", "sec rm-list")
    if (current.items.length === 0)
      body.append(el("p", "hint", current.loading ? t(msg`Loading…`) : t(msg`Nothing here yet.`)))
    else {
      const list = el("div", "ideas")
      for (const idea of current.items) list.append(ideaRow(idea))
      body.append(list)
      if (current.cursor)
        body.append(
          textButton(current.loading ? t(msg`Loading…`) : t(msg`Load more`), "link brand", () => void load(true))
        )
    }
    if (error) body.append(el("p", "error", error))
    const nodes: Node[] = [top, body, el("div", "fill")]
    if (!writer) {
      const foot = section("")
      foot.append(el("p", "note", t(msg`Voting and suggesting need a quick sign-in.`)))
      nodes.push(foot)
    }
    return nodes
  }

  function ideaNodes(id: string): Node[] {
    const state = details.get(id) ?? {}
    if (state.failed) return [failure(() => void loadIdea(id, true))]
    const detail = state.data
    if (!detail) return [loading()]
    const idea = detail.discussion
    const head = section("")
    const top = el("div", "idea-top")
    const heading = el("div", "idea-head")
    const title = el("h1", "article-title", idea.title)
    title.dir = "auto"
    const tags = el("div", "meta")
    tags.append(el("span", "stage", stageLabel(idea.stage)))
    if (idea.state === "pending") tags.append(el("span", "badge-line", t(msg`Waiting for approval`)))
    heading.append(title, tags)
    top.append(voteButton(idea), heading)
    head.append(top)
    if (idea.body_html) head.append(article(idea.body_html))
    if (error) head.append(el("p", "error", error))
    const nodes: Node[] = [head]

    const replies = section(`01 — ${t(msg`Replies`)}`)
    if (detail.posts.length === 0) replies.append(el("p", "hint", t(msg`No replies yet.`)))
    for (const post of detail.posts) {
      const item = el("div", "reply")
      const who = el("div", "who-line")
      const name = el("b", "", post.author.deleted ? "?" : post.author.name || "")
      name.dir = "auto"
      who.append(name)
      if (post.author.kind === "member") who.append(el("span", "tag", t(msg`Team`)))
      if (post.state === "pending") who.append(el("span", "badge-line", t(msg`Waiting for approval`)))
      item.append(who, article(post.body_html))
      replies.append(item)
    }
    nodes.push(replies)

    if (!idea.locked) {
      const compose = section(`02 — ${t(msg`Write a reply`)}`)
      replyField.setAttribute("aria-label", t(msg`Write a reply`))
      if (notice) compose.append(el("p", "note", notice))
      syncSend()
      compose.append(replyField, replyButton)
      nodes.push(compose)
    }
    nodes.push(el("div", "fill"))
    return nodes
  }

  function renderSimilar(): void {
    similarBox.replaceChildren(...similar.map((idea) => ideaRow(idea)))
    similarSection.hidden = similar.length === 0
  }

  function suggestNodes(): Node[] {
    const sent = suggested
    if (sent) {
      const card = el("div", "done-card")
      card.insertAdjacentHTML("beforeend", icons.check)
      const actions = el("div", "rowbtns")
      actions.append(
        textButton(t(msg`Done`), "quiet", () => {
          suggested = null
          back()
        }),
        textButton(t(msg`View idea`), "primary", () => {
          suggested = null
          stack.pop()
          details.set(sent.id, { data: { discussion: sent, posts: [] } })
          screen = { kind: "idea", id: sent.id }
          render()
        })
      )
      card.append(
        el(
          "p",
          "empty",
          sent.state === "pending"
            ? t(msg`Thanks! The team will review your idea before it is shown.`)
            : t(msg`Thanks! Your idea is on the board.`)
        ),
        actions
      )
      const sec = section("")
      sec.append(card)
      return [sec]
    }
    const form = section(`01 — ${t(msg`Suggest an idea`)}`)
    titleField.placeholder = t(msg`Idea in a few words`)
    titleField.setAttribute("aria-label", t(msg`Idea in a few words`))
    bodyField.placeholder = t(msg`Describe your idea`)
    bodyField.setAttribute("aria-label", t(msg`Describe your idea`))
    titleField.disabled = bodyField.disabled = sending
    form.append(titleField, bodyField)

    similarSection.replaceChildren(el("h2", "label", `02 — ${t(msg`Similar ideas`)}`), similarBox)
    renderSimilar()

    const foot = section("")
    submit.textContent = sending ? t(msg`Sending…`) : t(msg`Send idea`)
    foot.append(submit)
    if (error) foot.append(el("p", "error", error))
    if (!writer) foot.append(el("p", "note", t(msg`Voting and suggesting need a quick sign-in.`)))
    syncSend()
    return [form, similarSection, el("div", "fill"), foot]
  }

  function render(): void {
    if (destroyed) return
    const nodes: Node[] = []
    if (screen.kind === "list") nodes.push(...listNodes())
    else {
      nodes.push(backRow())
      nodes.push(...(screen.kind === "idea" ? ideaNodes(screen.id) : suggestNodes()))
    }
    const active = (root.getRootNode() as Document | ShadowRoot).activeElement
    const keep = active === titleField || active === bodyField || active === replyField ? active : null
    const scroll = root.scrollTop
    root.replaceChildren(...nodes)
    if (keep instanceof HTMLElement) keep.focus({ preventScroll: true })
    root.scrollTop = scroll
  }

  function go(next: Screen): void {
    stack.push(screen)
    screen = next
    error = ""
    notice = ""
    if (next.kind === "idea") void loadIdea(next.id)
    render()
    root.scrollTop = 0
  }

  function back(): void {
    const previous = stack.pop()
    if (!previous) return context.back()
    screen = previous
    error = ""
    notice = ""
    render()
    root.scrollTop = 0
  }

  const offReader = client.on("reader", (reader) => {
    writer = reader ? "reader" : writer === "reader" ? null : writer
    if (acting === 0) refresh()
  })

  void client
    .discussionWriter()
    .catch(() => null)
    .then((value) => {
      writer = value
      render()
    })
  if (screen.kind === "idea") void loadIdea(screen.id)
  else void load(false)
  void loadBoard()
  return () => {
    destroyed = true
    clearTimeout(timer)
    offReader()
  }
}

import type { MessageDescriptor, Messages } from "@lingui/core"
import { msg } from "@lingui/core/macro"
import type { Discussion, DiscussionAuthor, DiscussionDetail, DiscussionPost } from "../client/api"
import type { YuvaClient } from "../client/client"
import type { ReaderSession } from "../client/reader"
import { messages as en } from "../locales/en/thread"
import { messages as tr } from "../locales/tr/thread"
import { translate } from "../translate"
import { el } from "./dom"
import { preview } from "./preview"

export type View = { kind: "list" } | { kind: "thread"; id: string } | { kind: "new" }
export type Writer = "reader" | "contact" | null
type Badge = "answered" | "pending" | "team" | "locked"

export interface DiscussionDetailEvent {
  page: string
  discussion_id: string
  state: Discussion["state"]
}

export interface DiscussionPostDetail {
  page: string
  discussion_id: string
  post_id: string
  state: DiscussionPost["state"]
}

export interface DiscussionsState {
  view: View
  items: Discussion[]
  thread: DiscussionDetail | null
  threadState: "loading" | "ready" | "missing" | "error"
  writer: Writer
  reader: ReaderSession | null
  busy: Set<string>
  notice: string
  error: string
  confirm: string
  focus: string
}

export interface ThreadHost {
  s: DiscussionsState
  readonly root: ParentNode
  locale(): string
  direction(): "ltr" | "rtl"
  page(): string
  pageTitle(): string
  mode(): string
  readonly icons: Record<"up" | "lock" | "clock", string>
  client(): YuvaClient
  render(): void
  emit(type: string, detail: unknown): void
  act<T>(key: string, action: () => Promise<T>): Promise<T | undefined>
  signIn(): Promise<void>
  open(id: string, focus: boolean): Promise<void>
  toList(): void
  forget(): void
  button(className: string, text: string, key: string, onClick: () => void, html?: string): HTMLButtonElement
  avatar(author: DiscussionAuthor, small?: boolean): HTMLElement
  badge(kind: Badge, short?: boolean): HTMLElement
  time(value: string): HTMLTimeElement
  authorName(author: DiscussionAuthor): string
  noticeBox(text: string, icon?: string): HTMLElement
}

export interface Thread {
  threadView(): Node[]
  newView(): Node[]
  syncSend(): void
  close(): void
  reset(): void
}

const editWindow = 30 * 60_000
const catalogs: Record<string, Messages> = { en, tr }

const svg = (path: string) =>
  `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${path}</svg>`
const checkCircle = svg('<circle cx="12" cy="12" r="9"/><path d="M8 12l3 3 5-6"/>')
const backIcon = svg('<path d="M19 12H5M12 19l-7-7 7-7"/>')
const bell = svg('<path d="M18 8a6 6 0 10-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 01-3.4 0"/>')

export const styles = `
.thread { display: flex; flex-direction: column; gap: 12px; }
.thread-head { display: flex; flex-direction: column; gap: 8px; }
.post { display: flex; gap: 12px; padding: 14px; border: 1px solid var(--yuva-border); border-radius: 12px; }
.post.accepted { border-color: var(--yuva-success); box-shadow: inset 0 0 0 1px var(--yuva-success); }
.post.pending { border-style: dashed; }
.side { display: flex; flex-direction: column; align-items: center; gap: 6px; }
.vote {
  display: inline-flex; flex-direction: column; align-items: center; gap: 0; min-inline-size: 40px; padding: 4px 6px;
  border: 1px solid var(--yuva-border); border-radius: 8px; background: none; cursor: pointer; font-size: 0.85em; font-weight: 600;
}
.vote:hover:not(:disabled) { border-color: var(--yuva-accent); }
.vote[aria-pressed="true"] { border-color: var(--yuva-accent); color: var(--yuva-accent); background: var(--yuva-soft); }
.post-body { flex: 1; min-inline-size: 0; display: flex; flex-direction: column; gap: 8px; }
.post-head { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; font-size: 0.88em; }
.name { font-weight: 600; }
.answer-label { display: flex; align-items: center; gap: 6px; color: var(--yuva-success); font-weight: 600; font-size: 0.85em; }
.tools { display: flex; flex-wrap: wrap; gap: 4px 8px; align-items: center; }
.children { display: flex; flex-direction: column; gap: 10px; margin-block-start: 4px; padding-inline-start: 14px; border-inline-start: 2px solid var(--yuva-border); }
.children .post { padding: 10px 0; border: 0; box-shadow: none; }
.children .post + .post { border-block-start: 1px solid var(--yuva-border); border-radius: 0; }
.deleted { font-style: italic; opacity: 0.65; }
.signin { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 16px; padding: 14px; border: 1px solid var(--yuva-border); border-radius: 12px; }
.composer { display: flex; flex-direction: column; border: 1px solid var(--yuva-border); border-radius: 12px; overflow: hidden; }
.composer:focus-within { border-color: var(--yuva-accent); }
.composer .ctabs { display: flex; gap: 4px; padding: 0 8px; border-block-end: 1px solid var(--yuva-border); }
.composer .ctab { padding: 7px 10px; border: 0; border-block-end: 2px solid transparent; background: none; cursor: pointer; font-size: 0.88em; opacity: 0.75; margin-block-end: -1px; }
.composer .ctab[aria-selected="true"] { border-block-end-color: var(--yuva-accent); font-weight: 600; opacity: 1; }
.composer .ctab:focus-visible { outline: 2px solid var(--yuva-accent); outline-offset: -2px; }
.composer textarea, .composer .preview { border: 0; border-radius: 0; min-block-size: 110px; padding: 10px 12px; }
.composer textarea:focus-visible { outline: none; }
.composer .preview { overflow-wrap: anywhere; }
.composer .foot { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; justify-content: space-between; padding: 8px 10px; border-block-start: 1px solid var(--yuva-border); }
.composer .hint { font-size: 0.8em; opacity: 0.7; }
.composer .buttons { display: flex; gap: 8px; margin-inline-start: auto; }
.field-title { font-size: 1.05em; font-weight: 600; }
.form { display: flex; flex-direction: column; gap: 10px; }
.md { overflow-wrap: anywhere; }
.md > :first-child { margin-block-start: 0; }
.md > :last-child { margin-block-end: 0; }
.md p, .md ul, .md ol, .md pre, .md blockquote, .md table { margin: 0 0 0.75em; }
.md h1, .md h2, .md h3, .md h4, .md h5, .md h6 { margin: 0.9em 0 0.4em; font-size: 1em; font-weight: 600; }
.md h1, .md h3 { font-size: 1.1em; }
.md ul, .md ol { padding-inline-start: 1.5em; }
.md a { color: var(--yuva-accent); }
.md code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.88em; padding: 1px 5px; border-radius: 5px; background: var(--yuva-soft); }
.md pre { padding: 10px 12px; border-radius: 8px; background: var(--yuva-soft); overflow-x: auto; }
.md pre code { padding: 0; background: none; }
.md blockquote { padding-inline-start: 12px; border-inline-start: 3px solid var(--yuva-border); opacity: 0.85; }
.md table { border-collapse: collapse; display: block; overflow-x: auto; }
.md th, .md td { border: 1px solid var(--yuva-border); padding: 4px 8px; }
.md img { max-inline-size: 100%; }
.md hr { border: 0; border-block-start: 1px solid var(--yuva-border); }
@media (max-width: 560px) {
  .post { padding: 12px; gap: 10px; }
}
`

class Composer {
  area = el("textarea")
  previewing = false

  constructor(rows: number) {
    this.area.rows = rows
    this.area.maxLength = 20000
    this.area.dir = "auto"
  }

  get value(): string {
    return this.area.value.trim()
  }

  clear(): void {
    this.area.value = ""
    this.previewing = false
  }
}

export function createThread(host: ThreadHost): Thread {
  const s = host.s
  const t = (descriptor: MessageDescriptor) => translate(catalogs[host.locale()] ?? en, descriptor, en)
  const reply = new Composer(4)
  const draft = new Composer(6)
  const title = el("input")
  let inline: { target: string; composer: Composer } | null = null
  let edit: { post: DiscussionPost | null; composer: Composer; title: HTMLInputElement } | null = null

  title.maxLength = 200
  title.dir = "auto"
  title.className = "field-title"
  for (const area of [reply.area, draft.area]) area.addEventListener("input", () => syncSend())
  title.addEventListener("input", () => syncSend())

  function syncSend(): void {
    for (const button of host.root.querySelectorAll<HTMLButtonElement>("[data-requires]")) {
      const needs = button.dataset.requires
      const busy = s.busy.has(button.dataset.busy ?? "")
      const ok =
        needs === "start"
          ? title.value.trim().length >= 3 && draft.value !== ""
          : needs === "reply"
            ? reply.value !== ""
            : needs === "inline"
              ? !!inline?.composer.value
              : needs === "edit"
                ? !!edit?.composer.value && (!!edit.post || edit.title.value.trim().length >= 3)
                : true
      button.disabled = busy || !ok
    }
  }

  async function start(): Promise<void> {
    const heading = title.value.trim()
    const body = draft.value
    if (heading.length < 3 || !body) return
    const page = host.page()
    const discussion = await host.act("start", () =>
      host.client().startDiscussion({ page_url: page, page_title: host.pageTitle() || undefined, title: heading, body })
    )
    if (!discussion) return
    host.forget()
    host.emit("yuva-discussion", {
      page,
      discussion_id: discussion.id,
      state: discussion.state,
    } satisfies DiscussionDetailEvent)
    if (host.page() !== page) return
    title.value = ""
    draft.clear()
    s.items = [discussion, ...s.items.filter((d) => d.id !== discussion.id)]
    s.notice =
      discussion.state === "pending"
        ? t(msg`Thanks! Your discussion will appear for everyone once the team approves it.`)
        : ""
    s.thread = { discussion, posts: [] }
    await host.open(discussion.id, true)
  }

  async function send(composer: Composer, parent?: string): Promise<void> {
    const thread = s.thread
    const body = composer.value
    if (!thread || !body) return
    const page = host.page()
    const id = thread.discussion.id
    const post = await host.act(parent ? `reply-${parent}` : "reply", () =>
      host.client().replyToDiscussion(id, { body, parent_post_id: parent })
    )
    if (!post) return
    host.forget()
    host.emit("yuva-post", {
      page,
      discussion_id: id,
      post_id: post.id,
      state: post.state,
    } satisfies DiscussionPostDetail)
    composer.clear()
    if (parent) inline = null
    s.notice =
      post.state === "pending" ? t(msg`Thanks! Your reply will appear for everyone once the team approves it.`) : ""
    if (s.thread?.discussion.id === id) {
      s.thread = { ...s.thread, posts: [...s.thread.posts, post] }
      s.focus = `post-${post.id}`
      host.render()
      void host.open(id, false)
    }
  }

  async function vote(post: { id: string; voted?: boolean }, opening: boolean): Promise<void> {
    if (!s.writer) return void host.signIn()
    const updated = await host.act(`vote-${post.id}`, () => host.client().voteDiscussionPost(post.id, !post.voted))
    if (!updated || !s.thread) return
    host.forget()
    if (opening) {
      const discussion = { ...s.thread.discussion, votes: updated.votes, voted: updated.voted }
      s.thread = { ...s.thread, discussion }
      s.items = s.items.map((d) => (d.id === discussion.id ? discussion : d))
    } else
      s.thread = {
        ...s.thread,
        posts: s.thread.posts.map((p) => (p.id === updated.id ? { ...p, ...updated } : p)),
      }
  }

  async function follow(): Promise<void> {
    const thread = s.thread
    if (!thread) return
    if (!s.writer) return void host.signIn()
    const following = !thread.discussion.following
    const done = await host.act("follow", () =>
      host
        .client()
        .followDiscussion(thread.discussion.id, following)
        .then(() => true)
    )
    if (done && s.thread?.discussion.id === thread.discussion.id)
      s.thread = { ...s.thread, discussion: { ...s.thread.discussion, following } }
  }

  async function accept(post: DiscussionPost | null): Promise<void> {
    const thread = s.thread
    if (!thread) return
    const discussion = await host.act(`accept-${post?.id ?? "clear"}`, () =>
      host.client().acceptDiscussionAnswer(thread.discussion.id, post?.id ?? null)
    )
    if (!discussion) return
    host.forget()
    s.focus = post ? `post-${post.id}` : "thread-title"
    await host.open(discussion.id, false)
  }

  async function saveEdit(): Promise<void> {
    const current = edit
    const thread = s.thread
    if (!current || !thread) return
    const body = current.composer.value
    const opening = !current.post
    const heading = current.title.value.trim()
    if (!body || (opening && heading.length < 3)) return
    const postId = current.post?.id ?? thread.discussion.opening_post_id
    const saved = await host.act(`edit-${postId}`, () =>
      host.client().editDiscussionPost(postId, opening ? { body, title: heading } : { body })
    )
    if (!saved) return
    host.forget()
    edit = null
    s.focus = opening ? "thread-title" : `post-${postId}`
    await host.open(thread.discussion.id, false)
  }

  async function remove(postId: string, opening: boolean): Promise<void> {
    const thread = s.thread
    if (!thread) return
    const done = await host.act(`delete-${postId}`, () =>
      host
        .client()
        .deleteDiscussionPost(postId)
        .then(() => true)
    )
    s.confirm = ""
    if (!done) return
    host.forget()
    if (opening) {
      try {
        await host.client().getDiscussion(thread.discussion.id)
      } catch {
        s.items = s.items.filter((d) => d.id !== thread.discussion.id)
        s.thread = null
        return host.toList()
      }
    }
    await host.open(thread.discussion.id, false)
  }

  function md(html: string): HTMLElement {
    const body = el("div", "md")
    body.dir = "auto"
    // body_html is rendered and sanitized by the server; the shadow root keeps the host's styles out.
    body.innerHTML = html
    for (const link of body.querySelectorAll("a[href]")) link.setAttribute("target", "_blank")
    return body
  }

  function composerBox(
    composer: Composer,
    options: {
      label: string
      placeholder: string
      submit: string
      busy: string
      requires: string
      onSubmit: () => void
      onCancel?: () => void
      focusKey: string
    }
  ): HTMLElement {
    const box = el("div", "composer")
    const tabs = el("div", "ctabs")
    tabs.setAttribute("role", "tablist")
    tabs.setAttribute("aria-label", options.label)
    const panelId = `p-${options.focusKey}`
    const write = host.button("ctab", t(msg`Write`), `${options.focusKey}-write`, () => {
      composer.previewing = false
      s.focus = options.focusKey
      host.render()
    })
    const show = host.button("ctab", t(msg`Preview`), `${options.focusKey}-preview`, () => {
      composer.previewing = true
      host.render()
    })
    for (const [tab, selected] of [
      [write, !composer.previewing],
      [show, composer.previewing],
    ] as const) {
      tab.setAttribute("role", "tab")
      tab.setAttribute("aria-selected", String(selected))
      tab.setAttribute("aria-controls", panelId)
      tab.tabIndex = selected ? 0 : -1
      tab.addEventListener("keydown", (event) => {
        if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return
        event.preventDefault()
        composer.previewing = !composer.previewing
        s.focus = `${options.focusKey}-${composer.previewing ? "preview" : "write"}`
        host.render()
      })
    }
    tabs.append(write, show)
    box.append(tabs)
    if (composer.previewing) {
      const panel = el("div", "preview md")
      panel.dir = "auto"
      panel.id = panelId
      panel.setAttribute("role", "tabpanel")
      panel.setAttribute("aria-label", t(msg`Preview`))
      if (composer.value) panel.append(preview(composer.area.value))
      else panel.append(el("p", "muted", t(msg`Nothing to preview.`)))
      box.append(panel)
    } else {
      composer.area.id = panelId
      composer.area.placeholder = options.placeholder
      composer.area.setAttribute("aria-label", options.label)
      composer.area.dataset.key = options.focusKey
      composer.area.disabled = s.busy.has(options.busy)
      composer.area.onkeydown = (event) => {
        if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
          event.preventDefault()
          options.onSubmit()
        } else if (event.key === "Escape" && options.onCancel) options.onCancel()
      }
      box.append(composer.area)
    }
    const foot = el("div", "foot")
    foot.append(el("span", "hint", t(msg`Markdown is supported.`)))
    const buttons = el("div", "buttons")
    if (options.onCancel)
      buttons.append(host.button("ghost", t(msg`Cancel`), `${options.focusKey}-cancel`, options.onCancel))
    const submit = host.button(
      "send",
      s.busy.has(options.busy) ? t(msg`Sending…`) : options.submit,
      `${options.focusKey}-submit`,
      options.onSubmit
    )
    submit.dataset.requires = options.requires
    submit.dataset.busy = options.busy
    buttons.append(submit)
    foot.append(buttons)
    box.append(foot)
    return box
  }

  function signInBox(text: string): HTMLElement {
    const box = el("div", "signin")
    box.append(el("p", "", text))
    box.append(host.button("send", t(msg`Sign in`), "sign-in-box", () => void host.signIn()))
    return box
  }

  function back(): HTMLElement {
    const button = host.button("plain", t(msg`All discussions`), "back", () => host.toList(), backIcon)
    button.style.alignSelf = "flex-start"
    button.style.display = "inline-flex"
    button.style.gap = "6px"
    button.style.alignItems = "center"
    const arrow = button.querySelector("svg")
    if (arrow && host.direction() === "rtl") arrow.style.transform = "scaleX(-1)"
    return button
  }

  function newView(): Node[] {
    const form = el("div", "form")
    form.append(el("h3", "", t(msg`Start a discussion`)))
    if (!s.writer) {
      form.append(signInBox(t(msg`Sign in to start a discussion. Your name is shown with your post.`)))
      return [back(), form]
    }
    title.placeholder = t(msg`Title`)
    title.setAttribute("aria-label", t(msg`Title`))
    title.dataset.key = "new-title"
    title.disabled = s.busy.has("start")
    form.append(title)
    form.append(
      composerBox(draft, {
        label: t(msg`Your question or idea`),
        placeholder: t(msg`Ask a question, share an idea or describe a problem with this page.`),
        submit: t(msg`Start discussion`),
        busy: "start",
        requires: "start",
        focusKey: "new-body",
        onSubmit: () => void start(),
        onCancel: () => host.toList(),
      })
    )
    if (host.mode() === "moderated" && s.reader)
      form.append(el("p", "muted", t(msg`Your first posts are checked by the team before they appear for everyone.`)))
    return [back(), form]
  }

  function editable(post: { created_at: string; mine?: boolean }): boolean {
    return !!post.mine && !s.thread?.discussion.locked && Date.now() - Date.parse(post.created_at) < editWindow
  }

  function postTools(post: DiscussionPost | null, top: DiscussionPost | null): HTMLElement | null {
    const thread = s.thread
    if (!thread) return null
    const discussion = thread.discussion
    const opening = !post
    const id = post?.id ?? discussion.opening_post_id
    const mine = opening ? !!discussion.mine : !!post.mine
    const deleted = opening ? !!discussion.deleted : post.deleted
    const tools = el("div", "tools")
    if (deleted) return null
    if (!opening && top && !discussion.locked && s.writer && post.state === "published") {
      tools.append(
        host.button("plain", t(msg`Reply`), `reply-to-${post.id}`, () => {
          inline = inline?.target === top.id ? inline : { target: top.id, composer: new Composer(3) }
          inline.composer.area.oninput = () => syncSend()
          s.focus = `inline-${top.id}`
          host.render()
        })
      )
    }
    if (!opening && discussion.mine && !post.parent_post_id && post.state === "published" && !discussion.locked) {
      const accepted = discussion.answer_post_id === post.id
      const button = host.button(
        "plain",
        accepted ? t(msg`Unmark as answer`) : t(msg`Mark as answer`),
        `accept-${post.id}`,
        () => void accept(accepted ? null : post),
        accepted ? "" : checkCircle
      )
      button.style.display = "inline-flex"
      button.style.gap = "4px"
      button.style.alignItems = "center"
      button.disabled = s.busy.has(`accept-${post.id}`) || s.busy.has("accept-clear")
      tools.append(button)
    }
    if (mine && editable(opening ? discussion : post)) {
      tools.append(
        host.button("plain", t(msg`Edit`), `edit-${id}`, () => {
          const composer = new Composer(opening ? 6 : 4)
          composer.area.value = opening ? discussion.body : post.body
          composer.area.oninput = () => syncSend()
          const heading = el("input")
          heading.className = "field-title"
          heading.maxLength = 200
          heading.dir = "auto"
          heading.value = discussion.title
          heading.oninput = () => syncSend()
          edit = { post, composer, title: heading }
          s.focus = opening ? "edit-title" : `edit-body-${id}`
          host.render()
        })
      )
    }
    if (mine) {
      if (s.confirm === id) {
        const yes = host.button("plain danger", t(msg`Delete`), `delete-yes-${id}`, () => void remove(id, opening))
        yes.disabled = s.busy.has(`delete-${id}`)
        tools.append(
          el("span", "muted", opening ? t(msg`Delete this discussion?`) : t(msg`Delete this reply?`)),
          yes,
          host.button("plain", t(msg`Keep`), `delete-no-${id}`, () => {
            s.confirm = ""
            s.focus = `delete-${id}`
            host.render()
          })
        )
      } else
        tools.append(
          host.button("plain", t(msg`Delete`), `delete-${id}`, () => {
            s.confirm = id
            s.focus = `delete-no-${id}`
            host.render()
          })
        )
    }
    return tools.childElementCount ? tools : null
  }

  function voteButton(target: { id: string; votes: number; voted?: boolean; mine?: boolean }, opening: boolean) {
    const button = host.button(
      "vote",
      String(target.votes),
      `vote-${target.id}`,
      () => void vote(target, opening),
      host.icons.up
    )
    button.setAttribute("aria-pressed", String(!!target.voted))
    button.setAttribute(
      "aria-label",
      target.voted ? t(msg`Remove your vote (${target.votes} votes)`) : t(msg`Upvote (${target.votes} votes)`)
    )
    button.disabled = !!target.mine || !!s.thread?.discussion.locked || s.busy.has(`vote-${target.id}`)
    return button
  }

  function postCard(post: DiscussionPost, top: DiscussionPost | null, children: DiscussionPost[] = []): HTMLElement {
    const discussion = s.thread!.discussion
    const accepted = discussion.answer_post_id === post.id
    const card = el("article", `post${accepted ? " accepted" : ""}${post.state === "pending" ? " pending" : ""}`)
    card.dataset.key = `post-${post.id}`
    card.tabIndex = -1
    card.setAttribute("aria-label", host.authorName(post.author))
    const side = el("div", "side")
    if (!post.deleted) side.append(voteButton(post, false))
    if (!top || top.id === post.id) card.append(side)
    const body = el("div", "post-body")
    if (accepted) {
      const label = el("div", "answer-label")
      label.innerHTML = checkCircle
      label.append(t(msg`Accepted answer`))
      body.append(label)
    }
    body.append(postHead(post.author, post.created_at, post.edited_at, post.state, post.deleted))
    if (edit?.post?.id === post.id) body.append(editForm())
    else if (post.deleted) body.append(el("p", "deleted", t(msg`This reply was deleted.`)))
    else body.append(md(post.body_html))
    const tools = postTools(post, top)
    if (tools) body.append(tools)
    if (children.length || inline?.target === post.id) {
      const nested = el("div", "children")
      for (const child of children) nested.append(postCard(child, post))
      if (inline?.target === post.id) {
        const current = inline
        nested.append(
          composerBox(current.composer, {
            label: t(msg`Your reply`),
            placeholder: t(msg`Write a reply…`),
            submit: t(msg`Reply`),
            busy: `reply-${post.id}`,
            requires: "inline",
            focusKey: `inline-${post.id}`,
            onSubmit: () => void send(current.composer, post.id),
            onCancel: () => {
              inline = null
              s.focus = `reply-to-${post.id}`
              host.render()
            },
          })
        )
      }
      body.append(nested)
    }
    card.append(body)
    return card
  }

  function postHead(
    author: DiscussionAuthor,
    created: string,
    edited: string | undefined,
    state: DiscussionPost["state"],
    deleted: boolean
  ): HTMLElement {
    const head = el("div", "post-head")
    if (!deleted) {
      head.append(host.avatar(author, true), el("span", "name", host.authorName(author)))
      if (author.kind === "member" && !author.deleted) head.append(host.badge("team"))
    }
    head.append(host.time(created))
    if (edited && !deleted) head.append(el("span", "muted", t(msg`edited`)))
    if (state === "pending") head.append(host.badge("pending"))
    return head
  }

  function editForm(): HTMLElement {
    const current = edit!
    const form = el("div", "form")
    if (!current.post) {
      current.title.setAttribute("aria-label", t(msg`Title`))
      current.title.dataset.key = "edit-title"
      form.append(current.title)
    }
    const id = current.post?.id ?? s.thread!.discussion.opening_post_id
    form.append(
      composerBox(current.composer, {
        label: t(msg`Edit your post`),
        placeholder: "",
        submit: t(msg`Save`),
        busy: `edit-${id}`,
        requires: "edit",
        focusKey: `edit-body-${id}`,
        onSubmit: () => void saveEdit(),
        onCancel: () => {
          edit = null
          s.focus = `edit-${id}`
          host.render()
        },
      })
    )
    return form
  }

  function threadView(): Node[] {
    const nodes: Node[] = [back()]
    const thread = s.thread
    if (!thread || s.threadState === "missing") {
      const box = el("div", "empty")
      box.append(
        el("p", "", s.threadState === "loading" ? t(msg`Loading…`) : t(msg`This discussion is no longer available.`))
      )
      nodes.push(box)
      return nodes
    }
    const discussion = thread.discussion
    const view = el("div", "thread")
    const head = el("div", "thread-head")
    if (edit && !edit.post) {
      head.append(postHead(discussion.author, discussion.created_at, discussion.edited_at, discussion.state, false))
      head.append(editForm())
      view.append(head)
    } else {
      const heading = el("h3", "", discussion.title)
      heading.dir = "auto"
      heading.tabIndex = -1
      heading.dataset.key = "thread-title"
      head.append(heading)
      const meta = el("div", "meta")
      if (discussion.answer_post_id) meta.append(host.badge("answered"))
      if (discussion.state === "pending") meta.append(host.badge("pending"))
      if (discussion.locked) meta.append(host.badge("locked"))
      if (meta.childElementCount) head.append(meta)
      view.append(head)

      const opening = el("article", `post${discussion.state === "pending" ? " pending" : ""}`)
      const side = el("div", "side")
      if (!discussion.deleted)
        side.append(
          voteButton(
            { id: discussion.opening_post_id, votes: discussion.votes, voted: discussion.voted, mine: discussion.mine },
            true
          )
        )
      const body = el("div", "post-body")
      body.append(
        postHead(discussion.author, discussion.created_at, discussion.edited_at, discussion.state, !!discussion.deleted)
      )
      body.append(
        discussion.deleted ? el("p", "deleted", t(msg`The opening post was deleted.`)) : md(discussion.body_html)
      )
      const tools = postTools(null, null)
      if (tools) body.append(tools)
      opening.append(side, body)
      view.append(opening)
    }

    const actions = el("div", "bar")
    const followButton = host.button(
      "ghost",
      discussion.following ? t(msg`Following`) : t(msg`Follow`),
      "follow",
      () => void follow(),
      bell
    )
    followButton.setAttribute("aria-pressed", String(!!discussion.following))
    followButton.title = t(msg`Get an e-mail when someone replies or an answer is accepted.`)
    followButton.disabled = s.busy.has("follow")
    const count = el("span", "muted", t(msg`Replies: ${discussion.reply_count}`))
    actions.append(count, followButton)
    view.append(actions)

    if (s.threadState === "loading" && !thread.posts.length && discussion.reply_count > 0) {
      const skeleton = el("div", "skeleton")
      skeleton.setAttribute("aria-busy", "true")
      skeleton.setAttribute("aria-label", t(msg`Loading…`))
      view.append(skeleton)
    }
    const tops = thread.posts.filter((p) => !p.parent_post_id)
    const replies = el("div", "thread")
    for (const top of tops)
      replies.append(
        postCard(
          top,
          top,
          thread.posts.filter((p) => p.parent_post_id === top.id)
        )
      )
    if (tops.length) view.append(replies)

    if (s.notice) view.append(host.noticeBox(s.notice))
    if (discussion.locked)
      view.append(host.noticeBox(t(msg`This discussion is locked. New replies are closed.`), host.icons.lock))
    else if (!s.writer) view.append(signInBox(t(msg`Sign in to reply, vote and follow this discussion.`)))
    else
      view.append(
        composerBox(reply, {
          label: t(msg`Your reply`),
          placeholder: t(msg`Write a reply…`),
          submit: t(msg`Reply`),
          busy: "reply",
          requires: "reply",
          focusKey: "reply",
          onSubmit: () => void send(reply),
        })
      )
    nodes.push(view)
    return nodes
  }

  return {
    threadView,
    newView,
    syncSend,
    close() {
      inline = null
      edit = null
    },
    reset() {
      inline = null
      edit = null
      reply.clear()
    },
  }
}

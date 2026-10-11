import { msg } from "@lingui/core/macro"
import {
  ApiError,
  type Discussion,
  type DiscussionAuthor,
  type DiscussionPage,
  type DiscussionPost,
  type DiscussionDetail,
  type DiscussionSort,
} from "../client/api"
import type { ReaderSession } from "../client/reader"
import { YuvaDocsElement, el } from "./base"
import { preview } from "./preview"

type View = { kind: "list" } | { kind: "thread"; id: string } | { kind: "new" }
type Writer = "reader" | "contact" | null

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

const cacheFor = 60_000
const editWindow = 30 * 60_000
const loads = new Map<string, { at: number; request: Promise<DiscussionPage> }>()

function forget(prefix: string): void {
  for (const key of loads.keys()) if (key.startsWith(prefix)) loads.delete(key)
}

const svg = (path: string, size = 16) =>
  `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${path}</svg>`
const icons = {
  up: svg('<path d="M12 19V5M5 12l7-7 7 7"/>'),
  comment: svg('<path d="M21 12a8 8 0 01-11.6 7.1L4 20l1-4.6A8 8 0 1121 12z"/>'),
  check: svg('<path d="M20 6L9 17l-5-5"/>'),
  checkCircle: svg('<circle cx="12" cy="12" r="9"/><path d="M8 12l3 3 5-6"/>'),
  lock: svg('<rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V7a4 4 0 018 0v4"/>'),
  clock: svg('<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>'),
  back: svg('<path d="M19 12H5M12 19l-7-7 7-7"/>'),
  bell: svg('<path d="M18 8a6 6 0 10-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 01-3.4 0"/>'),
}

const styles = `
.frame { display: flex; flex-direction: column; gap: 16px; }
:host { --yuva-success: #1a7f37; --yuva-warn: #9a6700; }
@media (prefers-color-scheme: dark) { :host { --yuva-success: #3fb950; --yuva-warn: #d29922; } }
h2 { margin: 0; font-size: 1.2em; font-weight: 600; }
h3 { margin: 0; font-size: 1.15em; font-weight: 600; overflow-wrap: anywhere; }
h3:focus { outline: none; }
button { font: inherit; color: inherit; }
.head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 16px; }
.who { display: flex; align-items: center; gap: 8px; font-size: 0.9em; }
.bar { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; }
.tabs { display: inline-flex; padding: 3px; gap: 2px; border: 1px solid var(--yuva-border); border-radius: 10px; }
.tab { padding: 4px 12px; border: 0; border-radius: 7px; background: none; cursor: pointer; opacity: 0.8; }
.tab[aria-selected="true"] { background: var(--yuva-soft); opacity: 1; font-weight: 600; }
.tab:focus-visible, .ghost:focus-visible, .vote:focus-visible, .title:focus-visible, .chip:focus-visible { outline: 2px solid var(--yuva-accent); outline-offset: 2px; }
.ghost {
  display: inline-flex; align-items: center; gap: 6px; padding: 5px 10px;
  border: 1px solid var(--yuva-border); border-radius: 8px; background: none; cursor: pointer;
}
.ghost:hover:not(:disabled) { background: var(--yuva-soft); }
.ghost:disabled, .vote:disabled { opacity: 0.55; cursor: default; }
.ghost[aria-pressed="true"] { border-color: var(--yuva-accent); color: var(--yuva-accent); }
.plain { padding: 2px 4px; border: 0; background: none; cursor: pointer; font-size: 0.875em; opacity: 0.75; border-radius: 6px; }
.plain:hover:not(:disabled) { opacity: 1; background: var(--yuva-soft); }
.plain:focus-visible { outline: 2px solid var(--yuva-accent); opacity: 1; }
.plain.danger { color: var(--yuva-danger); opacity: 1; }
.list { list-style: none; margin: 0; padding: 0; border: 1px solid var(--yuva-border); border-radius: 12px; overflow: hidden; }
.item { display: flex; gap: 12px; padding: 12px 14px; align-items: flex-start; }
.item + .item { border-block-start: 1px solid var(--yuva-border); }
.item:hover { background: var(--yuva-soft); }
.main { flex: 1; min-inline-size: 0; display: flex; flex-direction: column; gap: 2px; }
.title { padding: 0; border: 0; background: none; cursor: pointer; text-align: start; font-weight: 600; font-size: 1em; overflow-wrap: anywhere; }
.title:hover { color: var(--yuva-accent); text-decoration: underline; }
.excerpt { opacity: 0.75; font-size: 0.9em; overflow: hidden; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow-wrap: anywhere; }
.meta { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; font-size: 0.82em; opacity: 0.8; }
.counts { display: flex; gap: 12px; font-size: 0.85em; opacity: 0.8; flex-shrink: 0; padding-block-start: 2px; }
.count { display: inline-flex; align-items: center; gap: 4px; }
.avatar {
  flex-shrink: 0; inline-size: 32px; block-size: 32px; border-radius: 50%;
  display: inline-grid; place-items: center; font-size: 0.8em; font-weight: 600; color: #fff;
  background: var(--avatar, #6b7280); text-transform: uppercase; user-select: none;
}
.avatar.small { inline-size: 24px; block-size: 24px; font-size: 0.68em; }
.avatar.team { background: var(--yuva-accent); color: var(--yuva-on-accent); }
.avatar.gone { background: var(--yuva-soft); color: inherit; }
.badge {
  display: inline-flex; align-items: center; gap: 4px; padding: 1px 8px; border-radius: 999px;
  font-size: 0.78em; font-weight: 600; border: 1px solid currentColor; line-height: 1.5; white-space: nowrap;
}
.badge.answered { color: var(--yuva-success); }
.badge.pending { color: var(--yuva-warn); }
.badge.team { color: var(--yuva-accent); }
.badge.locked { opacity: 0.75; }
.empty { padding: 28px 16px; text-align: center; border: 1px dashed var(--yuva-border); border-radius: 12px; }
.empty p + p { margin-block-start: 4px; }
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
.notice { padding: 10px 12px; border-radius: 10px; background: var(--yuva-soft); font-size: 0.9em; display: flex; gap: 8px; align-items: flex-start; }
.notice svg { flex-shrink: 0; margin-block-start: 3px; }
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
.more { align-self: center; }
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
.skeleton { block-size: 64px; border-radius: 12px; background: var(--yuva-soft); }
@media (max-width: 560px) {
  .item { gap: 10px; padding: 12px; }
  .item .avatar { display: none; }
  .post { padding: 12px; gap: 10px; }
  .bar .send { inline-size: 100%; }
}
`

function hue(text: string): number {
  let hash = 0
  for (const char of text) hash = (hash * 31 + char.charCodeAt(0)) | 0
  return Math.abs(hash) % 360
}

function excerptOf(html: string): string {
  const template = document.createElement("template")
  template.innerHTML = html
  return (template.content.textContent ?? "").replace(/\s+/g, " ").trim().slice(0, 240)
}

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

export class YuvaDiscussionsElement extends YuvaDocsElement {
  #view: View = { kind: "list" }
  #sort: DiscussionSort = "new"
  #items: Discussion[] = []
  #cursor: string | undefined
  #listState: "loading" | "ready" | "error" | "off" = "loading"
  #loadedFor = ""
  #thread: DiscussionDetail | null = null
  #threadState: "loading" | "ready" | "missing" | "error" = "loading"
  #writer: Writer = null
  #reader: ReaderSession | null = null
  #offReader: (() => void) | null = null
  #busy = new Set<string>()
  #notice = ""
  #error = ""
  #confirm = ""
  #reply = new Composer(4)
  #inline: { target: string; composer: Composer } | null = null
  #edit: { post: DiscussionPost | null; composer: Composer; title: HTMLInputElement } | null = null
  #draft = new Composer(6)
  #title = el("input")
  #focus = ""

  constructor() {
    super()
    this.#title.maxLength = 200
    this.#title.dir = "auto"
    this.#title.className = "field-title"
    for (const area of [this.#reply.area, this.#draft.area]) area.addEventListener("input", () => this.#syncSend())
    this.#title.addEventListener("input", () => this.#syncSend())
  }

  get discussions(): Discussion[] {
    return this.#items
  }

  get reader(): ReaderSession | null {
    return this.#reader
  }

  connectedCallback(): void {
    super.connectedCallback()
    this.#watchReader()
  }

  disconnectedCallback(): void {
    super.disconnectedCallback()
    this.#offReader?.()
    this.#offReader = null
  }

  attributeChangedCallback(name: string, previous: string | null, value: string | null): void {
    super.attributeChangedCallback(name, previous, value)
    if (previous === value || !this.isConnected) return
    if (name === "channel" || name === "server") {
      this.#offReader?.()
      this.#offReader = null
      this.#watchReader()
    } else if (name === "identity-token") void this.#refreshWriter(true)
  }

  setIdentityToken(source: Parameters<YuvaDocsElement["setIdentityToken"]>[0]): void {
    super.setIdentityToken(source)
    void this.#refreshWriter(true)
  }

  #watchReader(): void {
    if (!this.channel || this.#offReader) return
    const client = this.client()
    this.#reader = client.reader
    this.#offReader = client.on("reader", (reader) => {
      this.#reader = reader
      this.emit("yuva-reader", reader ? reader.reader : null)
      void this.#refreshWriter(true)
    })
    void this.#refreshWriter(false)
  }

  async #refreshWriter(reload: boolean): Promise<void> {
    if (!this.channel) return
    const writer = await this.client()
      .discussionWriter()
      .catch(() => null)
    const changed = writer !== this.#writer
    this.#writer = writer
    if (reload && changed) {
      this.#loadedFor = ""
      void this.#load(false)
      if (this.#view.kind === "thread") void this.#open(this.#view.id, false)
    }
    this.render()
  }

  protected reset(): void {
    this.#view = { kind: "list" }
    this.#items = []
    this.#cursor = undefined
    this.#listState = "loading"
    this.#loadedFor = ""
    this.#thread = null
    this.#notice = ""
    this.#error = ""
    this.#confirm = ""
    this.#inline = null
    this.#edit = null
    this.#reply.clear()
  }

  protected pageChanged(): void {
    void this.#load(false)
  }

  reload(): Promise<void> {
    return this.#load(true)
  }

  open(id: string): Promise<void> {
    return this.#open(id, true)
  }

  signIn(): Promise<void> {
    this.#error = ""
    return this.client()
      .readerSignIn()
      .then(() => undefined)
      .catch((error: unknown) => {
        const code = error instanceof ApiError ? error.code : ""
        if (code === "sign_in_cancelled") return
        this.#error =
          code === "popup_blocked"
            ? this.t(msg`Your browser blocked the sign-in window. Allow pop-ups for this site and try again.`)
            : this.#failure(error)
        this.render()
      })
  }

  signOut(): Promise<void> {
    return this.client().readerSignOut()
  }

  get #prefix(): string {
    return `${this.server} ${this.channel} ${this.pageUrl} `
  }

  async #load(force: boolean, more = false): Promise<void> {
    const page = this.pageUrl
    if (!this.channel || !page || this.refused) return
    const reader = this.client().reader
    const auth = reader ? reader.token.slice(-12) : (this.#writer ?? "-")
    const key = `${this.#prefix}${this.#sort} ${auth}`
    if (!force && !more && this.#loadedFor === key) return
    this.#loadedFor = key
    let request: Promise<DiscussionPage>
    if (more) request = this.client().listDiscussions(page, { sort: this.#sort, cursor: this.#cursor })
    else {
      let load = loads.get(key)
      if (force || !load || Date.now() - load.at > cacheFor) {
        load = { at: Date.now(), request: this.client().listDiscussions(page, { sort: this.#sort }) }
        loads.set(key, load)
        const current = load
        load.request.catch(() => loads.get(key) === current && loads.delete(key))
      }
      request = load.request
      if (!this.#items.length) this.#listState = "loading"
    }
    if (more) this.#busy.add("more")
    this.render()
    try {
      const result = await request
      if (this.#loadedFor !== key) return
      this.#items = more
        ? [...this.#items, ...result.items.filter((d) => !this.#items.some((i) => i.id === d.id))]
        : result.items
      this.#cursor = result.next_cursor
      this.#listState = "ready"
    } catch (error) {
      if (this.#loadedFor !== key || this.refusal(error)) return
      if (!more) this.#loadedFor = ""
      this.#listState = error instanceof ApiError && error.code === "discussions_off" ? "off" : more ? "ready" : "error"
      if (more) this.#error = this.#failure(error)
    } finally {
      this.#busy.delete("more")
    }
    this.render()
  }

  async #open(id: string, focus: boolean): Promise<void> {
    const page = this.pageUrl
    const known = this.#thread?.discussion.id === id ? this.#thread : null
    this.#view = { kind: "thread", id }
    const listed = this.#items.find((d) => d.id === id)
    this.#thread = known ?? (listed ? { discussion: listed, posts: [] } : null)
    this.#threadState = known ? "ready" : "loading"
    if (focus) {
      this.#notice = known ? this.#notice : ""
      this.#error = ""
      this.#focus = "thread-title"
    }
    this.render()
    try {
      const detail = await this.client().getDiscussion(id)
      if (this.pageUrl !== page || this.#view.kind !== "thread" || this.#view.id !== id) return
      this.#thread = detail
      this.#threadState = "ready"
      this.#items = this.#items.map((d) => (d.id === id ? detail.discussion : d))
    } catch (error) {
      if (this.pageUrl !== page || this.#view.kind !== "thread" || this.#view.id !== id) return
      if (this.refusal(error, false)) return
      this.#threadState = error instanceof ApiError && error.status === 404 ? "missing" : "error"
    }
    this.render()
  }

  #toList(): void {
    this.#view = { kind: "list" }
    this.#inline = null
    this.#edit = null
    this.#confirm = ""
    this.#error = ""
    this.#focus = this.#thread ? `open-${this.#thread.discussion.id}` : "start"
    this.render()
    void this.#load(false)
  }

  #failure(error: unknown): string {
    const code = error instanceof ApiError ? error.code : ""
    if (code === "sign_in_required") return this.t(msg`Sign in to take part in the discussion.`)
    if (code === "discussion_locked") return this.t(msg`This discussion is locked.`)
    if (code === "edit_window_closed") return this.t(msg`Posts can be edited for 30 minutes after they are written.`)
    if (code === "discussions_off") return this.t(msg`Discussions are turned off for this site.`)
    if (code === "forbidden") return this.t(msg`You can't do that.`)
    if (code === "validation_failed") return this.t(msg`Check the title and the text and try again.`)
    return this.errorText(error)
  }

  async #act<T>(key: string, action: () => Promise<T>): Promise<T | undefined> {
    if (this.#busy.has(key)) return
    this.#busy.add(key)
    this.#error = ""
    this.render()
    try {
      return await action()
    } catch (error) {
      if (!this.refusal(error, false)) this.#error = this.#failure(error)
      if (error instanceof ApiError && error.code === "sign_in_required") void this.#refreshWriter(false)
      return undefined
    } finally {
      this.#busy.delete(key)
      this.render()
    }
  }

  async #start(): Promise<void> {
    const title = this.#title.value.trim()
    const body = this.#draft.value
    if (title.length < 3 || !body) return
    const page = this.pageUrl
    const discussion = await this.#act("start", () =>
      this.client().startDiscussion({ page_url: page, page_title: this.pageTitle || undefined, title, body })
    )
    if (!discussion) return
    forget(this.#prefix)
    this.emit("yuva-discussion", {
      page,
      discussion_id: discussion.id,
      state: discussion.state,
    } satisfies DiscussionDetailEvent)
    if (this.pageUrl !== page) return
    this.#title.value = ""
    this.#draft.clear()
    this.#items = [discussion, ...this.#items.filter((d) => d.id !== discussion.id)]
    this.#notice =
      discussion.state === "pending"
        ? this.t(msg`Thanks! Your discussion will appear for everyone once the team approves it.`)
        : ""
    this.#thread = { discussion, posts: [] }
    await this.#open(discussion.id, true)
  }

  async #send(composer: Composer, parent?: string): Promise<void> {
    const thread = this.#thread
    const body = composer.value
    if (!thread || !body) return
    const page = this.pageUrl
    const id = thread.discussion.id
    const post = await this.#act(parent ? `reply-${parent}` : "reply", () =>
      this.client().replyToDiscussion(id, { body, parent_post_id: parent })
    )
    if (!post) return
    forget(this.#prefix)
    this.emit("yuva-post", {
      page,
      discussion_id: id,
      post_id: post.id,
      state: post.state,
    } satisfies DiscussionPostDetail)
    composer.clear()
    if (parent) this.#inline = null
    this.#notice =
      post.state === "pending"
        ? this.t(msg`Thanks! Your reply will appear for everyone once the team approves it.`)
        : ""
    if (this.#thread?.discussion.id === id) {
      this.#thread = { ...this.#thread, posts: [...this.#thread.posts, post] }
      this.#focus = `post-${post.id}`
      this.render()
      void this.#open(id, false)
    }
  }

  async #vote(post: { id: string; voted?: boolean }, opening: boolean): Promise<void> {
    if (!this.#writer) return void this.signIn()
    const updated = await this.#act(`vote-${post.id}`, () => this.client().voteDiscussionPost(post.id, !post.voted))
    if (!updated || !this.#thread) return
    forget(this.#prefix)
    if (opening) {
      const discussion = { ...this.#thread.discussion, votes: updated.votes, voted: updated.voted }
      this.#thread = { ...this.#thread, discussion }
      this.#items = this.#items.map((d) => (d.id === discussion.id ? discussion : d))
    } else
      this.#thread = {
        ...this.#thread,
        posts: this.#thread.posts.map((p) => (p.id === updated.id ? { ...p, ...updated } : p)),
      }
  }

  async #follow(): Promise<void> {
    const thread = this.#thread
    if (!thread) return
    if (!this.#writer) return void this.signIn()
    const following = !thread.discussion.following
    const done = await this.#act("follow", () =>
      this.client()
        .followDiscussion(thread.discussion.id, following)
        .then(() => true)
    )
    if (done && this.#thread?.discussion.id === thread.discussion.id)
      this.#thread = { ...this.#thread, discussion: { ...this.#thread.discussion, following } }
  }

  async #accept(post: DiscussionPost | null): Promise<void> {
    const thread = this.#thread
    if (!thread) return
    const discussion = await this.#act(`accept-${post?.id ?? "clear"}`, () =>
      this.client().acceptDiscussionAnswer(thread.discussion.id, post?.id ?? null)
    )
    if (!discussion) return
    forget(this.#prefix)
    this.#focus = post ? `post-${post.id}` : "thread-title"
    await this.#open(discussion.id, false)
  }

  async #saveEdit(): Promise<void> {
    const edit = this.#edit
    const thread = this.#thread
    if (!edit || !thread) return
    const body = edit.composer.value
    const opening = !edit.post
    const title = edit.title.value.trim()
    if (!body || (opening && title.length < 3)) return
    const postId = edit.post?.id ?? thread.discussion.opening_post_id
    const saved = await this.#act(`edit-${postId}`, () =>
      this.client().editDiscussionPost(postId, opening ? { body, title } : { body })
    )
    if (!saved) return
    forget(this.#prefix)
    this.#edit = null
    this.#focus = opening ? "thread-title" : `post-${postId}`
    await this.#open(thread.discussion.id, false)
  }

  async #delete(postId: string, opening: boolean): Promise<void> {
    const thread = this.#thread
    if (!thread) return
    const done = await this.#act(`delete-${postId}`, () =>
      this.client()
        .deleteDiscussionPost(postId)
        .then(() => true)
    )
    this.#confirm = ""
    if (!done) return
    forget(this.#prefix)
    if (opening) {
      try {
        await this.client().getDiscussion(thread.discussion.id)
      } catch {
        this.#items = this.#items.filter((d) => d.id !== thread.discussion.id)
        this.#thread = null
        return this.#toList()
      }
    }
    await this.#open(thread.discussion.id, false)
  }

  #syncSend(): void {
    for (const button of this.root.querySelectorAll<HTMLButtonElement>("[data-requires]")) {
      const needs = button.dataset.requires
      const busy = this.#busy.has(button.dataset.busy ?? "")
      const ok =
        needs === "start"
          ? this.#title.value.trim().length >= 3 && this.#draft.value !== ""
          : needs === "reply"
            ? this.#reply.value !== ""
            : needs === "inline"
              ? !!this.#inline?.composer.value
              : needs === "edit"
                ? !!this.#edit?.composer.value && (!!this.#edit.post || this.#edit.title.value.trim().length >= 3)
                : true
      button.disabled = busy || !ok
    }
  }

  get #mode(): string {
    return this.inbox?.chat.discussions ?? "moderated"
  }

  #button(className: string, text: string, key: string, onClick: () => void, html = ""): HTMLButtonElement {
    const button = el("button", className)
    button.type = "button"
    if (html) button.innerHTML = html
    button.append(text)
    button.dataset.key = key
    button.addEventListener("click", onClick)
    return button
  }

  #authorName(author: DiscussionAuthor): string {
    if (author.deleted) return author.kind === "member" ? this.t(msg`Former team member`) : this.t(msg`Deleted account`)
    if (author.kind === "anonymous") return this.t(msg`Anonymous`)
    return author.name || this.t(msg`Reader`)
  }

  #avatar(author: DiscussionAuthor, small = false): HTMLElement {
    const avatar = el("span", `avatar${small ? " small" : ""}`, author.deleted ? "" : author.initials || "?")
    avatar.setAttribute("aria-hidden", "true")
    if (author.deleted || author.kind === "anonymous") avatar.classList.add("gone")
    else if (author.kind === "member") avatar.classList.add("team")
    else avatar.style.setProperty("--avatar", `hsl(${hue(author.name || author.initials)} 45% 42%)`)
    return avatar
  }

  #badge(kind: "answered" | "pending" | "team" | "locked", short = false): HTMLElement {
    const text =
      kind === "answered"
        ? this.t(msg`Answered`)
        : kind === "pending"
          ? short
            ? this.t(msg`Pending`)
            : this.t(msg`Awaiting approval`)
          : kind === "team"
            ? this.t(msg`Team`)
            : this.t(msg`Locked`)
    const badge = el("span", `badge ${kind}`)
    if (kind !== "team")
      badge.innerHTML = kind === "answered" ? icons.check : kind === "pending" ? icons.clock : icons.lock
    badge.append(text)
    return badge
  }

  #time(value: string): HTMLTimeElement {
    const time = el("time")
    time.dateTime = value
    const date = new Date(value)
    time.title = date.toLocaleString(this.locale)
    const seconds = (date.getTime() - Date.now()) / 1000
    const steps: [Intl.RelativeTimeFormatUnit, number][] = [
      ["second", 60],
      ["minute", 60],
      ["hour", 24],
      ["day", 30],
      ["month", 12],
      ["year", Infinity],
    ]
    let amount = seconds
    for (const [unit, size] of steps) {
      if (Math.abs(amount) < size) {
        time.textContent =
          unit === "second"
            ? this.t(msg`just now`)
            : new Intl.RelativeTimeFormat(this.locale, { numeric: "auto" }).format(Math.round(amount), unit)
        break
      }
      amount /= size
    }
    return time
  }

  #md(html: string): HTMLElement {
    const body = el("div", "md")
    body.dir = "auto"
    // body_html is rendered and sanitized by the server; the shadow root keeps the host's styles out.
    body.innerHTML = html
    for (const link of body.querySelectorAll("a[href]")) link.setAttribute("target", "_blank")
    return body
  }

  #composer(
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
    const write = this.#button("ctab", this.t(msg`Write`), `${options.focusKey}-write`, () => {
      composer.previewing = false
      this.#focus = options.focusKey
      this.render()
    })
    const show = this.#button("ctab", this.t(msg`Preview`), `${options.focusKey}-preview`, () => {
      composer.previewing = true
      this.render()
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
        this.#focus = `${options.focusKey}-${composer.previewing ? "preview" : "write"}`
        this.render()
      })
    }
    tabs.append(write, show)
    box.append(tabs)
    if (composer.previewing) {
      const panel = el("div", "preview md")
      panel.dir = "auto"
      panel.id = panelId
      panel.setAttribute("role", "tabpanel")
      panel.setAttribute("aria-label", this.t(msg`Preview`))
      if (composer.value) panel.append(preview(composer.area.value))
      else panel.append(el("p", "muted", this.t(msg`Nothing to preview.`)))
      box.append(panel)
    } else {
      composer.area.id = panelId
      composer.area.placeholder = options.placeholder
      composer.area.setAttribute("aria-label", options.label)
      composer.area.dataset.key = options.focusKey
      composer.area.disabled = this.#busy.has(options.busy)
      composer.area.onkeydown = (event) => {
        if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
          event.preventDefault()
          options.onSubmit()
        } else if (event.key === "Escape" && options.onCancel) options.onCancel()
      }
      box.append(composer.area)
    }
    const foot = el("div", "foot")
    foot.append(el("span", "hint", this.t(msg`Markdown is supported.`)))
    const buttons = el("div", "buttons")
    if (options.onCancel)
      buttons.append(this.#button("ghost", this.t(msg`Cancel`), `${options.focusKey}-cancel`, options.onCancel))
    const submit = this.#button(
      "send",
      this.#busy.has(options.busy) ? this.t(msg`Sending…`) : options.submit,
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

  #signInBox(text: string): HTMLElement {
    const box = el("div", "signin")
    box.append(el("p", "", text))
    box.append(this.#button("send", this.t(msg`Sign in`), "sign-in-box", () => void this.signIn()))
    return box
  }

  #header(): HTMLElement {
    const head = el("div", "head")
    head.append(el("h2", "", this.t(msg`Discussions`)))
    const who = el("div", "who")
    const reader = this.#reader
    if (reader) {
      const name = reader.reader.name || this.t(msg`Reader`)
      who.append(this.#avatar({ kind: "reader", name, initials: reader.reader.initials, deleted: false }, true))
      who.append(el("span", "", name))
      who.append(this.#button("plain", this.t(msg`Sign out`), "sign-out", () => void this.signOut()))
    } else if (!this.#writer && this.#mode !== "off") {
      who.append(this.#button("ghost", this.t(msg`Sign in`), "sign-in", () => void this.signIn()))
    }
    head.append(who)
    return head
  }

  #listView(): Node[] {
    const nodes: Node[] = []
    const bar = el("div", "bar")
    const tabs = el("div", "tabs")
    tabs.setAttribute("role", "tablist")
    tabs.setAttribute("aria-label", this.t(msg`Sort discussions`))
    const sorts: [DiscussionSort, string][] = [
      ["new", this.t(msg`New`)],
      ["top", this.t(msg`Top`)],
      ["unanswered", this.t(msg`Unanswered`)],
    ]
    sorts.forEach(([sort, label], index) => {
      const tab = this.#button("tab", label, `sort-${sort}`, () => this.#setSort(sort))
      tab.setAttribute("role", "tab")
      tab.setAttribute("aria-selected", String(this.#sort === sort))
      tab.setAttribute("aria-controls", "yuva-discussion-list")
      tab.tabIndex = this.#sort === sort ? 0 : -1
      tab.addEventListener("keydown", (event) => {
        const step = event.key === "ArrowRight" ? 1 : event.key === "ArrowLeft" ? -1 : 0
        if (!step) return
        event.preventDefault()
        const forward = this.direction === "rtl" ? -step : step
        const next = sorts[(index + forward + sorts.length) % sorts.length]
        if (next) this.#setSort(next[0])
      })
      tabs.append(tab)
    })
    const start = this.#button("send", this.t(msg`Start a discussion`), "start", () => {
      this.#view = { kind: "new" }
      this.#error = ""
      this.#notice = ""
      this.#focus = "new-title"
      this.render()
    })
    bar.append(tabs, start)
    nodes.push(bar)

    if (this.#listState === "loading" && !this.#items.length) {
      const skeleton = el("div", "skeleton")
      skeleton.setAttribute("aria-busy", "true")
      skeleton.setAttribute("aria-label", this.t(msg`Loading discussions…`))
      nodes.push(skeleton)
      return nodes
    }
    if (this.#listState === "error" && !this.#items.length) {
      const box = el("div", "empty")
      box.append(el("p", "", this.t(msg`Could not load the discussions.`)))
      const retry = this.#button("ghost", this.t(msg`Try again`), "retry", () => void this.#load(true))
      retry.style.marginBlockStart = "8px"
      box.append(retry)
      nodes.push(box)
      return nodes
    }
    if (!this.#items.length) {
      const box = el("div", "empty")
      box.append(
        el(
          "p",
          "",
          this.#sort === "unanswered"
            ? this.t(msg`Every discussion on this page has an answer.`)
            : this.t(msg`No discussions about this page yet.`)
        )
      )
      if (this.#sort !== "unanswered")
        box.append(el("p", "muted", this.t(msg`Have a question or an idea? Start the first one.`)))
      nodes.push(box)
      return nodes
    }
    const list = el("ul", "list")
    list.id = "yuva-discussion-list"
    list.setAttribute("aria-label", this.t(msg`Discussions about this page`))
    list.setAttribute("aria-busy", String(this.#listState === "loading"))
    for (const discussion of this.#items) {
      const item = el("li", "item")
      item.append(this.#avatar(discussion.author))
      const main = el("div", "main")
      const title = this.#button(
        "title",
        discussion.title,
        `open-${discussion.id}`,
        () => void this.#open(discussion.id, true)
      )
      title.dir = "auto"
      main.append(title)
      const excerpt = discussion.deleted ? "" : excerptOf(discussion.body_html)
      if (excerpt) {
        const text = el("p", "excerpt", excerpt)
        text.dir = "auto"
        main.append(text)
      }
      const meta = el("div", "meta")
      meta.append(el("span", "", this.#authorName(discussion.author)), this.#time(discussion.created_at))
      if (discussion.author.kind === "member" && !discussion.author.deleted) meta.append(this.#badge("team"))
      if (discussion.answer_post_id) meta.append(this.#badge("answered"))
      if (discussion.state === "pending") meta.append(this.#badge("pending"))
      if (discussion.locked) meta.append(this.#badge("locked"))
      main.append(meta)
      const counts = el("div", "counts")
      const votes = el("span", "count")
      votes.innerHTML = icons.up
      votes.append(String(discussion.votes))
      votes.setAttribute("aria-label", this.t(msg`Votes: ${discussion.votes}`))
      votes.title = votes.getAttribute("aria-label") ?? ""
      const replies = el("span", "count")
      replies.innerHTML = icons.comment
      replies.append(String(discussion.reply_count))
      replies.setAttribute("aria-label", this.t(msg`Replies: ${discussion.reply_count}`))
      replies.title = replies.getAttribute("aria-label") ?? ""
      counts.append(votes, replies)
      item.append(main, counts)
      list.append(item)
    }
    nodes.push(list)
    if (this.#cursor) {
      const more = this.#button(
        "ghost more",
        this.#busy.has("more") ? this.t(msg`Loading…`) : this.t(msg`Show more`),
        "more",
        () => void this.#load(false, true)
      )
      more.disabled = this.#busy.has("more")
      nodes.push(more)
    }
    return nodes
  }

  #setSort(sort: DiscussionSort): void {
    if (this.#sort === sort) return
    this.#sort = sort
    this.#items = []
    this.#cursor = undefined
    this.#listState = "loading"
    this.#focus = `sort-${sort}`
    void this.#load(false)
  }

  #back(): HTMLElement {
    const back = this.#button("plain", this.t(msg`All discussions`), "back", () => this.#toList(), icons.back)
    back.style.alignSelf = "flex-start"
    back.style.display = "inline-flex"
    back.style.gap = "6px"
    back.style.alignItems = "center"
    const arrow = back.querySelector("svg")
    if (arrow && this.direction === "rtl") arrow.style.transform = "scaleX(-1)"
    return back
  }

  #newView(): Node[] {
    const form = el("div", "form")
    form.append(el("h3", "", this.t(msg`Start a discussion`)))
    if (!this.#writer) {
      form.append(this.#signInBox(this.t(msg`Sign in to start a discussion. Your name is shown with your post.`)))
      return [this.#back(), form]
    }
    this.#title.placeholder = this.t(msg`Title`)
    this.#title.setAttribute("aria-label", this.t(msg`Title`))
    this.#title.dataset.key = "new-title"
    this.#title.disabled = this.#busy.has("start")
    form.append(this.#title)
    form.append(
      this.#composer(this.#draft, {
        label: this.t(msg`Your question or idea`),
        placeholder: this.t(msg`Ask a question, share an idea or describe a problem with this page.`),
        submit: this.t(msg`Start discussion`),
        busy: "start",
        requires: "start",
        focusKey: "new-body",
        onSubmit: () => void this.#start(),
        onCancel: () => this.#toList(),
      })
    )
    if (this.#mode === "moderated" && this.#reader)
      form.append(
        el("p", "muted", this.t(msg`Your first posts are checked by the team before they appear for everyone.`))
      )
    return [this.#back(), form]
  }

  #editable(post: { created_at: string; mine?: boolean }): boolean {
    return !!post.mine && !this.#thread?.discussion.locked && Date.now() - Date.parse(post.created_at) < editWindow
  }

  #postTools(post: DiscussionPost | null, top: DiscussionPost | null): HTMLElement | null {
    const thread = this.#thread
    if (!thread) return null
    const discussion = thread.discussion
    const opening = !post
    const id = post?.id ?? discussion.opening_post_id
    const mine = opening ? !!discussion.mine : !!post.mine
    const deleted = opening ? !!discussion.deleted : post.deleted
    const tools = el("div", "tools")
    if (deleted) return null
    if (!opening && top && !discussion.locked && this.#writer && post.state === "published") {
      tools.append(
        this.#button("plain", this.t(msg`Reply`), `reply-to-${post.id}`, () => {
          this.#inline = this.#inline?.target === top.id ? this.#inline : { target: top.id, composer: new Composer(3) }
          this.#inline.composer.area.oninput = () => this.#syncSend()
          this.#focus = `inline-${top.id}`
          this.render()
        })
      )
    }
    if (!opening && discussion.mine && !post.parent_post_id && post.state === "published" && !discussion.locked) {
      const accepted = discussion.answer_post_id === post.id
      const accept = this.#button(
        "plain",
        accepted ? this.t(msg`Unmark as answer`) : this.t(msg`Mark as answer`),
        `accept-${post.id}`,
        () => void this.#accept(accepted ? null : post),
        accepted ? "" : icons.checkCircle
      )
      accept.style.display = "inline-flex"
      accept.style.gap = "4px"
      accept.style.alignItems = "center"
      accept.disabled = this.#busy.has(`accept-${post.id}`) || this.#busy.has("accept-clear")
      tools.append(accept)
    }
    if (mine && this.#editable(opening ? discussion : post)) {
      tools.append(
        this.#button("plain", this.t(msg`Edit`), `edit-${id}`, () => {
          const composer = new Composer(opening ? 6 : 4)
          composer.area.value = opening ? discussion.body : post.body
          composer.area.oninput = () => this.#syncSend()
          const title = el("input")
          title.className = "field-title"
          title.maxLength = 200
          title.dir = "auto"
          title.value = discussion.title
          title.oninput = () => this.#syncSend()
          this.#edit = { post, composer, title }
          this.#focus = opening ? "edit-title" : `edit-body-${id}`
          this.render()
        })
      )
    }
    if (mine) {
      if (this.#confirm === id) {
        const yes = this.#button(
          "plain danger",
          this.t(msg`Delete`),
          `delete-yes-${id}`,
          () => void this.#delete(id, opening)
        )
        yes.disabled = this.#busy.has(`delete-${id}`)
        tools.append(
          el("span", "muted", opening ? this.t(msg`Delete this discussion?`) : this.t(msg`Delete this reply?`)),
          yes,
          this.#button("plain", this.t(msg`Keep`), `delete-no-${id}`, () => {
            this.#confirm = ""
            this.#focus = `delete-${id}`
            this.render()
          })
        )
      } else
        tools.append(
          this.#button("plain", this.t(msg`Delete`), `delete-${id}`, () => {
            this.#confirm = id
            this.#focus = `delete-no-${id}`
            this.render()
          })
        )
    }
    return tools.childElementCount ? tools : null
  }

  #voteButton(target: { id: string; votes: number; voted?: boolean; mine?: boolean }, opening: boolean): HTMLElement {
    const vote = this.#button(
      "vote",
      String(target.votes),
      `vote-${target.id}`,
      () => void this.#vote(target, opening),
      icons.up
    )
    vote.setAttribute("aria-pressed", String(!!target.voted))
    vote.setAttribute(
      "aria-label",
      target.voted ? this.t(msg`Remove your vote (${target.votes} votes)`) : this.t(msg`Upvote (${target.votes} votes)`)
    )
    vote.disabled = !!target.mine || !!this.#thread?.discussion.locked || this.#busy.has(`vote-${target.id}`)
    return vote
  }

  #postCard(post: DiscussionPost, top: DiscussionPost | null, children: DiscussionPost[] = []): HTMLElement {
    const discussion = this.#thread!.discussion
    const accepted = discussion.answer_post_id === post.id
    const card = el("article", `post${accepted ? " accepted" : ""}${post.state === "pending" ? " pending" : ""}`)
    card.dataset.key = `post-${post.id}`
    card.tabIndex = -1
    card.setAttribute("aria-label", this.#authorName(post.author))
    const side = el("div", "side")
    if (!post.deleted) side.append(this.#voteButton(post, false))
    if (!top || top.id === post.id) card.append(side)
    const body = el("div", "post-body")
    if (accepted) {
      const label = el("div", "answer-label")
      label.innerHTML = icons.checkCircle
      label.append(this.t(msg`Accepted answer`))
      body.append(label)
    }
    body.append(this.#postHead(post.author, post.created_at, post.edited_at, post.state, post.deleted))
    if (this.#edit?.post?.id === post.id) body.append(this.#editForm())
    else if (post.deleted) body.append(el("p", "deleted", this.t(msg`This reply was deleted.`)))
    else body.append(this.#md(post.body_html))
    const tools = this.#postTools(post, top)
    if (tools) body.append(tools)
    if (children.length || this.#inline?.target === post.id) {
      const nested = el("div", "children")
      for (const child of children) nested.append(this.#postCard(child, post))
      if (this.#inline?.target === post.id) {
        const inline = this.#inline
        nested.append(
          this.#composer(inline.composer, {
            label: this.t(msg`Your reply`),
            placeholder: this.t(msg`Write a reply…`),
            submit: this.t(msg`Reply`),
            busy: `reply-${post.id}`,
            requires: "inline",
            focusKey: `inline-${post.id}`,
            onSubmit: () => void this.#send(inline.composer, post.id),
            onCancel: () => {
              this.#inline = null
              this.#focus = `reply-to-${post.id}`
              this.render()
            },
          })
        )
      }
      body.append(nested)
    }
    card.append(body)
    return card
  }

  #postHead(
    author: DiscussionAuthor,
    created: string,
    edited: string | undefined,
    state: DiscussionPost["state"],
    deleted: boolean
  ): HTMLElement {
    const head = el("div", "post-head")
    if (!deleted) {
      head.append(this.#avatar(author, true), el("span", "name", this.#authorName(author)))
      if (author.kind === "member" && !author.deleted) head.append(this.#badge("team"))
    }
    head.append(this.#time(created))
    if (edited && !deleted) head.append(el("span", "muted", this.t(msg`edited`)))
    if (state === "pending") head.append(this.#badge("pending"))
    return head
  }

  #editForm(): HTMLElement {
    const edit = this.#edit!
    const form = el("div", "form")
    if (!edit.post) {
      edit.title.setAttribute("aria-label", this.t(msg`Title`))
      edit.title.dataset.key = "edit-title"
      form.append(edit.title)
    }
    const id = edit.post?.id ?? this.#thread!.discussion.opening_post_id
    form.append(
      this.#composer(edit.composer, {
        label: this.t(msg`Edit your post`),
        placeholder: "",
        submit: this.t(msg`Save`),
        busy: `edit-${id}`,
        requires: "edit",
        focusKey: `edit-body-${id}`,
        onSubmit: () => void this.#saveEdit(),
        onCancel: () => {
          this.#edit = null
          this.#focus = `edit-${id}`
          this.render()
        },
      })
    )
    return form
  }

  #threadView(): Node[] {
    const nodes: Node[] = [this.#back()]
    const thread = this.#thread
    if (!thread || this.#threadState === "missing") {
      const box = el("div", "empty")
      box.append(
        el(
          "p",
          "",
          this.#threadState === "loading" ? this.t(msg`Loading…`) : this.t(msg`This discussion is no longer available.`)
        )
      )
      nodes.push(box)
      return nodes
    }
    const discussion = thread.discussion
    const view = el("div", "thread")
    const head = el("div", "thread-head")
    if (this.#edit && !this.#edit.post) {
      head.append(
        this.#postHead(discussion.author, discussion.created_at, discussion.edited_at, discussion.state, false)
      )
      head.append(this.#editForm())
      view.append(head)
    } else {
      const title = el("h3", "", discussion.title)
      title.dir = "auto"
      title.tabIndex = -1
      title.dataset.key = "thread-title"
      head.append(title)
      const meta = el("div", "meta")
      if (discussion.answer_post_id) meta.append(this.#badge("answered"))
      if (discussion.state === "pending") meta.append(this.#badge("pending"))
      if (discussion.locked) meta.append(this.#badge("locked"))
      if (meta.childElementCount) head.append(meta)
      view.append(head)

      const opening = el("article", `post${discussion.state === "pending" ? " pending" : ""}`)
      const side = el("div", "side")
      if (!discussion.deleted)
        side.append(
          this.#voteButton(
            { id: discussion.opening_post_id, votes: discussion.votes, voted: discussion.voted, mine: discussion.mine },
            true
          )
        )
      const body = el("div", "post-body")
      body.append(
        this.#postHead(
          discussion.author,
          discussion.created_at,
          discussion.edited_at,
          discussion.state,
          !!discussion.deleted
        )
      )
      body.append(
        discussion.deleted
          ? el("p", "deleted", this.t(msg`The opening post was deleted.`))
          : this.#md(discussion.body_html)
      )
      const tools = this.#postTools(null, null)
      if (tools) body.append(tools)
      opening.append(side, body)
      view.append(opening)
    }

    const actions = el("div", "bar")
    const follow = this.#button(
      "ghost",
      discussion.following ? this.t(msg`Following`) : this.t(msg`Follow`),
      "follow",
      () => void this.#follow(),
      icons.bell
    )
    follow.setAttribute("aria-pressed", String(!!discussion.following))
    follow.title = this.t(msg`Get an e-mail when someone replies or an answer is accepted.`)
    follow.disabled = this.#busy.has("follow")
    const count = el("span", "muted", this.t(msg`Replies: ${discussion.reply_count}`))
    actions.append(count, follow)
    view.append(actions)

    if (this.#threadState === "loading" && !thread.posts.length && discussion.reply_count > 0) {
      const skeleton = el("div", "skeleton")
      skeleton.setAttribute("aria-busy", "true")
      skeleton.setAttribute("aria-label", this.t(msg`Loading…`))
      view.append(skeleton)
    }
    const tops = thread.posts.filter((p) => !p.parent_post_id)
    const replies = el("div", "thread")
    for (const top of tops)
      replies.append(
        this.#postCard(
          top,
          top,
          thread.posts.filter((p) => p.parent_post_id === top.id)
        )
      )
    if (tops.length) view.append(replies)

    if (this.#notice) view.append(this.#noticeBox(this.#notice))
    if (discussion.locked)
      view.append(this.#noticeBox(this.t(msg`This discussion is locked. New replies are closed.`), icons.lock))
    else if (!this.#writer)
      view.append(this.#signInBox(this.t(msg`Sign in to reply, vote and follow this discussion.`)))
    else
      view.append(
        this.#composer(this.#reply, {
          label: this.t(msg`Your reply`),
          placeholder: this.t(msg`Write a reply…`),
          submit: this.t(msg`Reply`),
          busy: "reply",
          requires: "reply",
          focusKey: "reply",
          onSubmit: () => void this.#send(this.#reply),
        })
      )
    nodes.push(view)
    return nodes
  }

  #noticeBox(text: string, icon = icons.clock): HTMLElement {
    const box = el("div", "notice")
    box.setAttribute("role", "status")
    box.innerHTML = icon
    box.append(el("p", "", text))
    return box
  }

  protected render(): void {
    if (this.refused || !this.channel) {
      this.root.replaceChildren()
      return
    }
    let focused = ""
    try {
      focused = (this.root.activeElement as HTMLElement | null)?.dataset?.key ?? ""
    } catch {}
    const children: Node[] = [this.#header()]
    if (this.#mode === "off" || this.#listState === "off") {
      children.push(el("p", "muted", this.t(msg`Discussions are turned off for this site.`)))
    } else if (this.#view.kind === "thread") children.push(...this.#threadView())
    else if (this.#view.kind === "new") children.push(...this.#newView())
    else {
      if (this.#notice) children.push(this.#noticeBox(this.#notice))
      children.push(...this.#listView())
    }
    if (this.#error) {
      const error = el("p", "error", this.#error)
      error.setAttribute("role", "alert")
      children.push(error)
    }
    this.frame(styles, ...children)
    this.#syncSend()
    const wanted = this.#focus
    this.#focus = ""
    const key = wanted || focused
    if (!key) return
    let active: Element | null = null
    try {
      active = this.root.activeElement
    } catch {}
    if (active instanceof HTMLElement && active.dataset.key === key) return
    const target = [...this.root.querySelectorAll<HTMLElement>("[data-key]")].find((node) => node.dataset.key === key)
    target?.focus({ preventScroll: !wanted })
  }
}

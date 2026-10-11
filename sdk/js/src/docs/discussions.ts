import { msg } from "@lingui/core/macro"
import {
  ApiError,
  type Discussion,
  type DiscussionAuthor,
  type DiscussionPage,
  type DiscussionSort,
} from "../client/api"
import type { ReaderSession } from "../client/reader"
import { YuvaDocsElement, el } from "./base"
import type { DiscussionsState, Thread, ThreadHost } from "./thread"

export type { DiscussionDetailEvent, DiscussionPostDetail } from "./thread"

export type ThreadModule = typeof import("./thread")

const hashPrefix = "#yuva-discussion-"
const cacheFor = 60_000
const loads = new Map<string, { at: number; request: Promise<DiscussionPage> }>()

function forget(prefix: string): void {
  for (const key of loads.keys()) if (key.startsWith(prefix)) loads.delete(key)
}

const svg = (path: string) =>
  `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${path}</svg>`
const icons = {
  up: svg('<path d="M12 19V5M5 12l7-7 7 7"/>'),
  comment: svg('<path d="M21 12a8 8 0 01-11.6 7.1L4 20l1-4.6A8 8 0 1121 12z"/>'),
  check: svg('<path d="M20 6L9 17l-5-5"/>'),
  lock: svg('<rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V7a4 4 0 018 0v4"/>'),
  clock: svg('<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>'),
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
.notice { padding: 10px 12px; border-radius: 10px; background: var(--yuva-soft); font-size: 0.9em; display: flex; gap: 8px; align-items: flex-start; }
.notice svg { flex-shrink: 0; margin-block-start: 3px; }
.more { align-self: center; }
.skeleton { block-size: 64px; border-radius: 12px; background: var(--yuva-soft); }
@media (max-width: 560px) {
  .item { gap: 10px; padding: 12px; }
  .item .avatar { display: none; }
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

export class YuvaDiscussionsElement extends YuvaDocsElement {
  static loadThread: () => Promise<ThreadModule>

  #s: DiscussionsState = {
    view: { kind: "list" },
    items: [],
    thread: null,
    threadState: "loading",
    writer: null,
    reader: null,
    busy: new Set<string>(),
    notice: "",
    error: "",
    confirm: "",
    focus: "",
  }
  #sort: DiscussionSort = "new"
  #cursor: string | undefined
  #listState: "loading" | "ready" | "error" | "off" = "loading"
  #loadedFor = ""
  #offReader: (() => void) | null = null
  #thread: Thread | null = null
  #threadLoad: Promise<Thread | null> | null = null
  #threadStyles = ""

  get discussions(): Discussion[] {
    return this.#s.items
  }

  get reader(): ReaderSession | null {
    return this.#s.reader
  }

  #hashChanged = () => this.#fromHash()

  connectedCallback(): void {
    super.connectedCallback()
    window.addEventListener("hashchange", this.#hashChanged)
    this.#watchReader()
  }

  disconnectedCallback(): void {
    super.disconnectedCallback()
    window.removeEventListener("hashchange", this.#hashChanged)
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
    this.#s.reader = client.reader
    this.#offReader = client.on("reader", (reader) => {
      this.#s.reader = reader
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
    const changed = writer !== this.#s.writer
    this.#s.writer = writer
    if (reload && changed) {
      this.#loadedFor = ""
      void this.#load(false)
      if (this.#s.view.kind === "thread") void this.#open(this.#s.view.id, false)
    }
    this.render()
  }

  protected reset(): void {
    const s = this.#s
    s.view = { kind: "list" }
    s.items = []
    this.#cursor = undefined
    this.#listState = "loading"
    this.#loadedFor = ""
    s.thread = null
    s.notice = ""
    s.error = ""
    s.confirm = ""
    this.#thread?.reset()
  }

  protected pageChanged(): void {
    void this.#load(false)
    this.#fromHash()
  }

  #linked(): string {
    const hash = location.hash
    return hash.startsWith(hashPrefix) ? decodeURIComponent(hash.slice(hashPrefix.length)) : ""
  }

  #fromHash(): void {
    const id = this.#linked()
    if (!id || !this.channel || this.refused) return
    if (this.#s.view.kind === "thread" && this.#s.view.id === id) return
    this.#thread?.close()
    this.#s.confirm = ""
    void this.#open(id, true)
    this.scrollIntoView({ block: "start" })
  }

  #prefetchOn(button: HTMLElement): void {
    const prefetch = () => void this.#loadThread()
    button.addEventListener("pointerenter", prefetch, { once: true })
    button.addEventListener("focus", prefetch, { once: true })
  }

  #loadThread(): Promise<Thread | null> {
    this.#threadLoad ??= YuvaDiscussionsElement.loadThread().then(
      (module) => {
        this.#threadStyles = module.styles
        this.#thread = module.createThread(this.#host)
        if (this.#s.view.kind !== "list") this.render()
        return this.#thread
      },
      () => {
        this.#threadLoad = null
        if (this.#s.view.kind !== "list") {
          this.#s.view = { kind: "list" }
          this.#s.error = this.t(msg`Could not open the discussion. Try again.`)
          this.render()
        }
        return null
      }
    )
    return this.#threadLoad
  }

  #host: ThreadHost = this.#makeHost()

  #makeHost(): ThreadHost {
    return {
      s: this.#s,
      icons,
      root: this.root,
      locale: () => this.locale,
      direction: () => this.direction,
      page: () => this.pageUrl,
      pageTitle: () => this.pageTitle,
      mode: () => this.#mode,
      client: () => this.client(),
      render: () => this.render(),
      emit: (type, detail) => this.emit(type, detail),
      act: (key, action) => this.#act(key, action),
      signIn: () => this.signIn(),
      open: (id, focus) => this.#open(id, focus),
      toList: () => this.#toList(),
      forget: () => forget(this.#prefix),
      button: (className, text, key, onClick, html) => this.#button(className, text, key, onClick, html),
      avatar: (author, small) => this.#avatar(author, small),
      badge: (kind, short) => this.#badge(kind, short),
      time: (value) => this.#time(value),
      authorName: (author) => this.#authorName(author),
      noticeBox: (text, icon) => this.#noticeBox(text, icon),
    }
  }

  reload(): Promise<void> {
    return this.#load(true)
  }

  open(id: string): Promise<void> {
    return this.#open(id, true)
  }

  signIn(): Promise<void> {
    this.#s.error = ""
    return this.client()
      .readerSignIn()
      .then(() => undefined)
      .catch((error: unknown) => {
        const code = error instanceof ApiError ? error.code : ""
        if (code === "sign_in_cancelled") return
        this.#s.error =
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
    const auth = reader ? reader.token.slice(-12) : (this.#s.writer ?? "-")
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
      if (!this.#s.items.length) this.#listState = "loading"
    }
    if (more) this.#s.busy.add("more")
    this.render()
    try {
      const result = await request
      if (this.#loadedFor !== key) return
      this.#s.items = more
        ? [...this.#s.items, ...result.items.filter((d) => !this.#s.items.some((i) => i.id === d.id))]
        : result.items
      this.#cursor = result.next_cursor
      this.#listState = "ready"
    } catch (error) {
      if (this.#loadedFor !== key || this.refusal(error)) return
      if (!more) this.#loadedFor = ""
      this.#listState = error instanceof ApiError && error.code === "discussions_off" ? "off" : more ? "ready" : "error"
      if (more) this.#s.error = this.#failure(error)
    } finally {
      this.#s.busy.delete("more")
    }
    this.render()
  }

  async #open(id: string, focus: boolean): Promise<void> {
    const s = this.#s
    const page = this.pageUrl
    const known = s.thread?.discussion.id === id ? s.thread : null
    void this.#loadThread()
    s.view = { kind: "thread", id }
    const listed = s.items.find((d) => d.id === id)
    s.thread = known ?? (listed ? { discussion: listed, posts: [] } : null)
    s.threadState = known ? "ready" : "loading"
    if (focus) {
      s.notice = known ? s.notice : ""
      s.error = ""
      s.focus = "thread-title"
    }
    this.render()
    try {
      const detail = await this.client().getDiscussion(id)
      if (this.pageUrl !== page || s.view.kind !== "thread" || s.view.id !== id) return
      const titled = !!s.thread
      s.thread = detail
      s.threadState = "ready"
      if (focus && !titled) s.focus = "thread-title"
      s.items = s.items.map((d) => (d.id === id ? detail.discussion : d))
    } catch (error) {
      if (this.pageUrl !== page || s.view.kind !== "thread" || s.view.id !== id) return
      if (this.refusal(error, false)) return
      s.threadState = error instanceof ApiError && error.status === 404 ? "missing" : "error"
    }
    this.render()
  }

  #toList(): void {
    const s = this.#s
    if (this.#linked()) {
      const url = new URL(location.href)
      url.hash = ""
      history.replaceState(history.state, "", url.href)
    }
    s.view = { kind: "list" }
    this.#thread?.close()
    s.confirm = ""
    s.error = ""
    s.focus = s.thread ? `open-${s.thread.discussion.id}` : "start"
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
    const s = this.#s
    if (s.busy.has(key)) return
    s.busy.add(key)
    s.error = ""
    this.render()
    try {
      return await action()
    } catch (error) {
      if (!this.refusal(error, false)) s.error = this.#failure(error)
      if (error instanceof ApiError && error.code === "sign_in_required") void this.#refreshWriter(false)
      return undefined
    } finally {
      s.busy.delete(key)
      this.render()
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
    if (author.deleted) return author.kind === "member" ? this.t(msg`Deleted member`) : this.t(msg`Deleted reader`)
    if (author.kind === "anonymous") return this.t(msg`Anonymous`)
    return author.name || (author.kind === "member" ? this.t(msg`Team member`) : this.t(msg`Reader`))
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

  #header(): HTMLElement {
    const head = el("div", "head")
    head.append(el("h2", "", this.t(msg`Discussions`)))
    const who = el("div", "who")
    const reader = this.#s.reader
    if (reader) {
      const name = reader.reader.name || this.t(msg`Reader`)
      who.append(this.#avatar({ kind: "reader", name, initials: reader.reader.initials, deleted: false }, true))
      who.append(el("span", "", name))
      who.append(this.#button("plain", this.t(msg`Sign out`), "sign-out", () => void this.signOut()))
    } else if (!this.#s.writer && this.#mode !== "off") {
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
      void this.#loadThread()
      this.#s.view = { kind: "new" }
      this.#s.error = ""
      this.#s.notice = ""
      this.#s.focus = "new-title"
      this.render()
    })
    this.#prefetchOn(start)
    bar.append(tabs, start)
    nodes.push(bar)

    if (this.#listState === "loading" && !this.#s.items.length) {
      const skeleton = el("div", "skeleton")
      skeleton.setAttribute("aria-busy", "true")
      skeleton.setAttribute("aria-label", this.t(msg`Loading discussions…`))
      nodes.push(skeleton)
      return nodes
    }
    if (this.#listState === "error" && !this.#s.items.length) {
      const box = el("div", "empty")
      box.append(el("p", "", this.t(msg`Could not load the discussions.`)))
      const retry = this.#button("ghost", this.t(msg`Try again`), "retry", () => void this.#load(true))
      retry.style.marginBlockStart = "8px"
      box.append(retry)
      nodes.push(box)
      return nodes
    }
    if (!this.#s.items.length) {
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
    for (const discussion of this.#s.items) {
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
      this.#prefetchOn(title)
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
        this.#s.busy.has("more") ? this.t(msg`Loading…`) : this.t(msg`Show more`),
        "more",
        () => void this.#load(false, true)
      )
      more.disabled = this.#s.busy.has("more")
      nodes.push(more)
    }
    return nodes
  }

  #setSort(sort: DiscussionSort): void {
    if (this.#sort === sort) return
    this.#sort = sort
    this.#s.items = []
    this.#cursor = undefined
    this.#listState = "loading"
    this.#s.focus = `sort-${sort}`
    void this.#load(false)
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
    } else if (this.#s.view.kind !== "list") {
      const thread = this.#thread
      if (thread) children.push(...(this.#s.view.kind === "thread" ? thread.threadView() : thread.newView()))
      else {
        const skeleton = el("div", "skeleton")
        skeleton.setAttribute("aria-busy", "true")
        skeleton.setAttribute("aria-label", this.t(msg`Loading…`))
        children.push(skeleton)
      }
    } else {
      if (this.#s.notice) children.push(this.#noticeBox(this.#s.notice))
      children.push(...this.#listView())
    }
    if (this.#s.error) {
      const error = el("p", "error", this.#s.error)
      error.setAttribute("role", "alert")
      children.push(error)
    }
    this.frame(styles + this.#threadStyles, ...children)
    if (this.#s.view.kind !== "list" && !this.#thread) return
    this.#thread?.syncSend()
    const wanted = this.#s.focus
    this.#s.focus = ""
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

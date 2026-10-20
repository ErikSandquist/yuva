import { msg } from "@lingui/core/macro"
import type { ChangelogLabel, ClientWhatsNewEntry } from "../client/api"
import { el, textButton } from "../panel/helpers"
import { icons } from "../panel/icons"
import { readStored, writeStored, type AppContext } from "../types"
import { updatesSince } from "../updates"
import type { Translate } from "./apps"
import { article } from "./help"

type Screen = { kind: "list" } | { kind: "entry"; id: string }

const pageSize = 20

export function whatsNewApp(context: AppContext, t: Translate, root: HTMLElement): () => void {
  const { client, locale } = context
  const since = updatesSince(readStored(client.channel).whats_new_seen)
  const items: ClientWhatsNewEntry[] = []
  const stack: Screen[] = []
  let screen: Screen = context.params.entry ? { kind: "entry", id: context.params.entry } : { kind: "list" }
  if (context.params.entry) stack.push({ kind: "list" })
  let cursor: string | undefined
  let loading = false
  let loaded = false
  let failed = false
  let single: { entry?: ClientWhatsNewEntry; failed?: boolean } = {}
  let destroyed = false

  const labelText = (label: ChangelogLabel): string =>
    label === "new" ? t(msg`New`) : label === "improved" ? t(msg`Improved`) : t(msg`Fixed`)

  function dateText(value: string): string {
    const at = new Date(value)
    return new Intl.DateTimeFormat(
      locale,
      at.getFullYear() === new Date().getFullYear()
        ? { month: "short", day: "numeric" }
        : { year: "numeric", month: "short", day: "numeric" }
    ).format(at)
  }

  function markSeen(): void {
    const newest = items.reduce((latest, entry) => Math.max(latest, Date.parse(entry.published_at)), 0)
    if (!newest || newest <= since) return
    const stored = readStored(client.channel)
    if (stored.whats_new_seen && Date.parse(stored.whats_new_seen) >= newest) return
    writeStored(client.channel, { ...stored, whats_new_seen: new Date(newest).toISOString() })
    context.seenUpdates()
  }

  function meta(entry: ClientWhatsNewEntry): HTMLElement {
    const line = el("div", "wn-meta")
    for (const label of entry.labels) line.append(el("span", `wn-label ${label}`, labelText(label)))
    line.append(el("span", "wn-date", dateText(entry.published_at)))
    if (Date.parse(entry.published_at) > since) {
      const dot = el("span", "unread-dot")
      dot.setAttribute("role", "img")
      dot.setAttribute("aria-label", t(msg`Unread`))
      line.append(dot)
    }
    return line
  }

  async function load(more: boolean): Promise<void> {
    if (loading) return
    loading = true
    failed = false
    render()
    try {
      const page = await client.whatsNew({ locale, limit: pageSize, cursor: more ? cursor : undefined })
      const known = new Set(items.map((entry) => entry.id))
      items.push(...page.items.filter((entry) => !known.has(entry.id)))
      cursor = page.next_cursor
      loaded = true
      markSeen()
    } catch {
      failed = true
    }
    loading = false
    render()
  }

  async function loadEntry(id: string): Promise<void> {
    single = {}
    const known = items.find((entry) => entry.id === id)
    if (known) {
      single = { entry: known }
      return render()
    }
    render()
    try {
      single = { entry: await client.whatsNewEntry(id, { locale }) }
    } catch {
      single = { failed: true }
    }
    render()
  }

  function go(next: Screen): void {
    stack.push(screen)
    screen = next
    if (next.kind === "entry") void loadEntry(next.id)
    render()
    root.scrollTop = 0
  }

  function back(): void {
    const previous = stack.pop()
    if (!previous) return context.back()
    screen = previous
    render()
    root.scrollTop = 0
  }

  function notice(text: string, retry?: () => void): HTMLElement {
    const sec = el("section", "sec")
    sec.append(el("p", retry ? "empty" : "hint", text))
    if (retry) sec.append(textButton(t(msg`Try again`), "primary", retry))
    return sec
  }

  function listNodes(): Node[] {
    if (failed && items.length === 0) return [notice(t(msg`Could not load this. Try again.`), () => void load(false))]
    if (items.length === 0) return [notice(loaded ? t(msg`No updates yet.`) : t(msg`Loading…`))]
    const list = el("section", "sec wn-list")
    for (const entry of items) {
      const row = el("button", "wn-entry")
      row.type = "button"
      const title = el("b", "", entry.title)
      title.dir = "auto"
      row.append(meta(entry), title)
      if (entry.excerpt) {
        const lede = el("span", "wn-lede", entry.excerpt)
        lede.dir = "auto"
        row.append(lede)
      }
      row.addEventListener("click", () => go({ kind: "entry", id: entry.id }))
      list.append(row)
    }
    if (cursor)
      list.append(textButton(loading ? t(msg`Loading…`) : t(msg`Load more`), "link brand", () => void load(true)))
    if (failed) list.append(el("p", "error", t(msg`Could not load this. Try again.`)))
    return [list, el("div", "fill")]
  }

  function entryNodes(id: string): Node[] {
    if (single.failed) return [notice(t(msg`Could not load this. Try again.`), () => void loadEntry(id))]
    const entry = single.entry
    if (!entry) return [notice(t(msg`Loading…`))]
    const sec = el("section", "sec")
    const title = el("h1", "article-title", entry.title)
    title.dir = "auto"
    sec.append(meta(entry), title)
    if (entry.image) {
      const image = el("img", "wn-image")
      image.src = entry.image.url
      image.alt = ""
      image.width = entry.image.width
      image.height = entry.image.height
      image.loading = "lazy"
      sec.append(image)
    }
    sec.append(article(entry.html))
    const link = el("a", "link brand", t(msg`Read on the web`))
    link.href = entry.url
    link.target = "_blank"
    link.rel = "noopener noreferrer"
    sec.append(link)
    return [sec, el("div", "fill")]
  }

  function backRow(): HTMLElement {
    const sec = el("div", "hback")
    const button = textButton("", "link soft", back)
    button.insertAdjacentHTML("beforeend", icons.back)
    button.append(t(msg`Back`))
    sec.append(button)
    return sec
  }

  function render(): void {
    if (destroyed) return
    const nodes: Node[] = []
    if (screen.kind === "list") nodes.push(...listNodes())
    else nodes.push(backRow(), ...entryNodes(screen.id))
    const scroll = root.scrollTop
    root.replaceChildren(...nodes)
    root.scrollTop = scroll
  }

  if (screen.kind === "entry") void loadEntry(screen.id)
  void load(false)
  return () => {
    destroyed = true
  }
}

import { msg } from "@lingui/core/macro"
import type {
  ClientHelp,
  ClientHelpArticle,
  ClientHelpArticleSummary,
  ClientHelpCollectionDetail,
  ClientHelpSearchResults,
  HelpHighlight,
  PageRating,
} from "../client/api"
import { el, searchField, textButton } from "../panel/helpers"
import { icons } from "../panel/icons"
import type { AppContext } from "../types"
import type { Translate } from "./apps"

type Screen = { kind: "home" } | { kind: "collection"; id: string } | { kind: "article"; id: string; search?: string }

interface Loaded<T> {
  data?: T
  failed?: boolean
}

const minQuery = 2
const unsafe = "script, style, iframe, object, embed, form, link, meta, base"

function pageUrl(): string | undefined {
  if (!/^https?:$/.test(location.protocol)) return undefined
  const url = location.origin + location.pathname
  return url.length > 2000 ? undefined : url
}

function marked(text: string, highlights: HelpHighlight[], field: HelpHighlight["field"]): Node[] {
  const ranges = highlights.filter((h) => h.field === field && h.end > h.start).sort((a, b) => a.start - b.start)
  const nodes: Node[] = []
  let at = 0
  for (const { start, end } of ranges) {
    if (start < at) continue
    if (start > at) nodes.push(document.createTextNode(text.slice(at, start)))
    nodes.push(el("mark", "", text.slice(start, end)))
    at = end
  }
  if (at < text.length) nodes.push(document.createTextNode(text.slice(at)))
  return nodes
}

export function article(html: string): HTMLElement {
  const prose = el("div", "prose")
  const template = document.createElement("template")
  template.innerHTML = html
  for (const node of template.content.querySelectorAll(unsafe)) node.remove()
  for (const node of template.content.querySelectorAll("*")) {
    for (const attribute of Array.from(node.attributes))
      if (
        /^on/i.test(attribute.name) ||
        (/^(href|src|xlink:href)$/i.test(attribute.name) && /^\s*javascript:/i.test(attribute.value))
      )
        node.removeAttribute(attribute.name)
    if (node instanceof HTMLAnchorElement) {
      node.target = "_blank"
      node.rel = "noopener noreferrer"
    }
    if (node instanceof HTMLImageElement) node.loading = "lazy"
  }
  prose.append(template.content)
  return prose
}

export function helpApp(context: AppContext, t: Translate, root: HTMLElement): () => void {
  const { client, locale } = context
  const help = client.help
  const reactions = new Map<string, PageRating>()
  const collections = new Map<string, Loaded<ClientHelpCollectionDetail>>()
  const articles = new Map<string, Loaded<ClientHelpArticle>>()
  const stack: Screen[] = []
  let screen: Screen = context.params.article ? { kind: "article", id: context.params.article } : { kind: "home" }
  let center: Loaded<ClientHelp> = {}
  let query = context.params.query ?? ""
  let results: ClientHelpSearchResults | null = null
  let searched = ""
  let searchFailed = false
  let timer: ReturnType<typeof setTimeout> | undefined
  let sequence = 0
  let destroyed = false

  const field = searchField(
    t(msg`Search for help`),
    query,
    (value) => {
      query = value
      void search(true)
    },
    (value) => {
      query = value
      clearTimeout(timer)
      if (query.trim().length < minQuery) {
        sequence++
        results = null
        searched = ""
        render()
      } else timer = setTimeout(() => void search(false), 250)
    }
  )

  function section(label: string): HTMLElement {
    const sec = el("section", "sec")
    if (label) sec.append(el("h2", "label", label))
    return sec
  }

  function messageUs(articleId?: string): HTMLElement {
    const sec = section("")
    const row = el("div", "ask")
    const words = el("div", "ask-text")
    words.append(el("b", "", t(msg`Still stuck?`)), el("span", "", t(msg`Ask the team, we read every message.`)))
    row.append(
      words,
      textButton(t(msg`Message us`), "primary", () => context.ask(articleId))
    )
    sec.append(row)
    return sec
  }

  function row(item: ClientHelpArticleSummary, highlights: HelpHighlight[], from?: string): HTMLElement {
    const button = el("button", "hrow")
    button.type = "button"
    const text = el("span", "hrow-text")
    const title = el("b")
    title.dir = "auto"
    title.append(...marked(item.title, highlights, "title"))
    const excerpt = el("span")
    excerpt.dir = "auto"
    excerpt.append(...marked(item.excerpt, highlights, "excerpt"))
    text.append(title)
    if (item.excerpt) text.append(excerpt)
    button.append(text)
    button.insertAdjacentHTML("beforeend", icons.chevron)
    button.addEventListener("click", () => {
      if (from) void help.click({ q: from, locale, article_id: item.id }).catch(() => undefined)
      go({ kind: "article", id: item.id, search: from })
    })
    return button
  }

  function list(label: string, items: ClientHelpArticleSummary[]): HTMLElement {
    const sec = section(label)
    const rows = el("div", "hrows")
    for (const item of items) rows.append(row(item, []))
    sec.append(rows)
    return sec
  }

  function pills(items: { id: string; name: string; count?: number }[]): HTMLElement {
    const wrap = el("div", "pills")
    for (const item of items) {
      const pill = textButton(item.name, "pill", () => go({ kind: "collection", id: item.id }))
      pill.dir = "auto"
      if (item.count !== undefined) pill.append(el("span", "", new Intl.NumberFormat(locale).format(item.count)))
      wrap.append(pill)
    }
    return wrap
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

  function homeNodes(): Node[] {
    const nodes: Node[] = []
    const top = el("section", "sec")
    top.append(field)
    nodes.push(top)
    if (query.trim().length >= minQuery && searched) {
      const found = results?.items ?? []
      const sec = section(`01 — ${t(msg`Best matches`)}`)
      if (searchFailed) return [...nodes, failure(() => void search(true))]
      if (found.length === 0) sec.append(el("p", "hint", t(msg`No articles found.`)))
      else {
        const rows = el("div", "hrows")
        for (const item of found) rows.append(row(item, item.highlights, searched))
        sec.append(rows)
      }
      nodes.push(sec)
      if (found.length === 0) nodes.push(el("div", "fill"), messageUs())
      return nodes
    }
    if (center.failed) return [...nodes, failure(loadCenter)]
    const data = center.data
    if (!data) return [...nodes, loading()]
    const best = data.suggested.length > 0 ? data.suggested : data.popular
    let n = 1
    if (best.length > 0)
      nodes.push(list(`0${n++} — ${data.suggested.length > 0 ? t(msg`Best matches`) : t(msg`Popular articles`)}`, best))
    const roots = data.collections.filter((c) => !c.parent_id)
    if (roots.length > 0) {
      const sec = section(`0${n++} — ${t(msg`Browse`)}`)
      sec.append(pills(roots.map((c) => ({ id: c.id, name: c.name, count: c.article_count }))))
      nodes.push(sec)
    }
    nodes.push(el("div", "fill"), messageUs())
    return nodes
  }

  function collectionNodes(id: string): Node[] {
    const state = collections.get(id) ?? {}
    if (state.failed) return [failure(() => void loadCollection(id))]
    const data = state.data
    if (!data) return [loading()]
    const head = section(`01 — ${data.collection.name}`)
    if (data.collection.description) {
      const description = el("p", "hint", data.collection.description)
      description.dir = "auto"
      head.append(description)
    }
    if (data.collections.length > 0)
      head.append(pills(data.collections.map((c) => ({ id: c.id, name: c.name, count: c.article_count }))))
    const nodes: Node[] = [head]
    if (data.articles.length > 0) nodes.push(list("", data.articles))
    nodes.push(el("div", "fill"), messageUs())
    return nodes
  }

  function rate(item: ClientHelpArticle, search: string | undefined, rating: PageRating): void {
    const previous = reactions.get(item.id)
    if (previous === rating) return
    reactions.set(item.id, rating)
    render()
    void help.react(item.id, { rating, previous, locale: item.locale, search }).catch(() => {
      if (previous) reactions.set(item.id, previous)
      else reactions.delete(item.id)
      render()
    })
  }

  function articleNodes(id: string, search?: string): Node[] {
    const state = articles.get(id) ?? {}
    if (state.failed) return [failure(() => void loadArticle(id))]
    const item = state.data
    if (!item) return [loading()]
    const body = section("")
    const title = el("h1", "article-title", item.title)
    title.dir = "auto"
    body.append(title, article(item.body_html))
    const nodes: Node[] = [body]

    const verdict = section(`01 — ${t(msg`Was this helpful?`)}`)
    const given = reactions.get(item.id)
    const group = el("div", "seg")
    group.setAttribute("role", "group")
    group.setAttribute("aria-label", t(msg`Was this helpful?`))
    for (const [rating, label] of [
      ["up", t(msg`Yes`)],
      ["down", t(msg`No`)],
    ] as const) {
      const button = textButton(label, "", () => rate(item, search, rating))
      button.setAttribute("aria-pressed", String(given === rating))
      group.append(button)
    }
    verdict.append(group)
    if (given) verdict.append(el("p", "note", t(msg`Thanks for your feedback`)))
    nodes.push(verdict)

    if (item.related.length > 0) nodes.push(list(`02 — ${t(msg`Related articles`)}`, item.related))
    nodes.push(el("div", "fill"), messageUs(item.id))
    return nodes
  }

  function render(): void {
    if (destroyed) return
    const nodes: Node[] = []
    if (screen.kind === "home") nodes.push(...homeNodes())
    else {
      nodes.push(backRow())
      nodes.push(
        ...(screen.kind === "collection" ? collectionNodes(screen.id) : articleNodes(screen.id, screen.search))
      )
    }
    const input = field.querySelector("input")
    const focused = (root.getRootNode() as Document | ShadowRoot).activeElement === input
    root.replaceChildren(...nodes)
    if (focused) input?.focus({ preventScroll: true })
    root.scrollTop = 0
  }

  function go(next: Screen): void {
    stack.push(screen)
    screen = next
    if (next.kind === "collection" && !collections.get(next.id)?.data) void loadCollection(next.id)
    if (next.kind === "article") void loadArticle(next.id)
    render()
  }

  function back(): void {
    const previous = stack.pop()
    if (!previous) return context.back()
    screen = previous
    render()
  }

  async function search(final: boolean): Promise<void> {
    const q = query.trim()
    clearTimeout(timer)
    if (q.length < minQuery) return
    const mine = ++sequence
    try {
      const found = await help.search(q, { locale, limit: 8, final })
      if (mine !== sequence || destroyed) return
      results = found
      searched = q
      searchFailed = false
    } catch {
      if (mine !== sequence || destroyed) return
      searchFailed = true
      searched = q
    }
    render()
  }

  async function loadCenter(): Promise<void> {
    center = {}
    render()
    try {
      center = { data: await help.get({ locale, page: pageUrl() }) }
    } catch {
      center = { failed: true }
    }
    render()
  }

  async function loadCollection(id: string): Promise<void> {
    collections.set(id, {})
    render()
    try {
      collections.set(id, { data: await help.collection(id, { locale }) })
    } catch {
      collections.set(id, { failed: true })
    }
    render()
  }

  async function loadArticle(id: string): Promise<void> {
    if (articles.get(id)?.data) return
    articles.set(id, {})
    render()
    try {
      articles.set(id, { data: await help.article(id, { locale }) })
    } catch {
      articles.set(id, { failed: true })
    }
    render()
  }

  if (screen.kind === "home") void loadCenter()
  else void loadArticle(screen.id)
  if (query.trim().length >= minQuery) void search(true)
  return () => {
    destroyed = true
    clearTimeout(timer)
  }
}

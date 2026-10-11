export const MERGE_FIELDS = [
  "contact.first_name",
  "contact.name",
  "contact.email",
  "inbox.name",
  "list.name",
  "unsubscribe_url",
  "preferences_url",
] as const

export const ATTRIBUTE_PREFIX = "contact.attributes."

const ATTRIBUTE_KEY = /^[A-Za-z0-9_.-]{1,64}$/

const MERGE_SOURCE = /\{\{\s*([^|}\s]+)\s*(?:\|\s*"([^"]*)"\s*)?\}\}/.source

function mergeMatches(text: string) {
  return [...text.matchAll(new RegExp(MERGE_SOURCE, "g"))]
}

export function knownField(field: string) {
  if ((MERGE_FIELDS as readonly string[]).includes(field)) return true
  return field.startsWith(ATTRIBUTE_PREFIX) && ATTRIBUTE_KEY.test(field.slice(ATTRIBUTE_PREFIX.length))
}

export function mergeToken(field: string, fallback?: string) {
  const f = fallback?.replace(/"/g, "").slice(0, 100)
  return f ? `{{ ${field} | "${f}" }}` : `{{ ${field} }}`
}

export type MergeUse = { field: string; fallback?: string }

export function mergeFields(text: string): MergeUse[] {
  return mergeMatches(text).map((m) => ({ field: m[1], fallback: m[2] || undefined }))
}

const BLOCK_TAGS = new Set(["p", "ul", "ol", "li"])
const INLINE_TAGS = new Set(["strong", "em", "u", "s", "a", "code"])
const RENAME: Record<string, string> = { b: "strong", i: "em", strike: "s", del: "s" }

function escape(s: string) {
  return s
    .replace(/&/g, "&amp;")
    .replace(/'/g, "&#39;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&#34;")
    .replace(/\r/g, "&#13;")
}

const SAFE_HREF = /^(https?:|mailto:|tel:)/i

function serialize(node: Node, inline: boolean): string {
  if (node.nodeType === Node.TEXT_NODE) return escape(node.textContent ?? "")
  if (!(node instanceof Element)) return ""
  const field = node.getAttribute("data-merge-field")
  if (field !== null) return escape(mergeToken(field, node.getAttribute("data-fallback") ?? undefined))
  const inner = () => Array.from(node.childNodes, (c) => serialize(c, inline)).join("")
  const raw = node.tagName.toLowerCase()
  const tag = RENAME[raw] ?? raw
  if (tag === "br") return "<br>"
  if (tag === "a") {
    const href = node.getAttribute("href")?.trim() ?? ""
    if (!SAFE_HREF.test(href)) return inner()
    return `<a href="${escape(href)}">${inner()}</a>`
  }
  if (INLINE_TAGS.has(tag)) return `<${tag}>${inner()}</${tag}>`
  if (BLOCK_TAGS.has(tag) && !inline) return `<${tag}>${inner()}</${tag}>`
  return inner()
}

function parse(html: string) {
  return new DOMParser().parseFromString(`<body>${html}</body>`, "text/html").body
}

export function canonical(html: string, inline = false) {
  return Array.from(parse(html).childNodes, (c) => serialize(c, inline)).join("")
}

export function stripParagraph(html: string) {
  const body = parse(html)
  const first = body.firstElementChild
  if (body.childElementCount === 1 && first?.tagName === "P") return canonical(first.innerHTML, true)
  return canonical(html, true)
}

export function plainText(html: string) {
  const body = parse(html)
  for (const el of body.querySelectorAll("[data-merge-field]"))
    el.replaceWith(mergeToken(el.getAttribute("data-merge-field") ?? "", el.getAttribute("data-fallback") ?? undefined))
  for (const br of body.querySelectorAll("br")) br.replaceWith(" ")
  return (body.textContent ?? "").replace(/\s+/g, " ").trim()
}

function chip(doc: Document, field: string, fallback?: string) {
  const span = doc.createElement("span")
  span.setAttribute("data-merge-field", field)
  if (fallback) span.setAttribute("data-fallback", fallback)
  span.textContent = field
  return span
}

export function withChips(html: string) {
  const body = parse(html)
  const doc = body.ownerDocument
  const walker = doc.createTreeWalker(body, NodeFilter.SHOW_TEXT)
  const texts: Text[] = []
  while (walker.nextNode()) texts.push(walker.currentNode as Text)
  for (const node of texts) {
    const value = node.data
    const matches = mergeMatches(value)
    if (matches.length === 0) continue
    const frag = doc.createDocumentFragment()
    let last = 0
    for (const m of matches) {
      if (m.index > last) frag.append(value.slice(last, m.index))
      frag.append(chip(doc, m[1], m[2] || undefined))
      last = m.index + m[0].length
    }
    if (last < value.length) frag.append(value.slice(last))
    node.replaceWith(frag)
  }
  return body.innerHTML
}

export function textWithChips(text: string) {
  const doc = document.implementation.createHTMLDocument("")
  const p = doc.createElement("p")
  let last = 0
  for (const m of mergeMatches(text)) {
    if (m.index > last) p.append(text.slice(last, m.index))
    p.append(chip(doc, m[1], m[2] || undefined))
    last = m.index + m[0].length
  }
  if (last < text.length) p.append(text.slice(last))
  return p.outerHTML
}

export function textOf(html: string) {
  return (parse(canonical(html)).textContent ?? "").replace(/\s+/g, " ").trim()
}

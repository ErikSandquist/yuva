import { inlineNodes } from "../panel/text"

const linkPattern = /\[([^\]\n]+)\]\(([^)\s]+)\)/g

function inline(text: string): Node[] {
  const nodes: Node[] = []
  let last = 0
  for (const match of text.matchAll(linkPattern)) {
    const index = match.index ?? 0
    if (index > last) nodes.push(...inlineNodes(text.slice(last, index)))
    const [whole, label = "", href = ""] = match
    let url: URL | null = null
    try {
      url = new URL(href)
    } catch {}
    if (url && (url.protocol === "http:" || url.protocol === "https:")) {
      const a = document.createElement("a")
      a.href = url.href
      a.target = "_blank"
      a.rel = "nofollow ugc noopener"
      a.append(...inlineNodes(label))
      nodes.push(a)
    } else nodes.push(document.createTextNode(whole))
    last = index + whole.length
  }
  if (last < text.length) nodes.push(...inlineNodes(text.slice(last)))
  return nodes
}

function block(tag: string, lines: string[]): HTMLElement {
  const element = document.createElement(tag)
  element.append(...inline(lines.join(" ")))
  return element
}

// A preview built from DOM nodes and text only; what is published is the server's body_html.
export function preview(markdown: string): DocumentFragment {
  const fragment = document.createDocumentFragment()
  const lines = markdown.replace(/\r\n?/g, "\n").split("\n")
  let paragraph: string[] = []
  let list: HTMLElement | null = null
  let quote: string[] = []
  const flush = () => {
    if (paragraph.length) fragment.append(block("p", paragraph))
    if (quote.length) {
      const element = document.createElement("blockquote")
      element.append(preview(quote.join("\n")))
      fragment.append(element)
    }
    paragraph = []
    quote = []
    list = null
  }
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i] ?? ""
    const fence = /^\s*(```|~~~)/.exec(line)
    if (fence) {
      flush()
      const code: string[] = []
      while (++i < lines.length && !(lines[i] ?? "").trimStart().startsWith(fence[1] ?? "```"))
        code.push(lines[i] ?? "")
      const pre = document.createElement("pre")
      pre.append(Object.assign(document.createElement("code"), { textContent: code.join("\n") }))
      fragment.append(pre)
      continue
    }
    if (/^\s*$/.test(line)) {
      flush()
      continue
    }
    const quoted = /^\s*>\s?(.*)$/.exec(line)
    if (quoted) {
      if (paragraph.length || list) flush()
      quote.push(quoted[1] ?? "")
      continue
    }
    const heading = /^\s*(#{1,6})\s+(.*)$/.exec(line)
    if (heading) {
      flush()
      fragment.append(block(`h${Math.min(6, (heading[1]?.length ?? 1) + 2)}`, [heading[2] ?? ""]))
      continue
    }
    const item = /^\s*(?:([-*+])|(\d+)[.)])\s+(.*)$/.exec(line)
    if (item) {
      const tag = item[1] ? "ul" : "ol"
      if (paragraph.length || quote.length || (list && list.tagName.toLowerCase() !== tag)) flush()
      if (!list) {
        list = document.createElement(tag)
        fragment.append(list)
      }
      list.append(block("li", [item[3] ?? ""]))
      continue
    }
    if (quote.length) flush()
    if (list && /^\s{2,}/.test(line)) {
      list.lastElementChild?.append(" ", ...inline(line.trim()))
      continue
    }
    list = null
    paragraph.push(line.trim())
  }
  flush()
  return fragment
}

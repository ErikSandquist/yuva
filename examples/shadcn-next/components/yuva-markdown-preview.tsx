import * as React from "react"

const inlinePattern =
  /\[([^\]\n]+)\]\(([^)\s]+)\)|`([^`\n]+)`|\*\*([^*\n]+)\*\*|__([^_\n]+)__|\*([^*\n]+)\*|_([^_\n]+)_|~~([^~\n]+)~~/g

function safeUrl(href: string): string | null {
  try {
    const url = new URL(href)
    return url.protocol === "http:" || url.protocol === "https:" ? url.href : null
  } catch {
    return null
  }
}

function inline(text: string, key: string): React.ReactNode[] {
  const nodes: React.ReactNode[] = []
  let last = 0
  let n = 0
  for (const match of text.matchAll(inlinePattern)) {
    const index = match.index ?? 0
    if (index > last) nodes.push(text.slice(last, index))
    const k = `${key}.${n++}`
    const [whole, label, href, code, strong, strong2, em, em2, strike] = match
    if (label !== undefined && href !== undefined) {
      const url = safeUrl(href)
      nodes.push(
        url ? (
          <a key={k} href={url} target="_blank" rel="nofollow ugc noopener">
            {inline(label, k)}
          </a>
        ) : (
          whole
        )
      )
    } else if (code !== undefined) nodes.push(<code key={k}>{code}</code>)
    else if ((strong ?? strong2) !== undefined)
      nodes.push(<strong key={k}>{inline(strong ?? strong2 ?? "", k)}</strong>)
    else if ((em ?? em2) !== undefined) nodes.push(<em key={k}>{inline(em ?? em2 ?? "", k)}</em>)
    else if (strike !== undefined) nodes.push(<del key={k}>{inline(strike, k)}</del>)
    last = index + whole.length
  }
  if (last < text.length) nodes.push(text.slice(last))
  return nodes
}

// Built from React elements and text only, for the draft; what is published is the server's body_html.
export function markdownPreview(markdown: string, key = "p"): React.ReactNode[] {
  const out: React.ReactNode[] = []
  const lines = markdown.replace(/\r\n?/g, "\n").split("\n")
  let paragraph: string[] = []
  let quote: string[] = []
  let list: { ordered: boolean; items: string[] } | null = null
  const id = () => `${key}.${out.length}`
  const flush = () => {
    if (paragraph.length) out.push(<p key={id()}>{inline(paragraph.join(" "), id())}</p>)
    if (quote.length) out.push(<blockquote key={id()}>{markdownPreview(quote.join("\n"), id())}</blockquote>)
    if (list) {
      const k = id()
      const items = list.items.map((item, i) => <li key={i}>{inline(item, `${k}.${i}`)}</li>)
      out.push(list.ordered ? <ol key={k}>{items}</ol> : <ul key={k}>{items}</ul>)
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
      out.push(
        <pre key={id()}>
          <code>{code.join("\n")}</code>
        </pre>
      )
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
      const level = Math.min(6, (heading[1]?.length ?? 1) + 2)
      out.push(React.createElement(`h${level}`, { key: id() }, inline(heading[2] ?? "", id())))
      continue
    }
    const item = /^\s*(?:([-*+])|(\d+)[.)])\s+(.*)$/.exec(line)
    if (item) {
      const ordered = !item[1]
      if (paragraph.length || quote.length || (list && list.ordered !== ordered)) flush()
      list ??= { ordered, items: [] }
      list.items.push(item[3] ?? "")
      continue
    }
    if (quote.length) flush()
    if (list && /^\s{2,}/.test(line)) {
      list.items[list.items.length - 1] += ` ${line.trim()}`
      continue
    }
    if (list) flush()
    paragraph.push(line.trim())
  }
  flush()
  return out
}

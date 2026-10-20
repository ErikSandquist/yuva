export type Format = "bold" | "italic" | "code" | "heading" | "bullets" | "numbers" | "quote" | "link" | "codeblock"

type Edit = { from: number; to: number; insert: string; start: number; end: number }

function replace(el: HTMLTextAreaElement, e: Edit) {
  el.focus()
  el.setSelectionRange(e.from, e.to)
  if (!document.execCommand("insertText", false, e.insert)) {
    el.setRangeText(e.insert, e.from, e.to, "end")
    el.dispatchEvent(new Event("input", { bubbles: true }))
  }
  el.setSelectionRange(e.start, e.end)
}

function wrap(text: string, from: number, to: number, mark: string, placeholder: string): Edit {
  const selected = text.slice(from, to)
  const around = text.slice(from - mark.length, from) === mark && text.slice(to, to + mark.length) === mark
  if (around) {
    return {
      from: from - mark.length,
      to: to + mark.length,
      insert: selected,
      start: from - mark.length,
      end: to - mark.length,
    }
  }
  const inner = selected || placeholder
  return { from, to, insert: mark + inner + mark, start: from + mark.length, end: from + mark.length + inner.length }
}

function lines(text: string, from: number, to: number, kind: "heading" | "bullets" | "numbers" | "quote"): Edit {
  const begin = text.lastIndexOf("\n", from - 1) + 1
  const nl = text.indexOf("\n", to)
  const end = nl === -1 ? text.length : nl
  const rows = text.slice(begin, end).split("\n")
  let out: string[]
  if (kind === "heading") {
    out = rows.map((r) => {
      if (r.startsWith("### ")) return r.slice(4)
      if (r.startsWith("## ")) return `### ${r.slice(3)}`
      return `## ${r.replace(/^#+\s*/, "")}`
    })
  } else {
    const prefix = (i: number) => (kind === "bullets" ? "- " : kind === "numbers" ? `${i + 1}. ` : "> ")
    const has = (r: string) =>
      kind === "bullets" ? /^[-*] /.test(r) : kind === "numbers" ? /^\d+\. /.test(r) : r.startsWith("> ")
    const strip = (r: string) => (kind === "numbers" ? r.replace(/^\d+\. /, "") : r.slice(2))
    out = rows.every(has) ? rows.map(strip) : rows.map((r, i) => prefix(i) + r)
  }
  const insert = out.join("\n")
  return { from: begin, to: end, insert, start: begin, end: begin + insert.length }
}

export function format(el: HTMLTextAreaElement, kind: Format) {
  const text = el.value
  const from = el.selectionStart
  const to = el.selectionEnd
  if (kind === "bold") return replace(el, wrap(text, from, to, "**", "bold"))
  if (kind === "italic") return replace(el, wrap(text, from, to, "_", "italic"))
  if (kind === "code") return replace(el, wrap(text, from, to, "`", "code"))
  if (kind === "link") {
    const label = text.slice(from, to) || "text"
    const insert = `[${label}](https://)`
    const start = from + label.length + 3
    return replace(el, { from, to, insert, start, end: start + "https://".length })
  }
  if (kind === "codeblock") {
    const selected = text.slice(from, to)
    const lead = from > 0 && text[from - 1] !== "\n" ? "\n" : ""
    const inner = selected || "code"
    const insert = `${lead}\`\`\`\n${inner}\n\`\`\`\n`
    const start = from + lead.length + 4
    return replace(el, { from, to, insert, start, end: start + inner.length })
  }
  return replace(el, lines(text, from, to, kind))
}

export function insertBlock(el: HTMLTextAreaElement, markdown: string) {
  const text = el.value
  const at = el.selectionEnd
  const lead = at > 0 && text[at - 1] !== "\n" ? "\n\n" : ""
  const tail = text[at] === undefined || text[at] === "\n" ? "\n" : "\n\n"
  const insert = `${lead}${markdown}${tail}`
  replace(el, { from: at, to: at, insert, start: at + insert.length, end: at + insert.length })
}

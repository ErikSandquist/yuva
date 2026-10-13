import fs from "node:fs"
import path from "node:path"
import { Marked, type Tokens } from "marked"
import { REPO } from "@/lib/routes"

export type Kind =
  | "Added"
  | "Changed"
  | "Deprecated"
  | "Removed"
  | "Fixed"
  | "Security"
  | "Performance"
  | "Breaking Changes"
export type Item = { title?: string; html: string }
export type Group = { kind: Kind; items: Item[] }
export type Release = { version: string; date: string; intro: string; groups: Group[]; compare?: string; tag: string }

const escape = (s: string) =>
  s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;")

const marked = new Marked({
  renderer: {
    html: ({ text }) => escape(text),
    image: ({ text }) => escape(text),
    link({ href, title, tokens }) {
      const inner = this.parser.parseInline(tokens)
      if (!/^(https?:\/\/|mailto:|#|\/)/i.test(href)) return inner
      return `<a href="${escape(href)}"${title ? ` title="${escape(title)}"` : ""}>${inner}</a>`
    },
  },
})

const heading = /^## \[([^\]]+)\](?: - (\d{4}-\d{2}-\d{2}))?\s*$/gm
const reference = /^\[([^\]]+)\]: (\S+)\s*$/gm
const titled = /^\*\*(.+?)\*\*:?\s*/
const capital = (s: string) => s.replace(/^[a-z]/, (c) => c.toUpperCase())

function groups(md: string) {
  const out: Group[] = []
  const intro: string[] = []
  for (const token of marked.lexer(md)) {
    if (token.type === "heading" && token.depth === 3) out.push({ kind: token.text as Kind, items: [] })
    else if (token.type === "list" && out.length)
      out.at(-1)!.items.push(
        ...(token as Tokens.List).items.map((li) => {
          const text = li.text.replace(/\s*\n\s*/g, " ")
          const m = text.match(titled)
          return m
            ? { title: m[1], html: marked.parseInline(capital(text.slice(m[0].length)), { async: false }) }
            : { html: marked.parseInline(text, { async: false }) }
        })
      )
    else if (token.type === "paragraph" && !out.length) intro.push(marked.parseInline(token.text, { async: false }))
  }
  return { groups: out, intro: intro.join(" ") }
}

function changelogReleases(): Release[] {
  const text = fs.readFileSync(path.resolve("../CHANGELOG.md"), "utf8")
  const refs = new Map([...text.matchAll(reference)].map((m) => [m[1]!, m[2]!]))
  const body = text.replace(reference, "")
  const marks = [...body.matchAll(heading)]
  return marks
    .map((m, i) => {
      const start = m.index! + m[0].length
      const end = i + 1 < marks.length ? marks[i + 1]!.index! : body.length
      return { version: m[1]!, date: m[2] ?? "", md: body.slice(start, end).trim() }
    })
    .filter((r) => r.version !== "Unreleased" && r.date)
    .map((r) => ({
      version: r.version,
      date: r.date,
      ...groups(r.md),
      compare: refs.get(r.version),
      tag: `${REPO}/releases/tag/v${r.version}`,
    }))
}

type Api = {
  tag_name: string
  draft: boolean
  published_at: string | null
  body: string | null
}

const kinds: Record<string, Kind> = {
  Features: "Added",
  "Bug Fixes": "Fixed",
  Performance: "Performance",
  "Breaking Changes": "Breaking Changes",
}
const commitTail = /\s*&nbsp;-&nbsp;\s*by .*?\[<samp>\(([0-9a-f]+)\)<\/samp>\]\((https:\/\/github\.com\/[^)\s]+)\)\s*$/s
const scoped = /^\*\*([^*]+)\*\*:\s*(.*)$/s

function commit(raw: string, scope?: string): Item {
  const text = raw.replace(/\s*\n\s*/g, " ")
  const m = text.match(commitTail)
  const subject = m ? text.slice(0, m.index) : text
  const sha = m ? ` <a href="${escape(m[2]!)}"><code>${escape(m[1]!)}</code></a>` : ""
  const prefix = scope ? `<code>${escape(scope)}</code> ` : ""
  return { html: `${prefix}${marked.parseInline(capital(subject), { async: false })}${sha}` }
}

function githubGroups(md: string) {
  const out: Group[] = []
  for (const token of marked.lexer(md)) {
    if (token.type === "heading" && token.depth === 3) {
      const name = token.text
        .replace(/&nbsp;/g, " ")
        .replace(/[^\p{L}\p{N} ]/gu, "")
        .trim()
      out.push({ kind: kinds[name] ?? (name as Kind), items: [] })
    } else if (token.type === "list" && out.length) {
      for (const li of (token as Tokens.List).items) {
        const sub = li.tokens.find((t): t is Tokens.List => t.type === "list")
        if (sub) {
          const scope = li.text.split("\n")[0]!.match(/^\*\*([^*]+)\*\*/)?.[1]
          out.at(-1)!.items.push(...sub.items.map((c) => commit(c.text, scope)))
        } else {
          const m = li.text.match(scoped)
          out.at(-1)!.items.push(m ? commit(m[2]!, m[1]) : commit(li.text))
        }
      }
    }
  }
  return out.filter((g) => g.items.length)
}

function githubRelease(r: Api): Release {
  const version = r.tag_name.replace(/^v/, "")
  const body = r.body ?? ""
  const compare = body.match(/\((https:\/\/github\.com\/productdevbook\/yuva\/compare\/[^)\s]+)\)/)?.[1]
  return {
    version,
    date: r.published_at!.slice(0, 10),
    intro: "",
    groups: githubGroups(body),
    compare,
    tag: `${REPO}/releases/tag/${r.tag_name}`,
  }
}

async function fetchReleases(): Promise<Release[]> {
  const headers: Record<string, string> = {
    Accept: "application/vnd.github+json",
    "X-GitHub-Api-Version": "2022-11-28",
  }
  if (process.env.GITHUB_TOKEN) headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`
  const all: Api[] = []
  for (let page = 1; ; page++) {
    const res = await fetch(`https://api.github.com/repos/productdevbook/yuva/releases?per_page=100&page=${page}`, {
      headers,
    })
    if (!res.ok) throw new Error(`GitHub releases: HTTP ${res.status}`)
    const batch = (await res.json()) as Api[]
    all.push(...batch)
    if (batch.length < 100) break
  }
  return all.filter((r) => !r.draft && r.published_at && /^v\d+\.\d+\.\d+/.test(r.tag_name)).map(githubRelease)
}

const order = (a: string, b: string) => {
  const x = a.split(/[.-]/).map(Number)
  const y = b.split(/[.-]/).map(Number)
  for (let i = 0; i < 3; i++) if (x[i] !== y[i]) return (y[i] ?? 0) - (x[i] ?? 0)
  return 0
}

async function load(): Promise<Release[]> {
  const local = changelogReleases()
  let remote: Release[] = []
  try {
    remote = await fetchReleases()
    if (!remote.length) throw new Error("GitHub returned no releases")
  } catch (error) {
    if (process.env.CI) throw new Error(`Cannot build the releases page: ${(error as Error).message}`)
    console.warn(`[releases] ${(error as Error).message}; using CHANGELOG.md only`)
  }
  const known = new Set(local.map((r) => r.version))
  return [...local, ...remote.filter((r) => !known.has(r.version))].sort((a, b) => order(a.version, b.version))
}

let cache: Promise<Release[]> | undefined
export const releases = () => (cache ??= load())

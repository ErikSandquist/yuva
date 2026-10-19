import type { MessageDescriptor } from "@lingui/core"
import { msg } from "@lingui/core/macro"
import {
  call,
  type Fetch,
  type PublicStatusComponent,
  type PublicStatusDay,
  type PublicStatusPage,
  type StatusState,
} from "../client/api"
import { el } from "../docs/dom"
import { catalogs } from "../catalogs/status"
import { messages as en } from "../locales/en/status"
import { setStyles } from "../styles"
import { directionOf, localeIn, translate } from "../translate"

type Tone = "ok" | "warn" | "partial" | "maint" | "bad"

const days = 90
const refreshMs = 60_000
const fetcher: Fetch = (url, init) => fetch(url, init)
const svgNS = "http://www.w3.org/2000/svg"

const tones: Record<StatusState, Tone> = {
  operational: "ok",
  degraded: "warn",
  partial_outage: "partial",
  maintenance: "maint",
  major_outage: "bad",
}

const severity: StatusState[] = ["operational", "maintenance", "degraded", "partial_outage", "major_outage"]
const ongoing = new Set(["investigating", "identified", "monitoring", "in_progress"])

const Base = (typeof HTMLElement === "undefined" ? class {} : HTMLElement) as typeof HTMLElement

export const scriptSrc =
  typeof document !== "undefined" && document.currentScript instanceof HTMLScriptElement
    ? document.currentScript.src
    : ""

const light = `--paper:var(--yuva-status-paper,transparent);--ink:var(--yuva-status-ink,#0b0b0f);--soft:var(--yuva-status-soft,#52525b);--faint:var(--yuva-status-faint,#71717a);--rule:var(--yuva-status-rule,#ececef);--cross:var(--yuva-status-cross,#a1a1aa);--ok:var(--yuva-status-ok,#16a34a);--warn:var(--yuva-status-warn,#eab308);--partial:var(--yuva-status-partial,#f97316);--maint:var(--yuva-status-maint,#2563eb);--bad:var(--yuva-status-bad,#dc2626);--ok-text:var(--yuva-status-ok-text,#52525b);--warn-text:var(--yuva-status-warn-text,#a16207);--bad-text:var(--yuva-status-bad-text,#b91c1c)`
const dark = `--paper:var(--yuva-status-paper,transparent);--ink:var(--yuva-status-ink,#fafafa);--soft:var(--yuva-status-soft,#a1a1aa);--faint:var(--yuva-status-faint,#8a8a93);--rule:var(--yuva-status-rule,#1f1f23);--cross:var(--yuva-status-cross,#52525b);--ok:var(--yuva-status-ok,#4ade80);--warn:var(--yuva-status-warn,#facc15);--partial:var(--yuva-status-partial,#fb923c);--maint:var(--yuva-status-maint,#60a5fa);--bad:var(--yuva-status-bad,#f87171);--ok-text:var(--yuva-status-ok-text,#a1a1aa);--warn-text:var(--yuva-status-warn-text,#facc15);--bad-text:var(--yuva-status-bad-text,#fca5a5)`

const styles = `
:host{display:block}
:host([hidden]),[hidden]{display:none!important}
*{box-sizing:border-box}
.root{${light};container-type:inline-size;font:14px/1.4 var(--yuva-status-font,inherit);color:var(--ink);background:var(--paper)}
.root.dark{${dark}}
@media (prefers-color-scheme:dark){.root.auto{${dark}}}
a{color:inherit;text-decoration:none}
a:hover{text-decoration:underline;text-underline-offset:3px}
:focus-visible{outline:2px solid var(--ink);outline-offset:-2px}
section{padding:0 var(--yuva-status-gutter,40px);border-top:1px solid var(--rule)}
.frame{position:relative;margin-block-end:24px;border:1px solid var(--rule);border-block-start:0}
.cross{position:absolute;inline-size:11px;block-size:11px;stroke:var(--cross);stroke-width:1}
.cross.t{top:-6px}
.cross.b{bottom:-6px}
.cross.s{left:-6px}
.cross.e{right:-6px}
.bar{display:grid;grid-template-columns:minmax(0,1fr) 360px auto}
.cell{display:flex;align-items:center;gap:12px;min-width:0;min-height:52px;padding:0 20px;background:var(--paper)}
.bar>.cell+.cell{border-inline-start:1px solid var(--rule)}
.dot{flex:none;inline-size:8px;block-size:8px;background:var(--tone)}
.text{display:flex;gap:12px;min-width:0;flex:1}
.title{flex:none;max-inline-size:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-weight:600}
.line{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--soft);font-size:13px}
.span{gap:14px}
.span .label{font:500 10px ui-monospace,SFMono-Regular,Menlo,monospace;letter-spacing:1px;color:var(--faint);white-space:nowrap}
.strip{flex:1;min-inline-size:0;block-size:12px;display:block}
.strip path{fill:var(--tone)}
.uptime{font-size:13px;font-weight:600;font-variant-numeric:tabular-nums;white-space:nowrap}
.by{min-inline-size:max-content;gap:6px;font-size:12px;color:var(--faint);white-space:nowrap}
.by svg{flex:none;inline-size:14px;block-size:14px;display:block}
.parts{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));border-block-start:1px solid var(--rule)}
.part{justify-content:space-between;align-items:baseline;gap:8px;min-block-size:44px;padding-block:12px;font-size:13px}
.part:not(:nth-child(4n+1)),.blank:not(:nth-child(4n+1)){border-inline-start:1px solid var(--rule)}
.part .name,.part .state{display:flex;align-items:center;block-size:20px;line-height:20px}
.part .name{min-width:0;display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.blank{min-block-size:44px}
.part .state{flex:none;align-items:center;gap:6px;font-size:12px;color:var(--state-text,var(--soft))}
.part .dot{inline-size:6px;block-size:6px}
.ok{--tone:var(--ok)}.warn{--tone:var(--warn)}.partial{--tone:var(--partial)}.maint{--tone:var(--maint)}.bad{--tone:var(--bad)}
.part.ok{--state-text:var(--ok-text)}.part.warn{--state-text:var(--warn-text)}.part.partial,.part.bad{--state-text:var(--bad-text)}
@container (max-width:720px){
.bar{grid-template-columns:minmax(0,1fr)}
.bar>.cell+.cell{border-inline-start:0;border-block-start:1px solid var(--rule)}
.parts{grid-template-columns:repeat(2,minmax(0,1fr))}
.part:not(:nth-child(4n+1)),.blank:not(:nth-child(4n+1)){border-inline-start:0}
.blank.wide{display:none}
.part:nth-child(2n),.blank:nth-child(2n){border-inline-start:1px solid var(--rule)}
}
@container (max-width:600px){
section{padding:0 16px}
.cross{display:none}
.text{flex-direction:column;gap:2px;padding-block:8px}
.cell{padding:0 16px}
}
`

export interface StatusOverview {
  state: StatusState
  title: string
  line: string
}

export function mergeDays(components: PublicStatusComponent[]): PublicStatusDay[] {
  const byDate = new Map<string, { state: StatusState; sum: number; count: number }>()
  for (const component of components) {
    for (const day of component.days ?? []) {
      const entry = byDate.get(day.date) ?? { state: "operational", sum: 0, count: 0 }
      if (severity.indexOf(day.state) > severity.indexOf(entry.state)) entry.state = day.state
      if (day.uptime !== undefined) {
        entry.sum += day.uptime
        entry.count++
      }
      byDate.set(day.date, entry)
    }
  }
  return [...byDate.entries()]
    .sort(([a], [b]) => (a < b ? -1 : 1))
    .slice(-days)
    .map(([date, { state, sum, count }]) => ({ date, state, uptime: count ? sum / count : undefined }))
}

export function meanUptime(list: PublicStatusDay[]): number | undefined {
  const values = list.flatMap((day) => (day.uptime === undefined ? [] : [day.uptime]))
  return values.length ? values.reduce((a, b) => a + b, 0) / values.length : undefined
}

export class YuvaStatusElement extends Base {
  static observedAttributes = ["page", "server", "theme", "components", "lang"]

  #root: ShadowRoot
  #connected = false
  #generation = 0
  #data: PublicStatusPage | null = null
  #timer: ReturnType<typeof setInterval> | undefined
  #loadedAt = 0

  constructor() {
    super()
    this.#root = this.attachShadow({ mode: "open" })
  }

  get page(): string | null {
    return this.getAttribute("page")
  }

  set page(value: string | null) {
    this.#attribute("page", value)
  }

  get server(): string {
    const base = this.getAttribute("server") || (scriptSrc ? new URL(scriptSrc).origin : location.origin)
    return base.replace(/\/+$/, "")
  }

  set server(value: string | null) {
    this.#attribute("server", value)
  }

  get locale(): string {
    return localeIn(catalogs, [this.getAttribute("lang"), document.documentElement.lang, navigator.language])
  }

  connectedCallback(): void {
    this.#connected = true
    document.addEventListener("visibilitychange", this.#visibility)
    this.#timer = setInterval(() => void this.#refresh(), refreshMs)
    void this.#load()
  }

  disconnectedCallback(): void {
    this.#connected = false
    document.removeEventListener("visibilitychange", this.#visibility)
    clearInterval(this.#timer)
  }

  attributeChangedCallback(name: string, previous: string | null, value: string | null): void {
    if (!this.#connected || previous === value) return
    if (name === "page" || name === "server") return void this.#load()
    this.#render()
  }

  #visibility = (): void => {
    if (document.visibilityState === "visible" && Date.now() - this.#loadedAt >= refreshMs) void this.#refresh()
  }

  #attribute(name: string, value: string | null): void {
    if (value === null || value === undefined || value === "") this.removeAttribute(name)
    else this.setAttribute(name, value)
  }

  #t(descriptor: MessageDescriptor): string {
    return translate(catalogs[this.locale] ?? en, descriptor, en)
  }

  #refresh(): Promise<void> {
    if (document.visibilityState !== "visible") return Promise.resolve()
    return this.#fetch(false)
  }

  #load(): Promise<void> {
    this.#data = null
    this.#render()
    return this.#fetch(true)
  }

  async #fetch(first: boolean): Promise<void> {
    const slug = this.page
    if (!slug) return
    const generation = ++this.#generation
    try {
      const data = await call<PublicStatusPage>(fetcher, this.server, `/client/v1/status/${encodeURIComponent(slug)}`)
      if (generation !== this.#generation) return
      this.#data = data
      this.#loadedAt = Date.now()
      this.#render()
    } catch {
      if (first && generation === this.#generation) console.warn(`Yuva: status page ${slug} could not be loaded`)
    }
  }

  #stateTitle(state: StatusState): string {
    if (state === "degraded") return this.#t(msg`Degraded performance`)
    if (state === "partial_outage") return this.#t(msg`Partial outage`)
    if (state === "major_outage") return this.#t(msg`Major outage`)
    if (state === "maintenance") return this.#t(msg`Under maintenance`)
    return this.#t(msg`All systems operational`)
  }

  #stateLabel(state: StatusState): string {
    if (state === "degraded") return this.#t(msg`Slow`)
    if (state === "partial_outage") return this.#t(msg`Partial`)
    if (state === "major_outage") return this.#t(msg`Down`)
    if (state === "maintenance") return this.#t(msg`Maintenance`)
    return this.#t(msg`OK`)
  }

  overview(data: PublicStatusPage): StatusOverview {
    const incident = data.open.find((item) => ongoing.has(item.state))
    if (incident) return { state: data.state, title: incident.title, line: incident.updates[0]?.body ?? "" }
    return {
      state: data.state,
      title: this.#stateTitle(data.state),
      line: data.state === "operational" ? this.#t(msg`Everything is working.`) : "",
    }
  }

  #cross(position: string): SVGElement {
    const svg = document.createElementNS(svgNS, "svg")
    svg.setAttribute("class", `cross ${position}`)
    svg.setAttribute("viewBox", "0 0 11 11")
    svg.setAttribute("aria-hidden", "true")
    const path = document.createElementNS(svgNS, "path")
    path.setAttribute("d", "M5.5 0v11M0 5.5h11")
    svg.append(path)
    return svg
  }

  #strip(list: PublicStatusDay[], uptime: string): SVGElement {
    const svg = document.createElementNS(svgNS, "svg")
    svg.setAttribute("class", "strip")
    svg.setAttribute("viewBox", `0 0 ${days * 2} 8`)
    svg.setAttribute("preserveAspectRatio", "none")
    svg.setAttribute("shape-rendering", "crispEdges")
    svg.setAttribute("role", "img")
    svg.setAttribute("aria-label", uptime ? this.#t(msg`Last 90 days, ${uptime} uptime`) : this.#t(msg`Last 90 days`))
    const paths: Record<Tone, string> = { ok: "", warn: "", partial: "", maint: "", bad: "" }
    const offset = days - list.length
    list.forEach((day, index) => {
      paths[tones[day.state]] += `M${(offset + index) * 2} 0h1.4v8h-1.4Z`
    })
    for (const [tone, d] of Object.entries(paths)) {
      if (!d) continue
      const path = document.createElementNS(svgNS, "path")
      path.setAttribute("class", tone)
      path.setAttribute("d", d)
      svg.append(path)
    }
    return svg
  }

  #mark(): SVGElement {
    const svg = document.createElementNS(svgNS, "svg")
    svg.setAttribute("viewBox", "0 0 1024 1024")
    svg.setAttribute("aria-hidden", "true")
    const parts: [string, Record<string, string>][] = [
      ["rect", { width: "1024", height: "1024", rx: "230", fill: "#e4572e" }],
      [
        "path",
        {
          d: "M512 248c-160 0-288 104-288 240 0 74 38 140 98 184l-26 104 118-62c30 9 63 14 98 14 160 0 288-104 288-240S672 248 512 248z",
          fill: "#fff",
        },
      ],
      [
        "path",
        {
          d: "M392 470c0 66 54 120 120 120s120-54 120-120",
          fill: "none",
          stroke: "#e4572e",
          "stroke-width": "64",
          "stroke-linecap": "round",
        },
      ],
    ]
    for (const [tag, attributes] of parts) {
      const part = document.createElementNS(svgNS, tag)
      for (const [name, value] of Object.entries(attributes)) part.setAttribute(name, value)
      svg.append(part)
    }
    return svg
  }

  #render(): void {
    const data = this.#data
    if (!data) return void this.#root.replaceChildren()
    const locale = this.locale
    const theme = this.getAttribute("theme")
    const root = el("div", `root ${theme === "light" || theme === "dark" ? theme : "auto"}`)
    root.dir = directionOf(locale)
    root.lang = locale

    const overview = this.overview(data)
    const merged = mergeDays(data.components)
    const mean = meanUptime(merged)
    const uptime =
      mean === undefined
        ? ""
        : new Intl.NumberFormat(locale, {
            style: "percent",
            minimumFractionDigits: 2,
            maximumFractionDigits: 2,
          }).format(Math.floor(mean * 100 + 1e-9) / 10000)

    const section = el("section")
    const name = data.name
    section.setAttribute("aria-label", this.#t(msg`${name} system status`))
    const frame = el("div", "frame")
    frame.append(this.#cross("t s"), this.#cross("t e"), this.#cross("b s"), this.#cross("b e"))

    const bar = el("div", "bar")
    const state = el("a", `cell ${tones[overview.state]}`)
    if (/^https?:\/\//i.test(data.url)) state.href = data.url
    const dot = el("span", "dot")
    dot.setAttribute("aria-hidden", "true")
    const text = el("span", "text")
    text.append(el("span", "title", overview.title))
    if (overview.line) text.append(el("span", "line", overview.line))
    state.append(dot, text)

    const span = el("span", "cell span")
    span.append(el("span", "label", this.#t(msg`90 D`)), this.#strip(merged, uptime))
    if (uptime) span.append(el("span", "uptime", uptime))

    const by = el("a", "cell by")
    by.href = "https://useyuva.com"
    by.append(this.#mark(), this.#t(msg`Status by Yuva`))
    bar.append(state, span, by)
    frame.append(bar)

    if (this.hasAttribute("components") && this.getAttribute("components") !== "false" && data.components.length) {
      const parts = el("div", "parts")
      for (const component of data.components) {
        const part = el("span", `cell part ${tones[component.state]}`)
        const status = el("span", "state", this.#stateLabel(component.state))
        const marker = el("span", "dot")
        marker.setAttribute("aria-hidden", "true")
        status.append(marker)
        part.append(el("span", "name", component.name), status)
        parts.append(part)
      }
      const count = data.components.length
      for (let index = count; index % 4 !== 0; index++) {
        parts.append(el("span", index >= Math.ceil(count / 2) * 2 ? "cell blank wide" : "cell blank"))
      }
      frame.append(parts)
    }

    section.append(frame)
    root.append(section)
    const fallback = setStyles(this.#root, styles)
    this.#root.replaceChildren(...(fallback ? [fallback] : []), root)
  }
}

export function defineStatus(): void {
  if (typeof customElements === "undefined") return
  if (!customElements.get("yuva-status")) customElements.define("yuva-status", YuvaStatusElement)
}

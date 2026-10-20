import { msg } from "@lingui/core/macro"
import { call, type Fetch, type PublicStatusPage, type StatusState } from "../client/api"
import { el, textButton } from "../panel/helpers"
import { icons } from "../panel/icons"
import type { AppContext } from "../types"
import type { Translate } from "./apps"

const fetcher: Fetch = (url, init) => fetch(url, init)
const shownDays = 60

const overall = {
  operational: msg`All systems operational`,
  degraded: msg`Degraded performance`,
  partial_outage: msg`Partial outage`,
  major_outage: msg`Major outage`,
  maintenance: msg`Maintenance`,
}

const part = {
  operational: msg`OK`,
  degraded: msg`Slow`,
  partial_outage: msg`Partial`,
  major_outage: msg`Down`,
  maintenance: msg`Maintenance`,
}

const tone: Record<StatusState, string> = {
  operational: "",
  degraded: "warn",
  partial_outage: "bad",
  major_outage: "bad",
  maintenance: "maint",
}

const incidentState = {
  investigating: msg`Investigating`,
  identified: msg`Identified`,
  monitoring: msg`Monitoring`,
  resolved: msg`Resolved`,
  scheduled: msg`Scheduled`,
  in_progress: msg`In progress`,
  completed: msg`Completed`,
}

function slugOf(url: string): string | null {
  try {
    const match = /^\/status\/([^/]+)\/?$/.exec(new URL(url).pathname)
    return match?.[1] ? decodeURIComponent(match[1]) : null
  } catch {
    return null
  }
}

interface Incident {
  id: string
  title: string
  url: string
  state: keyof typeof incidentState
  at: string
}

export function statusApp(context: AppContext, t: Translate, root: HTMLElement): () => void {
  const status = context.inbox.status
  const { locale } = context
  let page: PublicStatusPage | null = null
  let destroyed = false

  function section(label: string): HTMLElement {
    const sec = el("section", "sec")
    if (label) sec.append(el("h2", "label", label))
    return sec
  }

  function date(value: string): string {
    return new Intl.DateTimeFormat(locale, { month: "short", day: "numeric" }).format(new Date(value))
  }

  function ago(value: string): string {
    const minutes = Math.max(0, Math.round((Date.now() - Date.parse(value)) / 60_000))
    const format = new Intl.RelativeTimeFormat(locale, { numeric: "auto" })
    if (minutes < 60) return format.format(-minutes, "minute")
    if (minutes < 1440) return format.format(-Math.round(minutes / 60), "hour")
    return format.format(-Math.round(minutes / 1440), "day")
  }

  function render(): void {
    if (destroyed || !status) return
    const state = page?.state ?? status.state
    const head = section("")
    const stat = el("div", "stat")
    const mark = el("span", `stat-mark ${tone[state]}`)
    mark.innerHTML = state === "operational" ? icons.check : icons.alert
    const words = el("div")
    words.append(el("b", "", t(overall[state])))
    if (page) words.append(el("span", "", ago(page.updated_at)))
    stat.append(mark, words)
    head.append(stat)
    const nodes: Node[] = [head]

    if (page && page.components.length > 0) {
      const sec = section(`01 — ${t(msg`Components`)}`)
      for (const component of page.components) {
        const row = el("div", "comp")
        const top = el("div", "comp-top")
        top.append(
          el("span", "", component.name),
          el(
            "span",
            "",
            component.uptime === undefined
              ? t(part[component.state])
              : new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 2 }).format(
                  component.uptime / 100
                )
          )
        )
        row.append(top)
        const days = (component.days ?? []).slice(-shownDays)
        if (days.length > 0) {
          const cells = el("div", "cells")
          cells.setAttribute("role", "img")
          cells.setAttribute("aria-label", `${component.name}, ${t(part[component.state])}`)
          for (const day of days) {
            const cell = el("i", tone[day.state])
            cell.title = `${date(day.date)}: ${t(part[day.state])}`
            cells.append(cell)
          }
          row.append(cells)
        }
        sec.append(row)
      }
      nodes.push(sec)
    }

    const incidents: Incident[] = page
      ? [...page.open, ...page.recent].slice(0, 4).map((incident) => ({
          id: incident.id,
          title: incident.title,
          url: incident.url,
          state: incident.state,
          at: incident.updated_at,
        }))
      : status.open.map((incident) => ({
          id: incident.id,
          title: incident.title,
          url: incident.url,
          state: incident.state,
          at: incident.updated_at,
        }))
    const list = section(`${page && page.components.length > 0 ? "02" : "01"} — ${t(msg`Incidents`)}`)
    if (incidents.length === 0) list.append(el("p", "hint", t(msg`No open incidents.`)))
    for (const incident of incidents) {
      const item = el("div", "inc")
      const link = el("a", "", incident.title)
      link.href = incident.url
      link.target = "_blank"
      link.rel = "noopener noreferrer"
      link.dir = "auto"
      item.append(link, el("span", "", `${t(incidentState[incident.state])} · ${date(incident.at)}`))
      list.append(item)
    }
    nodes.push(list)

    const foot = section("")
    const open = el("a", "link brand", t(msg`Open status page`))
    open.href = status.url
    open.target = "_blank"
    open.rel = "noopener noreferrer"
    foot.append(open)
    if (context.mailingLists)
      foot.append(textButton(t(msg`Get notified about incidents`), "primary", () => context.open("subscribe")))
    nodes.push(el("div", "fill"), foot)
    root.replaceChildren(...nodes)
  }

  render()
  const slug = status ? slugOf(status.url) : null
  if (slug)
    void call<PublicStatusPage>(fetcher, context.client.server, `/client/v1/status/${encodeURIComponent(slug)}`)
      .then((result) => {
        page = result
        render()
      })
      .catch(() => undefined)
  return () => {
    destroyed = true
  }
}

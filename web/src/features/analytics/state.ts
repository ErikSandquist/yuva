import { useSearchParams } from "react-router"

import type {
  AnalyticsCompare,
  AnalyticsDimension,
  AnalyticsFilter,
  AnalyticsFilterDimension,
  AnalyticsMetric,
  AnalyticsProperty,
  AnalyticsRange,
} from "@/lib/api"

export type Period = NonNullable<AnalyticsRange["period"]>
export type FilterOp = AnalyticsFilter["op"]

export const PERIODS: Period[] = [
  "today",
  "yesterday",
  "7d",
  "28d",
  "30d",
  "90d",
  "month",
  "last_month",
  "6mo",
  "12mo",
  "year",
]

export const DEFAULT_PERIOD: Period = "30d"

export const API_PERIODS: Period[] = [
  "15m",
  "1h",
  "6h",
  "24h",
  "today",
  "yesterday",
  "7d",
  "30d",
  "90d",
  "month",
  "last_month",
  "6mo",
  "12mo",
]

export const API_DEFAULT_PERIOD: Period = "24h"

export type Kind = AnalyticsProperty["kind"]

const periodsOf = (kind: Kind) => (kind === "api" ? API_PERIODS : PERIODS)
const defaultOf = (kind: Kind) => (kind === "api" ? API_DEFAULT_PERIOD : DEFAULT_PERIOD)

export const FILTER_DIMENSIONS: AnalyticsFilterDimension[] = [
  "page",
  "entry_page",
  "exit_page",
  "hostname",
  "channel",
  "source",
  "referrer",
  "utm_source",
  "utm_medium",
  "utm_campaign",
  "utm_term",
  "utm_content",
  "country",
  "region",
  "city",
  "device",
  "browser",
  "browser_version",
  "os",
  "os_version",
  "screen_size",
  "language",
  "platform",
  "app_version",
  "visitor_type",
  "event",
]

export const OPS: FilterOp[] = ["is", "is_not", "contains", "not_contains", "matches"]

export const KPIS: Exclude<AnalyticsMetric, "revenue">[] = [
  "visitors",
  "visits",
  "pageviews",
  "views_per_visit",
  "bounce_rate",
  "visit_duration",
]

export const HOURLY_METRICS: AnalyticsMetric[] = ["visitors", "visits", "pageviews", "events"]

const DATE = /^\d{4}-\d{2}-\d{2}$/

function validDate(s: string) {
  return DATE.test(s) && !Number.isNaN(Date.parse(`${s}T00:00:00Z`))
}

function customRange(raw: string): { from: string; to: string } | null {
  if (!raw.startsWith("custom:")) return null
  const [from, to] = raw.slice(7).split(",")
  if (!from || !to || !validDate(from) || !validDate(to) || to < from) return null
  return { from, to }
}

export function parseRange(raw: string | null, kind: Kind = "site"): AnalyticsRange {
  if (raw && periodsOf(kind).includes(raw as Period)) return { period: raw as Period }
  return (raw && customRange(raw)) || { period: defaultOf(kind) }
}

export function parseCompare(raw: string | null): AnalyticsCompare | undefined {
  if (raw === "previous" || raw === "year") return { with: raw }
  const custom = raw && customRange(raw)
  return custom ? { with: "range", ...custom } : undefined
}

const PROP = "prop."

export function parseFilters(values: string[]): AnalyticsFilter[] {
  const out: AnalyticsFilter[] = []
  for (const v of values) {
    const a = v.indexOf(":")
    const b = a < 0 ? -1 : v.indexOf(":", a + 1)
    if (b < 0) continue
    const head = v.slice(0, a)
    const op = v.slice(a + 1, b) as FilterOp
    if (!OPS.includes(op)) continue
    const value = v.slice(b + 1)
    if (head.startsWith(PROP)) {
      let prop = ""
      try {
        prop = decodeURIComponent(head.slice(PROP.length))
      } catch {
        continue
      }
      if (prop) out.push({ dimension: "prop", prop, op, values: [value] })
      continue
    }
    const dimension = head as AnalyticsFilterDimension
    if (!FILTER_DIMENSIONS.includes(dimension)) continue
    out.push({ dimension, op, values: [value] })
  }
  return out.slice(0, 10)
}

function filterParam(f: AnalyticsFilter) {
  const head = f.dimension === "prop" ? `${PROP}${encodeURIComponent(f.prop ?? "")}` : f.dimension
  return `${head}:${f.op}:${f.values[0] ?? ""}`
}

function sameTarget(a: AnalyticsFilter, b: AnalyticsFilter) {
  return a.dimension === b.dimension && (a.dimension !== "prop" || a.prop === b.prop)
}

export function rangeParam(range: AnalyticsRange) {
  return range.period ?? `custom:${range.from},${range.to}`
}

function today() {
  const d = new Date()
  return new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()))
}

export function isoDay(d: Date) {
  return d.toISOString().slice(0, 10)
}

export function todayIso() {
  return isoDay(today())
}

export function rangeDays(range: AnalyticsRange): number {
  if (!range.period) {
    return Math.round((Date.parse(`${range.to}T00:00:00Z`) - Date.parse(`${range.from}T00:00:00Z`)) / 86_400_000) + 1
  }
  const now = today()
  switch (range.period) {
    case "15m":
    case "1h":
    case "6h":
    case "24h":
    case "today":
    case "yesterday":
      return 1
    case "7d":
      return 7
    case "28d":
      return 28
    case "30d":
      return 30
    case "90d":
      return 90
    case "month":
      return now.getUTCDate()
    case "last_month":
      return new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 0)).getUTCDate()
    case "6mo":
      return 183
    case "12mo":
      return 365
    case "year":
      return Math.round((now.getTime() - Date.UTC(now.getUTCFullYear(), 0, 1)) / 86_400_000) + 1
  }
}

export function grainFor(range: AnalyticsRange, metric: AnalyticsMetric): AnalyticsDimension {
  const days = rangeDays(range)
  if (days <= 2 && HOURLY_METRICS.includes(metric)) return "time:hour"
  return days <= 90 ? "time:day" : "time:week"
}

export type View = {
  range: AnalyticsRange
  compare?: AnalyticsCompare
  filters: AnalyticsFilter[]
  segment?: string
  metric: Exclude<AnalyticsMetric, "revenue">
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

export function scopeOf(view: View): { filters: AnalyticsFilter[] } {
  return { filters: view.filters }
}

export function sameFilters(a: AnalyticsFilter[], b: AnalyticsFilter[]) {
  const key = (f: AnalyticsFilter) => filterParam(f)
  const x = a.map(key).sort()
  const y = b.map(key).sort()
  return x.length === y.length && x.every((v, i) => v === y[i])
}

export function apiGrainFor(range: AnalyticsRange): AnalyticsDimension {
  switch (range.period) {
    case "15m":
    case "1h":
    case "6h":
      return "time:minute"
    case "24h":
    case "today":
    case "yesterday":
      return "time:hour"
  }
  return rangeDays(range) <= 2 ? "time:hour" : "time:day"
}

export function useView(kind: Kind = "site") {
  const [params, setParams] = useSearchParams()
  const metric = params.get("metric") as View["metric"] | null
  const view: View = {
    range: parseRange(params.get("period"), kind),
    compare: parseCompare(params.get("compare")),
    filters: parseFilters(params.getAll("f")),
    segment: UUID.test(params.get("segment") ?? "") ? params.get("segment")! : undefined,
    metric: metric && KPIS.includes(metric) ? metric : "visitors",
  }
  const update = (fn: (p: URLSearchParams) => void) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        fn(next)
        return next
      },
      { replace: true }
    )
  const writeFilters = (p: URLSearchParams, filters: AnalyticsFilter[]) => {
    p.delete("f")
    for (const f of filters) p.append("f", filterParam(f))
  }
  return {
    view,
    setRange: (range: AnalyticsRange) =>
      update((p) => {
        const v = rangeParam(range)
        if (v === defaultOf(kind)) p.delete("period")
        else p.set("period", v)
      }),
    setCompare: (compare: AnalyticsCompare | undefined) =>
      update((p) => {
        if (!compare) p.delete("compare")
        else if (compare.with === "range") p.set("compare", `custom:${compare.from},${compare.to}`)
        else p.set("compare", compare.with)
      }),
    setMetric: (m: AnalyticsMetric) =>
      update((p) => {
        if (m === "visitors") p.delete("metric")
        else p.set("metric", m)
      }),
    addFilter: (dimension: AnalyticsFilterDimension, value: string, op: FilterOp = "is", prop?: string) =>
      update((p) => {
        const f: AnalyticsFilter = prop ? { dimension, prop, op, values: [value] } : { dimension, op, values: [value] }
        writeFilters(p, [...view.filters.filter((x) => !sameTarget(x, f)), f])
      }),
    setFilters: (filters: AnalyticsFilter[]) => update((p) => writeFilters(p, filters.slice(0, 10))),
    setSegment: (id: string | undefined, filters: AnalyticsFilter[]) =>
      update((p) => {
        if (id) p.set("segment", id)
        else p.delete("segment")
        writeFilters(p, filters.slice(0, 10))
      }),
    setFilter: (i: number, f: AnalyticsFilter) =>
      update((p) =>
        writeFilters(
          p,
          view.filters.map((x, j) => (j === i ? f : x))
        )
      ),
    removeFilter: (i: number) =>
      update((p) =>
        writeFilters(
          p,
          view.filters.filter((_, j) => j !== i)
        )
      ),
    clearFilters: () =>
      update((p) => {
        p.delete("f")
        p.delete("segment")
      }),
  }
}

export function useViewSearch(withSegment = false) {
  const [params] = useSearchParams()
  const keep = new URLSearchParams()
  for (const k of withSegment ? ["period", "compare", "f", "segment"] : ["period", "compare", "f"])
    for (const v of params.getAll(k)) keep.append(k, v)
  const s = keep.toString()
  return s ? `?${s}` : ""
}

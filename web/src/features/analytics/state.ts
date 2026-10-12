import { useSearchParams } from "react-router"

import type {
  AnalyticsCompare,
  AnalyticsDimension,
  AnalyticsFilter,
  AnalyticsFilterDimension,
  AnalyticsMetric,
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
]

export const OPS: FilterOp[] = ["is", "is_not", "contains", "not_contains", "matches"]

export const KPIS: AnalyticsMetric[] = [
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

export function parseRange(raw: string | null): AnalyticsRange {
  if (raw && PERIODS.includes(raw as Period)) return { period: raw as Period }
  return (raw && customRange(raw)) || { period: DEFAULT_PERIOD }
}

export function parseCompare(raw: string | null): AnalyticsCompare | undefined {
  if (raw === "previous" || raw === "year") return { with: raw }
  const custom = raw && customRange(raw)
  return custom ? { with: "range", ...custom } : undefined
}

export function parseFilters(values: string[]): AnalyticsFilter[] {
  const out: AnalyticsFilter[] = []
  for (const v of values) {
    const a = v.indexOf(":")
    const b = a < 0 ? -1 : v.indexOf(":", a + 1)
    if (b < 0) continue
    const dimension = v.slice(0, a) as AnalyticsFilterDimension
    const op = v.slice(a + 1, b) as FilterOp
    if (!FILTER_DIMENSIONS.includes(dimension) || !OPS.includes(op)) continue
    out.push({ dimension, op, values: [v.slice(b + 1)] })
  }
  return out.slice(0, 10)
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
  metric: AnalyticsMetric
}

export function useView() {
  const [params, setParams] = useSearchParams()
  const metric = params.get("metric") as AnalyticsMetric | null
  const view: View = {
    range: parseRange(params.get("period")),
    compare: parseCompare(params.get("compare")),
    filters: parseFilters(params.getAll("f")),
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
    for (const f of filters) p.append("f", `${f.dimension}:${f.op}:${f.values[0] ?? ""}`)
  }
  return {
    view,
    setRange: (range: AnalyticsRange) =>
      update((p) => {
        const v = rangeParam(range)
        if (v === DEFAULT_PERIOD) p.delete("period")
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
    addFilter: (dimension: AnalyticsFilterDimension, value: string, op: FilterOp = "is") =>
      update((p) =>
        writeFilters(p, [...view.filters.filter((f) => f.dimension !== dimension), { dimension, op, values: [value] }])
      ),
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
    clearFilters: () => update((p) => p.delete("f")),
  }
}

export function useViewSearch() {
  const [params] = useSearchParams()
  const keep = new URLSearchParams()
  for (const k of ["period", "compare", "f"]) for (const v of params.getAll(k)) keep.append(k, v)
  const s = keep.toString()
  return s ? `?${s}` : ""
}

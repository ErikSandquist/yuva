import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { CircleSlashIcon, EyeOffIcon, GaugeIcon, LayersIcon, SigmaIcon } from "lucide-react"
import { useState } from "react"
import { Area, AreaChart, Bar, BarChart, CartesianGrid, ReferenceLine, XAxis, YAxis } from "recharts"

import { Notice, toast } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart"
import { Skeleton } from "@/components/ui/skeleton"
import { ComparePicker, FilterBar, RangePicker } from "@/features/analytics/Controls"
import { Change } from "@/features/analytics/Overview"
import { Panel, PanelProblem, QueryProblem, useCurrentProperty } from "@/features/analytics/parts"
import { useAnnotations, useApiQuery, useApiRoutes, useUpdateRoute } from "@/features/analytics/queries"
import { apiGrainFor, EXTRA_DIMENSIONS, isMinuteRange, useView } from "@/features/analytics/state"
import { formatMetric, num, useAnalyticsText, valueText, type NumMetric } from "@/features/analytics/text"
import type {
  AnalyticsAnnotation,
  AnalyticsDimension,
  AnalyticsFilter,
  AnalyticsFilterDimension,
  AnalyticsRange,
  AnalyticsResult,
  AnalyticsRoute,
  AnalyticsUsage,
} from "@/lib/api"
import { cn } from "@/lib/utils"

type Cap = NonNullable<AnalyticsResult["meta"]["caps_reached"]>[number]

export function useApiView() {
  return useView("api")
}

export const ALL_METHODS = "all"

export function routeFilters(route?: string, method?: string): AnalyticsFilter[] {
  const out: AnalyticsFilter[] = []
  if (route !== undefined) out.push({ dimension: "route", op: "is", values: [route] })
  if (method && method !== ALL_METHODS) out.push({ dimension: "method", op: "is", values: [method] })
  return out
}

export function routePath(propertyId: string, method: string, route: string) {
  const tail = route.startsWith("/") ? route.slice(1) : route
  return `/analytics/${propertyId}/routes/${encodeURIComponent(method)}/${tail.split("/").map(encodeURIComponent).join("/")}`
}

const WALL = 16

export function wallTime(v: string) {
  return new Date(`${v.length > 10 ? v.slice(0, WALL) : `${v}T00:00`}:00Z`)
}

export function repeatedWalls(labels: string[]) {
  const seen = new Set<string>()
  const twice = new Set<string>()
  for (const v of labels) {
    const w = v.slice(0, WALL)
    if (seen.has(w)) twice.add(w)
    seen.add(w)
  }
  return twice
}

export function bucketText(fmt: Intl.DateTimeFormat, v: string, repeated: Set<string>) {
  const text = fmt.format(wallTime(v))
  const offset = v.slice(WALL)
  return offset && repeated.has(v.slice(0, WALL)) ? `${text} (UTC${offset})` : text
}

function localDate(at: number, zone: string) {
  try {
    return new Intl.DateTimeFormat("en-CA", {
      timeZone: zone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(at)
  } catch {
    return new Date(at).toISOString().slice(0, 10)
  }
}

export function deployBucket(labels: string[], at: string, zone: string) {
  if (labels.length === 0) return undefined
  const when = Date.parse(at)
  let x: string | undefined
  if (labels[0]!.length > 10) {
    for (const l of labels) if (Date.parse(l) <= when) x = l
  } else {
    const day = localDate(when, zone)
    for (const l of labels) if (l <= day) x = l
  }
  return x
}

export type Blocker = "two_extra" | "core" | "minutes"

export function blockerOf(
  range: AnalyticsRange,
  filters: AnalyticsFilter[],
  dims: AnalyticsDimension[]
): Blocker | null {
  const all = [...filters.map((f) => f.dimension as string), ...dims.filter((d) => !d.startsWith("time:"))]
  const extras = new Set(all.filter((d) => EXTRA_DIMENSIONS.includes(d as AnalyticsFilterDimension)))
  if (extras.size === 0) return null
  if (extras.size > 1) return "two_extra"
  if (
    all.some(
      (d) =>
        !EXTRA_DIMENSIONS.includes(d as AnalyticsFilterDimension) && !["route", "method", "status_class"].includes(d)
    )
  )
    return "core"
  if (isMinuteRange(range) || dims.includes("time:minute")) return "minutes"
  return null
}

export function Blocked({ reason, className }: { reason: Blocker; className?: string }) {
  const { view, setRange, setFilters } = useApiView()
  return (
    <Notice icon={CircleSlashIcon} className={className} data-testid={`api-blocked-${reason}`}>
      <p>
        {reason === "two_extra" ? (
          <Trans>
            Consumers, countries and app versions are counted on their own, so only one of them can be used at a time.
            Remove a filter on one of them.
          </Trans>
        ) : reason === "core" ? (
          <Trans>
            Consumers, countries and app versions are counted with the route, method and status class only. Remove the
            other filters to see this.
          </Trans>
        ) : (
          <Trans>
            Consumers, countries and app versions are counted per hour, so they need a range of whole days, not the last
            minutes or hours.
          </Trans>
        )}
      </p>
      {reason === "minutes" ? (
        <Button size="sm" variant="outline" className="w-fit" onClick={() => setRange({ period: "today" })}>
          <Trans>Show today</Trans>
        </Button>
      ) : reason === "core" ? (
        <Button
          size="sm"
          variant="outline"
          className="w-fit"
          onClick={() =>
            setFilters(
              view.filters.filter((f) =>
                ["route", "method", "status_class", ...EXTRA_DIMENSIONS].includes(f.dimension as string)
              )
            )
          }
        >
          <Trans>Remove the other filters</Trans>
        </Button>
      ) : null}
    </Notice>
  )
}

export function RequestsBanner({ usage }: { usage: AnalyticsUsage }) {
  const { i18n } = useLingui()
  const used = usage.requests ?? 0
  const cap = usage.requests_limit
  if (!cap) return null
  const fmt = new Intl.NumberFormat(i18n.locale)
  const limit = fmt.format(cap)
  if (usage.requests_limit_reached_at || used >= cap) {
    return (
      <Notice tone="danger" icon={GaugeIcon} data-testid="api-requests-cap-reached">
        <Trans>
          This workspace reached its {limit} API requests for this month. Further requests are not counted until the
          month ends.
        </Trans>
      </Notice>
    )
  }
  if (used < cap * 0.8) return null
  const count = fmt.format(used)
  return (
    <Notice tone="warning" icon={GaugeIcon} data-testid="api-requests-cap-near">
      <Trans>
        {count} of {limit} API requests counted this month. Requests over the limit are not counted until the month
        ends.
      </Trans>
    </Notice>
  )
}

function CapLine({ cap }: { cap: Cap }) {
  const { i18n } = useLingui()
  const n = (v: number) => new Intl.NumberFormat(i18n.locale).format(v)
  switch (cap) {
    case "routes": {
      const max = n(1000)
      return <Trans>This API reached {max} routes. New routes are counted under Other.</Trans>
    }
    case "consumers": {
      const max = n(10000)
      return <Trans>This API reached {max} consumers. New consumers are counted under Other.</Trans>
    }
    case "api_versions": {
      const max = n(50)
      return <Trans>This API reached {max} API versions. New versions are counted under Other.</Trans>
    }
    case "app_versions": {
      const max = n(200)
      return <Trans>This API reached {max} app versions. New versions are counted under Other.</Trans>
    }
    case "environments": {
      const max = n(20)
      return <Trans>This API reached {max} environments. New environments are counted under Other.</Trans>
    }
    case "services": {
      const max = n(100)
      return <Trans>The workspace reached {max} services. New services are counted under Other.</Trans>
    }
    case "dimension_rows": {
      const max = n(50000)
      return (
        <Trans>
          An hour of this range reached {max} rows of consumers, countries or app versions. New ones in that hour are
          counted under Other.
        </Trans>
      )
    }
  }
}

function CapsNotice({ caps }: { caps?: Cap[] }) {
  if (!caps?.length) return null
  return (
    <Notice tone="warning" icon={LayersIcon} data-testid="api-caps-reached">
      <ul className="flex flex-col gap-1">
        {caps.map((c) => (
          <li key={c}>
            <CapLine cap={c} />
          </li>
        ))}
      </ul>
    </Notice>
  )
}

function Estimated({ show }: { show?: boolean }) {
  const { t } = useLingui()
  if (!show) return null
  return (
    <span
      className="inline-flex shrink-0 items-center gap-1 rounded-full bg-muted px-2 py-px text-caption font-normal text-muted-foreground"
      title={t`Your server sent one record for several successful requests, so counts are estimates.`}
      data-testid="api-estimated"
    >
      <SigmaIcon className="size-3 shrink-0" />
      <Trans>Estimated</Trans>
    </span>
  )
}

export function ApiControls({ meta, compare = true }: { meta?: AnalyticsResult["meta"]; compare?: boolean }) {
  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex flex-wrap items-center gap-2">
        <RangePicker kind="api" />
        {compare && <ComparePicker kind="api" />}
        <Estimated show={meta?.estimated} />
      </div>
      <FilterBar kind="api" />
      <CapsNotice caps={meta?.caps_reached} />
    </div>
  )
}

const KPIS: NumMetric[] = [
  "requests",
  "requests_per_minute",
  "error_rate",
  "client_error_rate",
  "p50",
  "p95",
  "p99",
  "apdex",
]

export function Kpis({ base, testId }: { base: AnalyticsFilter[]; testId: string }) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const { view } = useApiView()
  const filters = [...base, ...view.filters]
  const blocked = blockerOf(view.range, filters, [])
  const q = useApiQuery(property.id, { metrics: KPIS, range: view.range, filters, compare: view.compare }, !blocked)
  const totals = q.data?.totals
  const before = q.data?.comparison?.totals
  return (
    <>
      <ApiControls meta={q.data?.meta} />
      {blocked ? (
        <Blocked reason={blocked} />
      ) : (
        <>
          <QueryProblem error={q.error} />
          <div className="grid grid-cols-4 gap-2 phone:grid-cols-2" role="group" data-testid={testId}>
            {KPIS.map((m) => (
              <div
                key={m}
                title={text.metricHelp[m]}
                className="flex min-w-0 flex-col items-start rounded-2xl border bg-card px-4 py-3 phone:px-3"
                data-testid={`kpi-${m}`}
              >
                <span className="max-w-full truncate text-small text-faint">{text.metric[m]}</span>
                {q.isPending ? (
                  <Skeleton className="mt-1 h-7 w-16" />
                ) : (
                  <span className="flex flex-wrap items-baseline gap-x-2">
                    <b className="text-page tabular-nums">{formatMetric(m, num(totals, m), i18n.locale)}</b>
                    {view.compare && <Change now={num(totals, m)} before={num(before, m)} metric={m} />}
                  </span>
                )}
                {view.compare && !q.isPending && (
                  <span className="max-w-full truncate text-caption text-faint tabular-nums">
                    {formatMetric(m, num(before, m), i18n.locale)}
                  </span>
                )}
              </div>
            ))}
          </div>
        </>
      )}
    </>
  )
}

const SERIES_COLOR: Partial<Record<NumMetric, string>> = {
  requests: "var(--brand)",
  errors: "var(--destructive)",
  error_rate: "var(--destructive)",
  client_error_rate: "var(--warning)",
  conversations: "var(--brand)",
  p50: "var(--muted-foreground)",
  p95: "var(--brand)",
  p99: "var(--warning)",
}

function grainText(grain: AnalyticsDimension) {
  if (grain === "time:minute") return <Trans>per minute</Trans>
  if (grain === "time:hour") return <Trans>per hour</Trans>
  if (grain === "time:week") return <Trans>per week</Trans>
  return <Trans>per day</Trans>
}

function useDeploys(propertyId: string, meta?: AnalyticsResult["meta"]) {
  const notes = useAnnotations(propertyId, meta?.from, meta?.to)
  return (notes.data ?? []).filter((a): a is AnalyticsAnnotation & { at: string } => a.kind === "deploy" && !!a.at)
}

export function SeriesChart({
  title,
  metrics,
  base,
  testId,
  noFilters,
  grain: fixedGrain,
}: {
  title: React.ReactNode
  metrics: NumMetric[]
  base: AnalyticsFilter[]
  testId: string
  noFilters?: boolean
  grain?: AnalyticsDimension
}) {
  const { t, i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const { view } = useApiView()
  const grain = fixedGrain ?? apiGrainFor(view.range)
  const filters = noFilters ? [] : [...base, ...view.filters]
  const blocked = blockerOf(view.range, filters, [grain])
  const q = useApiQuery(
    property.id,
    { metrics, dimensions: [grain], range: view.range, filters, compare: view.compare },
    !blocked
  )
  const deploys = useDeploys(property.id, q.data?.meta)
  const short = grain === "time:minute" || grain === "time:hour"
  const tick = new Intl.DateTimeFormat(
    i18n.locale,
    short ? { timeStyle: "short", timeZone: "UTC" } : { day: "numeric", month: "short", timeZone: "UTC" }
  )
  const long = new Intl.DateTimeFormat(i18n.locale, {
    dateStyle: "medium",
    ...(short ? { timeStyle: "short" } : {}),
    timeZone: "UTC",
  })
  const first = metrics[0]!
  const before = q.data?.comparison?.rows ?? []
  const data = (q.data?.rows ?? []).map((r, i) => {
    const row: Record<string, string | number | undefined> = { at: r.dimensions[0] ?? "" }
    for (const m of metrics) row[m] = num(r.metrics, m) ?? 0
    if (view.compare) row.previous = num(before[i]?.metrics, first) ?? 0
    return row
  })
  const labels = data.map((d) => String(d.at))
  const repeated = repeatedWalls(labels)
  const top = Math.max(0, ...data.flatMap((d) => [...metrics, "previous"].map((m) => Number(d[m]) || 0)))
  const axisWidth = Math.max(
    40,
    ...[top, top * 1.25, top / 2].map((v) => formatMetric(first, v, i18n.locale).length * 7 + 12)
  )
  const config: ChartConfig = {
    ...Object.fromEntries(metrics.map((m) => [m, { label: text.metric[m], color: SERIES_COLOR[m] ?? "var(--brand)" }])),
    ...(view.compare ? { previous: { label: t`Compared range`, color: "var(--muted-foreground)" } } : {}),
  }
  const marks = deploys.flatMap((a) => {
    const x = deployBucket(labels, a.at, property.timezone)
    return x ? [{ a, x }] : []
  })
  return (
    <Panel
      title={
        <>
          {title}
          <span className="ms-2 text-caption font-normal text-faint">{grainText(grain)}</span>
        </>
      }
      action={
        (metrics.length > 1 || view.compare) && (
          <span className="flex flex-wrap items-center gap-3 text-caption text-muted-foreground">
            {metrics.map((m) => (
              <span key={m} className="inline-flex items-center gap-1.5">
                <span className="size-2.5 rounded-[2px]" style={{ background: SERIES_COLOR[m] }} />
                {text.metric[m]}
              </span>
            ))}
            {view.compare && (
              <span className="inline-flex items-center gap-1.5">
                <span className="size-2.5 rounded-[2px] border border-dashed border-muted-foreground" />
                {t`Compared range`}
              </span>
            )}
          </span>
        )
      }
      testId={testId}
    >
      {blocked ? (
        <Blocked reason={blocked} />
      ) : q.isPending ? (
        <Skeleton className="h-48 w-full" />
      ) : q.error ? (
        <PanelProblem error={q.error} />
      ) : (
        <>
          <ChartContainer config={config} className="aspect-auto h-48 w-full">
            <AreaChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: -4 }}>
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey="at"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                minTickGap={28}
                interval="preserveStartEnd"
                tickFormatter={(v) => tick.format(wallTime(String(v)))}
              />
              <YAxis
                tickLine={false}
                axisLine={false}
                width={axisWidth}
                tickFormatter={(v) => formatMetric(first, Number(v), i18n.locale)}
              />
              <ChartTooltip
                cursor={false}
                content={
                  <ChartTooltipContent
                    labelFormatter={(_, p) => (p?.[0] ? bucketText(long, String(p[0].payload.at), repeated) : "")}
                    formatter={(value, name) => (
                      <div className="flex w-full items-center gap-2">
                        <span
                          className={cn(
                            "size-2.5 shrink-0 rounded-[2px]",
                            name === "previous" && "border border-dashed border-muted-foreground"
                          )}
                          style={name === "previous" ? undefined : { background: SERIES_COLOR[name as NumMetric] }}
                        />
                        <span className="flex-1 text-muted-foreground">{config[String(name)]?.label}</span>
                        <span className="font-mono font-medium text-foreground tabular-nums">
                          {formatMetric(name === "previous" ? first : (name as NumMetric), Number(value), i18n.locale)}
                        </span>
                      </div>
                    )}
                  />
                }
              />
              {view.compare && (
                <Area
                  dataKey="previous"
                  type="monotone"
                  stroke="var(--color-previous)"
                  strokeDasharray="4 4"
                  strokeWidth={1.5}
                  fill="none"
                  isAnimationActive={false}
                />
              )}
              {metrics.map((m, i) => (
                <Area
                  key={m}
                  dataKey={m}
                  type="monotone"
                  stroke={`var(--color-${m})`}
                  strokeWidth={i === 0 || m === "p95" ? 2 : 1.5}
                  fill={i === 0 ? `var(--color-${m})` : "none"}
                  fillOpacity={0.12}
                  isAnimationActive={false}
                />
              ))}
              {marks.map(({ a, x }) => (
                <ReferenceLine
                  key={a.id}
                  x={x}
                  stroke="var(--muted-foreground)"
                  strokeDasharray="3 3"
                  strokeOpacity={0.8}
                  ifOverflow="extendDomain"
                />
              ))}
            </AreaChart>
          </ChartContainer>
          {marks.length > 0 && <DeployList items={marks.map((m) => m.a)} />}
        </>
      )}
    </Panel>
  )
}

function DeployList({ items }: { items: (AnalyticsAnnotation & { at: string })[] }) {
  const { i18n } = useLingui()
  const fmt = new Intl.DateTimeFormat(i18n.locale, { dateStyle: "medium", timeStyle: "short" })
  return (
    <ul className="mt-2 flex flex-wrap gap-x-4 gap-y-1 px-1 text-caption text-faint" data-testid="api-deploys">
      {items.slice(-6).map((a) => {
        const release = a.release ?? a.text
        const where = [a.service, a.environment].filter(Boolean).join(" · ")
        const at = fmt.format(new Date(a.at))
        return (
          <li key={a.id} className="inline-flex min-w-0 items-center gap-1.5">
            <span className="h-3 border-s border-dashed border-muted-foreground" aria-hidden />
            <span className="truncate">
              {where ? (
                <Trans>
                  {release} deployed to {where}, {at}
                </Trans>
              ) : (
                <Trans>
                  {release} deployed, {at}
                </Trans>
              )}
            </span>
          </li>
        )
      })}
    </ul>
  )
}

export function LastHour({ base }: { base: AnalyticsFilter[] }) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const { view } = useApiView()
  const filters = [...base, ...view.filters]
  const blocked = blockerOf({ period: "1h" }, filters, ["time:minute"])
  const q = useApiQuery(
    property.id,
    { metrics: ["requests", "errors"], dimensions: ["time:minute"], range: { period: "1h" }, filters },
    !blocked
  )
  const time = new Intl.DateTimeFormat(i18n.locale, { timeStyle: "short", timeZone: "UTC" })
  const data = (q.data?.rows ?? []).map((r) => {
    const requests = num(r.metrics, "requests") ?? 0
    const errors = num(r.metrics, "errors") ?? 0
    return { at: r.dimensions[0] ?? "", ok: Math.max(0, requests - errors), errors, requests }
  })
  const repeated = repeatedWalls(data.map((d) => d.at))
  const config: ChartConfig = {
    ok: { label: text.metric.requests, color: "var(--brand)" },
    errors: { label: text.metric.errors, color: "var(--destructive)" },
  }
  const fmt = (v: number) => formatMetric("requests", v, i18n.locale)
  const requests = num(q.data?.totals, "requests") ?? 0
  const errors = num(q.data?.totals, "errors") ?? 0
  return (
    <Panel
      title={<Trans>The last 60 minutes</Trans>}
      action={
        q.data &&
        !blocked && (
          <span className="text-caption text-faint tabular-nums">
            <Plural value={requests} one="# request" other="# requests" />
            {" · "}
            <Plural value={errors} one="# error" other="# errors" />
          </span>
        )
      }
      testId="api-last-hour"
    >
      {blocked ? (
        <p className="px-2 py-6 text-center text-body text-faint">
          <Trans>Minutes are not kept per consumer, country or app version. Remove that filter to see them.</Trans>
        </p>
      ) : q.isPending ? (
        <Skeleton className="h-36 w-full" />
      ) : q.error ? (
        <PanelProblem error={q.error} />
      ) : (
        <ChartContainer config={config} className="aspect-auto h-36 w-full">
          <BarChart data={data} margin={{ top: 4, right: 4, bottom: 0, left: -12 }} barCategoryGap={1}>
            <CartesianGrid vertical={false} />
            <XAxis
              dataKey="at"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              minTickGap={32}
              tickFormatter={(v) => time.format(wallTime(String(v)))}
            />
            <YAxis allowDecimals={false} tickLine={false} axisLine={false} width={48} tickFormatter={(v) => fmt(v)} />
            <ChartTooltip
              cursor={false}
              content={
                <ChartTooltipContent
                  labelFormatter={(_, p) => (p?.[0] ? bucketText(time, String(p[0].payload.at), repeated) : "")}
                />
              }
            />
            <Bar dataKey="ok" stackId="a" fill="var(--color-ok)" isAnimationActive={false} />
            <Bar
              dataKey="errors"
              stackId="a"
              fill="var(--color-errors)"
              radius={[2, 2, 0, 0]}
              isAnimationActive={false}
            />
          </BarChart>
        </ChartContainer>
      )}
    </Panel>
  )
}

export function Charts({ base }: { base: AnalyticsFilter[] }) {
  const { t } = useLingui()
  return (
    <>
      <SeriesChart title={t`Requests`} metrics={["requests", "errors"]} base={base} testId="api-chart-requests" />
      <div className="grid gap-4 md:grid-cols-2">
        <SeriesChart
          title={t`Error rates`}
          metrics={["error_rate", "client_error_rate"]}
          base={base}
          testId="api-chart-errors"
        />
        <SeriesChart title={t`Latency`} metrics={["p95", "p50", "p99"]} base={base} testId="api-chart-latency" />
      </div>
    </>
  )
}

export function ConversationsChart() {
  const { t } = useLingui()
  const property = useCurrentProperty()
  const { view } = useApiView()
  if (!property.inbox_id) return null
  const grain = apiGrainFor(view.range)
  const daily = grain === "time:day" || grain === "time:week"
  if (!daily || view.filters.length > 0) {
    return (
      <Panel title={t`Errors and conversations`} testId="api-chart-conversations">
        <p className="px-2 py-6 text-center text-body text-faint">
          {view.filters.length > 0 ? (
            <Trans>Conversations are counted for the whole API. Remove the filters to see them next to errors.</Trans>
          ) : (
            <Trans>
              Conversations are counted per day. Pick a range of at least 3 days to see them next to errors.
            </Trans>
          )}
        </p>
      </Panel>
    )
  }
  return (
    <SeriesChart
      title={t`Errors and conversations`}
      metrics={["errors", "conversations"]}
      base={[]}
      noFilters
      grain={grain}
      testId="api-chart-conversations"
    />
  )
}

const SIZES: NumMetric[] = ["response_bytes", "response_size_p50", "response_size_p95", "request_bytes"]

export function SizeStats({ base }: { base: AnalyticsFilter[] }) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const { view } = useApiView()
  const filters = [...base, ...view.filters]
  const blocked = blockerOf(view.range, filters, [])
  const q = useApiQuery(property.id, { metrics: SIZES, range: view.range, filters }, !blocked)
  if (blocked) return null
  return (
    <Panel title={<Trans>Sizes</Trans>} testId="api-sizes">
      {q.isPending ? (
        <Skeleton className="h-16 w-full" />
      ) : q.error ? (
        <PanelProblem error={q.error} />
      ) : (
        <dl className="grid grid-cols-2 gap-x-4 gap-y-3 px-1 sm:grid-cols-4">
          {SIZES.map((m) => (
            <div key={m} className="flex min-w-0 flex-col" title={text.metricHelp[m]}>
              <dt className="truncate text-small text-faint">{text.metric[m]}</dt>
              <dd className="text-title tabular-nums">{formatMetric(m, num(q.data?.totals, m), i18n.locale)}</dd>
            </div>
          ))}
        </dl>
      )}
    </Panel>
  )
}

function MethodTag({ method }: { method: string }) {
  const text = useAnalyticsText()
  const { i18n } = useLingui()
  return (
    <span className="inline-flex h-5 shrink-0 items-center rounded-md bg-muted px-1.5 font-mono text-caption font-medium text-muted-foreground">
      {method === "_OTHER" ? valueText(text, "method", method, i18n.locale) : method}
    </span>
  )
}

export function InferredTag() {
  const { t } = useLingui()
  return (
    <span
      className="inline-flex shrink-0 items-center rounded-full bg-muted px-1.5 text-caption text-muted-foreground"
      title={t`Made from request paths, with ids taken out. Your router did not name this pattern.`}
      data-testid="api-route-inferred"
    >
      <Trans>inferred</Trans>
    </span>
  )
}

export function RouteName({
  method,
  route,
  inferred,
  wrap,
}: {
  method?: string
  route: string
  inferred?: boolean
  wrap?: boolean
}) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const plain = route.startsWith("/")
  return (
    <span className={cn("flex min-w-0 items-center gap-x-2", wrap && "flex-wrap gap-y-1")}>
      {method && method !== ALL_METHODS && <MethodTag method={method} />}
      <span
        className={cn("min-w-0", wrap ? "break-all" : "truncate", plain && "font-mono text-small")}
        dir={plain ? "ltr" : undefined}
      >
        {valueText(text, "route", route, i18n.locale)}
      </span>
      {inferred && <InferredTag />}
    </span>
  )
}

export function useRouteIndex(propertyId: string) {
  const routes = useApiRoutes(propertyId)
  return new Map((routes.data ?? []).map((r) => [r.pattern, r]))
}

export function useHideRoute(propertyId: string) {
  const { t, i18n } = useLingui()
  const text = useAnalyticsText()
  const set = useUpdateRoute(propertyId)
  return {
    error: set.error,
    pending: set.isPending,
    run: (r: AnalyticsRoute, hidden: boolean) =>
      set.mutate(
        { routeId: r.id, hidden },
        {
          onSuccess: () => {
            if (!hidden) return
            const name = valueText(text, "route", r.pattern, i18n.locale)
            toast(t`${name} is hidden from every view.`, () => set.mutate({ routeId: r.id, hidden: false }))
          },
        }
      ),
  }
}

export function HideRouteButton({ route, propertyId }: { route?: AnalyticsRoute; propertyId: string }) {
  const { t } = useLingui()
  const hide = useHideRoute(propertyId)
  if (!route) return null
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-sm"
      disabled={hide.pending}
      onClick={() => hide.run(route, true)}
      aria-label={t`Hide route`}
      title={t`Hide this route, with all its methods, from every view`}
      data-testid="api-hide-route"
    >
      <EyeOffIcon />
    </Button>
  )
}

export function ApdexThreshold({ route, canManage }: { route: AnalyticsRoute; canManage: boolean }) {
  const { t, i18n } = useLingui()
  const property = useCurrentProperty()
  const update = useUpdateRoute(property.id)
  const [value, setValue] = useState(String(route.apdex_ms ?? ""))
  const ms = (v: number) => formatMetric("p50", v, i18n.locale)
  const current = route.apdex_ms
  const n = Number(value)
  const valid = Number.isInteger(n) && n >= 1 && n <= 60_000
  const own = current !== undefined ? ms(current) : ""
  const fallback = ms(property.apdex_ms)
  return (
    <Panel title={<Trans>Apdex threshold</Trans>} testId="api-route-apdex">
      <div className="flex flex-col gap-3 px-1">
        <p className="text-body text-muted-foreground">
          {current !== undefined ? (
            <Trans>
              This route counts a request as satisfied within {own}, and as tolerated within four times that.
            </Trans>
          ) : (
            <Trans>This route uses the property’s threshold of {fallback}.</Trans>
          )}
        </p>
        {canManage && (
          <form
            className="flex flex-wrap items-center gap-2"
            onSubmit={(e) => {
              e.preventDefault()
              if (valid) update.mutate({ routeId: route.id, apdex_ms: n })
            }}
          >
            <Input
              type="number"
              inputMode="numeric"
              min={1}
              max={60000}
              value={value}
              placeholder={String(property.apdex_ms)}
              onChange={(e) => setValue(e.target.value)}
              aria-label={t`Threshold in milliseconds`}
              className="w-28 tabular-nums"
              data-testid="api-route-apdex-input"
            />
            <span className="text-small text-faint">
              <Trans>ms</Trans>
            </span>
            <Button type="submit" size="sm" disabled={!valid || update.isPending || n === current}>
              <Trans>Save</Trans>
            </Button>
            {current !== undefined && (
              <Button
                type="button"
                size="sm"
                variant="ghost"
                disabled={update.isPending}
                onClick={() => update.mutate({ routeId: route.id, apdex_ms: null }, { onSuccess: () => setValue("") })}
                data-testid="api-route-apdex-reset"
              >
                <Trans>Use the property’s</Trans>
              </Button>
            )}
          </form>
        )}
        {update.error ? <PanelProblem error={update.error} /> : null}
      </div>
    </Panel>
  )
}

import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { ArrowDownIcon, ArrowLeftIcon, GaugeIcon, LayersIcon, SigmaIcon } from "lucide-react"
import { Link, useParams, useSearchParams } from "react-router"
import { Area, AreaChart, Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts"

import { Notice, Segmented } from "@/components/common"
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { RangePicker } from "@/features/analytics/Controls"
import { ApiInstallSnippets } from "@/features/analytics/Install"
import { BarList, CsvButton, Panel, PanelProblem, QueryProblem, useCurrentProperty } from "@/features/analytics/parts"
import { downloadCsv, useAnalyticsTable, useApiQuery } from "@/features/analytics/queries"
import { apiGrainFor, useView, useViewSearch } from "@/features/analytics/state"
import {
  formatCompact,
  formatMetric,
  num,
  OTHER,
  useAnalyticsText,
  valueText,
  type NumMetric,
} from "@/features/analytics/text"
import type { AnalyticsDimension, AnalyticsFilter, AnalyticsQuery, AnalyticsResult, AnalyticsUsage } from "@/lib/api"
import { cn } from "@/lib/utils"

type Cap = NonNullable<AnalyticsResult["meta"]["caps_reached"]>[number]

const KPIS: NumMetric[] = ["requests", "requests_per_minute", "error_rate", "client_error_rate", "p95", "p99"]

function useApiView() {
  return useView("api")
}

function routeFilters(route?: string, method?: string): AnalyticsFilter[] {
  const out: AnalyticsFilter[] = []
  if (route !== undefined) out.push({ dimension: "route", op: "is", values: [route] })
  if (method) out.push({ dimension: "method", op: "is", values: [method] })
  return out
}

export function routePath(propertyId: string, method: string, route: string) {
  const tail = route.startsWith("/") ? route.slice(1) : route
  return `/analytics/${propertyId}/routes/${encodeURIComponent(method)}/${tail.split("/").map(encodeURIComponent).join("/")}`
}

function bucketDate(v: string) {
  return new Date(v.length > 10 ? `${v}:00Z` : `${v}T00:00:00Z`)
}

export function RequestsBanner({ usage }: { usage: AnalyticsUsage }) {
  const { i18n } = useLingui()
  const used = usage.requests ?? 0
  const cap = usage.requests_limit
  if (!cap) return null
  const fmt = new Intl.NumberFormat(i18n.locale)
  const limit = fmt.format(cap)
  if (used >= cap) {
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

function ApiControls({ meta }: { meta?: AnalyticsResult["meta"] }) {
  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex flex-wrap items-center gap-2">
        <RangePicker kind="api" />
        <Estimated show={meta?.estimated} />
      </div>
      <CapsNotice caps={meta?.caps_reached} />
    </div>
  )
}

function Kpis({ filters, testId }: { filters: AnalyticsFilter[]; testId: string }) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const { view } = useApiView()
  const q = useApiQuery(property.id, { metrics: KPIS, range: view.range, filters })
  return (
    <>
      <ApiControls meta={q.data?.meta} />
      <QueryProblem error={q.error} />
      <div className="grid grid-cols-3 gap-2 phone:grid-cols-2" role="group" data-testid={testId}>
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
              <b className="text-page tabular-nums">{formatMetric(m, num(q.data?.totals, m), i18n.locale)}</b>
            )}
          </div>
        ))}
      </div>
    </>
  )
}

const SERIES_COLOR: Partial<Record<NumMetric, string>> = {
  requests: "var(--brand)",
  errors: "var(--destructive)",
  error_rate: "var(--destructive)",
  client_error_rate: "var(--warning)",
  p50: "var(--muted-foreground)",
  p95: "var(--brand)",
  p99: "var(--warning)",
}

function grainText(grain: AnalyticsDimension) {
  if (grain === "time:minute") return <Trans>per minute</Trans>
  if (grain === "time:hour") return <Trans>per hour</Trans>
  return <Trans>per day</Trans>
}

function SeriesChart({
  title,
  metrics,
  filters,
  testId,
}: {
  title: React.ReactNode
  metrics: NumMetric[]
  filters: AnalyticsFilter[]
  testId: string
}) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const { view } = useApiView()
  const grain = apiGrainFor(view.range)
  const q = useApiQuery(property.id, { metrics, dimensions: [grain], range: view.range, filters })
  const short = grain !== "time:day"
  const tick = new Intl.DateTimeFormat(
    i18n.locale,
    short ? { timeStyle: "short", timeZone: "UTC" } : { day: "numeric", month: "short", timeZone: "UTC" }
  )
  const long = new Intl.DateTimeFormat(i18n.locale, {
    dateStyle: "medium",
    ...(short ? { timeStyle: "short" } : {}),
    timeZone: "UTC",
  })
  const data = (q.data?.rows ?? []).map((r) => {
    const row: Record<string, string | number> = { at: r.dimensions[0] ?? "" }
    for (const m of metrics) row[m] = num(r.metrics, m) ?? 0
    return row
  })
  const config: ChartConfig = Object.fromEntries(
    metrics.map((m) => [m, { label: text.metric[m], color: SERIES_COLOR[m] ?? "var(--brand)" }])
  )
  return (
    <Panel
      title={
        <>
          {title}
          <span className="ms-2 text-caption font-normal text-faint">{grainText(grain)}</span>
        </>
      }
      action={
        metrics.length > 1 && (
          <span className="flex flex-wrap items-center gap-3 text-caption text-muted-foreground">
            {metrics.map((m) => (
              <span key={m} className="inline-flex items-center gap-1.5">
                <span className="size-2.5 rounded-[2px]" style={{ background: SERIES_COLOR[m] }} />
                {text.metric[m]}
              </span>
            ))}
          </span>
        )
      }
      testId={testId}
    >
      {q.isPending ? (
        <Skeleton className="h-48 w-full" />
      ) : q.error ? (
        <PanelProblem error={q.error} />
      ) : (
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
              tickFormatter={(v) => tick.format(bucketDate(String(v)))}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              width={56}
              tickFormatter={(v) => formatMetric(metrics[0]!, Number(v), i18n.locale)}
            />
            <ChartTooltip
              cursor={false}
              content={
                <ChartTooltipContent
                  labelFormatter={(_, p) => (p?.[0] ? long.format(bucketDate(String(p[0].payload.at))) : "")}
                  formatter={(value, name) => (
                    <div className="flex w-full items-center gap-2">
                      <span
                        className="size-2.5 shrink-0 rounded-[2px]"
                        style={{ background: SERIES_COLOR[name as NumMetric] }}
                      />
                      <span className="flex-1 text-muted-foreground">{config[String(name)]?.label}</span>
                      <span className="font-mono font-medium text-foreground tabular-nums">
                        {formatMetric(name as NumMetric, Number(value), i18n.locale)}
                      </span>
                    </div>
                  )}
                />
              }
            />
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
          </AreaChart>
        </ChartContainer>
      )}
    </Panel>
  )
}

function LastHour({ filters }: { filters: AnalyticsFilter[] }) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const q = useApiQuery(property.id, {
    metrics: ["requests", "errors"],
    dimensions: ["time:minute"],
    range: { period: "1h" },
    filters,
  })
  const time = new Intl.DateTimeFormat(i18n.locale, { timeStyle: "short", timeZone: "UTC" })
  const data = (q.data?.rows ?? []).map((r) => {
    const requests = num(r.metrics, "requests") ?? 0
    const errors = num(r.metrics, "errors") ?? 0
    return { at: r.dimensions[0] ?? "", ok: Math.max(0, requests - errors), errors, requests }
  })
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
        q.data && (
          <span className="text-caption text-faint tabular-nums">
            <Plural value={requests} one="# request" other="# requests" />
            {" · "}
            <Plural value={errors} one="# error" other="# errors" />
          </span>
        )
      }
      testId="api-last-hour"
    >
      {q.isPending ? (
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
              tickFormatter={(v) => time.format(bucketDate(String(v)))}
            />
            <YAxis allowDecimals={false} tickLine={false} axisLine={false} width={48} tickFormatter={(v) => fmt(v)} />
            <ChartTooltip
              cursor={false}
              content={
                <ChartTooltipContent
                  labelFormatter={(_, p) => (p?.[0] ? time.format(bucketDate(String(p[0].payload.at))) : "")}
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

function MethodTag({ method }: { method: string }) {
  const text = useAnalyticsText()
  const { i18n } = useLingui()
  return (
    <span className="inline-flex h-5 shrink-0 items-center rounded-md bg-muted px-1.5 font-mono text-caption font-medium text-muted-foreground">
      {method === "_OTHER" ? valueText(text, "method", method, i18n.locale) : method}
    </span>
  )
}

function RouteName({ method, route }: { method?: string; route: string }) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const plain = route.startsWith("/")
  return (
    <span className="flex min-w-0 items-center gap-2">
      {method && <MethodTag method={method} />}
      <span className={cn("min-w-0 truncate", plain && "font-mono text-small")} dir={plain ? "ltr" : undefined}>
        {valueText(text, "route", route, i18n.locale)}
      </span>
    </span>
  )
}

function TopRoutes() {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const search = useViewSearch()
  const { view } = useApiView()
  const q = useApiQuery(property.id, {
    metrics: ["requests"],
    dimensions: ["route", "method"],
    range: view.range,
    limit: 8,
  })
  const rows = q.data?.rows ?? []
  return (
    <Panel
      title={<Trans>Busiest routes</Trans>}
      testId="api-top-routes"
      footer={
        <Link
          to={`/analytics/${property.id}/routes${search}`}
          className="text-small text-brand underline-offset-4 hover:underline"
        >
          <Trans>All routes</Trans>
        </Link>
      }
    >
      {q.isPending ? (
        <Skeleton className="h-40 w-full" />
      ) : q.error ? (
        <PanelProblem error={q.error} />
      ) : rows.length === 0 ? (
        <p className="px-2 py-6 text-center text-body text-faint">
          <Trans>No requests in this range.</Trans>
        </p>
      ) : (
        <BarList
          head={[text.dimension.route, text.metric.requests]}
          items={rows.map((r, i) => {
            const [route = "", method = ""] = r.dimensions
            const v = num(r.metrics, "requests") ?? 0
            return {
              key: `${method} ${route} ${i}`,
              label: <RouteName method={method} route={route} />,
              value: v,
              display: formatCompact(v, i18n.locale),
              to: route === OTHER ? undefined : `${routePath(property.id, method, route)}${search}`,
            }
          })}
        />
      )}
    </Panel>
  )
}

function useHasRequests(propertyId: string) {
  const q = useApiQuery(propertyId, { metrics: ["requests"], range: { period: "12mo" } })
  return { pending: q.isPending, error: q.error, any: (num(q.data?.totals, "requests") ?? 0) > 0 }
}

function ApiEmpty() {
  const property = useCurrentProperty()
  return (
    <section className="flex flex-col gap-4" data-testid="api-empty">
      <Notice>
        <p className="font-medium text-foreground">
          <Trans>No requests yet</Trans>
        </p>
        <p>
          <Trans>
            Add the middleware to your server or send records with any HTTP client. Requests show up here about 15
            seconds after your server answered them.
          </Trans>
        </p>
      </Notice>
      <div className="flex flex-col gap-5 rounded-2xl border bg-card p-4">
        <ApiInstallSnippets property={property} />
      </div>
    </section>
  )
}

function WhenRequests({ children }: { children: React.ReactNode }) {
  const property = useCurrentProperty()
  const has = useHasRequests(property.id)
  if (has.pending) return <Skeleton className="h-64 rounded-2xl" />
  if (has.error) return <QueryProblem error={has.error} />
  if (!has.any) return <ApiEmpty />
  return <>{children}</>
}

function Charts({ filters }: { filters: AnalyticsFilter[] }) {
  const { t } = useLingui()
  return (
    <>
      <SeriesChart title={t`Requests`} metrics={["requests", "errors"]} filters={filters} testId="api-chart-requests" />
      <div className="grid gap-4 md:grid-cols-2">
        <SeriesChart
          title={t`Error rates`}
          metrics={["error_rate", "client_error_rate"]}
          filters={filters}
          testId="api-chart-errors"
        />
        <SeriesChart title={t`Latency`} metrics={["p95", "p50", "p99"]} filters={filters} testId="api-chart-latency" />
      </div>
    </>
  )
}

export function ApiOverview() {
  return (
    <WhenRequests>
      <div className="flex flex-col gap-4" data-testid="api-overview">
        <Kpis filters={[]} testId="api-kpis" />
        <LastHour filters={[]} />
        <Charts filters={[]} />
        <TopRoutes />
      </div>
    </WhenRequests>
  )
}

type Sort = "traffic" | "errors" | "latency"

const SORTS: Record<Sort, { metric: NumMetric; order: AnalyticsQuery["order_by"] }> = {
  traffic: { metric: "requests", order: [{ metric: "requests", dir: "desc" }] },
  errors: {
    metric: "error_rate",
    order: [
      { metric: "error_rate", dir: "desc" },
      { metric: "errors", dir: "desc" },
    ],
  },
  latency: { metric: "p95", order: [{ metric: "p95", dir: "desc" }] },
}

const TABLE: NumMetric[] = ["requests", "error_rate", "client_error_rate", "p50", "p95", "p99"]

export function ApiRoutes() {
  const { t, i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const search = useViewSearch()
  const { view } = useApiView()
  const [params, setParams] = useSearchParams()
  const raw = params.get("sort") as Sort | null
  const sort: Sort = raw && raw in SORTS ? raw : "traffic"
  const body: AnalyticsQuery = {
    dataset: "api",
    metrics: TABLE,
    dimensions: ["route", "method"],
    range: view.range,
    order_by: SORTS[sort].order,
    limit: 100,
  }
  const table = useAnalyticsTable(property.id, body)
  const rows = table.data?.pages.flatMap((p) => p.rows) ?? []
  const meta = table.data?.pages[0]?.meta
  const phoneMetric = SORTS[sort].metric
  const names: Record<Sort, string> = { traffic: t`Traffic`, errors: t`Errors`, latency: t`Latency` }
  return (
    <WhenRequests>
      <section className="flex flex-col gap-3" data-testid="api-routes">
        <ApiControls meta={meta} />
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="-mx-1 max-w-[calc(100%+0.5rem)] overflow-x-auto px-1 py-1">
            <Segmented
              value={sort}
              onChange={(v) =>
                setParams(
                  (prev) => {
                    const next = new URLSearchParams(prev)
                    if (v === "traffic") next.delete("sort")
                    else next.set("sort", v)
                    return next
                  },
                  { replace: true }
                )
              }
              label={t`Sort routes by`}
              items={(Object.keys(SORTS) as Sort[]).map((s) => ({
                value: s,
                label: names[s],
                testId: `api-sort-${s}`,
              }))}
            />
          </div>
          <CsvButton testId="analytics-csv" run={() => downloadCsv(property.id, body, `${property.name}-routes`)} />
        </div>
        <QueryProblem error={table.error} />
        <div className="overflow-hidden rounded-2xl border bg-card">
          <table className="w-full table-fixed text-body" data-testid="api-routes-table">
            <thead>
              <tr className="border-b text-caption text-faint">
                <th className="px-4 py-2.5 text-start font-medium">{text.dimension.route}</th>
                {TABLE.map((m) => (
                  <th
                    key={m}
                    className={cn(
                      "w-24 px-2 py-2.5 text-end font-medium",
                      m !== phoneMetric && "phone:hidden",
                      m === SORTS[sort].metric && "text-foreground"
                    )}
                    title={text.metricHelp[m]}
                    aria-sort={m === SORTS[sort].metric ? "descending" : undefined}
                  >
                    <span className="inline-flex items-center gap-1">
                      {text.metric[m]}
                      {m === SORTS[sort].metric && <ArrowDownIcon className="size-3" />}
                    </span>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y">
              {table.isPending ? (
                <tr>
                  <td colSpan={TABLE.length + 1} className="p-4">
                    <Skeleton className="h-24 w-full" />
                  </td>
                </tr>
              ) : rows.length === 0 ? (
                <tr>
                  <td colSpan={TABLE.length + 1} className="px-4 py-6 text-faint">
                    <Trans>No requests in this range.</Trans>
                  </td>
                </tr>
              ) : (
                rows.map((r, i) => {
                  const [route = "", method = ""] = r.dimensions
                  return (
                    <tr key={`${method} ${route} ${i}`} className="hover:bg-muted/50" data-testid="api-route-row">
                      <td className="px-4 py-2">
                        {route === OTHER ? (
                          <RouteName method={method} route={route} />
                        ) : (
                          <Link
                            to={`${routePath(property.id, method, route)}${search}`}
                            className="block max-w-full rounded outline-none hover:text-brand focus-visible:ring-2 focus-visible:ring-ring"
                          >
                            <RouteName method={method} route={route} />
                          </Link>
                        )}
                      </td>
                      {TABLE.map((m) => (
                        <td
                          key={m}
                          className={cn("px-2 py-2 text-end tabular-nums", m !== phoneMetric && "phone:hidden")}
                        >
                          {formatMetric(m, num(r.metrics, m), i18n.locale)}
                        </td>
                      ))}
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
          {table.hasNextPage && (
            <div className="flex justify-center border-t p-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => void table.fetchNextPage()}
                disabled={table.isFetchingNextPage}
              >
                <Trans>Load more</Trans>
              </Button>
            </div>
          )}
        </div>
      </section>
    </WhenRequests>
  )
}

function statusTone(code: string) {
  if (code === "0" || code.startsWith("5")) return "bg-destructive"
  if (code.startsWith("4")) return "bg-warning"
  if (code.startsWith("2")) return "bg-success"
  return "bg-muted-foreground"
}

function StatusCodes({ filters }: { filters: AnalyticsFilter[] }) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const { view } = useApiView()
  const q = useApiQuery(property.id, {
    metrics: ["requests"],
    dimensions: ["status"],
    range: view.range,
    filters,
    limit: 20,
  })
  const rows = q.data?.rows ?? []
  const total = num(q.data?.totals, "requests") ?? 0
  const pct = new Intl.NumberFormat(i18n.locale, { style: "percent", maximumFractionDigits: 1 })
  return (
    <Panel title={<Trans>Status codes</Trans>} testId="api-status-codes">
      {q.isPending ? (
        <Skeleton className="h-32 w-full" />
      ) : q.error ? (
        <PanelProblem error={q.error} />
      ) : rows.length === 0 ? (
        <p className="px-2 py-6 text-center text-body text-faint">
          <Trans>No requests in this range.</Trans>
        </p>
      ) : (
        <BarList
          head={[text.dimension.status, text.metric.requests]}
          items={rows.map((r) => {
            const code = r.dimensions[0] ?? ""
            const v = num(r.metrics, "requests") ?? 0
            return {
              key: code,
              label: (
                <span className="inline-flex items-center gap-2">
                  <span className={cn("size-2 shrink-0 rounded-full", statusTone(code))} aria-hidden />
                  <span className="tabular-nums">{valueText(text, "status", code, i18n.locale)}</span>
                </span>
              ),
              value: v,
              display: `${formatCompact(v, i18n.locale)} · ${pct.format(total ? v / total : 0)}`,
            }
          })}
        />
      )}
    </Panel>
  )
}

export function ApiRoutePage() {
  const property = useCurrentProperty()
  const search = useViewSearch()
  const { method = "", "*": tail = "" } = useParams()
  const route = tail.startsWith("(") ? tail : `/${tail}`
  const filters = routeFilters(route, method)
  return (
    <div className="flex flex-col gap-4" data-testid="api-route">
      <header className="flex flex-col gap-1">
        <Link
          to={`/analytics/${property.id}/routes${search}`}
          className="inline-flex w-fit items-center gap-1.5 text-body text-faint transition-colors hover:text-foreground"
          data-testid="api-route-back"
        >
          <ArrowLeftIcon className="size-3.5 rtl:rotate-180" />
          <Trans>All routes</Trans>
        </Link>
        <h2 className="text-title" data-testid="api-route-name">
          <RouteName method={method} route={route} />
        </h2>
      </header>
      <Kpis filters={filters} testId="api-route-kpis" />
      <Charts filters={filters} />
      <div className="grid gap-4 md:grid-cols-2">
        <StatusCodes filters={filters} />
        <LastHour filters={filters} />
      </div>
    </div>
  )
}

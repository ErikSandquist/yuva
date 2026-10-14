import { Trans, useLingui } from "@lingui/react/macro"
import { ArrowDownIcon, ArrowLeftIcon, EyeIcon, EyeOffIcon } from "lucide-react"
import { useState } from "react"
import { Link, useParams, useSearchParams } from "react-router"

import { ErrorLine, Notice, Segmented } from "@/components/common"
import { formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { ApiInstallSnippets } from "@/features/analytics/Install"
import {
  ALL_METHODS,
  ApdexThreshold,
  ApiControls,
  Blocked,
  blockerOf,
  Charts,
  ConversationsChart,
  HideRouteButton,
  Kpis,
  LastHour,
  LatencyHeatmap,
  RouteName,
  routeFilters,
  routePath,
  SizeStats,
  useApiView,
  useHideRoute,
  useRouteIndex,
} from "@/features/analytics/MonitorParts"
import { BarList, CsvButton, Panel, PanelProblem, QueryProblem, useCurrentProperty } from "@/features/analytics/parts"
import { downloadCsv, useAnalyticsTable, useApiQuery, useApiRoutes } from "@/features/analytics/queries"
import { EXTRA_DIMENSIONS, useViewSearch } from "@/features/analytics/state"
import {
  formatCompact,
  formatMetric,
  num,
  OTHER,
  useAnalyticsText,
  valueText,
  type NumMetric,
} from "@/features/analytics/text"
import { ChoiceSelect } from "@/features/settings/ui"
import type { AnalyticsFilter, AnalyticsFilterDimension, AnalyticsQuery } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"

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

function apiValue(text: ReturnType<typeof useAnalyticsText>, dim: AnalyticsFilterDimension, v: string, locale: string) {
  if (dim === "platform" && v === "") return text.platform.server ?? v
  return valueText(text, dim, v, locale)
}

function TopRoutes() {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const search = useViewSearch()
  const { view } = useApiView()
  const blocked = blockerOf(view.range, view.filters, ["route", "method"])
  const q = useApiQuery(
    property.id,
    { metrics: ["requests"], dimensions: ["route", "method"], range: view.range, filters: view.filters, limit: 8 },
    !blocked
  )
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
      {blocked ? (
        <Blocked reason={blocked} />
      ) : q.isPending ? (
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

const OVERVIEW_DIMS: AnalyticsFilterDimension[] = [
  "service",
  "environment",
  "api_version",
  "method",
  "status_class",
  "status",
  "source",
  "platform",
  "consumer",
  "country",
  "app_version",
]

const ROUTE_DIMS: AnalyticsFilterDimension[] = [
  "status_class",
  "service",
  "environment",
  "api_version",
  "source",
  "platform",
  "country",
  "app_version",
]

const BREAKDOWN_METRICS: NumMetric[] = ["requests", "error_rate", "p95"]

function BreakdownPanel({ base, dims }: { base: AnalyticsFilter[]; dims: AnalyticsFilterDimension[] }) {
  const { t, i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const { view, addFilter } = useApiView()
  const [dim, setDim] = useState(dims[0]!)
  const filters = [...base, ...view.filters]
  const blocked = blockerOf(view.range, filters, [dim])
  const q = useApiQuery(
    property.id,
    { metrics: BREAKDOWN_METRICS, dimensions: [dim], range: view.range, filters, limit: 10 },
    !blocked
  )
  const rows = q.data?.rows ?? []
  const filtered = new Set(filters.map((f) => f.dimension))
  return (
    <Panel
      title={<Trans>Breakdown</Trans>}
      action={
        <ChoiceSelect
          id={`api-breakdown-${base.length}`}
          label={t`Break down by`}
          value={dim}
          onChange={(v) => setDim(v as AnalyticsFilterDimension)}
          options={dims.map((d) => [d, text.dimension[d]] as const)}
          className="w-44"
        />
      }
      testId="api-breakdown"
    >
      {blocked ? (
        <Blocked reason={blocked} />
      ) : q.isPending ? (
        <Skeleton className="h-40 w-full" />
      ) : q.error ? (
        <PanelProblem error={q.error} />
      ) : rows.length === 0 ? (
        <p className="px-2 py-6 text-center text-body text-faint">
          {EXTRA_DIMENSIONS.includes(dim) ? (
            <Trans>No requests in this range carried this value.</Trans>
          ) : (
            <Trans>No requests in this range.</Trans>
          )}
        </p>
      ) : (
        <table className="w-full table-fixed text-body" data-testid="api-breakdown-table">
          <thead>
            <tr className="text-caption text-faint">
              <th className="px-1 pb-1.5 text-start font-medium">{text.dimension[dim]}</th>
              {BREAKDOWN_METRICS.map((m, i) => (
                <th key={m} className={cn("w-20 px-1 pb-1.5 text-end font-medium", i > 0 && "phone:w-16")}>
                  {text.metric[m]}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((r, i) => {
              const v = r.dimensions[0] ?? ""
              const label = apiValue(text, dim, v, i18n.locale)
              const can = v !== OTHER && !filtered.has(dim)
              return (
                <tr key={`${v}${i}`} className="hover:bg-muted/50">
                  <td className="truncate px-1 py-1.5">
                    {can ? (
                      <button
                        type="button"
                        className="max-w-full truncate rounded text-start outline-none hover:text-brand focus-visible:ring-2 focus-visible:ring-ring"
                        onClick={() => addFilter(dim, v)}
                        title={t`Filter by this`}
                        data-testid="api-breakdown-row"
                      >
                        {label}
                      </button>
                    ) : (
                      label
                    )}
                  </td>
                  {BREAKDOWN_METRICS.map((m) => (
                    <td key={m} className="px-1 py-1.5 text-end tabular-nums">
                      {m === "requests"
                        ? formatCompact(num(r.metrics, m) ?? 0, i18n.locale)
                        : formatMetric(m, num(r.metrics, m), i18n.locale)}
                    </td>
                  ))}
                </tr>
              )
            })}
          </tbody>
        </table>
      )}
      {EXTRA_DIMENSIONS.includes(dim) && !blocked && rows.length > 0 && (
        <p className="mt-2 px-1 text-caption text-faint">
          <Trans>
            Only requests that carried a value are counted here, so the rows can add up to less than the total.
          </Trans>
        </p>
      )}
    </Panel>
  )
}

export function ApiOverview() {
  return (
    <WhenRequests>
      <div className="flex flex-col gap-4" data-testid="api-overview">
        <Kpis base={[]} testId="api-kpis" />
        <LastHour base={[]} />
        <Charts base={[]} />
        <div className="grid gap-4 md:grid-cols-2">
          <BreakdownPanel base={[]} dims={OVERVIEW_DIMS} />
          <TopRoutes />
        </div>
        <SizeStats base={[]} />
        <ConversationsChart />
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

function SortPicker({ sort }: { sort: Sort }) {
  const { t } = useLingui()
  const [, setParams] = useSearchParams()
  const names: Record<Sort, string> = { traffic: t`Traffic`, errors: t`Errors`, latency: t`Latency` }
  return (
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
      label={t`Sort by`}
      items={(Object.keys(SORTS) as Sort[]).map((s) => ({ value: s, label: names[s], testId: `api-sort-${s}` }))}
    />
  )
}

function useSort() {
  const [params] = useSearchParams()
  const raw = params.get("sort") as Sort | null
  return raw && raw in SORTS ? raw : "traffic"
}

const TABLE: NumMetric[] = ["requests", "error_rate", "client_error_rate", "p50", "p95", "p99", "apdex"]

function MetricHead({ metrics, sorted, phone }: { metrics: NumMetric[]; sorted?: NumMetric; phone: NumMetric }) {
  const text = useAnalyticsText()
  return (
    <>
      {metrics.map((m) => (
        <th
          key={m}
          className={cn(
            "w-24 px-2 py-2.5 text-end font-medium",
            m !== phone && "phone:hidden",
            m === sorted && "text-foreground"
          )}
          title={text.metricHelp[m]}
          aria-sort={m === sorted ? "descending" : undefined}
        >
          <span className="inline-flex items-center gap-1">
            {text.metric[m]}
            {m === sorted && <ArrowDownIcon className="size-3" />}
          </span>
        </th>
      ))}
    </>
  )
}

type RouteList = "all" | "new" | "silent"

function AllRoutes() {
  const { t, i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const search = useViewSearch()
  const { view } = useApiView()
  const sort = useSort()
  const { canManage } = useSession()
  const index = useRouteIndex(property.id)
  const body: AnalyticsQuery = {
    dataset: "api",
    metrics: TABLE,
    dimensions: ["route", "method"],
    range: view.range,
    filters: view.filters,
    order_by: SORTS[sort].order,
    limit: 100,
  }
  const blocked = blockerOf(view.range, view.filters, ["route", "method"])
  const table = useAnalyticsTable(property.id, body, !blocked)
  const rows = table.data?.pages.flatMap((p) => p.rows) ?? []
  const phone = SORTS[sort].metric
  const cols = TABLE.length + (canManage ? 2 : 1)
  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="-mx-1 max-w-[calc(100%+0.5rem)] overflow-x-auto px-1 py-1">
          <SortPicker sort={sort} />
        </div>
        <CsvButton testId="analytics-csv" run={() => downloadCsv(property.id, body, `${property.name}-routes`)} />
      </div>
      {blocked ? (
        <Blocked reason={blocked} />
      ) : (
        <>
          <QueryProblem error={table.error} />
          <div className="overflow-hidden rounded-2xl border bg-card">
            <table className="w-full table-fixed text-body" data-testid="api-routes-table">
              <thead>
                <tr className="border-b text-caption text-faint">
                  <th className="px-4 py-2.5 text-start font-medium">{text.dimension.route}</th>
                  <MetricHead metrics={TABLE} sorted={SORTS[sort].metric} phone={phone} />
                  {canManage && (
                    <th className="w-11">
                      <span className="sr-only">{t`Actions`}</span>
                    </th>
                  )}
                </tr>
              </thead>
              <tbody className="divide-y">
                {table.isPending ? (
                  <tr>
                    <td colSpan={cols} className="p-4">
                      <Skeleton className="h-24 w-full" />
                    </td>
                  </tr>
                ) : rows.length === 0 ? (
                  <tr>
                    <td colSpan={cols} className="px-4 py-6 text-faint">
                      <Trans>No requests in this range.</Trans>
                    </td>
                  </tr>
                ) : (
                  rows.map((r, i) => {
                    const [route = "", method = ""] = r.dimensions
                    const info = index.get(route)
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
                              <RouteName method={method} route={route} inferred={info?.inferred} wrap />
                            </Link>
                          )}
                        </td>
                        {TABLE.map((m) => (
                          <td key={m} className={cn("px-2 py-2 text-end tabular-nums", m !== phone && "phone:hidden")}>
                            {formatMetric(m, num(r.metrics, m), i18n.locale)}
                          </td>
                        ))}
                        {canManage && (
                          <td className="py-1 pe-2 text-end">
                            {route !== OTHER && <HideRouteButton route={info} propertyId={property.id} />}
                          </td>
                        )}
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
        </>
      )}
    </>
  )
}

const CHANGED: NumMetric[] = ["requests", "error_rate", "p95"]

function ChangedRoutes({ list }: { list: "new" | "silent" }) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const search = useViewSearch()
  const { view } = useApiView()
  const index = useRouteIndex(property.id)
  const blocked = blockerOf(view.range, view.filters, ["route"])
  const q = useApiQuery(
    property.id,
    {
      metrics: CHANGED,
      dimensions: ["route"],
      range: view.range,
      filters: view.filters,
      routes: list,
      compare: list === "silent" ? { with: "previous" } : undefined,
      limit: 1000,
    },
    !blocked
  )
  const rows = q.data?.rows ?? []
  const before = new Map((q.data?.comparison?.rows ?? []).map((r) => [r.dimensions[0] ?? "", r]))
  const metrics: NumMetric[] = list === "new" ? CHANGED : ["requests"]
  return blocked ? (
    <Blocked reason={blocked} />
  ) : (
    <>
      <p className="text-body text-muted-foreground">
        {list === "new" ? (
          <Trans>Routes that got their first request in this range.</Trans>
        ) : (
          <Trans>Routes that were called in the period before this range and not since it began.</Trans>
        )}
      </p>
      <QueryProblem error={q.error} />
      <div className="overflow-hidden rounded-2xl border bg-card">
        <table className="w-full table-fixed text-body" data-testid={`api-routes-${list}`}>
          <thead>
            <tr className="border-b text-caption text-faint">
              <th className="px-4 py-2.5 text-start font-medium">{text.dimension.route}</th>
              {list === "silent" ? (
                <th className="w-28 px-2 py-2.5 text-end font-medium">
                  <Trans>Requests before</Trans>
                </th>
              ) : (
                <MetricHead metrics={metrics} phone="requests" />
              )}
              <th className="w-44 px-4 py-2.5 text-end font-medium phone:hidden">
                {list === "new" ? <Trans>First request</Trans> : <Trans>Last request</Trans>}
              </th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {q.isPending ? (
              <tr>
                <td colSpan={metrics.length + 2} className="p-4">
                  <Skeleton className="h-24 w-full" />
                </td>
              </tr>
            ) : rows.length === 0 ? (
              <tr>
                <td colSpan={metrics.length + 2} className="px-4 py-6 text-faint">
                  {list === "new" ? <Trans>No new routes in this range.</Trans> : <Trans>No route went silent.</Trans>}
                </td>
              </tr>
            ) : (
              rows.map((r) => {
                const route = r.dimensions[0] ?? ""
                const info = index.get(route)
                const at = list === "new" ? info?.first_seen_at : info?.last_seen_at
                return (
                  <tr key={route} className="hover:bg-muted/50" data-testid="api-route-row">
                    <td className="px-4 py-2">
                      <Link
                        to={`${routePath(property.id, ALL_METHODS, route)}${search}`}
                        className="block max-w-full rounded outline-none hover:text-brand focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        <RouteName route={route} inferred={info?.inferred} wrap />
                      </Link>
                    </td>
                    {list === "silent" ? (
                      <td className="px-2 py-2 text-end tabular-nums">
                        {formatMetric("requests", num(before.get(route)?.metrics, "requests"), i18n.locale)}
                      </td>
                    ) : (
                      metrics.map((m) => (
                        <td
                          key={m}
                          className={cn("px-2 py-2 text-end tabular-nums", m !== "requests" && "phone:hidden")}
                        >
                          {formatMetric(m, num(r.metrics, m), i18n.locale)}
                        </td>
                      ))
                    )}
                    <td className="px-4 py-2 text-end text-small text-muted-foreground tabular-nums phone:hidden">
                      {at ? formatDateTime(at, i18n.locale) : "–"}
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
    </>
  )
}

export function ApiRoutes() {
  const { t } = useLingui()
  const [params, setParams] = useSearchParams()
  const raw = params.get("list") as RouteList | null
  const list: RouteList = raw === "new" || raw === "silent" ? raw : "all"
  const names: Record<RouteList, string> = { all: t`All routes`, new: t`New`, silent: t`Silent` }
  return (
    <WhenRequests>
      <section className="flex flex-col gap-3" data-testid="api-routes">
        <ApiControls compare={false} />
        <div className="-mx-1 max-w-[calc(100%+0.5rem)] overflow-x-auto px-1 py-1">
          <Segmented
            value={list}
            onChange={(v) =>
              setParams(
                (prev) => {
                  const next = new URLSearchParams(prev)
                  if (v === "all") next.delete("list")
                  else next.set("list", v)
                  return next
                },
                { replace: true }
              )
            }
            label={t`Which routes`}
            items={(["all", "new", "silent"] as RouteList[]).map((v) => ({
              value: v,
              label: names[v],
              testId: `api-routes-list-${v}`,
            }))}
          />
        </div>
        {list === "all" ? <AllRoutes /> : <ChangedRoutes list={list} />}
      </section>
    </WhenRequests>
  )
}

const CONSUMER_METRICS: NumMetric[] = ["requests", "errors", "error_rate", "p50", "p95", "p99"]

export function ApiConsumers() {
  const { t, i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const { view, addFilter } = useApiView()
  const sort = useSort()
  const body: AnalyticsQuery = {
    dataset: "api",
    metrics: CONSUMER_METRICS,
    dimensions: ["consumer"],
    range: view.range,
    filters: view.filters,
    order_by: SORTS[sort].order,
    limit: 100,
  }
  const blocked = blockerOf(view.range, view.filters, ["consumer"])
  const table = useAnalyticsTable(property.id, body, !blocked)
  const rows = table.data?.pages.flatMap((p) => p.rows) ?? []
  const phone = SORTS[sort].metric
  const filtered = view.filters.some((f) => f.dimension === "consumer")
  return (
    <WhenRequests>
      <section className="flex flex-col gap-3" data-testid="api-consumers">
        <ApiControls meta={table.data?.pages[0]?.meta} compare={false} />
        <p className="text-body text-muted-foreground">
          <Trans>
            Who called the API, as your server names them. Requests without a consumer are not in this list, and
            consumers are counted per hour.
          </Trans>
        </p>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="-mx-1 max-w-[calc(100%+0.5rem)] overflow-x-auto px-1 py-1">
            <SortPicker sort={sort} />
          </div>
          {!blocked && (
            <CsvButton
              testId="analytics-csv"
              run={() => downloadCsv(property.id, body, `${property.name}-consumers`)}
            />
          )}
        </div>
        {blocked ? (
          <Blocked reason={blocked} />
        ) : (
          <>
            <QueryProblem error={table.error} />
            <div className="overflow-hidden rounded-2xl border bg-card">
              <table className="w-full table-fixed text-body" data-testid="api-consumers-table">
                <thead>
                  <tr className="border-b text-caption text-faint">
                    <th className="px-4 py-2.5 text-start font-medium">{text.dimension.consumer}</th>
                    <MetricHead metrics={CONSUMER_METRICS} sorted={SORTS[sort].metric} phone={phone} />
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {table.isPending ? (
                    <tr>
                      <td colSpan={CONSUMER_METRICS.length + 1} className="p-4">
                        <Skeleton className="h-24 w-full" />
                      </td>
                    </tr>
                  ) : rows.length === 0 ? (
                    <tr>
                      <td colSpan={CONSUMER_METRICS.length + 1} className="px-4 py-6 text-faint">
                        <Trans>No requests in this range named a consumer.</Trans>
                      </td>
                    </tr>
                  ) : (
                    rows.map((r, i) => {
                      const v = r.dimensions[0] ?? ""
                      const label = valueText(text, "consumer", v, i18n.locale)
                      return (
                        <tr key={`${v}${i}`} className="hover:bg-muted/50" data-testid="api-consumer-row">
                          <td className="truncate px-4 py-2">
                            {v === OTHER || filtered ? (
                              label
                            ) : (
                              <button
                                type="button"
                                className="max-w-full truncate rounded text-start outline-none hover:text-brand focus-visible:ring-2 focus-visible:ring-ring"
                                onClick={() => addFilter("consumer", v)}
                                title={t`Filter by this`}
                              >
                                {label}
                              </button>
                            )}
                          </td>
                          {CONSUMER_METRICS.map((m) => (
                            <td
                              key={m}
                              className={cn("px-2 py-2 text-end tabular-nums", m !== phone && "phone:hidden")}
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
          </>
        )}
      </section>
    </WhenRequests>
  )
}

function TopConsumers({ base }: { base: AnalyticsFilter[] }) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const search = useViewSearch()
  const { view } = useApiView()
  const filters = [...base, ...view.filters]
  const blocked = blockerOf(view.range, filters, ["consumer"])
  const q = useApiQuery(
    property.id,
    { metrics: ["requests"], dimensions: ["consumer"], range: view.range, filters, limit: 8 },
    !blocked
  )
  const rows = q.data?.rows ?? []
  return (
    <Panel
      title={<Trans>Top consumers</Trans>}
      testId="api-top-consumers"
      footer={
        <Link
          to={`/analytics/${property.id}/consumers${search}`}
          className="text-small text-brand underline-offset-4 hover:underline"
        >
          <Trans>All consumers</Trans>
        </Link>
      }
    >
      {blocked ? (
        <Blocked reason={blocked} />
      ) : q.isPending ? (
        <Skeleton className="h-40 w-full" />
      ) : q.error ? (
        <PanelProblem error={q.error} />
      ) : rows.length === 0 ? (
        <p className="px-2 py-6 text-center text-body text-faint">
          <Trans>No requests in this range named a consumer.</Trans>
        </p>
      ) : (
        <BarList
          head={[text.dimension.consumer, text.metric.requests]}
          items={rows.map((r, i) => {
            const v = r.dimensions[0] ?? ""
            const n = num(r.metrics, "requests") ?? 0
            return {
              key: `${v}${i}`,
              label: valueText(text, "consumer", v, i18n.locale),
              value: n,
              display: formatCompact(n, i18n.locale),
            }
          })}
        />
      )}
    </Panel>
  )
}

function statusTone(code: string) {
  if (code === "0" || code.startsWith("5")) return "bg-destructive"
  if (code.startsWith("4")) return "bg-warning"
  if (code.startsWith("2")) return "bg-success"
  return "bg-muted-foreground"
}

function StatusCodes({ base }: { base: AnalyticsFilter[] }) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const { view } = useApiView()
  const filters = [...base, ...view.filters]
  const blocked = blockerOf(view.range, filters, ["status"])
  const q = useApiQuery(
    property.id,
    { metrics: ["requests"], dimensions: ["status"], range: view.range, filters, limit: 20 },
    !blocked
  )
  const rows = q.data?.rows ?? []
  const total = num(q.data?.totals, "requests") ?? 0
  const pct = new Intl.NumberFormat(i18n.locale, { style: "percent", maximumFractionDigits: 1 })
  return (
    <Panel title={<Trans>Status codes</Trans>} testId="api-status-codes">
      {blocked ? (
        <Blocked reason={blocked} />
      ) : q.isPending ? (
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
  const base = routeFilters(route, method)
  const { canManage } = useSession()
  const routes = useApiRoutes(property.id)
  const info = useRouteIndex(property.id).get(route)
  const hide = useHideRoute(property.id)
  const dims = method === ALL_METHODS ? (["method", ...ROUTE_DIMS] as AnalyticsFilterDimension[]) : ROUTE_DIMS
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
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h2 className="min-w-0 flex-1 text-title" data-testid="api-route-name">
            <RouteName method={method} route={route} inferred={info?.inferred} wrap />
          </h2>
          {canManage && info && !info.hidden && (
            <Button
              variant="outline"
              size="sm"
              disabled={hide.pending}
              onClick={() => hide.run(info, true)}
              data-testid="api-hide-route"
            >
              <EyeOffIcon />
              <Trans>Hide route</Trans>
            </Button>
          )}
        </div>
        <ErrorLine error={hide.error} />
      </header>
      {info?.hidden && (
        <Notice icon={EyeOffIcon} data-testid="api-route-hidden">
          <p>
            <Trans>
              This route is hidden, with all its methods, so its numbers are left out of every view. Requests to it are
              still counted and show again with it.
            </Trans>
          </p>
          {canManage && (
            <Button
              size="sm"
              variant="outline"
              className="w-fit"
              disabled={hide.pending}
              onClick={() => hide.run(info, false)}
              data-testid="api-show-route"
            >
              <EyeIcon />
              <Trans>Show it again</Trans>
            </Button>
          )}
        </Notice>
      )}
      {routes.isPending ? (
        <Skeleton className="h-64 rounded-2xl" />
      ) : (
        !info?.hidden && (
          <>
            <Kpis base={base} testId="api-route-kpis" />
            {info && <ApdexThreshold key={`${info.id}-${info.apdex_ms ?? ""}`} route={info} canManage={canManage} />}
            <Charts base={base} />
            <LatencyHeatmap base={base} />
            <div className="grid gap-4 md:grid-cols-2">
              <StatusCodes base={base} />
              <LastHour base={base} />
            </div>
            <div className="grid gap-4 md:grid-cols-2">
              <BreakdownPanel base={base} dims={dims} />
              <TopConsumers base={base} />
            </div>
            <SizeStats base={base} />
          </>
        )
      )}
    </div>
  )
}

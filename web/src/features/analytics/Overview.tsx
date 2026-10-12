import { Trans, useLingui } from "@lingui/react/macro"
import { ArrowDownRightIcon, ArrowUpRightIcon } from "lucide-react"
import { useState } from "react"
import { Link } from "react-router"
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts"

import { Segmented } from "@/components/common"
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart"
import { Skeleton } from "@/components/ui/skeleton"
import { groupDimensions, RowLabel, type Group } from "@/features/analytics/Breakdown"
import {
  Attribution,
  BarList,
  CsvButton,
  Panel,
  PanelProblem,
  QueryProblem,
  useCurrentProperty,
} from "@/features/analytics/parts"
import { downloadCsv, useAnalytics, type Query } from "@/features/analytics/queries"
import { grainFor, KPIS, useView, useViewSearch } from "@/features/analytics/state"
import { formatCompact, formatMetric, lowerIsBetter, OTHER, useAnalyticsText } from "@/features/analytics/text"
import type { AnalyticsDimension, AnalyticsFilterDimension, AnalyticsMetric } from "@/lib/api"
import { cn } from "@/lib/utils"

function Change({ now, before, metric }: { now?: number; before?: number; metric: AnalyticsMetric }) {
  const { i18n } = useLingui()
  if (now === undefined || before === undefined) return null
  if (before === 0) return <span className="text-caption text-faint">–</span>
  const change = (now - before) / before
  const up = change >= 0
  const good = lowerIsBetter(metric) ? !up : up
  const Icon = up ? ArrowUpRightIcon : ArrowDownRightIcon
  return (
    <span
      className={cn(
        "inline-flex items-center gap-0.5 text-caption font-medium tabular-nums",
        change === 0 ? "text-faint" : good ? "text-success" : "text-destructive"
      )}
      data-testid="kpi-change"
    >
      <Icon className="size-3.5" />
      {new Intl.NumberFormat(i18n.locale, { style: "percent", maximumFractionDigits: 0, signDisplay: "never" }).format(
        change
      )}
    </span>
  )
}

function Kpis() {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const { view, setMetric } = useView()
  const q = useAnalytics(property.id, {
    metrics: KPIS,
    range: view.range,
    filters: view.filters,
    compare: view.compare,
  })
  const totals = q.data?.totals
  const before = q.data?.comparison?.totals
  return (
    <>
      <QueryProblem error={q.error} />
      <div className="grid grid-cols-3 gap-2 phone:grid-cols-2" role="group" data-testid="analytics-kpis">
        {KPIS.map((m) => (
          <button
            key={m}
            type="button"
            aria-pressed={view.metric === m}
            onClick={() => setMetric(m)}
            title={text.metricHelp[m]}
            className={cn(
              "flex min-w-0 flex-col items-start rounded-2xl border bg-card px-4 py-3 text-start outline-none transition-colors hover:border-foreground/20 focus-visible:ring-2 focus-visible:ring-ring phone:px-3",
              view.metric === m && "border-brand/60 ring-1 ring-brand/30"
            )}
            data-testid={`kpi-${m}`}
          >
            <span className="max-w-full truncate text-small text-faint">{text.metric[m]}</span>
            {q.isPending ? (
              <Skeleton className="mt-1 h-7 w-16" />
            ) : (
              <span className="flex flex-wrap items-baseline gap-x-2">
                <b className="text-page tabular-nums">{formatMetric(m, totals?.[m], i18n.locale)}</b>
                {view.compare && <Change now={totals?.[m]} before={before?.[m]} metric={m} />}
              </span>
            )}
            {view.compare && !q.isPending && (
              <span className="max-w-full truncate text-caption text-faint tabular-nums">
                {formatMetric(m, before?.[m], i18n.locale)}
              </span>
            )}
          </button>
        ))}
      </div>
    </>
  )
}

function bucketDate(grain: AnalyticsDimension, v: string) {
  return new Date(grain === "time:hour" ? `${v}:00Z` : `${v}T00:00:00Z`)
}

function TimeChart() {
  const { t, i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const { view } = useView()
  const metric = view.metric
  const grain = grainFor(view.range, metric)
  const q = useAnalytics(property.id, {
    metrics: [metric],
    dimensions: [grain],
    range: view.range,
    filters: view.filters,
    compare: view.compare,
  })
  const hour = grain === "time:hour"
  const tick = new Intl.DateTimeFormat(
    i18n.locale,
    hour
      ? { timeStyle: "short", timeZone: "UTC" }
      : {
          day: "numeric",
          month: "short",
          timeZone: "UTC",
        }
  )
  const long = new Intl.DateTimeFormat(i18n.locale, {
    dateStyle: "medium",
    ...(hour ? { timeStyle: "short" } : {}),
    timeZone: "UTC",
  })
  const before = q.data?.comparison?.rows ?? []
  const data = (q.data?.rows ?? []).map((r, i) => ({
    at: r.dimensions[0] ?? "",
    value: r.metrics[metric] ?? 0,
    previous: view.compare ? (before[i]?.metrics[metric] ?? 0) : undefined,
    previousAt: before[i]?.dimensions[0],
  }))
  const config: ChartConfig = {
    value: { label: text.metric[metric], color: "var(--brand)" },
    ...(view.compare ? { previous: { label: t`Compared days`, color: "var(--muted-foreground)" } } : {}),
  }
  const weekly = grain === "time:week"
  return (
    <Panel
      title={
        <>
          {text.metric[metric]}
          <span className="ms-2 text-caption font-normal text-faint">
            {weekly ? <Trans>per week</Trans> : hour ? <Trans>per hour</Trans> : <Trans>per day</Trans>}
          </span>
        </>
      }
      testId="analytics-chart"
    >
      {q.isPending ? (
        <Skeleton className="h-56 w-full" />
      ) : q.error ? (
        <PanelProblem error={q.error} />
      ) : (
        <ChartContainer config={config} className="aspect-auto h-56 w-full">
          <AreaChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: -12 }}>
            <CartesianGrid vertical={false} />
            <XAxis
              dataKey="at"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              minTickGap={28}
              interval="preserveStartEnd"
              tickFormatter={(v) => tick.format(bucketDate(grain, String(v)))}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              width={48}
              allowDecimals={metric === "views_per_visit"}
              tickFormatter={(v) => formatMetric(metric, Number(v), i18n.locale)}
            />
            <ChartTooltip
              cursor={false}
              content={
                <ChartTooltipContent
                  labelFormatter={(_, p) => (p?.[0] ? long.format(bucketDate(grain, String(p[0].payload.at))) : "")}
                  formatter={(value, name, item) => (
                    <div className="flex w-full items-center gap-2">
                      <span
                        className={cn(
                          "size-2.5 shrink-0 rounded-[2px]",
                          name === "previous" ? "border border-dashed border-muted-foreground" : "bg-brand"
                        )}
                      />
                      <span className="flex-1 text-muted-foreground">
                        {name === "previous" && item.payload.previousAt
                          ? long.format(bucketDate(grain, String(item.payload.previousAt)))
                          : config[String(name)]?.label}
                      </span>
                      <span className="font-mono font-medium text-foreground tabular-nums">
                        {formatMetric(metric, Number(value), i18n.locale)}
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
            <Area
              dataKey="value"
              type="monotone"
              stroke="var(--color-value)"
              strokeWidth={2}
              fill="var(--color-value)"
              fillOpacity={0.12}
              isAnimationActive={false}
            />
          </AreaChart>
        </ChartContainer>
      )}
    </Panel>
  )
}

function BreakdownCard({
  group,
  title,
  dims,
  testId,
}: {
  group: Group
  title: React.ReactNode
  dims: AnalyticsFilterDimension[]
  testId: string
}) {
  const { t, i18n } = useLingui()
  const text = useAnalyticsText()
  const property = useCurrentProperty()
  const search = useViewSearch()
  const { view, addFilter } = useView()
  const [dim, setDim] = useState(dims[0])
  const by = dims.includes(dim) ? dim : dims[0]
  const body: Query = {
    metrics: ["visitors"],
    dimensions: [by],
    range: view.range,
    filters: view.filters,
    limit: 8,
  }
  const q = useAnalytics(property.id, body)
  const rows = q.data?.rows ?? []
  const join = search ? `${search}&by=${by}` : `?by=${by}`
  return (
    <Panel
      title={title}
      testId={testId}
      action={
        dims.length > 1 && (
          <Segmented
            value={by}
            onChange={setDim}
            label={t`Breakdown`}
            itemClassName="h-6 px-2 text-small"
            items={dims.map((d) => ({ value: d, label: text.dimension[d] }))}
          />
        )
      }
      footer={
        <>
          <Link
            to={`/analytics/${property.id}/${group}${join}`}
            className="text-small text-brand underline-offset-4 hover:underline"
          >
            <Trans>Details</Trans>
          </Link>
          <span className="flex-1" />
          {group === "locations" && <Attribution propertyId={property.id} />}
          <CsvButton run={() => downloadCsv(property.id, body, `${property.name}-${by}`)} />
        </>
      }
    >
      {q.isPending ? (
        <Skeleton className="h-40 w-full" />
      ) : q.error ? (
        <PanelProblem error={q.error} />
      ) : rows.length === 0 ? (
        <p className="px-2 py-6 text-center text-body text-faint">
          <Trans>Nothing counted in this range.</Trans>
        </p>
      ) : (
        <BarList
          head={[text.dimension[by], text.metric.visitors]}
          items={rows.map((r, i) => {
            const value = r.dimensions[0] ?? ""
            return {
              key: `${value}${i}`,
              label: <RowLabel dim={by} row={r} value={value} text={text} />,
              value: r.metrics.visitors ?? 0,
              display: formatCompact(r.metrics.visitors ?? 0, i18n.locale),
              onClick: value === OTHER ? undefined : () => addFilter(by, value),
            }
          })}
        />
      )}
    </Panel>
  )
}

export function Overview() {
  const property = useCurrentProperty()
  const text = useAnalyticsText()
  const sources: AnalyticsFilterDimension[] = ["channel", "source", "utm_campaign"]
  return (
    <div className="flex flex-col gap-4" data-testid="analytics-overview">
      <Kpis />
      <TimeChart />
      <div className="grid gap-4 md:grid-cols-2">
        <BreakdownCard
          group="pages"
          title={<Trans>Top pages</Trans>}
          dims={["page", "entry_page", "exit_page"]}
          testId="card-pages"
        />
        <BreakdownCard group="sources" title={<Trans>Sources</Trans>} dims={sources} testId="card-sources" />
        <BreakdownCard
          group="locations"
          title={<Trans>Locations</Trans>}
          dims={groupDimensions("locations", property)}
          testId="card-locations"
        />
        <BreakdownCard
          group="devices"
          title={<Trans>Devices</Trans>}
          dims={["device", "browser", "os"]}
          testId="card-devices"
        />
      </div>
      <p className="mx-1 text-caption text-faint">{text.metricHelp.visitors}</p>
    </div>
  )
}

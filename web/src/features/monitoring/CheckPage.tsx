import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { PauseIcon, PencilIcon, PlayIcon, ShieldCheckIcon, Trash2Icon } from "lucide-react"
import { useState } from "react"
import { Link, useNavigate, useParams } from "react-router"
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts"

import { ErrorBoundary, ErrorLine, Segmented, useConfirm } from "@/components/common"
import { Pane, PaneBack } from "@/components/common/Column"
import { formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart"
import { Skeleton } from "@/components/ui/skeleton"
import { Panel, PanelProblem } from "@/features/analytics/parts"
import { useAnalyticsProperties } from "@/features/analytics/properties"
import { formatMetric, num, useAnalyticsText, type NumMetric } from "@/features/analytics/text"
import { AlertHistory, AlertRules } from "@/features/monitoring/Alerts"
import { useCheck, useCheckResults, useDeleteCheck, useSaveCheck, useUptime } from "@/features/monitoring/queries"
import { formatInterval, formatMs, useMonitoringText } from "@/features/monitoring/text"
import type {
  AnalyticsDimension,
  AnalyticsFilter,
  MonitorCheck,
  MonitorCheckResult,
  MonitorCheckState,
} from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"

type Span = "24h" | "7d" | "30d" | "90d"

const GRAIN: Record<Span, AnalyticsDimension> = {
  "24h": "time:hour",
  "7d": "time:hour",
  "30d": "time:day",
  "90d": "time:day",
}

export function StateDot({
  state,
  paused,
  className,
}: {
  state: MonitorCheckState
  paused?: boolean
  className?: string
}) {
  return (
    <span
      aria-hidden
      className={cn(
        "inline-block size-2.5 shrink-0 rounded-full",
        paused
          ? "bg-muted-foreground/50"
          : state === "up"
            ? "bg-success"
            : state === "down"
              ? "bg-destructive"
              : "bg-warning",
        className
      )}
    />
  )
}

export function StateText({ check }: { check: MonitorCheck }) {
  const text = useMonitoringText()
  if (check.paused) return <Trans>Paused</Trans>
  return <>{text.state[check.state]}</>
}

function wall(v: string) {
  return new Date(`${v.length > 10 ? v.slice(0, 16) : `${v}T00:00`}:00Z`)
}

const FIGURES: NumMetric[] = ["uptime", "downtime", "response_time_p50", "response_time_p95", "runs", "failures"]

function Figures({ filters, span, https }: { filters: AnalyticsFilter[]; span: Span; https: boolean }) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const metrics: NumMetric[] = https ? [...FIGURES, "cert_days_left"] : FIGURES
  const q = useUptime({ metrics, range: { period: span }, filters })
  return (
    <div className="grid grid-cols-4 gap-2 phone:grid-cols-2" data-testid="check-figures">
      {metrics.map((m) => (
        <div
          key={m}
          className="flex min-w-0 flex-col rounded-2xl border bg-card px-4 py-3 phone:px-3"
          title={text.metricHelp[m]}
        >
          <span className="truncate text-small text-faint">{text.metric[m]}</span>
          {q.isPending ? (
            <Skeleton className="mt-1 h-7 w-16" />
          ) : q.error ? (
            <span className="text-body text-faint">–</span>
          ) : (
            <b className="text-page tabular-nums">
              {m === "cert_days_left"
                ? (num(q.data?.totals, m) ?? "–")
                : formatMetric(m, num(q.data?.totals, m), i18n.locale)}
            </b>
          )}
        </div>
      ))}
    </div>
  )
}

function ResponseChart({ filters, span }: { filters: AnalyticsFilter[]; span: Span }) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const grain = GRAIN[span]
  const q = useUptime({
    metrics: ["response_time_p50", "response_time_p95", "uptime"],
    dimensions: [grain],
    range: { period: span },
    filters,
  })
  const hourly = grain === "time:hour"
  const tick = new Intl.DateTimeFormat(
    i18n.locale,
    hourly
      ? { hour: "numeric", minute: "2-digit", timeZone: "UTC" }
      : { day: "numeric", month: "short", timeZone: "UTC" }
  )
  const long = new Intl.DateTimeFormat(i18n.locale, {
    day: "numeric",
    month: "short",
    ...(hourly ? { hour: "numeric", minute: "2-digit" } : { year: "numeric" }),
    timeZone: "UTC",
  })
  const data = (q.data?.rows ?? []).map((r) => ({
    at: r.dimensions[0] ?? "",
    p50: num(r.metrics, "response_time_p50") ?? null,
    p95: num(r.metrics, "response_time_p95") ?? null,
    uptime: num(r.metrics, "uptime"),
  }))
  const top = Math.max(0, ...data.map((d) => d.p95 ?? 0))
  const width = Math.max(44, Math.ceil(formatMs(top * 1.25, i18n.locale).length * 7.5) + 14)
  const config: ChartConfig = {
    p95: { label: text.metric.response_time_p95, color: "var(--brand)" },
    p50: { label: text.metric.response_time_p50, color: "var(--muted-foreground)" },
  }
  return (
    <Panel
      title={
        <>
          <Trans>Response time</Trans>
          <span className="ms-2 text-caption font-normal text-faint">
            {hourly ? <Trans>per hour, UTC</Trans> : <Trans>per day, UTC</Trans>}
          </span>
        </>
      }
      action={
        <span className="flex flex-wrap items-center gap-3 text-caption text-muted-foreground">
          <span className="inline-flex items-center gap-1.5">
            <span className="size-2.5 rounded-[2px] bg-brand" />
            {text.metric.response_time_p95}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="size-2.5 rounded-[2px] bg-muted-foreground" />
            {text.metric.response_time_p50}
          </span>
        </span>
      }
      testId="check-response-chart"
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
              tickFormatter={(v) => tick.format(wall(String(v)))}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              width={width}
              tickFormatter={(v) => formatMs(Number(v), i18n.locale)}
            />
            <ChartTooltip
              cursor={false}
              content={
                <ChartTooltipContent
                  labelFormatter={(_, p) => {
                    const row = p?.[0]?.payload as (typeof data)[number] | undefined
                    if (!row) return ""
                    const up = formatMetric("uptime", row.uptime, i18n.locale)
                    return `${long.format(wall(row.at))} · ${text.metric.uptime} ${up}`
                  }}
                  formatter={(value, name) => (
                    <div className="flex w-full items-center gap-2">
                      <span
                        className={cn(
                          "size-2.5 shrink-0 rounded-[2px]",
                          name === "p95" ? "bg-brand" : "bg-muted-foreground"
                        )}
                      />
                      <span className="flex-1 text-muted-foreground">{config[String(name)]?.label}</span>
                      <span className="font-mono font-medium text-foreground tabular-nums">
                        {formatMs(Number(value), i18n.locale)}
                      </span>
                    </div>
                  )}
                />
              }
            />
            <Area
              dataKey="p95"
              type="monotone"
              stroke="var(--color-p95)"
              strokeWidth={2}
              fill="var(--color-p95)"
              fillOpacity={0.12}
              connectNulls
              isAnimationActive={false}
            />
            <Area
              dataKey="p50"
              type="monotone"
              stroke="var(--color-p50)"
              strokeWidth={1.5}
              fill="none"
              connectNulls
              isAnimationActive={false}
            />
          </AreaChart>
        </ChartContainer>
      )}
    </Panel>
  )
}

function Timings({ r }: { r: MonitorCheckResult }) {
  const { i18n } = useLingui()
  const parts: [string, number][] = [
    ["DNS", r.timings.dns],
    ["TCP", r.timings.connect],
    ["TLS", r.timings.tls],
    ["TTFB", r.timings.ttfb],
  ]
  return (
    <span className="flex flex-wrap gap-x-3 gap-y-0.5 text-caption text-faint tabular-nums" dir="ltr">
      {parts.map(([k, v]) => (
        <span key={k}>
          {k} {formatMs(v, i18n.locale)}
        </span>
      ))}
    </span>
  )
}

function CertDays({ days }: { days: number }) {
  return (
    <span className="inline-flex items-center gap-1 text-caption text-faint">
      <ShieldCheckIcon className="size-3" />
      <Plural value={days} one="# day left" other="# days left" />
    </span>
  )
}

function Runs({ checkId }: { checkId: string }) {
  const { i18n } = useLingui()
  const text = useMonitoringText()
  const results = useCheckResults(checkId)
  const items = results.data?.pages.flatMap((p) => p.items) ?? []
  return (
    <Panel title={<Trans>Recent runs</Trans>} testId="check-runs">
      {results.isPending ? (
        <Skeleton className="h-40 w-full" />
      ) : results.error ? (
        <PanelProblem error={results.error} />
      ) : items.length === 0 ? (
        <p className="px-2 py-6 text-center text-body text-faint">
          <Trans>No runs yet. The first one is due now.</Trans>
        </p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[560px] text-body">
            <thead>
              <tr className="text-caption text-faint">
                <th className="px-1 pb-1.5 text-start font-medium">
                  <Trans>When</Trans>
                </th>
                <th className="px-1 pb-1.5 text-start font-medium">
                  <Trans>Result</Trans>
                </th>
                <th className="px-1 pb-1.5 text-start font-medium">
                  <Trans>Timings</Trans>
                </th>
                <th className="w-20 px-1 pb-1.5 text-end font-medium">
                  <Trans>Total</Trans>
                </th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {items.map((r) => (
                <tr key={r.at} data-testid="check-run">
                  <td className="px-1 py-1.5 align-top text-small whitespace-nowrap tabular-nums">
                    {formatDateTime(r.at, i18n.locale)}
                  </td>
                  <td className="px-1 py-1.5 align-top">
                    <span className="flex flex-col gap-0.5">
                      <span className="inline-flex flex-wrap items-center gap-2">
                        <span
                          className={cn(
                            "size-2 rounded-full",
                            r.ok ? "bg-success" : r.state === "unknown" ? "bg-warning" : "bg-destructive"
                          )}
                          aria-hidden
                        />
                        <span className="tabular-nums">{r.status ? r.status : "–"}</span>
                        <span className="text-small text-muted-foreground">{text.runState[r.state]}</span>
                        {r.retried && (
                          <span className="rounded-full bg-muted px-1.5 text-caption text-muted-foreground">
                            <Trans>retried</Trans>
                          </span>
                        )}
                        {r.cert_days_left !== undefined && <CertDays days={r.cert_days_left} />}
                      </span>
                      {!r.ok && r.error && (
                        <span className="text-small text-destructive">
                          {text.error[r.error]}
                          {r.detail && <span className="text-muted-foreground" dir="auto">{` · ${r.detail}`}</span>}
                        </span>
                      )}
                    </span>
                  </td>
                  <td className="px-1 py-1.5 align-top">
                    <Timings r={r} />
                  </td>
                  <td className="px-1 py-1.5 text-end align-top tabular-nums">
                    {formatMs(r.timings.total, i18n.locale)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {results.hasNextPage && (
        <div className="flex justify-center pt-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => void results.fetchNextPage()}
            disabled={results.isFetchingNextPage}
          >
            <Trans>Load more</Trans>
          </Button>
        </div>
      )}
    </Panel>
  )
}

type Period = { state: "up" | "down"; from: string; to?: string }

function periods(results: MonitorCheckResult[]) {
  const out: Period[] = []
  const chronological = [...results].reverse()
  for (const r of chronological) {
    if (r.state !== "up" && r.state !== "down") continue
    const last = out[out.length - 1]
    if (last && last.state === r.state) continue
    if (last) last.to = r.at
    out.push({ state: r.state, from: r.at })
  }
  return out.reverse()
}

function StateHistory({ checkId }: { checkId: string }) {
  const { i18n } = useLingui()
  const text = useMonitoringText()
  const results = useCheckResults(checkId)
  const list = periods(results.data?.pages.flatMap((p) => p.items) ?? []).slice(0, 12)
  return (
    <Panel title={<Trans>State history</Trans>} testId="check-history">
      {results.isPending ? (
        <Skeleton className="h-24 w-full" />
      ) : list.length === 0 ? (
        <p className="px-2 py-6 text-center text-body text-faint">
          <Trans>No counted runs yet.</Trans>
        </p>
      ) : (
        <ul className="flex flex-col gap-1.5 px-1">
          {list.map((p) => {
            const from = formatDateTime(p.from, i18n.locale)
            const to = p.to ? formatDateTime(p.to, i18n.locale) : ""
            return (
              <li key={p.from} className="flex items-center gap-2 text-body">
                <StateDot state={p.state} />
                <span className="font-medium">{text.state[p.state]}</span>
                <span className="text-small text-muted-foreground tabular-nums">
                  {to ? (
                    <Trans>
                      from {from} to {to}
                    </Trans>
                  ) : (
                    <Trans>since {from}</Trans>
                  )}
                </span>
              </li>
            )
          })}
        </ul>
      )}
      <p className="mt-2 px-1 text-caption text-faint">
        <Trans>From the runs kept on the server, up to 30 days.</Trans>
      </p>
    </Panel>
  )
}

function Guarded({ children }: { children: React.ReactNode }) {
  return (
    <ErrorBoundary
      fallback={
        <Panel>
          <p className="px-2 py-6 text-center text-body text-faint">
            <Trans>This part could not be shown. Reload the page to try again.</Trans>
          </p>
        </Panel>
      }
    >
      {children}
    </ErrorBoundary>
  )
}

function DeleteTitle({ name }: { name: string }) {
  return <Trans>Delete the check “{name}”?</Trans>
}

export function CheckView({ check, back }: { check: MonitorCheck; back?: React.ReactNode }) {
  const { t, i18n } = useLingui()
  const { canManage } = useSession()
  const navigate = useNavigate()
  const save = useSaveCheck(check.id)
  const remove = useDeleteCheck(check.id)
  const [ask, confirm] = useConfirm()
  const [span, setSpan] = useState<Span>("24h")
  const properties = useAnalyticsProperties().data?.items ?? []
  const property = properties.find((p) => p.id === check.property_id)
  const filters: AnalyticsFilter[] = [{ dimension: "check", op: "is", values: [check.id] }]
  const since = check.state_since ? formatDateTime(check.state_since, i18n.locale) : ""
  const every = formatInterval(check.interval_seconds, i18n.locale)
  const https = check.url.toLowerCase().startsWith("https://")
  const spans: Record<Span, string> = {
    "24h": t`Last 24 hours`,
    "7d": t`Last 7 days`,
    "30d": t`Last 30 days`,
    "90d": t`Last 90 days`,
  }
  return (
    <div className="flex flex-col gap-5" data-testid="check-page">
      <header className="flex flex-col gap-1">
        {back}
        <div className="flex flex-wrap items-start gap-3">
          <div className="flex min-w-0 flex-1 flex-col">
            <h1 className="flex min-w-0 items-center gap-2.5 text-page break-words" data-testid="check-name">
              <StateDot state={check.state} paused={check.paused} className="size-3" />
              {check.name}
            </h1>
            <p className="mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-body text-muted-foreground">
              <span className="font-medium text-foreground" data-testid="check-state">
                <StateText check={check} />
              </span>
              {since && !check.paused && <span className="tabular-nums">{t`since ${since}`}</span>}
              <span aria-hidden>·</span>
              <span className="min-w-0 truncate font-mono text-small" dir="ltr">
                {check.method} {check.url}
              </span>
              <span aria-hidden>·</span>
              <span>{t`every ${every}`}</span>
              {property && (
                <>
                  <span aria-hidden>·</span>
                  <Link
                    to={`/analytics/${property.id}/uptime`}
                    className="text-brand underline-offset-4 hover:underline"
                  >
                    {property.name}
                  </Link>
                </>
              )}
            </p>
          </div>
          {canManage && (
            <div className="flex shrink-0 flex-wrap gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={save.isPending}
                onClick={() => save.mutate({ paused: !check.paused })}
                data-testid="check-pause"
              >
                {check.paused ? <PlayIcon /> : <PauseIcon />}
                {check.paused ? <Trans>Resume</Trans> : <Trans>Pause</Trans>}
              </Button>
              <Button variant="outline" size="sm" render={<Link to={`/monitoring/checks/${check.id}/edit`} />}>
                <PencilIcon />
                <Trans>Change</Trans>
              </Button>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={t`Delete the check`}
                onClick={() =>
                  ask({
                    title: <DeleteTitle name={check.name} />,
                    description: <Trans>Its runs, its daily numbers and its alert rules go with it.</Trans>,
                    confirm: <Trans>Delete check</Trans>,
                    run: () =>
                      remove.mutate(undefined, { onSuccess: () => navigate("/monitoring", { replace: true }) }),
                  })
                }
                data-testid="check-delete"
              >
                <Trash2Icon />
              </Button>
            </div>
          )}
        </div>
        <ErrorLine error={save.error ?? remove.error} />
      </header>
      <div className="-mx-1 max-w-[calc(100%+0.5rem)] overflow-x-auto px-1 py-1">
        <Segmented
          value={span}
          onChange={setSpan}
          label={t`Range`}
          items={(Object.keys(spans) as Span[]).map((s) => ({ value: s, label: spans[s] }))}
        />
      </div>
      <Guarded>
        <Figures filters={filters} span={span} https={https} />
      </Guarded>
      <Guarded>
        <ResponseChart filters={filters} span={span} />
      </Guarded>
      <div className="grid gap-4 md:grid-cols-[2fr_1fr]">
        <Guarded>
          <Runs checkId={check.id} />
        </Guarded>
        <Guarded>
          <StateHistory checkId={check.id} />
        </Guarded>
      </div>
      <AlertRules scope={{ check_id: check.id }} />
      <AlertHistory scope={{ check_id: check.id }} />
      {confirm}
    </div>
  )
}

export function CheckPage() {
  const { t } = useLingui()
  const { checkId = "" } = useParams()
  const check = useCheck(checkId)
  return (
    <Pane wide testId="check">
      {check.data ? (
        <CheckView check={check.data} back={<PaneBack to="/monitoring" label={t`Monitoring`} />} />
      ) : (
        <>
          <PaneBack to="/monitoring" label={t`Monitoring`} />
          {check.error ? <ErrorLine error={check.error} /> : <Skeleton className="h-64 rounded-2xl" />}
        </>
      )}
    </Pane>
  )
}

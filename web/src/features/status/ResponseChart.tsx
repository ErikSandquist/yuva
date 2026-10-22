import { Trans, useLingui } from "@lingui/react/macro"
import { useState } from "react"
import { Area, CartesianGrid, ComposedChart, Line, ReferenceLine, XAxis, YAxis } from "recharts"

import { ErrorLine, Segmented } from "@/components/common"
import { formatMs } from "@/components/common/text"
import { Skeleton } from "@/components/ui/skeleton"
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart"
import { useMonitorSummary } from "@/features/status/queries"
import { cn } from "@/lib/utils"
import type { StatusCheckRange, StatusMonitorKind } from "@/lib/api"

function Stat({ label, value, testId }: { label: React.ReactNode; value: string; testId?: string }) {
  return (
    <div className="min-w-0">
      <dt className="text-caption text-faint">{label}</dt>
      <dd className="font-mono text-title font-medium tabular-nums" data-testid={testId}>
        {value}
      </dd>
    </div>
  )
}

export function ResponseChart({
  pageId,
  monitorId,
  kind = "http",
}: {
  pageId: string
  monitorId: string
  kind?: StatusMonitorKind
}) {
  const { t, i18n } = useLingui()
  const [range, setRange] = useState<StatusCheckRange>("24h")
  const q = useMonitorSummary(pageId, monitorId, range)
  const s = q.data
  const none = "–"
  const time = new Intl.DateTimeFormat(i18n.locale, { timeStyle: "short" })
  const day = new Intl.DateTimeFormat(i18n.locale, { weekday: "short", day: "numeric" })
  const long = new Intl.DateTimeFormat(i18n.locale, { dateStyle: "medium", timeStyle: "short" })
  const data = (s?.buckets ?? []).map((b) => ({
    at: b.start,
    p50: b.p50_ms,
    p95: b.p95_ms,
    failures: b.failures,
  }))
  const config: ChartConfig = {
    p50: { label: t`Median`, color: "var(--brand)" },
    p95: { label: t`95th percentile`, color: "var(--muted-foreground)" },
  }
  const uptime =
    s?.uptime === undefined
      ? none
      : new Intl.NumberFormat(i18n.locale, { style: "percent", maximumFractionDigits: 2 }).format(s.uptime / 100)
  const hasTimes = data.some((d) => d.p50 !== undefined)
  const timed = kind === "http" || kind === "tcp"
  return (
    <section className="flex flex-col gap-3" data-testid="status-monitor-chart">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-body font-medium">{timed ? <Trans>Response time</Trans> : <Trans>Results</Trans>}</h3>
        <Segmented
          value={range}
          onChange={setRange}
          label={t`Range`}
          items={[
            { value: "24h", label: t`24 hours`, testId: "status-monitor-range-24h" },
            { value: "7d", label: t`7 days`, testId: "status-monitor-range-7d" },
          ]}
        />
      </div>
      <ErrorLine error={q.error} />
      {q.isPending ? (
        <Skeleton className="h-52 w-full rounded-2xl" />
      ) : s ? (
        <>
          <dl className="grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-4">
            <Stat label={<Trans>Uptime</Trans>} value={uptime} testId="status-monitor-uptime" />
            <Stat label={<Trans>Checks</Trans>} value={new Intl.NumberFormat(i18n.locale).format(s.checks)} />
            {timed ? (
              <>
                <Stat
                  label={<Trans>Median</Trans>}
                  value={s.p50_ms === undefined ? none : formatMs(s.p50_ms, i18n.locale)}
                />
                <Stat
                  label={<Trans>95th percentile</Trans>}
                  value={s.p95_ms === undefined ? none : formatMs(s.p95_ms, i18n.locale)}
                />
              </>
            ) : (
              <Stat label={<Trans>Failed</Trans>} value={new Intl.NumberFormat(i18n.locale).format(s.failures)} />
            )}
          </dl>
          {!timed ? (
            <>
              <span
                className="flex h-14 items-end gap-px"
                role="img"
                aria-label={t`Results`}
                data-testid="status-monitor-results"
              >
                {(s.buckets.length > 0 ? s.buckets : Array.from({ length: 48 }, () => null)).map((b, n) => (
                  <span
                    key={n}
                    title={b ? `${long.format(new Date(b.start))} · ${b.checks}` : undefined}
                    className={cn(
                      "min-w-px flex-1 rounded-[2px]",
                      !b || b.checks === 0 ? "bg-muted" : b.failures > 0 ? "bg-destructive" : "bg-success/60"
                    )}
                    style={{ height: !b || b.checks === 0 ? 3 : "100%" }}
                  />
                ))}
              </span>
              <ul className="flex flex-wrap gap-x-4 gap-y-1 text-caption text-muted-foreground" aria-hidden>
                <li className="flex items-center gap-1.5">
                  <span className="size-2.5 rounded-[2px] bg-success/60" />
                  <Trans>OK</Trans>
                </li>
                <li className="flex items-center gap-1.5">
                  <span className="size-2.5 rounded-[2px] bg-destructive" />
                  <Trans>Failed</Trans>
                </li>
              </ul>
            </>
          ) : hasTimes ? (
            <ChartContainer config={config} className="aspect-auto h-52 w-full">
              <ComposedChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
                <CartesianGrid vertical={false} />
                <XAxis
                  dataKey="at"
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  minTickGap={32}
                  interval="preserveStartEnd"
                  tickFormatter={(v) => (range === "24h" ? time : day).format(new Date(String(v)))}
                />
                <YAxis
                  tickLine={false}
                  axisLine={false}
                  width={60}
                  tickFormatter={(v) => formatMs(Number(v), i18n.locale)}
                />
                <ChartTooltip
                  cursor={false}
                  content={
                    <ChartTooltipContent
                      labelFormatter={(_, p) => (p?.[0] ? long.format(new Date(String(p[0].payload.at))) : "")}
                      formatter={(value, name) => (
                        <div className="flex w-full items-center gap-2">
                          <span
                            className={
                              name === "p95"
                                ? "h-0 w-2.5 shrink-0 border-t-2 border-dashed border-muted-foreground"
                                : "size-2.5 shrink-0 rounded-[2px] bg-brand"
                            }
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
                {data
                  .filter((d) => d.failures > 0)
                  .map((d) => (
                    <ReferenceLine
                      key={d.at}
                      x={d.at}
                      stroke="var(--destructive)"
                      strokeOpacity={0.45}
                      strokeWidth={4}
                    />
                  ))}
                <Line
                  dataKey="p95"
                  type="monotone"
                  stroke="var(--color-p95)"
                  strokeDasharray="4 4"
                  strokeWidth={1.5}
                  dot={false}
                  isAnimationActive={false}
                />
                <Area
                  dataKey="p50"
                  type="monotone"
                  stroke="var(--color-p50)"
                  strokeWidth={2}
                  fill="var(--color-p50)"
                  fillOpacity={0.12}
                  isAnimationActive={false}
                />
              </ComposedChart>
            </ChartContainer>
          ) : (
            <p className="rounded-2xl bg-muted/50 px-4 py-6 text-center text-body text-muted-foreground">
              <Trans>No response times in this range yet.</Trans>
            </p>
          )}
          {timed && (
            <ul className="flex flex-wrap gap-x-4 gap-y-1 text-caption text-muted-foreground" aria-hidden>
              <li className="flex items-center gap-1.5">
                <span className="size-2.5 rounded-[2px] bg-brand" />
                <Trans>Median</Trans>
              </li>
              <li className="flex items-center gap-1.5">
                <span className="h-0 w-3 border-t-2 border-dashed border-muted-foreground" />
                <Trans>95th percentile</Trans>
              </li>
              <li className="flex items-center gap-1.5">
                <span className="h-3 w-1 rounded-[1px] bg-destructive/45" />
                <Trans>Failed checks</Trans>
              </li>
            </ul>
          )}
        </>
      ) : null}
    </section>
  )
}

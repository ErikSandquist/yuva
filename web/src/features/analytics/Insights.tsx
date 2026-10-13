import { Trans, useLingui } from "@lingui/react/macro"
import { ArrowDownRightIcon, ArrowUpRightIcon, LifeBuoyIcon, SparkleIcon } from "lucide-react"

import { Panel } from "@/features/analytics/parts"
import { useInsights } from "@/features/analytics/queries"
import { scopeOf, type View } from "@/features/analytics/state"
import { formatMetric, useAnalyticsText, valueText } from "@/features/analytics/text"
import type { AnalyticsDimension, AnalyticsInsight, AnalyticsProperty, AnalyticsQuery } from "@/lib/api"
import { cn } from "@/lib/utils"

const ORDER: AnalyticsInsight["kind"][] = ["metric_change", "support_hotspot", "row_change", "new_value"]

export const SUPPORT_FILTERS = ["page", "hostname"]

export function useInsightText() {
  const { t, i18n } = useLingui()
  const text = useAnalyticsText()
  const pct = (v: number) =>
    new Intl.NumberFormat(i18n.locale, { style: "percent", maximumFractionDigits: 0, signDisplay: "never" }).format(
      v / 100
    )
  return (x: AnalyticsInsight) => {
    const metric = text.metric[x.metric]
    const lower = metric.toLocaleLowerCase(i18n.locale)
    const now = formatMetric(x.metric, x.current, i18n.locale)
    const before = formatMetric(x.metric, x.previous ?? 0, i18n.locale)
    const change = pct(x.change ?? 0)
    const dimension = (x.dimension ?? "page") as AnalyticsDimension
    const dim = (text.dimension as Record<string, string>)[dimension] ?? dimension
    const kind = dim.toLocaleLowerCase(i18n.locale)
    const value = valueText(text, dimension, x.value ?? "", i18n.locale)
    const up = (x.change ?? 0) >= 0
    switch (x.kind) {
      case "metric_change":
        return up
          ? t`${metric} rose ${change}: ${now}, against ${before} in the compared days.`
          : t`${metric} fell ${change}: ${now}, against ${before} in the compared days.`
      case "row_change":
        return up
          ? t`${dim} ${value}: ${lower} rose ${change}, ${now} against ${before}.`
          : t`${dim} ${value}: ${lower} fell ${change}, ${now} against ${before}.`
      case "new_value":
        return t`New ${kind}: ${value}. ${metric}: ${now}, none in the compared days.`
      case "support_hotspot": {
        const rate = formatMetric("support_rate", x.current, i18n.locale)
        const site = formatMetric("support_rate", x.previous ?? 0, i18n.locale)
        return t`${dim} ${value}: ${rate} conversations per 1,000 views, against ${site} across all pages.`
      }
    }
  }
}

export function InsightList({ items, testId }: { items: AnalyticsInsight[]; testId?: string }) {
  const sentence = useInsightText()
  const sorted = [...items].sort((a, b) => ORDER.indexOf(a.kind) - ORDER.indexOf(b.kind)).slice(0, 6)
  return (
    <ul className="flex flex-col gap-1" data-testid={testId}>
      {sorted.map((x, i) => {
        const up = (x.change ?? 0) >= 0
        const Icon =
          x.kind === "support_hotspot"
            ? LifeBuoyIcon
            : x.kind === "new_value"
              ? SparkleIcon
              : up
                ? ArrowUpRightIcon
                : ArrowDownRightIcon
        return (
          <li
            key={`${x.kind}${x.metric}${x.value ?? ""}${i}`}
            className="flex items-start gap-2.5 rounded-md px-1 py-1.5 text-body"
            data-testid={`insight-${x.kind}`}
          >
            <Icon
              className={cn("mt-0.5 size-4 shrink-0", x.kind === "support_hotspot" ? "text-warning" : "text-faint")}
            />
            <span className="min-w-0 flex-1 break-words" dir="auto">
              {sentence(x)}
            </span>
          </li>
        )
      })}
    </ul>
  )
}

export function OverviewInsights({ property, view }: { property: AnalyticsProperty; view: View }) {
  const compare = view.compare ?? { with: "previous" as const }
  const scope = scopeOf(view)
  const base = { range: view.range, ...scope, compare, limit: 100 }
  const bodies: AnalyticsQuery[] = [
    { ...base, metrics: ["visitors", "visits", "pageviews"] },
    { ...base, metrics: ["visitors"], dimensions: ["source"] },
    { ...base, metrics: ["visitors"], dimensions: ["page"] },
  ]
  if (property.inbox_id && scope.filters.every((f) => SUPPORT_FILTERS.includes(f.dimension))) {
    bodies.push({
      dataset: "support",
      metrics: ["conversations", "support_rate", "pageviews"],
      dimensions: ["page"],
      range: view.range,
      filters: scope.filters,
      limit: 100,
    })
  }
  const q = useInsights(property.id, bodies)
  const items = q.data ?? []
  if (items.length === 0) return null
  return (
    <Panel
      title={
        <>
          <Trans>What changed</Trans>
          <span className="ms-2 text-caption font-normal text-faint">
            {view.compare ? <Trans>against the compared days</Trans> : <Trans>against the period before</Trans>}
          </span>
        </>
      }
      testId="analytics-insights"
    >
      <InsightList items={items} />
    </Panel>
  )
}

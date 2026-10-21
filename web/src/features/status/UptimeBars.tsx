import { useLingui } from "@lingui/react/macro"

import { formatDate } from "@/components/common/text"
import { useStatusText } from "@/features/status/text"
import type { StatusComponentStateValue } from "@/lib/api"
import type { components } from "@/lib/schema.gen"
import { cn } from "@/lib/utils"

type Day = components["schemas"]["PublicStatusDay"]

const tone: Record<StatusComponentStateValue, string> = {
  operational: "bg-success/55",
  degraded: "bg-warning",
  partial_outage: "bg-warning",
  major_outage: "bg-destructive",
  maintenance: "bg-brand/70",
}

export function usePercent() {
  const { i18n } = useLingui()
  const fmt = new Intl.NumberFormat(i18n.locale, { style: "percent", maximumFractionDigits: 2 })
  return (v: number) => fmt.format(v / 100)
}

function shift(date: string, by: number) {
  const d = new Date(`${date}T00:00:00Z`)
  d.setUTCDate(d.getUTCDate() + by)
  return d.toISOString().slice(0, 10)
}

export function UptimeBars({ days, count, className }: { days: Day[]; count: number; className?: string }) {
  const { i18n } = useLingui()
  const text = useStatusText()
  const percent = usePercent()
  const last = days[days.length - 1]?.date
  const known = new Map(days.map((d) => [d.date, d]))
  const slots: Day[] = last
    ? Array.from({ length: Math.max(count, days.length) }, (_, i) => {
        const date = shift(last, i - Math.max(count, days.length) + 1)
        return known.get(date) ?? { date, state: "operational" }
      })
    : []
  return (
    <span className={cn("flex h-6 gap-px", className)} data-testid="status-uptime-bars">
      {slots.map((d) => (
        <span
          key={d.date}
          className={cn("min-w-px flex-1 rounded-[2px]", d.uptime === undefined ? "bg-muted" : tone[d.state])}
          title={`${formatDate(d.date, i18n.locale)} · ${d.uptime === undefined ? (known.has(d.date) ? text.component[d.state] : "–") : percent(d.uptime)}`}
        />
      ))}
    </span>
  )
}

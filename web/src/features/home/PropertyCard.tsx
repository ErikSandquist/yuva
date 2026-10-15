import { Trans, useLingui } from "@lingui/react/macro"

import { Sparkline } from "@/components/common"
import type { HomeProperty } from "@/lib/api"
import { cn } from "@/lib/utils"

const EXAMPLE_DAYS = [18, 26, 21, 34, 29, 41, 23]

function where(p: HomeProperty) {
  const host = p.domains[0]
  if (host) return p.domains.length > 1 ? `${host} +${p.domains.length - 1}` : host
  return p.platforms.join(", ")
}

export function PropertyCard({
  property,
  example,
  name,
  host,
  className,
}: {
  property?: HomeProperty
  example?: boolean
  name?: string
  host?: string
  className?: string
}) {
  const { t, i18n } = useLingui()
  const fmt = new Intl.NumberFormat(i18n.locale)
  const short = new Intl.DateTimeFormat(i18n.locale, { weekday: "short", timeZone: "UTC" })
  const days = property?.days ?? []
  const values = example ? EXAMPLE_DAYS : days.map((d) => d.visitors)
  const today = fmt.format(example ? EXAMPLE_DAYS[6] : (property?.visitors_today ?? 0))
  const live = example ? 3 : (property?.realtime_visitors ?? 0)
  const liveText = fmt.format(live)
  const series = example
    ? values.map((v) => fmt.format(v)).join(", ")
    : days.map((d) => `${short.format(new Date(`${d.day}T00:00:00Z`))} ${fmt.format(d.visitors)}`).join(", ")
  return (
    <div
      className={cn(
        "flex w-full min-w-0 flex-col gap-4 rounded-2xl border bg-card p-4",
        example && "max-w-[22rem] shadow-[0_24px_60px_-28px_rgb(15_23_42/0.35)]",
        className
      )}
      aria-hidden={example || undefined}
    >
      <div className="min-w-0">
        <p className="truncate font-medium">{example ? name : property?.name}</p>
        <p className="truncate text-small text-faint" dir="ltr">
          {example ? host : property && where(property)}
        </p>
      </div>
      <div className="flex items-end justify-between gap-4">
        <div className="min-w-0">
          <b className="block text-page tabular-nums">{today}</b>
          <span className="text-small text-faint">
            <Trans>visitors today</Trans>
          </span>
        </div>
        <Sparkline values={values} label={t`Visitors in the last seven days: ${series}`} className="w-28 shrink-0" />
      </div>
      <p className="flex items-center gap-2 text-small text-muted-foreground">
        <span className={cn("size-2 shrink-0 rounded-full", live > 0 ? "bg-success" : "bg-border")} />
        <Trans>{liveText} in the last 30 minutes</Trans>
      </p>
    </div>
  )
}

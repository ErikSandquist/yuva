import { Plural, Trans, useLingui } from "@lingui/react/macro"

import { ErrorLine, Segmented } from "@/components/common"
import { DailyChart } from "@/components/common/DailyChart"
import type { ChartConfig } from "@/components/ui/chart"
import { Skeleton } from "@/components/ui/skeleton"
import { GROWTH_WINDOWS, useGrowth, useGrowthDays } from "@/features/broadcasts/queries"
import { Card, EmptyRow, Section } from "@/features/settings/ui"
import type { MailingList } from "@/lib/api"
import { cn } from "@/lib/utils"

export function Growth({ list }: { list: MailingList }) {
  const { t, i18n } = useLingui()
  const [days, setDays] = useGrowthDays()
  const growth = useGrowth(list.id, days)
  const fmt = new Intl.NumberFormat(i18n.locale)
  const config: ChartConfig = {
    new: { label: t`New`, theme: { light: "#0891b2", dark: "#22d3ee" } },
    confirmed: { label: t`Confirmed`, theme: { light: "#15803d", dark: "#0d9488" } },
    unsubscribed: { label: t`Unsubscribed`, theme: { light: "#a1a1aa", dark: "#71717a" } },
  }
  const g = growth.data
  const rows = (g?.days ?? []).map((d) => ({
    day: d.date,
    new: d.new,
    confirmed: d.confirmed,
    unsubscribed: d.unsubscribed,
  }))
  const sum = (k: "new" | "confirmed" | "unsubscribed") => rows.reduce((n, r) => n + r[k], 0)
  const rate =
    g?.confirmation_rate !== undefined
      ? new Intl.NumberFormat(i18n.locale, { style: "percent" }).format(g.confirmation_rate)
      : "–"
  const tiles: [string, string, string?][] = [
    [fmt.format(sum("new")), t`new`],
    [fmt.format(sum("confirmed")), t`confirmed`, "text-success"],
    [fmt.format(sum("unsubscribed")), t`unsubscribed`],
    [rate, t`confirmed their address`],
  ]
  return (
    <Section
      title={<Trans>Growth</Trans>}
      description={
        <Trans>
          New counts every subscription that day, confirmed or not. The confirmation rate is the share of people asked
          to confirm who did.
        </Trans>
      }
      action={
        <Segmented
          value={String(days)}
          onChange={(v) => setDays(Number(v) as (typeof GROWTH_WINDOWS)[number])}
          label={t`Days`}
          items={GROWTH_WINDOWS.map((d) => ({
            value: String(d),
            label: <Plural value={d} one="# day" other="# days" />,
            testId: `growth-days-${d}`,
          }))}
        />
      }
    >
      {growth.isPending ? (
        <Skeleton className="h-64 rounded-2xl" />
      ) : !g ? (
        <ErrorLine error={growth.error} />
      ) : (
        <>
          <div className="grid grid-cols-4 gap-2 phone:grid-cols-2" data-testid="growth-tiles">
            {tiles.map(([value, label, tone]) => (
              <div key={label} className="rounded-2xl border bg-card px-4 py-3.5">
                <b className={cn("block text-page tabular-nums", tone)}>{value}</b>
                <span className="text-small text-faint">{label}</span>
              </div>
            ))}
          </div>
          {rows.some((r) => r.new + r.confirmed + r.unsubscribed > 0) ? (
            <Card>
              <DailyChart rows={rows} config={config} testId="growth-chart" />
            </Card>
          ) : (
            <Card flush>
              <EmptyRow>
                <Trans>Nobody joined or left in this window.</Trans>
              </EmptyRow>
            </Card>
          )}
        </>
      )}
    </Section>
  )
}

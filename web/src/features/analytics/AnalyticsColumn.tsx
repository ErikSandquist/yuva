import "@/features/analytics/i18n"

import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { ChartLineIcon, GaugeIcon, PlusIcon } from "lucide-react"
import { Link } from "react-router"

import { Dot, ErrorLine, Notice } from "@/components/common"
import { Column, ColumnLink } from "@/components/common/Column"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useAnalyticsProperties } from "@/features/analytics/properties"
import { useViewSearch } from "@/features/analytics/state"
import { formatCompact } from "@/features/analytics/text"
import { useNow } from "@/hooks/use-now"
import type { AnalyticsProperty, AnalyticsUsage } from "@/lib/api"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

const DAY = 86_400_000

export function UsageBanner({ usage, className }: { usage: AnalyticsUsage; className?: string }) {
  const { i18n } = useLingui()
  if (!usage.limit) return null
  const fmt = new Intl.NumberFormat(i18n.locale)
  const used = fmt.format(usage.events)
  const limit = fmt.format(usage.limit)
  if (usage.limit_reached_at) {
    return (
      <Notice tone="danger" icon={GaugeIcon} className={className} data-testid="analytics-cap-reached">
        <Trans>
          This workspace reached its {limit} analytics events for this month. New events are dropped until the month
          ends.
        </Trans>
      </Notice>
    )
  }
  if (usage.events < usage.limit * 0.8) return null
  return (
    <Notice tone="warning" icon={GaugeIcon} className={className} data-testid="analytics-cap-near">
      <Trans>
        {used} of {limit} analytics events used this month. Events over the limit are dropped until the month ends.
      </Trans>
    </Notice>
  )
}

function Row({ p, now, inboxColor }: { p: AnalyticsProperty; now: number; inboxColor?: string }) {
  const { t, i18n } = useLingui()
  const search = useViewSearch()
  const quiet = !p.last_event_at || now - Date.parse(p.last_event_at) > DAY
  const visitors = formatCompact(p.visitors_today, i18n.locale)
  return (
    <li>
      <ColumnLink to={`/analytics/${p.id}${search}`} testId="analytics-property">
        <span className="grid size-[34px] shrink-0 place-items-center rounded-[10px] border bg-surface text-muted-foreground">
          <ChartLineIcon className="size-4" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="flex items-center gap-2">
            {inboxColor && <Dot color={inboxColor} className="size-[7px] shrink-0 rounded-[2px]" />}
            <span className="truncate text-body font-medium">{p.name}</span>
            {quiet && (
              <span
                className="size-1.5 shrink-0 rounded-full bg-warning"
                title={p.first_event_at ? t`No events in the last 24 hours` : t`No events yet`}
                data-testid="analytics-quiet"
              />
            )}
          </span>
          <span className="block truncate text-caption text-faint" dir="ltr">
            {p.domains.join(", ")}
          </span>
        </span>
        <span className="flex shrink-0 flex-col items-end">
          <span className="text-body font-medium tabular-nums">{visitors}</span>
          <span className="text-caption text-faint">
            <Plural value={p.visitors_today} one="visitor today" other="visitors today" />
          </span>
        </span>
      </ColumnLink>
    </li>
  )
}

export default function AnalyticsColumn() {
  const { t } = useLingui()
  const { canManage } = useSession()
  const list = useAnalyticsProperties()
  const inboxes = useInboxes().data ?? []
  const colors = new Map(inboxes.map((i) => [i.id, i.branding.color]))
  const now = useNow(60_000)
  const items = list.data?.items ?? []
  return (
    <Column
      title={<Trans>Analytics</Trans>}
      testId="analytics-panel"
      bodyClassName="px-2"
      actions={
        canManage && (
          <Button variant="ghost" size="sm" render={<Link to="/analytics/new" />} data-testid="analytics-add">
            <PlusIcon />
            <Trans>Add</Trans>
          </Button>
        )
      }
    >
      {list.data && <UsageBanner usage={list.data.usage} className="mx-1 mb-2" />}
      {list.isPending ? (
        <div className="flex flex-col gap-2 p-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : list.error ? (
        <ErrorLine error={list.error} className="p-3" />
      ) : items.length === 0 ? (
        <div className="px-4 py-10 text-center text-body text-faint" data-testid="analytics-empty">
          <p className="mb-2 font-medium text-foreground">
            <Trans>No properties yet</Trans>
          </p>
          {canManage ? (
            <p>
              <Trans>Add a website to count its visitors, pages and sources, without cookies.</Trans>
            </p>
          ) : (
            <p>
              <Trans>The properties of your inboxes show up here once an owner or admin adds them.</Trans>
            </p>
          )}
        </div>
      ) : (
        <nav aria-label={t`Analytics properties`}>
          <ul className="flex flex-col">
            {items.map((p) => (
              <Row
                key={p.id}
                p={p}
                now={now}
                inboxColor={inboxes.length > 1 && p.inbox_id ? colors.get(p.inbox_id) : undefined}
              />
            ))}
          </ul>
        </nav>
      )}
    </Column>
  )
}

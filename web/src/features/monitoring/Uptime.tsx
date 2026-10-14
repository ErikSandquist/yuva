import { Trans, useLingui } from "@lingui/react/macro"
import { PlusIcon } from "lucide-react"
import { Link } from "react-router"

import { ErrorLine } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useAnalytics } from "@/features/analytics/queries"
import { formatMetric, num } from "@/features/analytics/text"
import { StateDot, StateText } from "@/features/monitoring/CheckPage"
import { useChecks } from "@/features/monitoring/queries"
import { formatInterval } from "@/features/monitoring/text"
import { Card, EmptyRow, Rows, Section } from "@/features/settings/ui"
import { useSession } from "@/lib/session"

export function PropertyUptime({ propertyId }: { propertyId: string }) {
  const { i18n } = useLingui()
  const { canManage } = useSession()
  const checks = useChecks(propertyId)
  const items = checks.data?.items ?? []
  const q = useAnalytics(
    propertyId,
    {
      dataset: "uptime",
      metrics: ["uptime", "response_time_p95"],
      dimensions: ["check"],
      range: { period: "7d" },
    },
    items.length > 0
  )
  const byCheck = new Map((q.data?.rows ?? []).map((r) => [r.dimensions[0] ?? "", r.metrics]))
  return (
    <div data-testid="property-uptime">
      <Section
        title={<Trans>Uptime checks</Trans>}
        description={<Trans>The checks linked to this property, with their last 7 days.</Trans>}
      >
        <Card flush>
          {checks.isPending ? (
            <Skeleton className="m-4 h-10" />
          ) : checks.error ? (
            <ErrorLine error={checks.error} className="p-4" />
          ) : items.length === 0 ? (
            <EmptyRow>
              <Trans>No uptime checks are linked to this property yet.</Trans>
            </EmptyRow>
          ) : (
            <Rows>
              {items.map((c) => {
                const m = byCheck.get(c.id)
                const every = formatInterval(c.interval_seconds, i18n.locale)
                return (
                  <li key={c.id}>
                    <Link
                      to={`/monitoring/checks/${c.id}`}
                      className="flex items-center gap-3 px-4 py-3 outline-none transition-colors hover:bg-muted/50 focus-visible:bg-muted"
                      data-testid="property-uptime-check"
                    >
                      <StateDot state={c.state} paused={c.paused} />
                      <span className="flex min-w-0 flex-1 flex-col">
                        <span className="truncate text-body font-medium">{c.name}</span>
                        <span className="truncate text-caption text-faint" dir="ltr">
                          {c.method} {c.url}
                        </span>
                      </span>
                      <span className="hidden flex-col items-end text-small sm:flex">
                        <span className="tabular-nums">{formatMetric("uptime", num(m, "uptime"), i18n.locale)}</span>
                        <span className="text-caption text-faint tabular-nums">
                          p95 {formatMetric("response_time_p95", num(m, "response_time_p95"), i18n.locale)}
                        </span>
                      </span>
                      <span className="flex w-24 shrink-0 flex-col items-end">
                        <span className="text-small font-medium">
                          <StateText check={c} />
                        </span>
                        <span className="text-caption text-faint">{every}</span>
                      </span>
                    </Link>
                  </li>
                )
              })}
            </Rows>
          )}
          {canManage && (
            <div className="border-t p-2">
              <Button
                variant="ghost"
                size="sm"
                render={<Link to={`/monitoring/checks/new?property=${propertyId}`} />}
                data-testid="property-uptime-add"
              >
                <PlusIcon />
                <Trans>Add an uptime check</Trans>
              </Button>
            </div>
          )}
        </Card>
      </Section>
    </div>
  )
}

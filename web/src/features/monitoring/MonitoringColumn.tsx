import "@/features/analytics/i18n"

import { Trans, useLingui } from "@lingui/react/macro"
import { PlusIcon } from "lucide-react"
import { Link } from "react-router"

import { ErrorLine } from "@/components/common"
import { Column, ColumnHeading, ColumnLink } from "@/components/common/Column"
import { formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { StateDot, StateText } from "@/features/monitoring/CheckPage"
import { useChecks } from "@/features/monitoring/queries"
import { formatMs } from "@/features/monitoring/text"
import type { MonitorCheck } from "@/lib/api"
import { useSession } from "@/lib/session"

function Row({ c }: { c: MonitorCheck }) {
  const { i18n } = useLingui()
  const last = c.last_result
  return (
    <li>
      <ColumnLink to={`/monitoring/checks/${c.id}`} testId="monitoring-check">
        <span className="grid size-[34px] shrink-0 place-items-center rounded-[10px] border bg-surface">
          <StateDot state={c.state} paused={c.paused} />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-body font-medium">{c.name}</span>
          <span className="block truncate text-caption text-faint" dir="ltr">
            {c.url.replace(/^https?:\/\//, "")}
          </span>
        </span>
        <span className="flex shrink-0 flex-col items-end">
          <span className="text-small font-medium">
            <StateText check={c} />
          </span>
          {last && (
            <span className="text-caption text-faint tabular-nums" title={formatDateTime(last.at, i18n.locale)}>
              {formatMs(last.timings.total, i18n.locale)}
            </span>
          )}
        </span>
      </ColumnLink>
    </li>
  )
}

export default function MonitoringColumn() {
  const { t } = useLingui()
  const { canManage } = useSession()
  const list = useChecks()
  const items = list.data?.items ?? []
  const down = items.filter((c) => c.state === "down" && !c.paused)
  const rest = items.filter((c) => !(c.state === "down" && !c.paused))
  return (
    <Column
      title={<Trans>Monitoring</Trans>}
      testId="monitoring-panel"
      bodyClassName="px-2"
      actions={
        canManage && (
          <Button variant="ghost" size="sm" render={<Link to="/monitoring/checks/new" />} data-testid="monitoring-add">
            <PlusIcon />
            <Trans>Add</Trans>
          </Button>
        )
      }
    >
      {list.isPending ? (
        <div className="flex flex-col gap-2 p-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : list.error ? (
        <ErrorLine error={list.error} className="p-3" />
      ) : items.length === 0 ? (
        <div className="px-4 py-10 text-center text-body text-faint" data-testid="monitoring-empty">
          <p className="mb-2 font-medium text-foreground">
            <Trans>No uptime checks yet</Trans>
          </p>
          {canManage ? (
            <p>
              <Trans>Add a check to hear when an address stops answering, and when it is back.</Trans>
            </p>
          ) : (
            <p>
              <Trans>Checks show up here once an owner or admin adds them.</Trans>
            </p>
          )}
        </div>
      ) : (
        <nav aria-label={t`Uptime checks`}>
          {down.length > 0 && (
            <>
              <ColumnHeading>
                <Trans>Down</Trans>
              </ColumnHeading>
              <ul className="flex flex-col">
                {down.map((c) => (
                  <Row key={c.id} c={c} />
                ))}
              </ul>
            </>
          )}
          <ColumnHeading>
            <Trans>Uptime checks</Trans>
          </ColumnHeading>
          <ul className="flex flex-col">
            {rest.map((c) => (
              <Row key={c.id} c={c} />
            ))}
          </ul>
        </nav>
      )}
    </Column>
  )
}

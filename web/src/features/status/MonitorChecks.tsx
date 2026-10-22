import { Trans, useLingui } from "@lingui/react/macro"

import { ErrorLine } from "@/components/common"
import { formatDateTime, formatMs } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useMonitorChecks } from "@/features/status/queries"
import { useStatusText } from "@/features/status/text"
import type { StatusMonitor } from "@/lib/api"
import { cn } from "@/lib/utils"

export function MonitorChecks({ monitor }: { monitor: StatusMonitor }) {
  const { i18n } = useLingui()
  const text = useStatusText()
  const outside = monitor.kind === "http" || monitor.kind === "tcp"
  const list = useMonitorChecks(monitor.page_id, monitor.id)
  const items = list.data?.pages.flatMap((p) => p.items) ?? []
  return (
    <section className="flex flex-col gap-2" data-testid="status-monitor-checks">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 className="text-body font-medium">
          <Trans>Recent checks</Trans>
        </h3>
        <span className="text-caption text-faint">
          {outside ? (
            <Trans>Every check of the last 30 days, newest first.</Trans>
          ) : (
            <Trans>Every beat, missed beat and pushed state of the last 30 days, newest first.</Trans>
          )}
        </span>
      </div>
      <ErrorLine error={list.error} />
      {list.isPending ? (
        <Skeleton className="h-32 w-full rounded-xl" />
      ) : (
        <div className="overflow-hidden rounded-xl border bg-card">
          {items.length === 0 ? (
            <p className="px-4 py-5 text-body text-faint">
              <Trans>Nothing has arrived yet.</Trans>
            </p>
          ) : (
            <ul className="divide-y">
              {items.map((c) => {
                const detail = c.message || c.error
                return (
                  <li key={c.id} className="flex gap-3 px-4 py-2.5" data-testid="status-check">
                    <span
                      aria-hidden
                      className={cn("mt-2 size-2 shrink-0 rounded-full", c.ok ? "bg-success" : "bg-destructive")}
                    />
                    <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                      <span className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5">
                        <span className="flex-1 basis-32 text-body font-medium">
                          <span className={c.ok ? "text-success" : "text-destructive"}>
                            {c.ok ? <Trans>OK</Trans> : <Trans>Failed</Trans>}
                          </span>
                          <span className="text-muted-foreground"> · {text.checkSource[c.source]}</span>
                          {c.state && !c.ok && (
                            <span className="text-muted-foreground"> · {text.component[c.state]}</span>
                          )}
                        </span>
                        {c.duration_ms !== undefined && (
                          <span className="shrink-0 font-mono text-caption text-muted-foreground tabular-nums">
                            {c.status_code ? `${c.status_code} · ` : ""}
                            {formatMs(c.duration_ms, i18n.locale)}
                          </span>
                        )}
                        <span className="shrink-0 text-caption text-faint">{formatDateTime(c.at, i18n.locale)}</span>
                      </span>
                      {detail && <span className="text-small break-words text-muted-foreground">{detail}</span>}
                    </span>
                  </li>
                )
              })}
            </ul>
          )}
        </div>
      )}
      {list.hasNextPage && (
        <div className="flex justify-center">
          <Button
            variant="ghost"
            size="sm"
            disabled={list.isFetchingNextPage}
            onClick={() => void list.fetchNextPage()}
            data-testid="status-checks-more"
          >
            <Trans>Load more</Trans>
          </Button>
        </div>
      )}
    </section>
  )
}

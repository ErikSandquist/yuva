import { Trans, useLingui } from "@lingui/react/macro"

import { ErrorLine } from "@/components/common"
import { formatDateTime, formatMs } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { Card, EmptyRow, Rows } from "@/features/settings/ui"
import { useMonitorChecks } from "@/features/status/queries"
import { ResponseChart } from "@/features/status/ResponseChart"
import { useStatusText } from "@/features/status/text"
import type { StatusMonitor } from "@/lib/api"

export function MonitorChecksDialog({
  pageId,
  monitor,
  onClose,
}: {
  pageId: string
  monitor: StatusMonitor
  onClose: () => void
}) {
  const { i18n } = useLingui()
  const text = useStatusText()
  const outside = monitor.kind === "http" || monitor.kind === "tcp"
  const list = useMonitorChecks(pageId, monitor.id)
  const items = list.data?.pages.flatMap((p) => p.items) ?? []
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-2xl" data-testid="status-monitor-checks">
        <DialogHeader>
          <DialogTitle>{monitor.name}</DialogTitle>
          <DialogDescription>
            {outside ? (
              <Trans>Every check of the last 30 days, newest first.</Trans>
            ) : (
              <Trans>Every beat, missed beat and pushed state of the last 30 days, newest first.</Trans>
            )}
          </DialogDescription>
        </DialogHeader>
        {outside && <ResponseChart pageId={pageId} monitorId={monitor.id} />}
        <ErrorLine error={list.error} />
        {list.isPending ? (
          <Skeleton className="h-32 w-full rounded-2xl" />
        ) : (
          <Card flush>
            {items.length === 0 ? (
              <EmptyRow>
                <Trans>Nothing has arrived yet.</Trans>
              </EmptyRow>
            ) : (
              <Rows>
                {items.map((c) => {
                  const detail = c.message || c.error
                  return (
                    <li key={c.id} className="flex flex-col gap-0.5 px-4 py-2.5" data-testid="status-check">
                      <span className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5">
                        <span className="flex-1 basis-40 text-body font-medium">
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
                    </li>
                  )
                })}
              </Rows>
            )}
          </Card>
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
      </DialogContent>
    </Dialog>
  )
}

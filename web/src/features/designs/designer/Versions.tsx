import { Trans, useLingui } from "@lingui/react/macro"
import { HistoryIcon, LoaderIcon, RotateCcwIcon } from "lucide-react"
import { useState } from "react"

import { ErrorLine } from "@/components/common"
import { formatDateTime, formatRelative } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { MailFrame } from "@/features/designs/designer/Preview"
import { useRender, useVersionDetail, useVersions } from "@/features/designs/queries"
import type { EmailDesignVersion } from "@/lib/api"
import { cn } from "@/lib/utils"
import { useMemberMap } from "@/lib/workspace"

function useReasonText() {
  const { t } = useLingui()
  return (r: EmailDesignVersion["reason"]) =>
    ({
      autosave: t`Saved while editing`,
      manual: t`Saved by hand`,
      test: t`Sent as a test`,
      scheduled: t`Scheduled to send`,
      restored: t`Restored`,
    })[r]
}

export function Versions({
  open,
  onOpenChange,
  designId,
  inboxId,
  current,
  onRestore,
  restoring,
  error,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  designId: string
  inboxId?: string
  current: number
  onRestore: (version: number) => void
  restoring: boolean
  error: unknown
}) {
  const { t, i18n } = useLingui()
  const versions = useVersions(designId, open)
  const members = useMemberMap()
  const reason = useReasonText()
  const [chosen, setChosen] = useState<number | null>(null)
  const detail = useVersionDetail(designId, open ? chosen : null)
  const render = useRender(detail.data ? { document: detail.data.document, inbox_id: inboxId } : null)
  const items = versions.data ?? []
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-[min(56rem,100vw)] max-w-none" data-testid="versions">
        <SheetHeader>
          <SheetTitle className="text-title">
            <Trans>Versions</Trans>
          </SheetTitle>
          <SheetDescription>
            <Trans>
              A version is kept at most every ten minutes while you edit, when you save by hand and when you restore
              one. The newest 50 stay.
            </Trans>
          </SheetDescription>
        </SheetHeader>
        <div className="flex min-h-0 flex-1 phone:flex-col">
          <ul
            className="flex w-72 shrink-0 flex-col gap-0.5 overflow-y-auto border-e p-2 phone:max-h-56 phone:w-full phone:border-e-0 phone:border-b"
            data-testid="version-list"
          >
            {versions.isPending ? (
              Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="m-1 h-12" />)
            ) : versions.error ? (
              <ErrorLine error={versions.error} className="p-2" />
            ) : items.length === 0 ? (
              <li className="px-3 py-6 text-center text-small text-faint">
                <Trans>No versions yet. One is kept after ten minutes of editing.</Trans>
              </li>
            ) : (
              items.map((v) => {
                const who = v.member_id ? members.get(v.member_id)?.name : undefined
                const when = formatRelative(v.created_at, i18n.locale)
                return (
                  <li key={v.version}>
                    <button
                      type="button"
                      onClick={() => setChosen(v.version)}
                      className={cn(
                        "flex w-full flex-col gap-0.5 rounded-xl px-3 py-2 text-start transition-colors hover:bg-muted",
                        chosen === v.version && "bg-muted"
                      )}
                      title={formatDateTime(v.created_at, i18n.locale)}
                      data-testid="version-row"
                    >
                      <span className="flex items-center gap-2 text-body font-medium">
                        <span className="min-w-0 flex-1 truncate">{when}</span>
                        {v.version === current && (
                          <span className="rounded-full bg-brand-wash px-2 py-px text-caption text-brand">
                            <Trans>Current</Trans>
                          </span>
                        )}
                      </span>
                      <span className="truncate text-caption text-faint">
                        {who ? `${reason(v.reason)} · ${who}` : reason(v.reason)}
                      </span>
                    </button>
                  </li>
                )
              })
            )}
          </ul>
          <div className="flex min-h-0 min-w-0 flex-1 flex-col bg-surface">
            {chosen === null ? (
              <div className="flex flex-1 flex-col items-center justify-center gap-2 p-6 text-center text-body text-faint">
                <HistoryIcon className="size-6" />
                <Trans>Pick a version to see it.</Trans>
              </div>
            ) : (
              <>
                <div className="flex items-center gap-3 border-b bg-card px-4 py-3">
                  <p className="min-w-0 flex-1 text-small text-muted-foreground">
                    <Trans>Restoring saves this version as the newest one. You can undo it.</Trans>
                  </p>
                  <Button
                    size="sm"
                    disabled={!detail.data || restoring}
                    onClick={() => onRestore(chosen)}
                    data-testid="version-restore"
                  >
                    {restoring ? <LoaderIcon className="animate-spin" /> : <RotateCcwIcon />}
                    <Trans>Restore</Trans>
                  </Button>
                </div>
                <ErrorLine error={error} className="px-4 pt-3" />
                <div className="min-h-0 flex-1 overflow-y-auto p-4">
                  {render.data ? (
                    <div className="overflow-hidden rounded-xl border bg-white">
                      <MailFrame html={render.data.html} title={t`Preview`} />
                    </div>
                  ) : render.error || detail.error ? (
                    <ErrorLine error={render.error ?? detail.error} />
                  ) : (
                    <Skeleton className="h-96 w-full rounded-xl" />
                  )}
                </div>
              </>
            )}
          </div>
        </div>
      </SheetContent>
    </Sheet>
  )
}

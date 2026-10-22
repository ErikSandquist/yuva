import { Trans, useLingui } from "@lingui/react/macro"
import { HistoryIcon, LoaderIcon, RotateCcwIcon } from "lucide-react"
import { useState } from "react"

import { ErrorLine } from "@/components/common"
import { formatDateTime, formatRelative } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { MailFrame } from "@/components/common/MailFrame"
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
      conflict: t`Kept from a conflict`,
    })[r]
}

type PanelProps = {
  designId: string
  inboxId?: string
  current: number
  onRestore: (version: number) => void
  restoring: boolean
  error: unknown
  scale?: number
}

export function VersionsPanel({ designId, inboxId, current, onRestore, restoring, error, scale = 0.4 }: PanelProps) {
  const { t, i18n } = useLingui()
  const versions = useVersions(designId, true)
  const members = useMemberMap()
  const reason = useReasonText()
  const [chosen, setChosen] = useState<number | null>(null)
  const detail = useVersionDetail(designId, chosen)
  const render = useRender(detail.data ? { document: detail.data.document, inbox_id: inboxId } : null)
  const items = versions.data ?? []
  return (
    <div className="flex flex-col gap-4 p-4" data-testid="versions">
      <p className="text-caption text-faint">
        <Trans>
          A version is kept at most every ten minutes while you edit, when you save by hand, when you restore one and
          when someone else's save wins a conflict. The newest 50 stay.
        </Trans>
      </p>
      <ul className="flex flex-col gap-0.5" data-testid="version-list">
        {versions.isPending ? (
          Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="my-1 h-12" />)
        ) : versions.error ? (
          <ErrorLine error={versions.error} />
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
                  aria-pressed={chosen === v.version}
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
      {chosen === null ? (
        items.length > 0 && (
          <p className="flex items-center justify-center gap-2 rounded-xl bg-surface px-3 py-6 text-center text-small text-faint">
            <HistoryIcon className="size-4 shrink-0" />
            <Trans>Pick a version to see it.</Trans>
          </p>
        )
      ) : (
        <div className="flex flex-col gap-3">
          <p className="text-small text-muted-foreground">
            <Trans>Restoring saves this version as the newest one. You can undo it.</Trans>
          </p>
          <Button disabled={!detail.data || restoring} onClick={() => onRestore(chosen)} data-testid="version-restore">
            {restoring ? <LoaderIcon className="animate-spin" /> : <RotateCcwIcon />}
            <Trans>Restore</Trans>
          </Button>
          <ErrorLine error={error} />
          {render.data ? (
            <div
              className="relative mx-auto h-80 overflow-hidden rounded-xl border bg-white"
              style={{ width: 600 * scale }}
            >
              <div
                className="pointer-events-none absolute top-0 start-0 origin-top-left rtl:origin-top-right"
                style={{ width: 600, transform: `scale(${scale})` }}
                aria-hidden
              >
                <MailFrame html={render.data.html} width={600} title={t`Preview`} />
              </div>
            </div>
          ) : render.error || detail.error ? (
            <ErrorLine error={render.error ?? detail.error} />
          ) : (
            <Skeleton className="h-80 w-full rounded-xl" />
          )}
        </div>
      )}
    </div>
  )
}

export function Versions({
  open,
  onOpenChange,
  ...panel
}: PanelProps & { open: boolean; onOpenChange: (open: boolean) => void }) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full" data-testid="versions-sheet">
        <SheetHeader>
          <SheetTitle className="text-title">
            <Trans>Versions</Trans>
          </SheetTitle>
        </SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto">{open && <VersionsPanel {...panel} scale={0.55} />}</div>
      </SheetContent>
    </Sheet>
  )
}

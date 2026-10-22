import { Trans, useLingui } from "@lingui/react/macro"
import { MailWarningIcon } from "lucide-react"
import { useState } from "react"
import { Link, useNavigate, useParams, useSearchParams } from "react-router"

import { ErrorLine, InboxMark, Notice, toast } from "@/components/common"
import { PageBar } from "@/components/common/PageBar"
import { LIST_KEYS } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { keyHint } from "@/features/broadcasts/BroadcastParts"
import { FormSnippets } from "@/features/broadcasts/FormSnippets"
import { Growth } from "@/features/broadcasts/Growth"
import { ListImports } from "@/features/broadcasts/Import"
import { ListExport, ListSettings } from "@/features/broadcasts/ListSettings"
import { listLink, useArchiveList, useList, useSender } from "@/features/broadcasts/queries"
import { Subscribers } from "@/features/broadcasts/Subscribers"
import { Chip } from "@/features/settings/ui"
import { FrameButton, FrameTabs, framePad } from "@/features/status/PageFrame"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { MailingList } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

const TABS = ["subscribers", "import", "growth", "form", "settings"] as const
type Tab = (typeof TABS)[number]

export { ListsOverview as ListsHome } from "@/features/broadcasts/ListsHome"

export function Tiles({ list }: { list: MailingList }) {
  const { t, i18n } = useLingui()
  const fmt = new Intl.NumberFormat(i18n.locale)
  const tiles: [string, string, string?][] = [
    [fmt.format(list.counts.subscribed), t`subscribed now`, "text-success"],
    [fmt.format(list.counts.pending), t`waiting for confirmation`, list.counts.pending ? "text-warning" : undefined],
    [fmt.format(list.counts.unsubscribed), t`unsubscribed now`],
  ]
  return (
    <div className="grid grid-cols-3 gap-2" data-testid="list-tiles">
      {tiles.map(([value, label, tone]) => (
        <div key={label} className="min-w-0 rounded-2xl border bg-card px-4 py-3.5 phone:px-3">
          <b className={cn("block text-page tabular-nums", tone)}>{value}</b>
          <span className="block text-small break-words text-faint">{label}</span>
        </div>
      ))}
    </div>
  )
}

function ExportDialog({ list, onClose }: { list: MailingList; onClose: () => void }) {
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-xl" data-testid="list-export-dialog">
        <DialogHeader>
          <DialogTitle className="sr-only">
            <Trans>Consent export</Trans>
          </DialogTitle>
        </DialogHeader>
        <ListExport list={list} />
      </DialogContent>
    </Dialog>
  )
}

export function ListPage() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { listId = "" } = useParams()
  const [params] = useSearchParams()
  const list = useList(listId)
  const { canManage } = useSession()
  const archive = useArchiveList(listId)
  const [exporting, setExporting] = useState(false)
  const l = list.data
  const inbox = (useInboxes().data ?? []).find((i) => i.id === l?.inbox_id)
  const from = useSender(l?.inbox_id ?? "", l?.channel_id)
  const raw = params.get("tab")
  const tab: Tab = TABS.includes(raw as Tab) ? (raw as Tab) : "subscribers"
  const go = (v: Tab) => navigate(listLink(listId, v === "subscribers" ? undefined : v), { replace: true })
  useHotkeys(
    {
      [LIST_KEYS.subscribers]: () => go("subscribers"),
      [LIST_KEYS.importTab]: () => go("import"),
      [LIST_KEYS.growth]: () => go("growth"),
      [LIST_KEYS.form]: () => go("form"),
      [LIST_KEYS.settings]: () => go("settings"),
      ...(canManage ? { [LIST_KEYS.export]: () => setExporting(true) } : {}),
    },
    !!l && !exporting
  )
  const names: Record<Tab, string> = {
    subscribers: t`Subscribers`,
    import: t`Import`,
    growth: t`Growth`,
    form: t`Form`,
    settings: t`Settings`,
  }
  if (!l) {
    return (
      <div className="flex min-h-0 min-w-0 flex-1 flex-col bg-background">
        <PageBar back="/broadcasts/lists" label={t`Lists`}>
          {list.isPending ? <Skeleton className="h-7 w-60" /> : <ErrorLine error={list.error} />}
        </PageBar>
        <div className={framePad}>{list.isPending && <Skeleton className="h-24 w-full rounded-2xl" />}</div>
      </div>
    )
  }
  const archived = !!l.archived_at
  const tabs = TABS.map((v) => ({
    key: v,
    to: listLink(l.id, v === "subscribers" ? undefined : v),
    label: names[v],
    testId: `list-tab-${v}`,
  }))
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col bg-background" data-testid="list-page" key={l.id}>
      <PageBar back="/broadcasts/lists" label={t`Lists`}>
        {inbox && <InboxMark name={inbox.name} color={inbox.branding.color} className="size-7 text-small" />}
        <div className="flex min-w-48 flex-1 basis-64 flex-col leading-snug">
          <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
            <h1 className="min-w-0 truncate text-[17px] font-semibold" data-testid="list-name-heading">
              {l.name}
            </h1>
            <Chip>{l.public ? <Trans>Public</Trans> : <Trans>Private</Trans>}</Chip>
            <Chip>{l.double_opt_in ? <Trans>Confirmation asked</Trans> : <Trans>No confirmation</Trans>}</Chip>
            {archived && (
              <Chip>
                <Trans>Archived</Trans>
              </Chip>
            )}
          </span>
          <span className="truncate text-small text-muted-foreground">
            {l.sender_ready && from ? (
              <Trans>From {from}</Trans>
            ) : (
              <Link to={listLink(l.id, "settings")} className="text-warning underline-offset-4 hover:underline">
                <Trans>Pick a sender in the settings</Trans>
              </Link>
            )}
            {l.description && ` · ${l.description}`}
          </span>
        </div>
        {canManage && (
          <FrameButton
            onClick={() => setExporting(true)}
            title={keyHint(t`Export`, LIST_KEYS.export)}
            data-testid="list-export-open"
          >
            <Trans>Export</Trans>
          </FrameButton>
        )}
      </PageBar>
      <div className="@container min-h-0 flex-1 in-data-frame:overflow-y-auto">
        <div className={cn(framePad, "max-w-[1200px]")}>
          {archived && (
            <Notice icon={MailWarningIcon} data-testid="list-archived-banner">
              <div className="flex flex-wrap items-center gap-3">
                <p className="min-w-48 flex-1">
                  <Trans>This list is archived. It takes no new subscribers and broadcasts cannot pick it.</Trans>
                </p>
                {canManage && (
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={archive.isPending}
                    onClick={() => archive.mutate(false, { onSuccess: () => toast(t`List restored`) })}
                    data-testid="list-restore"
                  >
                    <Trans>Restore</Trans>
                  </Button>
                )}
              </div>
              <ErrorLine error={archive.error} />
            </Notice>
          )}
          {!l.sender_ready && !archived && (
            <Notice tone="warning" icon={MailWarningIcon} data-testid="list-needs-sender">
              <p>
                <Trans>
                  This list has no e-mail channel that can send, so forms send no confirmation, new form subscribers
                  keep waiting and nobody gets a welcome mail.
                </Trans>
              </p>
              {tab !== "settings" && (
                <Link to={listLink(l.id, "settings")} className="w-fit text-brand underline-offset-4 hover:underline">
                  <Trans>Pick a sender in the settings</Trans>
                </Link>
              )}
            </Notice>
          )}
          <Tiles list={l} />
          <div className="border-b">
            <FrameTabs label={t`List`} items={tabs} current={tab} />
          </div>
          {tab === "subscribers" && <Subscribers list={l} />}
          {tab === "import" && <ListImports list={l} />}
          {tab === "growth" && <Growth list={l} />}
          {tab === "form" && <FormSnippets list={l} />}
          {tab === "settings" && <ListSettings list={l} />}
        </div>
      </div>
      {exporting && <ExportDialog list={l} onClose={() => setExporting(false)} />}
    </div>
  )
}

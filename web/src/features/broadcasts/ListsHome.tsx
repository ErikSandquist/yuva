import { Trans, useLingui } from "@lingui/react/macro"
import { useState } from "react"
import { Link, useLocation, useNavigate, useSearchParams } from "react-router"

import { ErrorLine, InboxMark, toast } from "@/components/common"
import { LIST_KEYS } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { keyHint, useNumber } from "@/features/broadcasts/BroadcastParts"
import { ConsentExplainer, NeedsSender } from "@/features/broadcasts/ListParts"
import { listLink, useArchiveList, useGrowth, useLists, useSender } from "@/features/broadcasts/queries"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { FrameTabs, look, PageTop } from "@/features/inbox/PageFrame"
import type { SetupState } from "@/features/setup/setup"
import { Chip } from "@/features/settings/ui"
import { framePad } from "@/features/status/PageFrame"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useRowFocus } from "@/hooks/use-row-focus"
import type { MailingList } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

const VIEWS = ["lists", "archived"] as const
type View = (typeof VIEWS)[number]

const BARS = 12

function ListCard({ l }: { l: MailingList }) {
  const { t } = useLingui()
  const num = useNumber()
  const { canManage } = useSession()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === l.inbox_id)
  const from = useSender(l.inbox_id, l.channel_id)
  const growth = useGrowth(l.id, 30).data
  const restore = useArchiveList(l.id)
  const days = growth?.days ?? []
  const net = (d: (typeof days)[number]) => (l.double_opt_in ? d.confirmed : d.new) - d.unsubscribed
  const fresh = days.reduce((n, d) => n + (l.double_opt_in ? d.confirmed : d.new), 0)
  const levels: number[] = []
  let level = l.counts.subscribed
  for (let i = BARS - 1; i >= 0; i--) {
    levels[i] = Math.max(0, level)
    const lo = Math.floor((i * days.length) / BARS)
    const hi = Math.floor(((i + 1) * days.length) / BARS)
    level -= days.slice(lo, hi).reduce((n, d) => n + net(d), 0)
  }
  const top = Math.max(1, ...levels)
  const color = inbox?.branding.color ?? "var(--brand)"
  const added = num(fresh)
  const unsubscribed = num(l.counts.unsubscribed)
  const pending = num(l.counts.pending)
  const archived = !!l.archived_at
  return (
    <li className="flex flex-col gap-2">
      <Link
        to={listLink(l.id)}
        className="flex h-full flex-col gap-3 rounded-xl border bg-card px-[18px] pt-4 pb-3.5 outline-none transition-colors hover:border-faint focus-visible:ring-2 focus-visible:ring-ring"
        data-testid="lists-card"
      >
        <span className="flex items-center gap-2">
          {inbox && <InboxMark name={inbox.name} color={inbox.branding.color} className="size-6 text-caption" />}
          <span className="min-w-0 flex-1 truncate text-[16.5px] font-semibold">{l.name}</span>
          {!l.public && (
            <Chip>
              <Trans>Private</Trans>
            </Chip>
          )}
          <Chip>{l.double_opt_in ? <Trans>Double opt-in</Trans> : <Trans>Single opt-in</Trans>}</Chip>
        </span>
        <span className="flex items-end gap-4">
          <span className="flex flex-col leading-tight">
            <b className="text-[26px] leading-[1.15] font-semibold tracking-[-0.01em] tabular-nums">
              {num(l.counts.subscribed)}
            </b>
            <span className="mt-1 text-small text-success">
              {growth ? <Trans>+{added} this month</Trans> : <>&nbsp;</>}
            </span>
          </span>
          <span className="flex h-11 flex-1 items-end gap-1" title={t`Subscribers over the last 30 days`}>
            {growth &&
              levels.map((v, i) => (
                <span
                  key={i}
                  className="flex-1 rounded-t-[2px]"
                  style={{
                    height: Math.max(4, Math.round((44 * v) / top)),
                    background: i === BARS - 1 ? color : `color-mix(in oklab, ${color} 45%, transparent)`,
                  }}
                />
              ))}
          </span>
        </span>
        <span className="mt-auto flex flex-wrap items-center justify-between gap-x-3 gap-y-1 border-t pt-3 text-small text-muted-foreground">
          <span className="flex min-w-0 items-center gap-2">
            {from ? (
              <span className="truncate">
                <Trans>From {from}</Trans>
              </span>
            ) : (
              <span className="text-warning">
                <Trans>No sender yet</Trans>
              </span>
            )}
            {from && !l.sender_ready && !archived && <NeedsSender />}
          </span>
          <span className="tabular-nums">
            {l.counts.pending > 0 ? (
              <Trans>
                {pending} waiting · {unsubscribed} unsubscribed
              </Trans>
            ) : (
              <Trans>{unsubscribed} unsubscribed</Trans>
            )}
          </span>
        </span>
      </Link>
      {archived && canManage && (
        <div className="flex justify-end">
          <Button
            variant="outline"
            className={look.btnSm}
            disabled={restore.isPending}
            onClick={() => restore.mutate(false, { onSuccess: () => toast(t`List restored`) })}
            data-testid="list-restore"
          >
            <Trans>Restore</Trans>
          </Button>
        </div>
      )}
      <ErrorLine error={restore.error} />
    </li>
  )
}

function ImportPicker({ lists, onClose }: { lists: MailingList[]; onClose: () => void }) {
  const navigate = useNavigate()
  const num = useNumber()
  const inboxes = useInboxes().data ?? []
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent data-testid="lists-import-dialog">
        <DialogHeader>
          <DialogTitle>
            <Trans>Import a CSV file</Trans>
          </DialogTitle>
          <DialogDescription>
            <Trans>Pick the list the people join.</Trans>
          </DialogDescription>
        </DialogHeader>
        <ul className="flex max-h-80 flex-col gap-1.5 overflow-y-auto">
          {lists.map((l) => {
            const inbox = inboxes.find((i) => i.id === l.inbox_id)
            return (
              <li key={l.id}>
                <button
                  type="button"
                  onClick={() => navigate(listLink(l.id, "import"))}
                  className="flex w-full items-center gap-3 rounded-lg border px-3 py-2 text-start outline-none transition-colors hover:border-faint focus-visible:ring-2 focus-visible:ring-ring"
                >
                  {inbox && <InboxMark name={inbox.name} color={inbox.branding.color} className="size-5 text-[9px]" />}
                  <span className="min-w-0 flex-1 truncate font-medium">{l.name}</span>
                  <span className="shrink-0 text-small text-faint tabular-nums">{num(l.counts.subscribed)}</span>
                </button>
              </li>
            )
          })}
        </ul>
        <div className="flex justify-end">
          <Button variant="ghost" onClick={onClose}>
            <Trans>Cancel</Trans>
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

export function ListsOverview() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { canManage } = useSession()
  const { pathname, search } = useLocation()
  const [params] = useSearchParams()
  const [inboxId] = useInboxFilter()
  const inboxes = useInboxes().data ?? []
  const scope = inboxId ? inboxes.find((i) => i.id === inboxId)?.name : undefined
  const raw = params.get("tab") as View | null
  const view: View = raw && VIEWS.includes(raw) ? raw : "lists"
  const archived = view === "archived"
  const lists = useLists(inboxId, archived)
  const open = useLists(inboxId, false)
  const items = lists.data?.pages.flatMap((p) => p.items) ?? []
  const importable = open.data?.pages.flatMap((p) => p.items) ?? []
  const [picking, setPicking] = useState(false)
  const grid = useRowFocus<HTMLUListElement>(!picking)
  const createTo = inboxId ? `/setup/newsletter?inbox=${inboxId}` : "/setup/newsletter"
  const newState = { back: pathname + search } satisfies SetupState
  const startImport = () => {
    if (importable.length === 1) navigate(listLink(importable[0].id, "import"))
    else if (importable.length > 1) setPicking(true)
  }
  useHotkeys(
    {
      ...(canManage ? { [LIST_KEYS.create]: () => navigate(createTo, { state: newState }) } : {}),
      ...(canManage && importable.length > 0 ? { [LIST_KEYS.import]: startImport } : {}),
      [LIST_KEYS.lists]: () => navigate("/broadcasts/lists"),
      [LIST_KEYS.archived]: () => navigate("/broadcasts/lists?tab=archived"),
    },
    !picking
  )
  const tabs = VIEWS.map((v) => ({
    key: v,
    to: v === "lists" ? "/broadcasts/lists" : "/broadcasts/lists?tab=archived",
    label: v === "lists" ? t`Lists` : t`Archived`,
    testId: `lists-view-${v}`,
  }))
  return (
    <main className="flex min-h-0 min-w-0 flex-1 flex-col bg-background" data-testid="lists-home">
      <PageTop
        title={<Trans>Lists</Trans>}
        sub={<Trans>Who agreed to hear from you, with consent recorded</Trans>}
        actions={
          canManage && (
            <>
              {importable.length > 0 && (
                <Button
                  variant="outline"
                  className={look.btn}
                  onClick={startImport}
                  title={keyHint(t`Import CSV`, LIST_KEYS.import)}
                  data-testid="lists-home-import"
                >
                  <Trans>Import CSV</Trans>
                </Button>
              )}
              <Button
                className={look.btnInk}
                render={<Link to={createTo} state={newState} />}
                title={keyHint(t`New list`, LIST_KEYS.create)}
                data-testid="lists-home-new"
              >
                <Trans>New list</Trans>
              </Button>
            </>
          )
        }
        tabs={<FrameTabs label={t`Lists`} value={view} items={tabs} />}
      />
      <div className={cn("min-h-0 flex-1 in-data-frame:overflow-y-auto", framePad)}>
        <ErrorLine error={lists.error} />
        {lists.isPending ? (
          <div className="grid gap-4 sm:grid-cols-2">
            <Skeleton className="h-40 rounded-xl" />
            <Skeleton className="h-40 rounded-xl" />
          </div>
        ) : items.length === 0 ? (
          <div
            className="flex flex-col items-center gap-2 rounded-xl border border-dashed px-6 py-10 text-center text-body text-muted-foreground"
            data-testid="lists-empty"
          >
            <p className="font-semibold text-foreground">
              {archived ? (
                <Trans>No archived lists</Trans>
              ) : scope ? (
                <Trans>No lists for {scope} yet</Trans>
              ) : (
                <Trans>No mailing lists yet</Trans>
              )}
            </p>
            <div className="max-w-xl">
              {archived ? (
                <p>
                  <Trans>Archived lists keep their subscribers and consent records but take no new sign-ups.</Trans>
                </p>
              ) : (
                <ConsentExplainer />
              )}
            </div>
            {!archived && canManage && (
              <Button
                className={cn(look.btnInk, "mt-1")}
                render={<Link to={createTo} state={newState} />}
                data-testid="lists-empty-new"
              >
                <Trans>New list</Trans>
              </Button>
            )}
          </div>
        ) : (
          <ul ref={grid} className="grid gap-4 sm:grid-cols-2" aria-label={t`Mailing lists`}>
            {items.map((l) => (
              <ListCard key={l.id} l={l} />
            ))}
          </ul>
        )}
        {lists.hasNextPage && (
          <Button
            variant="ghost"
            size="sm"
            className="self-center"
            onClick={() => void lists.fetchNextPage()}
            disabled={lists.isFetchingNextPage}
          >
            <Trans>Load more</Trans>
          </Button>
        )}
        {items.length > 0 && !archived && (
          <p className="text-small text-faint">
            <Trans>
              A list is who agreed to hear from you. Every subscriber keeps a consent record you can export.
            </Trans>
          </p>
        )}
        {!canManage && (
          <p className="text-caption text-faint" data-testid="lists-read-only">
            <Trans>Owners and admins create lists, add subscribers and send broadcasts.</Trans>
          </p>
        )}
      </div>
      {picking && <ImportPicker lists={importable} onClose={() => setPicking(false)} />}
    </main>
  )
}

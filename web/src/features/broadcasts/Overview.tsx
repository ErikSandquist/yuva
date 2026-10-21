import { Trans, useLingui } from "@lingui/react/macro"
import { useState } from "react"
import { Link, useLocation, useNavigate, useSearchParams } from "react-router"

import { ErrorLine, InboxMark } from "@/components/common"
import { BROADCAST_KEYS, keyLabel } from "@/components/common/ShortcutSheet"
import { formatDateTime, formatRelative, formatShort } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { broadcastLink, FINISHED, isActive, useBroadcasts } from "@/features/broadcasts/broadcastQueries"
import { BroadcastSteps, broadcastWhen } from "@/features/broadcasts/BroadcastList"
import { ConsentExplainer, NeedsSender } from "@/features/broadcasts/BroadcastsColumn"
import {
  ProgressBar,
  StateBadge,
  StatusNotice,
  useBroadcastsStatus,
  useNumber,
  usePercent,
} from "@/features/broadcasts/BroadcastParts"
import { NewBroadcastDialog } from "@/features/broadcasts/NewBroadcastDialog"
import { listLink, useGrowth, useInboxChannels, useLists, useShowArchived } from "@/features/broadcasts/queries"
import { TestSend, useTestSend } from "@/features/broadcasts/TestSend"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import type { SetupState } from "@/features/setup/setup"
import { Chip, FrameButton, FrameTabs, framePad, PageFrame, ProductTag } from "@/features/status/PageFrame"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useNow } from "@/hooks/use-now"
import { useRowFocus } from "@/hooks/use-row-focus"
import type { Broadcast, MailingList } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

const FILTERS = ["all", "sent", "scheduled", "drafts"] as const
type Filter = (typeof FILTERS)[number]

function matches(f: Filter, b: Broadcast) {
  if (f === "sent") return FINISHED.includes(b.state)
  if (f === "scheduled") return b.state === "scheduled"
  if (f === "drafts") return b.state === "draft"
  return true
}

const cols =
  "grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 @2xl:grid-cols-[minmax(0,1.4fr)_100px_90px_140px_64px_72px] @4xl:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)_100px_90px_140px_64px_72px]"

function useSender(inboxId: string, channelId?: string) {
  const channels = useInboxChannels(inboxId).data ?? []
  const c = channels.find((x) => x.id === channelId) ?? channels.find((x) => x.kind === "email")
  return c?.email ? c.email.from_address || c.email.address : undefined
}

function NextSend({ b, lists }: { b: Broadcast; lists: Map<string, MailingList> }) {
  const { t, i18n } = useLingui()
  const num = useNumber()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === b.inbox_id)
  const from = useSender(b.inbox_id, b.channel_id)
  const [testing, setTesting] = useState(false)
  const test = useTestSend(b.id)
  const now = useNow(60_000)
  const at = b.scheduled_at ?? b.updated_at
  const d = new Date(at)
  const soon = d.getTime() - now < 6 * 86400_000
  const when = soon
    ? new Intl.DateTimeFormat(i18n.locale, { weekday: "long", hour: "2-digit", minute: "2-digit" }).format(d)
    : formatDateTime(at, i18n.locale)
  const to = b.audience.list_ids
    .map((id) => lists.get(id)?.name)
    .filter(Boolean)
    .join(", ")
  const people = num(b.counts.recipients)
  const parts = [
    to ? t`Sends ${when} to ${to}` : t`Sends ${when}`,
    ...(b.counts.recipients > 0 ? [t`${people} people`] : []),
    ...(from ? [t`from ${from}`] : []),
  ]
  return (
    <div className="flex items-center gap-4 rounded-xl border bg-card px-4 py-4" data-testid="broadcasts-next">
      <time
        dateTime={at}
        className="flex w-[54px] shrink-0 flex-col items-center rounded-lg border bg-background py-1.5 leading-[1.1]"
        aria-hidden
      >
        <span className="text-[11px] font-semibold text-brand uppercase">
          {new Intl.DateTimeFormat(i18n.locale, { month: "short" }).format(d)}
        </span>
        <span className="text-[20px] font-semibold tabular-nums">
          {new Intl.DateTimeFormat(i18n.locale, { day: "numeric" }).format(d)}
        </span>
      </time>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="flex min-w-0 items-center gap-2">
          <Link
            to={broadcastLink(b.id)}
            className="truncate text-[16.5px] font-semibold outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
          >
            {b.name}
          </Link>
          {inbox && <ProductTag name={inbox.name} color={inbox.branding.color} />}
        </span>
        <span className="truncate text-body text-muted-foreground">{parts.join(" · ")}</span>
      </span>
      <FrameButton onClick={() => setTesting(true)} data-testid="broadcasts-next-test">
        <Trans>Send a test</Trans>
      </FrameButton>
      <FrameButton render={<Link to={broadcastLink(b.id)} />} data-testid="broadcasts-next-edit">
        <Trans>Edit</Trans>
      </FrameButton>
      <Dialog open={testing} onOpenChange={setTesting}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              <Trans>Send a test</Trans>
            </DialogTitle>
          </DialogHeader>
          <TestSend b={b} state={test} disabled={false} />
        </DialogContent>
      </Dialog>
    </div>
  )
}

function BroadcastRow({ b, lists, top }: { b: Broadcast; lists: Map<string, MailingList>; top: number }) {
  const { t, i18n } = useLingui()
  const num = useNumber()
  const percent = usePercent()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === b.inbox_id)
  const at = broadcastWhen(b)
  const c = b.counts
  const names = b.audience.list_ids.map((id) => lists.get(id)?.name).filter(Boolean)
  const listed = b.audience.list_ids.reduce((n, id) => n + (lists.get(id)?.counts.subscribed ?? 0), 0)
  const share = c.sent > 0 ? c.clicked / c.sent : 0
  return (
    <li>
      <Link
        to={broadcastLink(b.id)}
        className={cn(
          cols,
          "min-h-[58px] border-t px-4 py-2.5 outline-none transition-colors first:border-t-0 hover:bg-surface focus-visible:bg-surface"
        )}
        data-testid="broadcasts-table-row"
      >
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-body font-semibold">{b.name}</span>
          <span className="truncate text-small text-muted-foreground">{b.subject || t`No subject yet`}</span>
          {isActive(b) && <ProgressBar b={b} className="mt-1" />}
        </span>
        <span className="hidden min-w-0 items-center gap-1.5 text-body @4xl:flex">
          {inbox && <ProductTag name={inbox.name} color={inbox.branding.color} />}
          <span className="truncate">{names.join(", ") || "–"}</span>
        </span>
        <span className="justify-self-end @2xl:justify-self-start">
          <StateBadge state={b.state} />
        </span>
        <span className="hidden text-body tabular-nums @2xl:block">
          {c.recipients > 0 ? (
            num(c.recipients)
          ) : listed > 0 ? (
            <span className="text-muted-foreground" title={t`Subscribed to the list now`}>
              {num(listed)}
            </span>
          ) : (
            "–"
          )}
        </span>
        <span className="hidden items-center gap-2 @2xl:flex">
          <span className="relative h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
            {b.click_tracking && c.sent > 0 && (
              <span
                className="absolute inset-y-0 start-0 rounded-full bg-success"
                style={{ width: `${top > 0 ? Math.round((share / top) * 85) : 0}%` }}
              />
            )}
          </span>
          <span
            className="w-11 text-end text-caption text-muted-foreground tabular-nums"
            title={b.click_tracking ? undefined : t`Clicks not tracked`}
          >
            {b.click_tracking && c.sent > 0 ? percent(share) : "–"}
          </span>
        </span>
        <span className="hidden text-body tabular-nums @2xl:block">{c.sent > 0 ? num(c.replied) : "–"}</span>
        <span className="hidden text-small text-muted-foreground @2xl:block">
          {at ? (
            <time dateTime={at} title={formatRelative(at, i18n.locale)}>
              {formatShort(at, i18n.locale)}
            </time>
          ) : (
            "–"
          )}
        </span>
      </Link>
    </li>
  )
}

export function BroadcastsOverview() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const { canManage } = useSession()
  const status = useBroadcastsStatus()
  const [inboxId] = useInboxFilter()
  const broadcasts = useBroadcasts(inboxId)
  const lists = useLists(inboxId, false)
  const archived = useLists(inboxId, true)
  const [creating, setCreating] = useState(false)
  const raw = params.get("tab") as Filter | null
  const filter: Filter = raw && FILTERS.includes(raw) ? raw : "all"
  const items = broadcasts.data?.pages.flatMap((p) => p.items) ?? []
  const byId = new Map(
    [...(lists.data?.pages ?? []), ...(archived.data?.pages ?? [])].flatMap((p) => p.items).map((l) => [l.id, l])
  )
  const next = items
    .filter((b) => b.state === "scheduled" && b.scheduled_at)
    .sort((a, b) => a.scheduled_at!.localeCompare(b.scheduled_at!))[0]
  const rows = items.filter((b) => matches(filter, b))
  const top = Math.max(0, ...rows.map((b) => (b.counts.sent > 0 ? b.counts.clicked / b.counts.sent : 0)))
  const names: Record<Filter, string> = {
    all: t`All`,
    sent: t`Sent`,
    scheduled: t`Scheduled`,
    drafts: t`Drafts`,
  }
  const tabs = FILTERS.map((f) => ({
    key: f,
    to: f === "all" ? { search: "" } : { search: `?tab=${f}` },
    label: names[f],
    testId: `broadcasts-filter-${f}`,
  }))
  const mayCreate = canManage && status?.mode !== "off"
  const newKey = keyLabel(BROADCAST_KEYS.create).join("")
  const table = useRowFocus<HTMLUListElement>(!creating)
  useHotkeys(mayCreate ? { [BROADCAST_KEYS.create]: () => setCreating(true) } : {}, !creating)
  return (
    <PageFrame
      testId="broadcasts-home"
      title={<Trans>Broadcasts</Trans>}
      description={<Trans>E-mails to the people who signed up</Trans>}
      actions={
        mayCreate && (
          <FrameButton
            ink
            onClick={() => setCreating(true)}
            title={t`New broadcast (${newKey})`}
            data-testid="broadcasts-home-new"
          >
            <Trans>New broadcast</Trans>
          </FrameButton>
        )
      }
      tabs={<FrameTabs label={t`Broadcasts`} items={tabs} current={filter} />}
      bodyClassName="@container"
    >
      <div className={framePad}>
        <StatusNotice />
        <ErrorLine error={broadcasts.error} />
        {broadcasts.isPending ? (
          <Skeleton className="h-48 rounded-xl" />
        ) : items.length === 0 ? (
          <div
            className="flex flex-col gap-3 rounded-xl border border-dashed px-6 py-8 text-body text-muted-foreground"
            data-testid="broadcasts-home-empty"
          >
            <p className="font-medium text-foreground">
              <Trans>No broadcasts yet</Trans>
            </p>
            <BroadcastSteps />
          </div>
        ) : (
          <>
            {next && filter !== "sent" && filter !== "drafts" && <NextSend b={next} lists={byId} />}
            <div className="overflow-hidden rounded-xl border bg-card" data-testid="broadcasts-table">
              <div
                className={cn(
                  cols,
                  "hidden border-b bg-surface px-4 py-3 text-small font-medium text-muted-foreground @2xl:grid"
                )}
                aria-hidden
              >
                <span>
                  <Trans>Broadcast</Trans>
                </span>
                <span className="hidden @4xl:block">
                  <Trans>List</Trans>
                </span>
                <span>
                  <Trans>Status</Trans>
                </span>
                <span>
                  <Trans>Recipients</Trans>
                </span>
                <span>
                  <Trans>Clicked</Trans>
                </span>
                <span>
                  <Trans>Replied</Trans>
                </span>
                <span>
                  <Trans>Date</Trans>
                </span>
              </div>
              {rows.length === 0 ? (
                <p className="px-4 py-10 text-center text-body text-faint">
                  <Trans>Nothing here.</Trans>
                </p>
              ) : (
                <ul ref={table}>
                  {rows.map((b) => (
                    <BroadcastRow key={b.id} b={b} lists={byId} top={top} />
                  ))}
                </ul>
              )}
            </div>
            {broadcasts.hasNextPage && (
              <Button
                variant="ghost"
                size="sm"
                className="self-center"
                onClick={() => void broadcasts.fetchNextPage()}
                disabled={broadcasts.isFetchingNextPage}
              >
                <Trans>Load more</Trans>
              </Button>
            )}
            <p className="text-small text-faint">
              <Trans>Yuva counts clicks and replies. It does not track opens.</Trans>
            </p>
          </>
        )}
      </div>
      {creating && (
        <NewBroadcastDialog
          inboxId={inboxId}
          onClose={() => setCreating(false)}
          onCreated={(b) => {
            setCreating(false)
            navigate(broadcastLink(b.id))
          }}
        />
      )}
    </PageFrame>
  )
}

function ListCard({ l }: { l: MailingList }) {
  const { t } = useLingui()
  const num = useNumber()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === l.inbox_id)
  const from = useSender(l.inbox_id, l.channel_id)
  const growth = useGrowth(l.id, 30).data
  const days = growth?.days ?? []
  const net = (d: (typeof days)[number]) => (l.double_opt_in ? d.confirmed : d.new) - d.unsubscribed
  const fresh = days.reduce((n, d) => n + (l.double_opt_in ? d.confirmed : d.new), 0)
  const BARS = 12
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
  return (
    <li>
      <Link
        to={listLink(l.id)}
        className="flex h-full flex-col gap-3 rounded-xl border bg-card px-[18px] pt-4 pb-3.5 outline-none transition-colors hover:border-faint focus-visible:ring-2 focus-visible:ring-ring"
        data-testid="lists-card"
      >
        <span className="flex items-center gap-2">
          {inbox && <InboxMark name={inbox.name} color={inbox.branding.color} className="size-6 text-caption" />}
          <span className="min-w-0 flex-1 truncate text-[16.5px] font-semibold">{l.name}</span>
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
              <Trans>From no sender yet</Trans>
            )}
            {!l.sender_ready && !l.archived_at && <NeedsSender />}
          </span>
          <span className="tabular-nums">
            {l.counts.pending > 0 ? (
              <Trans>
                {pending} waiting · {unsubscribed} unsubscribed
              </Trans>
            ) : (
              <Trans>{unsubscribed} unsubscribed</Trans>
            )}
            {" · "}
            {l.public ? <Trans>public form</Trans> : <Trans>private</Trans>}
          </span>
        </span>
      </Link>
    </li>
  )
}

export function ListsOverview() {
  const { t } = useLingui()
  const { canManage } = useSession()
  const { pathname, search } = useLocation()
  const [inboxId] = useInboxFilter()
  const [archived, setArchived] = useShowArchived()
  const lists = useLists(inboxId, archived)
  const items = lists.data?.pages.flatMap((p) => p.items) ?? []
  const open = items.filter((l) => !l.archived_at)
  const grid = useRowFocus<HTMLUListElement>()
  return (
    <PageFrame
      testId="lists-home"
      title={<Trans>Lists</Trans>}
      description={<Trans>Who agreed to hear from you, with consent recorded</Trans>}
      actions={
        canManage && (
          <>
            {open.length === 1 ? (
              <FrameButton render={<Link to={listLink(open[0].id, "import")} />} data-testid="lists-home-import">
                <Trans>Import</Trans>
              </FrameButton>
            ) : (
              open.length > 1 && (
                <DropdownMenu>
                  <DropdownMenuTrigger render={<FrameButton />} data-testid="lists-home-import">
                    <Trans>Import</Trans>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="min-w-52">
                    {open.map((l) => (
                      <DropdownMenuItem key={l.id} render={<Link to={listLink(l.id, "import")} />}>
                        {l.name}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              )
            )}
            <FrameButton
              ink
              render={
                <Link
                  to={inboxId ? `/setup/newsletter?inbox=${inboxId}` : "/setup/newsletter"}
                  state={{ back: pathname + search } satisfies SetupState}
                />
              }
              data-testid="lists-home-new"
            >
              <Trans>New list</Trans>
            </FrameButton>
          </>
        )
      }
    >
      <div className={framePad}>
        <ErrorLine error={lists.error} />
        {lists.isPending ? (
          <div className="grid gap-4 sm:grid-cols-2">
            <Skeleton className="h-40 rounded-xl" />
            <Skeleton className="h-40 rounded-xl" />
          </div>
        ) : items.length === 0 ? (
          <div className="flex flex-col gap-2 rounded-xl border border-dashed px-6 py-8 text-body text-muted-foreground">
            <p className="font-medium text-foreground">
              {archived ? <Trans>No archived lists</Trans> : <Trans>No mailing lists yet</Trans>}
            </p>
            {!archived && <ConsentExplainer />}
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
        <Label className="mt-2 w-fit gap-3 font-normal text-muted-foreground">
          <Switch checked={archived} onCheckedChange={setArchived} data-testid="lists-archived" />
          <Trans>Show archived lists</Trans>
        </Label>
        {!canManage && (
          <p className="text-caption text-faint" data-testid="lists-read-only">
            <Trans>Owners and admins create lists, add subscribers and send broadcasts.</Trans>
          </p>
        )}
      </div>
    </PageFrame>
  )
}

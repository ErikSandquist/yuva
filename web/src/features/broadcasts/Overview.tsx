import { Trans, useLingui } from "@lingui/react/macro"
import { PlusIcon } from "lucide-react"
import { useState } from "react"
import { Link, useLocation, useNavigate, useSearchParams } from "react-router"

import { ErrorLine, InboxMark } from "@/components/common"
import { Pane } from "@/components/common/Column"
import { BROADCAST_KEYS, keyLabel } from "@/components/common/ShortcutSheet"
import { formatDateTime, formatRelative, formatShort } from "@/components/common/text"
import { Button } from "@/components/ui/button"
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
import { listLink, useGrowth, useLists, useShowArchived } from "@/features/broadcasts/queries"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { PageHeader, PageTabs } from "@/features/settings/ui"
import type { SetupState } from "@/features/setup/setup"
import { useHotkeys } from "@/hooks/use-hotkeys"
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
  "grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 @2xl:grid-cols-[minmax(0,1.4fr)_96px_72px_120px_64px_64px] @4xl:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)_96px_72px_120px_64px_64px]"

function NextSend({ b, lists }: { b: Broadcast; lists: Map<string, MailingList> }) {
  const { i18n } = useLingui()
  const num = useNumber()
  const at = b.scheduled_at ?? b.updated_at
  const d = new Date(at)
  const when = formatDateTime(at, i18n.locale)
  const names = b.audience.list_ids.map((id) => lists.get(id)?.name).filter(Boolean)
  const to = names.join(", ")
  const people = num(b.counts.recipients)
  return (
    <Link
      to={broadcastLink(b.id)}
      className="flex items-center gap-4 rounded-2xl border bg-card p-4 outline-none transition-colors hover:border-faint focus-visible:ring-2 focus-visible:ring-ring"
      data-testid="broadcasts-next"
    >
      <time
        dateTime={at}
        className="flex w-13 shrink-0 flex-col items-center rounded-xl border py-1.5 leading-tight"
        aria-hidden
      >
        <span className="text-caption font-semibold text-brand uppercase">
          {new Intl.DateTimeFormat(i18n.locale, { month: "short" }).format(d)}
        </span>
        <span className="text-title tabular-nums">
          {new Intl.DateTimeFormat(i18n.locale, { day: "numeric" }).format(d)}
        </span>
      </time>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-caption font-medium text-faint">
          <Trans>Next send</Trans>
        </span>
        <span className="truncate text-reading font-semibold">{b.name}</span>
        <span className="truncate text-small text-muted-foreground">
          {to && b.counts.recipients > 0 ? (
            <Trans>
              Sends {when} to {to} · {people} people
            </Trans>
          ) : to ? (
            <Trans>
              Sends {when} to {to}
            </Trans>
          ) : (
            <Trans>Sends {when}</Trans>
          )}
        </span>
      </span>
      <StateBadge state={b.state} />
    </Link>
  )
}

function BroadcastRow({ b, lists }: { b: Broadcast; lists: Map<string, MailingList> }) {
  const { t, i18n } = useLingui()
  const num = useNumber()
  const percent = usePercent()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === b.inbox_id)
  const at = broadcastWhen(b)
  const c = b.counts
  const names = b.audience.list_ids.map((id) => lists.get(id)?.name).filter(Boolean)
  const share = c.sent > 0 ? c.clicked / c.sent : 0
  return (
    <li>
      <Link
        to={broadcastLink(b.id)}
        className={cn(
          cols,
          "border-t px-4 py-3 outline-none transition-colors first:border-t-0 hover:bg-muted focus-visible:bg-muted"
        )}
        data-testid="broadcasts-table-row"
      >
        <span className="flex min-w-0 flex-col gap-0.5">
          <span className="truncate text-body font-medium">{b.name}</span>
          <span className="truncate text-small text-faint">{b.subject || t`No subject yet`}</span>
          {isActive(b) && <ProgressBar b={b} className="mt-1" />}
        </span>
        <span className="hidden min-w-0 items-center gap-1.5 text-small text-muted-foreground @4xl:flex">
          {inbox && <InboxMark name={inbox.name} color={inbox.branding.color} className="size-4 text-[9px]" />}
          <span className="truncate">{names.join(", ") || "–"}</span>
        </span>
        <span className="justify-self-end @2xl:justify-self-start">
          <StateBadge state={b.state} />
        </span>
        <span className="hidden text-small text-muted-foreground tabular-nums @2xl:block">
          {c.recipients > 0 ? num(c.recipients) : "–"}
        </span>
        <span className="hidden items-center gap-2 @2xl:flex">
          {b.click_tracking && c.sent > 0 ? (
            <>
              <span className="relative h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
                <span
                  className="absolute inset-y-0 start-0 rounded-full bg-success"
                  style={{ width: `${Math.min(100, share * 100)}%` }}
                />
              </span>
              <span className="w-11 text-end text-caption text-faint tabular-nums">{percent(share)}</span>
            </>
          ) : (
            <span className="text-small text-faint" title={b.click_tracking ? undefined : t`Clicks not tracked`}>
              –
            </span>
          )}
        </span>
        <span className="hidden text-small text-muted-foreground tabular-nums @2xl:block">
          {c.sent > 0 ? num(c.replied) : "–"}
        </span>
        <span className="hidden text-caption text-faint @2xl:block">
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
    count: broadcasts.data ? items.filter((b) => matches(f, b)).length : undefined,
    testId: `broadcasts-filter-${f}`,
  }))
  const mayCreate = canManage && status?.mode !== "off"
  const newKey = keyLabel(BROADCAST_KEYS.create).join("")
  const table = useRowFocus<HTMLUListElement>(!creating)
  useHotkeys(mayCreate ? { [BROADCAST_KEYS.create]: () => setCreating(true) } : {}, !creating)
  return (
    <Pane wide testId="broadcasts-home" className="@container gap-6">
      <PageHeader
        back={false}
        title={<Trans>Broadcasts</Trans>}
        description={<Trans>E-mails to the people who signed up for them.</Trans>}
        action={
          mayCreate && (
            <Button
              size="sm"
              onClick={() => setCreating(true)}
              title={t`New broadcast (${newKey})`}
              data-testid="broadcasts-home-new"
            >
              <PlusIcon />
              <Trans>New broadcast</Trans>
            </Button>
          )
        }
        tabs={<PageTabs label={t`Broadcasts`} items={tabs} current={filter} />}
      />
      <StatusNotice />
      <ErrorLine error={broadcasts.error} />
      {broadcasts.isPending ? (
        <Skeleton className="h-48 rounded-2xl" />
      ) : items.length === 0 ? (
        <div
          className="flex flex-col gap-3 rounded-2xl border border-dashed px-6 py-8 text-body text-muted-foreground"
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
          <div className="overflow-hidden rounded-2xl border bg-card" data-testid="broadcasts-table">
            <div
              className={cn(
                cols,
                "hidden border-b bg-surface px-4 py-2.5 text-caption font-medium text-faint @2xl:grid"
              )}
              aria-hidden
            >
              <span>
                <Trans>Broadcast</Trans>
              </span>
              <span className="hidden @4xl:block">
                <Trans>Lists</Trans>
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
                  <BroadcastRow key={b.id} b={b} lists={byId} />
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
    </Pane>
  )
}

function ListCard({ l }: { l: MailingList }) {
  const { t } = useLingui()
  const num = useNumber()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === l.inbox_id)
  const growth = useGrowth(l.id, 30).data
  const days = growth?.days ?? []
  const fresh = days.reduce((n, d) => n + d.new, 0)
  const top = Math.max(1, ...days.map((d) => d.new))
  const added = num(fresh)
  const unsubscribed = num(l.counts.unsubscribed)
  const pending = num(l.counts.pending)
  return (
    <li>
      <Link
        to={listLink(l.id)}
        className="flex h-full flex-col gap-4 rounded-2xl border bg-card p-4.5 outline-none transition-colors hover:border-faint focus-visible:ring-2 focus-visible:ring-ring"
        data-testid="lists-card"
      >
        <span className="flex items-center gap-2">
          {inbox && <InboxMark name={inbox.name} color={inbox.branding.color} className="size-6 text-caption" />}
          <span className="min-w-0 flex-1 truncate text-reading font-semibold">{l.name}</span>
          <span className="shrink-0 rounded-md border bg-surface px-1.5 text-caption text-muted-foreground">
            {l.double_opt_in ? <Trans>Confirmation asked</Trans> : <Trans>No confirmation</Trans>}
          </span>
        </span>
        <span className="flex items-end gap-4">
          <span className="flex flex-col leading-tight">
            <b className="text-page tabular-nums">{num(l.counts.subscribed)}</b>
            <span className="mt-1 text-caption text-success">
              {growth ? <Trans>+{added} in 30 days</Trans> : <>&nbsp;</>}
            </span>
          </span>
          <span className="flex h-11 flex-1 items-end gap-px" title={t`New subscriptions per day, last 30 days`}>
            {days.map((d) => (
              <span
                key={d.date}
                className={cn("min-w-px flex-1 rounded-t-[2px]", d.new > 0 ? "bg-success/60" : "bg-muted")}
                style={{ height: d.new > 0 ? Math.max(3, Math.round((44 * d.new) / top)) : 2 }}
              />
            ))}
          </span>
        </span>
        <span className="mt-auto flex flex-wrap items-center justify-between gap-x-3 gap-y-1 border-t pt-3 text-small text-faint">
          <span className="flex items-center gap-2">
            {l.public ? <Trans>Public</Trans> : <Trans>Private</Trans>}
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
  const grid = useRowFocus<HTMLUListElement>()
  return (
    <Pane wide testId="lists-home" className="gap-6">
      <PageHeader
        back={false}
        title={<Trans>Lists</Trans>}
        description={<Trans>Who agreed to hear from you, with their consent recorded.</Trans>}
        action={
          canManage && (
            <Button
              size="sm"
              render={
                <Link
                  to={inboxId ? `/setup/newsletter?inbox=${inboxId}` : "/setup/newsletter"}
                  state={{ back: pathname + search } satisfies SetupState}
                />
              }
              data-testid="lists-home-new"
            >
              <PlusIcon />
              <Trans>New list</Trans>
            </Button>
          )
        }
      />
      <ErrorLine error={lists.error} />
      {lists.isPending ? (
        <div className="grid gap-3 sm:grid-cols-2">
          <Skeleton className="h-40 rounded-2xl" />
          <Skeleton className="h-40 rounded-2xl" />
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col gap-2 rounded-2xl border border-dashed px-6 py-8 text-body text-muted-foreground">
          <p className="font-medium text-foreground">
            {archived ? <Trans>No archived lists</Trans> : <Trans>No mailing lists yet</Trans>}
          </p>
          {!archived && <ConsentExplainer />}
        </div>
      ) : (
        <ul ref={grid} className="grid gap-3 sm:grid-cols-2" aria-label={t`Mailing lists`}>
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
      <Label className="justify-between gap-3 rounded-2xl border bg-card px-4 py-3 font-normal text-muted-foreground">
        <Trans>Show archived lists</Trans>
        <Switch checked={archived} onCheckedChange={setArchived} data-testid="lists-archived" />
      </Label>
      {!canManage && (
        <p className="text-caption text-faint" data-testid="lists-read-only">
          <Trans>Owners and admins create lists, add subscribers and send broadcasts.</Trans>
        </p>
      )}
    </Pane>
  )
}

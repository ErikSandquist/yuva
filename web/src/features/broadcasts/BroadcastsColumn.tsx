import { Trans, useLingui } from "@lingui/react/macro"
import { MailWarningIcon, PlusIcon } from "lucide-react"
import { lazy, Suspense, useEffect, useRef, useState } from "react"
import { Link, useLocation, useMatch, useNavigate } from "react-router"

import { Dot, ErrorLine, Segmented } from "@/components/common"
import { Column, ColumnHeading } from "@/components/common/Column"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { BroadcastList } from "@/features/broadcasts/BroadcastList"
import { NewListDialog } from "@/features/broadcasts/ListForm"
import { listLink, useLists, useShowArchived } from "@/features/broadcasts/queries"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { InboxPicker } from "@/features/inbox/InboxPicker"
import { Label } from "@/components/ui/label"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useResetState } from "@/hooks/use-reset-state"
import type { Inbox, MailingList } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

export function ConsentExplainer() {
  return (
    <p>
      <Trans>
        A mailing list holds the people who agreed to get your broadcasts. They join on a subscribe form on your site,
        or your team adds them and records how they agreed. Every step is kept as proof of consent, and everyone can
        leave with one click.
      </Trans>
    </p>
  )
}

export function NeedsSender() {
  const { t } = useLingui()
  return (
    <span
      className="inline-flex shrink-0 items-center gap-1 rounded-full bg-warning/12 px-2 py-px text-caption font-medium text-warning"
      title={t`No e-mail channel that can send its confirmation and welcome mails`}
      data-testid="list-needs-sender-badge"
    >
      <MailWarningIcon className="size-3" />
      <Trans>Needs a sender</Trans>
    </span>
  )
}

function Row({ l, current, cursor }: { l: MailingList; current: boolean; cursor: boolean }) {
  const { t, i18n } = useLingui()
  const fmt = new Intl.NumberFormat(i18n.locale)
  const subscribed = fmt.format(l.counts.subscribed)
  const pending = fmt.format(l.counts.pending)
  return (
    <li>
      <Link
        to={listLink(l.id)}
        className={cn(
          "flex flex-col gap-1 rounded-xl px-2.5 py-2.5 outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring",
          current && "bg-muted",
          cursor && "ring-2 ring-ring"
        )}
        aria-current={current ? "page" : undefined}
        data-testid="list-row"
      >
        <span className="flex items-center gap-2">
          <span className="min-w-0 flex-1 truncate text-body font-medium">{l.name}</span>
          <span className="shrink-0 text-small tabular-nums" title={t`${subscribed} subscribed`}>
            {subscribed}
          </span>
        </span>
        <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-caption text-faint">
          <span className="min-w-0 flex-1 truncate">
            {l.counts.pending > 0 ? t`${pending} waiting for confirmation` : l.public ? t`Public` : t`Private`}
          </span>
          {!l.sender_ready && !l.archived_at && <NeedsSender />}
        </span>
      </Link>
    </li>
  )
}

function group(lists: MailingList[], inboxes: Inbox[]) {
  return inboxes
    .map((inbox) => ({ inbox, lists: lists.filter((l) => l.inbox_id === inbox.id) }))
    .filter((g) => g.lists.length > 0)
}

const DesignsList = lazy(() => import("@/features/designs/DesignsList"))

type Tab = "broadcasts" | "lists" | "designs"

const TAB_PATHS: Record<Tab, string> = {
  broadcasts: "/broadcasts",
  lists: "/broadcasts/lists",
  designs: "/broadcasts/designs",
}

export function BroadcastsColumn() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const tab: Tab = pathname.startsWith("/broadcasts/designs")
    ? "designs"
    : pathname.startsWith("/broadcasts/lists")
      ? "lists"
      : "broadcasts"
  return (
    <Column
      title={<Trans>Broadcasts</Trans>}
      testId="broadcasts-panel"
      actions={<InboxPicker className="max-w-40" />}
      bodyClassName="px-2"
    >
      <div className="px-1 pt-1 pb-3">
        <Segmented
          value={tab}
          onChange={(v) => navigate(TAB_PATHS[v])}
          label={t`Broadcasts`}
          className="w-full"
          itemClassName="flex-1 px-2"
          items={[
            { value: "broadcasts", label: t`Broadcasts`, testId: "broadcasts-tab-broadcasts" },
            { value: "lists", label: t`Lists`, testId: "broadcasts-tab-lists" },
            { value: "designs", label: t`Designs`, testId: "broadcasts-tab-designs" },
          ]}
        />
      </div>
      {tab === "broadcasts" ? (
        <BroadcastList />
      ) : tab === "designs" ? (
        <Suspense
          fallback={
            <div className="flex flex-col gap-2 p-2">
              <Skeleton className="h-12 w-full" />
              <Skeleton className="h-12 w-full" />
            </div>
          }
        >
          <DesignsList />
        </Suspense>
      ) : (
        <Lists />
      )}
    </Column>
  )
}

function Lists() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { canManage } = useSession()
  const [inboxId] = useInboxFilter()
  const [archived, setArchived] = useShowArchived()
  const inboxes = useInboxes().data ?? []
  const lists = useLists(inboxId, archived)
  const items = lists.data?.pages.flatMap((p) => p.items) ?? []
  const groups = inboxes.length > 1 && !inboxId ? group(items, inboxes) : [{ inbox: null, lists: items }]
  const ordered = groups.flatMap((g) => g.lists)
  const currentId = useMatch("/broadcasts/lists/:listId")?.params.listId
  const [creating, setCreating] = useState(false)
  const [cursor, setCursor] = useResetState(-1, JSON.stringify([inboxId, archived]))
  const list = useRef<HTMLDivElement>(null)

  useHotkeys({
    j: () => setCursor((i) => Math.min(ordered.length - 1, i + 1)),
    k: () => setCursor((i) => Math.max(0, i - 1)),
    ArrowDown: () => setCursor((i) => Math.min(ordered.length - 1, i + 1)),
    ArrowUp: () => setCursor((i) => Math.max(0, i - 1)),
    ...(cursor >= 0 && ordered[cursor]
      ? {
          Enter: () => {
            navigate(listLink(ordered[cursor].id))
            setCursor(-1)
          },
        }
      : {}),
  })
  useEffect(() => {
    if (cursor >= 0)
      list.current?.querySelectorAll("[data-testid=list-row]")[cursor]?.scrollIntoView({ block: "nearest" })
  }, [cursor])

  let index = 0
  return (
    <>
      <nav aria-label={t`Mailing lists`} className="flex flex-col" ref={list}>
        <div className="flex items-center gap-2 px-2.5 pt-1 pb-1">
          <h3 className="min-w-0 flex-1 text-small font-medium text-faint">
            <Trans>Mailing lists</Trans>
          </h3>
          {canManage && (
            <Button variant="outline" size="sm" onClick={() => setCreating(true)} data-testid="new-list">
              <PlusIcon />
              <Trans>New list</Trans>
            </Button>
          )}
        </div>
        {lists.isPending ? (
          <div className="flex flex-col gap-2 p-2">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : lists.error ? (
          <ErrorLine error={lists.error} className="p-3" />
        ) : items.length === 0 ? (
          <div className="px-4 py-8 text-center text-body text-faint" data-testid="lists-empty">
            <p className="mb-2 font-medium text-foreground">
              {archived ? <Trans>No archived lists</Trans> : <Trans>No mailing lists yet</Trans>}
            </p>
            {!archived && <ConsentExplainer />}
          </div>
        ) : (
          groups.map((g) => (
            <section key={g.inbox?.id ?? "all"} className="flex flex-col">
              {g.inbox && (
                <ColumnHeading>
                  <span className="flex items-center gap-1.5">
                    <Dot color={g.inbox.branding.color} className="size-[7px] rounded-[2px]" />
                    {g.inbox.name}
                  </span>
                </ColumnHeading>
              )}
              <ul className="flex flex-col">
                {g.lists.map((l) => {
                  const i = index++
                  return <Row key={l.id} l={l} current={l.id === currentId} cursor={i === cursor} />
                })}
              </ul>
            </section>
          ))
        )}
        {lists.hasNextPage && (
          <div className="mt-2 flex justify-center">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void lists.fetchNextPage()}
              disabled={lists.isFetchingNextPage}
            >
              <Trans>Load more</Trans>
            </Button>
          </div>
        )}
        <Label className="mt-3 justify-between gap-3 px-2.5 py-2 font-normal text-muted-foreground">
          <Trans>Show archived lists</Trans>
          <Switch checked={archived} onCheckedChange={setArchived} data-testid="lists-archived" />
        </Label>
        {!canManage && (
          <p className="px-2.5 pt-2 text-caption text-faint" data-testid="lists-read-only">
            <Trans>Owners and admins create lists, add subscribers and send broadcasts.</Trans>
          </p>
        )}
      </nav>
      {creating && (
        <NewListDialog
          inboxId={inboxId}
          onClose={() => setCreating(false)}
          onCreated={(l) => {
            setCreating(false)
            navigate(listLink(l.id))
          }}
        />
      )}
    </>
  )
}

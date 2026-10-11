import { Trans, useLingui } from "@lingui/react/macro"
import { PlusIcon } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { Link, useMatch, useNavigate } from "react-router"

import { Dot, ErrorLine } from "@/components/common"
import { ColumnHeading } from "@/components/common/Column"
import { BROADCAST_KEYS, keyLabel } from "@/components/common/ShortcutSheet"
import { formatRelative, formatShort } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { broadcastLink, FINISHED, isActive, useBroadcasts } from "@/features/broadcasts/broadcastQueries"
import { NewBroadcastDialog } from "@/features/broadcasts/NewBroadcastDialog"
import {
  progressOf,
  ProgressBar,
  StateBadge,
  StatusNotice,
  useBroadcastsStatus,
  useNumber,
} from "@/features/broadcasts/BroadcastParts"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useResetState } from "@/hooks/use-reset-state"
import type { Broadcast } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

export function BroadcastSteps() {
  return (
    <ol className="flex list-decimal flex-col gap-1 ps-5 text-start">
      <li>
        <Trans>
          A{" "}
          <Link to="/broadcasts/lists" className="text-brand underline-offset-4 hover:underline">
            mailing list
          </Link>{" "}
          holds the people who agreed to get your mail.
        </Trans>
      </li>
      <li>
        <Trans>
          A{" "}
          <Link to="/broadcasts/designs" className="text-brand underline-offset-4 hover:underline">
            design
          </Link>{" "}
          is how the mail looks.
        </Trans>
      </li>
      <li>
        <Trans>A broadcast sends a design to one or more lists, now or later, and reports what happened.</Trans>
      </li>
    </ol>
  )
}

function when(b: Broadcast) {
  if (b.state === "scheduled") return b.scheduled_at
  if (FINISHED.includes(b.state)) return b.finished_at ?? b.started_at ?? b.updated_at
  if (isActive(b)) return b.started_at ?? b.scheduled_at ?? b.updated_at
  return b.updated_at
}

function Row({ b, current, cursor }: { b: Broadcast; current: boolean; cursor: boolean }) {
  const { t, i18n } = useLingui()
  const num = useNumber()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === b.inbox_id)
  const at = when(b)
  const { done, total } = progressOf(b)
  const recipients = num(b.counts.recipients)
  const sent = num(done)
  const all = num(total)
  return (
    <li>
      <Link
        to={broadcastLink(b.id)}
        className={cn(
          "flex flex-col gap-1 rounded-xl px-2.5 py-2.5 outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring",
          current && "bg-muted",
          cursor && "ring-2 ring-ring"
        )}
        aria-current={current ? "page" : undefined}
        data-testid="broadcast-row"
      >
        <span className="flex min-w-0 items-center gap-2">
          <span className="min-w-0 flex-1 truncate text-body font-medium">{b.name}</span>
          <StateBadge state={b.state} />
        </span>
        <span className="flex min-w-0 items-center gap-1.5 text-caption text-faint">
          {inbox && (
            <span className="inline-flex min-w-0 items-center gap-1">
              <Dot color={inbox.branding.color} className="size-[6px] shrink-0 rounded-[2px]" />
              <span className="truncate">{inbox.name}</span>
            </span>
          )}
          {inbox && <span aria-hidden>·</span>}
          {isActive(b) && b.state !== "preparing" ? (
            <span className="shrink-0 tabular-nums">{t`${sent} of ${all}`}</span>
          ) : b.counts.recipients > 0 ? (
            <span className="shrink-0 tabular-nums">{t`${recipients} recipients`}</span>
          ) : (
            <span className="min-w-0 truncate">{b.subject || t`No subject yet`}</span>
          )}
          {at && (
            <>
              <span aria-hidden>·</span>
              <time className="shrink-0" dateTime={at} title={formatRelative(at, i18n.locale)}>
                {formatShort(at, i18n.locale)}
              </time>
            </>
          )}
        </span>
        {isActive(b) && <ProgressBar b={b} className="mt-1" />}
      </Link>
    </li>
  )
}

export function BroadcastList() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { canManage } = useSession()
  const status = useBroadcastsStatus()
  const [inboxId] = useInboxFilter()
  const broadcasts = useBroadcasts(inboxId)
  const items = broadcasts.data?.pages.flatMap((p) => p.items) ?? []
  const sections: { key: string; title: string; items: Broadcast[] }[] = [
    { key: "sending", title: t`Sending`, items: items.filter(isActive) },
    { key: "scheduled", title: t`Scheduled`, items: items.filter((b) => b.state === "scheduled") },
    { key: "drafts", title: t`Drafts`, items: items.filter((b) => b.state === "draft") },
    { key: "sent", title: t`Sent`, items: items.filter((b) => FINISHED.includes(b.state)) },
  ].filter((s) => s.items.length > 0)
  const ordered = sections.flatMap((s) => s.items)
  const currentId = useMatch("/broadcasts/:broadcastId")?.params.broadcastId
  const [creating, setCreating] = useState(false)
  const [cursor, setCursor] = useResetState(-1, inboxId)
  const list = useRef<HTMLDivElement>(null)
  const mayCreate = canManage && status?.mode !== "off"
  const newKey = keyLabel(BROADCAST_KEYS.create).join("")

  useHotkeys({
    j: () => setCursor((i) => Math.min(ordered.length - 1, i + 1)),
    k: () => setCursor((i) => Math.max(0, i - 1)),
    ArrowDown: () => setCursor((i) => Math.min(ordered.length - 1, i + 1)),
    ArrowUp: () => setCursor((i) => Math.max(0, i - 1)),
    ...(mayCreate ? { [BROADCAST_KEYS.create]: () => setCreating(true) } : {}),
    ...(cursor >= 0 && ordered[cursor]
      ? {
          Enter: () => {
            navigate(broadcastLink(ordered[cursor].id))
            setCursor(-1)
          },
        }
      : {}),
  })
  useEffect(() => {
    if (cursor >= 0)
      list.current?.querySelectorAll("[data-testid=broadcast-row]")[cursor]?.scrollIntoView({ block: "nearest" })
  }, [cursor])

  let index = 0
  return (
    <>
      <nav aria-label={t`Broadcasts`} className="flex flex-col" ref={list} data-testid="broadcast-list">
        <div className="flex items-center gap-2 px-2.5 pt-1 pb-1">
          <h3 className="min-w-0 flex-1 text-small font-medium text-faint">
            <Trans>Broadcasts</Trans>
          </h3>
          {mayCreate && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => setCreating(true)}
              title={t`New broadcast (${newKey})`}
              data-testid="new-broadcast"
            >
              <PlusIcon />
              <Trans>New broadcast</Trans>
            </Button>
          )}
        </div>
        <StatusNotice className="mx-1 my-2" />
        {broadcasts.isPending ? (
          <div className="flex flex-col gap-2 p-2">
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        ) : broadcasts.error ? (
          <ErrorLine error={broadcasts.error} className="p-3" />
        ) : items.length === 0 ? (
          <div className="flex flex-col gap-3 px-4 py-8 text-body text-faint" data-testid="broadcasts-empty">
            <p className="text-center font-medium text-foreground">
              <Trans>No broadcasts yet</Trans>
            </p>
            <BroadcastSteps />
          </div>
        ) : (
          sections.map((s) => (
            <section key={s.key} className="flex flex-col" data-testid={`broadcasts-${s.key}`}>
              <ColumnHeading>{s.title}</ColumnHeading>
              <ul className="flex flex-col">
                {s.items.map((b) => {
                  const i = index++
                  return <Row key={b.id} b={b} current={b.id === currentId} cursor={i === cursor} />
                })}
              </ul>
            </section>
          ))
        )}
        {broadcasts.hasNextPage && (
          <div className="mt-2 flex justify-center">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void broadcasts.fetchNextPage()}
              disabled={broadcasts.isFetchingNextPage}
            >
              <Trans>Load more</Trans>
            </Button>
          </div>
        )}
        {!canManage && (
          <p className="px-2.5 pt-3 text-caption text-faint" data-testid="broadcasts-read-only">
            <Trans>
              Owners and admins write and send broadcasts. You see the broadcasts of your inboxes, so you know what a
              reply answers.
            </Trans>
          </p>
        )}
      </nav>
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
    </>
  )
}

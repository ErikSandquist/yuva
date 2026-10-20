import { Trans, useLingui } from "@lingui/react/macro"
import { GlobeIcon, HourglassIcon } from "lucide-react"
import { useEffect, useRef } from "react"
import { Link, useNavigate } from "react-router"

import { ErrorLine, hostOf } from "@/components/common"
import { Pane, PaneBack } from "@/components/common/Column"
import { DISCUSSION_KEYS } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { DocsSitePicker, usePickedSite } from "@/features/docs/DocsColumn"
import { useDocsDays, useDocsSummary } from "@/features/docs/queries"
import { ListSkeleton, LoadMore, ThreadRow } from "@/features/docs/discussions/parts"
import {
  THREAD_SORTS,
  THREAD_VIEWS,
  threadLink,
  useDiscussions,
  useThreadSort,
  useThreadView,
  viewFilters,
  type ThreadView,
} from "@/features/docs/discussions/queries"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { Card, EmptyRow } from "@/features/settings/ui"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useResetState } from "@/hooks/use-reset-state"
import type { DiscussionTeamSort } from "@/lib/api"
import type { DiscussionFilters } from "@/lib/keys"
import { cn } from "@/lib/utils"

export function ThreadList({
  filters,
  showPage,
  from,
  keyboard,
  empty,
}: {
  filters: DiscussionFilters
  showPage: boolean
  from?: string
  keyboard?: boolean
  empty: React.ReactNode
}) {
  const navigate = useNavigate()
  const list = useDiscussions(filters)
  const items = list.data?.pages.flatMap((p) => p.items) ?? []
  const [cursor, setCursor] = useResetState(-1, JSON.stringify(filters))
  const ref = useRef<HTMLUListElement>(null)
  const down = () => setCursor((i) => Math.min(items.length - 1, i + 1))
  const up = () => setCursor((i) => Math.max(0, i - 1))
  useHotkeys(
    {
      [DISCUSSION_KEYS.next]: down,
      [DISCUSSION_KEYS.previous]: up,
      ArrowDown: down,
      ArrowUp: up,
      Enter: () => {
        const d = items[cursor]
        if (d) navigate(threadLink(d.id), { state: from ? { from } : undefined })
      },
    },
    !!keyboard
  )
  useEffect(() => {
    if (cursor >= 0)
      ref.current?.querySelectorAll("[data-testid=thread-row]")[cursor]?.scrollIntoView({ block: "nearest" })
  }, [cursor])
  return (
    <Card flush data-testid="thread-list">
      {list.isPending ? (
        <ListSkeleton />
      ) : list.error ? (
        <ErrorLine error={list.error} className="p-4" />
      ) : items.length === 0 ? (
        <EmptyRow>{empty}</EmptyRow>
      ) : (
        <ul ref={ref} className="divide-y">
          {items.map((d, i) => (
            <ThreadRow key={d.id} d={d} cursor={i === cursor} showPage={showPage} from={from} />
          ))}
        </ul>
      )}
      <LoadMore list={list} />
    </Card>
  )
}

function Chips<T extends string>({
  value,
  onChange,
  items,
  label,
  testId,
}: {
  value: T
  onChange: (v: T) => void
  items: { value: T; label: string; title?: string }[]
  label: string
  testId: string
}) {
  return (
    <div className="flex flex-wrap gap-1.5" role="toolbar" aria-label={label}>
      {items.map((it) => (
        <button
          key={it.value}
          type="button"
          aria-pressed={value === it.value}
          title={it.title}
          onClick={() => onChange(it.value)}
          className={cn(
            "h-7 shrink-0 rounded-full px-3 text-small font-medium transition-colors",
            value === it.value ? "bg-brand-wash text-brand" : "bg-muted text-muted-foreground hover:text-foreground"
          )}
          data-testid={`${testId}-${it.value}`}
        >
          {it.label}
        </button>
      ))}
    </div>
  )
}

export function useViewText(): Record<ThreadView, string> {
  const { t } = useLingui()
  return {
    all: t`All`,
    unanswered: t`Unanswered`,
    pending: t`Waiting for approval`,
    locked: t`Locked`,
    hidden: t`Hidden`,
  }
}

export function DiscussionsPane() {
  const { t } = useLingui()
  const [inboxId] = useInboxFilter()
  const site = usePickedSite(inboxId)
  const [days] = useDocsDays()
  const [view, setView] = useThreadView()
  const [sort, setSort] = useThreadSort()
  const viewText = useViewText()
  const pending = useDocsSummary(inboxId, site, days).data?.totals.pending_posts ?? 0
  const sortText: Record<DiscussionTeamSort, string> = {
    activity: t`Recent activity`,
    new: t`Newest`,
    top: t`Most votes`,
    votes: t`Most votes`,
  }
  const empty: Record<ThreadView, React.ReactNode> = {
    all: <Trans>No discussions yet. Readers start them under your documentation pages.</Trans>,
    unanswered: <Trans>Every open discussion has an answer.</Trans>,
    pending: <Trans>Nothing is waiting for approval.</Trans>,
    locked: <Trans>No locked discussions.</Trans>,
    hidden: <Trans>No hidden discussions.</Trans>,
  }
  const filters: DiscussionFilters = {
    inbox_id: inboxId || undefined,
    site: site || undefined,
    sort,
    ...viewFilters(view),
  }
  return (
    <Pane testId="discussions">
      <header className="flex flex-col">
        <PaneBack to="/docs" label={t`Docs`} />
        <div className="flex flex-wrap items-start justify-between gap-3">
          <h1 className="min-w-0 text-page break-words">
            <Trans>Discussions</Trans>
          </h1>
          {pending > 0 && (
            <Button variant="outline" size="sm" render={<Link to="/docs/discussions/queue" />} data-testid="queue-link">
              <HourglassIcon className="text-warning" />
              <Trans>{pending} waiting for approval</Trans>
            </Button>
          )}
        </div>
        <p className="mt-1.5 text-reading text-muted-foreground">
          <Trans>Public threads under your documentation pages, in every state.</Trans>
        </p>
        {site && (
          <p className="mt-1 inline-flex items-center gap-1.5 text-body text-muted-foreground" data-testid="docs-site">
            <GlobeIcon className="size-4 text-faint" />
            <span dir="ltr">{hostOf(site)}</span>
          </p>
        )}
      </header>
      <DocsSitePicker className="hidden phone:flex" />
      <div className="flex flex-col gap-2.5">
        <Chips
          value={view}
          onChange={setView}
          label={t`Show`}
          testId="thread-view"
          items={THREAD_VIEWS.map((v) => ({ value: v, label: viewText[v] }))}
        />
        <Chips
          value={sort}
          onChange={setSort}
          label={t`Sort`}
          testId="thread-sort"
          items={THREAD_SORTS.map((s) => ({ value: s, label: sortText[s] }))}
        />
      </div>
      <ThreadList filters={filters} showPage from="/docs/discussions" keyboard empty={empty[view]} />
    </Pane>
  )
}

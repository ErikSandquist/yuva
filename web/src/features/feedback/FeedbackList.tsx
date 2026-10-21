import { Trans, useLingui } from "@lingui/react/macro"
import { CheckIcon, MessageSquareHeartIcon, PaperclipIcon, ReplyIcon, SearchIcon, XIcon } from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"

import { ContactAvatar, ErrorLine, isVisitor, Kbd, useContactName, VisitorAvatar } from "@/components/common"
import { formatShort } from "@/components/common/text"
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group"
import { Skeleton } from "@/components/ui/skeleton"
import { CategoryChip, ProgressChip, ScoreMark } from "@/features/conversation/Feedback"
import { IdeaMark } from "@/features/conversation/IdeaChip"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { InboxPicker } from "@/features/inbox/InboxPicker"
import { FilterChip, ListHead, UnderlineTabs } from "@/features/inbox/ListHead"
import { ProductTag } from "@/features/inbox/ProductTag"
import { useListCursor } from "@/features/inbox/listCursor"
import { setListQuery, useListSearch } from "@/features/inbox/listSearch"
import { useConversations, useCounts } from "@/features/inbox/queries"
import type { ConversationListItem, FeedbackCategory } from "@/lib/api"
import { cn } from "@/lib/utils"
import { useChannelMap } from "@/lib/workspace"

type Tab = "open" | "resolved"
type Category = FeedbackCategory | "all"

const CATEGORIES: Category[] = ["all", "bug", "idea", "praise", "other"]

let rememberedTab: Tab = "open"
let rememberedCategory: Category = "all"
let focused = 0

function remember(tab: Tab, category: Category) {
  rememberedTab = tab
  rememberedCategory = category
}

function useDebounced(value: string, ms: number) {
  const [v, setV] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setV(value), ms)
    return () => clearTimeout(id)
  }, [value, ms])
  return v
}

function Row({
  c,
  current,
  cursor,
  onOpen,
}: {
  c: ConversationListItem
  current: boolean
  cursor: boolean
  onOpen: () => void
}) {
  const { t, i18n } = useLingui()
  const channel = useChannelMap().get(c.channel_id ?? "")
  const contactName = useContactName()
  const visitor = isVisitor(c.contact.name, c.contact.email)
  const name = contactName(c, channel, c.contact.name, c.contact.email)
  const f = c.feedback
  const where = f?.page_url
    ? f.page_title || f.page_url
    : [f?.app_version, [f?.os, f?.os_version].filter(Boolean).join(" "), f?.device_model].filter(Boolean).join(" · ")
  const answered = c.last_message?.author_type === "member" || c.last_message?.author_type === "bot"
  return (
    <button
      type="button"
      onClick={onOpen}
      className={cn(
        "flex w-full items-start gap-3 border-b px-4 py-3 text-start outline-none transition-colors",
        current ? "bg-muted" : "hover:bg-muted/60",
        cursor && !current && "bg-muted/60",
        cursor && "shadow-[inset_3px_0_0_var(--brand)] rtl:shadow-[inset_-3px_0_0_var(--brand)]"
      )}
      aria-current={current || undefined}
      data-testid="feedback-row"
      data-conversation-id={c.id}
      data-unread={c.unread || undefined}
    >
      {f && <CategoryChip category={f.category} className="mt-px w-20 justify-center rounded-full border py-px" />}
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className={cn("line-clamp-2 text-body", c.unread ? "font-semibold" : "font-medium")}>
          {f?.text || c.subject}
        </span>
        <span className="flex min-w-0 items-center gap-1.5 text-caption text-faint">
          <ProductTag id={c.inbox_id} />
          <span className="flex shrink-0 items-center gap-1 text-muted-foreground">
            {visitor ? (
              <VisitorAvatar className="size-4" />
            ) : (
              <ContactAvatar id={c.contact.id} name={name} className="size-4 text-[8px]" />
            )}
            <span className="max-w-28 truncate">{name}</span>
          </span>
          {where && (
            <>
              <span aria-hidden>·</span>
              <span className="min-w-0 truncate">{where}</span>
            </>
          )}
          {!!f?.attachment_count && (
            <span
              className="inline-flex shrink-0 items-center gap-0.5"
              title={t`Screenshots attached`}
              data-testid="feedback-attachments"
            >
              <PaperclipIcon className="size-3.5" aria-label={t`Screenshots attached`} />
              {f.attachment_count}
            </span>
          )}
          {answered && (
            <span className="inline-flex shrink-0 items-center gap-1 text-success" data-testid="feedback-answered">
              <ReplyIcon className="size-3.5" />
              <Trans>Answered</Trans>
            </span>
          )}
        </span>
        {(c.idea || f?.score) && (
          <span className="flex min-w-0 items-center gap-2 text-caption text-faint">
            <ScoreMark score={f?.score} />
            {c.idea && <IdeaMark idea={c.idea} className="max-w-full" />}
          </span>
        )}
      </span>
      <span className="flex shrink-0 flex-col items-end gap-1">
        <span className="flex items-center gap-1.5">
          {c.unread && (
            <span className="size-2 shrink-0 rounded-full bg-brand" aria-label={t`Unread`} data-testid="row-unread" />
          )}
          <time
            className={cn("text-caption", c.unread ? "font-medium text-brand" : "text-faint")}
            dateTime={c.last_activity_at}
          >
            {formatShort(c.last_activity_at, i18n.locale)}
          </time>
        </span>
        {f?.progress && <ProgressChip progress={f.progress} />}
      </span>
    </button>
  )
}

export function FeedbackList({ currentId, onOpen }: { currentId: string | null; onOpen: (id: string) => void }) {
  const { t, i18n } = useLingui()
  const { query, focus } = useListSearch()
  const q = useDebounced(query.trim(), 250)
  const [inboxId] = useInboxFilter()
  const [tab, setTabState] = useState<Tab>(rememberedTab)
  const [category, setCategoryState] = useState<Category>(rememberedCategory)
  const search = useRef<HTMLInputElement>(null)
  const scroller = useRef<HTMLDivElement>(null)
  const sentinel = useRef<HTMLDivElement>(null)
  const [nearEnd, setNearEnd] = useState(false)

  const counts = useCounts().data
  const list = useConversations({
    kind: ["feedback"],
    status: tab === "resolved" ? ["closed"] : ["open", "pending"],
    category: category === "all" ? undefined : category,
    inbox_id: inboxId || undefined,
    q: q || undefined,
  })
  const loaded = useMemo(() => list.data?.pages.flatMap((p) => p.items) ?? [], [list.data])
  const rows = loaded
  const { cursorId, setCursorId, moveCursor, openCursor } = useListCursor(rows, currentId, onOpen, true, scroller)

  useEffect(() => {
    if (focus <= focused) return
    focused = focus
    search.current?.focus()
  }, [focus])

  useEffect(() => {
    const el = sentinel.current
    const root = scroller.current
    if (!el || !root || typeof IntersectionObserver === "undefined") return
    const io = new IntersectionObserver((entries) => setNearEnd(entries.some((e) => e.isIntersecting)), {
      root,
      rootMargin: "400px",
    })
    io.observe(el)
    return () => io.disconnect()
  }, [])
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = list
  useEffect(() => {
    if (nearEnd && hasNextPage && !isFetchingNextPage) void fetchNextPage()
  }, [nearEnd, hasNextPage, isFetchingNextPage, fetchNextPage, loaded.length])

  const reset = () => {
    setCursorId(null)
    scroller.current?.scrollTo({ top: 0 })
  }
  const setTab = (next: Tab) => {
    remember(next, category)
    setTabState(next)
    reset()
  }
  const setCategory = (next: Category) => {
    remember(tab, next)
    setCategoryState(next)
    reset()
  }

  const fmt = new Intl.NumberFormat(i18n.locale)
  const categoryLabels: Record<Category, string> = {
    all: t`All`,
    bug: t`Bug`,
    idea: t`Idea`,
    praise: t`Praise`,
    other: t`Other`,
  }
  const open = (k: Category) =>
    k === "all" ? (counts?.feedback ?? 0) : (counts?.feedback_categories.find((x) => x.category === k)?.count ?? 0)

  return (
    <nav aria-label={t`Feedback`} className="flex h-full min-h-0 flex-col bg-background" data-testid="feedback-list">
      <ListHead
        title={<Trans>Feedback</Trans>}
        sub={<Trans>Bugs, ideas and praise sent from your apps and pages</Trans>}
      >
        <div data-testid="feedback-tabs">
          <UnderlineTabs
            value={tab}
            onChange={setTab}
            label={t`Status`}
            items={[
              {
                value: "open",
                label: <Trans>Open</Trans>,
                count: inboxId ? undefined : counts?.feedback,
                testId: "feedback-tab-open",
              },
              { value: "resolved", label: <Trans>Resolved</Trans>, testId: "feedback-tab-resolved" },
            ]}
          />
        </div>
      </ListHead>
      <div className="flex shrink-0 flex-col gap-2 border-b px-3 py-2">
        <div className="flex items-center gap-1.5">
          <InputGroup className="h-8 min-w-0 flex-1 rounded-lg border-transparent bg-muted shadow-none focus-within:border-border focus-within:bg-card dark:bg-muted">
            <InputGroupAddon>
              <SearchIcon className="size-[15px] text-faint" />
            </InputGroupAddon>
            <InputGroupInput
              ref={search}
              name="list-search"
              value={query}
              onChange={(e) => {
                setListQuery(e.target.value)
                setCursorId(null)
              }}
              onKeyDown={(e) => {
                if (e.key === "ArrowDown" || e.key === "ArrowUp") {
                  e.preventDefault()
                  moveCursor(e.key === "ArrowDown" ? 1 : -1)
                } else if (e.key === "Enter") {
                  e.preventDefault()
                  openCursor()
                } else if (e.key === "Escape") {
                  if (query) setListQuery("")
                  else e.currentTarget.blur()
                }
              }}
              placeholder={t`Search`}
              aria-label={t`Search feedback`}
              data-testid="list-search"
            />
            <InputGroupAddon align="inline-end">
              {query ? (
                <InputGroupButton size="icon-xs" onClick={() => setListQuery("")} aria-label={t`Clear the search`}>
                  <XIcon />
                </InputGroupButton>
              ) : (
                <Kbd className="phone:hidden">/</Kbd>
              )}
            </InputGroupAddon>
          </InputGroup>
          <InboxPicker className="max-w-36" />
        </div>
        <div
          className="flex flex-wrap gap-1.5"
          role="toolbar"
          aria-label={t`Category`}
          data-testid="feedback-categories"
        >
          {CATEGORIES.map((k) => (
            <FilterChip
              key={k}
              active={category === k}
              onClick={() => setCategory(k)}
              testId={`feedback-category-${k}`}
            >
              {categoryLabels[k]}
              {!inboxId && open(k) > 0 && <span className="tabular-nums opacity-60">{fmt.format(open(k))}</span>}
            </FilterChip>
          ))}
        </div>
      </div>
      <div ref={scroller} className="min-h-0 flex-1 overflow-y-auto" data-testid="list">
        {list.isPending ? (
          <div className="grid gap-1 p-3">
            {[0, 1, 2, 3].map((i) => (
              <div key={i} className="flex items-center gap-3 py-1.5">
                <Skeleton className="size-10 rounded-full" />
                <div className="flex flex-1 flex-col gap-2">
                  <Skeleton className="h-4 w-1/2" />
                  <Skeleton className="h-3 w-3/4" />
                </div>
              </div>
            ))}
          </div>
        ) : rows.length === 0 && !hasNextPage ? (
          <p className="px-6 py-12 text-center text-body text-faint" data-testid="list-empty">
            {q ? (
              <Trans>Nothing matches your search here.</Trans>
            ) : tab === "open" ? (
              <Trans>No open feedback.</Trans>
            ) : (
              <Trans>Nothing is resolved yet.</Trans>
            )}
          </p>
        ) : (
          rows.map((c) => (
            <Row key={c.id} c={c} current={c.id === currentId} cursor={c.id === cursorId} onOpen={() => onOpen(c.id)} />
          ))
        )}
        <div ref={sentinel} className="h-px" />
        {isFetchingNextPage && (
          <div className="flex items-center gap-3 px-3 py-2">
            <Skeleton className="size-10 rounded-full" />
            <Skeleton className="h-4 w-1/2" />
          </div>
        )}
        <ErrorLine error={list.error} className="px-4 py-2" />
      </div>
    </nav>
  )
}

export function EmptyFeedback() {
  const open = useCounts().data?.feedback
  const none = open === 0
  return (
    <div
      className="flex min-h-0 flex-1 flex-col items-center justify-center bg-surface px-8 text-center"
      data-testid="feedback-empty"
    >
      <div className="mb-5 grid size-16 place-items-center rounded-[20px] bg-brand-wash text-brand">
        {none ? <CheckIcon className="size-7" /> : <MessageSquareHeartIcon className="size-7" />}
      </div>
      <h1 className="text-title">
        {none ? <Trans>No open feedback.</Trans> : <Trans>Pick a piece of feedback</Trans>}
      </h1>
      <p className="mt-1.5 text-body text-muted-foreground">
        {none ? (
          <Trans>When someone sends feedback, it will be in the list.</Trans>
        ) : (
          <Trans>Choose one from the list, or move with ↑ ↓ and open it with Enter.</Trans>
        )}
      </p>
    </div>
  )
}

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

function Pill({
  active,
  onClick,
  children,
  testId,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
  testId: string
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={cn(
        "inline-flex h-7 shrink-0 items-center gap-1.5 rounded-full px-3 text-small font-medium transition-colors",
        active ? "bg-brand-wash text-brand" : "bg-muted text-muted-foreground hover:text-foreground"
      )}
      data-testid={testId}
    >
      {children}
    </button>
  )
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
        "group flex w-full items-center gap-3 ps-3 text-start outline-none transition-colors",
        current ? "bg-muted" : "hover:bg-muted/60",
        cursor && !current && "bg-muted/60",
        cursor && "shadow-[inset_3px_0_0_var(--brand)] rtl:shadow-[inset_-3px_0_0_var(--brand)]"
      )}
      aria-current={current || undefined}
      data-testid="feedback-row"
      data-conversation-id={c.id}
      data-unread={c.unread || undefined}
    >
      <span className="shrink-0 py-2.5">
        {visitor ? (
          <VisitorAvatar className="size-10" />
        ) : (
          <ContactAvatar id={c.contact.id} name={name} className="size-10" />
        )}
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5 border-b py-3 pe-3 group-last:border-transparent">
        <span className="flex min-w-0 items-center gap-2">
          {f && <CategoryChip category={f.category} />}
          <ScoreMark score={f?.score} />
          <span
            className={cn("min-w-0 flex-1 truncate text-small", c.unread ? "font-semibold" : "text-muted-foreground")}
          >
            {name}
          </span>
          <time
            className={cn("shrink-0 text-caption", c.unread ? "font-medium text-brand" : "text-faint")}
            dateTime={c.last_activity_at}
          >
            {formatShort(c.last_activity_at, i18n.locale)}
          </time>
        </span>
        <span className={cn("line-clamp-2 text-reading", c.unread ? "font-semibold" : "font-medium")}>
          {f?.text || c.subject}
        </span>
        <span className="flex min-w-0 items-center gap-2 text-caption text-faint">
          <span className="min-w-0 flex-1 truncate">{where}</span>
          {c.idea && <IdeaMark idea={c.idea} className="max-w-[45%]" />}
          {f?.progress && <ProgressChip progress={f.progress} />}
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
          {c.unread && (
            <span className="size-2.5 shrink-0 rounded-full bg-brand" aria-label={t`Unread`} data-testid="row-unread" />
          )}
        </span>
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
      <div className="flex h-15 shrink-0 items-center gap-2 ps-4 pe-3">
        <h1 className="min-w-0 flex-1 truncate text-title">
          <Trans>Feedback</Trans>
        </h1>
      </div>
      <div className="flex shrink-0 items-center gap-1 px-3 pb-2">
        <InputGroup className="h-9 flex-1 rounded-xl border-transparent bg-muted shadow-none focus-within:border-border focus-within:bg-card dark:bg-muted">
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
        <InboxPicker className="max-w-44" />
      </div>
      <div
        className="flex shrink-0 gap-1.5 px-3 pb-2"
        role="toolbar"
        aria-label={t`Status`}
        data-testid="feedback-tabs"
      >
        <Pill active={tab === "open"} onClick={() => setTab("open")} testId="feedback-tab-open">
          <Trans>Open</Trans>
        </Pill>
        <Pill active={tab === "resolved"} onClick={() => setTab("resolved")} testId="feedback-tab-resolved">
          <Trans>Resolved</Trans>
        </Pill>
      </div>
      <div
        className="flex shrink-0 flex-wrap gap-1.5 px-3 pb-2.5"
        role="toolbar"
        aria-label={t`Category`}
        data-testid="feedback-categories"
      >
        {CATEGORIES.map((k) => (
          <Pill key={k} active={category === k} onClick={() => setCategory(k)} testId={`feedback-category-${k}`}>
            {categoryLabels[k]}
            {open(k) > 0 && <span className="tabular-nums opacity-70">{fmt.format(open(k))}</span>}
          </Pill>
        ))}
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

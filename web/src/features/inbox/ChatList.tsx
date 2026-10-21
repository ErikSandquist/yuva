import { Trans, useLingui } from "@lingui/react/macro"
import {
  BookUserIcon,
  BotIcon,
  CheckIcon,
  ClockIcon,
  MailIcon,
  MegaphoneIcon,
  MailOpenIcon,
  PinIcon,
  PinOffIcon,
  CommandIcon,
  EllipsisVerticalIcon,
  FlameIcon,
  InboxIcon,
  KeyboardIcon,
  ListChecksIcon,
  MessageSquarePlusIcon,
  ReplyIcon,
  SearchIcon,
  SettingsIcon,
  XIcon,
} from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"
import { Link, useNavigate } from "react-router"

import { useShell } from "@/app/shell"
import { TeamMenu } from "@/app/TeamMenu"
import { UserMenu } from "@/app/UserMenu"
import {
  ContactAvatar,
  ErrorLine,
  ImportedChip,
  isVisitor,
  Kbd,
  MemberAvatar,
  placeOf,
  SourceBadge,
  sourceOf,
  toast,
  useContactName,
  VisitorAvatar,
} from "@/components/common"
import { keyLabel, mod, SHORTCUTS } from "@/components/common/ShortcutSheet"
import { formatShort, useErrorText } from "@/components/common/text"
import { Alert } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuTrigger } from "@/components/ui/context-menu"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group"
import { Skeleton } from "@/components/ui/skeleton"
import { NewConversationDialog } from "@/features/contact/ContactDetails"
import { ContactSearch } from "@/features/contact/ContactSearch"
import { useDraftText } from "@/features/conversation/actions"
import { useLiveIds } from "@/features/conversation/liveness"
import { Ticks } from "@/features/conversation/messages/MessageActions"
import { currentRating, RatingMark } from "@/features/conversation/rating"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { useListCursor } from "@/features/inbox/listCursor"
import { LinkIncidentDialog } from "@/features/status/LinkIncidentDialog"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { ImportBanner } from "@/features/migrations/ImportBanner"
import { InboxPicker } from "@/features/inbox/InboxPicker"
import { ListHead, UnderlineTabs } from "@/features/inbox/ListHead"
import { ProductTag, useProductTag } from "@/features/inbox/ProductTag"
import { useListActions } from "@/features/inbox/memberState"
import { setListQuery, useListSearch } from "@/features/inbox/listSearch"
import { SourceFilter, sourceFilters, useSourceFilter } from "@/features/inbox/SourceFilter"
import { useBulkUpdateConversations, useConversations, useCounts } from "@/features/inbox/queries"
import { useIsPhone } from "@/hooks/use-media-query"
import type { Contact, ConversationListItem } from "@/lib/api"
import type { ConversationFilters } from "@/lib/keys"
import { useSetAvailability } from "@/lib/availability"
import { useSession } from "@/lib/session"
import { useTyping } from "@/lib/typing"
import { cn } from "@/lib/utils"
import { useChannelMap, useInboxes, useMemberMap } from "@/lib/workspace"

export type Chip = "all" | "unread" | "mine" | "unassigned" | "snoozed" | "done"
const CHIPS: Chip[] = ["all", "unread", "mine", "unassigned", "snoozed", "done"]

const chipFilters: Record<Chip, ConversationFilters> = {
  all: {},
  unread: {},
  mine: { status: ["open"], assignee: "me" },
  unassigned: { status: ["open"], assignee: "unassigned" },
  snoozed: { status: ["snoozed"] },
  done: { status: ["closed"] },
}

let remembered: Chip = "all"
let focused = 0

function rememberChip(c: Chip) {
  remembered = c
}

function useDebounced(value: string, ms: number) {
  const [v, setV] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setV(value), ms)
    return () => clearTimeout(id)
  }, [value, ms])
  return v
}

function Preview({ c, current, receipts }: { c: ConversationListItem; current: boolean; receipts: boolean }) {
  const { t } = useLingui()
  const draft = useDraftText(c.id)
  const typing = useTyping(c.id).some((a) => a.type === "contact")
  if (typing) {
    return (
      <span className="truncate text-brand" data-testid="row-typing">
        <Trans>typing…</Trans>
      </span>
    )
  }
  if (draft && !current) {
    return (
      <span className="truncate" data-testid="row-draft">
        <span className="font-medium text-brand">
          <Trans>Draft:</Trans>
        </span>{" "}
        {draft}
      </span>
    )
  }
  const m = c.last_message
  if (!m) return <span className="truncate italic">{c.subject || t`No messages yet`}</span>
  return (
    <span className="flex min-w-0 items-center gap-1">
      {m.author_type === "bot" && <BotIcon className="size-3.5 shrink-0" aria-label={t`Bot`} data-testid="row-bot" />}
      {m.author_type === "member" && !receipts && (
        <ReplyIcon className="size-3.5 shrink-0" aria-label={t`Team reply`} />
      )}
      {(m.author_type === "member" || m.author_type === "bot") && receipts && (
        <Ticks at={m.created_at} readAt={c.last_read_by_contact_at} />
      )}
      <span className="truncate">{m.text || c.subject}</span>
    </span>
  )
}

export function RowMenuItems({
  c,
  Item,
  onDone,
}: {
  c: ConversationListItem
  Item: typeof ContextMenuItem
  onDone?: () => void
}) {
  const actions = useListActions()
  return (
    <>
      <Item
        onClick={() => {
          actions.pin(c.id, !c.pinned_at)
          onDone?.()
        }}
        data-testid="row-pin"
      >
        {c.pinned_at ? <PinOffIcon /> : <PinIcon />}
        {c.pinned_at ? <Trans>Unpin</Trans> : <Trans>Pin to the top</Trans>}
      </Item>
      <Item
        onClick={() => {
          if (c.unread) actions.markRead(c.id)
          else actions.markUnread(c.id)
          onDone?.()
        }}
        data-testid="row-mark-unread"
      >
        {c.unread ? <MailOpenIcon /> : <MailIcon />}
        {c.unread ? <Trans>Mark as read</Trans> : <Trans>Mark as unread</Trans>}
      </Item>
    </>
  )
}

function Row({
  c,
  current,
  cursor,
  live,
  selecting,
  checked,
  onOpen,
}: {
  c: ConversationListItem
  current: boolean
  cursor: boolean
  live: boolean
  selecting: boolean
  checked: boolean
  onOpen: () => void
}) {
  const { t, i18n } = useLingui()
  const channel = useChannelMap().get(c.channel_id ?? "")
  const members = useMemberMap()
  const assignee = c.assignee_id ? members.get(c.assignee_id) : undefined
  const contactName = useContactName()
  const visitor = isVisitor(c.contact.name, c.contact.email)
  const name = contactName(c, channel, c.contact.name, c.contact.email)
  const source = sourceOf(c, channel)
  const place = placeOf(c, channel)
  const rating = currentRating(c)
  const product = useProductTag()
  return (
    <ContextMenu>
      <ContextMenuTrigger
        render={<button type="button" />}
        onClick={onOpen}
        className={cn(
          "group flex w-full items-center gap-3 ps-4 text-start outline-none transition-colors",
          current ? "bg-muted" : "hover:bg-muted/60",
          cursor && !current && "bg-muted/60",
          cursor && "shadow-[inset_3px_0_0_var(--brand)] rtl:shadow-[inset_-3px_0_0_var(--brand)]"
        )}
        aria-current={current || undefined}
        aria-pressed={selecting ? checked : undefined}
        data-checked={checked || undefined}
        data-testid="list-row"
        data-conversation-id={c.id}
        data-unread={c.unread || undefined}
        data-pinned={c.pinned_at ? true : undefined}
      >
        {selecting && (
          <span
            aria-hidden
            className={cn(
              "grid size-5 shrink-0 place-items-center rounded-[6px] border",
              checked ? "border-brand bg-brand text-white" : "border-input bg-background"
            )}
            data-testid="row-check"
          >
            {checked && <CheckIcon className="size-3.5" strokeWidth={3} />}
          </span>
        )}
        <span className="relative shrink-0 self-start pt-3">
          {visitor ? (
            <VisitorAvatar className="size-9" />
          ) : (
            <ContactAvatar id={c.contact.id} name={name} className="size-9" />
          )}
          {live && (
            <span
              className="absolute -end-0.5 bottom-0 size-3 rounded-full border-2 border-background bg-success"
              title={t`Online now`}
              data-testid="row-live"
            />
          )}
        </span>
        <span className="flex min-w-0 flex-1 flex-col gap-0.5 border-b py-3 pe-4 group-last:border-transparent">
          <span className="flex min-w-0 items-baseline gap-2">
            <span className={cn("min-w-0 flex-1 truncate text-body", c.unread ? "font-semibold" : "font-medium")}>
              {name}
            </span>
            {c.pinned_at && (
              <PinIcon
                className="size-3.5 shrink-0 rotate-45 text-faint"
                aria-label={t`Pinned`}
                data-testid="row-pinned"
              />
            )}
            <time
              className={cn("shrink-0 text-caption", c.unread ? "font-medium text-brand" : "text-faint")}
              dateTime={c.last_activity_at}
            >
              {formatShort(c.last_activity_at, i18n.locale)}
            </time>
          </span>
          <span className="flex min-w-0 items-center gap-1.5 text-small text-muted-foreground">
            {c.status === "snoozed" && <ClockIcon className="size-3.5 shrink-0 text-faint" aria-label={t`Snoozed`} />}
            <span className="flex min-w-0 flex-1">
              <Preview c={c} current={current} receipts={channel?.kind === "chat" || channel?.kind === "app"} />
            </span>
            {c.unread && (
              <span className="size-2 shrink-0 rounded-full bg-brand" aria-label={t`Unread`} data-testid="row-unread" />
            )}
          </span>
          {(source || place || c.broadcast_id || c.imported || rating || assignee || product(c.inbox_id)) && (
            <span className="mt-1 flex min-w-0 items-center gap-1.5 text-caption text-faint" data-testid="row-source">
              <ProductTag id={c.inbox_id} />
              {c.imported && <ImportedChip at={c.imported.at} />}
              {source && <SourceBadge source={source} />}
              {c.broadcast_id && (
                <MegaphoneIcon
                  className="size-3.5 shrink-0 text-brand"
                  aria-label={t`Reply to a broadcast`}
                  data-testid="row-broadcast"
                />
              )}
              {place && <span className="min-w-0 truncate">{place}</span>}
              <span className="flex-1" />
              {rating && <RatingMark rating={rating.rating} comment={rating.comment} />}
              {assignee && (
                <span title={t`Assigned to ${assignee.name || assignee.email}`} data-testid="row-assignee">
                  <MemberAvatar name={assignee.name || assignee.email} className="size-[18px]" />
                </span>
              )}
            </span>
          )}
        </span>
      </ContextMenuTrigger>
      <ContextMenuContent className="min-w-52" data-testid="row-menu">
        <RowMenuItems c={c} Item={ContextMenuItem} />
      </ContextMenuContent>
    </ContextMenu>
  )
}

function AwayBanner() {
  const { me } = useSession()
  const set = useSetAvailability()
  if (me.person.availability !== "away") return null
  return (
    <Alert
      role="status"
      className="mx-3 mb-2 flex items-center gap-2 rounded-xl border-border bg-card py-2 ps-3 pe-2 text-small text-muted-foreground"
      data-testid="away-banner"
    >
      <ClockIcon className="size-3.5 shrink-0" />
      <span className="min-w-0 flex-1">
        <Trans>You are away. Live chat shows nobody available and notifications pause.</Trans>
      </span>
      <Button
        variant="outline"
        size="sm"
        className="shrink-0"
        onClick={() => set.mutate("auto")}
        disabled={set.isPending}
      >
        <Trans>I'm back</Trans>
      </Button>
    </Alert>
  )
}

function CleanupBanner({ onReview }: { onReview: (id: string) => void }) {
  const { t } = useLingui()
  const errorText = useErrorText()
  const [inboxId] = useInboxFilter()
  const counts = useCounts(["conversation"]).data
  const spam = useConversations({ status: ["open"], spam: true, kind: ["conversation"] }, !!counts?.spam)
  const bulk = useBulkUpdateConversations()
  const items = (spam.data?.pages.flatMap((p) => p.items) ?? []).filter((c) => !inboxId || c.inbox_id === inboxId)
  if (!counts?.spam || items.length === 0) return null
  const ids = items.slice(0, 100).map((c) => c.id)
  const closeAll = () =>
    bulk.mutate(
      { conversation_ids: ids, status: "closed" },
      {
        onSuccess: (r) => {
          const done = r.updated.map((c) => c.id)
          toast(t`${done.length} closed`, () =>
            bulk.mutate({ conversation_ids: done, status: "open" }, { onError: (e) => toast(errorText(e)) })
          )
        },
        onError: (e) => toast(errorText(e)),
      }
    )
  return (
    <Alert
      role="status"
      className="mx-3 mb-2 flex flex-wrap items-center gap-2 rounded-xl border-dashed border-border bg-card py-2 ps-3 pe-2 text-small"
      data-testid="cleanup-banner"
    >
      <span className="min-w-0 flex-1">
        <b className="font-medium">
          <Trans>Quick cleanup</Trans>
        </b>{" "}
        <span className="text-faint">
          <Trans>{items.length} marked as spam · no reply needed</Trans>
        </span>
      </span>
      <span className="flex gap-1">
        <Button variant="ghost" size="sm" onClick={() => onReview(items[0].id)}>
          <Trans>Review</Trans>
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={closeAll}
          disabled={bulk.isPending}
          data-testid="cleanup-close-all"
        >
          <Trans>Close all</Trans>
        </Button>
      </span>
    </Alert>
  )
}

function StartConversation({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const [contact, setContact] = useState<Contact | null>(null)
  return (
    <>
      <Dialog
        open={open && !contact}
        onOpenChange={(o) => {
          if (!o) onOpenChange(false)
        }}
      >
        <DialogContent data-testid="start-conversation">
          <DialogHeader>
            <DialogTitle>
              <Trans>Who is it for?</Trans>
            </DialogTitle>
          </DialogHeader>
          {open && <ContactSearch exclude="" onPick={setContact} />}
        </DialogContent>
      </Dialog>
      {contact && (
        <NewConversationDialog
          contact={contact}
          open={open}
          onOpenChange={(o) => {
            if (!o) {
              setContact(null)
              onOpenChange(false)
            }
          }}
        />
      )}
    </>
  )
}

function ListHeader({ onSelect, children }: { onSelect: () => void; children: React.ReactNode }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { membership, canManage } = useSession()
  const { openPalette, openShortcuts } = useShell()
  const [starting, setStarting] = useState(false)
  const phone = useIsPhone()
  const [inboxId] = useInboxFilter()
  const product = useInboxes().data?.find((i) => i.id === inboxId)?.name
  const menu = (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<Button variant="ghost" size="icon-sm" />}
        aria-label={t`Menu`}
        title={t`Menu`}
        data-testid="list-menu"
      >
        <EllipsisVerticalIcon />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-56">
        <DropdownMenuItem onClick={() => navigate("/contacts")} data-testid="open-contacts">
          <BookUserIcon />
          <Trans>Contacts</Trans>
          <DropdownMenuShortcut>{keyLabel(SHORTCUTS.contacts).join(" ")}</DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => navigate("/settings")}>
          <SettingsIcon />
          <Trans>Settings</Trans>
          <DropdownMenuShortcut>{mod} ,</DropdownMenuShortcut>
        </DropdownMenuItem>
        {canManage && (
          <DropdownMenuItem onClick={() => navigate("/setup/new")}>
            <InboxIcon />
            <Trans>Add an inbox</Trans>
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={onSelect} data-testid="select-conversations">
          <ListChecksIcon />
          <Trans>Select conversations</Trans>
          <DropdownMenuShortcut>{keyLabel(SHORTCUTS.select).join(" ")}</DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={openPalette}>
          <CommandIcon />
          <Trans>Everything</Trans>
          <DropdownMenuShortcut>{mod} K</DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem onClick={openShortcuts}>
          <KeyboardIcon />
          <Trans>Keyboard shortcuts</Trans>
          <DropdownMenuShortcut>?</DropdownMenuShortcut>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
  const start = <StartConversation open={starting} onOpenChange={setStarting} />
  if (phone) {
    return (
      <div className="flex shrink-0 flex-col border-b">
        <div className="flex h-15 shrink-0 items-center gap-1 ps-2 pe-1.5">
          <UserMenu align="start" />
          <span className="min-w-0 flex-1 truncate ps-1 text-title" data-testid="workspace-name">
            {membership.workspace.name}
          </span>
          <TeamMenu />
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => setStarting(true)}
            aria-label={t`New conversation`}
            title={t`New conversation`}
            data-testid="new-conversation"
          >
            <MessageSquarePlusIcon />
          </Button>
          <Button
            variant="ghost"
            size="icon-sm"
            render={<Link to="/settings" />}
            aria-label={t`Settings`}
            title={t`Settings (${mod},)`}
            data-testid="open-settings"
          >
            <SettingsIcon />
          </Button>
          {menu}
        </div>
        <div className="px-3 [&>[role=toolbar]]:mt-0">{children}</div>
        {start}
      </div>
    )
  }
  return (
    <ListHead
      title={<Trans>Inbox</Trans>}
      sub={product ? <Trans>Conversations in {product}</Trans> : <Trans>Conversations from every product</Trans>}
      action={
        <>
          <Button
            size="sm"
            onClick={() => setStarting(true)}
            title={t`New conversation`}
            data-testid="new-conversation"
          >
            <MessageSquarePlusIcon />
            <span className="@max-sm/list:sr-only">
              <Trans>New conversation</Trans>
            </span>
          </Button>
          {menu}
        </>
      }
    >
      {children}
      {start}
    </ListHead>
  )
}

export function ChatList({
  currentId,
  onOpen,
  active = true,
}: {
  currentId: string | null
  onOpen: (id: string) => void
  active?: boolean
}) {
  const { t } = useLingui()
  const { query, focus } = useListSearch()
  const q = useDebounced(query.trim(), 250)
  const [inboxId] = useInboxFilter()
  const [chip, setChipState] = useState<Chip>(remembered)
  const search = useRef<HTMLInputElement>(null)
  const scroller = useRef<HTMLDivElement>(null)
  const sentinel = useRef<HTMLDivElement>(null)
  const [nearEnd, setNearEnd] = useState(false)
  const [selecting, setSelecting] = useState(false)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [linking, setLinking] = useState(false)

  const [source] = useSourceFilter()
  const narrowed = !!source.source || !!source.site
  const filters = {
    kind: ["conversation" as const],
    ...chipFilters[chip],
    ...sourceFilters(source),
    inbox_id: inboxId || undefined,
    q: q || undefined,
  }
  const list = useConversations(filters)
  const counts = useCounts(["conversation"]).data
  const pinnedList = useConversations({ ...filters, pinned: true })
  const loaded = useMemo(() => list.data?.pages.flatMap((p) => p.items) ?? [], [list.data])
  const pinnedLoaded = useMemo(() => pinnedList.data?.pages.flatMap((p) => p.items) ?? [], [pinnedList.data])
  const keep = (c: ConversationListItem) => chip !== "unread" || c.unread
  const pinnedRows = [...pinnedLoaded, ...loaded.filter((c) => c.pinned_at && !pinnedLoaded.some((x) => x.id === c.id))]
    .filter((c) => c.pinned_at && keep(c))
    .sort((a, b) => (a.last_activity_at < b.last_activity_at ? 1 : -1))
  const mainRows = loaded.filter((c) => !c.pinned_at && keep(c))
  const rows = [...pinnedRows, ...mainRows]
  const live = useLiveIds(rows.filter((c) => c.status !== "closed"))
  const { cursorId, setCursorId, moveCursor, openCursor } = useListCursor(rows, currentId, onOpen, active, scroller)

  const toggle = (id: string) => {
    setSelecting(true)
    setSelected((s) => {
      const next = new Set(s)
      if (!next.delete(id)) next.add(id)
      return next
    })
  }
  const stopSelecting = () => {
    setSelecting(false)
    setSelected(new Set())
  }
  const linkKey = keyLabel(SHORTCUTS.linkSelected).join("")
  const chosen = rows.filter((c) => selected.has(c.id)).map((c) => c.id)
  const picked = chosen.length
  useHotkeys(
    {
      [SHORTCUTS.select]: () => {
        const id = cursorId ?? currentId
        if (id) toggle(id)
        else setSelecting(true)
      },
      ...(selecting
        ? {
            Escape: stopSelecting,
            ...(chosen.length > 0 ? { [SHORTCUTS.linkSelected]: () => setLinking(true) } : {}),
          }
        : {}),
    },
    active
  )

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

  const setChip = (c: Chip) => {
    rememberChip(c)
    setChipState(c)
    setCursorId(null)
    scroller.current?.scrollTo({ top: 0 })
  }

  const labels: Record<Chip, string> = {
    all: t`All`,
    unread: t`Unread`,
    mine: t`Mine`,
    unassigned: t`Unassigned`,
    snoozed: t`Snoozed`,
    done: t`Done`,
  }
  const empty: Record<Chip, React.ReactNode> = {
    all: <Trans>No conversations yet.</Trans>,
    unread: <Trans>You have read everything.</Trans>,
    mine: <Trans>Nothing is assigned to you.</Trans>,
    unassigned: <Trans>Every open conversation has someone.</Trans>,
    snoozed: <Trans>Nothing is snoozed.</Trans>,
    done: <Trans>Nothing is done yet.</Trans>,
  }

  return (
    <nav
      aria-label={t`Conversations`}
      className="@container/list flex h-full min-h-0 flex-col bg-background"
      data-testid="chat-list"
    >
      <ListHeader onSelect={() => setSelecting(true)}>
        {selecting ? (
          <div
            className="flex h-11.5 shrink-0 items-center gap-1.5"
            role="toolbar"
            aria-label={t`Selection`}
            data-testid="selection-bar"
          >
            <span className="me-auto min-w-0 truncate text-small font-medium" aria-live="polite">
              {chosen.length === 0 ? <Trans>Pick conversations</Trans> : <Trans>Selected: {picked}</Trans>}
            </span>
            <Button
              size="sm"
              disabled={chosen.length === 0}
              onClick={() => setLinking(true)}
              title={t`Link to an incident (${linkKey})`}
              data-testid="link-selected"
            >
              <FlameIcon />
              <Trans>Link to an incident</Trans>
            </Button>
            <Button variant="ghost" size="sm" onClick={stopSelecting} title={t`Clear the selection (Esc)`}>
              <Trans>Cancel</Trans>
            </Button>
          </div>
        ) : (
          <div data-testid="chips">
            <UnderlineTabs
              value={chip}
              onChange={setChip}
              label={t`Filter`}
              items={CHIPS.map((k) => ({
                value: k,
                label: labels[k],
                count: inboxId
                  ? undefined
                  : k === "mine"
                    ? counts?.mine
                    : k === "unassigned"
                      ? counts?.unassigned
                      : undefined,
                testId: `chip-${k}`,
              }))}
            />
          </div>
        )}
      </ListHeader>
      <div className="flex shrink-0 items-center gap-1.5 border-b px-3 py-2">
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
            aria-label={t`Search conversations`}
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
        <SourceFilter
          inboxId={inboxId}
          onChange={() => {
            setCursorId(null)
            scroller.current?.scrollTo({ top: 0 })
          }}
        />
      </div>
      <AwayBanner />
      <CleanupBanner onReview={onOpen} />
      <ImportBanner />
      <div ref={scroller} className="min-h-0 flex-1 overflow-y-auto" data-testid="list">
        {list.isPending ? (
          <div className="grid gap-1 p-3">
            {[0, 1, 2, 3].map((i) => (
              <div key={i} className="flex items-center gap-3 py-1.5">
                <Skeleton className="size-12 rounded-full" />
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
            ) : narrowed ? (
              <Trans>Nothing matches these filters.</Trans>
            ) : (
              empty[chip]
            )}
          </p>
        ) : (
          <>
            {pinnedRows.length > 0 && (
              <div
                className="flex items-center gap-1.5 px-4 pt-1 pb-0.5 text-caption font-medium text-faint"
                data-testid="pinned-heading"
              >
                <PinIcon className="size-3 rotate-45" />
                <Trans>Pinned</Trans>
              </div>
            )}
            {pinnedRows.map((c) => (
              <Row
                key={c.id}
                c={c}
                current={c.id === currentId}
                cursor={c.id === cursorId}
                live={live.has(c.id)}
                selecting={selecting}
                checked={selected.has(c.id)}
                onOpen={() => (selecting ? toggle(c.id) : onOpen(c.id))}
              />
            ))}
            {pinnedRows.length > 0 && mainRows.length > 0 && <div className="mt-1 border-t" role="separator" />}
            {mainRows.map((c) => (
              <Row
                key={c.id}
                c={c}
                current={c.id === currentId}
                cursor={c.id === cursorId}
                live={live.has(c.id)}
                selecting={selecting}
                checked={selected.has(c.id)}
                onOpen={() => (selecting ? toggle(c.id) : onOpen(c.id))}
              />
            ))}
          </>
        )}
        <div ref={sentinel} className="h-px" />
        {isFetchingNextPage && (
          <div className="flex items-center gap-3 px-3 py-2">
            <Skeleton className="size-12 rounded-full" />
            <Skeleton className="h-4 w-1/2" />
          </div>
        )}
        <ErrorLine error={list.error} className="px-4 py-2" />
      </div>
      {linking && (
        <LinkIncidentDialog conversationIds={chosen} onClose={() => setLinking(false)} onDone={stopSelecting} />
      )}
    </nav>
  )
}

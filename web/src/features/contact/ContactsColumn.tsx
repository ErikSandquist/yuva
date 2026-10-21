import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { BookUserIcon, SearchIcon, XIcon } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { Link, useMatch, useNavigate, useSearchParams } from "react-router"

import { ContactAvatar, ErrorLine, InboxBadges } from "@/components/common"
import { PaneEmpty } from "@/components/common/Column"
import { formatDateTime, formatShort } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group"
import { Skeleton } from "@/components/ui/skeleton"
import { useContactDirectory, type DirectoryFilter } from "@/features/contact/queries"
import { InboxPicker } from "@/features/inbox/InboxPicker"
import { ListHead, UnderlineTabs } from "@/features/inbox/ListHead"
import { contactName } from "@/features/contact/Section"
import { useDebounced } from "@/hooks/use-debounced"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useInboxes } from "@/lib/workspace"
import { useResetState } from "@/hooks/use-reset-state"
import type { Contact } from "@/lib/api"
import { cn } from "@/lib/utils"

function Row({ c, current, cursor }: { c: Contact; current: boolean; cursor: boolean }) {
  const { t, i18n } = useLingui()
  const name = contactName(c, t`Unnamed contact`)
  const known = c.emails.length > 0 || c.external_ids.length > 0
  const last = c.activity?.last_conversation_at
  return (
    <li>
      <Link
        to={`/contacts/${c.id}`}
        className={cn(
          "flex items-start gap-3 border-b px-4 py-3 outline-none transition-colors hover:bg-muted/60 focus-visible:bg-muted/60",
          current && "bg-muted hover:bg-muted",
          cursor && "shadow-[inset_3px_0_0_var(--brand)] rtl:shadow-[inset_-3px_0_0_var(--brand)]"
        )}
        aria-current={current ? "page" : undefined}
        data-testid="contact-row"
      >
        <ContactAvatar id={c.id} name={name} className="size-9 shrink-0" />
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="flex items-baseline gap-2">
            <span className="min-w-0 flex-1 truncate text-body font-medium">{name}</span>
            {last && (
              <time
                className="shrink-0 text-caption text-faint"
                dateTime={last}
                title={formatDateTime(last, i18n.locale)}
              >
                {formatShort(last, i18n.locale)}
              </time>
            )}
          </span>
          <span className="truncate text-small text-muted-foreground">
            {c.emails[0] && c.emails[0] !== name ? (
              c.emails[0]
            ) : known ? (
              (c.external_ids[0]?.external_id ?? "")
            ) : (
              <Trans>Visitor</Trans>
            )}
          </span>
          {(c.inboxes.length > 0 || (c.activity && c.activity.open_conversations > 0)) && (
            <span className="mt-1 flex min-w-0 items-center gap-2">
              <InboxBadges inboxes={c.inboxes} max={2} className="min-w-0 flex-1" />
              {c.activity && c.activity.open_conversations > 0 && (
                <span className="shrink-0 text-caption text-brand">
                  <Plural value={c.activity.open_conversations} one="# open" other="# open" />
                </span>
              )}
            </span>
          )}
        </span>
      </Link>
    </li>
  )
}

const FILTERS: DirectoryFilter[] = ["all", "open", "known", "visitor"]

export function ContactsColumn() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const currentId = useMatch("/contacts/:contactId")?.params.contactId
  const [q, setQ] = useState("")
  const query = useDebounced(q.trim(), 250)
  const [filter, setFilter] = useState<DirectoryFilter>("all")
  const [params, setParams] = useSearchParams()
  const inboxes = useInboxes()
  const picked = params.get("inbox") ?? ""
  const inboxId = inboxes.data?.some((i) => i.id === picked) ? picked : ""
  const directory = useContactDirectory(query, filter, inboxId)
  const items = directory.data?.pages.flatMap((p) => p.items) ?? []
  const [cursor, setCursor] = useResetState(-1, `${filter} ${query} ${inboxId}`)
  const list = useRef<HTMLUListElement>(null)

  useHotkeys({
    j: () => setCursor((i) => Math.min(items.length - 1, i + 1)),
    k: () => setCursor((i) => Math.max(0, i - 1)),
    ArrowDown: () => setCursor((i) => Math.min(items.length - 1, i + 1)),
    ArrowUp: () => setCursor((i) => Math.max(0, i - 1)),
    Enter: () => cursor >= 0 && items[cursor] && navigate(`/contacts/${items[cursor].id}`),
  })
  useEffect(() => {
    if (cursor >= 0)
      list.current?.querySelectorAll("[data-testid=contact-row]")[cursor]?.scrollIntoView({ block: "nearest" })
  }, [cursor])

  const labels: Record<DirectoryFilter, string> = { all: t`All`, open: t`Open`, known: t`Known`, visitor: t`Visitors` }
  const titles: Record<DirectoryFilter, string | undefined> = {
    all: undefined,
    open: t`Has an open conversation`,
    known: undefined,
    visitor: undefined,
  }

  return (
    <nav aria-label={t`Contacts`} className="flex h-full min-h-0 flex-col bg-background" data-testid="contacts-page">
      <ListHead title={<Trans>Contacts</Trans>} sub={<Trans>Everyone who has written to your products</Trans>}>
        <UnderlineTabs
          value={filter}
          onChange={setFilter}
          label={t`Show`}
          items={FILTERS.map((k) => ({ value: k, label: labels[k], title: titles[k], testId: `contacts-filter-${k}` }))}
        />
      </ListHead>
      <div className="flex shrink-0 items-center gap-1.5 border-b px-3 py-2">
        <InputGroup className="h-8 min-w-0 flex-1 rounded-lg border-transparent bg-muted shadow-none focus-within:border-border focus-within:bg-card dark:bg-muted">
          <InputGroupAddon>
            <SearchIcon className="size-[15px] text-faint" />
          </InputGroupAddon>
          <InputGroupInput
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Escape") {
                if (q) setQ("")
                else e.currentTarget.blur()
              }
              if (e.key === "ArrowDown") {
                e.preventDefault()
                e.currentTarget.blur()
                setCursor(0)
              }
              if (e.key === "Enter" && items[0]) {
                e.preventDefault()
                navigate(`/contacts/${items[cursor >= 0 ? cursor : 0].id}`)
              }
            }}
            placeholder={t`Name, e-mail or id`}
            aria-label={t`Search contacts`}
            data-testid="contacts-search"
          />
          {q && (
            <InputGroupAddon align="inline-end">
              <InputGroupButton size="icon-xs" onClick={() => setQ("")} aria-label={t`Clear the search`}>
                <XIcon />
              </InputGroupButton>
            </InputGroupAddon>
          )}
        </InputGroup>
        <InboxPicker
          className="max-w-36"
          value={inboxId}
          onChange={(id) =>
            setParams(
              (prev) => {
                const next = new URLSearchParams(prev)
                if (id) next.set("inbox", id)
                else next.delete("inbox")
                return next
              },
              { replace: true }
            )
          }
        />
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto pb-4">
        {directory.isPending ? (
          <div className="flex flex-col gap-2 p-2">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : directory.error ? (
          <ErrorLine error={directory.error} className="p-3" />
        ) : items.length === 0 ? (
          <p className="px-4 py-10 text-center text-body text-faint">
            {query ? (
              <Trans>No contact matches “{query}”.</Trans>
            ) : filter !== "all" || inboxId ? (
              <Trans>No contacts in this view.</Trans>
            ) : (
              <Trans>No contacts yet. They appear when someone writes in.</Trans>
            )}
          </p>
        ) : (
          <ul ref={list} className="flex flex-col">
            {items.map((c, i) => (
              <Row key={c.id} c={c} current={c.id === currentId} cursor={i === cursor} />
            ))}
          </ul>
        )}
        {directory.hasNextPage && (
          <div className="mt-2 flex justify-center">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void directory.fetchNextPage()}
              disabled={directory.isFetchingNextPage}
            >
              <Trans>Load more</Trans>
            </Button>
          </div>
        )}
      </div>
    </nav>
  )
}

export function ContactsHome() {
  return (
    <PaneEmpty icon={BookUserIcon} title={<Trans>Pick a contact</Trans>} testId="contacts-home">
      <Trans>
        Search by name, e-mail or the id your app gives them, then open one to see their conversations, details and
        notes.
      </Trans>
    </PaneEmpty>
  )
}

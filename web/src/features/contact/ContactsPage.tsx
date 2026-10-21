import { Trans, useLingui } from "@lingui/react/macro"
import { ChevronDownIcon, EllipsisIcon, PlusIcon, SearchIcon, XIcon } from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"
import { Link, useNavigate, useSearchParams } from "react-router"

import { ContactAvatar, ErrorLine, toast } from "@/components/common"
import { formatDate, useErrorText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { BeforeRows, CardSection, DetailRows, ProductRows, Seen } from "@/features/contact/ContactCard"
import { NewConversationDialog, useContactConversations } from "@/features/contact/ContactDetails"
import { MergeContactDialog } from "@/features/contact/MergeContactDialog"
import { useContact, useContactDirectory, useCreateContact } from "@/features/contact/queries"
import { contactName } from "@/features/contact/Section"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { Ago, FrameTabs, look, PageTop } from "@/features/inbox/PageFrame"
import { ProductTag } from "@/features/inbox/ProductTag"
import { useDebounced } from "@/hooks/use-debounced"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { Contact } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

type Tab = "all" | "recent" | "duplicates"

const GRID = "grid grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)_90px_110px_minmax(0,90px)] items-center gap-3 px-4"

export function countryOf(c: Contact, locale: string) {
  const attr = c.attributes?.country
  if (typeof attr === "string" && attr) return attr
  const region = c.locale?.split(/[-_]/)[1]
  if (!region) return undefined
  try {
    return new Intl.DisplayNames([locale], { type: "region" }).of(region.toUpperCase())
  } catch {
    return undefined
  }
}

const DISMISSED = "contacts-not-same"

function readDismissed(): string[] {
  try {
    const v = JSON.parse(localStorage.getItem(DISMISSED) ?? "[]")
    return Array.isArray(v) ? v.filter((x) => typeof x === "string") : []
  } catch {
    return []
  }
}

type Pair = { key: string; a: Contact; b: Contact; inbox?: string }

function useDuplicates(items: Contact[], dismissed: string[]) {
  const inboxes = useInboxes().data
  return useMemo(() => {
    const groups = new Map<string, Contact[]>()
    for (const c of items) {
      const n = c.name?.trim().toLocaleLowerCase()
      if (!n) continue
      groups.set(n, [...(groups.get(n) ?? []), c])
    }
    const pairs: Pair[] = []
    for (const g of groups.values()) {
      if (g.length < 2) continue
      const sorted = [...g].sort((x, y) => (y.activity?.conversations ?? 0) - (x.activity?.conversations ?? 0))
      const [a, ...rest] = sorted
      for (const b of rest) {
        const key = [a.id, b.id].sort().join(":")
        if (dismissed.includes(key)) continue
        const shared = a.inboxes.find((i) => b.inboxes.some((j) => j.id === i.id))
        pairs.push({ key, a, b, inbox: shared ? inboxes?.find((i) => i.id === shared.id)?.name : undefined })
      }
    }
    return pairs
  }, [items, dismissed, inboxes])
}

function ProductChip({ value, onChange }: { value: string; onChange: (id: string) => void }) {
  const { t } = useLingui()
  const inboxes = useInboxes().data ?? []
  const current = inboxes.find((i) => i.id === value)
  if (inboxes.length < 2) return null
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<button type="button" className={current ? look.chipOn : look.chipDash} />}
        aria-label={t`Filter by product`}
        data-testid="contacts-product"
      >
        {current ? (
          <>
            <span className="max-w-28 truncate">{current.name}</span>
            <ChevronDownIcon className="size-3.5" />
          </>
        ) : (
          <>
            <PlusIcon className="size-3.5" />
            <Trans>Product</Trans>
          </>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-44">
        <DropdownMenuRadioGroup value={value} onValueChange={(v) => onChange(String(v))}>
          <DropdownMenuRadioItem value="">
            <Trans>Every product</Trans>
          </DropdownMenuRadioItem>
          {inboxes.map((i) => (
            <DropdownMenuRadioItem key={i.id} value={i.id}>
              <span className="truncate">{i.name}</span>
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function Row({ c, current, cursor, search }: { c: Contact; current: boolean; cursor: boolean; search: string }) {
  const { t, i18n } = useLingui()
  const name = contactName(c, t`Unnamed contact`)
  const sub = c.emails[0] && c.emails[0] !== name ? c.emails[0] : (c.external_ids[0]?.external_id ?? "")
  const seen = c.activity?.last_seen_at
  return (
    <li className="border-t">
      <Link
        to={`/contacts/${c.id}${search}`}
        className={cn(
          GRID,
          "py-2.5 outline-none transition-colors hover:bg-surface focus-visible:bg-surface",
          current && "bg-surface shadow-[inset_2px_0_0_var(--brand)] rtl:shadow-[inset_-2px_0_0_var(--brand)]",
          cursor && !current && "bg-surface"
        )}
        aria-current={current ? "page" : undefined}
        data-testid="contact-row"
      >
        <span className="flex min-w-0 items-center gap-2.5">
          <ContactAvatar id={c.id} name={name} className="size-[30px] shrink-0 text-caption font-semibold" />
          <span className="flex min-w-0 flex-col leading-[1.3]">
            <span className="truncate font-medium">{name}</span>
            {sub ? (
              <span className="truncate text-caption text-faint">{sub}</span>
            ) : (
              <span className="truncate text-caption text-faint">
                <Trans>Visitor</Trans>
              </span>
            )}
          </span>
        </span>
        <span className="flex min-w-0 flex-wrap gap-1 overflow-hidden">
          {c.inboxes.slice(0, 3).map((i) => (
            <ProductTag key={i.id} id={i.id} always />
          ))}
        </span>
        <span className="tabular-nums">
          {c.activity ? new Intl.NumberFormat(i18n.locale).format(c.activity.conversations) : "—"}
        </span>
        <span className="truncate">{seen ? <Ago iso={seen} /> : <span className="text-faint">—</span>}</span>
        <span className="truncate">{countryOf(c, i18n.locale) ?? <span className="text-faint">—</span>}</span>
      </Link>
    </li>
  )
}

function Panel({ id }: { id: string }) {
  const { t, i18n } = useLingui()
  const { canManage } = useSession()
  const contact = useContact(id)
  const conversations = useContactConversations(id)
  const [merging, setMerging] = useState(false)
  const [starting, setStarting] = useState(false)
  const c = contact.data
  if (contact.isPending) {
    return (
      <div className="flex flex-col gap-3 p-5">
        <Skeleton className="size-12 rounded-full" />
        <Skeleton className="h-5 w-32" />
      </div>
    )
  }
  if (!c) return <ErrorLine error={contact.error} className="p-5" />
  const name = contactName(c, t`Unnamed contact`)
  return (
    <div className="flex flex-col" data-testid="contact-panel">
      <header className="flex flex-col items-start border-b px-5 pt-5 pb-4">
        <ContactAvatar id={c.id} name={name} className="size-12 text-[15px] font-semibold" />
        <h2 className="mt-3 max-w-full truncate text-[18px] leading-6 font-semibold" data-testid="contact-name">
          {name}
        </h2>
        {c.emails[0] && c.emails[0] !== name && (
          <span className="max-w-full truncate text-muted-foreground">{c.emails[0]}</span>
        )}
        <div className="mt-1">
          <Seen contactId={c.id} />
        </div>
        {c.blocked && (
          <span className="mt-1.5 rounded-md bg-destructive/10 px-2 py-0.5 text-caption text-destructive">
            <Trans>Blocked</Trans>
          </span>
        )}
        <div className="mt-3 flex gap-2">
          <Button className={look.btnPSm} onClick={() => setStarting(true)} data-testid="new-conversation-open">
            <Trans>Write</Trans>
          </Button>
          {canManage && (
            <Button
              variant="outline"
              className={look.btnSm}
              onClick={() => setMerging(true)}
              data-testid="merge-contact"
            >
              <Trans>Merge…</Trans>
            </Button>
          )}
          <DropdownMenu>
            <DropdownMenuTrigger
              render={<Button variant="outline" className={cn(look.btnSm, "w-[30px] px-0")} />}
              aria-label={t`More`}
              data-testid="contact-more"
            >
              <EllipsisIcon />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="min-w-48">
              <DropdownMenuItem render={<Link to={`/contacts/${c.id}?view=page`} />} data-testid="open-contact-page">
                <Trans>Open full page</Trans>
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => void navigator.clipboard?.writeText(c.id)}>
                <Trans>Copy contact id</Trans>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </header>
      {c.inboxes.length > 0 && (
        <CardSection title={<Trans>Products</Trans>} className="px-5">
          <ProductRows contact={c} />
        </CardSection>
      )}
      <CardSection title={<Trans>Attributes</Trans>} className="px-5">
        <DetailRows contact={c} />
        <div className="mt-2 flex items-baseline gap-3">
          <span className="text-muted-foreground">
            <Trans>First seen</Trans>
          </span>
          <span className="min-w-0 flex-1 truncate text-end">{formatDate(c.created_at, i18n.locale)}</span>
        </div>
      </CardSection>
      <CardSection title={<Trans>Conversations</Trans>} className="border-b-0 px-5">
        {conversations.isPending ? (
          <Skeleton className="h-10 w-full" />
        ) : (
          <BeforeRows items={conversations.items} dots limit={8} />
        )}
      </CardSection>
      {canManage && <MergeContactDialog target={c} open={merging} onOpenChange={setMerging} />}
      <NewConversationDialog contact={c} open={starting} onOpenChange={setStarting} />
    </div>
  )
}

function Side({ c }: { c: Contact }) {
  const { t } = useLingui()
  const name = contactName(c, t`Unnamed contact`)
  const products = c.inboxes.map((i) => i.name).join(", ")
  const n = c.activity?.conversations ?? 0
  return (
    <div className="flex min-w-0 flex-1 items-center gap-3 px-4 py-4">
      <ContactAvatar id={c.id} name={name} className="size-10 shrink-0 font-semibold" />
      <span className="flex min-w-0 flex-col leading-[1.35]">
        <Link to={`/contacts/${c.id}`} className="truncate font-semibold hover:underline">
          {name}
        </Link>
        <span className="truncate text-caption text-muted-foreground">{c.emails[0] ?? <Trans>no e-mail</Trans>}</span>
        <span className="truncate text-caption text-muted-foreground">
          {products ? `${products} · ` : ""}
          {n === 1 ? <Trans>1 conversation</Trans> : <Trans>{n} conversations</Trans>}
        </span>
      </span>
    </div>
  )
}

function PairCard({ pair, onDismiss }: { pair: Pair; onDismiss: () => void }) {
  const { canManage } = useSession()
  const [merging, setMerging] = useState(false)
  const shared = pair.inbox
  return (
    <li
      className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_220px] overflow-hidden rounded-xl border"
      data-testid="duplicate-pair"
    >
      <Side c={pair.a} />
      <div className="my-3 flex min-w-0 border-s border-dashed">
        <Side c={pair.b} />
      </div>
      <div className="flex flex-col justify-center gap-2.5 border-s px-4 py-3.5">
        <p className="text-caption text-muted-foreground">
          {shared ? <Trans>Same name, both wrote to {shared}.</Trans> : <Trans>Same name.</Trans>}
        </p>
        <div className="flex gap-2">
          {canManage && (
            <Button className={look.btnPSm} onClick={() => setMerging(true)} data-testid="duplicate-merge">
              <Trans>Merge</Trans>
            </Button>
          )}
          <Button variant="outline" className={look.btnSm} onClick={onDismiss} data-testid="duplicate-dismiss">
            <Trans>Not the same</Trans>
          </Button>
        </div>
      </div>
      {canManage && (
        <MergeContactDialog target={pair.a} initialSource={pair.b} open={merging} onOpenChange={setMerging} />
      )}
    </li>
  )
}

function AddContactDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const navigate = useNavigate()
  const errorText = useErrorText()
  const create = useCreateContact()
  const [name, setName] = useState("")
  const [email, setEmail] = useState("")
  const ready = !!name.trim() || !!email.trim()
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent data-testid="add-contact">
        <DialogHeader>
          <DialogTitle>
            <Trans>Add contact</Trans>
          </DialogTitle>
          <DialogDescription>
            <Trans>Someone who has not written yet. You can start a conversation with them next.</Trans>
          </DialogDescription>
        </DialogHeader>
        <form
          id="add-contact-form"
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault()
            if (!ready) return
            create.mutate(
              { name: name.trim() || undefined, emails: email.trim() ? [email.trim()] : undefined },
              {
                onSuccess: (c) => {
                  onOpenChange(false)
                  setName("")
                  setEmail("")
                  navigate(`/contacts/${c.id}`)
                },
                onError: (err) => toast(errorText(err)),
              }
            )
          }}
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="add-contact-name">
              <Trans>Name</Trans>
            </Label>
            <Input
              id="add-contact-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={200}
              className="h-9"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="add-contact-email">
              <Trans>E-mail</Trans>
            </Label>
            <Input
              id="add-contact-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="h-9"
            />
          </div>
        </form>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            <Trans>Cancel</Trans>
          </Button>
          <Button
            type="submit"
            form="add-contact-form"
            disabled={!ready || create.isPending}
            data-testid="add-contact-create"
          >
            <Trans>Add contact</Trans>
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function ContactsPage({ contactId }: { contactId?: string }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const tabParam = params.get("tab")
  const tab: Tab = tabParam === "recent" || tabParam === "duplicates" ? tabParam : "all"
  const [global] = useInboxFilter()
  const inboxes = useInboxes().data
  const picked = params.get("inbox") ?? ""
  const chosen = inboxes?.some((i) => i.id === picked) ? picked : ""
  const inboxId = chosen || global
  const product = inboxes?.find((i) => i.id === inboxId)?.name
  const q = params.get("q") ?? ""
  const setQ = (v: string) => set("q", v)
  const query = useDebounced(q.trim(), 250)
  const directory = useContactDirectory(query, "all", inboxId, tab === "recent" ? "last_seen" : "created")
  const everyone = useContactDirectory("", "all", inboxId, "last_seen")
  const items = useMemo(() => directory.data?.pages.flatMap((p) => p.items) ?? [], [directory.data])
  const all = useMemo(() => everyone.data?.pages.flatMap((p) => p.items) ?? [], [everyone.data])
  const [dismissed, setDismissed] = useState(readDismissed)
  const pairs = useDuplicates(all, dismissed)
  const [cursor, setCursor] = useState(-1)
  const [adding, setAdding] = useState(false)
  const list = useRef<HTMLUListElement>(null)
  const search = useRef<HTMLInputElement>(null)
  const suffix = params.toString() ? `?${params.toString()}` : ""
  const shown = tab === "duplicates" ? undefined : (contactId ?? items[0]?.id)

  const set = (key: string, value: string) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (value) next.set(key, value)
        else next.delete(key)
        return next
      },
      { replace: true }
    )

  const dismiss = (key: string) => {
    const next = [...dismissed, key]
    setDismissed(next)
    try {
      localStorage.setItem(DISMISSED, JSON.stringify(next))
    } catch {
      return
    }
  }

  useHotkeys(
    tab === "duplicates"
      ? {}
      : {
          j: () => setCursor((i) => Math.min(items.length - 1, i + 1)),
          k: () => setCursor((i) => Math.max(0, i - 1)),
          ArrowDown: () => setCursor((i) => Math.min(items.length - 1, i + 1)),
          ArrowUp: () => setCursor((i) => Math.max(0, i - 1)),
          Enter: () => cursor >= 0 && items[cursor] && navigate(`/contacts/${items[cursor].id}${suffix}`),
          "/": () => search.current?.focus(),
        }
  )
  useEffect(() => {
    if (cursor >= 0)
      list.current?.querySelectorAll("[data-testid=contact-row]")[cursor]?.scrollIntoView({ block: "nearest" })
  }, [cursor])

  return (
    <main className="flex min-h-0 flex-1 flex-col" data-testid="contacts-page">
      <PageTop
        title={<Trans>Contacts</Trans>}
        sub={
          product ? (
            <Trans>People who wrote to {product}</Trans>
          ) : (
            <Trans>Everyone who has written to your products</Trans>
          )
        }
        actions={
          <Button className={look.btnInk} onClick={() => setAdding(true)} data-testid="add-contact-open">
            <Trans>Add contact</Trans>
          </Button>
        }
        tabs={
          <FrameTabs<Tab>
            label={t`Show`}
            value={tab}
            onChange={(k) => set("tab", k === "all" ? "" : k)}
            items={[
              { key: "all", label: <Trans>All</Trans>, testId: "contacts-filter-all" },
              { key: "recent", label: <Trans>Recently active</Trans>, testId: "contacts-filter-recent" },
              {
                key: "duplicates",
                label: <Trans>Duplicates</Trans>,
                count: everyone.data && !everyone.hasNextPage ? pairs.length : undefined,
                testId: "contacts-filter-duplicates",
              },
            ]}
          />
        }
      />
      {tab === "duplicates" ? (
        <div className="min-h-0 flex-1 overflow-y-auto px-6 py-5" data-testid="duplicates">
          <p className="mb-3 text-muted-foreground">
            <Trans>
              These look like the same person. Merging keeps the left one and moves addresses, ids and conversations
              into it.
            </Trans>
          </p>
          {everyone.isPending ? (
            <Skeleton className="h-24 w-full max-w-[980px] rounded-xl" />
          ) : pairs.length === 0 ? (
            <p className="max-w-[980px] rounded-xl border border-dashed px-4 py-10 text-center text-faint">
              <Trans>No contacts look alike right now.</Trans>
            </p>
          ) : (
            <ul className="flex max-w-[980px] flex-col gap-4">
              {pairs.map((p) => (
                <PairCard key={p.key} pair={p} onDismiss={() => dismiss(p.key)} />
              ))}
            </ul>
          )}
          {everyone.hasNextPage && (
            <Button
              variant="ghost"
              size="sm"
              className="mt-3"
              onClick={() => void everyone.fetchNextPage()}
              disabled={everyone.isFetchingNextPage}
            >
              <Trans>Look through more contacts</Trans>
            </Button>
          )}
        </div>
      ) : (
        <div className="flex min-h-0 flex-1">
          <div className="min-h-0 min-w-0 flex-1 overflow-y-auto px-6 py-4">
            <div className="mb-3 flex flex-wrap items-center gap-2">
              <label className="relative flex h-[34px] w-[280px] max-w-full items-center">
                <SearchIcon className="pointer-events-none absolute start-2.5 size-3.5 text-faint" aria-hidden />
                <input
                  ref={search}
                  value={q}
                  onChange={(e) => {
                    setQ(e.target.value)
                    setCursor(-1)
                  }}
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
                      navigate(`/contacts/${items[cursor >= 0 ? cursor : 0].id}${suffix}`)
                    }
                  }}
                  placeholder={t`Search name, e-mail or id…`}
                  aria-label={t`Search contacts`}
                  className="size-full rounded-lg border border-input bg-background ps-8 pe-8 text-body outline-none placeholder:text-faint focus-visible:border-faint"
                  data-testid="contacts-search"
                />
                {q && (
                  <button
                    type="button"
                    onClick={() => setQ("")}
                    className="absolute end-2 text-faint hover:text-foreground"
                    aria-label={t`Clear the search`}
                  >
                    <XIcon className="size-3.5" />
                  </button>
                )}
              </label>
              <ProductChip value={chosen} onChange={(id) => set("inbox", id)} />
            </div>
            <div className={look.card}>
              <div className={cn(GRID, "h-[38px]", look.th)} aria-hidden>
                <span>
                  <Trans>Person</Trans>
                </span>
                <span>
                  <Trans>Products</Trans>
                </span>
                <span>
                  <Trans>Convers.</Trans>
                </span>
                <span>
                  <Trans>Last seen</Trans>
                </span>
                <span>
                  <Trans>Country</Trans>
                </span>
              </div>
              {directory.isPending ? (
                <div className="flex flex-col gap-2 border-t p-3">
                  <Skeleton className="h-10 w-full" />
                  <Skeleton className="h-10 w-full" />
                </div>
              ) : directory.error ? (
                <ErrorLine error={directory.error} className="border-t p-4" />
              ) : items.length === 0 ? (
                <p className="border-t px-4 py-10 text-center text-faint">
                  {query ? (
                    <Trans>No contact matches “{query}”.</Trans>
                  ) : inboxId ? (
                    <Trans>No contacts in this view.</Trans>
                  ) : (
                    <Trans>No contacts yet. They appear when someone writes in.</Trans>
                  )}
                </p>
              ) : (
                <ul ref={list} className="flex flex-col">
                  {items.map((c, i) => (
                    <Row key={c.id} c={c} current={c.id === shown} cursor={i === cursor} search={suffix} />
                  ))}
                </ul>
              )}
            </div>
            {directory.hasNextPage && (
              <div className="mt-3 flex justify-center">
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
          {shown && (
            <aside
              className="flex w-[340px] shrink-0 flex-col overflow-y-auto border-s"
              aria-label={t`Contact details`}
            >
              <Panel key={shown} id={shown} />
            </aside>
          )}
        </div>
      )}
      <AddContactDialog open={adding} onOpenChange={setAdding} />
    </main>
  )
}

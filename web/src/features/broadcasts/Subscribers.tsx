import { Trans, useLingui } from "@lingui/react/macro"
import { ChevronRightIcon, PlusIcon, SearchIcon, XIcon } from "lucide-react"
import { useState } from "react"

import { ErrorLine, Segmented } from "@/components/common"
import { formatDate, formatDateTime, SUBSCRIPTION_STATES, useEnumText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group"
import { Item } from "@/components/ui/item"
import { Skeleton } from "@/components/ui/skeleton"
import { AddSubscriberDialog } from "@/features/broadcasts/AddSubscriberDialog"
import { useSubscriptions } from "@/features/broadcasts/queries"
import { StateTag, SubscriberSheet } from "@/features/broadcasts/SubscriberSheet"
import { Card, EmptyRow, Rows, Section } from "@/features/settings/ui"
import { useDebounced } from "@/hooks/use-debounced"
import type { MailingList, Subscription, SubscriptionState } from "@/lib/api"
import { useSession } from "@/lib/session"

function SubscriberRow({ s, onOpen }: { s: Subscription; onOpen: () => void }) {
  const { i18n } = useLingui()
  const text = useEnumText()
  return (
    <li>
      <Item
        render={<button type="button" />}
        className="flex-nowrap gap-3 rounded-none px-4 py-2.5 [button]:hover:bg-muted"
        onClick={onOpen}
        data-testid="subscriber-row"
      >
        <span className="grid min-w-0 flex-1 items-center gap-x-4 gap-y-0.5 text-start sm:grid-cols-[minmax(0,1fr)_10rem_9rem_6.5rem]">
          <span className="flex min-w-0 flex-col">
            <span className="truncate text-body font-medium" dir="ltr">
              {s.email}
            </span>
            {s.contact.name && <span className="truncate text-small text-faint">{s.contact.name}</span>}
          </span>
          <span className="flex flex-wrap items-center gap-x-2 sm:contents">
            <span className="min-w-0">
              <StateTag state={s.state} />
            </span>
            <span className="truncate text-caption text-muted-foreground">{text.subscriptionSource[s.source]}</span>
            <time
              className="text-caption text-faint tabular-nums sm:text-end"
              dateTime={s.created_at}
              title={formatDateTime(s.created_at, i18n.locale)}
            >
              {formatDate(s.created_at, i18n.locale)}
            </time>
          </span>
        </span>
        <ChevronRightIcon className="size-4 shrink-0 text-faint rtl:rotate-180" />
      </Item>
    </li>
  )
}

export function Subscribers({ list }: { list: MailingList }) {
  const { t } = useLingui()
  const text = useEnumText()
  const { canManage } = useSession()
  const [state, setState] = useState<SubscriptionState | "">("")
  const [query, setQuery] = useState("")
  const q = useDebounced(query.trim(), 300)
  const [open, setOpen] = useState<string | null>(null)
  const [adding, setAdding] = useState(false)
  const subs = useSubscriptions(list.id, state, q)
  const items = subs.data?.pages.flatMap((p) => p.items) ?? []
  const filtered = !!state || !!q
  return (
    <Section
      title={<Trans>Subscribers</Trans>}
      action={
        canManage &&
        !list.archived_at && (
          <Button size="sm" onClick={() => setAdding(true)} data-testid="add-subscriber">
            <PlusIcon />
            <Trans>Add subscriber</Trans>
          </Button>
        )
      }
    >
      <div className="flex flex-wrap items-center gap-2">
        <div className="max-w-full overflow-x-auto">
          <Segmented
            label={t`Show`}
            value={state || "all"}
            onChange={(v) => setState(v === "all" ? "" : (v as SubscriptionState))}
            items={[
              { value: "all", label: t`All`, testId: "subscribers-all" },
              ...SUBSCRIPTION_STATES.map((s) => ({
                value: s,
                label: s === "pending" ? t`Waiting` : text.subscription[s],
                title: text.subscription[s],
                testId: `subscribers-${s}`,
              })),
            ]}
          />
        </div>
        <InputGroup className="h-9 min-w-48 flex-1 rounded-[10px] bg-card">
          <InputGroupAddon>
            <SearchIcon />
          </InputGroupAddon>
          <InputGroupInput
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Escape") {
                if (query) setQuery("")
                else e.currentTarget.blur()
              }
            }}
            placeholder={t`Address or name`}
            aria-label={t`Search subscribers`}
            data-testid="subscribers-search"
          />
          {query && (
            <InputGroupAddon align="inline-end">
              <InputGroupButton size="icon-xs" onClick={() => setQuery("")} aria-label={t`Clear the search`}>
                <XIcon />
              </InputGroupButton>
            </InputGroupAddon>
          )}
        </InputGroup>
      </div>
      <Card flush>
        {subs.isPending ? (
          <div className="flex flex-col gap-2 p-4">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : items.length === 0 ? (
          <EmptyRow>
            {filtered ? (
              <Trans>Nobody matches.</Trans>
            ) : (
              <Trans>No subscribers yet. Put the subscribe form on your site, or add people who already agreed.</Trans>
            )}
          </EmptyRow>
        ) : (
          <Rows data-testid="subscribers">
            {items.map((s) => (
              <SubscriberRow key={s.id} s={s} onOpen={() => setOpen(s.id)} />
            ))}
          </Rows>
        )}
        {subs.hasNextPage && (
          <div className="flex justify-center border-t p-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void subs.fetchNextPage()}
              disabled={subs.isFetchingNextPage}
            >
              <Trans>Load more</Trans>
            </Button>
          </div>
        )}
        <ErrorLine error={subs.error} className="px-4 pb-3" />
      </Card>
      <SubscriberSheet
        listId={list.id}
        subscriptionId={open}
        row={items.find((s) => s.id === open)}
        onClose={() => setOpen(null)}
      />
      {adding && (
        <AddSubscriberDialog
          list={list}
          onClose={() => setAdding(false)}
          onAdded={(s) => {
            setAdding(false)
            setOpen(s.id)
          }}
        />
      )}
    </Section>
  )
}

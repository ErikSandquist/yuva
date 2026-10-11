import "@/features/designs/i18n"

import { Trans, useLingui } from "@lingui/react/macro"
import { ArchiveIcon, LayoutTemplateIcon, PlusIcon, SearchIcon } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { Link, useNavigate } from "react-router"

import { Dot, ErrorLine } from "@/components/common"
import { formatRelative } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { designLink, useDesigns, useShowArchivedDesigns } from "@/features/designs/queries"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { useDebounced } from "@/hooks/use-debounced"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useResetState } from "@/hooks/use-reset-state"
import type { EmailDesignSummary } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

function Row({ d, cursor }: { d: EmailDesignSummary; cursor: boolean }) {
  const { i18n } = useLingui()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === d.inbox_id)
  return (
    <li>
      <Link
        to={designLink(d.id)}
        className={cn(
          "flex items-center gap-3 rounded-xl px-2.5 py-2.5 outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring",
          cursor && "ring-2 ring-ring"
        )}
        data-testid="design-row"
      >
        <span className="grid size-9 shrink-0 place-items-center rounded-lg bg-brand-wash text-brand">
          <LayoutTemplateIcon className="size-4" />
        </span>
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="truncate text-body font-medium">{d.name}</span>
          <span className="flex min-w-0 items-center gap-1.5 text-caption text-faint">
            {inbox && (
              <span className="inline-flex min-w-0 items-center gap-1">
                <Dot color={inbox.branding.color} className="size-[6px] shrink-0 rounded-[2px]" />
                <span className="truncate">{inbox.name}</span>
                <span aria-hidden>·</span>
              </span>
            )}
            <span className="shrink-0">{formatRelative(d.updated_at, i18n.locale)}</span>
            {d.archived_at && <ArchiveIcon className="size-3 shrink-0" />}
          </span>
        </span>
      </Link>
    </li>
  )
}

export default function DesignsList() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { canManage } = useSession()
  const [inboxId] = useInboxFilter()
  const [archived, setArchived] = useShowArchivedDesigns()
  const [q, setQ] = useState("")
  const query = useDebounced(q.trim(), 250)
  const designs = useDesigns(inboxId, query, archived)
  const items = designs.data?.pages.flatMap((p) => p.items) ?? []
  const [cursor, setCursor] = useResetState(-1, JSON.stringify([inboxId, archived, query]))
  const list = useRef<HTMLUListElement>(null)

  useHotkeys(
    {
      j: () => setCursor((i) => Math.min(items.length - 1, i + 1)),
      k: () => setCursor((i) => Math.max(0, i - 1)),
      ArrowDown: () => setCursor((i) => Math.min(items.length - 1, i + 1)),
      ArrowUp: () => setCursor((i) => Math.max(0, i - 1)),
      ...(cursor >= 0 && items[cursor] ? { Enter: () => navigate(designLink(items[cursor].id)) } : {}),
    },
    canManage
  )
  useEffect(() => {
    if (cursor >= 0)
      list.current?.querySelectorAll("[data-testid=design-row]")[cursor]?.scrollIntoView({ block: "nearest" })
  }, [cursor])

  if (!canManage)
    return (
      <p className="px-2.5 py-6 text-body text-faint" data-testid="designs-read-only">
        <Trans>Owners and admins make e-mail designs and send broadcasts.</Trans>
      </p>
    )

  return (
    <div className="flex flex-col" data-testid="designs-list">
      <div className="flex items-center gap-2 px-2.5 pt-1 pb-2">
        <h3 className="min-w-0 flex-1 text-small font-medium text-faint">
          <Trans>E-mail designs</Trans>
        </h3>
        <Button variant="outline" size="sm" render={<Link to="/broadcasts/designs/new" />} data-testid="new-design">
          <PlusIcon />
          <Trans>New design</Trans>
        </Button>
      </div>
      <div className="relative px-1 pb-2">
        <SearchIcon className="pointer-events-none absolute top-1/2 start-3.5 size-4 -translate-y-1/2 text-faint" />
        <Input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder={t`Search designs`}
          aria-label={t`Search designs`}
          className="ps-9"
          data-testid="designs-search"
        />
      </div>
      {designs.isPending ? (
        <div className="flex flex-col gap-2 p-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : designs.error ? (
        <ErrorLine error={designs.error} className="p-3" />
      ) : items.length === 0 ? (
        <div className="px-4 py-8 text-center text-body text-faint" data-testid="designs-empty">
          {query ? (
            <Trans>No designs match.</Trans>
          ) : archived ? (
            <Trans>No archived designs</Trans>
          ) : (
            <>
              <p className="mb-2 font-medium text-foreground">
                <Trans>No designs yet</Trans>
              </p>
              <p>
                <Trans>A design is the look of a mail: start from a template, then use it for your broadcasts.</Trans>
              </p>
            </>
          )}
        </div>
      ) : (
        <ul className="flex flex-col" ref={list}>
          {items.map((d, i) => (
            <Row key={d.id} d={d} cursor={i === cursor} />
          ))}
        </ul>
      )}
      {designs.hasNextPage && (
        <div className="mt-2 flex justify-center">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => void designs.fetchNextPage()}
            disabled={designs.isFetchingNextPage}
          >
            <Trans>Load more</Trans>
          </Button>
        </div>
      )}
      <Label className="mt-3 justify-between gap-3 px-2.5 py-2 font-normal text-muted-foreground">
        <Trans>Show archived designs</Trans>
        <Switch checked={archived} onCheckedChange={setArchived} data-testid="designs-archived" />
      </Label>
    </div>
  )
}

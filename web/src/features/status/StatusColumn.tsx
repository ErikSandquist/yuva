import { Trans, useLingui } from "@lingui/react/macro"
import { PlusIcon } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { Link, useMatch, useNavigate } from "react-router"

import { Dot, ErrorLine } from "@/components/common"
import { Column, ColumnHeading } from "@/components/common/Column"
import { keyLabel, STATUS_KEYS } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { InboxPicker } from "@/features/inbox/InboxPicker"
import { CreatePageDialog } from "@/features/status/CreatePageDialog"
import { statusLink, useStatusPages } from "@/features/status/queries"
import { StateDot, useStatusText } from "@/features/status/text"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useResetState } from "@/hooks/use-reset-state"
import type { Inbox, StatusPage } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

function PageRow({ p, current, cursor }: { p: StatusPage; current: boolean; cursor: boolean }) {
  const { t, i18n } = useLingui()
  const text = useStatusText()
  const subscribed = new Intl.NumberFormat(i18n.locale).format(p.email_subscribers.counts.subscribed)
  return (
    <li>
      <Link
        to={statusLink(p.id)}
        className={cn(
          "flex flex-col gap-1 rounded-xl px-2.5 py-2.5 outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring",
          current && "bg-muted",
          cursor && "ring-2 ring-ring"
        )}
        aria-current={current ? "page" : undefined}
        data-testid="status-row"
      >
        <span className="flex items-center gap-2">
          <span className="min-w-0 flex-1 truncate text-body font-medium">{p.name}</span>
          <span className="shrink-0 text-caption text-faint">
            {p.published ? <Trans>Published</Trans> : <Trans>Unpublished</Trans>}
          </span>
        </span>
        <span className="flex items-center gap-2 text-caption text-muted-foreground">
          <StateDot state={p.state} />
          <span className="min-w-0 flex-1 truncate">{text.component[p.state]}</span>
          {p.email_subscribers.enabled && (
            <span className="shrink-0 text-faint tabular-nums" title={t`${subscribed} subscribers`}>
              {subscribed}
            </span>
          )}
        </span>
      </Link>
    </li>
  )
}

export function StatusColumn() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { canManage } = useSession()
  const [inboxId] = useInboxFilter()
  const inboxQuery = useInboxes()
  const inboxes = inboxQuery.data ?? []
  const pages = useStatusPages(inboxId)
  const items = pages.data ?? []
  const shown = inboxId ? inboxes.filter((i) => i.id === inboxId) : inboxes
  const groups = shown.map((inbox) => ({ inbox, page: items.find((p) => p.inbox_id === inbox.id) }))
  const ordered = groups.flatMap((g) => (g.page ? [g.page] : []))
  const currentId = useMatch("/status-pages/:pageId/*")?.params.pageId
  const [creating, setCreating] = useState<Inbox | null>(null)
  const [cursor, setCursor] = useResetState(-1, inboxId)
  const list = useRef<HTMLDivElement>(null)
  const missing = canManage ? groups.find((g) => !g.page)?.inbox : undefined
  const newKey = keyLabel(STATUS_KEYS.incident).join("")

  useHotkeys({
    j: () => setCursor((i) => Math.min(ordered.length - 1, i + 1)),
    k: () => setCursor((i) => Math.max(0, i - 1)),
    ArrowDown: () => setCursor((i) => Math.min(ordered.length - 1, i + 1)),
    ArrowUp: () => setCursor((i) => Math.max(0, i - 1)),
    ...(missing && !currentId ? { [STATUS_KEYS.incident]: () => setCreating(missing) } : {}),
    ...(cursor >= 0 && ordered[cursor]
      ? {
          Enter: () => {
            navigate(statusLink(ordered[cursor].id))
            setCursor(-1)
          },
        }
      : {}),
  })
  useEffect(() => {
    if (cursor >= 0)
      list.current?.querySelectorAll("[data-testid=status-row]")[cursor]?.scrollIntoView({ block: "nearest" })
  }, [cursor])

  let index = 0
  return (
    <>
      <Column
        title={<Trans>Status</Trans>}
        testId="status-panel"
        actions={<InboxPicker className="max-w-40" />}
        bodyClassName="px-2"
      >
        <nav aria-label={t`Status pages`} className="flex flex-col" ref={list}>
          {pages.isPending || inboxQuery.isPending ? (
            <div className="flex flex-col gap-2 p-2">
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
            </div>
          ) : pages.error ? (
            <ErrorLine error={pages.error} className="p-3" />
          ) : groups.length === 0 ? (
            <p className="px-4 py-8 text-center text-body text-faint" data-testid="status-empty">
              <Trans>No inbox to show a status page for.</Trans>
            </p>
          ) : (
            groups.map(({ inbox, page }) => (
              <section key={inbox.id} className="flex flex-col" data-testid="status-inbox">
                <ColumnHeading>
                  <span className="flex items-center gap-1.5">
                    <Dot color={inbox.branding.color} className="size-[7px] rounded-[2px]" />
                    {inbox.name}
                  </span>
                </ColumnHeading>
                {page ? (
                  <ul className="flex flex-col">
                    <PageRow p={page} current={page.id === currentId} cursor={index++ === cursor} />
                  </ul>
                ) : canManage ? (
                  <div className="px-2.5 py-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setCreating(inbox)}
                      title={t`Create status page (${newKey})`}
                      data-testid="create-status-page-open"
                    >
                      <PlusIcon />
                      <Trans>Create status page</Trans>
                    </Button>
                  </div>
                ) : (
                  <p className="px-2.5 py-2 text-small text-faint" data-testid="status-none">
                    <Trans>No status page yet. Owners and admins create it.</Trans>
                  </p>
                )}
              </section>
            ))
          )}
        </nav>
      </Column>
      {creating && (
        <CreatePageDialog
          inboxId={creating.id}
          inboxName={creating.name}
          onClose={() => setCreating(null)}
          onCreated={(p) => {
            setCreating(null)
            navigate(statusLink(p.id, "settings"))
          }}
        />
      )}
    </>
  )
}

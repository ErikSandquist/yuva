import "@/features/changelog/i18n"
import { Trans, useLingui } from "@lingui/react/macro"
import { PlusIcon } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { Link, useMatch, useNavigate } from "react-router"

import { Dot } from "@/components/common"
import { Column, ColumnHeading } from "@/components/common/Column"
import { CHANGELOG_KEYS } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { CreateChangelogDialog } from "@/features/changelog/CreateChangelogDialog"
import { changelogLink, changelogOfInbox, useChangelogs } from "@/features/changelog/queries"
import { ErrorLine } from "@/features/changelog/text"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { InboxPicker } from "@/features/inbox/InboxPicker"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useResetState } from "@/hooks/use-reset-state"
import type { Changelog, Inbox } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

function ChangelogRow({ c, current, cursor }: { c: Changelog; current: boolean; cursor: boolean }) {
  const published = c.counts.published
  const total = c.counts.entries
  const scheduled = c.counts.scheduled
  return (
    <li>
      <Link
        to={changelogLink(c.id)}
        className={cn(
          "flex flex-col gap-1 rounded-xl px-2.5 py-2.5 outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring",
          current && "bg-muted",
          cursor && "ring-2 ring-ring"
        )}
        aria-current={current ? "page" : undefined}
        data-testid="changelog-row"
      >
        <span className="flex items-center gap-2">
          <span className="min-w-0 flex-1 truncate text-body font-medium">{c.name}</span>
          <span className="shrink-0 text-caption text-faint">
            {c.published ? <Trans>Published</Trans> : <Trans>Unpublished</Trans>}
          </span>
        </span>
        <span className="truncate text-caption text-muted-foreground">
          <Trans>
            {published} of {total} entries published
          </Trans>
          {scheduled > 0 && (
            <>
              {" · "}
              <Trans>Scheduled: {scheduled}</Trans>
            </>
          )}
        </span>
      </Link>
    </li>
  )
}

export default function ChangelogColumn() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { canManage } = useSession()
  const [inboxId] = useInboxFilter()
  const inboxQuery = useInboxes()
  const inboxes = inboxQuery.data ?? []
  const changelogs = useChangelogs()
  const shown = inboxId ? inboxes.filter((i) => i.id === inboxId) : inboxes
  const groups = shown.map((inbox) => ({ inbox, changelog: changelogOfInbox(changelogs.data, inbox.id) }))
  const ordered = groups.flatMap((g) => (g.changelog ? [g.changelog] : []))
  const currentId = useMatch("/updates/:changelogId/*")?.params.changelogId
  const [creating, setCreating] = useState<Inbox | null>(null)
  const [cursor, setCursor] = useResetState(-1, inboxId)
  const list = useRef<HTMLDivElement>(null)
  const missing = canManage ? groups.find((g) => !g.changelog)?.inbox : undefined

  useHotkeys({
    ...(!currentId
      ? {
          j: () => setCursor((i) => Math.min(ordered.length - 1, i + 1)),
          k: () => setCursor((i) => Math.max(0, i - 1)),
          ArrowDown: () => setCursor((i) => Math.min(ordered.length - 1, i + 1)),
          ArrowUp: () => setCursor((i) => Math.max(0, i - 1)),
        }
      : {}),
    ...(missing && !currentId ? { [CHANGELOG_KEYS.create]: () => setCreating(missing) } : {}),
    ...(cursor >= 0 && ordered[cursor] && !currentId
      ? {
          Enter: () => {
            navigate(changelogLink(ordered[cursor].id))
            setCursor(-1)
          },
        }
      : {}),
  })
  useEffect(() => {
    if (cursor >= 0)
      list.current?.querySelectorAll("[data-testid=changelog-row]")[cursor]?.scrollIntoView({ block: "nearest" })
  }, [cursor])

  let index = 0
  return (
    <>
      <Column
        title={<Trans>What’s new</Trans>}
        testId="changelog-panel"
        actions={<InboxPicker className="max-w-40" />}
        bodyClassName="px-2"
      >
        <nav aria-label={t`What’s new`} className="flex flex-col" ref={list}>
          {changelogs.isPending || inboxQuery.isPending ? (
            <div className="flex flex-col gap-2 p-2">
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
            </div>
          ) : changelogs.error ? (
            <ErrorLine error={changelogs.error} className="p-3" />
          ) : groups.length === 0 ? (
            <p className="px-4 py-8 text-center text-body text-faint" data-testid="changelog-empty">
              <Trans>No inbox to show a changelog for.</Trans>
            </p>
          ) : (
            groups.map(({ inbox, changelog }) => (
              <section key={inbox.id} className="flex flex-col" data-testid="changelog-inbox">
                <ColumnHeading>
                  <span className="flex items-center gap-1.5">
                    <Dot color={inbox.branding.color} className="size-[7px] rounded-[2px]" />
                    {inbox.name}
                  </span>
                </ColumnHeading>
                {changelog ? (
                  <ul className="flex flex-col">
                    <ChangelogRow c={changelog} current={changelog.id === currentId} cursor={index++ === cursor} />
                  </ul>
                ) : canManage ? (
                  <div className="px-2.5 py-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setCreating(inbox)}
                      data-testid="create-changelog-open"
                    >
                      <PlusIcon />
                      <Trans>Create changelog</Trans>
                    </Button>
                  </div>
                ) : (
                  <p className="px-2.5 py-2 text-small text-faint" data-testid="changelog-none">
                    <Trans>No changelog yet. Owners and admins create it.</Trans>
                  </p>
                )}
              </section>
            ))
          )}
        </nav>
      </Column>
      {creating && (
        <CreateChangelogDialog
          inboxId={creating.id}
          inboxName={creating.name}
          onClose={() => setCreating(null)}
          onCreated={(c) => {
            setCreating(null)
            navigate(changelogLink(c.id, "settings"))
          }}
        />
      )}
    </>
  )
}

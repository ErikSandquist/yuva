import "@/features/roadmap/i18n"
import { Trans, useLingui } from "@lingui/react/macro"
import { PlusIcon } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { Link, useMatch, useNavigate } from "react-router"

import { Dot, ErrorLine } from "@/components/common"
import { Column, ColumnHeading } from "@/components/common/Column"
import { keyLabel, ROADMAP_KEYS } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { InboxPicker } from "@/features/inbox/InboxPicker"
import { CreateBoardDialog } from "@/features/roadmap/CreateBoardDialog"
import { boardLink, boardOfInbox, useBoards } from "@/features/roadmap/queries"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useResetState } from "@/hooks/use-reset-state"
import type { Inbox, RoadmapBoard } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

function BoardRow({ b, current, cursor }: { b: RoadmapBoard; current: boolean; cursor: boolean }) {
  const { counts } = b
  const ideas = counts.ideas + counts.planned + counts.in_progress + counts.shipped
  const pending = counts.pending
  return (
    <li>
      <Link
        to={boardLink(b.id)}
        className={cn(
          "flex flex-col gap-1 rounded-xl px-2.5 py-2.5 outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring",
          current && "bg-muted",
          cursor && "ring-2 ring-ring"
        )}
        aria-current={current ? "page" : undefined}
        data-testid="board-row"
      >
        <span className="flex items-center gap-2">
          <span className="min-w-0 flex-1 truncate text-body font-medium">{b.name}</span>
          <span className="shrink-0 text-caption text-faint">
            {b.public ? <Trans>Public</Trans> : <Trans>Private</Trans>}
          </span>
        </span>
        <span className="truncate text-caption text-muted-foreground">
          <Trans>Ideas: {ideas}</Trans>
          {pending > 0 && (
            <>
              {" · "}
              <Trans>Waiting for approval: {pending}</Trans>
            </>
          )}
        </span>
      </Link>
    </li>
  )
}

export default function RoadmapColumn() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { canManage } = useSession()
  const [inboxId] = useInboxFilter()
  const inboxQuery = useInboxes()
  const inboxes = inboxQuery.data ?? []
  const boards = useBoards()
  const shown = inboxId ? inboxes.filter((i) => i.id === inboxId) : inboxes
  const groups = shown.map((inbox) => ({ inbox, board: boardOfInbox(boards.data, inbox.id) }))
  const ordered = groups.flatMap((g) => (g.board ? [g.board] : []))
  const currentId = useMatch("/board/:boardId/*")?.params.boardId
  const [creating, setCreating] = useState<Inbox | null>(null)
  const [cursor, setCursor] = useResetState(-1, inboxId)
  const list = useRef<HTMLDivElement>(null)
  const missing = canManage ? groups.find((g) => !g.board)?.inbox : undefined
  const newKey = keyLabel(ROADMAP_KEYS.newBoard).join("")

  useHotkeys({
    ...(!currentId
      ? {
          j: () => setCursor((i) => Math.min(ordered.length - 1, i + 1)),
          k: () => setCursor((i) => Math.max(0, i - 1)),
          ArrowDown: () => setCursor((i) => Math.min(ordered.length - 1, i + 1)),
          ArrowUp: () => setCursor((i) => Math.max(0, i - 1)),
        }
      : {}),
    ...(missing && !currentId ? { [ROADMAP_KEYS.newBoard]: () => setCreating(missing) } : {}),
    ...(cursor >= 0 && ordered[cursor] && !currentId
      ? {
          Enter: () => {
            navigate(boardLink(ordered[cursor].id))
            setCursor(-1)
          },
        }
      : {}),
  })
  useEffect(() => {
    if (cursor >= 0)
      list.current?.querySelectorAll("[data-testid=board-row]")[cursor]?.scrollIntoView({ block: "nearest" })
  }, [cursor])

  let index = 0
  return (
    <>
      <Column
        title={<Trans>Roadmap</Trans>}
        testId="roadmap-panel"
        actions={<InboxPicker className="max-w-40" />}
        bodyClassName="px-2"
      >
        <nav aria-label={t`Roadmap boards`} className="flex flex-col" ref={list}>
          {boards.isPending || inboxQuery.isPending ? (
            <div className="flex flex-col gap-2 p-2">
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
            </div>
          ) : boards.error ? (
            <ErrorLine error={boards.error} className="p-3" />
          ) : groups.length === 0 ? (
            <p className="px-4 py-8 text-center text-body text-faint" data-testid="roadmap-empty">
              <Trans>No inbox to show a board for.</Trans>
            </p>
          ) : (
            groups.map(({ inbox, board }) => (
              <section key={inbox.id} className="flex flex-col" data-testid="roadmap-inbox">
                <ColumnHeading>
                  <span className="flex items-center gap-1.5">
                    <Dot color={inbox.branding.color} className="size-[7px] rounded-[2px]" />
                    {inbox.name}
                  </span>
                </ColumnHeading>
                {board ? (
                  <ul className="flex flex-col">
                    <BoardRow b={board} current={board.id === currentId} cursor={index++ === cursor} />
                  </ul>
                ) : canManage ? (
                  <div className="px-2.5 py-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setCreating(inbox)}
                      title={t`Create board (${newKey})`}
                      data-testid="create-board-open"
                    >
                      <PlusIcon />
                      <Trans>Create board</Trans>
                    </Button>
                  </div>
                ) : (
                  <p className="px-2.5 py-2 text-small text-faint" data-testid="roadmap-none">
                    <Trans>No board yet. Owners and admins create it.</Trans>
                  </p>
                )}
              </section>
            ))
          )}
        </nav>
      </Column>
      {creating && (
        <CreateBoardDialog
          inboxId={creating.id}
          inboxName={creating.name}
          onClose={() => setCreating(null)}
          onCreated={(b) => {
            setCreating(null)
            navigate(boardLink(b.id, "settings"))
          }}
        />
      )}
    </>
  )
}

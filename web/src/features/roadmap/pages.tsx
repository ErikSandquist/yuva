import "@/features/roadmap/i18n"
import { Trans, useLingui } from "@lingui/react/macro"
import { ArrowUpRightIcon, KanbanIcon, PlusIcon, SettingsIcon } from "lucide-react"
import { useState } from "react"
import { Link, Navigate, useNavigate, useParams, useSearchParams } from "react-router"

import { ErrorLine, InboxMark } from "@/components/common"
import { PaneBack } from "@/components/common/Column"
import { keyLabel, ROADMAP_KEYS } from "@/components/common/ShortcutSheet"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { useDiscussion } from "@/features/docs/discussions/queries"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { Board } from "@/features/roadmap/Board"
import { CreateBoardDialog } from "@/features/roadmap/CreateBoardDialog"
import { IdeaPane } from "@/features/roadmap/IdeaPane"
import { NewIdeaDialog } from "@/features/roadmap/NewIdeaDialog"
import { boardLink, boardOfInbox, ideaLink, useBoard, useBoards, useUpdateBoard } from "@/features/roadmap/queries"
import { Settings } from "@/features/roadmap/Settings"
import { FrameButton, FrameTabs, PageFrame } from "@/features/status/PageFrame"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useMediaQuery } from "@/hooks/use-media-query"
import type { Discussion, IdeaStage, Inbox, RoadmapBoard } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

const TABS = ["board", "settings"] as const
type Tab = (typeof TABS)[number]

function BoardsMenu({
  boards,
  missing,
  onCreate,
}: {
  boards: RoadmapBoard[]
  missing?: Inbox
  onCreate: (i: Inbox) => void
}) {
  const { t } = useLingui()
  const inboxes = new Map((useInboxes().data ?? []).map((i) => [i.id, i]))
  const newKey = keyLabel(ROADMAP_KEYS.newBoard).join("")
  if (boards.length === 0 && !missing) return null
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<FrameButton aria-label={t`Roadmap boards`} title={t`Roadmap boards`} className="px-2.5" />}
        data-testid="roadmap-boards-menu"
      >
        <SettingsIcon />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-56">
        {boards.map((b) => {
          const inbox = inboxes.get(b.inbox_id)
          return (
            <DropdownMenuItem key={b.id} render={<Link to={boardLink(b.id)} />} data-testid="board-row">
              <InboxMark name={inbox?.name ?? b.name} color={inbox?.branding.color} className="size-4 text-[9px]" />
              <span className="flex-1 truncate">{b.name}</span>
              <span className="text-caption text-faint">
                {b.public ? <Trans>Public</Trans> : <Trans>Private</Trans>}
              </span>
            </DropdownMenuItem>
          )
        })}
        {missing && (
          <DropdownMenuItem onClick={() => onCreate(missing)} data-testid="create-board-open">
            <PlusIcon />
            <Trans>Create board</Trans>
            <DropdownMenuShortcut>{newKey}</DropdownMenuShortcut>
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function ViewBoard({ boards }: { boards: RoadmapBoard[] }) {
  const open = boards.filter((b) => b.public)
  if (open.length === 0) return null
  const label = (
    <>
      <Trans>View board</Trans>
      <ArrowUpRightIcon className="rtl:-scale-x-100" />
    </>
  )
  if (open.length === 1)
    return (
      <FrameButton
        render={<a href={open[0].url} target="_blank" rel="noreferrer noopener" />}
        data-testid="board-open-public"
      >
        {label}
      </FrameButton>
    )
  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<FrameButton />} data-testid="board-open-public">
        {label}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-48">
        {open.map((b) => (
          <DropdownMenuItem key={b.id} render={<a href={b.url} target="_blank" rel="noreferrer noopener" />}>
            {b.name}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function RoadmapHome() {
  const navigate = useNavigate()
  const { canManage } = useSession()
  const [inboxId] = useInboxFilter()
  const inboxes = useInboxes().data ?? []
  const boards = useBoards()
  const scoped = (boards.data ?? []).filter((b) => !inboxId || b.inbox_id === inboxId)
  const shown = inboxId ? inboxes.filter((i) => i.id === inboxId) : inboxes
  const missing = canManage && boards.data ? shown.find((i) => !boardOfInbox(boards.data, i.id)) : undefined
  const [creating, setCreating] = useState<Inbox | null>(null)
  const [writing, setWriting] = useState(false)
  const [moving, setMoving] = useState<{ idea: Discussion; to: IdeaStage } | null>(null)
  useHotkeys(missing ? { [ROADMAP_KEYS.newBoard]: () => setCreating(missing) } : {}, !creating && !writing)
  return (
    <PageFrame
      testId="roadmap-home"
      title={<Trans>Roadmap</Trans>}
      description={<Trans>Ideas your customers vote on, with the feedback behind them</Trans>}
      actions={
        <>
          <BoardsMenu boards={scoped} missing={missing} onCreate={setCreating} />
          <ViewBoard boards={scoped} />
          {scoped.length > 0 && (
            <FrameButton ink onClick={() => setWriting(true)} data-testid="new-idea">
              <Trans>New idea</Trans>
            </FrameButton>
          )}
        </>
      }
      bodyClassName="flex overflow-hidden in-data-frame:overflow-hidden"
    >
      {boards.isPending ? (
        <div className="flex flex-1 gap-4 p-6">
          <Skeleton className="h-64 flex-1 rounded-xl" />
          <Skeleton className="h-64 flex-1 rounded-xl" />
          <Skeleton className="h-64 flex-1 rounded-xl" />
          <Skeleton className="h-64 flex-1 rounded-xl" />
        </div>
      ) : boards.error ? (
        <ErrorLine error={boards.error} className="m-6" />
      ) : scoped.length === 0 ? (
        <div className="m-6 flex flex-1 flex-col items-center gap-3 self-start rounded-xl border border-dashed px-6 py-12 text-center">
          <KanbanIcon className="size-6 text-faint" />
          <p className="max-w-md text-body text-muted-foreground">
            <Trans>
              Show your customers what you are building, and link the feedback behind each idea. Create a board for an
              inbox to sort ideas into stages.
            </Trans>
          </p>
          {missing && (
            <FrameButton ink onClick={() => setCreating(missing)} data-testid="create-board-empty">
              <PlusIcon />
              <Trans>Create board</Trans>
            </FrameButton>
          )}
        </div>
      ) : (
        <Board boards={scoped} moving={moving} onMoving={setMoving} onMoved={() => undefined} />
      )}
      {writing && <NewIdeaDialog boards={scoped} onClose={() => setWriting(false)} />}
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
    </PageFrame>
  )
}

export function IdeaRedirect() {
  const { ideaId = "" } = useParams()
  const idea = useDiscussion(ideaId)
  const d = idea.data?.discussion
  if (d?.board_id) return <Navigate to={ideaLink(d.board_id, d.id)} replace />
  return idea.isPending ? (
    <div className="flex-1 bg-background" />
  ) : (
    <div className="flex flex-1 flex-col bg-background p-6">
      <PaneBack to="/board" label="" />
      <ErrorLine error={idea.error} />
    </div>
  )
}

export function BoardPane() {
  const { t } = useLingui()
  const { boardId = "", ideaId } = useParams()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const { canManage } = useSession()
  const boardQuery = useBoard(boardId)
  const patch = useUpdateBoard(boardId)
  const board = boardQuery.data
  const inbox = (useInboxes().data ?? []).find((i) => i.id === board?.inbox_id)
  const split = useMediaQuery("(min-width: 1180px)")
  const [moving, setMoving] = useState<{ idea: Discussion; to: IdeaStage } | null>(null)
  const [writing, setWriting] = useState(false)
  const raw = params.get("tab")
  const tab: Tab = raw === "settings" && canManage ? "settings" : "board"
  const setTab = (v: Tab) => navigate(boardLink(boardId, v === "board" ? undefined : v), { replace: true })

  useHotkeys(
    {
      [ROADMAP_KEYS.board]: () => setTab("board"),
      ...(canManage ? { [ROADMAP_KEYS.settings]: () => setTab("settings") } : {}),
      ...(canManage && board ? { [ROADMAP_KEYS.publicBoard]: () => patch.mutate({ public: !board.public }) } : {}),
      ...(board?.public && !ideaId
        ? { [ROADMAP_KEYS.openPage]: () => window.open(board.url, "_blank", "noopener,noreferrer") }
        : {}),
    },
    !!board
  )

  if (boardQuery.isPending) {
    return (
      <div className="flex flex-1 flex-col gap-4 bg-background p-6">
        <Skeleton className="h-8 w-60" />
        <Skeleton className="h-40 w-full rounded-2xl" />
      </div>
    )
  }
  if (!board) {
    return (
      <div className="flex flex-1 flex-col bg-background p-6">
        <PaneBack to="/board" label={t`Roadmap`} />
        {boardQuery.error ? (
          <ErrorLine error={boardQuery.error} />
        ) : (
          <p className="text-body text-muted-foreground">
            <Trans>This board no longer exists or you cannot see it.</Trans>
          </p>
        )}
      </div>
    )
  }

  const showIdea = !!ideaId && tab === "board"
  const showBoard = tab === "board" && (!showIdea || split)
  const publicKey = keyLabel(ROADMAP_KEYS.publicBoard).join("")
  const idea = showIdea && (
    <aside
      className={cn(
        "min-h-0 overflow-y-auto bg-background px-6 pt-4 pb-24 phone:px-4",
        split ? "w-[min(520px,45%)] shrink-0 border-s" : "mx-auto w-full max-w-[640px]"
      )}
      data-testid="idea-pane"
    >
      <IdeaPane board={board} ideaId={ideaId} key={ideaId} />
    </aside>
  )
  if (showIdea && !split)
    return (
      <main className="flex min-h-0 min-w-0 flex-1 flex-col bg-background" data-testid="roadmap-board" key={board.id}>
        {idea}
      </main>
    )
  return (
    <PageFrame
      testId="roadmap-board"
      key={board.id}
      back={{ to: "/board", label: t`Roadmap` }}
      title={
        <span className="flex items-center gap-2.5">
          {inbox && <InboxMark name={inbox.name} color={inbox.branding.color} className="size-6 text-caption" />}
          <span data-testid="board-name">{board.name}</span>
        </span>
      }
      description={
        <span data-testid="board-visibility">
          {board.public ? <Trans>Public</Trans> : <Trans>Private, only your team sees it</Trans>}
        </span>
      }
      actions={
        <>
          <ViewBoard boards={[board]} />
          {canManage && (
            <FrameButton
              disabled={patch.isPending}
              onClick={() => patch.mutate({ public: !board.public })}
              title={board.public ? t`Make private (${publicKey})` : t`Make public (${publicKey})`}
              data-testid="board-toggle-public"
            >
              {board.public ? <Trans>Make private</Trans> : <Trans>Make public</Trans>}
            </FrameButton>
          )}
          <FrameButton ink onClick={() => setWriting(true)} data-testid="new-idea">
            <Trans>New idea</Trans>
          </FrameButton>
        </>
      }
      notice={<ErrorLine error={patch.error} />}
      tabs={
        canManage && (
          <FrameTabs
            label={t`Roadmap`}
            current={tab}
            items={[
              { key: "board", to: boardLink(board.id), label: t`Board`, testId: "board-tab-board" },
              {
                key: "settings",
                to: boardLink(board.id, "settings"),
                label: t`Settings`,
                testId: "board-tab-settings",
              },
            ]}
          />
        )
      }
      bodyClassName={tab === "board" ? "flex overflow-hidden" : undefined}
    >
      {tab === "settings" && canManage && (
        <div className="pt-4">
          <Settings board={board} />
        </div>
      )}
      {showBoard && (
        <Board
          boards={[board]}
          ideaId={ideaId}
          moving={moving}
          onMoving={setMoving}
          onMoved={(idea, stage) => {
            if (stage === "shipped" && (idea.linked_count ?? 0) > 0 && !ideaId)
              navigate(ideaLink(board.id, idea.id), { state: { offerReply: true } })
          }}
        />
      )}
      {idea}
      {writing && <NewIdeaDialog boards={[board]} onClose={() => setWriting(false)} />}
    </PageFrame>
  )
}

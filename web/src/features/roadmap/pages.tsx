import "@/features/roadmap/i18n"
import { Trans, useLingui } from "@lingui/react/macro"
import { ExternalLinkIcon, KanbanIcon } from "lucide-react"
import { useState } from "react"
import { Navigate, useNavigate, useParams, useSearchParams } from "react-router"

import { Dot, ErrorLine, Segmented } from "@/components/common"
import { PaneBack, PaneEmpty } from "@/components/common/Column"
import { keyLabel, ROADMAP_KEYS } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useDiscussion } from "@/features/docs/discussions/queries"
import { Board } from "@/features/roadmap/Board"
import { IdeaPane } from "@/features/roadmap/IdeaPane"
import { boardLink, ideaLink, useBoard, useUpdateBoard } from "@/features/roadmap/queries"
import { Settings } from "@/features/roadmap/Settings"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useMediaQuery } from "@/hooks/use-media-query"
import type { Discussion, IdeaStage } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

const TABS = ["board", "settings"] as const
type Tab = (typeof TABS)[number]

export function RoadmapHome() {
  return (
    <PaneEmpty icon={KanbanIcon} title={<Trans>Roadmap</Trans>} testId="roadmap-home">
      <Trans>
        Show your customers what you are building, and link the feedback behind each idea. Pick an inbox’s board to sort
        ideas into stages.
      </Trans>
    </PaneEmpty>
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
  return (
    <main className="flex min-h-0 min-w-0 flex-1 flex-col bg-background" data-testid="roadmap-board" key={board.id}>
      {(!showIdea || split) && (
        <header className="flex flex-col gap-3 px-6 pt-6 pb-4 phone:px-4 phone:pt-3">
          <div>
            <PaneBack to="/board" label={t`Roadmap`} />
            <div className="flex flex-wrap items-start justify-between gap-3">
              <h1 className="min-w-0 text-page break-words" data-testid="board-name">
                {board.name}
              </h1>
              <div className="flex flex-wrap items-center gap-2">
                {board.public && (
                  <Button
                    variant="outline"
                    size="sm"
                    render={<a href={board.url} target="_blank" rel="noreferrer noopener" />}
                    data-testid="board-open-public"
                  >
                    <ExternalLinkIcon />
                    <Trans>View board</Trans>
                  </Button>
                )}
                {canManage && (
                  <Button
                    variant={board.public ? "outline" : "default"}
                    size="sm"
                    disabled={patch.isPending}
                    onClick={() => patch.mutate({ public: !board.public })}
                    title={board.public ? t`Make private (${publicKey})` : t`Make public (${publicKey})`}
                    data-testid="board-toggle-public"
                  >
                    {board.public ? <Trans>Make private</Trans> : <Trans>Make public</Trans>}
                  </Button>
                )}
              </div>
            </div>
            <p className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-body text-muted-foreground">
              {inbox && (
                <>
                  <span className="inline-flex items-center gap-1.5">
                    <Dot color={inbox.branding.color} className="size-[7px] rounded-[2px]" />
                    {inbox.name}
                  </span>
                  <span aria-hidden>·</span>
                </>
              )}
              <span data-testid="board-visibility">
                {board.public ? <Trans>Public</Trans> : <Trans>Private, only your team sees it</Trans>}
              </span>
            </p>
            <ErrorLine error={patch.error} className="mt-2" />
          </div>
          {canManage && (
            <div className="-mx-1 -my-3 max-w-[calc(100%+0.5rem)] overflow-x-auto px-1 py-1">
              <Segmented
                value={tab}
                onChange={setTab}
                label={t`Roadmap`}
                items={[
                  { value: "board", label: t`Board`, testId: "board-tab-board" },
                  { value: "settings", label: t`Settings`, testId: "board-tab-settings" },
                ]}
              />
            </div>
          )}
        </header>
      )}
      {tab === "settings" && canManage && (
        <div className="min-h-0 flex-1 overflow-y-auto pt-4">
          <Settings board={board} />
        </div>
      )}
      <div className={cn("flex min-h-0 flex-1", !showBoard && !showIdea && "hidden")}>
        {showBoard && (
          <Board
            board={board}
            ideaId={ideaId}
            moving={moving}
            onMoving={setMoving}
            onMoved={(idea, stage) => {
              if (stage === "shipped" && (idea.linked_count ?? 0) > 0 && !ideaId)
                navigate(ideaLink(board.id, idea.id), { state: { offerReply: true } })
            }}
          />
        )}
        {showIdea && (
          <aside
            className={cn(
              "min-h-0 overflow-y-auto bg-background px-6 pt-4 pb-24 phone:px-4",
              split ? "w-[min(520px,45%)] shrink-0 border-s" : "mx-auto w-full max-w-[640px]"
            )}
            data-testid="idea-pane"
          >
            <IdeaPane board={board} ideaId={ideaId} key={ideaId} />
          </aside>
        )}
      </div>
    </main>
  )
}

import {
  DndContext,
  PointerSensor,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core"
import { Trans, useLingui } from "@lingui/react/macro"
import {
  ArrowBigUpIcon,
  ArrowRightLeftIcon,
  CheckIcon,
  GripVerticalIcon,
  HourglassIcon,
  MessageSquareHeartIcon,
  MessageSquareIcon,
} from "lucide-react"
import { useEffect, useState } from "react"
import { Link, useNavigate } from "react-router"

import { ErrorLine, toast } from "@/components/common"
import { AuthorName } from "@/features/docs/discussions/parts"
import { ROADMAP_KEYS } from "@/components/common/ShortcutSheet"
import { IDEA_STAGES, useEnumText, useErrorText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { useModerate } from "@/features/docs/discussions/queries"
import { MoveDialog } from "@/features/roadmap/MoveDialog"
import { ideaLink, useBoardIdeas, useIdeas } from "@/features/roadmap/queries"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { Discussion, IdeaStage, RoadmapBoard } from "@/lib/api"
import { cn } from "@/lib/utils"

type Move = { idea: Discussion; to: IdeaStage }

const STAGE_DOT: Record<IdeaStage, string> = {
  ideas: "bg-faint",
  planned: "bg-blue-600 dark:bg-blue-400",
  in_progress: "bg-warning",
  shipped: "bg-success",
  closed: "bg-muted-foreground/40",
}

function Card({
  idea,
  board,
  current,
  cursor,
  onMove,
}: {
  idea: Discussion
  board: string
  current: boolean
  cursor: boolean
  onMove: (to: IdeaStage) => void
}) {
  const { t, i18n } = useLingui()
  const text = useEnumText()
  const { setNodeRef, listeners, attributes, isDragging } = useDraggable({ id: idea.id, data: { idea } })
  const fmt = new Intl.NumberFormat(i18n.locale)
  const [votes, linked, replies] = [idea.votes, idea.linked_count ?? 0, idea.reply_count].map((n) => fmt.format(n))
  const pending = idea.pending_posts ?? 0
  const title = idea.title
  return (
    <li
      ref={setNodeRef}
      className={cn(
        "group flex items-start gap-1 rounded-xl border bg-card py-2.5 ps-1 pe-1.5 transition-colors hover:border-faint",
        current && "border-brand",
        cursor && "ring-2 ring-ring",
        isDragging && "opacity-40"
      )}
      data-testid="idea-card"
      data-idea-id={idea.id}
    >
      <button
        type="button"
        className="grid h-6 w-5 shrink-0 cursor-grab touch-none place-items-center rounded-md text-faint outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring active:cursor-grabbing"
        aria-label={t`Drag ${title} to another stage`}
        {...attributes}
        {...listeners}
      >
        <GripVerticalIcon className="size-4" />
      </button>
      <span
        className="flex w-10 shrink-0 flex-col items-center rounded-lg border bg-background py-1 leading-tight tabular-nums"
        title={t`${votes} votes`}
        data-testid="idea-card-votes"
      >
        <ArrowBigUpIcon className="size-3.5 text-faint" aria-hidden />
        <span className="sr-only">{t`${votes} votes`}</span>
        <span className="text-body font-semibold" aria-hidden>
          {votes}
        </span>
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-1.5 ps-1.5">
        <Link
          to={ideaLink(board, idea.id)}
          className="line-clamp-3 text-body font-medium break-words outline-none hover:underline focus-visible:underline"
          data-testid="idea-card-link"
        >
          {idea.title}
        </Link>
        <span className="flex flex-wrap items-center gap-x-3 gap-y-1 text-caption text-muted-foreground tabular-nums">
          <span className="inline-flex items-center gap-1" title={t`${replies} replies`}>
            <MessageSquareIcon className="size-3.5" />
            <span className="sr-only">{t`${replies} replies`}</span>
            <span aria-hidden>{replies}</span>
          </span>
          <span
            className={cn("inline-flex items-center gap-1", (idea.linked_count ?? 0) === 0 && "opacity-45")}
            title={t`${linked} linked feedback`}
            data-testid="idea-card-linked"
          >
            <MessageSquareHeartIcon className="size-3.5" />
            <span className="sr-only">{t`${linked} linked feedback`}</span>
            <span aria-hidden>{linked}</span>
          </span>
          {pending > 0 && (
            <span
              className="inline-flex items-center gap-1 font-medium text-warning"
              title={t`${pending} waiting for approval`}
              data-testid="idea-card-pending"
            >
              <HourglassIcon className="size-3.5" />
              {pending}
            </span>
          )}
        </span>
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="ghost" size="icon-xs" className="shrink-0 text-faint" />}
          aria-label={t`Move ${title} to a stage`}
          title={t`Move to a stage`}
          data-testid="idea-card-stage"
        >
          <ArrowRightLeftIcon />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-44">
          {IDEA_STAGES.map((s) => (
            <DropdownMenuItem key={s} disabled={s === idea.stage} onClick={() => onMove(s)}>
              {text.stage[s]}
              {s === idea.stage && <CheckIcon className="ms-auto" />}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </li>
  )
}

type Ideas = ReturnType<typeof useIdeas>

function StageColumn({
  board,
  stage,
  list,
  items,
  ideaId,
  cursorId,
  onMove,
}: {
  board: RoadmapBoard
  stage: IdeaStage
  list: Ideas
  items: Discussion[]
  ideaId?: string
  cursorId: string | null
  onMove: (m: Move) => void
}) {
  const text = useEnumText()
  const { setNodeRef, isOver } = useDroppable({ id: stage })
  const count = board.counts[stage]
  return (
    <section
      ref={setNodeRef}
      className={cn(
        "flex min-w-56 flex-1 basis-0 snap-start flex-col gap-2 rounded-2xl border bg-surface p-2 transition-colors phone:min-w-[82vw]",
        stage === "closed" && "opacity-80",
        isOver && "bg-brand-wash"
      )}
      aria-label={text.stage[stage]}
      data-testid="stage-column"
      data-stage={stage}
    >
      <h2 className="flex items-center gap-2 px-1.5 pt-1 pb-0.5 text-body font-semibold">
        <span aria-hidden className={cn("size-2 shrink-0 rounded-full", STAGE_DOT[stage])} />
        <span className="min-w-0 flex-1 truncate">{text.stage[stage]}</span>
        <span className="text-caption text-faint tabular-nums" data-testid="stage-count">
          {count}
        </span>
      </h2>
      {list.isPending ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-16 rounded-xl" />
          <Skeleton className="h-16 rounded-xl" />
        </div>
      ) : (
        <ul className="flex min-h-12 flex-col gap-2">
          {items.map((idea) => (
            <Card
              key={idea.id}
              idea={idea}
              board={board.id}
              current={idea.id === ideaId}
              cursor={idea.id === cursorId}
              onMove={(to) => onMove({ idea, to })}
            />
          ))}
        </ul>
      )}
      {list.hasNextPage && (
        <Button variant="ghost" size="sm" onClick={() => void list.fetchNextPage()} disabled={list.isFetchingNextPage}>
          <Trans>Load more</Trans>
        </Button>
      )}
      <ErrorLine error={list.error} className="px-1.5 text-caption" />
    </section>
  )
}

function Pending({ board }: { board: RoadmapBoard }) {
  const { t } = useLingui()
  const errorText = useErrorText()
  const list = useIdeas({ board_id: board.id, state: "pending" })
  const moderate = useModerate()
  const items = list.data?.pages.flatMap((p) => p.items) ?? []
  if (items.length === 0) return null
  return (
    <section
      className="flex flex-col gap-3 rounded-2xl border p-3 ps-4"
      aria-label={t`Waiting for approval`}
      data-testid="pending-strip"
    >
      <h2 className="flex items-center gap-2.5 text-body">
        <span className="grid h-5.5 min-w-5.5 place-items-center rounded-full bg-brand px-1.5 text-caption font-semibold text-white tabular-nums">
          {items.length}
        </span>
        <span className="font-medium">
          <Trans>Readers suggested new ideas. Review them before they show on the public board.</Trans>
        </span>
      </h2>
      <ul className="flex gap-2 overflow-x-auto pb-1">
        {items.map((idea) => (
          <li
            key={idea.id}
            className="flex w-64 shrink-0 flex-col gap-1.5 rounded-xl border bg-card p-2.5"
            data-testid="pending-idea"
          >
            <Link
              to={ideaLink(board.id, idea.id)}
              className="line-clamp-2 text-body font-medium break-words hover:underline"
            >
              {idea.title}
            </Link>
            <span className="truncate text-caption text-faint">
              <AuthorName author={idea.author} className="font-normal" />
            </span>
            <span className="-mx-1 flex gap-0.5">
              <Button
                variant="ghost"
                size="sm"
                className="text-success hover:text-success"
                disabled={moderate.isPending}
                onClick={() =>
                  moderate.mutate(
                    { postId: idea.opening_post_id, to: "published" },
                    { onSuccess: () => toast(t`Approved`), onError: (e) => toast(errorText(e)) }
                  )
                }
                data-testid="pending-approve"
              >
                <CheckIcon />
                <Trans>Approve</Trans>
              </Button>
              <Button
                variant="ghost"
                size="sm"
                disabled={moderate.isPending}
                onClick={() =>
                  moderate.mutate(
                    { postId: idea.opening_post_id, to: "hidden" },
                    { onSuccess: () => toast(t`Hidden`), onError: (e) => toast(errorText(e)) }
                  )
                }
              >
                <Trans>Hide</Trans>
              </Button>
            </span>
          </li>
        ))}
      </ul>
    </section>
  )
}

export function Board({
  board,
  ideaId,
  moving,
  onMoving,
  onMoved,
}: {
  board: RoadmapBoard
  ideaId?: string
  moving: Move | null
  onMoving: (m: Move | null) => void
  onMoved: (idea: Discussion, stage: IdeaStage) => void
}) {
  const navigate = useNavigate()
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }))
  const lists = useBoardIdeas(board.id)
  const columns = IDEA_STAGES.map((stage) => lists[stage].data?.pages.flatMap((p) => p.items) ?? [])
  const [pos, setPos] = useState<{ col: number; row: number } | null>(null)
  const size = (col: number) => columns[col]?.length ?? 0
  const cursorIdea = pos ? columns[pos.col]?.[pos.row] : undefined
  const cursorId = cursorIdea?.id ?? null
  const step = (dc: number, dr: number) => {
    if (!pos) {
      const first = IDEA_STAGES.findIndex((_, i) => size(i) > 0)
      if (first >= 0) setPos({ col: first, row: 0 })
      return
    }
    if (dc === 0) {
      const len = size(pos.col)
      if (len > 0) setPos({ col: pos.col, row: Math.min(Math.max(pos.row + dr, 0), len - 1) })
      return
    }
    for (let i = 1; i <= IDEA_STAGES.length; i++) {
      const col = (pos.col + dc * i + IDEA_STAGES.length * i) % IDEA_STAGES.length
      if (size(col) > 0) return setPos({ col, row: Math.min(pos.row, size(col) - 1) })
    }
  }
  useEffect(() => {
    if (cursorId)
      document.querySelector(`[data-idea-id="${cursorId}"]`)?.scrollIntoView({ block: "nearest", inline: "nearest" })
  }, [cursorId])

  const onDragEnd = (e: DragEndEvent) => {
    const idea = e.active.data.current?.idea as Discussion | undefined
    const to = e.over?.id as IdeaStage | undefined
    if (idea && to && IDEA_STAGES.includes(to) && to !== idea.stage) onMoving({ idea, to })
  }

  const down = () => step(0, 1)
  const up = () => step(0, -1)
  const shift = (d: number) => {
    const to = cursorIdea && IDEA_STAGES[IDEA_STAGES.indexOf(cursorIdea.stage ?? "ideas") + d]
    if (cursorIdea && to) onMoving({ idea: cursorIdea, to })
  }
  useHotkeys(
    {
      j: down,
      k: up,
      ArrowDown: down,
      ArrowUp: up,
      [ROADMAP_KEYS.nextColumn]: () => step(1, 0),
      [ROADMAP_KEYS.previousColumn]: () => step(-1, 0),
      ...(cursorIdea
        ? {
            [ROADMAP_KEYS.open]: () => navigate(ideaLink(board.id, cursorIdea.id)),
            [ROADMAP_KEYS.move]: () => onMoving({ idea: cursorIdea, to: cursorIdea.stage ?? "ideas" }),
            [ROADMAP_KEYS.moveBack]: () => shift(-1),
            [ROADMAP_KEYS.moveForward]: () => shift(1),
          }
        : {}),
    },
    !ideaId && !moving
  )

  return (
    <>
      <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3 overflow-y-auto px-6 pb-6 phone:px-4">
        <Pending board={board} />
        <DndContext sensors={sensors} onDragEnd={onDragEnd}>
          <div
            className="flex min-h-0 flex-1 snap-x items-start gap-3 overflow-x-auto pb-3"
            data-testid="board-columns"
          >
            {IDEA_STAGES.map((stage, i) => (
              <StageColumn
                key={stage}
                board={board}
                stage={stage}
                list={lists[stage]}
                items={columns[i] ?? []}
                ideaId={ideaId}
                cursorId={cursorId}
                onMove={onMoving}
              />
            ))}
          </div>
        </DndContext>
      </div>
      {moving && (
        <MoveDialog
          idea={moving.idea}
          to={moving.to}
          onClose={() => onMoving(null)}
          onMoved={(stage) => {
            onMoving(null)
            onMoved(moving.idea, stage)
          }}
        />
      )}
    </>
  )
}

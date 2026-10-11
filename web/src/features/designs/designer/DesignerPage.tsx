import "@/features/designs/i18n"

import {
  DndContext,
  DragOverlay,
  PointerSensor,
  pointerWithin,
  rectIntersection,
  useSensor,
  useSensors,
  type CollisionDetection,
  type DragEndEvent,
  type DragMoveEvent,
  type DragStartEvent,
} from "@dnd-kit/core"
import { Trans, useLingui } from "@lingui/react/macro"
import {
  ArchiveIcon,
  ArrowLeftIcon,
  BlocksIcon,
  CheckIcon,
  CircleAlertIcon,
  CloudOffIcon,
  CopyIcon,
  DownloadIcon,
  EllipsisIcon,
  HistoryIcon,
  KeyboardIcon,
  LayersIcon,
  LoaderIcon,
  PaletteIcon,
  Redo2Icon,
  SaveIcon,
  TriangleAlertIcon,
  Undo2Icon,
} from "lucide-react"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { Link, Navigate, useNavigate, useParams } from "react-router"

import { useShell } from "@/app/shell"
import { ErrorLine, Kbd, Notice, Segmented, toast, useConfirm } from "@/components/common"
import { DESIGNER_KEYS, mod } from "@/components/common/ShortcutSheet"
import { formatBytes } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import {
  children,
  countBlocks,
  duplicateBlock,
  fileName,
  findBlock,
  imageIds,
  insertBlock,
  insertNear,
  makeBlock,
  MAX_BLOCKS,
  moveBlock,
  nudge,
  removeBlock,
  resolveDrop,
  updateBlock,
  walk,
  type Block,
  type BlockType,
  type Doc,
} from "@/features/designs/doc"
import { readDraft, serialize, stateOf, useAutosave, type SaveStatus } from "@/features/designs/designer/autosave"
import { BlockList, EmptyCanvas, FONTS, RequiredFooter } from "@/features/designs/designer/Canvas"
import { ConflictDialog } from "@/features/designs/designer/ConflictDialog"
import { DesignerContext, type Designer, type DropTarget } from "@/features/designs/designer/context"
import { clearActiveEditor, TextToolbar } from "@/features/designs/designer/editor"
import { useHistory, type DesignState } from "@/features/designs/designer/history"
import { BlockIcon } from "@/features/designs/designer/icons"
import { ImageLibrary } from "@/features/designs/designer/ImageLibrary"
import { Inspector } from "@/features/designs/designer/Inspector"
import { BlockPalette, Layers } from "@/features/designs/designer/Panels"
import { Preview, type PreviewMode } from "@/features/designs/designer/Preview"
import { ThemePanel } from "@/features/designs/designer/ThemePanel"
import { Versions } from "@/features/designs/designer/Versions"
import {
  conflictOf,
  designLink,
  issuesOf,
  restoreVersion,
  useArchiveDesign,
  useCreateDesign,
  useDesign,
  useDuplicateDesign,
  useImageMap,
  useInvalidateVersions,
  useRender,
} from "@/features/designs/queries"
import { isWarning, useBlockNames, useIssueText } from "@/features/designs/text"
import { useDebounced } from "@/hooks/use-debounced"
import { useIsPhone } from "@/hooks/use-media-query"
import type { EmailDesign, EmailDesignIssue } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"

type Mode = "edit" | PreviewMode

const CLIP = 102 * 1024
const NEAR = 90 * 1024

const collision: CollisionDetection = (args) => {
  const hits = pointerWithin(args)
  if (hits.length === 0) return rectIntersection(args)
  const area = (id: string | number) => {
    const r = args.droppableRects.get(id)
    return r ? r.width * r.height : Infinity
  }
  return [...hits].sort((a, b) => area(a.id) - area(b.id))
}

function isTyping(el: EventTarget | null) {
  return el instanceof HTMLElement && (el.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(el.tagName))
}

function hasLayer() {
  return !!document.querySelector(
    ':is([role="dialog"], [role="alertdialog"], [role="menu"], [role="listbox"]):not([data-closed])'
  )
}

function download(state: DesignState) {
  const blob = new Blob([JSON.stringify(state.document, null, 2) + "\n"], { type: "application/json" })
  const url = URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = url
  a.download = fileName(state.name)
  document.body.append(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

export default function DesignerPage() {
  const { t } = useLingui()
  const { designId = "" } = useParams()
  const { canManage, workspaceId } = useSession()
  const design = useDesign(designId)
  if (!canManage) return <Navigate to="/broadcasts" replace />
  if (design.isPending)
    return (
      <div className="flex h-svh flex-col" data-testid="designer-loading">
        <div className="flex h-14 items-center gap-3 border-b px-4">
          <Skeleton className="h-7 w-48" />
        </div>
        <div className="flex flex-1 gap-0">
          <Skeleton className="m-4 w-60 rounded-xl phone:hidden" />
          <Skeleton className="m-4 flex-1 rounded-xl" />
          <Skeleton className="m-4 w-72 rounded-xl phone:hidden" />
        </div>
      </div>
    )
  if (!design.data)
    return (
      <div className="mx-auto flex max-w-lg flex-col gap-4 px-6 py-16">
        <Link
          to="/broadcasts/designs"
          className="inline-flex items-center gap-1.5 text-body text-faint hover:text-foreground"
        >
          <ArrowLeftIcon className="size-3.5 rtl:rotate-180" />
          {t`Designs`}
        </Link>
        <ErrorLine error={design.error} />
      </div>
    )
  return <DesignerScreen key={designId} initial={design.data} ws={workspaceId} />
}

function bootState(initial: EmailDesign, ws: string) {
  const server = stateOf(initial)
  const draft = readDraft(ws, initial.id)
  if (draft && serialize(draft.state) !== serialize(server)) {
    if (draft.base === initial.version) return { state: draft.state, base: initial.version, conflict: null }
    return { state: draft.state, base: draft.base, conflict: initial }
  }
  return { state: server, base: initial.version, conflict: null as EmailDesign | null }
}

function StatusLine({ status, onRetry, onChecks }: { status: SaveStatus; onRetry: () => void; onChecks: () => void }) {
  const { t } = useLingui()
  const base = "inline-flex shrink-0 items-center gap-1.5 text-small whitespace-nowrap"
  switch (status) {
    case "saved":
      return (
        <span className={cn(base, "text-faint")} role="status" data-testid="save-status" data-status={status}>
          <CheckIcon className="size-3.5" />
          <span className="phone:hidden">
            <Trans>Saved</Trans>
          </span>
        </span>
      )
    case "dirty":
    case "saving":
      return (
        <span className={cn(base, "text-faint")} role="status" data-testid="save-status" data-status={status}>
          <LoaderIcon className={cn("size-3.5", status === "saving" && "animate-spin")} />
          <span className="phone:hidden">
            <Trans>Saving…</Trans>
          </span>
        </span>
      )
    case "offline":
      return (
        <span
          className={cn(base, "text-warning")}
          role="status"
          title={t`Offline. Your changes are kept on this device and saved when you are back.`}
          data-testid="save-status"
          data-status={status}
        >
          <CloudOffIcon className="size-3.5" />
          <span className="phone:hidden">
            <Trans>Offline, kept on this device</Trans>
          </span>
        </span>
      )
    case "invalid":
      return (
        <button
          type="button"
          className={cn(base, "text-destructive hover:underline")}
          onClick={onChecks}
          data-testid="save-status"
          data-status={status}
        >
          <CircleAlertIcon className="size-3.5" />
          <span className="phone:hidden">
            <Trans>Not saved: fix the marked blocks</Trans>
          </span>
        </button>
      )
    case "conflict":
      return (
        <span className={cn(base, "text-warning")} role="status" data-testid="save-status" data-status={status}>
          <TriangleAlertIcon className="size-3.5" />
          <span className="phone:hidden">
            <Trans>Changed elsewhere</Trans>
          </span>
        </span>
      )
    case "error":
      return (
        <button
          type="button"
          className={cn(base, "text-destructive hover:underline")}
          onClick={onRetry}
          data-testid="save-status"
          data-status={status}
        >
          <CircleAlertIcon className="size-3.5" />
          <Trans>Not saved. Try again</Trans>
        </button>
      )
  }
}

function SizeMeter({ size }: { size: number | undefined }) {
  const { i18n } = useLingui()
  if (size === undefined) return null
  const pct = Math.min(100, (size / CLIP) * 100)
  const tone = size > CLIP ? "bg-destructive" : size > NEAR ? "bg-warning" : "bg-success"
  const used = formatBytes(size, i18n.locale)
  const limit = formatBytes(CLIP, i18n.locale)
  return (
    <div className="flex flex-col gap-1.5" data-testid="size-meter">
      <div className="flex items-center justify-between text-small">
        <span className="text-muted-foreground">
          <Trans>Size</Trans>
        </span>
        <span className="tabular-nums">
          <Trans>
            {used} of {limit}
          </Trans>
        </span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-muted">
        <div className={cn("h-full rounded-full", tone)} style={{ width: `${pct}%` }} />
      </div>
      <p className="text-caption text-faint">
        <Trans>Gmail cuts mails off at about 102 KB of HTML, hiding the unsubscribe link.</Trans>
      </p>
    </div>
  )
}

function Checks({
  size,
  issues,
  open,
  onOpenChange,
  onShow,
  doc,
}: {
  size: number | undefined
  issues: EmailDesignIssue[]
  open: boolean
  onOpenChange: (o: boolean) => void
  onShow: (id: string) => void
  doc: Doc
}) {
  const { t, i18n } = useLingui()
  const text = useIssueText()
  const names = useBlockNames()
  const errors = issues.filter((i) => !isWarning(i)).length
  const warnings = issues.length - errors
  const tone =
    errors > 0 || (size ?? 0) > CLIP
      ? "text-destructive"
      : warnings > 0 || (size ?? 0) > NEAR
        ? "text-warning"
        : "text-success"
  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger
        render={<Button variant="ghost" size="sm" className="gap-1.5 px-2.5" data-testid="checks" />}
        aria-label={t`Checks`}
      >
        {errors + warnings > 0 ? <TriangleAlertIcon className={tone} /> : <CheckIcon className={tone} />}
        <span className="tabular-nums">
          {size !== undefined ? formatBytes(size, i18n.locale) : "…"}
          {errors + warnings > 0 && ` · ${errors + warnings}`}
        </span>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-96 max-w-[calc(100vw-2rem)] gap-4">
        <SizeMeter size={size} />
        {issues.length === 0 ? (
          <p className="flex items-center gap-2 text-small text-success">
            <CheckIcon className="size-4" />
            <Trans>No problems found.</Trans>
          </p>
        ) : (
          <ul className="flex max-h-80 flex-col gap-1.5 overflow-y-auto" data-testid="checks-list">
            {issues.map((i, n) => {
              const b = i.block_id ? findBlock(doc, i.block_id) : null
              return (
                <li key={n} className="flex gap-2 rounded-lg bg-surface px-2.5 py-2 text-small">
                  {isWarning(i) ? (
                    <TriangleAlertIcon className="mt-0.5 size-3.5 shrink-0 text-warning" />
                  ) : (
                    <CircleAlertIcon className="mt-0.5 size-3.5 shrink-0 text-destructive" />
                  )}
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    {b && <span className="text-caption font-medium text-faint">{names[b.type]}</span>}
                    <span>{text(i)}</span>
                  </span>
                  {b && (
                    <Button
                      variant="link"
                      size="sm"
                      className="h-auto self-start text-small"
                      onClick={() => {
                        onShow(b.id)
                        onOpenChange(false)
                      }}
                    >
                      <Trans>Show</Trans>
                    </Button>
                  )}
                </li>
              )
            })}
          </ul>
        )}
      </PopoverContent>
    </Popover>
  )
}

function IconButton({
  label,
  keys,
  onClick,
  disabled,
  children,
  testId,
}: {
  label: string
  keys?: string[]
  onClick: () => void
  disabled?: boolean
  children: React.ReactNode
  testId?: string
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={label}
            disabled={disabled}
            onClick={onClick}
            data-testid={testId}
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent>
        {label}
        {keys && <span className="opacity-60">{keys.join(" ")}</span>}
      </TooltipContent>
    </Tooltip>
  )
}

function DesignerScreen({ initial, ws }: { initial: EmailDesign; ws: string }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const shell = useShell()
  const phone = useIsPhone()
  const names = useBlockNames()
  const [boot] = useState(() => bootState(initial, ws))
  const history = useHistory(boot.state)
  const commit = history.commit
  const state = history.state
  const doc = state.document
  const readOnly = doc.schema !== 1
  const save = useAutosave({
    ws,
    design: initial,
    base: boot.base,
    state,
    enabled: !readOnly,
    initialConflict: boot.conflict,
  })
  const [selectedRaw, setSelected] = useState<string | null>(null)
  const selected = selectedRaw && findBlock(doc, selectedRaw) ? selectedRaw : null
  const selectedBlock = selected ? findBlock(doc, selected) : null
  const [mode, setMode] = useState<Mode>("edit")
  const [left, setLeft] = useState<"blocks" | "layers">("blocks")
  const [sheet, setSheet] = useState<null | "blocks" | "layers" | "style">(null)
  const [library, setLibrary] = useState<string | null>(null)
  const altFocus = useRef<string | null>(null)
  const [toolSlot, setToolSlot] = useState<HTMLElement | null>(null)
  const [versionsOpen, setVersionsOpen] = useState(false)
  const [checksOpen, setChecksOpen] = useState(false)
  const [drop, setDrop] = useState<DropTarget | null>(null)
  const [dragging, setDragging] = useState<{ id: string; type: BlockType } | null>(null)
  const [restoring, setRestoring] = useState(false)
  const [restoreError, setRestoreError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)
  const images = useImageMap(useMemo(() => imageIds(doc), [doc]))
  const create = useCreateDesign()
  const duplicate = useDuplicateDesign()
  const archive = useArchiveDesign()
  const invalidateVersions = useInvalidateVersions(initial.id)
  const [confirm, confirmDialog] = useConfirm()
  const palette = useRef<HTMLDivElement>(null)

  const renderInput = useMemo(
    () => ({ document: doc, inbox_id: state.inbox_id, dark: mode === "dark" || undefined }),
    [doc, state.inbox_id, mode]
  )
  const debounced = useDebounced(renderInput, mode === "edit" ? 2000 : 400)
  const render = useRender(debounced)
  const stale = debounced !== renderInput || render.isPlaceholderData || render.isFetching

  const defaults = {
    heading: t`A clear heading`,
    text: t`Write what you want to tell your readers.`,
    button: t`Read more`,
    footer: t`You can reply to this mail; a person reads every answer.`,
    columnText: t`Some text for this column.`,
  }

  const commitDoc = useCallback(
    (fn: (d: Doc) => Doc, key?: string) => commit((s) => ({ ...s, document: fn(s.document) }), key),
    [commit]
  )

  const select = useCallback((id: string | null) => {
    setSelected(id)
    if (!id) clearActiveEditor()
  }, [])

  const add = (type: BlockType, at?: DropTarget) => {
    if (readOnly) return
    if (countBlocks(doc) >= MAX_BLOCKS) {
      toast(t`A design can hold at most 200 blocks.`)
      return
    }
    const block = makeBlock(type, defaults)
    const next = at ? insertBlock(doc, at.container, at.index, block) : insertNear(doc, selected, block)
    history.commit({ ...state, document: next })
    setSelected(block.id)
    setSheet(null)
    if (type === "image") setLibrary(block.id)
    requestAnimationFrame(() =>
      document.querySelector(`[data-block="${block.id}"]`)?.scrollIntoView({ block: "nearest", behavior: "smooth" })
    )
  }

  const ctx: Designer = {
    doc,
    selected,
    select,
    setDoc: commitDoc,
    update: (id, patch, key) =>
      commitDoc((d) => updateBlock(d, id, patch as Partial<Block> | ((b: Block) => Block)), key),
    remove: (id) => {
      const entries = walk(doc)
      const i = entries.findIndex((e) => e.block.id === id)
      const next = entries.slice(i + 1).find((e) => !findBlock({ ...doc, blocks: [entries[i].block] }, e.block.id))
      commitDoc((d) => removeBlock(d, id))
      setSelected(next?.block.id ?? entries[i - 1]?.block.id ?? null)
    },
    duplicate: (id) => {
      if (countBlocks(doc) >= MAX_BLOCKS) return toast(t`A design can hold at most 200 blocks.`)
      const r = duplicateBlock(doc, id)
      history.commit({ ...state, document: r.doc })
      setSelected(r.id)
    },
    nudge: (id, by) => commitDoc((d) => nudge(d, id, by)),
    images,
    issues: new Map(),
    readOnly,
    openLibrary: (id) => setLibrary(id),
    drop,
    dragging: dragging?.id ?? null,
    toolSlot,
  }

  const renderIssues = render.data && !render.isPlaceholderData ? render.data.warnings : null
  const allIssues = useMemo(() => {
    const list = [...save.errors, ...issuesOf(render.error)]
    const warnings = renderIssues ?? save.warnings
    const seen = new Set(list.map((i) => `${i.code}:${i.block_id}:${i.field}`))
    for (const w of warnings) if (!seen.has(`${w.code}:${w.block_id}:${w.field}`)) list.push(w)
    return list
  }, [save.errors, save.warnings, render.error, renderIssues])
  for (const i of allIssues) {
    if (!i.block_id) continue
    const list = ctx.issues.get(i.block_id) ?? []
    list.push(i)
    ctx.issues.set(i.block_id, list)
  }

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))

  const targetOf = (e: DragMoveEvent): DropTarget | null => {
    const over = e.over
    const a = e.active.data.current as { kind: "new" | "block"; type: BlockType; id?: string } | undefined
    if (!over || !a) return null
    const data = over.data.current as { kind: "block" | "container"; container: string; index?: number; id?: string }
    const moving = a.kind === "block" ? a.id : undefined
    if (data.kind === "block") {
      if (moving && data.id === moving) return null
      const start = e.activatorEvent as PointerEvent
      const y = start.clientY + e.delta.y
      const after = y > over.rect.top + over.rect.height / 2
      return resolveDrop(doc, a.type, { container: data.container, index: (data.index ?? 0) + (after ? 1 : 0) }, moving)
    }
    return resolveDrop(doc, a.type, { container: data.container, index: children(doc, data.container).length }, moving)
  }

  const onDragStart = (e: DragStartEvent) => {
    const a = e.active.data.current as { type: BlockType; id?: string }
    setDragging({ id: String(e.active.id), type: a.type })
  }
  const onDragMove = (e: DragMoveEvent) => {
    const next = targetOf(e)
    setDrop((d) => (d?.container === next?.container && d?.index === next?.index ? d : next))
  }
  const onDragEnd = (e: DragEndEvent) => {
    const target = targetOf(e)
    const a = e.active.data.current as { kind: "new" | "block"; type: BlockType; id?: string }
    setDrop(null)
    setDragging(null)
    if (!target) return
    if (a.kind === "new") add(a.type, target)
    else if (a.id) {
      commitDoc((d) => moveBlock(d, a.id!, target.container, target.index))
      setSelected(a.id)
    }
  }

  const flushNow = () => {
    void save.flush(true).then(() => invalidateVersions())
  }

  const leave = () => {
    void save.flush()
    navigate("/broadcasts/designs")
  }

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const typing = isTyping(e.target)
      const meta = e.metaKey || e.ctrlKey
      const key = e.key.toLowerCase()
      if (meta && !e.altKey && (key === "z" || key === "y")) {
        if (hasLayer() && !document.querySelector("[data-testid=designer]")?.contains(e.target as Node)) return
        e.preventDefault()
        if (key === "y" || e.shiftKey) history.redo()
        else history.undo()
        return
      }
      if (meta && key === "s") {
        e.preventDefault()
        flushNow()
        toast(t`Saved, with a version kept`)
        return
      }
      if (hasLayer() || readOnly) return
      if (e.key === "Escape" && typing && (e.target as HTMLElement).isContentEditable) {
        ;(e.target as HTMLElement).blur()
        return
      }
      if (typing) return
      if (meta && key === "d" && selected) {
        e.preventDefault()
        ctx.duplicate(selected)
        return
      }
      if (meta) return
      if (e.altKey && (e.key === "ArrowUp" || e.key === "ArrowDown")) {
        if (!selected) return
        e.preventDefault()
        ctx.nudge(selected, e.key === "ArrowUp" ? -1 : 1)
        return
      }
      if (e.altKey) return
      if (e.key === "ArrowUp" || e.key === "ArrowDown") {
        const list = walk(doc)
        if (list.length === 0) return
        e.preventDefault()
        const i = list.findIndex((x) => x.block.id === selected)
        const next =
          i < 0
            ? e.key === "ArrowDown"
              ? 0
              : list.length - 1
            : Math.max(0, Math.min(list.length - 1, i + (e.key === "ArrowDown" ? 1 : -1)))
        const id = list[next].block.id
        setSelected(id)
        document.querySelector(`[data-block="${id}"]`)?.scrollIntoView({ block: "nearest" })
        return
      }
      if ((e.key === "Backspace" || e.key === "Delete") && selected) {
        e.preventDefault()
        ctx.remove(selected)
        return
      }
      if (e.key === "Escape" && selected) {
        setSelected(null)
        return
      }
      if (e.key === DESIGNER_KEYS.edit) setMode("edit")
      else if (e.key === DESIGNER_KEYS.desktop) setMode("desktop")
      else if (e.key === DESIGNER_KEYS.phone) setMode("phone")
      else if (e.key === DESIGNER_KEYS.dark) setMode("dark")
      else if (e.key === DESIGNER_KEYS.add) {
        e.preventDefault()
        setMode("edit")
        if (phone) setSheet("blocks")
        else {
          setLeft("blocks")
          requestAnimationFrame(() => palette.current?.querySelector<HTMLButtonElement>("button")?.focus())
        }
      } else if (e.key === DESIGNER_KEYS.theme) {
        setSelected(null)
        if (phone) setSheet("style")
      } else if (e.key === DESIGNER_KEYS.versions) setVersionsOpen(true)
      else if (e.key === DESIGNER_KEYS.image && selectedBlock?.type === "image") setLibrary(selectedBlock.id)
      else if (e.key === "?") shell.openShortcuts()
      else return
      e.preventDefault()
    }
    window.addEventListener("keydown", onKey, true)
    return () => window.removeEventListener("keydown", onKey, true)
  })

  const resolveTheirs = () => {
    const theirs = save.conflict
    if (!theirs) return
    save.resolve.theirs(theirs)
    history.commit(stateOf(theirs))
  }
  const resolveMine = () => {
    if (save.conflict) save.resolve.mine(save.conflict)
  }
  const resolveCopy = () => {
    const theirs = save.conflict
    if (!theirs) return
    const name = state.name
    setBusy(true)
    create.mutate(
      { name: t`${name} (copy)`, inbox_id: state.inbox_id, document: doc },
      {
        onSuccess: (copy) => {
          save.resolve.theirs(theirs)
          history.reset(stateOf(theirs))
          navigate(designLink(copy.id))
        },
        onError: (err) => toast(err.message),
        onSettled: () => setBusy(false),
      }
    )
  }

  const restore = (version: number) => {
    setRestoring(true)
    setRestoreError(null)
    restoreVersion(initial.id, version, save.version())
      .then((d) => {
        save.resolve.restored(d)
        history.commit(stateOf(d))
        setVersionsOpen(false)
        invalidateVersions()
        toast(t`Version restored`, () => history.undo())
      })
      .catch((err) => {
        const theirs = conflictOf(err)
        if (theirs) {
          setVersionsOpen(false)
          save.conflictWith(theirs)
        } else setRestoreError(err)
      })
      .finally(() => setRestoring(false))
  }

  const issueCount = allIssues.length
  const size = render.data?.size_bytes
  const theme = doc.theme
  const showLeft = !phone && mode === "edit"
  const right = selectedBlock ? (
    <Inspector key={selectedBlock.id} block={selectedBlock} />
  ) : (
    <ThemePanel inboxId={state.inbox_id} onInbox={(id) => history.commit({ ...state, inbox_id: id })} />
  )

  const canvas = (
    <div
      className="min-h-full px-6 py-8 phone:px-3 phone:py-5"
      style={{ background: theme.colors.page }}
      onClick={() => select(null)}
      data-testid="canvas"
    >
      <div
        className="mx-auto max-w-full overflow-visible shadow-[0_1px_3px_rgb(0_0_0/0.06),0_12px_32px_-16px_rgb(0_0_0/0.18)]"
        style={
          {
            width: theme.width,
            background: theme.colors.card,
            borderRadius: theme.radius ?? 0,
            fontFamily: FONTS[theme.font],
            color: theme.colors.text,
            "--email-link": theme.colors.link,
          } as React.CSSProperties
        }
        dir={theme.direction === "rtl" ? "rtl" : theme.direction === "ltr" ? "ltr" : undefined}
        data-testid="canvas-card"
      >
        {doc.blocks.length === 0 ? (
          <EmptyCanvas />
        ) : (
          <div className="py-3">
            <BlockList container="root" blocks={doc.blocks} nested={false} />
          </div>
        )}
      </div>
      <RequiredFooter doc={doc} />
    </div>
  )

  const modes = [
    { value: "edit" as const, label: t`Edit`, testId: "mode-edit" },
    { value: "desktop" as const, label: t`Desktop`, testId: "mode-desktop" },
    { value: "phone" as const, label: t`Phone`, testId: "mode-phone" },
    { value: "dark" as const, label: t`Dark`, testId: "mode-dark" },
  ]

  return (
    <DesignerContext.Provider value={ctx}>
      <DndContext
        sensors={sensors}
        collisionDetection={collision}
        onDragStart={onDragStart}
        onDragMove={onDragMove}
        onDragEnd={onDragEnd}
        onDragCancel={() => {
          setDrop(null)
          setDragging(null)
        }}
      >
        <div className="flex h-svh flex-col bg-background" data-testid="designer">
          <header className="flex h-14 shrink-0 items-center gap-2 border-b px-3 phone:gap-1 phone:px-2">
            <IconButton label={t`Back to designs`} onClick={leave} testId="designer-back">
              <ArrowLeftIcon className="rtl:rotate-180" />
            </IconButton>
            <input
              value={state.name}
              maxLength={200}
              onChange={(e) => history.commit({ ...state, name: e.target.value }, "name")}
              onBlur={() => !state.name.trim() && history.commit({ ...state, name: initial.name })}
              aria-label={t`Design name`}
              readOnly={readOnly}
              className="h-8 min-w-0 flex-1 rounded-lg bg-transparent px-2 text-body font-semibold outline-none hover:bg-muted focus:bg-muted sm:max-w-80"
              data-testid="design-name"
            />
            <StatusLine status={save.status} onRetry={() => void save.flush()} onChecks={() => setChecksOpen(true)} />
            <div className="flex-1 phone:hidden" />
            {!phone && <Segmented value={mode} onChange={setMode} label={t`View`} items={modes} className="shrink-0" />}
            <div className="flex-1 phone:hidden" />
            <IconButton
              label={t`Undo`}
              keys={[mod, "Z"]}
              onClick={history.undo}
              disabled={!history.canUndo || readOnly}
              testId="undo"
            >
              <Undo2Icon />
            </IconButton>
            <IconButton
              label={t`Redo`}
              keys={["⇧", mod, "Z"]}
              onClick={history.redo}
              disabled={!history.canRedo || readOnly}
              testId="redo"
            >
              <Redo2Icon />
            </IconButton>
            {!phone && (
              <Checks
                size={size}
                issues={allIssues}
                open={checksOpen}
                onOpenChange={setChecksOpen}
                onShow={(id) => {
                  setMode("edit")
                  setSelected(id)
                  requestAnimationFrame(() =>
                    document
                      .querySelector(`[data-block="${id}"]`)
                      ?.scrollIntoView({ block: "center", behavior: "smooth" })
                  )
                }}
                doc={doc}
              />
            )}
            {!phone && (
              <IconButton label={t`Versions`} keys={["V"]} onClick={() => setVersionsOpen(true)} testId="versions-open">
                <HistoryIcon />
              </IconButton>
            )}
            <DropdownMenu>
              <DropdownMenuTrigger
                render={<Button variant="ghost" size="icon-sm" />}
                aria-label={t`More actions`}
                data-testid="designer-menu"
              >
                <EllipsisIcon />
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="min-w-64">
                {phone && (
                  <>
                    <DropdownMenuItem onClick={() => setChecksOpen(true)}>
                      <TriangleAlertIcon />
                      <span className="flex-1">
                        <Trans>Checks</Trans>
                      </span>
                      {issueCount > 0 && <span className="text-faint tabular-nums">{issueCount}</span>}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => setVersionsOpen(true)}>
                      <HistoryIcon />
                      <Trans>Versions</Trans>
                    </DropdownMenuItem>
                  </>
                )}
                <DropdownMenuItem onClick={flushNow} disabled={readOnly} data-testid="save-version">
                  <SaveIcon />
                  <span className="flex-1">
                    <Trans>Save now and keep a version</Trans>
                  </span>
                  <span className="flex gap-0.5">
                    <Kbd>{mod}</Kbd>
                    <Kbd>S</Kbd>
                  </span>
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => download(state)} data-testid="download-json">
                  <DownloadIcon />
                  <Trans>Download JSON</Trans>
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() =>
                    duplicate.mutate(initial.id, {
                      onSuccess: (d) => {
                        toast(t`Copy made`)
                        navigate(designLink(d.id))
                      },
                      onError: (err) => toast(err.message),
                    })
                  }
                >
                  <CopyIcon />
                  <Trans>Make a copy</Trans>
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => shell.openShortcuts()}>
                  <KeyboardIcon />
                  <span className="flex-1">
                    <Trans>Keyboard shortcuts</Trans>
                  </span>
                  <Kbd>?</Kbd>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  variant="destructive"
                  onClick={() =>
                    confirm({
                      title: t`Archive this design?`,
                      description: t`It leaves the list of designs. Its versions are kept, and saving it again brings it back.`,
                      confirm: t`Archive`,
                      run: () =>
                        archive.mutate(initial.id, {
                          onSuccess: () => {
                            toast(t`Design archived`)
                            navigate("/broadcasts/designs")
                          },
                        }),
                    })
                  }
                  data-testid="archive-design"
                >
                  <ArchiveIcon />
                  <Trans>Archive</Trans>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            {!phone && (
              <Button size="sm" onClick={leave} className="ms-1" data-testid="designer-done">
                <Trans>Done</Trans>
              </Button>
            )}
          </header>
          {phone && (
            <div className="border-b px-2 py-1.5">
              <Segmented
                value={mode}
                onChange={setMode}
                label={t`View`}
                items={modes}
                className="w-full"
                itemClassName="flex-1 px-1"
              />
            </div>
          )}
          {readOnly && (
            <Notice tone="warning" icon={TriangleAlertIcon} className="m-3 mb-0" data-testid="designer-read-only">
              <p>
                <Trans>A newer version of Yuva made this design. You can look at it here, but not change it.</Trans>
              </p>
            </Notice>
          )}
          {save.problem && save.status === "invalid" && save.errors.length === 0 && (
            <Notice tone="danger" icon={CircleAlertIcon} className="m-3 mb-0">
              <ErrorLine error={save.problem} />
            </Notice>
          )}
          <div className="flex min-h-0 flex-1">
            {showLeft && (
              <aside
                className="flex w-64 shrink-0 flex-col border-e max-[1100px]:w-56"
                aria-label={t`Blocks and layers`}
              >
                <div className="px-3 pt-3">
                  <Segmented
                    value={left}
                    onChange={setLeft}
                    label={t`Left panel`}
                    className="w-full"
                    itemClassName="flex-1"
                    items={[
                      { value: "blocks", label: t`Blocks`, testId: "tab-blocks" },
                      { value: "layers", label: t`Layers`, testId: "tab-layers" },
                    ]}
                  />
                </div>
                <div className="min-h-0 flex-1 overflow-y-auto p-3" ref={palette}>
                  {left === "blocks" ? <BlockPalette onAdd={(type) => add(type)} /> : <Layers />}
                </div>
              </aside>
            )}
            <main className="flex min-w-0 flex-1 flex-col">
              {mode === "edit" ? (
                <>
                  {!readOnly && <TextToolbar owner={selected} slot={setToolSlot} />}
                  <div className="min-h-0 flex-1 overflow-y-auto">{canvas}</div>
                </>
              ) : (
                <div className="min-h-0 flex-1 overflow-y-auto">
                  <Preview
                    mode={mode}
                    html={render.data?.html}
                    loading={render.isPending}
                    error={render.error}
                    stale={stale}
                  />
                </div>
              )}
              {phone && (
                <nav className="flex h-14 shrink-0 items-stretch border-t bg-card" aria-label={t`Designer`}>
                  {(
                    [
                      ["blocks", BlocksIcon, t`Add`],
                      ["layers", LayersIcon, t`Layers`],
                      ["style", PaletteIcon, selectedBlock ? names[selectedBlock.type] : t`Design`],
                    ] as const
                  ).map(([key, Icon, label]) => (
                    <button
                      key={key}
                      type="button"
                      onClick={() => setSheet(key)}
                      className="flex flex-1 flex-col items-center justify-center gap-0.5 text-caption text-muted-foreground"
                      data-testid={`phone-${key}`}
                    >
                      {key === "style" && selectedBlock ? (
                        <BlockIcon type={selectedBlock.type} className="size-5" />
                      ) : (
                        <Icon className="size-5" />
                      )}
                      <span className="max-w-full truncate px-1">{label}</span>
                    </button>
                  ))}
                </nav>
              )}
            </main>
            {!phone && (
              <aside
                className="w-80 shrink-0 overflow-y-auto border-s max-[1100px]:w-72"
                aria-label={selectedBlock ? t`Block settings` : t`Design settings`}
              >
                {right}
              </aside>
            )}
          </div>
        </div>
        <DragOverlay dropAnimation={null}>
          {dragging && (
            <div className="inline-flex items-center gap-2 rounded-full bg-brand px-3 py-1.5 text-small font-medium text-white shadow-lg">
              <BlockIcon type={dragging.type} className="size-4" />
              {names[dragging.type]}
            </div>
          )}
        </DragOverlay>
      </DndContext>
      {phone && (
        <Sheet open={sheet !== null} onOpenChange={(o) => !o && setSheet(null)}>
          <SheetContent className="w-full" data-testid="designer-sheet">
            <SheetHeader>
              <SheetTitle className="text-title">
                {sheet === "blocks" ? (
                  <Trans>Add a block</Trans>
                ) : sheet === "layers" ? (
                  <Trans>Layers</Trans>
                ) : selectedBlock ? (
                  names[selectedBlock.type]
                ) : (
                  <Trans>Design</Trans>
                )}
              </SheetTitle>
            </SheetHeader>
            <div className="min-h-0 flex-1 overflow-y-auto">
              {sheet === "blocks" && (
                <div className="p-4">
                  <BlockPalette onAdd={(type) => add(type)} />
                </div>
              )}
              {sheet === "layers" && (
                <div className="p-2" onClickCapture={() => setTimeout(() => setSheet(null), 0)}>
                  <Layers />
                </div>
              )}
              {sheet === "style" && right}
            </div>
          </SheetContent>
        </Sheet>
      )}
      {phone && (
        <Sheet open={checksOpen} onOpenChange={setChecksOpen}>
          <SheetContent className="w-full">
            <SheetHeader>
              <SheetTitle className="text-title">
                <Trans>Checks</Trans>
              </SheetTitle>
            </SheetHeader>
            <div className="flex flex-col gap-4 overflow-y-auto p-4">
              <SizeMeter size={size} />
              <PhoneIssues
                issues={allIssues}
                doc={doc}
                onShow={(id) => {
                  setChecksOpen(false)
                  setMode("edit")
                  setSelected(id)
                }}
              />
            </div>
          </SheetContent>
        </Sheet>
      )}
      <ImageLibrary
        open={library !== null}
        onOpenChange={(o) => !o && setLibrary(null)}
        current={library ? (findBlock(doc, library) as { image_id?: string } | null)?.image_id : undefined}
        onPick={(img) => {
          const block = library ? findBlock(doc, library) : null
          if (block?.type === "image" && block.image_id !== img.id) {
            ctx.update(block.id, block.decorative ? { image_id: img.id } : { image_id: img.id, alt: "" })
            if (!block.decorative) altFocus.current = block.id
          }
          setLibrary(null)
        }}
        finalFocus={() => {
          const id = altFocus.current
          altFocus.current = null
          return (id && document.getElementById(`alt-${id}`)) || true
        }}
      />
      <Versions
        open={versionsOpen}
        onOpenChange={setVersionsOpen}
        designId={initial.id}
        inboxId={state.inbox_id}
        current={save.version()}
        onRestore={restore}
        restoring={restoring}
        error={restoreError}
      />
      <ConflictDialog
        theirs={save.conflict}
        busy={busy}
        onTheirs={resolveTheirs}
        onMine={resolveMine}
        onCopy={resolveCopy}
        onDownload={() => download(state)}
      />
      {confirmDialog}
      <span className="sr-only" aria-live="polite">
        {save.status === "saved" ? t`Saved` : ""}
      </span>
    </DesignerContext.Provider>
  )
}

function PhoneIssues({ issues, doc, onShow }: { issues: EmailDesignIssue[]; doc: Doc; onShow: (id: string) => void }) {
  const text = useIssueText()
  const names = useBlockNames()
  if (issues.length === 0)
    return (
      <p className="flex items-center gap-2 text-small text-success">
        <CheckIcon className="size-4" />
        <Trans>No problems found.</Trans>
      </p>
    )
  return (
    <ul className="flex flex-col gap-1.5">
      {issues.map((i, n) => {
        const b = i.block_id ? findBlock(doc, i.block_id) : null
        return (
          <li key={n}>
            <button
              type="button"
              disabled={!b}
              onClick={() => b && onShow(b.id)}
              className="flex w-full gap-2 rounded-lg bg-surface px-2.5 py-2 text-start text-small"
            >
              {isWarning(i) ? (
                <TriangleAlertIcon className="mt-0.5 size-3.5 shrink-0 text-warning" />
              ) : (
                <CircleAlertIcon className="mt-0.5 size-3.5 shrink-0 text-destructive" />
              )}
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                {b && <span className="text-caption font-medium text-faint">{names[b.type]}</span>}
                <span>{text(i)}</span>
              </span>
            </button>
          </li>
        )
      })}
    </ul>
  )
}

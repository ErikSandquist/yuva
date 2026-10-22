import "@/features/roadmap/i18n"
import { Trans, useLingui } from "@lingui/react/macro"
import { Command as CommandPrimitive } from "cmdk"
import { ArrowBigUpIcon, CheckIcon, LightbulbIcon, PlusIcon } from "lucide-react"
import { useEffect, useState } from "react"
import { useNavigate } from "react-router"

import { ErrorLine, toast } from "@/components/common"
import { useEnumText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Command, CommandDialog, CommandEmpty, CommandItem, CommandList } from "@/components/ui/command"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { boardOfInbox, ideaLink, useBoards, useIdeaSearch, useLinkIdea, useStartIdea } from "@/features/roadmap/queries"
import { Field } from "@/features/settings/ui"
import type { Conversation } from "@/lib/api"

function firstLine(text: string) {
  const line = text.split("\n").find((l) => l.trim()) ?? ""
  const chars = [...line.trim()]
  return chars.length > 120 ? chars.slice(0, 120).join("").trim() + "…" : line.trim()
}

function NewIdea({ c, onClose, onBack }: { c: Conversation; onClose: () => void; onBack: () => void }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const start = useStartIdea(c.id)
  const [title, setTitle] = useState(() => (c.feedback?.text ? firstLine(c.feedback.text) : c.subject).slice(0, 200))
  const [body, setBody] = useState(() => c.feedback?.text?.trim() ?? "")
  const ready = title.trim().length >= 3 && !!body.trim() && !start.isPending
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-xl" data-testid="new-idea-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            if (!ready) return
            start.mutate(
              { title: title.trim(), body: body.trim() },
              {
                onSuccess: (d) => {
                  toast(t`Idea started`)
                  onClose()
                  if (d.discussion.board_id) navigate(ideaLink(d.discussion.board_id, d.discussion.id))
                },
              }
            )
          }}
        >
          <DialogHeader>
            <DialogTitle>
              <Trans>New idea from this feedback</Trans>
            </DialogTitle>
            <DialogDescription>
              <Trans>
                Starts an idea in the Ideas stage and links this feedback to it. Nothing of the feedback is shown except
                the text below.
              </Trans>
            </DialogDescription>
          </DialogHeader>
          <Field label={<Trans>Title</Trans>} htmlFor="idea-title">
            <Input
              id="idea-title"
              required
              minLength={3}
              maxLength={200}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              data-testid="new-idea-title"
            />
          </Field>
          <Field label={<Trans>Description</Trans>} htmlFor="idea-body" hint={<Trans>Markdown works.</Trans>}>
            <Textarea
              id="idea-body"
              required
              rows={6}
              maxLength={20000}
              value={body}
              onChange={(e) => setBody(e.target.value)}
              className="max-h-64"
              data-testid="new-idea-body"
            />
          </Field>
          <ErrorLine error={start.error} />
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onBack}>
              <Trans>Back</Trans>
            </Button>
            <Button type="submit" disabled={!ready} data-testid="new-idea-submit">
              <Trans>Start the idea</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export default function LinkIdeaDialog({
  c,
  onClose,
  startNew = false,
}: {
  c: Conversation
  onClose: () => void
  startNew?: boolean
}) {
  const { t } = useLingui()
  const text = useEnumText()
  const boards = useBoards()
  const board = boardOfInbox(boards.data, c.inbox_id)
  const [q, setQ] = useState("")
  const navigate = useNavigate()
  const [creating, setCreating] = useState(startNew)
  const [error, setError] = useState<unknown>(null)
  const [search, setSearch] = useState("")
  useEffect(() => {
    const id = setTimeout(() => setSearch(q), 250)
    return () => clearTimeout(id)
  }, [q])
  const ideas = useIdeaSearch(board?.id ?? "", search)
  const { link } = useLinkIdea()
  const items = ideas.data ?? []

  if (creating) return <NewIdea c={c} onClose={onClose} onBack={() => setCreating(false)} />
  return (
    <CommandDialog
      open
      onOpenChange={(o) => !o && !link.isPending && onClose()}
      title={t`Link to an idea`}
      description={t`Pick an idea of this inbox’s board`}
      className="top-[14vh] w-[560px] max-w-[calc(100%-32px)] gap-0 rounded-[18px]! border bg-card p-0 shadow-[0_30px_80px_-20px_rgb(0_0_0/0.35)] sm:max-w-[560px]"
    >
      <Command className="rounded-[18px]! bg-card p-0" data-testid="link-idea" loop shouldFilter={false}>
        <div className="flex items-center gap-3 border-b pe-[18px]">
          <CommandPrimitive.Input
            value={q}
            onValueChange={setQ}
            placeholder={t`Link this feedback to an idea…`}
            aria-label={t`Search ideas`}
            className="min-w-0 flex-1 bg-transparent px-[18px] py-4 text-base outline-none md:text-body placeholder:text-faint"
            data-testid="link-idea-search"
          />
          {board && <span className="max-w-40 shrink-0 truncate text-caption text-faint">{board.name}</span>}
        </div>
        <CommandList className="max-h-[50vh] p-1.5">
          {boards.isPending || (board && ideas.isPending) ? (
            <div className="grid gap-2 p-2">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
            </div>
          ) : !board ? (
            <div className="flex flex-col items-center gap-3 px-6 py-8 text-center text-muted-foreground">
              <Trans>This inbox has no roadmap board. Create one under Roadmap first.</Trans>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  onClose()
                  navigate("/board")
                }}
                data-testid="link-idea-roadmap"
              >
                <Trans>Open Roadmap</Trans>
              </Button>
            </div>
          ) : (
            <>
              {items.length === 0 && (
                <CommandEmpty className="px-6 py-8 text-faint">
                  <Trans>No idea matches.</Trans>
                </CommandEmpty>
              )}
              {items.map((idea) => {
                const already = c.idea?.id === idea.id
                const votes = idea.votes
                return (
                  <CommandItem
                    key={idea.id}
                    value={idea.id}
                    disabled={link.isPending || already}
                    onSelect={() => {
                      setError(null)
                      link.mutate(
                        { ideaId: idea.id, conversationId: c.id },
                        {
                          onSuccess: () => {
                            const title = idea.title
                            toast(t`Linked to ${title}`)
                            onClose()
                          },
                          onError: setError,
                        }
                      )
                    }}
                    data-testid="link-idea-option"
                  >
                    <LightbulbIcon className="text-muted-foreground" />
                    <span className="min-w-0 flex-1 truncate">{idea.title}</span>
                    <span className="inline-flex shrink-0 items-center gap-0.5 text-caption text-faint tabular-nums">
                      <ArrowBigUpIcon className="size-3.5" aria-label={t`${votes} votes`} />
                      {idea.votes}
                    </span>
                    <span className="shrink-0 text-caption text-faint">{idea.stage ? text.stage[idea.stage] : ""}</span>
                    {already && <CheckIcon className="text-success" aria-label={t`Already linked`} />}
                  </CommandItem>
                )
              })}
              <CommandItem value="new-idea" onSelect={() => setCreating(true)} data-testid="link-idea-new">
                <PlusIcon className="text-muted-foreground" />
                <Trans>New idea from this feedback</Trans>
              </CommandItem>
            </>
          )}
        </CommandList>
        {error !== null && (
          <div className="border-t px-4 py-2.5 text-small text-muted-foreground" role="status">
            <ErrorLine error={error} className="text-small" />
          </div>
        )}
      </Command>
    </CommandDialog>
  )
}

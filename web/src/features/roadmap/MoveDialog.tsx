import { Trans, useLingui } from "@lingui/react/macro"
import { useState } from "react"

import { ErrorLine, toast } from "@/components/common"
import { IDEA_STAGES, useEnumText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Textarea } from "@/components/ui/textarea"
import { progressOf, useLinkedFeedback, useMoveStage } from "@/features/roadmap/queries"
import { STAGE_DOT } from "@/features/roadmap/stages"
import { Field } from "@/features/settings/ui"
import type { Discussion, IdeaStage } from "@/lib/api"
import { cn } from "@/lib/utils"

export function MoveDialog({
  idea,
  to: initial,
  onClose,
  onMoved,
}: {
  idea: Discussion
  to: IdeaStage
  onClose: () => void
  onMoved: (stage: IdeaStage) => void
}) {
  const { t } = useLingui()
  const text = useEnumText()
  const move = useMoveStage(idea.id)
  const linkedList = useLinkedFeedback(idea.id)
  const [to, setTo] = useState(initial)
  const [comment, setComment] = useState("")
  const items = linkedList.data?.pages.flatMap((p) => p.items) ?? []
  const loaded = !linkedList.isPending && !linkedList.hasNextPage
  const linked = loaded ? items.length : (idea.linked_count ?? 0)
  const target = to in progressOf ? progressOf[to as keyof typeof progressOf] : undefined
  const gets = items.filter((c) => c.feedback?.progress !== target).length
  const title = idea.title
  const progress = target ? text.progress[target] : ""
  return (
    <Dialog open onOpenChange={(o) => !o && !move.isPending && onClose()}>
      <DialogContent className="sm:max-w-xl" data-testid="move-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            if (to === idea.stage || move.isPending) return
            const stage = text.stage[to]
            move.mutate(
              { stage: to, comment: comment.trim() || undefined },
              {
                onSuccess: () => {
                  toast(t`Moved to ${stage}`)
                  onMoved(to)
                },
              }
            )
          }}
        >
          <DialogHeader>
            <DialogTitle>
              <Trans>Move “{title}”</Trans>
            </DialogTitle>
            <DialogDescription data-testid="move-effect">
              {linked === 0 ? (
                <Trans>No feedback is linked to this idea yet.</Trans>
              ) : !progress ? (
                <Trans>The progress of the linked feedback does not change. Linked feedback: {linked}</Trans>
              ) : !loaded ? null : (
                <>
                  <Trans>
                    Linked feedback that gets the progress {progress}: {gets} of {linked}.
                  </Trans>
                  {gets < linked && (
                    <>
                      {" "}
                      <Trans>The rest already have it.</Trans>
                    </>
                  )}
                </>
              )}
            </DialogDescription>
          </DialogHeader>
          <Field label={<Trans>Stage</Trans>} htmlFor="move-stage">
            <div
              id="move-stage"
              role="radiogroup"
              aria-label={t`Stage`}
              className="grid grid-cols-5 gap-1.5 phone:grid-cols-3"
            >
              {IDEA_STAGES.map((s) => (
                <Button
                  key={s}
                  type="button"
                  variant="outline"
                  size="sm"
                  role="radio"
                  aria-checked={s === to}
                  className={cn(
                    "min-w-0 gap-1.5 rounded-lg px-1.5 font-normal",
                    s === to && "border-foreground bg-muted font-medium"
                  )}
                  onClick={() => setTo(s)}
                  data-testid={`move-stage-${s}`}
                >
                  <span aria-hidden className={cn("size-[7px] shrink-0 rounded-full", STAGE_DOT[s])} />
                  {text.stage[s]}
                </Button>
              ))}
            </div>
          </Field>
          <Field
            label={<Trans>Comment (optional)</Trans>}
            htmlFor="move-comment"
            hint={<Trans>Posted as your public reply on the idea. Markdown works.</Trans>}
          >
            <Textarea
              id="move-comment"
              rows={4}
              maxLength={20000}
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              data-testid="move-comment"
            />
          </Field>
          <ErrorLine error={move.error} />
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose} disabled={move.isPending}>
              <Trans>Cancel</Trans>
            </Button>
            <Button type="submit" disabled={to === idea.stage || move.isPending} data-testid="move-submit">
              <Trans>Move</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

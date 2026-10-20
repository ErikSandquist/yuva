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
import { progressOf, useMoveStage } from "@/features/roadmap/queries"
import { ChoiceSelect, Field } from "@/features/settings/ui"
import type { Discussion, IdeaStage } from "@/lib/api"

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
  const [to, setTo] = useState(initial)
  const [comment, setComment] = useState("")
  const linked = idea.linked_count ?? 0
  const title = idea.title
  const progress = to in progressOf ? text.progress[progressOf[to as keyof typeof progressOf]] : ""
  return (
    <Dialog open onOpenChange={(o) => !o && !move.isPending && onClose()}>
      <DialogContent data-testid="move-dialog">
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
            <DialogDescription>
              {progress && linked > 0 ? (
                <Trans>
                  Every linked feedback that does not have it yet gets the progress {progress}. Linked feedback:{" "}
                  {linked}
                </Trans>
              ) : (
                <Trans>The progress of the linked feedback does not change. Linked feedback: {linked}</Trans>
              )}
            </DialogDescription>
          </DialogHeader>
          <Field label={<Trans>Stage</Trans>} htmlFor="move-stage">
            <ChoiceSelect
              id="move-stage"
              value={to}
              onChange={setTo}
              options={IDEA_STAGES.map((s) => [s, text.stage[s]] as const)}
              label={t`Stage`}
            />
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

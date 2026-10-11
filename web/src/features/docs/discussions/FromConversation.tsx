import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { GlobeIcon } from "lucide-react"
import { useState } from "react"
import { useNavigate } from "react-router"

import { ErrorLine, Notice, toast } from "@/components/common"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { conversationPage, pageLabel } from "@/features/docs/PageContext"
import { threadLink } from "@/features/docs/discussions/queries"
import { Field } from "@/features/settings/ui"
import { api, unwrap, type Conversation, type DiscussionFromConversation, type Message } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

function useStartDiscussion(conversationId: string) {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return useMutation({
    mutationFn: (body: DiscussionFromConversation) =>
      unwrap(api.POST("/v1/conversations/{conversationId}/discussion", { params: { path: { conversationId } }, body })),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: keys.discussions(ws) })
      void qc.invalidateQueries({ queryKey: keys.docs(ws) })
    },
  })
}

const isMessage = (m: Message) => m.kind === "message" && !m.draft

function firstLine(text: string) {
  const line = text.split("\n").find((l) => l.trim()) ?? ""
  const words = [...line.trim()]
  return words.length > 120 ? words.slice(0, 120).join("").trim() + "…" : line.trim()
}

function isUrl(v: string) {
  try {
    const u = new URL(v)
    return u.protocol === "https:" || u.protocol === "http:"
  } catch {
    return false
  }
}

export function FromConversationDialog({
  c,
  messages,
  onClose,
}: {
  c: Conversation
  messages: Message[]
  onClose: () => void
}) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const start = useStartDiscussion(c.id)
  const page = conversationPage(c)
  const asked = messages.find((m) => isMessage(m) && m.direction !== "out")
  const answered = messages.findLast((m) => isMessage(m) && m.direction === "out" && m.author.type === "member")
  const [title, setTitle] = useState(() => (c.subject || firstLine(asked?.body ?? "")).slice(0, 200))
  const [body, setBody] = useState(() => asked?.body.trim() ?? "")
  const [answer, setAnswer] = useState(() => answered?.body.trim() ?? "")
  const [pageUrl, setPageUrl] = useState("")
  const titleOk = title.trim().length >= 3
  const pageOk = !!page || isUrl(pageUrl.trim())
  const ready = titleOk && !!body.trim() && pageOk && !start.isPending
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-xl" data-testid="discussion-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            if (!ready) return
            start.mutate(
              {
                title: title.trim(),
                body: body.trim(),
                answer: answer.trim() || undefined,
                page_url: page ? undefined : pageUrl.trim(),
              },
              {
                onSuccess: (d) => {
                  toast(t`Discussion started`)
                  onClose()
                  navigate(threadLink(d.discussion.id))
                },
              }
            )
          }}
        >
          <DialogHeader>
            <DialogTitle>
              <Trans>Turn into a discussion</Trans>
            </DialogTitle>
            <DialogDescription>
              <Trans>
                Starts a public thread with the text below. The conversation stays private and unchanged; nothing else
                of it is shown.
              </Trans>
            </DialogDescription>
          </DialogHeader>
          <Notice tone="warning" icon={GlobeIcon} data-testid="discussion-public-note">
            <p>
              <Trans>
                Anyone who opens the page can read this. Remove names, e-mail addresses, order numbers and anything else
                personal.
              </Trans>
            </p>
          </Notice>
          {page ? (
            <Field label={<Trans>Page</Trans>}>
              <a
                href={page.page_url}
                target="_blank"
                rel="noreferrer"
                className="truncate text-body text-brand underline-offset-4 hover:underline"
                dir="ltr"
                data-testid="discussion-dialog-page"
              >
                {pageLabel(page.page_url, page.page_title)}
              </a>
            </Field>
          ) : (
            <Field
              label={<Trans>Page</Trans>}
              htmlFor="discussion-page"
              hint={<Trans>The documentation page to put it under, on a site of this inbox.</Trans>}
            >
              <Input
                id="discussion-page"
                type="url"
                required
                maxLength={2000}
                value={pageUrl}
                placeholder="https://docs.example.com/guide"
                onChange={(e) => setPageUrl(e.target.value)}
                data-testid="discussion-dialog-page-input"
              />
            </Field>
          )}
          <Field label={<Trans>Title</Trans>} htmlFor="discussion-title">
            <Input
              id="discussion-title"
              required
              minLength={3}
              maxLength={200}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              data-testid="discussion-dialog-title"
            />
          </Field>
          <Field label={<Trans>Question</Trans>} htmlFor="discussion-body" hint={<Trans>Markdown works.</Trans>}>
            <Textarea
              id="discussion-body"
              required
              rows={5}
              maxLength={20000}
              value={body}
              onChange={(e) => setBody(e.target.value)}
              className="max-h-64"
              data-testid="discussion-dialog-body"
            />
          </Field>
          <Field
            label={<Trans>Answer (optional)</Trans>}
            htmlFor="discussion-answer"
            hint={<Trans>Posted by you and marked as the accepted answer.</Trans>}
          >
            <Textarea
              id="discussion-answer"
              rows={5}
              maxLength={20000}
              value={answer}
              onChange={(e) => setAnswer(e.target.value)}
              className="max-h-64"
              data-testid="discussion-dialog-answer"
            />
          </Field>
          <ErrorLine error={start.error} />
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              <Trans>Cancel</Trans>
            </Button>
            <Button type="submit" disabled={!ready} data-testid="discussion-dialog-submit">
              <Trans>Make it public</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

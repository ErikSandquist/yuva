import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"

import { ErrorLine } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { slugify } from "@/features/settings/inboxes/slugify"
import { Field } from "@/features/settings/ui"
import { useCannedReplies } from "@/lib/workspace"
import { api, unwrap, type CannedReply } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

export function CannedReplyDialog({ reply, onClose }: { reply: CannedReply | null; onClose: () => void }) {
  const { t } = useLingui()
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  const [title, setTitle] = useState(reply?.title ?? "")
  const [shortcut, setShortcut] = useState(reply?.shortcut ?? "")
  const [touched, setTouched] = useState(!!reply)
  const [body, setBody] = useState(reply?.body ?? "")
  const others = useCannedReplies().data ?? []
  const valid = /^[a-z0-9]+(-[a-z0-9]+)*$/.test(shortcut)
  const taken = others.some((r) => r.shortcut === shortcut && r.id !== reply?.id)
  const problem = shortcut === "" ? null : !valid ? "invalid" : taken ? "taken" : null
  const save = useMutation({
    mutationFn: () =>
      reply
        ? unwrap(
            api.PATCH("/v1/canned-replies/{cannedReplyId}", {
              params: { path: { cannedReplyId: reply.id } },
              body: { title, shortcut, body },
            })
          )
        : unwrap(api.POST("/v1/canned-replies", { body: { title, shortcut, body } })),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: keys.cannedReplies(ws) })
      onClose()
    },
  })
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            save.mutate()
          }}
        >
          <DialogHeader>
            <DialogTitle>{reply ? <Trans>Edit canned reply</Trans> : <Trans>New canned reply</Trans>}</DialogTitle>
          </DialogHeader>
          <Field label={<Trans>Title</Trans>} htmlFor="canned-title">
            <Input
              id="canned-title"
              required
              maxLength={200}
              value={title}
              placeholder={t`Thanks for writing`}
              onChange={(e) => {
                setTitle(e.target.value)
                if (!touched) setShortcut(slugify(e.target.value))
              }}
            />
          </Field>
          <Field
            label={<Trans>Shortcut</Trans>}
            htmlFor="canned-shortcut"
            hint={<Trans>Lowercase letters, digits and dashes.</Trans>}
            error={
              problem === "invalid" ? (
                <Trans>Use lowercase letters, digits and single dashes.</Trans>
              ) : problem === "taken" ? (
                <Trans>Another canned reply already uses this shortcut.</Trans>
              ) : undefined
            }
          >
            <div className="relative">
              <span aria-hidden className="pointer-events-none absolute start-3 top-1/2 -translate-y-1/2 text-faint">
                /
              </span>
              <Input
                id="canned-shortcut"
                required
                maxLength={64}
                value={shortcut}
                aria-invalid={problem !== null || undefined}
                className="ps-6 font-mono"
                onChange={(e) => {
                  setTouched(true)
                  setShortcut(e.target.value)
                }}
              />
            </div>
          </Field>
          <Field label={<Trans>Text</Trans>} htmlFor="canned-body">
            <Textarea id="canned-body" required rows={6} value={body} onChange={(e) => setBody(e.target.value)} />
          </Field>
          <ErrorLine error={save.error} />
          <DialogFooter>
            <Button type="submit" disabled={save.isPending || !valid || taken}>
              <Trans>Save</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

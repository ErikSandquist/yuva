import { Trans } from "@lingui/react/macro"
import { useMutation } from "@tanstack/react-query"
import { MailIcon } from "lucide-react"
import { useState } from "react"

import { ErrorLine, Notice } from "@/components/common"
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
import { useRefreshList } from "@/features/broadcasts/queries"
import { Field } from "@/features/settings/ui"
import { api, ApiError, unwrap, type MailingList, type Subscription } from "@/lib/api"

export function AddStatusSubscriberDialog({
  list,
  onClose,
  onAdded,
}: {
  list: MailingList
  onClose: () => void
  onAdded: (s: Subscription) => void
}) {
  const refresh = useRefreshList(list.id)
  const [email, setEmail] = useState("")
  const add = useMutation({
    mutationFn: () =>
      unwrap(
        api.POST("/v1/lists/{listId}/subscriptions", {
          params: { path: { listId: list.id } },
          body: { email: email.trim(), consent: { confirmed: false } },
        })
      ),
    onSuccess: (s) => {
      refresh()
      onAdded(s)
    },
  })
  const code = add.error instanceof ApiError ? add.error.code : undefined
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md" data-testid="add-subscriber-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            add.mutate()
          }}
        >
          <DialogHeader>
            <DialogTitle>
              <Trans>Add a subscriber</Trans>
            </DialogTitle>
            <DialogDescription>
              <Trans>They get a mail with a link and receive notices only after they click it.</Trans>
            </DialogDescription>
          </DialogHeader>
          <Field
            label={<Trans>E-mail address</Trans>}
            htmlFor="status-sub-email"
            error={code === "invalid_email" ? <Trans>This is not a valid e-mail address.</Trans> : undefined}
          >
            <Input
              id="status-sub-email"
              type="email"
              required
              autoFocus
              maxLength={320}
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              aria-invalid={code === "invalid_email" || undefined}
              dir="ltr"
              data-testid="sub-email"
            />
          </Field>
          <Notice icon={MailIcon} className="text-small">
            <Trans>
              Until they click the link they stay waiting for confirmation. Unconfirmed addresses are deleted after 30
              days.
            </Trans>
          </Notice>
          {code !== "invalid_email" && <ErrorLine error={add.error} />}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              <Trans>Cancel</Trans>
            </Button>
            <Button type="submit" disabled={add.isPending || !email.trim()} data-testid="sub-add">
              <Trans>Send the confirmation</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

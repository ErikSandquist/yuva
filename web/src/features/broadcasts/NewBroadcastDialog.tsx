import { Trans, useLingui } from "@lingui/react/macro"
import { LoaderIcon } from "lucide-react"
import { useState } from "react"

import { Dot, ErrorLine } from "@/components/common"
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { useCreateBroadcast } from "@/features/broadcasts/broadcastQueries"
import { senderState } from "@/features/broadcasts/BroadcastParts"
import { useInboxChannels } from "@/features/broadcasts/queries"
import { Field } from "@/features/settings/ui"
import type { Broadcast } from "@/lib/api"
import { useInboxes } from "@/lib/workspace"

export function NewBroadcastDialog({
  inboxId,
  onClose,
  onCreated,
}: {
  inboxId: string
  onClose: () => void
  onCreated: (b: Broadcast) => void
}) {
  const { t } = useLingui()
  const inboxes = useInboxes().data ?? []
  const [inbox, setInbox] = useState(inboxId || inboxes[0]?.id || "")
  const [name, setName] = useState("")
  const channels = useInboxChannels(inbox).data
  const sender = channels?.find((c) => c.kind === "email" && senderState(c) === "ready")
  const create = useCreateBroadcast()
  const items = Object.fromEntries(inboxes.map((i) => [i.id, i.name]))
  const submit = () => {
    if (!name.trim() || !inbox) return
    create.mutate({ inbox_id: inbox, name: name.trim(), channel_id: sender?.id }, { onSuccess: onCreated })
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent data-testid="new-broadcast-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            submit()
          }}
        >
          <DialogHeader>
            <DialogTitle>
              <Trans>New broadcast</Trans>
            </DialogTitle>
            <DialogDescription>
              <Trans>It stays a draft until you send or schedule it. Replies come to the inbox you pick.</Trans>
            </DialogDescription>
          </DialogHeader>
          <Field label={<Trans>Name</Trans>} htmlFor="broadcast-name-new" hint={<Trans>Only your team sees it.</Trans>}>
            <Input
              id="broadcast-name-new"
              value={name}
              maxLength={200}
              autoFocus
              required
              placeholder={t`October newsletter`}
              onChange={(e) => setName(e.target.value)}
              data-testid="new-broadcast-name"
            />
          </Field>
          {inboxes.length > 1 && (
            <Field label={<Trans>Inbox</Trans>}>
              <Select value={inbox} items={items} onValueChange={(v) => v && setInbox(String(v))}>
                <SelectTrigger className="w-full min-w-0" aria-label={t`Inbox`} data-testid="new-broadcast-inbox">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {inboxes.map((i) => (
                    <SelectItem key={i.id} value={i.id}>
                      <Dot color={i.branding.color} className="mt-1.5 size-2 rounded-[3px]" />
                      {i.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
          )}
          <ErrorLine error={create.error} />
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              <Trans>Cancel</Trans>
            </Button>
            <Button type="submit" disabled={!name.trim() || !inbox || create.isPending} data-testid="create-broadcast">
              {create.isPending && <LoaderIcon className="animate-spin" />}
              <Trans>Create draft</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

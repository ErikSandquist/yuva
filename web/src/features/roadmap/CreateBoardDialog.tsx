import { Trans, useLingui } from "@lingui/react/macro"
import { LoaderIcon } from "lucide-react"
import { useState } from "react"

import { ErrorLine } from "@/components/common"
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
import { useInboxChannels } from "@/features/broadcasts/queries"
import { useCreateBoard } from "@/features/roadmap/queries"
import { ChoiceSelect, Field } from "@/features/settings/ui"
import { slugify, validSlug } from "@/features/status/CreatePageDialog"
import type { RoadmapBoard } from "@/lib/api"

export function CreateBoardDialog({
  inboxId,
  inboxName,
  onClose,
  onCreated,
}: {
  inboxId: string
  inboxName: string
  onClose: () => void
  onCreated: (b: RoadmapBoard) => void
}) {
  const { t } = useLingui()
  const channels = (useInboxChannels(inboxId).data ?? []).filter((c) => c.kind === "chat")
  const [name, setName] = useState(inboxName)
  const [slug, setSlug] = useState(slugify(inboxName))
  const [edited, setEdited] = useState(false)
  const [picked, setPicked] = useState("")
  const channel = picked || channels[0]?.id || ""
  const create = useCreateBoard()
  const ok = name.trim() && validSlug(slug) && channel
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent data-testid="new-board-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            if (ok)
              create.mutate(
                { inbox_id: inboxId, channel_id: channel, name: name.trim(), slug },
                { onSuccess: onCreated }
              )
          }}
        >
          <DialogHeader>
            <DialogTitle>
              <Trans>Create board</Trans>
            </DialogTitle>
            <DialogDescription>
              <Trans>
                Ideas, votes and stages for {inboxName}, with the feedback behind each idea. It stays private until you
                make it public.
              </Trans>
            </DialogDescription>
          </DialogHeader>
          <Field label={<Trans>Name</Trans>} htmlFor="board-new-name">
            <Input
              id="board-new-name"
              value={name}
              maxLength={100}
              autoFocus
              required
              onChange={(e) => {
                setName(e.target.value)
                if (!edited) setSlug(slugify(e.target.value))
              }}
              data-testid="new-board-name"
            />
          </Field>
          <Field
            label={<Trans>Address</Trans>}
            htmlFor="board-new-slug"
            hint={<Trans>3 to 48 letters, numbers and hyphens. It is unique on this server.</Trans>}
          >
            <Input
              id="board-new-slug"
              value={slug}
              maxLength={48}
              required
              spellCheck={false}
              autoCapitalize="none"
              onChange={(e) => {
                setEdited(true)
                setSlug(e.target.value.toLowerCase())
              }}
              data-testid="new-board-slug"
            />
          </Field>
          <Field
            label={<Trans>Chat channel</Trans>}
            htmlFor="board-new-channel"
            hint={
              channels.length === 0 ? (
                <Trans>This inbox has no web chat channel. Add one first, because the public board embeds it.</Trans>
              ) : (
                <Trans>The public board embeds this chat, and ideas your team starts are filed on it.</Trans>
              )
            }
          >
            <ChoiceSelect
              id="board-new-channel"
              value={channel}
              onChange={setPicked}
              options={channels.map((c) => [c.id, c.name] as const)}
              label={t`Chat channel`}
              disabled={channels.length === 0}
            />
          </Field>
          <ErrorLine error={create.error} />
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              <Trans>Cancel</Trans>
            </Button>
            <Button type="submit" disabled={!ok || create.isPending} data-testid="create-board">
              {create.isPending && <LoaderIcon className="animate-spin" />}
              <Trans>Create board</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

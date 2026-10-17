import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
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
import { Textarea } from "@/components/ui/textarea"
import { Field } from "@/features/settings/ui"
import { api, unwrap, type Channel } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

function parseSettings(raw: string): Record<string, unknown> | null {
  if (raw.trim() === "") return {}
  try {
    const v = JSON.parse(raw)
    return v && typeof v === "object" && !Array.isArray(v) ? v : null
  } catch {
    return null
  }
}

export function ApiChannelDialog({
  inboxId,
  onClose,
  onCreated,
}: {
  inboxId: string
  onClose: () => void
  onCreated: (ch: Channel) => void
}) {
  const { t } = useLingui()
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  const [name, setName] = useState("")
  const [raw, setRaw] = useState("{}")
  const settings = parseSettings(raw)
  const save = useMutation({
    mutationFn: () =>
      unwrap(
        api.POST("/v1/inboxes/{inboxId}/channels", {
          params: { path: { inboxId } },
          body: { kind: "api", name, settings: settings ?? {} },
        })
      ),
    onSuccess: (saved) => {
      void qc.invalidateQueries({ queryKey: keys.channels(ws, inboxId) })
      onCreated(saved)
    },
  })
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90svh] p-0 sm:max-w-lg">
        <form
          className="flex min-w-0 flex-col"
          onSubmit={(e) => {
            e.preventDefault()
            if (settings) save.mutate()
          }}
        >
          <div className="flex flex-col gap-5 p-6">
            <DialogHeader>
              <DialogTitle>
                <Trans>Add channel</Trans>
              </DialogTitle>
              <DialogDescription>
                <Trans>The kind cannot be changed later.</Trans>
              </DialogDescription>
            </DialogHeader>
            <Field label={<Trans>Name</Trans>} htmlFor="channel-name">
              <Input
                id="channel-name"
                required
                maxLength={200}
                value={name}
                placeholder={t`Billing events`}
                onChange={(e) => setName(e.target.value)}
              />
            </Field>
            <Field
              label={<Trans>Settings (JSON)</Trans>}
              htmlFor="channel-settings"
              error={settings ? undefined : <Trans>This is not a JSON object.</Trans>}
            >
              <Textarea
                id="channel-settings"
                rows={6}
                spellCheck={false}
                className="font-mono text-caption"
                value={raw}
                aria-invalid={!settings}
                onChange={(e) => setRaw(e.target.value)}
              />
            </Field>
            <ErrorLine error={save.error} />
          </div>
          <DialogFooter className="sticky bottom-0 border-t bg-card px-6 py-3">
            <Button type="submit" disabled={save.isPending || !settings}>
              <Trans>Save</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"

import { BotAvatar, ErrorLine } from "@/components/common"
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
import { Field } from "@/features/settings/ui"
import { api, unwrap, type ApiKey } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

export function ApiKeyDialog({ apiKey, onClose }: { apiKey: ApiKey; onClose: () => void }) {
  const { t } = useLingui()
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  const [name, setName] = useState(apiKey.name)
  const [botName, setBotName] = useState(apiKey.bot_name ?? "")
  const [avatar, setAvatar] = useState(apiKey.bot_avatar_url ?? "")
  const save = useMutation({
    mutationFn: () =>
      unwrap(
        api.PATCH("/v1/api-keys/{apiKeyId}", {
          params: { path: { apiKeyId: apiKey.id } },
          body: {
            name: name.trim(),
            bot_name: botName.trim() || null,
            bot_avatar_url: avatar.trim() || null,
          },
        })
      ),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: keys.apiKeys(ws) })
      onClose()
    },
  })
  const preview = botName.trim() || name.trim() || t`Bot`
  const avatarUrl = avatar.trim()
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90svh] p-0 sm:max-w-lg">
        <form
          className="flex min-w-0 flex-col"
          onSubmit={(e) => {
            e.preventDefault()
            save.mutate()
          }}
        >
          <div className="flex flex-col gap-5 p-6">
            <DialogHeader>
              <DialogTitle>
                <Trans>Edit API key</Trans>
              </DialogTitle>
              <DialogDescription>
                <Trans>Scopes, inboxes and expiry stay as they are. For different ones, create a new key.</Trans>
              </DialogDescription>
            </DialogHeader>
            <Field label={<Trans>Name</Trans>} htmlFor="key-name">
              <Input
                id="key-name"
                required
                maxLength={200}
                value={name}
                placeholder={t`Production backend`}
                onChange={(e) => setName(e.target.value)}
              />
            </Field>
            <div className="grid gap-5 sm:grid-cols-2">
              <Field
                label={<Trans>Bot name</Trans>}
                htmlFor="key-bot-name"
                hint={<Trans>Shown on its messages. The key's name when empty.</Trans>}
              >
                <Input
                  id="key-bot-name"
                  maxLength={200}
                  value={botName}
                  placeholder={name.trim() || t`Billing bot`}
                  onChange={(e) => setBotName(e.target.value)}
                />
              </Field>
              <Field
                label={<Trans>Bot avatar</Trans>}
                htmlFor="key-bot-avatar"
                hint={<Trans>An https image URL.</Trans>}
              >
                <div className="flex items-center gap-2">
                  <BotAvatar
                    name={preview}
                    url={avatarUrl.startsWith("https://") ? avatarUrl : undefined}
                    className="size-8"
                  />
                  <Input
                    id="key-bot-avatar"
                    type="url"
                    pattern="https://.*"
                    maxLength={2000}
                    value={avatar}
                    placeholder="https://"
                    onChange={(e) => setAvatar(e.target.value)}
                    className="min-w-0 font-mono text-caption"
                  />
                </div>
              </Field>
            </div>
            <ErrorLine error={save.error} />
          </div>
          <DialogFooter className="sticky bottom-0 border-t bg-card px-6 py-3">
            <Button type="submit" disabled={save.isPending}>
              <Trans>Save</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

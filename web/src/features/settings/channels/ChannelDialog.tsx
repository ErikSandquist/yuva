import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"

import { ErrorLine } from "@/components/common"
import { useEnumText } from "@/components/common/text"
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
import { Textarea } from "@/components/ui/textarea"
import { EmailFields } from "@/features/settings/channels/email/EmailFields"
import {
  emailErrorField,
  emailForm,
  emailInput,
  useEmailFieldErrors,
  type EmailField,
  type EmailForm,
} from "@/features/settings/channels/email/form"
import { Field } from "@/features/settings/ui"
import { api, ApiError, unwrap, type Channel } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

type DialogKind = "email" | "api"
const DIALOG_KINDS: DialogKind[] = ["email", "api"]

function parseSettings(raw: string): Record<string, unknown> | null {
  if (raw.trim() === "") return {}
  try {
    const v = JSON.parse(raw)
    return v && typeof v === "object" && !Array.isArray(v) ? v : null
  } catch {
    return null
  }
}

export function ChannelDialog({
  inboxId,
  onClose,
  onCreated,
  kind: preset,
}: {
  inboxId: string
  onClose: () => void
  onCreated: (ch: Channel) => void
  kind?: DialogKind
}) {
  const { t } = useLingui()
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  const text = useEnumText()
  const fieldErrors = useEmailFieldErrors()
  const [kind, setKind] = useState<DialogKind>(preset ?? "email")
  const [name, setName] = useState("")
  const [raw, setRaw] = useState("{}")
  const [email, setEmail] = useState<EmailForm>(() => emailForm())
  const settings = kind === "api" ? parseSettings(raw) : {}
  const save = useMutation({
    mutationFn: () => {
      const typed = kind === "email" ? { email: emailInput(email) } : { settings: settings ?? {} }
      return unwrap(
        api.POST("/v1/inboxes/{inboxId}/channels", {
          params: { path: { inboxId } },
          body: { kind, name, ...typed },
        })
      )
    },
    onSuccess: (saved) => {
      void qc.invalidateQueries({ queryKey: keys.channels(ws, inboxId) })
      onCreated(saved)
    },
  })
  const bad = emailErrorField(save.error)
  const taken = save.error instanceof ApiError && save.error.code === "email_address_taken"
  const fieldError = (f: EmailField) => (bad === f && !taken ? fieldErrors[f] : undefined)
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
            <div className="grid gap-5 sm:grid-cols-2">
              <Field label={<Trans>Kind</Trans>}>
                <Select value={kind} onValueChange={(v) => setKind(v as DialogKind)} items={text.channel}>
                  <SelectTrigger className="w-full" aria-label={t`Kind`}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {DIALOG_KINDS.map((k) => (
                      <SelectItem key={k} value={k}>
                        {text.channel[k]}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
              <Field label={<Trans>Name</Trans>} htmlFor="channel-name">
                <Input
                  id="channel-name"
                  required
                  maxLength={200}
                  value={name}
                  placeholder={kind === "api" ? t`Billing events` : t`Support e-mail`}
                  onChange={(e) => setName(e.target.value)}
                />
              </Field>
            </div>
            {kind === "email" && (
              <EmailFields
                f={email}
                set={(patch) => setEmail((f) => ({ ...f, ...patch }))}
                error={fieldError}
                taken={taken}
              />
            )}
            {kind === "api" && (
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
            )}
            {!bad && <ErrorLine error={save.error} />}
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

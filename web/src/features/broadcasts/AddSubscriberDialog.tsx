import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation } from "@tanstack/react-query"
import { MailIcon } from "lucide-react"
import { useState } from "react"

import { ErrorLine, Notice } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Textarea } from "@/components/ui/textarea"
import { useRefreshList } from "@/features/broadcasts/queries"
import { Field } from "@/features/settings/ui"
import { api, ApiError, unwrap, type MailingList, type Subscription, type SubscriptionCreate } from "@/lib/api"
import { useInboxes } from "@/lib/workspace"

type Mode = "agreed" | "confirm"

function localNow() {
  const d = new Date()
  d.setSeconds(0, 0)
  return new Date(d.getTime() - d.getTimezoneOffset() * 60_000).toISOString().slice(0, 16)
}

function Choice({
  value,
  title,
  hint,
  disabled,
}: {
  value: Mode
  title: React.ReactNode
  hint: React.ReactNode
  disabled?: boolean
}) {
  return (
    <Label className="items-start gap-3 rounded-xl border px-3.5 py-3 font-normal has-data-checked:border-primary has-disabled:opacity-60">
      <RadioGroupItem value={value} disabled={disabled} className="mt-0.5" data-testid={`consent-${value}`} />
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="font-medium">{title}</span>
        <span className="text-small text-faint">{hint}</span>
      </span>
    </Label>
  )
}

export function AddSubscriberDialog({
  list,
  onClose,
  onAdded,
}: {
  list: MailingList
  onClose: () => void
  onAdded: (s: Subscription) => void
}) {
  const { t } = useLingui()
  const refresh = useRefreshList(list.id)
  const inbox = (useInboxes().data ?? []).find((i) => i.id === list.inbox_id)
  const listName = list.name
  const inboxName = inbox?.name ?? ""
  const [email, setEmail] = useState("")
  const [name, setName] = useState("")
  const [mode, setMode] = useState<Mode>("agreed")
  const [when, setWhen] = useState(localNow)
  const [now] = useState(localNow)
  const [statement, setStatement] = useState(
    () => t`I want to receive e-mails about ${listName} from ${inboxName}. I can unsubscribe at any time.`
  )
  const [page, setPage] = useState("")
  const [reference, setReference] = useState("")
  const [attested, setAttested] = useState(false)
  const [touched, setTouched] = useState(false)
  const agreed = mode === "agreed"
  const add = useMutation({
    mutationFn: () => {
      const body: SubscriptionCreate = {
        email: email.trim(),
        consent: agreed
          ? {
              confirmed: true,
              obtained_at: new Date(when).toISOString(),
              text: statement.trim(),
              page_url: page.trim() || undefined,
              reference: reference.trim() || undefined,
            }
          : { confirmed: false, reference: reference.trim() || undefined },
      }
      if (name.trim()) body.name = name.trim()
      return unwrap(api.POST("/v1/lists/{listId}/subscriptions", { params: { path: { listId: list.id } }, body }))
    },
    onSuccess: (s) => {
      refresh()
      onAdded(s)
    },
  })
  const code = add.error instanceof ApiError ? add.error.code : undefined
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90svh] p-0 sm:max-w-lg" data-testid="add-subscriber-dialog">
        <form
          className="flex min-w-0 flex-col"
          onSubmit={(e) => {
            e.preventDefault()
            setTouched(true)
            if (attested) add.mutate()
          }}
        >
          <div className="flex flex-col gap-5 p-6">
            <DialogHeader>
              <DialogTitle>
                <Trans>Add a subscriber to {listName}</Trans>
              </DialogTitle>
              <DialogDescription>
                <Trans>
                  Only add people who agreed to get this list. What you enter here is kept as their consent record.
                </Trans>
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-5 sm:grid-cols-2">
              <Field
                label={<Trans>E-mail address</Trans>}
                htmlFor="sub-email"
                error={code === "invalid_email" ? <Trans>This is not a valid e-mail address.</Trans> : undefined}
              >
                <Input
                  id="sub-email"
                  type="email"
                  required
                  maxLength={320}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  aria-invalid={code === "invalid_email" || undefined}
                  dir="ltr"
                  data-testid="sub-email"
                />
              </Field>
              <Field label={<Trans>Name (optional)</Trans>} htmlFor="sub-name">
                <Input id="sub-name" maxLength={200} value={name} onChange={(e) => setName(e.target.value)} />
              </Field>
            </div>
            <div className="flex flex-col gap-2" role="group" aria-labelledby="sub-mode">
              <span id="sub-mode" className="text-body font-medium">
                <Trans>How did they agree?</Trans>
              </span>
              <RadioGroup value={mode} onValueChange={(v) => setMode(v as Mode)} className="gap-2">
                <Choice
                  value="agreed"
                  title={<Trans>They already agreed</Trans>}
                  hint={
                    <Trans>
                      For example on your own sign-up form, at checkout or on paper. They are subscribed now.
                    </Trans>
                  }
                />
                <Choice
                  value="confirm"
                  title={<Trans>Mail them a confirmation</Trans>}
                  hint={<Trans>They get a mail with a link and are subscribed only after they click it.</Trans>}
                  disabled={!list.sender_ready}
                />
              </RadioGroup>
              {!list.sender_ready && (
                <p className="text-small text-faint">
                  <Trans>This list has no e-mail channel that can send, so it cannot mail a confirmation yet.</Trans>
                </p>
              )}
            </div>
            {agreed ? (
              <>
                <Field label={<Trans>When did they agree?</Trans>} htmlFor="sub-when">
                  <Input
                    id="sub-when"
                    type="datetime-local"
                    required
                    max={now}
                    value={when}
                    onChange={(e) => setWhen(e.target.value)}
                    className="sm:max-w-60"
                  />
                </Field>
                <Field
                  label={<Trans>What did they agree to?</Trans>}
                  htmlFor="sub-text"
                  hint={<Trans>The exact words they saw next to the checkbox or signature.</Trans>}
                >
                  <Textarea
                    id="sub-text"
                    required
                    rows={3}
                    maxLength={2000}
                    value={statement}
                    onChange={(e) => setStatement(e.target.value)}
                    data-testid="sub-text"
                  />
                </Field>
                <Field label={<Trans>Page where they agreed (optional)</Trans>} htmlFor="sub-page">
                  <Input
                    id="sub-page"
                    type="url"
                    maxLength={2048}
                    value={page}
                    placeholder="https://example.com/signup"
                    onChange={(e) => setPage(e.target.value)}
                    dir="ltr"
                  />
                </Field>
              </>
            ) : (
              <Notice icon={MailIcon} className="text-small">
                <Trans>
                  Until they click the link they stay waiting for confirmation and get no other mail. Unconfirmed
                  subscriptions are deleted after 30 days.
                </Trans>
              </Notice>
            )}
            <Field
              label={<Trans>How they agreed, or your own record (optional)</Trans>}
              htmlFor="sub-reference"
              hint={<Trans>Something you can find again if asked, such as an order number or a signed form.</Trans>}
            >
              <Input
                id="sub-reference"
                maxLength={500}
                value={reference}
                placeholder={t`Sign-up sheet at the spring fair`}
                onChange={(e) => setReference(e.target.value)}
              />
            </Field>
            <div className="flex flex-col gap-1.5">
              <Label className="items-start gap-3 font-normal">
                <Checkbox
                  checked={attested}
                  onCheckedChange={setAttested}
                  className="mt-0.5"
                  aria-invalid={(touched && !attested) || undefined}
                  data-testid="sub-attest"
                />
                <span className="text-body">
                  {agreed ? (
                    <Trans>
                      I confirm that this person agreed to get e-mails from this list as described above, and that we
                      can show how and when if asked.
                    </Trans>
                  ) : (
                    <Trans>I confirm that this person asked to join this list.</Trans>
                  )}
                </span>
              </Label>
              {touched && !attested && (
                <p role="alert" className="text-small text-destructive">
                  <Trans>Confirm the consent to add them.</Trans>
                </p>
              )}
            </div>
            {code !== "invalid_email" && <ErrorLine error={add.error} />}
          </div>
          <DialogFooter className="sticky bottom-0 border-t bg-card px-6 py-3">
            <Button type="submit" disabled={add.isPending} data-testid="sub-add">
              {agreed ? <Trans>Add subscriber</Trans> : <Trans>Send the confirmation</Trans>}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

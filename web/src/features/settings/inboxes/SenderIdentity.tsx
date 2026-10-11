import { Trans, useLingui } from "@lingui/react/macro"
import { useState } from "react"

import { toast } from "@/components/common"
import { useErrorText } from "@/components/common/text"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { useSaveInbox } from "@/features/settings/inboxes/queries"
import { Card, Rows, Section, SettingRow, StatusTag } from "@/features/settings/ui"
import type { Inbox, InboxSender } from "@/lib/api"
import { useSession } from "@/lib/session"

type Key = keyof InboxSender

const fieldClass = "h-9 w-72 min-w-0 rounded-[10px] bg-background phone:w-full"

export function senderComplete(s: InboxSender) {
  return !!(s.legal_name && s.postal_address && s.contact)
}

export function SenderIdentity({ inbox }: { inbox: Inbox }) {
  const { t } = useLingui()
  const errorText = useErrorText()
  const { canManage } = useSession()
  const save = useSaveInbox(inbox)
  const sender = inbox.sender ?? {}
  const [values, setValues] = useState<Record<Key, string>>({
    legal_name: sender.legal_name ?? "",
    postal_address: sender.postal_address ?? "",
    registration: sender.registration ?? "",
    contact: sender.contact ?? "",
  })
  const commit = (k: Key) => {
    const v = values[k].trim()
    if (v === (sender[k] ?? "")) return
    save.mutate(
      { sender: { [k]: v || null } },
      { onSuccess: () => toast(t`Saved`), onError: (e) => toast(errorText(e)) }
    )
  }
  const field = (k: Key) => ({
    value: values[k],
    onChange: (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
      setValues((v) => ({ ...v, [k]: e.target.value })),
    onBlur: () => commit(k),
  })
  const complete = senderComplete(sender)
  const footer = [values.legal_name, values.postal_address, values.registration, values.contact]
    .map((v) => v.trim())
    .filter(Boolean)
  return (
    <Section
      title={<Trans>Sender identity</Trans>}
      action={
        complete ? (
          <StatusTag tone="success">
            <Trans>Complete</Trans>
          </StatusTag>
        ) : (
          <StatusTag tone="warning" data-testid="sender-missing">
            <Trans>Needed for broadcasts</Trans>
          </StatusTag>
        )
      }
      description={
        <Trans>
          The law asks every marketing mail to say who sends it and how to reach them (CAN-SPAM, CASL, Türkiye's
          Regulation on Commercial Communication). Yuva puts this in the footer of every broadcast from this inbox, so
          fill it in before the first one.
        </Trans>
      }
    >
      <Card flush>
        <fieldset disabled={!canManage} data-testid="sender-identity">
          <Rows>
            <SettingRow
              title={<Trans>Legal name</Trans>}
              hint={<Trans>Your company or your own name</Trans>}
              htmlFor="sender-name"
            >
              <Input
                id="sender-name"
                className={fieldClass}
                maxLength={200}
                placeholder={t`Example Ltd.`}
                {...field("legal_name")}
              />
            </SettingRow>
            <li className="flex flex-col gap-2 px-4 py-3">
              <Label htmlFor="sender-address" className="block font-normal">
                <Trans>Postal address</Trans>
                <small className="mt-px block text-small text-faint">
                  <Trans>A real address where you receive post</Trans>
                </small>
              </Label>
              <Textarea
                id="sender-address"
                rows={2}
                maxLength={500}
                className="rounded-[10px] bg-background"
                {...field("postal_address")}
              />
            </li>
            <SettingRow
              title={<Trans>Registration</Trans>}
              hint={<Trans>Such as a MERSİS number, trade register or VAT id, if you have one</Trans>}
              htmlFor="sender-registration"
            >
              <Input id="sender-registration" className={fieldClass} maxLength={200} {...field("registration")} />
            </SettingRow>
            <SettingRow
              title={<Trans>Contact</Trans>}
              hint={<Trans>An e-mail address, phone number or web address</Trans>}
              htmlFor="sender-contact"
            >
              <Input
                id="sender-contact"
                className={fieldClass}
                maxLength={320}
                placeholder="hello@example.com"
                {...field("contact")}
              />
            </SettingRow>
          </Rows>
        </fieldset>
        {footer.length > 0 && (
          <div className="border-t px-4 py-3" data-testid="sender-footer-preview">
            <p className="mb-1.5 text-caption text-faint">
              <Trans>In the footer</Trans>
            </p>
            <p className="text-center text-caption whitespace-pre-line text-muted-foreground">{footer.join("\n")}</p>
          </div>
        )}
      </Card>
    </Section>
  )
}

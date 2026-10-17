import { Trans } from "@lingui/react/macro"
import { TriangleAlertIcon } from "lucide-react"
import { useState } from "react"

import { Notice } from "@/components/common"
import {
  AutoReplyFields,
  BroadcastFields,
  ReceivingFields,
  SendingFields,
  SesFields,
  sesShown,
} from "@/features/settings/channels/email/EmailFields"
import { ForwardingAddressField } from "@/features/settings/channels/email/ForwardingAddressField"
import {
  emailErrorField,
  emailForm,
  emailInput,
  useEmailFieldErrors,
  type EmailField,
  type EmailForm,
} from "@/features/settings/channels/email/form"
import { PageSection, useLatestChannel, usePatchChannel } from "@/features/settings/channels/sections"
import { FormActions, FormCard } from "@/features/settings/ui"
import { ApiError, type Channel } from "@/lib/api"

function domainOf(address: string) {
  const at = address.lastIndexOf("@")
  return at < 0 ? "" : address.slice(at + 1).toLowerCase()
}

function sendingDomain(f: EmailForm) {
  return domainOf(f.fromAddress.trim() || f.address.trim())
}

function EmailSection({
  channel,
  id,
  title,
  description,
  children,
  noSave,
}: {
  channel: Channel
  id: string
  title: React.ReactNode
  description?: React.ReactNode
  children: (p: {
    f: EmailForm
    set: (patch: Partial<EmailForm>) => void
    error: (field: EmailField) => string | undefined
    taken: boolean
    dkimWarning: React.ReactNode
  }) => React.ReactNode
  noSave?: boolean
}) {
  const patch = usePatchChannel(channel)
  const latest = useLatestChannel(channel)
  const fieldErrors = useEmailFieldErrors()
  const [draft, setDraft] = useState<Partial<EmailForm>>({})
  const base = emailForm(channel.email)
  const f: EmailForm = { ...base, ...draft }
  const dirty = (Object.keys(draft) as (keyof EmailForm)[]).some((k) => draft[k] !== base[k])
  const bad = emailErrorField(patch.error)
  const taken = patch.error instanceof ApiError && patch.error.code === "email_address_taken"
  const error = (field: EmailField) => (bad === field && !taken ? fieldErrors[field] : undefined)
  const dkimWarning =
    channel.email?.dkim?.enabled && sendingDomain(f) !== sendingDomain(base) ? (
      <Notice tone="warning" icon={TriangleAlertIcon} className="text-caption" data-testid="dkim-domain-warning">
        <Trans>Changing the sending domain turns DKIM signing off until you enable it again for the new domain.</Trans>
      </Notice>
    ) : null
  const submit = () =>
    patch.mutate({ email: emailInput({ ...emailForm(latest().email), ...draft }) }, { onSuccess: () => setDraft({}) })
  const set = (p: Partial<EmailForm>) => {
    patch.reset()
    setDraft((d) => ({ ...d, ...p }))
  }
  const body = children({ f, set, error, taken, dkimWarning })
  return (
    <PageSection id={id} title={title} description={description}>
      {noSave ? (
        <div className="flex flex-col gap-5 rounded-2xl border bg-card p-4">{body}</div>
      ) : (
        <FormCard
          onSubmit={submit}
          footer={
            <FormActions
              pending={patch.isPending}
              saved={patch.isSuccess && !dirty}
              error={bad && !taken ? undefined : patch.error}
              disabled={!dirty}
            />
          }
        >
          {body}
        </FormCard>
      )}
    </PageSection>
  )
}

export function ReceivingSection({ channel }: { channel: Channel }) {
  return (
    <EmailSection channel={channel} id="receiving" title={<Trans>Receiving</Trans>}>
      {({ f, set, error, taken, dkimWarning }) => (
        <>
          <ReceivingFields f={f} set={set} error={error} taken={taken} />
          <ForwardingAddressField channel={channel} />
          {dkimWarning}
        </>
      )}
    </EmailSection>
  )
}

export function RepliesSection({ channel }: { channel: Channel }) {
  return (
    <EmailSection channel={channel} id="replies" title={<Trans>Replies</Trans>}>
      {({ f, set, error, taken, dkimWarning }) => (
        <>
          <SendingFields f={f} set={set} error={error} taken={taken} />
          {dkimWarning}
        </>
      )}
    </EmailSection>
  )
}

export function SesSection({ channel }: { channel: Channel }) {
  const ses = channel.email?.ses
  if (!sesShown({ f: emailForm(channel.email), ses })) return null
  return (
    <EmailSection channel={channel} id="ses" title={<Trans>Amazon SES</Trans>} noSave={ses?.source === "server"}>
      {({ f, set, error, taken }) => <SesFields f={f} set={set} error={error} taken={taken} ses={ses} />}
    </EmailSection>
  )
}

export function BroadcastsSection({ channel }: { channel: Channel }) {
  return (
    <EmailSection channel={channel} id="broadcasts" title={<Trans>Broadcast sending</Trans>}>
      {({ f, set, error, taken }) => <BroadcastFields f={f} set={set} error={error} taken={taken} />}
    </EmailSection>
  )
}

export function AutoReplySection({ channel }: { channel: Channel }) {
  return (
    <EmailSection channel={channel} id="auto-reply" title={<Trans>Auto-reply</Trans>}>
      {({ f, set, error, taken }) => <AutoReplyFields f={f} set={set} error={error} taken={taken} />}
    </EmailSection>
  )
}

import { Trans, useLingui } from "@lingui/react/macro"
import { useState } from "react"

import { toast } from "@/components/common"
import { useErrorText } from "@/components/common/text"
import {
  DiscussionsFields,
  LookFields,
  SitesFields,
  VisitorsFields,
} from "@/features/settings/channels/chat/ChatFields"
import { ChatInstall } from "@/features/settings/channels/chat/ChatInstall"
import { chatChannelInput, chatErrorField, chatForm, type ChatForm } from "@/features/settings/channels/chat/form"
import { PageSection, useLatestChannel, usePatchChannel } from "@/features/settings/channels/sections"
import { Card, FormActions, FormCard } from "@/features/settings/ui"
import type { Channel } from "@/lib/api"

function useChatSection(channel: Channel) {
  const patch = usePatchChannel(channel)
  const latest = useLatestChannel(channel)
  const [draft, setDraft] = useState<Partial<ChatForm>>({})
  const base = chatForm(channel.chat)
  const f: ChatForm = { ...base, ...draft }
  const dirty = (Object.keys(draft) as (keyof ChatForm)[]).some((k) => k !== "touched" && draft[k] !== base[k])
  const input = chatChannelInput({ ...chatForm(latest().chat), ...draft })
  const set = (p: Partial<ChatForm>) => {
    patch.reset()
    setDraft((d) => ({ ...d, ...p }))
  }
  const submit = () => {
    setDraft((d) => ({ ...d, touched: true }))
    if (input.ok) patch.mutate({ chat: input.value }, { onSuccess: () => setDraft({}) })
  }
  return { patch, f, set, dirty, input, submit, serverError: chatErrorField(patch.error) }
}

function FormSection({
  channel,
  id,
  title,
  children,
}: {
  channel: Channel
  id: string
  title: React.ReactNode
  children: (s: ReturnType<typeof useChatSection>) => React.ReactNode
}) {
  const s = useChatSection(channel)
  return (
    <PageSection id={id} title={title}>
      <FormCard
        onSubmit={s.submit}
        footer={
          <FormActions
            pending={s.patch.isPending}
            saved={s.patch.isSuccess && !s.dirty}
            error={s.serverError ? undefined : s.patch.error}
            disabled={!s.dirty}
          />
        }
      >
        {children(s)}
      </FormCard>
    </PageSection>
  )
}

function SwitchSection({
  channel,
  id,
  title,
  children,
}: {
  channel: Channel
  id: string
  title: React.ReactNode
  children: (p: { f: ChatForm; set: (p: Partial<ChatForm>) => void }) => React.ReactNode
}) {
  const { t } = useLingui()
  const errorText = useErrorText()
  const patch = usePatchChannel(channel)
  const latest = useLatestChannel(channel)
  const [pending, setPending] = useState<Partial<ChatForm>>({})
  const f: ChatForm = { ...chatForm(channel.chat), ...pending }
  const set = (p: Partial<ChatForm>) => {
    const input = chatChannelInput({ ...chatForm(latest().chat), ...p })
    if (!input.ok) return
    setPending((x) => ({ ...x, ...p }))
    patch.mutate(
      { chat: input.value },
      {
        onSuccess: () => toast(t`Saved`),
        onError: (e) => toast(errorText(e)),
        onSettled: () => setPending({}),
      }
    )
  }
  return (
    <PageSection id={id} title={title}>
      <Card>{children({ f, set })}</Card>
    </PageSection>
  )
}

export function SitesSection({ channel }: { channel: Channel }) {
  return (
    <FormSection channel={channel} id="sites" title={<Trans>Sites</Trans>}>
      {({ f, set, input, serverError }) => (
        <SitesFields f={f} set={set} invalid={input.ok ? null : input} serverError={serverError} />
      )}
    </FormSection>
  )
}

export function VisitorsSection({ channel }: { channel: Channel }) {
  return (
    <SwitchSection channel={channel} id="visitors" title={<Trans>Visitors</Trans>}>
      {({ f, set }) => <VisitorsFields f={f} set={set} />}
    </SwitchSection>
  )
}

export function LookSection({ channel }: { channel: Channel }) {
  return (
    <FormSection channel={channel} id="look" title={<Trans>Look</Trans>}>
      {({ f, set, serverError }) => <LookFields f={f} set={set} serverError={serverError} />}
    </FormSection>
  )
}

export function DiscussionsSection({ channel }: { channel: Channel }) {
  return (
    <SwitchSection channel={channel} id="discussions" title={<Trans>Discussions</Trans>}>
      {({ f, set }) => <DiscussionsFields f={f} set={set} />}
    </SwitchSection>
  )
}

export function ChatInstallSection({ channel }: { channel: Channel }) {
  return (
    <PageSection id="install" title={<Trans>Install</Trans>}>
      <Card>
        <ChatInstall channel={channel} />
      </Card>
    </PageSection>
  )
}

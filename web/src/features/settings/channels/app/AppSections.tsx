import { Trans, useLingui } from "@lingui/react/macro"
import { useState } from "react"

import { toast } from "@/components/common"
import { useErrorText } from "@/components/common/text"
import { AppAccessFields } from "@/features/settings/channels/app/AppFields"
import { AppInstall } from "@/features/settings/channels/app/AppInstall"
import { appChannelInput, appForm, type AppForm } from "@/features/settings/channels/app/form"
import { PageSection, useLatestChannel, usePatchChannel } from "@/features/settings/channels/sections"
import { Card } from "@/features/settings/ui"
import type { Channel } from "@/lib/api"

export function AppAccessSection({ channel }: { channel: Channel }) {
  const { t } = useLingui()
  const errorText = useErrorText()
  const patch = usePatchChannel(channel)
  const latest = useLatestChannel(channel)
  const [pending, setPending] = useState<Partial<AppForm>>({})
  const f: AppForm = { ...appForm(channel.app), ...pending }
  const set = (p: Partial<AppForm>) => {
    const next = { ...appForm(latest().app), ...pending, ...p, touched: true }
    setPending((x) => ({ ...x, ...p, touched: true }))
    const input = appChannelInput(next)
    if (!input) return
    patch.mutate(
      { app: input },
      {
        onSuccess: () => toast(t`Saved`),
        onError: (e) => toast(errorText(e)),
        onSettled: () => setPending({}),
      }
    )
  }
  return (
    <PageSection id="access" title={<Trans>Access</Trans>}>
      <Card>
        <AppAccessFields f={f} set={set} />
      </Card>
    </PageSection>
  )
}

export function AppInstallSection({ channel }: { channel: Channel }) {
  return (
    <PageSection id="install" title={<Trans>Install</Trans>}>
      <Card>
        <AppInstall channel={channel} />
      </Card>
    </PageSection>
  )
}

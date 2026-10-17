import { Trans } from "@lingui/react/macro"
import { useEffect } from "react"
import { Navigate, useLocation, useParams } from "react-router"

import { ErrorLine } from "@/components/common"
import { Skeleton } from "@/components/ui/skeleton"
import { SesStatus } from "@/features/settings/channels/email/Ses"
import { DeliverabilityCard, DkimSection } from "@/features/settings/channels/email/Deliverability"
import { InboxProvider } from "@/features/settings/inboxes/InboxPage"
import { PageHeader } from "@/features/settings/ui"
import { useChannel } from "@/lib/workspace"

function useHashScroll(ready: boolean) {
  const { hash } = useLocation()
  useEffect(() => {
    if (!ready || !hash) return
    document.getElementById(hash.slice(1))?.scrollIntoView({ block: "start" })
  }, [ready, hash])
}

export function ChannelSendingPage() {
  const { inboxId = "", channelId = "" } = useParams()
  const channel = useChannel(channelId)
  const ch = channel.data
  useHashScroll(!!ch)
  if (ch && (ch.kind !== "email" || ch.inbox_id !== inboxId))
    return <Navigate to={`/settings/inboxes/${inboxId}`} replace />
  return (
    <InboxProvider>
      {(inbox) =>
        ch ? (
          <>
            <PageHeader
              title={ch.name}
              eyebrow={<Trans>Broadcast sending</Trans>}
              description={<span dir="ltr">{ch.email?.from_address || ch.email?.address}</span>}
              back={{ to: `/settings/inboxes/${inbox.id}`, label: inbox.name }}
            />
            {ch.email?.ses && <SesStatus ses={ch.email.ses} address={ch.email.from_address || ch.email.address} />}
            <DeliverabilityCard channel={ch} />
            <DkimSection key={ch.email?.dkim?.selector ?? "none"} channel={ch} />
          </>
        ) : channel.isPending ? (
          <Skeleton className="h-64 w-full rounded-2xl" />
        ) : (
          <ErrorLine error={channel.error} />
        )
      }
    </InboxProvider>
  )
}

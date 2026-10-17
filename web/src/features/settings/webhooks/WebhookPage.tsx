import { Trans, useLingui } from "@lingui/react/macro"
import { Navigate, useParams } from "react-router"

import { ErrorLine } from "@/components/common"
import { Skeleton } from "@/components/ui/skeleton"
import { OnThisPage, useHashScroll } from "@/features/settings/OnThisPage"
import { PageHeader } from "@/features/settings/ui"
import { AttemptLog } from "@/features/settings/webhooks/AttemptLog"
import { Deliveries } from "@/features/settings/webhooks/Deliveries"
import { EndpointStatus } from "@/features/settings/webhooks/parts"
import { useWebhook } from "@/features/settings/webhooks/queries"
import {
  EndpointSection,
  EventsSection,
  RemoveSection,
  SecretSection,
  SendingSection,
} from "@/features/settings/webhooks/sections"
import { SignatureHint } from "@/features/settings/webhooks/SignatureHint"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

export function WebhookPage() {
  const { t } = useLingui()
  const { webhookId = "" } = useParams()
  const { canManage } = useSession()
  const inboxes = useInboxes().data ?? []
  const endpoint = useWebhook(webhookId)
  useHashScroll(!!endpoint.data)
  const sections: [string, string][] = [
    ["endpoint", t`Endpoint`],
    ["events", t`Events`],
    ["sending", t`Sending`],
    ["secret", t`Signing secret`],
    ["deliveries", t`Deliveries`],
    ["attempts", t`Attempts`],
    ["verify", t`How to verify`],
    ["remove", t`Remove`],
  ]
  if (!canManage) return <Navigate to="/settings/profile" replace />
  const e = endpoint.data
  const inbox = e?.inbox_id ? inboxes.find((i) => i.id === e.inbox_id) : undefined
  return (
    <>
      <PageHeader
        back={{ to: "/settings/webhooks", label: t`Webhooks` }}
        title={
          e ? (
            <span className="font-mono text-title break-all" data-testid="webhook-url">
              {e.url}
            </span>
          ) : (
            <Trans>Webhook endpoint</Trans>
          )
        }
        description={
          e && (
            <>
              {e.description && <>{e.description} · </>}
              {inbox ? inbox.name : <Trans>All inboxes</Trans>}
            </>
          )
        }
        action={e && <EndpointStatus e={e} />}
      />
      {endpoint.isPending ? (
        <Skeleton className="h-48 w-full rounded-2xl" />
      ) : !e ? (
        <ErrorLine error={endpoint.error} />
      ) : (
        <>
          <OnThisPage items={sections} testId="webhook-sections" />
          <EndpointSection e={e} />
          <EventsSection e={e} />
          <SendingSection e={e} />
          <SecretSection e={e} />
          <div id="deliveries" className="scroll-mt-4">
            <Deliveries endpoint={e} />
          </div>
          <div id="attempts" className="scroll-mt-4">
            <AttemptLog endpoint={e} />
          </div>
          <div id="verify" className="scroll-mt-4 flex flex-col gap-2">
            <h2 className="mx-1 text-small font-medium text-faint">
              <Trans>How to verify</Trans>
            </h2>
            <SignatureHint />
          </div>
          <RemoveSection e={e} />
        </>
      )}
    </>
  )
}

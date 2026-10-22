import { Trans, useLingui } from "@lingui/react/macro"
import { Navigate, useParams } from "react-router"

import { ErrorLine } from "@/components/common"
import { Skeleton } from "@/components/ui/skeleton"
import { PageHeader, PageTabs, ProductTag } from "@/features/settings/ui"
import { useTab } from "@/features/settings/useTab"
import { AttemptLog } from "@/features/settings/webhooks/AttemptLog"
import { Deliveries } from "@/features/settings/webhooks/Deliveries"
import { EndpointStatus } from "@/features/settings/webhooks/parts"
import { useWebhook } from "@/features/settings/webhooks/queries"
import {
  DisabledBanner,
  EndpointSection,
  EventsSection,
  RemoveSection,
  SecretSection,
  SendingSection,
} from "@/features/settings/webhooks/sections"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

export function WebhookPage() {
  const { t } = useLingui()
  const { webhookId = "" } = useParams()
  const { canManage } = useSession()
  const inboxes = useInboxes().data ?? []
  const endpoint = useWebhook(webhookId)
  const { tab, to } = useTab(["settings", "deliveries"] as const, "settings")
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
            <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
              {e.description && <span>{e.description}</span>}
              {inbox ? (
                <ProductTag name={inbox.name} color={inbox.branding.color} />
              ) : (
                <span>
                  <Trans>All products</Trans>
                </span>
              )}
            </span>
          )
        }
        action={e && <EndpointStatus e={e} />}
        tabs={
          e && (
            <PageTabs
              label={t`Endpoint`}
              current={tab}
              items={[
                { key: "settings", to: to("settings"), label: t`Settings`, testId: "webhook-tab-settings" },
                { key: "deliveries", to: to("deliveries"), label: t`Deliveries`, testId: "webhook-tab-deliveries" },
              ]}
            />
          )
        }
      />
      {endpoint.isPending ? (
        <Skeleton className="h-48 w-full rounded-2xl" />
      ) : !e ? (
        <ErrorLine error={endpoint.error} />
      ) : (
        <>
          <DisabledBanner e={e} />
          {tab === "settings" ? (
            <>
              <EndpointSection e={e} />
              <EventsSection e={e} />
              <SendingSection e={e} />
              <SecretSection e={e} />
              <RemoveSection e={e} />
            </>
          ) : (
            <>
              <Deliveries endpoint={e} />
              <AttemptLog endpoint={e} />
            </>
          )}
        </>
      )}
    </>
  )
}

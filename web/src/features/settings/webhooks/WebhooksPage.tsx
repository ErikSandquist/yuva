import { Trans } from "@lingui/react/macro"
import { Navigate } from "react-router"

import { SettingsStrip } from "@/components/common/SettingsStrip"
import { CreateMenu, DeveloperTabs, DeveloperTitle } from "@/features/settings/DeveloperTabs"
import { PageHeader } from "@/features/settings/ui"
import { WebhookTable } from "@/features/settings/webhooks/WebhookList"
import { useSession } from "@/lib/session"

export function WebhooksPage() {
  const { canManage } = useSession()
  if (!canManage) return <Navigate to="/settings/profile" replace />
  return (
    <>
      <PageHeader
        title={<DeveloperTitle />}
        description={<Trans>Keys, webhooks and connected assistants</Trans>}
        tabs={<DeveloperTabs current="webhooks" />}
        action={<CreateMenu current="webhooks" />}
      />
      <SettingsStrip current="developer" />
      <WebhookTable />
    </>
  )
}

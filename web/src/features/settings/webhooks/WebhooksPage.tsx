import { Navigate } from "react-router"

import { DeveloperTabs, DeveloperTitle } from "@/features/settings/DeveloperTabs"
import { PageHeader } from "@/features/settings/ui"
import { WebhookList } from "@/features/settings/webhooks/WebhookList"
import { useSession } from "@/lib/session"

export function WebhooksPage() {
  const { canManage } = useSession()
  if (!canManage) return <Navigate to="/settings/profile" replace />
  return (
    <>
      <PageHeader title={<DeveloperTitle />} tabs={<DeveloperTabs current="webhooks" />} />
      <WebhookList />
    </>
  )
}

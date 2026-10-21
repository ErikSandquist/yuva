import { Trans } from "@lingui/react/macro"
import { Navigate, useLocation, useNavigate } from "react-router"

import { Button } from "@/components/ui/button"
import { DeveloperTabs, DeveloperTitle } from "@/features/settings/DeveloperTabs"
import { headButton, PageHeader } from "@/features/settings/ui"
import { WebhookTable } from "@/features/settings/webhooks/WebhookList"
import { useSession } from "@/lib/session"

export function WebhooksPage() {
  const { canManage } = useSession()
  const navigate = useNavigate()
  const location = useLocation()
  if (!canManage) return <Navigate to="/settings/profile" replace />
  return (
    <>
      <PageHeader
        title={<DeveloperTitle />}
        description={<Trans>Keys, webhooks and connected assistants</Trans>}
        tabs={<DeveloperTabs current="webhooks" />}
        action={
          <Button
            className={headButton.primary}
            onClick={() => navigate("/setup/webhook", { state: { back: location.pathname } })}
            data-testid="webhook-add"
          >
            <Trans>Add endpoint</Trans>
          </Button>
        }
      />
      <WebhookTable />
    </>
  )
}

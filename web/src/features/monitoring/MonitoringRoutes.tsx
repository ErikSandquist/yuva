import "@/features/analytics/i18n"

import { Trans } from "@lingui/react/macro"
import { ActivityIcon, PlusIcon } from "lucide-react"
import { Link, Navigate, Route, Routes } from "react-router"

import { PaneEmpty } from "@/components/common/Column"
import { Button } from "@/components/ui/button"
import { EditCheckPage, NewCheckPage } from "@/features/monitoring/CheckForm"
import { CheckPage } from "@/features/monitoring/CheckPage"
import { useChecks } from "@/features/monitoring/queries"
import { useIsPhone } from "@/hooks/use-media-query"
import { useSession } from "@/lib/session"

function MonitoringHome() {
  const { canManage } = useSession()
  const phone = useIsPhone()
  const first = useChecks().data?.items[0]
  if (first && !phone) return <Navigate to={`/monitoring/checks/${first.id}`} replace />
  return (
    <PaneEmpty
      icon={ActivityIcon}
      title={<Trans>Know when your services stop answering</Trans>}
      testId="monitoring-home"
    >
      <p>
        <Trans>
          Uptime checks call an address from the server on an interval, time each answer and alert you when it goes down
          and when it is back.
        </Trans>
      </p>
      {canManage && (
        <Button className="mt-4" render={<Link to="/monitoring/checks/new" />} data-testid="monitoring-home-add">
          <PlusIcon />
          <Trans>Add an uptime check</Trans>
        </Button>
      )}
    </PaneEmpty>
  )
}

export default function MonitoringRoutes() {
  return (
    <Routes>
      <Route index element={<MonitoringHome />} />
      <Route path="checks" element={<Navigate to="/monitoring" replace />} />
      <Route path="checks/new" element={<NewCheckPage />} />
      <Route path="checks/:checkId" element={<CheckPage />} />
      <Route path="checks/:checkId/edit" element={<EditCheckPage />} />
      <Route path="*" element={<Navigate to="/monitoring" replace />} />
    </Routes>
  )
}

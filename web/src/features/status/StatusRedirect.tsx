import { Navigate, useLocation, useParams } from "react-router"

import { incidentLink, statusLink } from "@/features/status/queries"

export function StatusRedirect() {
  const { pageId = "", incidentId } = useParams()
  const { pathname, search } = useLocation()
  const tab = new URLSearchParams(search).get("tab")
  if (incidentId) return <Navigate to={incidentLink(pageId, incidentId)} replace />
  if (pathname.endsWith("/monitors") || tab === "monitors") return <Navigate to="/monitors" replace />
  if (tab === "incidents" || !tab) return <Navigate to={tab ? "/incidents" : statusLink(pageId)} replace />
  return <Navigate to={statusLink(pageId, tab)} replace />
}

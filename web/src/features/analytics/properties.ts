import { useQuery } from "@tanstack/react-query"

import { api, ApiError, unwrap } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

export function useAnalyticsProperties() {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.analyticsProperties(ws),
    queryFn: () => unwrap(api.GET("/v1/analytics/properties")),
    staleTime: 60_000,
  })
}

export function useAnalyticsVisible() {
  const { canManage } = useSession()
  const list = useAnalyticsProperties()
  if (list.error instanceof ApiError && list.error.status === 404) return false
  return canManage || (list.data?.items.length ?? 0) > 0
}

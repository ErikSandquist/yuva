import { useQuery } from "@tanstack/react-query"

import { api, unwrap } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

export function useHome(enabled = true) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.home(ws),
    queryFn: () => unwrap(api.GET("/v1/home")),
    enabled,
  })
}

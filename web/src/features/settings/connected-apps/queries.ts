import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { api, unwrap } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

export function useGrants() {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.oauthGrants(ws),
    queryFn: () => unwrap(api.GET("/v1/oauth/grants")).then((r) => r.items),
    refetchOnWindowFocus: "always",
  })
}

export function useWorkspaceSettings() {
  const { workspaceId: ws } = useSession()
  return useQuery({ queryKey: keys.workspace(ws), queryFn: () => unwrap(api.GET("/v1/workspace")) })
}

export function useRevokeGrant() {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.DELETE("/v1/oauth/grants/{oauthGrantId}", { params: { path: { oauthGrantId: id } } })),
    onSettled: () => qc.invalidateQueries({ queryKey: keys.oauthGrants(ws) }),
  })
}

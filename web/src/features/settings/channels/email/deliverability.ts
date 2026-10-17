import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { emailForm, emailInput } from "@/features/settings/channels/email/form"
import { api, ApiError, unwrap, type Channel, type DeliverabilityCheck } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

export function channelLink(
  inboxId: string,
  channelId: string,
  hash?: "general" | "receiving" | "replies" | "ses" | "broadcasts" | "deliverability" | "dkim" | "install"
) {
  return `/settings/inboxes/${inboxId}/channels/${channelId}${hash ? `#${hash}` : ""}`
}

export function isChecking(c: DeliverabilityCheck | null | undefined) {
  return c?.state === "queued" || c?.state === "sent"
}

export class RateLimited extends ApiError {
  retryAt: number

  constructor(problem: ConstructorParameters<typeof ApiError>[1], seconds: number) {
    super(429, problem)
    this.retryAt = Date.now() + seconds * 1000
  }
}

export function useDeliverability(channelId: string | undefined) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.deliverability(ws, channelId ?? ""),
    queryFn: async () => {
      try {
        return await unwrap(
          api.GET("/v1/channels/{channelId}/deliverability", { params: { path: { channelId: channelId! } } })
        )
      } catch (err) {
        if (err instanceof ApiError && err.status === 404 && err.code === "not_found") return null
        throw err
      }
    },
    enabled: !!channelId,
    retry: false,
    refetchInterval: (q) => (isChecking(q.state.data) ? 5000 : false),
  })
}

export function useRunCheck(channelId: string) {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return useMutation({
    mutationFn: async () => {
      const { data, error, response } = await api.POST("/v1/channels/{channelId}/deliverability-check", {
        params: { path: { channelId } },
      })
      if (response.status === 429) {
        throw new RateLimited(error ?? undefined, Number(response.headers.get("Retry-After")) || 600)
      }
      if (!response.ok) throw new ApiError(response.status, error ?? undefined)
      return data!
    },
    onSuccess: (c) => qc.setQueryData(keys.deliverability(ws, channelId), c),
  })
}

function useRefreshChannel(channel: Channel) {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return () => {
    void qc.invalidateQueries({ queryKey: keys.channel(ws, channel.id) })
    void qc.invalidateQueries({ queryKey: keys.channels(ws, channel.inbox_id) })
  }
}

export function useCreateDkim(channel: Channel) {
  const refresh = useRefreshChannel(channel)
  return useMutation({
    mutationFn: (selector: string) =>
      unwrap(
        api.POST("/v1/channels/{channelId}/dkim-key", {
          params: { path: { channelId: channel.id } },
          body: { selector },
        })
      ),
    onSuccess: refresh,
  })
}

export function useSetDkim(channel: Channel) {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  const refresh = useRefreshChannel(channel)
  return useMutation({
    mutationFn: (enabled: boolean) =>
      unwrap(
        api.PATCH("/v1/channels/{channelId}", {
          params: { path: { channelId: channel.id } },
          body: { email: { ...emailInput(emailForm(channel.email)), dkim: { enabled } } },
        })
      ),
    onSuccess: (saved) => {
      qc.setQueryData(keys.channel(ws, saved.id), saved)
      refresh()
    },
  })
}

export function useDeleteDkim(channel: Channel) {
  const refresh = useRefreshChannel(channel)
  return useMutation({
    mutationFn: () =>
      unwrap(api.DELETE("/v1/channels/{channelId}/dkim-key", { params: { path: { channelId: channel.id } } })),
    onSuccess: refresh,
  })
}

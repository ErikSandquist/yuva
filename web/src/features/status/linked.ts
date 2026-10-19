import { useInfiniteQuery, useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query"

import { isClosed } from "@/features/status/text"
import { useStatusPages } from "@/features/status/queries"
import { api, unwrap, type StatusIncident, type StatusPage } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

export type OpenIncident = { page: StatusPage; incident: StatusIncident }

export function useOpenIncidents(enabled: boolean) {
  const { workspaceId: ws } = useSession()
  const pages = useStatusPages("")
  const results = useQueries({
    queries: (pages.data ?? []).map((page) => ({
      queryKey: keys.statusIncidents(ws, page.id, true),
      queryFn: () =>
        unwrap(
          api.GET("/v1/status-pages/{pageId}/incidents", {
            params: { path: { pageId: page.id }, query: { open: true, limit: 50 } },
          })
        ),
      enabled,
    })),
  })
  const items: OpenIncident[] = (pages.data ?? []).flatMap((page, i) =>
    (results[i]?.data?.items ?? []).filter((incident) => !isClosed(incident)).map((incident) => ({ page, incident }))
  )
  return {
    items,
    pageCount: pages.data?.length ?? 0,
    pending: pages.isPending || results.some((r) => r.isPending && r.fetchStatus !== "idle"),
    error: pages.error ?? results.find((r) => r.error)?.error ?? null,
  }
}

export function useConversationIncidents(conversationId: string) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.conversationIncidents(ws, conversationId),
    queryFn: () =>
      unwrap(api.GET("/v1/conversations/{conversationId}/incidents", { params: { path: { conversationId } } })).then(
        (r) => r.items
      ),
    retry: false,
  })
}

export function useLinkMutations() {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  const refresh = () => qc.invalidateQueries({ queryKey: keys.statusLinked(ws) })
  return {
    link: useMutation({
      mutationFn: (v: { pageId: string; incidentId: string; conversationId: string }) =>
        unwrap(
          api.PUT("/v1/status-pages/{pageId}/incidents/{incidentId}/conversations/{conversationId}", {
            params: { path: v },
          })
        ),
      onSuccess: refresh,
    }),
    unlink: useMutation({
      mutationFn: (v: { pageId: string; incidentId: string; conversationId: string }) =>
        unwrap(
          api.DELETE("/v1/status-pages/{pageId}/incidents/{incidentId}/conversations/{conversationId}", {
            params: { path: v },
          })
        ),
      onSuccess: refresh,
    }),
  }
}

export function useLinkedConversations(pageId: string, incidentId: string) {
  const { workspaceId: ws } = useSession()
  return useInfiniteQuery({
    queryKey: keys.statusLinkedList(ws, pageId, incidentId),
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET("/v1/status-pages/{pageId}/incidents/{incidentId}/conversations", {
          params: { path: { pageId, incidentId }, query: { cursor: pageParam, limit: 100 } },
        })
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
  })
}

export function useReplyToLinked(pageId: string, incidentId: string) {
  return useMutation({
    mutationFn: (body: { body: string; close: boolean }) =>
      unwrap(
        api.POST("/v1/status-pages/{pageId}/incidents/{incidentId}/reply", {
          params: { path: { pageId, incidentId } },
          body,
        })
      ),
  })
}

export function useReplyCounts(pageId: string, incidentId: string, replyId: string) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.statusReply(ws, incidentId, replyId, "counts"),
    queryFn: () =>
      unwrap(
        api.GET("/v1/status-pages/{pageId}/incidents/{incidentId}/replies/{replyId}", {
          params: { path: { pageId, incidentId, replyId }, query: { limit: 1 } },
        })
      ),
    enabled: !!replyId,
    retry: false,
    refetchInterval: (q) => (q.state.data && q.state.data.counts.pending === 0 ? false : 2000),
  })
}

export function useReplyOutcomes(pageId: string, incidentId: string, replyId: string, enabled: boolean) {
  const { workspaceId: ws } = useSession()
  return useInfiniteQuery({
    queryKey: keys.statusReply(ws, incidentId, replyId, "items"),
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET("/v1/status-pages/{pageId}/incidents/{incidentId}/replies/{replyId}", {
          params: { path: { pageId, incidentId, replyId }, query: { cursor: pageParam, limit: 100 } },
        })
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
    enabled,
  })
}

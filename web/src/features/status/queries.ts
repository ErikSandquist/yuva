import { useInfiniteQuery, useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  api,
  unwrap,
  type StatusComponentStateValue,
  type StatusIncidentCreate,
  type StatusIncidentPatch,
  type StatusCheckRange,
  type StatusMonitorCreate,
  type StatusMonitorUpdate,
  type StatusNotice,
  type StatusPage,
  type StatusPageCreate,
  type StatusPageUpdate,
} from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

export function statusLink(pageId: string, tab?: string) {
  return `/status?page=${pageId}${tab ? `&tab=${tab}` : ""}`
}

export function incidentLink(pageId: string, incidentId: string, view?: string) {
  return `/incidents/${pageId}/${incidentId}${view && view !== "active" ? `?view=${view}` : ""}`
}

export function monitorLink(monitorId: string) {
  return `/monitors/${monitorId}`
}

function useRefresh() {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return () => qc.invalidateQueries({ queryKey: keys.status(ws) })
}

export function useStatusPages(inboxId: string) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.statusPages(ws, inboxId),
    queryFn: () =>
      unwrap(api.GET("/v1/status-pages", { params: { query: { inbox_id: inboxId || undefined } } })).then(
        (r) => r.items
      ),
  })
}

export function useStatusPage(id: string) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.statusPage(ws, id),
    queryFn: () => unwrap(api.GET("/v1/status-pages/{pageId}", { params: { path: { pageId: id } } })),
    enabled: !!id,
  })
}

export function useCreateStatusPage() {
  const refresh = useRefresh()
  return useMutation({
    mutationFn: (body: StatusPageCreate) => unwrap(api.POST("/v1/status-pages", { body })),
    onSuccess: refresh,
  })
}

export function usePatchStatusPage(id: string) {
  const refresh = useRefresh()
  return useMutation({
    mutationFn: (body: StatusPageUpdate) =>
      unwrap(api.PATCH("/v1/status-pages/{pageId}", { params: { path: { pageId: id } }, body })),
    onSuccess: refresh,
  })
}

export function useDeleteStatusPage(id: string) {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return useMutation({
    mutationFn: () => unwrap(api.DELETE("/v1/status-pages/{pageId}", { params: { path: { pageId: id } } })),
    onSuccess: () => {
      qc.removeQueries({ predicate: (q) => q.queryKey.includes(id) })
      return qc.invalidateQueries({ queryKey: [...keys.status(ws), "pages"] })
    },
  })
}

export function useStatusPreview(id: string, enabled: boolean) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: [...keys.statusPage(ws, id), "preview"],
    queryFn: async () => {
      const { data, response } = await api.GET("/v1/status-pages/{pageId}/preview", {
        params: { path: { pageId: id } },
        parseAs: "text",
      })
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
      return data as string
    },
    enabled,
    gcTime: 0,
  })
}

export function usePublicStatus(page: StatusPage | undefined) {
  const { workspaceId: ws } = useSession()
  const slug = page?.slug ?? ""
  return useQuery({
    queryKey: [...keys.statusPage(ws, page?.id ?? ""), "public", slug],
    queryFn: () => unwrap(api.GET("/client/v1/status/{slug}", { params: { path: { slug } }, cache: "no-cache" })),
    enabled: !!page?.published && page.show_uptime,
    retry: false,
  })
}

export function useStatusGroups(pageId: string) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.statusGroups(ws, pageId),
    queryFn: () =>
      unwrap(api.GET("/v1/status-pages/{pageId}/groups", { params: { path: { pageId } } })).then((r) => r.items),
  })
}

export function useStatusComponents(pageId: string) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    enabled: !!pageId,
    queryKey: keys.statusComponents(ws, pageId),
    queryFn: () =>
      unwrap(api.GET("/v1/status-pages/{pageId}/components", { params: { path: { pageId } } })).then((r) => r.items),
  })
}

export function useGroupMutations(pageId: string) {
  const refresh = useRefresh()
  const path = { pageId }
  return {
    create: useMutation({
      mutationFn: (body: { name: string; position?: number }) =>
        unwrap(api.POST("/v1/status-pages/{pageId}/groups", { params: { path }, body })),
      onSuccess: refresh,
    }),
    update: useMutation({
      mutationFn: (v: { id: string; name?: string; position?: number }) =>
        unwrap(
          api.PATCH("/v1/status-pages/{pageId}/groups/{groupId}", {
            params: { path: { ...path, groupId: v.id } },
            body: { name: v.name, position: v.position },
          })
        ),
      onSuccess: refresh,
    }),
    remove: useMutation({
      mutationFn: (id: string) =>
        unwrap(
          api.DELETE("/v1/status-pages/{pageId}/groups/{groupId}", { params: { path: { ...path, groupId: id } } })
        ),
      onSuccess: refresh,
    }),
  }
}

export type ComponentInput = {
  name?: string
  description?: string
  group_id?: string | null
  position?: number
  hidden?: boolean
}

export function useComponentMutations(pageId: string) {
  const refresh = useRefresh()
  const path = { pageId }
  return {
    create: useMutation({
      mutationFn: (body: {
        name: string
        description?: string
        group_id?: string
        hidden?: boolean
        state?: StatusComponentStateValue
      }) => unwrap(api.POST("/v1/status-pages/{pageId}/components", { params: { path }, body })),
      onSuccess: refresh,
    }),
    update: useMutation({
      mutationFn: ({ id, ...body }: ComponentInput & { id: string }) =>
        unwrap(
          api.PATCH("/v1/status-pages/{pageId}/components/{componentId}", {
            params: { path: { ...path, componentId: id } },
            body,
          })
        ),
      onSuccess: refresh,
    }),
    remove: useMutation({
      mutationFn: (id: string) =>
        unwrap(
          api.DELETE("/v1/status-pages/{pageId}/components/{componentId}", {
            params: { path: { ...path, componentId: id } },
          })
        ),
      onSuccess: refresh,
    }),
    setState: useMutation({
      mutationFn: (v: { id: string; state: StatusComponentStateValue }) =>
        unwrap(
          api.POST("/v1/status-pages/{pageId}/components/{componentId}/state", {
            params: { path: { ...path, componentId: v.id } },
            body: { state: v.state },
          })
        ),
      onSuccess: refresh,
    }),
  }
}

export function useStatusIncidents(pageId: string, open: boolean) {
  const { workspaceId: ws } = useSession()
  return useInfiniteQuery({
    queryKey: keys.statusIncidents(ws, pageId, open),
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET("/v1/status-pages/{pageId}/incidents", {
          params: { path: { pageId }, query: { open: open || undefined, cursor: pageParam, limit: 50 } },
        })
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
  })
}

export function usePagesIncidents(pageIds: string[], open: boolean) {
  const { workspaceId: ws } = useSession()
  return useQueries({
    queries: pageIds.map((pageId) => ({
      queryKey: [...keys.status(ws), "page-incidents", pageId, open],
      queryFn: () =>
        unwrap(
          api.GET("/v1/status-pages/{pageId}/incidents", {
            params: { path: { pageId }, query: { open: open || undefined, limit: 50 } },
          })
        ).then((r) => r.items),
    })),
    combine: (results) => ({
      data: results.flatMap((r) => r.data ?? []),
      isPending: results.some((r) => r.isPending),
      error: results.find((r) => r.error)?.error ?? null,
    }),
  })
}

export function usePagesMonitors(pageIds: string[]) {
  const { workspaceId: ws } = useSession()
  return useQueries({
    queries: pageIds.map((pageId) => ({
      queryKey: keys.statusMonitors(ws, pageId),
      queryFn: () =>
        unwrap(api.GET("/v1/status-pages/{pageId}/monitors", { params: { path: { pageId } } })).then((r) => r.items),
    })),
    combine: (results) => ({
      data: results.flatMap((r) => r.data ?? []),
      isPending: results.some((r) => r.isPending),
      error: results.find((r) => r.error)?.error ?? null,
    }),
  })
}

export function usePagesComponents(pageIds: string[]) {
  const { workspaceId: ws } = useSession()
  return useQueries({
    queries: pageIds.map((pageId) => ({
      queryKey: keys.statusComponents(ws, pageId),
      queryFn: () =>
        unwrap(api.GET("/v1/status-pages/{pageId}/components", { params: { path: { pageId } } })).then((r) => r.items),
    })),
    combine: (results) => results.flatMap((r) => r.data ?? []),
  })
}

export function useStatusIncident(pageId: string, id: string) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.statusIncident(ws, pageId, id),
    queryFn: () =>
      unwrap(
        api.GET("/v1/status-pages/{pageId}/incidents/{incidentId}", {
          params: { path: { pageId, incidentId: id } },
        })
      ),
    enabled: !!id,
  })
}

export function useIncidentMutations(pageId: string) {
  const refresh = useRefresh()
  const path = { pageId }
  return {
    create: useMutation({
      mutationFn: (body: StatusIncidentCreate) =>
        unwrap(api.POST("/v1/status-pages/{pageId}/incidents", { params: { path }, body })),
      onSuccess: refresh,
    }),
    patch: useMutation({
      mutationFn: ({ id, ...body }: StatusIncidentPatch & { id: string }) =>
        unwrap(
          api.PATCH("/v1/status-pages/{pageId}/incidents/{incidentId}", {
            params: { path: { ...path, incidentId: id } },
            body,
          })
        ),
      onSuccess: refresh,
    }),
    remove: useMutation({
      mutationFn: (id: string) =>
        unwrap(
          api.DELETE("/v1/status-pages/{pageId}/incidents/{incidentId}", {
            params: { path: { ...path, incidentId: id } },
          })
        ),
      onSuccess: refresh,
    }),
    publish: useMutation({
      mutationFn: (v: { id: string; notify?: boolean }) =>
        unwrap(
          api.POST("/v1/status-pages/{pageId}/incidents/{incidentId}/publish", {
            params: { path: { ...path, incidentId: v.id } },
            body: v.notify === undefined ? undefined : { notify: v.notify },
          })
        ),
      onSuccess: refresh,
    }),
    postUpdate: useMutation({
      mutationFn: (v: { id: string; body: string; state?: StatusIncidentCreate["state"]; notify?: boolean }) =>
        unwrap(
          api.POST("/v1/status-pages/{pageId}/incidents/{incidentId}/updates", {
            params: { path: { ...path, incidentId: v.id } },
            body: { body: v.body, state: v.state, notify: v.notify },
          })
        ),
      onSuccess: refresh,
    }),
    editUpdate: useMutation({
      mutationFn: (v: { id: string; updateId: string; body: string }) =>
        unwrap(
          api.PATCH("/v1/status-pages/{pageId}/incidents/{incidentId}/updates/{updateId}", {
            params: { path: { ...path, incidentId: v.id, updateId: v.updateId } },
            body: { body: v.body },
          })
        ),
      onSuccess: refresh,
    }),
  }
}

export function useStatusMonitors(pageId: string) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.statusMonitors(ws, pageId),
    queryFn: () =>
      unwrap(api.GET("/v1/status-pages/{pageId}/monitors", { params: { path: { pageId } } })).then((r) => r.items),
  })
}

export function useMonitorChecks(pageId: string, monitorId: string) {
  const { workspaceId: ws } = useSession()
  return useInfiniteQuery({
    queryKey: keys.statusChecks(ws, pageId, monitorId),
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET("/v1/status-pages/{pageId}/monitors/{monitorId}/checks", {
          params: { path: { pageId, monitorId }, query: { cursor: pageParam, limit: 25 } },
        })
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
  })
}

export function useLastCheck(pageId: string, monitorId: string, enabled: boolean) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: [...keys.statusChecks(ws, pageId, monitorId), "last"],
    queryFn: () =>
      unwrap(
        api.GET("/v1/status-pages/{pageId}/monitors/{monitorId}/checks", {
          params: { path: { pageId, monitorId }, query: { limit: 1 } },
        })
      ).then((r) => r.items[0]),
    enabled,
  })
}

export function useMonitorSummary(pageId: string, monitorId: string, range: StatusCheckRange, enabled = true) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    enabled: enabled && !!monitorId,
    queryKey: [...keys.statusChecks(ws, pageId, monitorId), "summary", range],
    queryFn: () =>
      unwrap(
        api.GET("/v1/status-pages/{pageId}/monitors/{monitorId}/checks/summary", {
          params: { path: { pageId, monitorId }, query: { range } },
        })
      ),
  })
}

export function useMonitorMutations(pageId: string) {
  const refresh = useRefresh()
  const path = { pageId }
  return {
    create: useMutation({
      mutationFn: (body: StatusMonitorCreate) =>
        unwrap(api.POST("/v1/status-pages/{pageId}/monitors", { params: { path }, body })),
      onSuccess: refresh,
    }),
    update: useMutation({
      mutationFn: ({ id, ...body }: StatusMonitorUpdate & { id: string }) =>
        unwrap(
          api.PATCH("/v1/status-pages/{pageId}/monitors/{monitorId}", {
            params: { path: { ...path, monitorId: id } },
            body,
          })
        ),
      onSuccess: refresh,
    }),
    remove: useMutation({
      mutationFn: (id: string) =>
        unwrap(
          api.DELETE("/v1/status-pages/{pageId}/monitors/{monitorId}", {
            params: { path: { ...path, monitorId: id } },
          })
        ),
      onSuccess: refresh,
    }),
    rotate: useMutation({
      mutationFn: (id: string) =>
        unwrap(
          api.POST("/v1/status-pages/{pageId}/monitors/{monitorId}/token", {
            params: { path: { ...path, monitorId: id } },
          })
        ),
      onSuccess: refresh,
    }),
  }
}

export function useNoticeReach(pageId: string, componentIds: string[], enabled: boolean) {
  const { workspaceId: ws } = useSession()
  const ids = [...componentIds].sort()
  return useQuery({
    queryKey: keys.statusReach(ws, pageId, ids),
    queryFn: () =>
      unwrap(
        api.GET("/v1/status-pages/{pageId}/notice-reach", {
          params: { path: { pageId }, query: { component_id: ids.length ? ids : undefined } },
        })
      ).then((r) => r.subscribers),
    enabled,
  })
}

export function useStatusNotices(pageId: string, incidentId: string, enabled: boolean) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.statusNotices(ws, pageId, incidentId),
    queryFn: () =>
      unwrap(
        api.GET("/v1/status-pages/{pageId}/notices", {
          params: { path: { pageId }, query: { incident_id: incidentId, limit: 100 } },
        })
      ).then((r) => r.items),
    enabled,
  })
}

export function useCancelNotice(pageId: string) {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(
        api.POST("/v1/status-pages/{pageId}/notices/{noticeId}/cancel", {
          params: { path: { pageId, noticeId: id } },
        })
      ),
    onSuccess: (n: StatusNotice) => qc.invalidateQueries({ queryKey: keys.statusNotices(ws, pageId, n.incident_id) }),
  })
}

import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  api,
  unwrap,
  type StatusComponentStateValue,
  type StatusIncidentCreate,
  type StatusIncidentPatch,
  type StatusPageCreate,
  type StatusPageUpdate,
} from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

export function statusLink(pageId: string, tab?: string) {
  return `/status-pages/${pageId}${tab ? `?tab=${tab}` : ""}`
}

export function incidentLink(pageId: string, incidentId: string) {
  return `/status-pages/${pageId}/incidents/${incidentId}`
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
      mutationFn: (id: string) =>
        unwrap(
          api.POST("/v1/status-pages/{pageId}/incidents/{incidentId}/publish", {
            params: { path: { ...path, incidentId: id } },
          })
        ),
      onSuccess: refresh,
    }),
    postUpdate: useMutation({
      mutationFn: (v: { id: string; body: string; state?: StatusIncidentCreate["state"] }) =>
        unwrap(
          api.POST("/v1/status-pages/{pageId}/incidents/{incidentId}/updates", {
            params: { path: { ...path, incidentId: v.id } },
            body: { body: v.body, state: v.state },
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

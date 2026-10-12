import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useAnalyticsProperties } from "@/features/analytics/properties"
import {
  api,
  unwrap,
  type AnalyticsProperty,
  type AnalyticsPropertyUpdate,
  type AnalyticsQuery,
  type AnalyticsResult,
} from "@/lib/api"
import { saveDownload } from "@/lib/download"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

export function useProperty(id: string) {
  const { workspaceId: ws } = useSession()
  const list = useAnalyticsProperties()
  return useQuery({
    queryKey: keys.analyticsProperty(ws, id),
    queryFn: () => unwrap(api.GET("/v1/analytics/properties/{propertyId}", { params: { path: { propertyId: id } } })),
    initialData: () => list.data?.items.find((p) => p.id === id),
    initialDataUpdatedAt: () => list.dataUpdatedAt,
    staleTime: 30_000,
    enabled: !!id,
  })
}

export function useWaitForFirstEvent(id: string, waiting: boolean) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: [...keys.analyticsProperty(ws, id), "wait"],
    queryFn: () => unwrap(api.GET("/v1/analytics/properties/{propertyId}", { params: { path: { propertyId: id } } })),
    refetchInterval: (q) => (q.state.data?.first_event_at ? false : 3_000),
    enabled: !!id && waiting,
  })
}

export type Query = Omit<AnalyticsQuery, "limit"> & { limit?: number }

export function useAnalytics(id: string, query: Query, enabled = true) {
  const body: AnalyticsQuery = { limit: 100, ...query }
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: [...keys.analyticsData(ws, id), "query", body],
    queryFn: () =>
      unwrap(
        api.POST("/v1/analytics/properties/{propertyId}/query", { params: { path: { propertyId: id } }, body })
      ) as Promise<AnalyticsResult>,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
    enabled: !!id && enabled,
  })
}

export function useAnalyticsTable(id: string, body: AnalyticsQuery) {
  const { workspaceId: ws } = useSession()
  return useInfiniteQuery({
    queryKey: [...keys.analyticsData(ws, id), "table", body],
    queryFn: ({ pageParam }) =>
      unwrap(
        api.POST("/v1/analytics/properties/{propertyId}/query", {
          params: { path: { propertyId: id } },
          body: { ...body, cursor: pageParam },
        })
      ) as Promise<AnalyticsResult>,
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  })
}

export function downloadCsv(id: string, body: Query, name: string) {
  return saveDownload(
    api.POST("/v1/analytics/properties/{propertyId}/query", {
      params: { path: { propertyId: id }, query: { format: "csv" } },
      body: { ...body, limit: 1000, cursor: undefined },
      parseAs: "blob",
    }),
    `${name}.csv`
  )
}

export function useRealtime(id: string) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: [...keys.analyticsData(ws, id), "realtime"],
    queryFn: () =>
      unwrap(api.GET("/v1/analytics/properties/{propertyId}/realtime", { params: { path: { propertyId: id } } })),
    refetchInterval: 15_000,
    placeholderData: keepPreviousData,
  })
}

export function useHealth(id: string, days: number) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: [...keys.analyticsData(ws, id), "health", days],
    queryFn: () =>
      unwrap(
        api.GET("/v1/analytics/properties/{propertyId}/health", {
          params: { path: { propertyId: id }, query: { days } },
        })
      ),
    staleTime: days === 1 ? 10 * 60_000 : 30_000,
  })
}

export function useGeoAttribution(id: string) {
  const geo = useHealth(id, 1).data?.geoip
  return geo?.attribution ? { text: geo.attribution, url: geo.attribution_url } : null
}

function useStore() {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return {
    put: (p: AnalyticsProperty) => {
      qc.setQueryData(keys.analyticsProperty(ws, p.id), p)
      void qc.invalidateQueries({ queryKey: keys.analyticsProperties(ws) })
    },
    refresh: (id: string) => {
      void qc.invalidateQueries({ queryKey: keys.analyticsData(ws, id) })
      void qc.invalidateQueries({ queryKey: keys.analyticsProperties(ws) })
    },
    drop: (id: string) => {
      qc.removeQueries({ queryKey: keys.analyticsProperty(ws, id) })
      qc.removeQueries({ queryKey: keys.analyticsData(ws, id) })
      void qc.invalidateQueries({ queryKey: keys.analyticsProperties(ws) })
    },
  }
}

export function useCreateProperty() {
  const store = useStore()
  return useMutation({
    mutationFn: (body: { name: string; domains: string[]; inbox_id?: string; timezone: string }) =>
      unwrap(api.POST("/v1/analytics/properties", { body: { currency: "EUR", ...body } })),
    onSuccess: store.put,
  })
}

export function useUpdateProperty(id: string) {
  const store = useStore()
  return useMutation({
    mutationFn: (body: AnalyticsPropertyUpdate) =>
      unwrap(api.PATCH("/v1/analytics/properties/{propertyId}", { params: { path: { propertyId: id } }, body })),
    onSuccess: store.put,
  })
}

export function useRotateKey(id: string) {
  const store = useStore()
  return useMutation({
    mutationFn: () =>
      unwrap(api.POST("/v1/analytics/properties/{propertyId}/rotate-key", { params: { path: { propertyId: id } } })),
    onSuccess: store.put,
  })
}

export function useDeleteData(id: string) {
  const store = useStore()
  return useMutation({
    mutationFn: (body: { from: string; to: string } | { all: true }) =>
      unwrap(
        api.POST("/v1/analytics/properties/{propertyId}/delete-data", { params: { path: { propertyId: id } }, body })
      ),
    onSuccess: () => store.refresh(id),
  })
}

export function useDeleteProperty(id: string) {
  const store = useStore()
  return useMutation({
    mutationFn: (name: string) =>
      unwrap(
        api.DELETE("/v1/analytics/properties/{propertyId}", { params: { path: { propertyId: id } }, body: { name } })
      ),
    onSuccess: () => store.drop(id),
  })
}

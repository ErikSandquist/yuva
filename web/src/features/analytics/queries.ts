import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useAnalyticsProperties } from "@/features/analytics/properties"
import {
  api,
  unwrap,
  type AnalyticsFilter,
  type AnalyticsGoal,
  type AnalyticsGoalCreate,
  type AnalyticsProperty,
  type AnalyticsPropertyUpdate,
  type AnalyticsQuery,
  type AnalyticsResult,
} from "@/lib/api"
import { saveDownload } from "@/lib/download"
import type { paths } from "@/lib/schema.gen"
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

function useMeta(id: string) {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  const base = keys.analyticsData(ws, id)
  return {
    goals: [...base, "goals"] as const,
    funnels: [...base, "funnels"] as const,
    segments: [...base, "segments"] as const,
    annotations: [...base, "annotations"] as const,
    base,
    invalidate: (key: readonly unknown[]) => void qc.invalidateQueries({ queryKey: key }),
  }
}

const path = (id: string) => ({ params: { path: { propertyId: id } } })

export function useGoals(id: string) {
  const m = useMeta(id)
  return useQuery({
    queryKey: m.goals,
    queryFn: () => unwrap(api.GET("/v1/analytics/properties/{propertyId}/goals", path(id))),
    select: (d) => d.items,
    staleTime: 60_000,
  })
}

export function useSaveGoal(id: string, goalId?: string) {
  const m = useMeta(id)
  return useMutation({
    mutationFn: (body: AnalyticsGoalCreate) =>
      goalId
        ? unwrap(
            api.PATCH("/v1/analytics/properties/{propertyId}/goals/{goalId}", {
              params: { path: { propertyId: id, goalId } },
              body: { ...body, prop_key: body.kind === "event" ? (body.prop_key ?? null) : undefined },
            })
          )
        : unwrap(api.POST("/v1/analytics/properties/{propertyId}/goals", { ...path(id), body })),
    onSuccess: () => m.invalidate(m.base),
  })
}

export function useDeleteGoal(id: string) {
  const m = useMeta(id)
  return useMutation({
    mutationFn: (goalId: string) =>
      unwrap(
        api.DELETE("/v1/analytics/properties/{propertyId}/goals/{goalId}", {
          params: { path: { propertyId: id, goalId } },
        })
      ),
    onSuccess: () => m.invalidate(m.base),
  })
}

export function useGoalTotals(id: string, goals: AnalyticsGoal[], query: Omit<Query, "metrics" | "goal_id">) {
  const m = useMeta(id)
  const ids = goals.map((g) => g.id)
  return useQuery({
    queryKey: [...m.base, "goal-totals", ids, query],
    queryFn: async () => {
      const out = new Map<string, AnalyticsResult["totals"]>()
      const queue = [...ids]
      const worker = async () => {
        for (let goal = queue.shift(); goal; goal = queue.shift()) {
          const r = (await unwrap(
            api.POST("/v1/analytics/properties/{propertyId}/query", {
              ...path(id),
              body: { limit: 1, ...query, goal_id: goal, metrics: ["conversions", "conversion_rate", "revenue"] },
            })
          )) as AnalyticsResult
          out.set(goal, r.totals)
        }
      }
      await Promise.all([worker(), worker()])
      return out
    },
    placeholderData: keepPreviousData,
    staleTime: 30_000,
    enabled: ids.length > 0,
  })
}

export function useFunnels(id: string) {
  const m = useMeta(id)
  return useQuery({
    queryKey: m.funnels,
    queryFn: () => unwrap(api.GET("/v1/analytics/properties/{propertyId}/funnels", path(id))),
    select: (d) => d.items,
    staleTime: 60_000,
  })
}

export function useSaveFunnel(id: string, funnelId?: string) {
  const m = useMeta(id)
  return useMutation({
    mutationFn: (body: { name: string; steps: string[] }) =>
      funnelId
        ? unwrap(
            api.PATCH("/v1/analytics/properties/{propertyId}/funnels/{funnelId}", {
              params: { path: { propertyId: id, funnelId } },
              body,
            })
          )
        : unwrap(api.POST("/v1/analytics/properties/{propertyId}/funnels", { ...path(id), body })),
    onSuccess: () => m.invalidate(m.base),
  })
}

export function useDeleteFunnel(id: string) {
  const m = useMeta(id)
  return useMutation({
    mutationFn: (funnelId: string) =>
      unwrap(
        api.DELETE("/v1/analytics/properties/{propertyId}/funnels/{funnelId}", {
          params: { path: { propertyId: id, funnelId } },
        })
      ),
    onSuccess: () => m.invalidate(m.funnels),
  })
}

export type FunnelRange = { period: NonNullable<FunnelPeriod> } | { from: string; to: string }
type FunnelPeriod = NonNullable<
  paths["/v1/analytics/properties/{propertyId}/funnels/{funnelId}/report"]["get"]["parameters"]["query"]
>["period"]

export function useFunnelReport(id: string, funnelId: string | undefined, range: FunnelRange | null) {
  const m = useMeta(id)
  return useQuery({
    queryKey: [...m.base, "funnel", funnelId, range],
    queryFn: () =>
      unwrap(
        api.GET("/v1/analytics/properties/{propertyId}/funnels/{funnelId}/report", {
          params: { path: { propertyId: id, funnelId: funnelId! }, query: range ?? {} },
        })
      ),
    placeholderData: keepPreviousData,
    staleTime: 60_000,
    enabled: !!funnelId && !!range,
  })
}

export function useSegments(id: string) {
  const m = useMeta(id)
  return useQuery({
    queryKey: m.segments,
    queryFn: () => unwrap(api.GET("/v1/analytics/properties/{propertyId}/segments", path(id))),
    select: (d) => d.items,
    staleTime: 60_000,
  })
}

export function useSaveSegment(id: string) {
  const m = useMeta(id)
  return useMutation({
    mutationFn: ({ segmentId, ...body }: { segmentId?: string; name?: string; filters?: AnalyticsFilter[] }) =>
      segmentId
        ? unwrap(
            api.PATCH("/v1/analytics/properties/{propertyId}/segments/{segmentId}", {
              params: { path: { propertyId: id, segmentId } },
              body,
            })
          )
        : unwrap(
            api.POST("/v1/analytics/properties/{propertyId}/segments", {
              ...path(id),
              body: { name: body.name ?? "", filters: body.filters ?? [] },
            })
          ),
    onSuccess: () => m.invalidate(m.base),
  })
}

export function useDeleteSegment(id: string) {
  const m = useMeta(id)
  return useMutation({
    mutationFn: (segmentId: string) =>
      unwrap(
        api.DELETE("/v1/analytics/properties/{propertyId}/segments/{segmentId}", {
          params: { path: { propertyId: id, segmentId } },
        })
      ),
    onSuccess: () => m.invalidate(m.segments),
  })
}

export function useAnnotations(id: string, from?: string, to?: string) {
  const m = useMeta(id)
  return useQuery({
    queryKey: [...m.annotations, from, to],
    queryFn: () =>
      unwrap(
        api.GET("/v1/analytics/properties/{propertyId}/annotations", {
          params: { path: { propertyId: id }, query: { from, to } },
        })
      ),
    select: (d) => d.items,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
    enabled: !!from && !!to,
  })
}

export function useSaveAnnotation(id: string, annotationId?: string) {
  const m = useMeta(id)
  return useMutation({
    mutationFn: (body: { starts_on: string; ends_on: string; text: string }) =>
      annotationId
        ? unwrap(
            api.PATCH("/v1/analytics/properties/{propertyId}/annotations/{annotationId}", {
              params: { path: { propertyId: id, annotationId } },
              body,
            })
          )
        : unwrap(api.POST("/v1/analytics/properties/{propertyId}/annotations", { ...path(id), body })),
    onSuccess: () => m.invalidate(m.annotations),
  })
}

export function useDeleteAnnotation(id: string) {
  const m = useMeta(id)
  return useMutation({
    mutationFn: (annotationId: string) =>
      unwrap(
        api.DELETE("/v1/analytics/properties/{propertyId}/annotations/{annotationId}", {
          params: { path: { propertyId: id, annotationId } },
        })
      ),
    onSuccess: () => m.invalidate(m.annotations),
  })
}

import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  api,
  ApiError,
  unwrap,
  type AnalyticsQuery,
  type AnalyticsResult,
  type MonitorAlertRuleCreate,
  type MonitorAlertRuleFields,
  type MonitorCheckCreate,
  type MonitorCheckFields,
} from "@/lib/api"
import { keys } from "@/lib/keys"
import { useOptionalSession, useSession } from "@/lib/session"

function useBase() {
  const ws = useOptionalSession()?.workspaceId ?? ""
  return { ws, base: keys.monitoring(ws) }
}

export function useChecks(propertyId?: string) {
  const { ws, base } = useBase()
  return useQuery({
    queryKey: [...base, "checks", propertyId ?? ""],
    queryFn: () =>
      unwrap(api.GET("/v1/monitoring/checks", { params: { query: propertyId ? { property_id: propertyId } : {} } })),
    staleTime: 30_000,
    enabled: !!ws,
  })
}

export function useMonitoringVisible() {
  const { canManage } = useSession()
  const list = useChecks()
  if (list.error instanceof ApiError && list.error.status === 404) return false
  return canManage || (list.data?.items.length ?? 0) > 0
}

export function useCheck(id: string) {
  const { base } = useBase()
  return useQuery({
    queryKey: [...base, "check", id],
    queryFn: () => unwrap(api.GET("/v1/monitoring/checks/{checkId}", { params: { path: { checkId: id } } })),
    staleTime: 15_000,
    enabled: !!id,
  })
}

export function useCheckResults(id: string) {
  const { base } = useBase()
  return useInfiniteQuery({
    queryKey: [...base, "results", id],
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET("/v1/monitoring/checks/{checkId}/results", {
          params: { path: { checkId: id }, query: { before: pageParam, limit: 100 } },
        })
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => (last.items.length >= 100 ? last.items[last.items.length - 1]?.at : undefined),
    staleTime: 15_000,
  })
}

export function useUptime(query: Omit<AnalyticsQuery, "dataset" | "limit"> & { limit?: number }, enabled = true) {
  const { base } = useBase()
  const body: AnalyticsQuery = { ...query, limit: query.limit ?? 100, dataset: "uptime" }
  return useQuery({
    queryKey: [...base, "uptime", body],
    queryFn: () =>
      unwrap(api.POST("/v1/monitoring/query", { body: { insights: false, ...body } })) as Promise<AnalyticsResult>,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
    enabled,
  })
}

function useInvalidate() {
  const qc = useQueryClient()
  const { base } = useBase()
  return () => void qc.invalidateQueries({ queryKey: base })
}

export function useSaveCheck(id?: string) {
  const done = useInvalidate()
  return useMutation({
    mutationFn: (body: MonitorCheckFields) =>
      id
        ? unwrap(
            api.PATCH("/v1/monitoring/checks/{checkId}", { params: { path: { checkId: id } }, body: body as never })
          )
        : unwrap(api.POST("/v1/monitoring/checks", { body: body as MonitorCheckCreate })),
    onSuccess: done,
  })
}

export function useDeleteCheck(id: string) {
  const done = useInvalidate()
  return useMutation({
    mutationFn: () => unwrap(api.DELETE("/v1/monitoring/checks/{checkId}", { params: { path: { checkId: id } } })),
    onSuccess: done,
  })
}

export type RuleScope = { property_id?: string; check_id?: string }

export function useAlertRules(scope: RuleScope) {
  const { ws, base } = useBase()
  return useQuery({
    queryKey: [...base, "rules", scope],
    queryFn: () => unwrap(api.GET("/v1/monitoring/alert-rules", { params: { query: scope } })),
    select: (d) => d.items,
    staleTime: 30_000,
    enabled: !!ws,
  })
}

export function useSaveRule(id?: string) {
  const done = useInvalidate()
  return useMutation({
    mutationFn: (body: MonitorAlertRuleFields & Partial<MonitorAlertRuleCreate>) =>
      id
        ? unwrap(
            api.PATCH("/v1/monitoring/alert-rules/{ruleId}", {
              params: { path: { ruleId: id } },
              body: body as never,
            })
          )
        : unwrap(api.POST("/v1/monitoring/alert-rules", { body: body as MonitorAlertRuleCreate })),
    onSuccess: done,
  })
}

export function useDeleteRule() {
  const done = useInvalidate()
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.DELETE("/v1/monitoring/alert-rules/{ruleId}", { params: { path: { ruleId: id } } })),
    onSuccess: done,
  })
}

export function useMonitorAlerts(scope: RuleScope & { state?: "firing" | "resolved" }) {
  const { ws, base } = useBase()
  return useInfiniteQuery({
    queryKey: [...base, "alerts", scope],
    queryFn: ({ pageParam }) =>
      unwrap(api.GET("/v1/monitoring/alerts", { params: { query: { ...scope, before: pageParam, limit: 50 } } })),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => (last.items.length >= 50 ? last.items[last.items.length - 1]?.fired_at : undefined),
    staleTime: 30_000,
    enabled: !!ws,
  })
}

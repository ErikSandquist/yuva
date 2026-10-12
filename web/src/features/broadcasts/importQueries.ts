import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useEffect } from "react"

import {
  api,
  unwrap,
  type ListImport,
  type ListImportColumnMapping,
  type ListImportCounts,
  type ListImportState,
} from "@/lib/api"
import { saveDownload } from "@/lib/download"
import { keys } from "@/lib/keys"
import { useRealtimeStatus } from "@/lib/realtime"
import { useSession } from "@/lib/session"

export const IMPORT_MAX_BYTES = 20 * 1024 * 1024

export const REFUSALS = [
  "invalid_email",
  "no_mail_domain",
  "role_address",
  "duplicate",
  "suppressed",
  "opted_out",
  "unsubscribed",
] as const satisfies (keyof ListImportCounts)[]

export function isRunning(state: ListImportState) {
  return state === "validating" || state === "importing"
}

export function isOpen(state: ListImportState) {
  return isRunning(state) || state === "ready"
}

export function refused(c: ListImportCounts) {
  return REFUSALS.reduce((n, k) => n + c[k], 0)
}

export function useListImports(listId: string) {
  const { workspaceId: ws } = useSession()
  return useInfiniteQuery({
    queryKey: keys.listImports(ws, listId),
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET("/v1/lists/{listId}/imports", {
          params: { path: { listId }, query: { cursor: pageParam, limit: 20 } },
        })
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
  })
}

export function useListImport(listId: string, id: string | null) {
  const { workspaceId: ws } = useSession()
  const qc = useQueryClient()
  const live = useRealtimeStatus() === "live"
  const query = useQuery({
    queryKey: keys.listImport(ws, listId, id ?? ""),
    queryFn: () =>
      unwrap(api.GET("/v1/lists/{listId}/imports/{importId}", { params: { path: { listId, importId: id! } } })),
    enabled: !!id,
    refetchInterval: (q) => (!live && q.state.data && isRunning(q.state.data.state) ? 3000 : false),
  })
  useEffect(() => {
    if (live && id) void qc.invalidateQueries({ queryKey: keys.listImport(ws, listId, id) })
  }, [live, id, listId, ws, qc])
  return query
}

function usePutImport(listId: string) {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return (i: ListImport) => {
    qc.setQueryData(keys.listImport(ws, listId, i.id), i)
    void qc.invalidateQueries({ queryKey: keys.listImports(ws, listId) })
  }
}

export type UploadInput = {
  file: File
  attestation: string
  consentSource: string
  reconfirm: boolean
}

export function useUploadImport(listId: string) {
  const put = usePutImport(listId)
  return useMutation({
    mutationFn: (v: UploadInput) =>
      unwrap(
        api.POST("/v1/lists/{listId}/imports", {
          params: { path: { listId } },
          body: {
            file: v.file as unknown as string,
            attestation: v.attestation,
            consent_source: v.consentSource,
            reconfirm: v.reconfirm,
          },
          bodySerializer: () => {
            const form = new FormData()
            form.append("file", v.file)
            form.append("attestation", v.attestation)
            form.append("consent_source", v.consentSource)
            form.append("reconfirm", String(v.reconfirm))
            return form
          },
        })
      ),
    onSuccess: put,
  })
}

export function useRemapImport(listId: string, id: string) {
  const put = usePutImport(listId)
  return useMutation({
    mutationFn: (columns: ListImportColumnMapping[]) =>
      unwrap(
        api.PATCH("/v1/lists/{listId}/imports/{importId}", {
          params: { path: { listId, importId: id } },
          body: { columns },
        })
      ),
    onSuccess: put,
  })
}

export function useStartImport(listId: string, id: string) {
  const put = usePutImport(listId)
  return useMutation({
    mutationFn: () =>
      unwrap(api.POST("/v1/lists/{listId}/imports/{importId}/start", { params: { path: { listId, importId: id } } })),
    onSuccess: put,
  })
}

export function useCancelImport(listId: string, id: string) {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return useMutation({
    mutationFn: () =>
      unwrap(api.DELETE("/v1/lists/{listId}/imports/{importId}", { params: { path: { listId, importId: id } } })),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: keys.listImport(ws, listId, id) })
      void qc.invalidateQueries({ queryKey: keys.listImports(ws, listId) })
      void qc.invalidateQueries({ queryKey: keys.list(ws, listId) })
    },
  })
}

export function downloadErrors(listId: string, i: ListImport) {
  return saveDownload(
    api.GET("/v1/lists/{listId}/imports/{importId}/errors", {
      params: { path: { listId, importId: i.id } },
      parseAs: "blob",
    }),
    `refused-${i.filename.replace(/\.csv$/i, "")}.csv`
  )
}

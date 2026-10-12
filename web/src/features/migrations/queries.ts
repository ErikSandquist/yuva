import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useEffect } from "react"

import {
  api,
  unwrap,
  type Migration,
  type MigrationCredentialsInput,
  type MigrationMboxFormat,
  type MigrationPresetId,
  type MigrationSignInStart,
  type MigrationSource,
  type MigrationState,
  type MigrationUpdate,
} from "@/lib/api"
import { usePresetName } from "@/features/migrations/text"
import { keys } from "@/lib/keys"
import { setMigration } from "@/lib/live"
import { useRealtimeStatus } from "@/lib/realtime"
import { useSession } from "@/lib/session"

export const RUNNING: MigrationState[] = ["scanning", "copying", "syncing", "removing"]

function useRefetchOnReconnect(queryKey: readonly unknown[], enabled: boolean) {
  const qc = useQueryClient()
  const live = useRealtimeStatus() === "live"
  const key = JSON.stringify(queryKey)
  useEffect(() => {
    if (live && enabled) void qc.invalidateQueries({ queryKey: JSON.parse(key) as unknown[] })
  }, [live, enabled, key, qc])
}

export function useMigrations() {
  const { workspaceId: ws, canManage } = useSession()
  useRefetchOnReconnect(keys.migrationPages(ws), canManage)
  return useInfiniteQuery({
    queryKey: keys.migrationPages(ws),
    queryFn: ({ pageParam }) =>
      unwrap(api.GET("/v1/migrations", { params: { query: { cursor: pageParam, limit: 50 } } })),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
    enabled: canManage,
  })
}

export function useMigration(id: string | undefined) {
  const { workspaceId: ws, canManage } = useSession()
  useRefetchOnReconnect(keys.migration(ws, id ?? ""), canManage && !!id)
  return useQuery({
    queryKey: keys.migration(ws, id ?? ""),
    queryFn: () => unwrap(api.GET("/v1/migrations/{migrationId}", { params: { path: { migrationId: id! } } })),
    enabled: canManage && !!id,
    retry: false,
  })
}

export function useMigrationFolders(id: string, enabled = true) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.migrationFolders(ws, id),
    queryFn: () =>
      unwrap(api.GET("/v1/migrations/{migrationId}/folders", { params: { path: { migrationId: id } } })).then(
        (r) => r.items
      ),
    enabled,
  })
}

export function usePresets() {
  const { workspaceId: ws, canManage } = useSession()
  const presetName = usePresetName()
  return useQuery({
    queryKey: keys.migrationPresets(ws),
    queryFn: () => unwrap(api.GET("/v1/migrations/presets")).then((r) => r.items),
    select: (items) => items.map((p) => ({ ...p, name: presetName(p.id, p.name) })),
    staleTime: 10 * 60_000,
    enabled: canManage,
  })
}

export function useDetect() {
  return useMutation({
    mutationFn: (address: string) => unwrap(api.POST("/v1/migrations/detect", { body: { address } })),
  })
}

export function usePutMigration() {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return (m: Migration) => setMigration(qc, ws, m)
}

type CreateBody = {
  source?: MigrationSource
  preset: MigrationPresetId
  source_address: string
  inbox_id?: string
  channel_id?: string
}

export function useCreateMigration() {
  const put = usePutMigration()
  return useMutation({
    mutationFn: (body: CreateBody) => unwrap(api.POST("/v1/migrations", { body })),
    onSuccess: put,
  })
}

export function useReplaceDraft() {
  const put = usePutMigration()
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return useMutation({
    mutationFn: async ({ draft, body }: { draft: Migration; body: CreateBody }) => {
      const m = await unwrap(api.POST("/v1/migrations", { body }))
      put(m)
      if (draft.state === "draft" && draft.actions.includes("delete")) {
        const gone = await unwrap(
          api.DELETE("/v1/migrations/{migrationId}", { params: { path: { migrationId: draft.id } } })
        ).then(
          () => true,
          () => false
        )
        if (gone) qc.removeQueries({ queryKey: keys.migration(ws, draft.id) })
      }
      void qc.invalidateQueries({ queryKey: keys.migrationPages(ws) })
      return m
    },
  })
}

export function useUpdateMigration(id: string) {
  const put = usePutMigration()
  return useMutation({
    mutationFn: (body: MigrationUpdate) =>
      unwrap(api.PATCH("/v1/migrations/{migrationId}", { params: { path: { migrationId: id } }, body })),
    onSuccess: put,
  })
}

export function useSetCredentials(id: string) {
  return useMutation({
    mutationFn: (body: MigrationCredentialsInput) =>
      unwrap(api.PUT("/v1/migrations/{migrationId}/credentials", { params: { path: { migrationId: id } }, body })),
  })
}

export function useTestConnection(id: string) {
  const put = usePutMigration()
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return useMutation({
    mutationFn: () => unwrap(api.POST("/v1/migrations/{migrationId}/test", { params: { path: { migrationId: id } } })),
    onSuccess: (r) => {
      put(r.migration)
      void qc.invalidateQueries({ queryKey: keys.migrationFolders(ws, id) })
    },
  })
}

export function useStartSignIn(id: string) {
  return useMutation({
    mutationFn: (body: MigrationSignInStart) =>
      unwrap(api.POST("/v1/migrations/{migrationId}/sign-in", { params: { path: { migrationId: id } }, body })),
  })
}

export function useMigrationUpload(id: string, enabled: boolean) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.migrationUpload(ws, id),
    queryFn: () => unwrap(api.GET("/v1/migrations/{migrationId}/upload", { params: { path: { migrationId: id } } })),
    enabled,
    retry: false,
  })
}

export function uploadMigrationPart(id: string, part: number, blob: Blob, sha256: string, signal: AbortSignal) {
  return unwrap(
    api.POST("/v1/migrations/{migrationId}/upload/parts/{part}", {
      params: { path: { migrationId: id, part } },
      body: { file: blob as unknown as string, sha256 },
      bodySerializer: () => {
        const form = new FormData()
        form.append("sha256", sha256)
        form.append("file", blob, `part-${part}`)
        return form
      },
      signal,
    })
  )
}

export function completeMigrationUpload(
  id: string,
  body: { parts: number; size_bytes: number; filename?: string; mbox_format?: MigrationMboxFormat }
) {
  return unwrap(
    api.POST("/v1/migrations/{migrationId}/upload/complete", { params: { path: { migrationId: id } }, body })
  )
}

export function useMigrationPreview(m: Migration) {
  const { workspaceId: ws } = useSession()
  const scannedAt = m.estimate?.scanned_at ?? ""
  return useQuery({
    queryKey: keys.migrationPreview(ws, m.id, scannedAt),
    queryFn: () =>
      unwrap(api.GET("/v1/migrations/{migrationId}/preview", { params: { path: { migrationId: m.id } } })).then(
        (r) => r.items
      ),
    enabled: !!scannedAt,
    staleTime: Infinity,
  })
}

export function useMigrationErrors(id: string, enabled: boolean) {
  const { workspaceId: ws } = useSession()
  return useInfiniteQuery({
    queryKey: keys.migrationErrors(ws, id),
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET("/v1/migrations/{migrationId}/errors", {
          params: { path: { migrationId: id }, query: { cursor: pageParam, limit: 50 } },
        })
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
    enabled,
  })
}

type Step = "scan" | "start" | "pause" | "resume" | "cancel" | "retry" | "probe"

const stepPaths = {
  scan: "/v1/migrations/{migrationId}/scan",
  start: "/v1/migrations/{migrationId}/start",
  pause: "/v1/migrations/{migrationId}/pause",
  resume: "/v1/migrations/{migrationId}/resume",
  cancel: "/v1/migrations/{migrationId}/cancel",
  retry: "/v1/migrations/{migrationId}/retry",
  probe: "/v1/migrations/{migrationId}/probe",
} as const

export function useMigrationStep(id: string, step: Step) {
  const put = usePutMigration()
  return useMutation({
    mutationFn: () => unwrap(api.POST(stepPaths[step], { params: { path: { migrationId: id } } })),
    onSuccess: put,
  })
}

export function useFinishMigration(id: string) {
  const put = usePutMigration()
  return useMutation({
    mutationFn: (force: boolean) =>
      unwrap(
        api.POST("/v1/migrations/{migrationId}/finish", {
          params: { path: { migrationId: id } },
          body: force ? { force } : {},
        })
      ),
    onSuccess: put,
  })
}

export function useRemoveHistory(id: string) {
  const put = usePutMigration()
  return useMutation({
    mutationFn: (confirmCount: number) =>
      unwrap(
        api.POST("/v1/migrations/{migrationId}/remove-history", {
          params: { path: { migrationId: id } },
          body: { confirm_count: confirmCount },
        })
      ),
    onSuccess: put,
  })
}

export function useDeleteMigration(id: string) {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return useMutation({
    mutationFn: () => unwrap(api.DELETE("/v1/migrations/{migrationId}", { params: { path: { migrationId: id } } })),
    onSuccess: () => {
      qc.removeQueries({ queryKey: keys.migration(ws, id) })
      void qc.invalidateQueries({ queryKey: keys.migrationPages(ws) })
    },
  })
}

export function percentOf(m: Pick<Migration, "counts">) {
  const c = m.counts
  if (c.total <= 0) return 0
  return Math.min(100, Math.floor(((c.imported + c.duplicate + c.skipped + c.failed) / c.total) * 100))
}

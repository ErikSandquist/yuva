import { useQueryClient } from "@tanstack/react-query"
import { useCallback, useEffect, useRef, useState } from "react"

import { completeMigrationUpload, uploadMigrationPart, usePutMigration } from "@/features/migrations/queries"
import { api, ApiError, unwrap, type Migration, type MigrationMboxFormat, type MigrationUpload } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

export const PARALLEL = 4

const RETRIES = 6

export type PartState = "hashing" | "sending" | "retrying"

export type LocalProblem =
  | { kind: "too_large"; size: number; max: number }
  | { kind: "empty" }
  | { kind: "archive" }
  | { kind: "other_file"; stored: number }
  | { kind: "ended" }

export type Upload = {
  phase: "idle" | "checking" | "uploading" | "completing" | "done"
  file: File | null
  total: number
  partSize: number
  done: Set<number>
  doneBytes: number
  active: Map<number, PartState>
  error: unknown
  problem: LocalProblem | null
}

type Remembered = { name: string; size: number; modified: number }

function storeKey(id: string) {
  return `yuva.migration-upload.${id}`
}

export function remembered(id: string): Remembered | null {
  try {
    const raw = localStorage.getItem(storeKey(id))
    return raw ? (JSON.parse(raw) as Remembered) : null
  } catch {
    return null
  }
}

function remember(id: string, f: File | null) {
  try {
    if (f) localStorage.setItem(storeKey(id), JSON.stringify({ name: f.name, size: f.size, modified: f.lastModified }))
    else localStorage.removeItem(storeKey(id))
  } catch {
    return
  }
}

export async function sha256Hex(blob: Blob) {
  const digest = await crypto.subtle.digest("SHA-256", await blob.arrayBuffer())
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, "0")).join("")
}

export function isArchive(name: string) {
  return /\.(tgz|tar\.gz|tar|gz|bz2|7z|rar)$/i.test(name)
}

function sleep(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    const stop = () => {
      clearTimeout(timer)
      reject(signal.reason)
    }
    const timer = setTimeout(() => {
      signal.removeEventListener("abort", stop)
      resolve()
    }, ms)
    signal.addEventListener("abort", stop, { once: true })
  })
}

function online(signal: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    const stop = () => {
      window.removeEventListener("online", back)
      reject(signal.reason)
    }
    const back = () => {
      signal.removeEventListener("abort", stop)
      resolve()
    }
    window.addEventListener("online", back, { once: true })
    signal.addEventListener("abort", stop, { once: true })
  })
}

async function pause(ms: number, signal: AbortSignal) {
  await sleep(ms, signal)
  if (!navigator.onLine) await online(signal)
}

function retryable(err: unknown) {
  if (!(err instanceof ApiError)) return true
  return err.status >= 500 || err.status === 408 || err.status === 429 || err.code === "checksum_mismatch"
}

const initial: Upload = {
  phase: "idle",
  file: null,
  total: 0,
  partSize: 0,
  done: new Set(),
  doneBytes: 0,
  active: new Map(),
  error: null,
  problem: null,
}

export function useExportUpload(m: Migration) {
  const qc = useQueryClient()
  const put = usePutMigration()
  const { workspaceId: ws } = useSession()
  const [state, setState] = useState<Upload>(initial)
  const abort = useRef<AbortController | null>(null)
  const mbox = useRef<MigrationMboxFormat>("mboxrd")
  const id = m.id

  useEffect(() => () => abort.current?.abort(), [])

  const busy = state.phase === "checking" || state.phase === "uploading" || state.phase === "completing"
  useEffect(() => {
    if (!busy) return
    const warn = (e: BeforeUnloadEvent) => e.preventDefault()
    window.addEventListener("beforeunload", warn)
    return () => window.removeEventListener("beforeunload", warn)
  }, [busy])

  const start = useCallback(
    async (file: File, opts: { fresh?: boolean; mbox?: MigrationMboxFormat } = {}) => {
      abort.current?.abort()
      if (opts.mbox) mbox.current = opts.mbox
      const ctl = new AbortController()
      abort.current = ctl
      const signal = ctl.signal
      const set = (fn: (s: Upload) => Upload) => {
        if (!signal.aborted) setState(fn)
      }
      const fail = (error: unknown, problem: LocalProblem | null = null) =>
        set((s) => ({ ...s, phase: "idle", active: new Map(), error, problem }))

      setState({ ...initial, file, phase: "checking" })
      if (file.size === 0) return fail(null, { kind: "empty" })
      if (isArchive(file.name)) return fail(null, { kind: "archive" })

      let upload: MigrationUpload
      try {
        upload = await unwrap(
          api.GET("/v1/migrations/{migrationId}/upload", { params: { path: { migrationId: id } }, signal })
        )
      } catch (err) {
        return fail(err)
      }
      qc.setQueryData(keys.migrationUpload(ws, id), upload)
      if (upload.deleted_at) return fail(null, { kind: "ended" })
      if (file.size > upload.max_bytes) return fail(null, { kind: "too_large", size: file.size, max: upload.max_bytes })

      const partSize = upload.part_size
      const total = Math.ceil(file.size / partSize)
      const sizeOf = (n: number) => Math.min(partSize, file.size - n * partSize)
      const stored = new Map(upload.parts.map((p) => [p.part, p]))
      const kept = opts.fresh
        ? []
        : upload.parts.filter((p) => p.part < total && p.size_bytes === sizeOf(p.part)).map((p) => p.part)

      if (!opts.fresh && upload.parts.length > 0) {
        const before = remembered(id)
        let same = !before || (before.size === file.size && before.name === file.name)
        const probes = kept.length > 0 ? [kept[0]!, kept[kept.length - 1]!] : []
        try {
          for (const n of new Set(probes)) {
            if (!same) break
            const sum = await sha256Hex(file.slice(n * partSize, n * partSize + sizeOf(n)))
            if (signal.aborted) return
            same = sum === stored.get(n)?.sha256
          }
        } catch (err) {
          return fail(err)
        }
        if (!same || kept.length === 0) return fail(null, { kind: "other_file", stored: upload.parts.length })
      }
      remember(id, file)

      const done = new Set(kept)
      let doneBytes = kept.reduce((sum, n) => sum + sizeOf(n), 0)
      set((s) => ({ ...s, phase: "uploading", total, partSize, done: new Set(done), doneBytes }))

      const mark = (n: number, st: PartState | null) =>
        set((s) => {
          const active = new Map(s.active)
          if (st) active.set(n, st)
          else active.delete(n)
          return { ...s, active }
        })

      const send = async (n: number) => {
        const blob = file.slice(n * partSize, n * partSize + sizeOf(n))
        for (let attempt = 0; ; attempt++) {
          mark(n, attempt === 0 ? "hashing" : "retrying")
          try {
            const sum = await sha256Hex(blob)
            if (signal.aborted) return
            mark(n, attempt === 0 ? "sending" : "retrying")
            await uploadMigrationPart(id, n, blob, sum, signal)
            break
          } catch (err) {
            if (signal.aborted) throw err
            if (attempt >= RETRIES || !retryable(err)) throw err
            mark(n, "retrying")
            await pause(Math.min(30_000, 1000 * 2 ** attempt), signal)
          }
        }
        done.add(n)
        doneBytes += blob.size
        const nowDone = new Set(done)
        const nowBytes = doneBytes
        set((s) => {
          const active = new Map(s.active)
          active.delete(n)
          return { ...s, active, done: nowDone, doneBytes: nowBytes }
        })
      }

      const sendAll = async (parts: number[]) => {
        const queue = [...parts]
        let first: unknown = null
        const worker = async () => {
          while (queue.length > 0 && !first && !signal.aborted) {
            const n = queue.shift()!
            try {
              await send(n)
            } catch (err) {
              first ??= err
            }
          }
        }
        await Promise.all(Array.from({ length: Math.min(PARALLEL, queue.length) }, worker))
        if (first) throw first
      }

      try {
        await sendAll(Array.from({ length: total }, (_, n) => n).filter((n) => !done.has(n)))
        if (signal.aborted) return
        set((s) => ({ ...s, phase: "completing", active: new Map() }))
        const body = {
          parts: total,
          size_bytes: file.size,
          filename: file.name.slice(0, 255),
          ...(mbox.current === "mboxo" ? { mbox_format: mbox.current } : {}),
        }
        let result: Migration
        try {
          result = await completeMigrationUpload(id, body)
        } catch (err) {
          const missing = (
            err instanceof ApiError && err.code === "upload_incomplete"
              ? ((err.body as { missing_parts?: number[] } | undefined)?.missing_parts ?? [])
              : []
          ).filter((n) => n < total)
          if (missing.length === 0) throw err
          for (const n of missing) if (done.delete(n)) doneBytes -= sizeOf(n)
          const left = doneBytes
          set((s) => ({ ...s, phase: "uploading", done: new Set(done), doneBytes: left }))
          await sendAll(missing)
          if (signal.aborted) return
          set((s) => ({ ...s, phase: "completing", active: new Map() }))
          result = await completeMigrationUpload(id, body)
        }
        if (signal.aborted) return
        remember(id, null)
        put(result)
        void qc.invalidateQueries({ queryKey: keys.migrationUpload(ws, id) })
        set((s) => ({ ...s, phase: "done", active: new Map(), error: null, problem: null }))
      } catch (err) {
        if (signal.aborted) return
        if (err instanceof ApiError && err.code === "file_refused") remember(id, null)
        void qc.invalidateQueries({ queryKey: keys.migrationUpload(ws, id) })
        fail(err)
      }
    },
    [id, put, qc, ws]
  )

  const stop = useCallback(() => {
    abort.current?.abort()
    setState((s) => ({ ...s, phase: "idle", active: new Map() }))
    void qc.invalidateQueries({ queryKey: keys.migrationUpload(ws, id) })
  }, [id, qc, ws])

  return { state, start, stop, busy }
}

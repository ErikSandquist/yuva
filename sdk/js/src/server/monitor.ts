import type { components } from "../schema.gen"
import { createSender } from "./sender"

type ApiRecord = components["schemas"]["AnalyticsRequestRecord"]
type WireRecord = Omit<ApiRecord, "sample_rate"> & Partial<Pick<ApiRecord, "sample_rate">>

export const MAX_RECORD_BATCH = 500

export interface MonitorConfig {
  server: string
  apiKey: string
  propertyId: string
  environment?: string
  service?: string
  release?: string
  sampleRate?: number
  fetch?: typeof fetch
  flushInterval?: number
  queueSize?: number
  maxRetries?: number
  minBackoff?: number
  onError?: (error: Error) => void
}

export interface RequestRecord {
  method: string
  route?: string
  path?: string
  status: number
  duration: number
  requestSize?: number
  responseSize?: number
  time?: Date
  consumer?: string
  apiVersion?: string
  environment?: string
  service?: string
  release?: string
  ip?: string
  sampleRate?: number
}

export interface Monitor {
  send(record: RequestRecord): void
  record(record: RequestRecord): void
  flushed(): Promise<void>
  flush(): Promise<Error | undefined>
  close(): Promise<Error | undefined>
}

export interface MonitorOptions<R> {
  consumer?: (request: R) => string | null | undefined
  apiVersion?: (request: R) => string | null | undefined
  clientIP?: (request: R) => string | null | undefined
}

export class MonitorRecordError extends Error {
  readonly record: RequestRecord
  readonly detail: string

  constructor(record: RequestRecord, detail: string) {
    super(`analytics: request record ${record.method} ${record.route || record.path} refused: ${detail}`)
    this.name = "MonitorRecordError"
    this.record = record
    this.detail = detail
  }
}

const QUEUE_FULL = "analytics: the request record queue is full; record dropped"
const CLOSED = "analytics: monitor closed"

const size = (n: number | undefined) => (n !== undefined && Number.isFinite(n) && n >= 0 ? Math.floor(n) : undefined)

function wire(r: RequestRecord): WireRecord {
  return {
    method: r.method,
    route: r.route || undefined,
    path: r.route ? undefined : r.path,
    status: r.status,
    duration_us: Math.max(Math.round(r.duration * 1000), 0),
    request_size: size(r.requestSize),
    response_size: size(r.responseSize),
    time: r.time!.toISOString(),
    consumer: r.consumer || undefined,
    api_version: r.apiVersion || undefined,
    environment: r.environment || undefined,
    service: r.service || undefined,
    release: r.release || undefined,
    ip: r.ip || undefined,
    sample_rate: r.sampleRate && r.sampleRate > 1 ? r.sampleRate : undefined,
  }
}

const reporters = new WeakMap<Monitor, (error: Error) => void>()

export function createMonitor(config: MonitorConfig): Monitor {
  if (!config.server || !config.apiKey || !config.propertyId) {
    throw new Error("analytics: server, apiKey and propertyId are required")
  }
  const rate = config.sampleRate && config.sampleRate > 1 ? Math.floor(config.sampleRate) : 1
  let closed = false
  const report = (error: Error) => {
    try {
      config.onError?.(error)
    } catch {}
  }
  const sender = createSender<RequestRecord>({
    endpoint: `${config.server.replace(/\/+$/, "")}/v1/analytics/properties/${encodeURIComponent(config.propertyId)}/requests`,
    apiKey: config.apiKey,
    fetch: config.fetch,
    key: "records",
    batch: MAX_RECORD_BATCH,
    gzip: true,
    flushInterval: config.flushInterval,
    queueSize: config.queueSize && config.queueSize > 0 ? config.queueSize : 50_000,
    maxRetries: config.maxRetries,
    minBackoff: config.minBackoff,
    full: QUEUE_FULL,
    wire,
    refused: (record, detail) => new MonitorRecordError(record, detail),
    report,
  })

  function send(record: RequestRecord): void {
    if (!record.method || (!record.route && !record.path)) {
      throw new Error("analytics: a request record needs a method and a route or path")
    }
    if (closed) throw new Error(CLOSED)
    sender.push({ ...record, time: record.time ?? new Date() })
  }

  const monitor: Monitor = {
    send,
    record(record) {
      if (closed) return
      let weight = 1
      if (rate > 1 && record.status > 0 && record.status < 400) {
        if (Math.random() * rate >= 1) return
        weight = rate
      }
      send({
        ...record,
        environment: record.environment || config.environment,
        service: record.service || config.service,
        release: record.release || config.release,
        sampleRate: weight,
      })
    },
    flushed: () => sender.flushed(),
    flush: () => sender.run(true),
    async close() {
      if (closed) return undefined
      closed = true
      sender.stop()
      return sender.run(true)
    },
  }
  reporters.set(monitor, report)
  return monitor
}

function placeholder(segment: string): string {
  if (segment === "*" || segment === "**") return "*"
  const rest = /^(?:\*\*:|\*)(\w+)$/.exec(segment)
  if (rest) return `{${rest[1]}...}`
  return segment.replace(/:(\w+)(?:\{[^}]*\}|\([^)]*\))?[?+*]?/g, "{$1}")
}

export function routePattern(pattern: string): string {
  const route = pattern.split("/").map(placeholder).join("/")
  return route.startsWith("/") ? route : `/${route}`
}

export function contentLength(value: string | number | null | undefined): number | undefined {
  const n = typeof value === "number" ? value : value ? Number(value) : Number.NaN
  return Number.isInteger(n) && n >= 0 ? n : undefined
}

export interface Started {
  time: Date
  at: number
}

export const started = (): Started => ({ time: new Date(), at: performance.now() })

export interface Observed extends Started {
  method: string
  route?: string
  path: string
  status: number
  requestSize?: number
  responseSize?: number
}

export function observe<R>(monitor: Monitor, options: MonitorOptions<R> | undefined, request: R, o: Observed): void {
  try {
    const value = (fn: ((request: R) => string | null | undefined) | undefined) => fn?.(request) || undefined
    monitor.record({
      method: o.method,
      route: o.route ? routePattern(o.route) : undefined,
      path: o.path.split(/[?#]/)[0],
      status: o.status,
      duration: performance.now() - o.at,
      requestSize: o.requestSize,
      responseSize: o.responseSize,
      time: o.time,
      consumer: value(options?.consumer),
      apiVersion: value(options?.apiVersion),
      ip: value(options?.clientIP),
    })
  } catch (error) {
    reporters.get(monitor)?.(error instanceof Error ? error : new Error(String(error)))
  }
}

export function errorStatus(error: unknown): number {
  const e = error as { status?: unknown; statusCode?: unknown } | null
  const status = Number(e?.status ?? e?.statusCode)
  return Number.isInteger(status) && status >= 100 && status <= 599 ? status : 500
}

interface Closable {
  once(event: "close", listener: () => void): unknown
}

const closing = new WeakMap<object, WeakSet<Monitor>>()

export function flushOnClose(server: Closable | null | undefined, monitor: Monitor): void {
  if (!server || typeof server.once !== "function") return
  let monitors = closing.get(server)
  if (!monitors) closing.set(server, (monitors = new WeakSet()))
  if (monitors.has(monitor)) return
  monitors.add(monitor)
  server.once("close", () => void monitor.flush())
}

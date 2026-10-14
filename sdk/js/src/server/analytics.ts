import type { components } from "../schema.gen"

type WireEvent = components["schemas"]["AnalyticsServerEvent"]
type EventsResult = components["schemas"]["AnalyticsEventsResult"]

export const MAX_BATCH = 100

const defaultInterval = 5_000
const defaultQueue = 10_000
const defaultRetries = 5
const defaultMinBackoff = 1_000
const maxBackoff = 60_000

export interface ServerAnalyticsConfig {
  server: string
  apiKey: string
  propertyId: string
  fetch?: typeof fetch
  flushInterval?: number
  queueSize?: number
  maxRetries?: number
  minBackoff?: number
  clientIP?: (request: Request) => string | null | undefined
  onError?: (error: Error) => void
}

export interface ServerEvent {
  name: string
  url?: string
  referrer?: string
  time?: Date
  props?: Record<string, string | number | boolean>
  revenue?: { amount: string; currency: string }
  nonInteractive?: boolean
  consent?: boolean
  ip?: string
  userAgent?: string
  secChUaPlatform?: string
  secChUaMobile?: string
  acceptLanguage?: string
  visitorKey?: string
  persistentId?: string
  identityToken?: string
}

export interface ServerAnalytics {
  track(request: Request, event: ServerEvent): void
  send(event: ServerEvent): void
  flush(): Promise<Error | undefined>
  close(): Promise<Error | undefined>
}

export class AnalyticsEventError extends Error {
  readonly event: ServerEvent
  readonly detail: string

  constructor(event: ServerEvent, detail: string) {
    super(`analytics: event ${JSON.stringify(event.name)} refused: ${detail}`)
    this.name = "AnalyticsEventError"
    this.event = event
    this.detail = detail
  }
}

export class AnalyticsBatchError extends Error {
  readonly status: number
  readonly body: string
  readonly events: number

  constructor(events: number, status: number, body: string, cause?: unknown) {
    super(
      cause === undefined
        ? `analytics: ${events} events not sent: ${status} ${body}`
        : `analytics: ${events} events not sent: ${cause instanceof Error ? cause.message : String(cause)}`,
      cause === undefined ? undefined : { cause }
    )
    this.name = "AnalyticsBatchError"
    this.status = status
    this.body = body
    this.events = events
  }
}

const QUEUE_FULL = "analytics: the queue is full; event dropped"
const CLOSED = "analytics: client closed"

const rfc3339 = (time: Date) => time.toISOString().replace(/\.?0+Z$/, "Z")

function wire(event: ServerEvent): WireEvent {
  return {
    name: event.name,
    url: event.url!,
    referrer: event.referrer || undefined,
    timestamp: rfc3339(event.time!),
    props: event.props && Object.keys(event.props).length > 0 ? event.props : undefined,
    revenue: event.revenue ? { a: event.revenue.amount, c: event.revenue.currency } : undefined,
    interactive: event.nonInteractive ? false : undefined,
    consent: event.consent || undefined,
    ip: event.ip || undefined,
    user_agent: event.userAgent || undefined,
    sec_ch_ua_platform: event.secChUaPlatform || undefined,
    sec_ch_ua_mobile: event.secChUaMobile || undefined,
    accept_language: event.acceptLanguage || undefined,
    visitor_key: event.visitorKey || undefined,
    persistent_id: event.persistentId || undefined,
    identity_token: event.identityToken || undefined,
  }
}

function requestURL(request: Request): string {
  const url = new URL(request.url)
  if (request.headers.get("x-forwarded-proto")?.toLowerCase() === "https") url.protocol = "https:"
  url.hash = ""
  return url.href
}

// srvx sets `ip` from the socket; a forwarded header is only trustworthy behind a proxy the host knows.
const socketIP = (request: Request) => {
  const ip = (request as { ip?: unknown }).ip
  return typeof ip === "string" ? ip : undefined
}

type Timer = ReturnType<typeof setTimeout>

function unref(timer: Timer): Timer {
  if (typeof timer === "object" && timer && "unref" in timer && typeof timer.unref === "function") timer.unref()
  else (globalThis as { Deno?: { unrefTimer?(id: unknown): void } }).Deno?.unrefTimer?.(timer)
  return timer
}

const sleep = (ms: number, hold: boolean) =>
  new Promise<void>((resolve) => {
    const timer = setTimeout(resolve, ms)
    if (!hold) unref(timer)
  })

export function createAnalytics(config: ServerAnalyticsConfig): ServerAnalytics {
  if (!config.server || !config.apiKey || !config.propertyId) {
    throw new Error("analytics: server, apiKey and propertyId are required")
  }
  const endpoint = `${config.server.replace(/\/+$/, "")}/v1/analytics/properties/${encodeURIComponent(config.propertyId)}/events`
  const interval = config.flushInterval && config.flushInterval > 0 ? config.flushInterval : defaultInterval
  const queueSize = config.queueSize && config.queueSize > 0 ? config.queueSize : defaultQueue
  const maxRetries = config.maxRetries || defaultRetries
  const minBackoff = config.minBackoff && config.minBackoff > 0 ? config.minBackoff : defaultMinBackoff
  const clientIP = config.clientIP ?? socketIP

  let queue: ServerEvent[] = []
  let closed = false
  let timer: Timer | undefined
  let chain: Promise<unknown> = Promise.resolve()

  const report = (error: Error) => {
    try {
      config.onError?.(error)
    } catch {}
  }

  async function post(body: string) {
    const res = await (config.fetch ?? fetch)(endpoint, {
      method: "POST",
      headers: { Authorization: `Bearer ${config.apiKey}`, "Content-Type": "application/json" },
      body,
    })
    const raw = (await res.text()).slice(0, 64 << 10).trim()
    const seconds = Number(res.headers.get("retry-after"))
    return { status: res.status, raw, retryAfter: Number.isInteger(seconds) && seconds > 0 ? seconds * 1000 : 0 }
  }

  async function sendBatch(batch: ServerEvent[], hold: boolean): Promise<Error | undefined> {
    const body = JSON.stringify({ events: batch.map(wire) })
    let backoff = minBackoff
    for (let attempt = 0; ; attempt++) {
      const res = await post(body).catch((error: unknown) => ({ error }))
      if (!("error" in res)) {
        if (res.status === 202) {
          try {
            const result = JSON.parse(res.raw) as EventsResult
            for (const { index, detail } of result.errors ?? []) {
              const event = batch[index]
              if (event) report(new AnalyticsEventError(event, detail))
            }
          } catch {}
          return undefined
        }
        if (res.status !== 429 && res.status < 500) return new AnalyticsBatchError(batch.length, res.status, res.raw)
      }
      if (attempt >= maxRetries) {
        return "error" in res
          ? new AnalyticsBatchError(batch.length, 0, "", res.error)
          : new AnalyticsBatchError(batch.length, res.status, res.raw)
      }
      await sleep(Math.max(backoff, "error" in res ? 0 : res.retryAfter), hold)
      backoff = Math.min(backoff * 2, maxBackoff)
    }
  }

  async function sendAll(hold: boolean): Promise<Error | undefined> {
    let last: Error | undefined
    for (;;) {
      const batch = queue.slice(0, MAX_BATCH)
      queue = queue.slice(batch.length)
      if (batch.length === 0) return last
      const error = await sendBatch(batch, hold)
      if (error) {
        last = error
        report(error)
      }
    }
  }

  const run = (hold: boolean): Promise<Error | undefined> => {
    const next = chain.then(() => sendAll(hold))
    chain = next
    return next
  }

  function send(event: ServerEvent): void {
    if (!event.name || !event.url) throw new Error("analytics: an event needs a name and a url")
    if (closed) throw new Error(CLOSED)
    if (queue.length >= queueSize) {
      report(new Error(QUEUE_FULL))
      return
    }
    queue.push({ ...event, time: event.time ?? new Date() })
    if (queue.length >= MAX_BATCH) {
      void run(false)
    } else if (!timer) {
      timer = unref(
        setTimeout(() => {
          timer = undefined
          void run(false)
        }, interval)
      )
    }
  }

  function track(request: Request, event: ServerEvent): void {
    const headers = request.headers
    if (headers.get("dnt") === "1" || headers.get("sec-gpc") === "1") return
    const e = { ...event }
    if (!e.visitorKey) {
      if (!e.ip) e.ip = clientIP(request) ?? undefined
      if (!e.userAgent) {
        e.userAgent = headers.get("user-agent") ?? undefined
        e.secChUaPlatform = headers.get("sec-ch-ua-platform") ?? undefined
        e.secChUaMobile = headers.get("sec-ch-ua-mobile") ?? undefined
      }
    }
    if (!e.acceptLanguage) e.acceptLanguage = headers.get("accept-language") ?? undefined
    if (!e.url) e.url = requestURL(request)
    if (!e.referrer) e.referrer = headers.get("referer") ?? undefined
    send(e)
  }

  return {
    track,
    send,
    flush: () => run(true),
    async close() {
      if (closed) return undefined
      closed = true
      if (timer) clearTimeout(timer)
      timer = undefined
      return run(true)
    },
  }
}

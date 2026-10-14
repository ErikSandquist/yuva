import type { components } from "../schema.gen"
import { AnalyticsBatchError, createSender } from "./sender"

export { AnalyticsBatchError }

type WireEvent = components["schemas"]["AnalyticsServerEvent"]

export const MAX_BATCH = 100

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

export function createAnalytics(config: ServerAnalyticsConfig): ServerAnalytics {
  if (!config.server || !config.apiKey || !config.propertyId) {
    throw new Error("analytics: server, apiKey and propertyId are required")
  }
  const clientIP = config.clientIP ?? socketIP
  let closed = false
  const report = (error: Error) => {
    try {
      config.onError?.(error)
    } catch {}
  }
  const sender = createSender<ServerEvent>({
    endpoint: `${config.server.replace(/\/+$/, "")}/v1/analytics/properties/${encodeURIComponent(config.propertyId)}/events`,
    apiKey: config.apiKey,
    fetch: config.fetch,
    key: "events",
    batch: MAX_BATCH,
    gzip: false,
    flushInterval: config.flushInterval,
    queueSize: config.queueSize && config.queueSize > 0 ? config.queueSize : 10_000,
    maxRetries: config.maxRetries,
    minBackoff: config.minBackoff,
    full: QUEUE_FULL,
    wire,
    refused: (event, detail) => new AnalyticsEventError(event, detail),
    report,
  })

  function send(event: ServerEvent): void {
    if (!event.name || !event.url) throw new Error("analytics: an event needs a name and a url")
    if (closed) throw new Error(CLOSED)
    sender.push({ ...event, time: event.time ?? new Date() })
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
    flush: () => sender.run(true),
    async close() {
      if (closed) return undefined
      closed = true
      sender.stop()
      return sender.run(true)
    },
  }
}

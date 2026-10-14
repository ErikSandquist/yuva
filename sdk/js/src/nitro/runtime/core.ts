import {
  createAnalytics,
  createMonitor,
  routePattern,
  signIdentityToken,
  verifyWebhook,
  WebhookVerificationError,
  type WebhookEvent,
} from "../../server"
import type { YuvaContext, YuvaServer } from "./define"

export interface YuvaServerRuntimeConfig {
  server?: string
  identitySecret?: string
  webhookSecret?: string
  analytics?: { apiKey?: string; propertyId?: string }
  monitor?: YuvaMonitorRuntimeConfig
}

export interface YuvaMonitorRuntimeConfig {
  apiKey?: string
  propertyId?: string
  environment?: string
  service?: string
  release?: string
  sampleRate?: number
  flushInterval?: number
}

export interface YuvaRequestInfo {
  method: string
  path: string
  route?: string
  size?: string | null
}

export interface YuvaRuntimeConfig {
  yuva?: YuvaServerRuntimeConfig
  public?: { yuva?: { server?: string; analytics?: { server?: string } } }
}

export interface YuvaHookEvent {
  context: Record<string, unknown>
}

export interface YuvaHooks {
  hook(name: string, fn: (...args: never[]) => unknown): unknown
}

export interface YuvaAdapter {
  server: YuvaServer<YuvaHookEvent>
  config(): YuvaRuntimeConfig
  defineHandler(handler: (event: YuvaHookEvent) => Promise<Response>): unknown
  toRequest(event: YuvaHookEvent): Request
  trackRequest(event: YuvaHookEvent): Request
  onResponse(hooks: YuvaHooks, fn: (event: YuvaHookEvent) => void): void
  waitUntil(event: YuvaHookEvent): ((promise: Promise<unknown>) => void) | undefined
  serverlessWaitUntil(event: YuvaHookEvent): ((promise: Promise<unknown>) => void) | undefined
  isAsset(path: string): boolean
  requestInfo(event: YuvaHookEvent): YuvaRequestInfo
  onResponded(
    hooks: YuvaHooks,
    fn: (event: YuvaHookEvent, status: number, size: string | number | string[] | null | undefined) => void
  ): void
}

const noStore = { "cache-control": "no-store" }
const notFound = () => new Response(null, { status: 404 })

export function identityHandler(yuva: YuvaAdapter): unknown {
  return yuva.defineHandler(async (event) => {
    const secret = yuva.config().yuva?.identitySecret
    if (!secret || !yuva.server.user) return notFound()
    const claims = await yuva.server.user(event)
    if (!claims) return new Response(null, { status: 401, headers: noStore })
    return Response.json({ token: await signIdentityToken(secret, claims) }, { headers: noStore })
  })
}

export function webhookHandler(yuva: YuvaAdapter): unknown {
  return yuva.defineHandler(async (event) => {
    const secret = yuva.config().yuva?.webhookSecret
    if (!secret) return notFound()
    let payload: WebhookEvent
    try {
      payload = await verifyWebhook(secret, yuva.toRequest(event))
    } catch (error) {
      if (error instanceof WebhookVerificationError) return new Response(error.message, { status: 401 })
      throw error
    }
    const on = yuva.server.webhooks ?? {}
    if (!Object.hasOwn(on, payload.type)) return new Response(null, { status: 204 })
    const handler = on[payload.type] as ((payload: WebhookEvent, event: YuvaHookEvent) => unknown) | undefined
    const result = await handler?.(payload, event)
    return result instanceof Response ? result : new Response(null, { status: 204 })
  })
}

const yuvaServer = (config: YuvaRuntimeConfig) =>
  config.yuva?.server || config.public?.yuva?.analytics?.server || config.public?.yuva?.server

export function analyticsPlugin(yuva: YuvaAdapter) {
  return (nitroApp: { hooks: YuvaHooks }): void => {
    const config = yuva.config()
    const { apiKey, propertyId } = config.yuva?.analytics ?? {}
    const server = yuvaServer(config)
    if (!apiKey || !propertyId || !server) return
    const analytics = createAnalytics({ server, apiKey, propertyId })
    const tracked = new WeakSet<YuvaHookEvent>()
    nitroApp.hooks.hook("request", (event: YuvaHookEvent) => {
      event.context.yuva = {
        track(e) {
          analytics.track(yuva.trackRequest(event), e)
          tracked.add(event)
        },
      } satisfies YuvaContext
    })
    yuva.onResponse(nitroApp.hooks, (event) => {
      if (tracked.has(event)) yuva.waitUntil(event)?.(analytics.flush())
    })
    nitroApp.hooks.hook("close", () => analytics.close())
  }
}

const bytes = (value: string | number | string[] | null | undefined) => {
  const n = typeof value === "number" ? value : typeof value === "string" && value ? Number(value) : Number.NaN
  return Number.isInteger(n) && n >= 0 ? n : undefined
}

export function monitorPlugin(yuva: YuvaAdapter) {
  return (nitroApp: { hooks: YuvaHooks }): void => {
    const config = yuva.config()
    const { apiKey, propertyId, ...options } = config.yuva?.monitor ?? {}
    const server = yuvaServer(config)
    if (!apiKey || !propertyId || !server) return
    const monitor = createMonitor({ ...options, server, apiKey, propertyId })
    const starts = new WeakMap<YuvaHookEvent, { time: Date; at: number }>()
    nitroApp.hooks.hook("request", (event: YuvaHookEvent) => {
      starts.set(event, { time: new Date(), at: performance.now() })
    })
    yuva.onResponded(nitroApp.hooks, (event, status, size) => {
      const start = starts.get(event)
      if (!start) return
      starts.delete(event)
      const info = yuva.requestInfo(event)
      const path = info.path.split(/[?#]/)[0]!
      if (yuva.isAsset(path)) return
      monitor.record({
        method: info.method,
        route: info.route ? routePattern(info.route) : undefined,
        path,
        status,
        duration: performance.now() - start.at,
        time: start.time,
        requestSize: bytes(info.size),
        responseSize: bytes(size),
      })
      yuva.serverlessWaitUntil(event)?.(monitor.flush())
    })
    nitroApp.hooks.hook("close", () => monitor.close())
  }
}

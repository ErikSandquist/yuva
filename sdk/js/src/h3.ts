import {
  defineHandler,
  defineMiddleware,
  type EventHandlerRequest,
  type EventHandlerWithFetch,
  type H3Event,
  type Middleware,
  toResponse,
} from "h3"
import type { ServerAnalytics, ServerEvent } from "./server/analytics"
import {
  contentLength,
  flushOnClose,
  observe,
  serverless,
  started,
  type Monitor,
  type MonitorOptions,
} from "./server/monitor"
import { signIdentityToken, type IdentityClaims } from "./server/identity"
import { verifyWebhook, WebhookVerificationError, type WebhookEvent, type WebhookEventType } from "./server/webhook"

export interface YuvaContext {
  track(event: ServerEvent): void
}

declare module "h3" {
  interface H3EventContext {
    yuva?: YuvaContext
  }
}

type MaybePromise<T> = T | Promise<T>

export interface YuvaIdentityHandlerOptions {
  secret: string
  user: (event: H3Event) => MaybePromise<IdentityClaims | null | undefined>
  ttl?: number
}

const noStore = { "cache-control": "no-store" }

export function defineYuvaIdentityHandler({
  secret,
  user,
  ttl,
}: YuvaIdentityHandlerOptions): EventHandlerWithFetch<EventHandlerRequest, Promise<Response>> {
  return defineHandler(async (event) => {
    const claims = await user(event)
    if (!claims) return new Response(null, { status: 401, headers: noStore })
    const token = await signIdentityToken(secret, { ...claims, ttl: claims.ttl ?? ttl })
    return Response.json({ token }, { headers: noStore })
  })
}

export type WebhookPayload<K extends WebhookEventType> = Extract<WebhookEvent, { type: K }>

export type YuvaWebhookHandlers = {
  [K in WebhookEventType]?: (payload: WebhookPayload<K>, event: H3Event) => unknown
}

export interface YuvaWebhookHandlerOptions {
  secret: string
  tolerance?: number
  on: YuvaWebhookHandlers
}

export function defineYuvaWebhookHandler({
  secret,
  tolerance,
  on,
}: YuvaWebhookHandlerOptions): EventHandlerWithFetch<EventHandlerRequest, Promise<Response>> {
  return defineHandler(async (event) => {
    let payload: WebhookEvent
    try {
      payload = await verifyWebhook(secret, event.req, { tolerance })
    } catch (error) {
      if (error instanceof WebhookVerificationError) return new Response(error.message, { status: 401 })
      throw error
    }
    if (!Object.hasOwn(on, payload.type)) return new Response(null, { status: 204 })
    const handler = on[payload.type] as ((payload: WebhookEvent, event: H3Event) => unknown) | undefined
    const result = await handler?.(payload, event)
    return result instanceof Response ? result : new Response(null, { status: 204 })
  })
}

export interface YuvaAnalyticsOptions {
  analytics: ServerAnalytics
}

export function defineYuvaAnalytics({ analytics }: YuvaAnalyticsOptions): Middleware {
  return defineMiddleware(async (event, next) => {
    const request = event.req
    let tracked = false
    event.context.yuva = {
      track(e) {
        analytics.track(request, e)
        tracked = true
      },
    }
    try {
      return await next()
    } finally {
      if (tracked && request.waitUntil) {
        event.waitUntil(analytics.flush())
      }
    }
  })
}

export interface YuvaMonitorOptions extends MonitorOptions<H3Event> {
  monitor: Monitor
}

export function defineYuvaMonitor({ monitor, ...options }: YuvaMonitorOptions): Middleware {
  return defineMiddleware(async (event, next) => {
    const start = started()
    flushOnClose(event.runtime?.node?.req.socket?.server, monitor)
    let result: unknown
    try {
      result = next()
    } catch (error) {
      result = Promise.reject(error)
    }
    const response = await toResponse(result, event, { ...event.app?.config, onResponse: undefined })
    observe(monitor, options, event, {
      ...start,
      method: event.req.method,
      route: event.context.matchedRoute?.route,
      path: event.url.pathname,
      status: response.status,
      requestSize: contentLength(event.req.headers.get("content-length")),
      responseSize: contentLength(response.headers.get("content-length")),
    })
    if (event.req.waitUntil && serverless(event.req)) event.waitUntil(monitor.flush())
    return response
  })
}

import {
  createAnalytics,
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

export function analyticsPlugin(yuva: YuvaAdapter) {
  return (nitroApp: { hooks: YuvaHooks }): void => {
    const config = yuva.config()
    const { apiKey, propertyId } = config.yuva?.analytics ?? {}
    const server = config.yuva?.server || config.public?.yuva?.analytics?.server || config.public?.yuva?.server
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

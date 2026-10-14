import type { IdentityClaims, ServerEvent, WebhookEvent, WebhookEventType } from "../../server"

export interface YuvaServerTypes {}

export type YuvaServerEvent = YuvaServerTypes extends { event: infer E } ? E : import("h3").H3Event

type MaybePromise<T> = T | Promise<T>

export type YuvaServerWebhooks<E = YuvaServerEvent> = {
  [K in WebhookEventType]?: (payload: Extract<WebhookEvent, { type: K }>, event: E) => unknown
}

export interface YuvaServer<E = YuvaServerEvent> {
  user?: (event: E) => MaybePromise<IdentityClaims | null | undefined>
  webhooks?: YuvaServerWebhooks<E>
}

export interface YuvaContext {
  track(event: ServerEvent): void
}

export function defineYuvaServer<E = YuvaServerEvent>(server: YuvaServer<E>): YuvaServer<E> {
  return server
}

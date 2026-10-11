import {
  defineSubscribe,
  YuvaSubscribeElement,
  type SubscribeDetail,
  type SubscribeErrorDetail,
} from "./subscribe/element"

YuvaSubscribeElement.loadClient = () => import("./client/client")
defineSubscribe()

export { defineSubscribe, YuvaSubscribeElement, type SubscribeDetail, type SubscribeErrorDetail }
export type { IdentityTokenSource } from "./client/client"
export type { ClientMailingList, SubscriptionState } from "./client/api"

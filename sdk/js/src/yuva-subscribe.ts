import { defineSubscribe, scriptSrc, YuvaSubscribeElement, type SubscribeClientModule } from "./subscribe/element"

declare const YUVA_SUBSCRIBE_CLIENT: string

YuvaSubscribeElement.loadClient = () =>
  import(
    /* @vite-ignore */ new URL(YUVA_SUBSCRIBE_CLIENT, scriptSrc || location.href).href
  ) as Promise<SubscribeClientModule>
defineSubscribe()

import type { components } from "../schema.gen"

export type AnalyticsEvent = components["schemas"]["AnalyticsEvent"]
export type AnalyticsBatch = components["schemas"]["AnalyticsBatch"]

export interface AnalyticsOptions {
  server: string
  key: string
  hash?: boolean
  consent?: "required"
  manual?: boolean
}

export interface PageviewOptions {
  url?: string
  referrer?: string
}

export interface AnalyticsMethods {
  pageview(options?: PageviewOptions): void
  consent(choice: { analytics?: boolean } | boolean): void
  optOut(): void
  optIn(): void
}

export interface Analytics extends AnalyticsMethods {
  destroy(): void
}

export const OPT_OUT_KEY = "yuva_optout"

const noop = () => {}

export function createAnalytics(options: AnalyticsOptions): Analytics {
  const api: Analytics = { pageview: noop, consent: noop, optOut: noop, optIn: noop, destroy: noop }
  if (typeof window == "undefined" || !options.key) return api
  const nav = navigator as Navigator & { globalPrivacyControl?: boolean }
  const doc = document as Document & { prerendering?: boolean }
  const storage = () => {
    try {
      return localStorage
    } catch {}
  }
  const url = `${options.server.replace(/\/$/, "")}/client/v1/analytics/${options.key}`
  const hash = options.hash
  let blocked = nav.globalPrivacyControl === true || nav.doNotTrack == "1" || nav.webdriver
  let off = blocked || storage()?.getItem(OPT_OUT_KEY) == "1"
  let consented = !options.consent
  let queue: AnalyticsEvent[] = []
  let timer: ReturnType<typeof setTimeout> | undefined
  let view = ""
  let page = ""
  let referrer: string | undefined = doc.referrer
  let ms = 0
  let since = 0
  let depth = 0

  const flush = (beacon?: boolean) => {
    clearTimeout(timer)
    timer = undefined
    if (!queue.length) return
    const body = JSON.stringify(queue[1] ? { e: queue } : queue[0])
    queue = []
    if (!(beacon && nav.sendBeacon?.(url, body)))
      fetch(url, { method: "POST", body, keepalive: true, credentials: "omit" }).then(
        (response) => void (response.status == 403 && (off = blocked = true)),
        noop
      )
  }

  const send = (event: AnalyticsEvent) => {
    if (off || !consented) return
    if (hash) event.h = true
    if (options.consent) event.c = { a: 1 }
    queue.push(event)
    timer ??= setTimeout(flush)
  }

  const active = () => doc.visibilityState == "visible" && doc.hasFocus()
  const tick = () => {
    const now = Date.now()
    if (since) ms += now - since
    since = active() ? now : 0
  }
  const scroll = () => {
    depth = Math.max(
      depth,
      Math.min(100, Math.round(((scrollY + innerHeight) / doc.documentElement.scrollHeight) * 100)) || 0
    )
  }

  const engagement = () => {
    tick()
    if (page && ms > 0) send({ n: "engagement", u: page, ms: Math.min(ms, 18e5), sd: depth })
    ms = 0
    depth = 0
    scroll()
  }

  const key = () => location.pathname + (hash ? location.hash : "")

  const pageview = (opts: PageviewOptions = {}) => {
    if (off || !consented) return
    engagement()
    view = key()
    page = opts.url || location.href
    send({ n: "pageview", u: page, r: opts.referrer ?? referrer, w: innerWidth })
    referrer = undefined
  }

  const auto = () => !options.manual && view != key() && pageview()

  const leave = () => {
    engagement()
    flush(true)
  }
  const hidden = () => (doc.visibilityState == "hidden" ? leave() : tick())

  const click = (event: Event) => {
    const href = (event.target as Element).closest?.("a")?.getAttribute("href")
    const choice = href == "#yuva-optout" ? api.optOut : href == "#yuva-optin" ? api.optIn : 0
    if (choice) {
      event.preventDefault()
      choice()
    }
  }

  const push = history.pushState
  const wrapped: typeof push = function (this: History, ...args) {
    push.apply(this, args)
    auto()
  }
  history.pushState = wrapped

  const listeners: [EventTarget, string, EventListener][] = [
    [window, "popstate", auto],
    [window, "hashchange", auto],
    [window, "scroll", scroll],
    [window, "focus", tick],
    [window, "blur", tick],
    [window, "pagehide", leave],
    [doc, "visibilitychange", hidden],
    [doc, "click", click],
    [doc, "prerenderingchange", auto],
  ]
  for (const [target, type, listener] of listeners) target.addEventListener(type, listener)

  api.pageview = pageview
  api.consent = (choice) => {
    consented = typeof choice == "object" ? !!choice.analytics : choice
    if (consented) auto()
    else queue = []
  }
  api.optOut = () => {
    off = true
    queue = []
    storage()?.setItem(OPT_OUT_KEY, "1")
  }
  api.optIn = () => {
    off = blocked
    storage()?.removeItem(OPT_OUT_KEY)
  }
  api.destroy = () => {
    leave()
    off = true
    if (history.pushState == wrapped) history.pushState = push
    for (const [target, type, listener] of listeners) target.removeEventListener(type, listener)
  }

  if (!doc.prerendering) auto()
  return api
}

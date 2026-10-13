import type { components } from "../schema.gen"

export type AnalyticsEvent = components["schemas"]["AnalyticsEvent"]
export type AnalyticsBatch = components["schemas"]["AnalyticsBatch"]
export type AnalyticsProps = NonNullable<AnalyticsEvent["p"]>

export interface AnalyticsOptions {
  server: string
  key: string
  hash?: boolean
  consent?: "required"
  manual?: boolean
  /** Send `outbound_click` for clicks on links to another host. */
  outbound?: boolean
  /** Send `file_download` for clicks on links to pdf, zip, dmg, exe, apk, csv, xls(x), doc(x), ppt(x), mp3 and mp4 files. */
  files?: boolean
  /** Send `chat_open` when a `<yuva-chat>` on the page opens. */
  chat?: boolean
}

export interface PageviewOptions {
  url?: string
  referrer?: string
}

export interface TrackOptions {
  /** Up to 30 props; keys must not start with `$` or `yuva_`. */
  props?: AnalyticsProps
  /** The amount as a decimal string (`"29.90"`) or a number, in an ISO 4217 currency. Never converted. */
  revenue?: { amount: string | number; currency: string }
}

type Choice = { analytics?: boolean; persistent?: boolean }

export interface AnalyticsMethods {
  /** A custom event. `pageview`, `engagement`, `screen` and names starting with `$` or `yuva_` are refused by the server. */
  track(name: string, options?: TrackOptions): void
  /** Marks the current page as the site's 404 page (`not_found`, non-interactive). */
  notFound(): void
  pageview(options?: PageviewOptions): void
  /** `persistent: true` keeps a random id on this device for 13 months and sends it on every event; never under GPC or Do Not Track. Without it a stored id is withdrawn and deleted. */
  consent(choice: Choice | boolean): void
  /** Links the persistent id to a contact with a fresh identity token from your backend; needs `persistent` consent. */
  identify(token: string): void
  /** The persistent id while `persistent` consent is given, for erasure requests. */
  persistentId(): string | undefined
  optOut(): void
  optIn(): void
}

export interface Analytics extends AnalyticsMethods {
  destroy(): void
}

declare const YUVA_ANALYTICS_LITE: true | undefined

type Basic = Omit<Analytics, "identify" | "persistentId">

export const OPT_OUT_KEY = "yuva_optout"
export const PERSISTENT_KEY = "yuva_pid"

const noop = (): undefined => {}
// The script tag build defines it to stay within its size budget; it leaves out the consent features.
const lite = typeof YUVA_ANALYTICS_LITE != "undefined"

const stored = (): string | undefined => {
  try {
    const { id, createdAt } = JSON.parse(localStorage.getItem(PERSISTENT_KEY)!)
    if (Date.now() - createdAt < 3.4e10) return id
  } catch {}
}

const keep = (): string | undefined => {
  try {
    if (!stored())
      localStorage.setItem(PERSISTENT_KEY, JSON.stringify({ id: crypto.randomUUID(), createdAt: Date.now() }))
  } catch {}
  return stored()
}

const withdrawal = (u: string): AnalyticsEvent[] => {
  const p = stored()
  try {
    localStorage.removeItem(PERSISTENT_KEY)
  } catch {}
  return p ? [{ n: "yuva_consent", u, c: { a: 0, p } }] : []
}

export function createAnalytics(options: AnalyticsOptions): Analytics {
  if (typeof window == "undefined" || !options.key) {
    const idle: Basic = {
      pageview: noop,
      track: noop,
      notFound: noop,
      consent: noop,
      optOut: noop,
      optIn: noop,
      destroy: noop,
    }
    return lite ? (idle as Analytics) : { ...idle, identify: noop, persistentId: noop }
  }
  const nav = navigator as Navigator & { globalPrivacyControl?: boolean }
  const doc = document as Document & { prerendering?: boolean }
  const storage = () => {
    try {
      return localStorage
    } catch {}
  }
  const url = `${options.server.replace(/\/$/, "")}/client/v1/analytics/${options.key}`
  const hash = options.hash
  let blocked = nav.globalPrivacyControl || nav.doNotTrack == "1" || nav.webdriver
  let off = blocked || storage()?.getItem(OPT_OUT_KEY) == "1"
  let consented = !options.consent
  let queue: AnalyticsEvent[] = []
  let view = ""
  let page = ""
  let referrer: string | undefined = doc.referrer
  let ms = 0
  let since = 0
  let depth = 0
  let pid: string | undefined

  const flush = (beacon?: boolean) => {
    const body = JSON.stringify(queue[1] ? { e: queue } : queue[0])
    if (queue[0] && !((queue = []), beacon && nav.sendBeacon?.(url, body)))
      fetch(url, { method: "POST", body, keepalive: true, credentials: "omit" }).then(
        (res) => void (res.status == 403 && (off = blocked = true)),
        noop
      )
  }

  const send = (event: AnalyticsEvent) => {
    if (off || !consented) return
    if (hash) event.h = true
    if (lite ? options.consent : options.consent || pid) event.c = lite ? { a: 1 } : { ...event.c, a: 1, p: pid }
    if (queue.push(event) < 2) setTimeout(flush)
  }

  const tick = () => {
    const now = Date.now()
    if (since) ms += now - since
    since = doc.visibilityState == "visible" && doc.hasFocus() ? now : 0
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

  const track = (name: string, { props, revenue }: TrackOptions = {}) =>
    send({ n: name, u: page || location.href, p: props, $: revenue && { a: revenue.amount, c: revenue.currency } })

  const auto = () => !options.manual && view != key() && pageview()

  const leave = () => {
    engagement()
    flush(true)
  }

  const optOut = () => {
    off = true
    queue = lite ? [] : withdrawal(page || location.href)
    if (!lite) {
      pid = undefined
      flush(true)
    }
    storage()?.setItem(OPT_OUT_KEY, "1")
  }
  const optIn = () => {
    off = blocked
    storage()?.removeItem(OPT_OUT_KEY)
  }

  const push = history.pushState
  const wrapped: typeof push = function (this: History, ...args) {
    push.apply(this, args)
    auto()
  }
  history.pushState = wrapped

  // Capture on window also sees the document's events that do not bubble, such as prerenderingchange.
  const on: Record<string, (event: Event) => void> = {
    popstate: auto,
    hashchange: auto,
    prerenderingchange: auto,
    scroll,
    focus: tick,
    blur: tick,
    pagehide: leave,
    visibilitychange: () => (doc.visibilityState == "hidden" ? leave() : tick()),
    click: (event) => {
      const link = (event.target as Element).closest?.("a")
      if (!link) return
      const href = link.getAttribute("href")
      const choice = href == "#yuva-optout" ? optOut : href == "#yuva-optin" ? optIn : 0
      const name =
        options.files && /\.(pdf|zip|dmg|exe|apk|csv|xlsx?|docx?|pptx?|mp[34])$/i.test(link.pathname)
          ? "file_download"
          : options.outbound && link.host != location.host && "outbound_click"
      if (choice) {
        event.preventDefault()
        choice()
      } else if (name && /^https?:/.test(link.href)) track(name, { props: { url: link.href } })
    },
  }
  if (options.chat) on["yuva-open"] = () => track("chat_open")
  for (const t in on) addEventListener(t, on[t]!, true)

  if (!doc.prerendering) auto()
  const api: Basic = {
    pageview,
    track,
    notFound: () => track("not_found"),
    consent: (choice) => {
      consented = choice === true || !!(choice as Choice).analytics
      if (!lite) {
        pid = undefined
        if (consented && (choice as Choice).persistent && !off) pid = keep()
      }
      if (consented) auto()
      else queue = []
      if (!lite && !pid) {
        const [withdrawn] = withdrawal(page || location.href)
        if (withdrawn) {
          queue.push(withdrawn)
          flush(true)
        }
      }
    },
    optOut,
    optIn,
    destroy: () => {
      leave()
      off = true
      if (history.pushState == wrapped) history.pushState = push
      for (const t in on) removeEventListener(t, on[t]!, true)
    },
  }
  return lite
    ? (api as Analytics)
    : {
        ...api,
        identify: (j: string) => void (pid && send({ n: "yuva_consent", u: page || location.href, c: { j } })),
        persistentId: () => pid,
      }
}

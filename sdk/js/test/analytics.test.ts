import { afterEach, beforeEach, describe, expect, setSystemTime, test } from "bun:test"
import {
  createAnalytics,
  OPT_OUT_KEY,
  type Analytics,
  type AnalyticsBatch,
  type AnalyticsEvent,
} from "../src/analytics"

interface Sent {
  via: "fetch" | "beacon"
  url: string
  body: AnalyticsBatch
  init?: RequestInit
}

const server = "https://yuva.test"
const endpoint = `${server}/client/v1/analytics/yuva_ak_test`
const realStorage = globalThis.localStorage

let sent: Sent[] = []
let status = 202
let beacon = true
let visibility: DocumentVisibilityState = "visible"
let instance: Analytics | undefined
let now = Date.parse("2026-10-12T10:00:00Z")

const events = (): AnalyticsEvent[] => sent.flatMap(({ body }) => ("e" in body ? body.e : [body]))
const names = () => events().map((event) => `${event.n} ${new URL(event.u).pathname}${new URL(event.u).hash}`)
const settle = () => new Promise((resolve) => setTimeout(resolve, 5))
const advance = (ms: number) => setSystemTime((now += ms))
const start = (options: Partial<Parameters<typeof createAnalytics>[0]> = {}) =>
  (instance = createAnalytics({ server, key: "yuva_ak_test", ...options }))
const hide = () => {
  visibility = "hidden"
  document.dispatchEvent(new Event("visibilitychange"))
}
const show = () => {
  visibility = "visible"
  document.dispatchEvent(new Event("visibilitychange"))
}
const signal = (name: string, value: unknown) =>
  Object.defineProperty(navigator, name, { value, configurable: true, writable: true })

beforeEach(() => {
  ;(window as unknown as { happyDOM: { setURL(url: string): void } }).happyDOM.setURL("https://site.test/")
  sent = []
  status = 202
  beacon = true
  visibility = "visible"
  setSystemTime(now)
  realStorage.clear()
  for (const name of ["globalPrivacyControl", "doNotTrack", "webdriver"]) signal(name, undefined)
  Object.defineProperty(document, "visibilityState", { get: () => visibility, configurable: true })
  document.hasFocus = () => true
  globalThis.fetch = (async (url: string, init: RequestInit) => {
    sent.push({ via: "fetch", url, init, body: JSON.parse(init.body as string) })
    return new Response(null, { status })
  }) as typeof fetch
  signal("sendBeacon", (url: string, body: string) => {
    if (!beacon) return false
    sent.push({ via: "beacon", url, body: JSON.parse(body) })
    return true
  })
})

afterEach(() => {
  instance?.destroy()
  instance = undefined
  setSystemTime()
})

describe("default mode", () => {
  test("touches no cookie and no storage except reading the opt-out", async () => {
    const touched: string[] = []
    const watch = (name: string, value: object) =>
      new Proxy(value, {
        get(target, property) {
          touched.push(`${name}.${String(property)}`)
          const member = Reflect.get(target, property, target) as unknown
          return typeof member == "function" ? member.bind(target) : member
        },
      })
    const descriptors = ["localStorage", "sessionStorage", "indexedDB"].map(
      (name) => [name, Object.getOwnPropertyDescriptor(globalThis, name)] as const
    )
    for (const name of ["localStorage", "sessionStorage", "indexedDB"])
      Object.defineProperty(globalThis, name, {
        configurable: true,
        get: () => (name == "localStorage" ? watch(name, realStorage) : (touched.push(name), undefined)),
      })
    Object.defineProperty(document, "cookie", {
      configurable: true,
      get: () => (touched.push("cookie"), ""),
      set: () => touched.push("cookie="),
    })
    try {
      start()
      advance(1000)
      history.pushState(null, "", "/pricing")
      advance(4000)
      hide()
      await settle()
    } finally {
      for (const [name, descriptor] of descriptors)
        if (descriptor) Object.defineProperty(globalThis, name, descriptor)
        else delete (globalThis as Record<string, unknown>)[name]
      delete (document as unknown as Record<string, unknown>).cookie
    }
    expect(names()).toEqual(["pageview /", "engagement /", "pageview /pricing", "engagement /pricing"])
    expect(touched).toEqual(["localStorage.getItem"])
    expect(realStorage.length).toBe(0)
  })

  test("sends the page, referrer and width as text/plain without credentials", async () => {
    start()
    await settle()
    expect(sent).toHaveLength(1)
    const [first] = sent
    expect(first?.url).toBe(endpoint)
    expect(first?.init).toMatchObject({ method: "POST", keepalive: true, credentials: "omit" })
    expect(first?.init?.headers).toBeUndefined()
    expect(typeof first?.init?.body).toBe("string")
    expect(first?.body).toEqual({ n: "pageview", u: "https://site.test/", r: "", w: innerWidth })
  })

  for (const [name, value] of [
    ["globalPrivacyControl", true],
    ["doNotTrack", "1"],
    ["webdriver", true],
  ] as const)
    test(`${name} sends nothing`, async () => {
      signal(name, value)
      const analytics = start()
      analytics.pageview()
      history.pushState(null, "", "/next")
      analytics.consent({ analytics: true })
      analytics.optIn()
      history.pushState(null, "", "/again")
      advance(2000)
      hide()
      await settle()
      expect(sent).toEqual([])
    })
})

describe("consent mode", () => {
  test("sends nothing before consent and stops when it is withdrawn", async () => {
    const analytics = start({ consent: "required" })
    history.pushState(null, "", "/a")
    analytics.pageview()
    advance(3000)
    hide()
    show()
    await settle()
    expect(sent).toEqual([])

    analytics.consent({ analytics: true })
    analytics.consent({ analytics: true })
    await settle()
    expect(names()).toEqual(["pageview /a"])
    expect(events()[0]?.c).toEqual({ a: 1 })

    analytics.consent(false)
    history.pushState(null, "", "/b")
    advance(3000)
    hide()
    await settle()
    expect(names()).toEqual(["pageview /a"])
  })
})

describe("navigation", () => {
  test("one pageview per route; replaceState and query changes do not count", async () => {
    start()
    history.pushState(null, "", "/a")
    history.pushState(null, "", "/a?tab=2")
    history.replaceState(null, "", "/c")
    history.pushState(null, "", "/b")
    history.pushState(null, "", "/b#section")
    history.back = () => {}
    ;(window as unknown as { happyDOM: { setURL(url: string): void } }).happyDOM.setURL("https://site.test/a")
    window.dispatchEvent(new PopStateEvent("popstate"))
    await settle()
    expect(names().filter((name) => name.startsWith("pageview"))).toEqual([
      "pageview /",
      "pageview /a",
      "pageview /b",
      "pageview /a",
    ])
    expect(events().filter((event) => event.n == "pageview" && event.r !== undefined)).toHaveLength(1)
  })

  test("data-hash counts fragments as routes", async () => {
    start({ hash: true })
    location.hash = "#/inbox"
    window.dispatchEvent(new HashChangeEvent("hashchange"))
    history.pushState(null, "", "/#/settings")
    await settle()
    expect(names()).toEqual(["pageview /", "pageview /#/inbox", "pageview /#/settings"])
    expect(events().every((event) => event.h === true)).toBe(true)
  })

  test("manual mode sends only the pageviews the host asks for", async () => {
    const analytics = start({ manual: true })
    history.pushState(null, "", "/a")
    await settle()
    expect(sent).toEqual([])
    analytics.pageview({ url: "https://site.test/virtual" })
    await settle()
    expect(names()).toEqual(["pageview /virtual"])
  })
})

describe("sending", () => {
  test("batches the engagement with the next pageview and beacons on hide", async () => {
    start()
    await settle()
    advance(5000)
    history.pushState(null, "", "/next")
    await settle()
    expect(sent[1]?.via).toBe("fetch")
    expect(sent[1]?.body).toMatchObject({
      e: [
        { n: "engagement", u: "https://site.test/", ms: 5000 },
        { n: "pageview", u: "https://site.test/next" },
      ],
    })

    advance(2500)
    hide()
    expect(sent[2]).toMatchObject({ via: "beacon", url: endpoint, body: { n: "engagement", ms: 2500 } })
    window.dispatchEvent(new Event("pagehide"))
    expect(sent).toHaveLength(3)

    show()
    advance(1000)
    beacon = false
    window.dispatchEvent(new Event("pagehide"))
    expect(sent[3]).toMatchObject({ via: "fetch", body: { n: "engagement", ms: 1000 } })
    expect(sent[3]?.init?.keepalive).toBe(true)
  })

  test("caps engagement at 30 minutes and does not count unfocused time", async () => {
    start()
    document.hasFocus = () => false
    window.dispatchEvent(new Event("blur"))
    advance(60_000)
    document.hasFocus = () => true
    window.dispatchEvent(new Event("focus"))
    advance(1000)
    hide()
    show()
    advance(3_600_000)
    hide()
    await settle()
    expect(
      events()
        .filter((event) => event.n == "engagement")
        .map((event) => event.ms)
    ).toEqual([1000, 1_800_000])
  })

  test("stops after a 403", async () => {
    status = 403
    const analytics = start()
    await settle()
    expect(sent).toHaveLength(1)
    history.pushState(null, "", "/a")
    analytics.optIn()
    analytics.pageview()
    advance(3000)
    hide()
    await settle()
    expect(sent).toHaveLength(1)
  })
})

describe("opt-out", () => {
  test("links store the objection and the next page load sends nothing", async () => {
    document.body.innerHTML = '<a href="#yuva-optout">Do not count me</a><a href="#yuva-optin">Count me</a>'
    const [out, back] = document.querySelectorAll("a")
    start()
    await settle()
    out!.click()
    expect(realStorage.getItem(OPT_OUT_KEY)).toBe("1")
    expect(location.hash).toBe("")
    history.pushState(null, "", "/a")
    instance?.destroy()

    start()
    history.pushState(null, "", "/b")
    await settle()
    expect(names()).toEqual(["pageview /"])

    back!.click()
    expect(realStorage.getItem(OPT_OUT_KEY)).toBeNull()
    history.pushState(null, "", "/c")
    await settle()
    expect(names()).toEqual(["pageview /", "pageview /c"])
    document.body.replaceChildren()
  })
})

describe("events", () => {
  test("track sends props and revenue on the current page", async () => {
    const analytics = start()
    analytics.track("Signup", { props: { plan: "pro", seats: 3 }, revenue: { amount: "29.90", currency: "EUR" } })
    analytics.notFound()
    await settle()
    expect(events().slice(1)).toEqual([
      { n: "Signup", u: "https://site.test/", p: { plan: "pro", seats: 3 }, $: { a: "29.90", c: "EUR" } },
      { n: "not_found", u: "https://site.test/" },
    ])
  })

  test("outbound, file and chat events only with their options", async () => {
    document.body.innerHTML =
      '<a href="https://other.test/x">Other</a><a href="/files/report.PDF?v=2">Report</a><a href="/about">About</a><a href="mailto:a@other.test">Mail</a><yuva-chat></yuva-chat>'
    const [other, file, about, mail] = document.querySelectorAll("a")
    const chat = document.querySelector("yuva-chat")!
    const prevent = (event: Event) => event.preventDefault()
    document.addEventListener("click", prevent)
    const interact = async () => {
      for (const link of [other, file, about, mail]) link!.click()
      chat.dispatchEvent(new Event("yuva-open", { bubbles: true, composed: true }))
      await settle()
    }
    try {
      start()
      await interact()
      expect(names()).toEqual(["pageview /"])
      instance?.destroy()

      sent = []
      start({ outbound: true, files: true, chat: true })
      await interact()
      expect(events().map(({ n, p }) => [n, p?.url])).toEqual([
        ["pageview", undefined],
        ["outbound_click", "https://other.test/x"],
        ["file_download", "https://site.test/files/report.PDF?v=2"],
        ["chat_open", undefined],
      ])
    } finally {
      document.removeEventListener("click", prevent)
      document.body.replaceChildren()
    }
  })

  test("consent mode and opt-out hold back custom events", async () => {
    const analytics = start({ consent: "required", chat: true })
    analytics.track("Signup")
    document.dispatchEvent(new Event("yuva-open", { bubbles: true }))
    await settle()
    expect(sent).toEqual([])
    analytics.consent({ analytics: true })
    await settle()
    analytics.optOut()
    analytics.track("Signup")
    analytics.notFound()
    await settle()
    expect(names()).toEqual(["pageview /"])
  })
})

describe("/yuva-analytics.js", () => {
  test("reads its attributes, drains yuvaq and keeps an existing window.yuva", async () => {
    const script = document.createElement("script")
    script.src = "https://yuva.test/yuva-analytics.js"
    script.dataset.key = "yuva_ak_script"
    script.dataset.api = "https://site.test/stats"
    script.dataset.manual = ""
    Object.defineProperty(document, "currentScript", { value: script, configurable: true })
    const host = window as unknown as { yuva?: Record<string, unknown>; yuvaq?: unknown }
    const open = () => {}
    host.yuva = { open }
    host.yuvaq = [["pageview", { url: "https://site.test/queued" }]]
    await import("../src/yuva-analytics")
    await settle()
    expect(sent.map(({ url }) => url)).toEqual(["https://site.test/stats/client/v1/analytics/yuva_ak_script"])
    expect(names()).toEqual(["pageview /queued"])
    expect(host.yuva.open).toBe(open)
    ;(host.yuvaq as { push(call: unknown[]): void }).push(["pageview"])
    await settle()
    ;(host.yuva.optOut as () => void)()
    ;(host.yuva.pageview as () => void)()
    await settle()
    expect(names()).toEqual(["pageview /queued", "pageview /"])
    delete (document as unknown as Record<string, unknown>).currentScript
  })
})

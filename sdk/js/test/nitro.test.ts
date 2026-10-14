import { afterAll, beforeAll, describe, expect, test } from "bun:test"
import { mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { createServer, type Server } from "node:http"
import type { AddressInfo } from "node:net"
import { tmpdir } from "node:os"
import { WritableStream as NativeWritableStream } from "node:stream/web"
import { join } from "node:path"
import { H3 } from "h3"
import { createApp, createRouter, defineEventHandler, toNodeListener } from "h3-v1"
import {
  analyticsPlugin,
  identityHandler,
  webhookHandler,
  type YuvaAdapter,
  type YuvaRuntimeConfig,
} from "../src/nitro/runtime/core"
import { defineYuvaServer, type YuvaContext } from "../src/nitro/runtime/define"
import { IDENTITY_ROUTE, serverModule, serverParts, serverRuntimeConfig, WEBHOOK_ROUTE } from "../src/nitro/setup"
import { signWebhook, type WebhookEvent } from "../src/server"

const identitySecret = "yuva_is_test-secret"
const webhookSecret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"
const decode = (part: string) => JSON.parse(Buffer.from(part, "base64url").toString("utf8"))
const realFetch = globalThis.fetch

type TestEvent = { headers?: unknown; req?: Request; context: Record<string, unknown> }
const header = (event: TestEvent, name: string) =>
  (event.headers instanceof Headers ? event.headers : event.req?.headers)?.get(name)

const seen: { payload: WebhookEvent; path: string | undefined }[] = []
const server = defineYuvaServer<TestEvent>({
  user: (event) => (header(event, "authorization") === "Session ada" ? { subject: "user-42", name: "Ada" } : null),
  webhooks: {
    "conversation.created": (payload, event) => {
      seen.push({ payload, path: header(event, "x-path") ?? undefined })
    },
    "conversation.rated": () => new Response("rated", { status: 200 }),
  },
})

const runtime: { config: YuvaRuntimeConfig } = { config: {} }
const globals = globalThis as { __yuvaServer?: unknown; __yuvaRuntime?: unknown }
globals.__yuvaServer = server
globals.__yuvaRuntime = runtime

const dir = mkdtempSync(join(tmpdir(), "yuva-nitro-"))
writeFileSync(join(dir, "yuva.mjs"), "export default globalThis.__yuvaServer\n")
writeFileSync(
  join(dir, "runtime-config.mjs"),
  "export const useRuntimeConfig = () => globalThis.__yuvaRuntime.config\n"
)

async function adapter(nitro: 2 | 3): Promise<YuvaAdapter> {
  const path = join(dir, `server-v${nitro}.mjs`)
  writeFileSync(
    path,
    serverModule({
      nitro,
      server: join(dir, "yuva.mjs"),
      runtimeConfig: join(dir, "runtime-config.mjs"),
      h3: Bun.resolveSync("h3-v1", import.meta.dir),
    })
  )
  return (await import(path)) as YuvaAdapter
}

function hookable() {
  const hooks = new Map<string, ((...args: never[]) => unknown)[]>()
  return {
    hook(name: string, fn: (...args: never[]) => unknown) {
      hooks.set(name, [...(hooks.get(name) ?? []), fn])
    },
    async callHook(name: string, ...args: unknown[]) {
      for (const fn of hooks.get(name) ?? []) await (fn as (...args: unknown[]) => unknown)(...args)
    },
  }
}

interface App {
  fetch(request: Request, waitUntil?: (promise: Promise<unknown>) => void): Promise<Response>
  close(): Promise<void>
}

type Track = (event: TestEvent) => void
const signup: Track = (event) => (event.context.yuva as YuvaContext | undefined)?.track({ name: "Signup" })

async function nitro3(yuva: YuvaAdapter): Promise<App> {
  const hooks = hookable()
  analyticsPlugin(yuva)({ hooks })
  const app = new H3({
    onRequest: (event) => hooks.callHook("request", event),
    onResponse: (response, event) => hooks.callHook("response", response, event),
  })
    .get(IDENTITY_ROUTE, identityHandler(yuva) as (event: unknown) => Promise<Response>)
    .post(WEBHOOK_ROUTE, webhookHandler(yuva) as (event: unknown) => Promise<Response>)
    .post("/signup", (event) => {
      signup(event as unknown as TestEvent)
      return "ok"
    })
  return {
    fetch(request, waitUntil) {
      if (waitUntil) Object.defineProperty(request, "waitUntil", { value: waitUntil })
      return Promise.resolve(app.fetch(request))
    },
    close: () => hooks.callHook("close"),
  }
}

async function nitro2(yuva: YuvaAdapter): Promise<App> {
  const hooks = hookable()
  analyticsPlugin(yuva)({ hooks })
  let waitUntil: ((promise: Promise<unknown>) => void) | undefined
  const app = createApp({
    onRequest: async (event) => {
      event.context.waitUntil = waitUntil
      Object.assign(event, { waitUntil: (promise: Promise<unknown>) => event.context.waitUntil?.(promise) })
      await hooks.callHook("request", event)
    },
    onAfterResponse: (event) => hooks.callHook("afterResponse", event),
  })
  const router = createRouter()
    .get(IDENTITY_ROUTE, identityHandler(yuva) as ReturnType<typeof defineEventHandler>)
    .post(WEBHOOK_ROUTE, webhookHandler(yuva) as ReturnType<typeof defineEventHandler>)
    .post(
      "/signup",
      defineEventHandler((event) => {
        signup(event as unknown as TestEvent)
        return "ok"
      })
    )
  app.use(router)
  const listener: Server = createServer(toNodeListener(app))
  await new Promise<void>((resolve) => listener.listen(0, "127.0.0.1", resolve))
  const base = `http://127.0.0.1:${(listener.address() as AddressInfo).port}`
  return {
    async fetch(request, until) {
      waitUntil = until
      const url = new URL(request.url)
      const body = request.method === "GET" ? undefined : await request.arrayBuffer()
      return Bun.fetch(`${base}${url.pathname}`, { method: request.method, headers: request.headers, body })
    },
    async close() {
      await hooks.callHook("close")
      await new Promise((resolve) => listener.close(resolve))
    },
  }
}

const sent: { url: string; body: { events: Record<string, unknown>[] } }[] = []

// happy-dom replaces WritableStream, and h3 v1 pipes web responses into one on the node server.
const domWritableStream = globalThis.WritableStream

beforeAll(() => {
  globalThis.WritableStream = NativeWritableStream as typeof WritableStream
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input instanceof Request ? input.url : input)
    if (!url.startsWith("https://yuva.test/")) return realFetch(input, init)
    sent.push({ url, body: JSON.parse(init!.body as string) })
    return new Response(`{"accepted": 1, "errors": []}`, { status: 202 })
  }) as typeof fetch
})

afterAll(() => {
  globalThis.WritableStream = domWritableStream
  globalThis.fetch = realFetch
  rmSync(dir, { recursive: true, force: true })
})

async function signed(body: string, secret = webhookSecret) {
  const now = new Date()
  return new Request(`http://app.test${WEBHOOK_ROUTE}`, {
    method: "POST",
    headers: {
      "x-path": WEBHOOK_ROUTE,
      "webhook-id": "msg_nitro",
      "webhook-timestamp": String(Math.floor(now.getTime() / 1000)),
      "webhook-signature": await signWebhook(secret, "msg_nitro", now, body),
    },
    body,
  })
}

const configured: YuvaRuntimeConfig = {
  yuva: { identitySecret, webhookSecret, analytics: { apiKey: "yuva_key", propertyId: "prop-1" } },
  public: { yuva: { server: "https://yuva.test" } },
}

for (const [major, start] of [
  [2, nitro2],
  [3, nitro3],
] as const) {
  describe(`nitro v${major}`, () => {
    let app: App

    beforeAll(async () => {
      runtime.config = configured
      app = await start(await adapter(major))
    })

    afterAll(() => app.close())

    test("identity answers a fresh token for the signed-in user", async () => {
      runtime.config = configured
      const res = await app.fetch(
        new Request(`http://app.test${IDENTITY_ROUTE}`, { headers: { authorization: "Session ada" } })
      )
      expect(res.status).toBe(200)
      expect(res.headers.get("cache-control")).toBe("no-store")
      const claims = decode(((await res.json()) as { token: string }).token.split(".")[1]!)
      expect(claims.sub).toBe("user-42")
      expect(claims.name).toBe("Ada")
    })

    test("identity answers 401 without a user and 404 without a secret", async () => {
      runtime.config = configured
      const anonymous = await app.fetch(new Request(`http://app.test${IDENTITY_ROUTE}`))
      expect(anonymous.status).toBe(401)
      runtime.config = { yuva: { identitySecret: "" } }
      const off = await app.fetch(
        new Request(`http://app.test${IDENTITY_ROUTE}`, { headers: { authorization: "Session ada" } })
      )
      expect(off.status).toBe(404)
    })

    test("webhook verifies the raw body and dispatches by type", async () => {
      runtime.config = configured
      seen.length = 0
      const body = `{"type":"conversation.created","data":{"id":"c-${major}"}}`
      expect((await app.fetch(await signed(body))).status).toBe(204)
      expect(seen).toEqual([{ payload: JSON.parse(body), path: WEBHOOK_ROUTE }])
      const rated = await app.fetch(await signed(`{"type":"conversation.rated","data":{}}`))
      expect(rated.status).toBe(200)
      expect(await rated.text()).toBe("rated")
      expect((await app.fetch(await signed(`{"type":"message.created","data":{}}`))).status).toBe(204)
      const forged = await app.fetch(await signed(body, "whsec_Ceo5qEr07ixe2NLpvHk3FH9bwy/WavXrAFQ/9tdO6mc="))
      expect(forged.status).toBe(401)
      expect(seen).toHaveLength(1)
      runtime.config = {}
      expect((await app.fetch(await signed(body))).status).toBe(404)
    })

    test("analytics flushes through waitUntil after a tracked request", async () => {
      sent.length = 0
      const waiting: Promise<unknown>[] = []
      const res = await app.fetch(
        new Request("http://app.test/signup", {
          method: "POST",
          headers: { "user-agent": "test-agent", "accept-language": "tr" },
        }),
        (promise) => waiting.push(promise)
      )
      expect(await res.text()).toBe("ok")
      for (let i = 0; i < 100 && waiting.length === 0; i++) await Bun.sleep(5)
      await Promise.all(waiting)
      expect(waiting).toHaveLength(1)
      expect(sent).toHaveLength(1)
      expect(sent[0]!.url).toBe("https://yuva.test/v1/analytics/properties/prop-1/events")
      expect(sent[0]!.body.events[0]).toMatchObject({
        name: "Signup",
        user_agent: "test-agent",
        accept_language: "tr",
      })
      expect(String(sent[0]!.body.events[0]!.url)).toEndWith("/signup")
    })

    test("analytics leaves the queue to close without waitUntil", async () => {
      sent.length = 0
      await app.fetch(new Request("http://app.test/signup", { method: "POST" }))
      await Bun.sleep(10)
      expect(sent).toHaveLength(0)
      await app.close()
      expect(sent).toHaveLength(1)
      app = await start(await adapter(major))
    })
  })
}

describe("build-time setup", () => {
  test("a part is on when its secret is in the config or the build environment", () => {
    const config = serverRuntimeConfig(undefined, { identitySecret: "yuva_is_x" })
    expect(serverParts(config, {}, "NUXT_")).toEqual({ identity: true, webhook: false, analytics: false })
    expect(
      serverParts(
        config,
        {
          NUXT_YUVA_WEBHOOK_SECRET: "whsec_x",
          NUXT_YUVA_ANALYTICS_API_KEY: "k",
          NUXT_YUVA_ANALYTICS_PROPERTY_ID: "p",
        },
        "NUXT_"
      )
    ).toEqual({ identity: true, webhook: true, analytics: true })
    expect(serverParts(serverRuntimeConfig(undefined, {}), { NUXT_YUVA_IDENTITY_SECRET: "x" }, "NITRO_")).toEqual({
      identity: false,
      webhook: false,
      analytics: false,
    })
  })

  test("secrets from the build environment are not written into the config", () => {
    const config = serverRuntimeConfig({ webhookSecret: "whsec_config" }, { identitySecret: "yuva_is_option" })
    expect(config).toEqual({
      server: "",
      identitySecret: "yuva_is_option",
      webhookSecret: "whsec_config",
      analytics: { apiKey: "", propertyId: "" },
    })
  })
})

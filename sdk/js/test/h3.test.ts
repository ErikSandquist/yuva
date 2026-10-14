import { describe, expect, test } from "bun:test"
import { H3 } from "h3"
import { defineYuvaAnalytics, defineYuvaIdentityHandler, defineYuvaWebhookHandler } from "../src/h3"
import { createAnalytics, signWebhook, type WebhookEvent } from "../src/server"

const decode = (part: string) => JSON.parse(Buffer.from(part, "base64url").toString("utf8"))

describe("defineYuvaIdentityHandler", () => {
  const app = new H3().get(
    "/yuva-token",
    defineYuvaIdentityHandler({
      secret: "yuva_is_test-secret",
      ttl: 120,
      user: async (event) =>
        event.req.headers.get("authorization") === "Session ada" ? { subject: "user-42", name: "Ada" } : null,
    })
  )

  test("answers with a fresh token for the signed-in user", async () => {
    const res = await app.fetch(
      new Request("http://app.test/yuva-token", { headers: { authorization: "Session ada" } })
    )
    expect(res.status).toBe(200)
    expect(res.headers.get("content-type")).toStartWith("application/json")
    expect(res.headers.get("cache-control")).toBe("no-store")
    const claims = decode(((await res.json()) as { token: string }).token.split(".")[1]!)
    expect(claims.sub).toBe("user-42")
    expect(claims.name).toBe("Ada")
    expect(claims.exp - claims.iat).toBe(120)
  })

  test("401 without a user", async () => {
    const res = await app.fetch(new Request("http://app.test/yuva-token"))
    expect(res.status).toBe(401)
    expect(res.headers.get("cache-control")).toBe("no-store")
  })
})

describe("defineYuvaWebhookHandler", () => {
  const secret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"

  async function signed(body: string, sign = secret) {
    const now = new Date()
    return new Request("http://app.test/webhooks/yuva", {
      method: "POST",
      headers: {
        "webhook-id": "msg_h3",
        "webhook-timestamp": String(Math.floor(now.getTime() / 1000)),
        "webhook-signature": await signWebhook(sign, "msg_h3", now, body),
      },
      body,
    })
  }

  function setup() {
    const seen: WebhookEvent[] = []
    const app = new H3().post(
      "/webhooks/yuva",
      defineYuvaWebhookHandler({
        secret,
        on: {
          "conversation.created": (payload, event) => {
            seen.push(payload)
            expect(event.url.pathname).toBe("/webhooks/yuva")
          },
          "conversation.rated": () => new Response("rated", { status: 200 }),
        },
      })
    )
    return { app, seen }
  }

  test("dispatches by event type", async () => {
    const { app, seen } = setup()
    const body = `{"type":"conversation.created","data":{"id":"c-1"}}`
    const res = await app.fetch(await signed(body))
    expect(res.status).toBe(204)
    expect(seen).toEqual([JSON.parse(body)])
  })

  test("answers with the handler's response", async () => {
    const { app } = setup()
    const res = await app.fetch(await signed(`{"type":"conversation.rated","data":{}}`))
    expect(res.status).toBe(200)
    expect(await res.text()).toBe("rated")
  })

  test("204 for types without a handler", async () => {
    const { app, seen } = setup()
    for (const type of ["message.created", "constructor", "toString"]) {
      const res = await app.fetch(await signed(`{"type":"${type}","data":{}}`))
      expect(res.status).toBe(204)
    }
    expect(seen).toEqual([])
  })

  test("401 on a bad signature", async () => {
    const { app, seen } = setup()
    const res = await app.fetch(
      await signed(`{"type":"conversation.created","data":{}}`, "whsec_Ceo5qEr07ixe2NLpvHk3FH9bwy/WavXrAFQ/9tdO6mc=")
    )
    expect(res.status).toBe(401)
    const unsigned = await app.fetch(new Request("http://app.test/webhooks/yuva", { method: "POST", body: "{}" }))
    expect(unsigned.status).toBe(401)
    expect(seen).toEqual([])
  })
})

describe("defineYuvaAnalytics", () => {
  function setup() {
    const sent: { events: Record<string, unknown>[] }[] = []
    const analytics = createAnalytics({
      server: "https://yuva.test",
      apiKey: "yuva_key",
      propertyId: "prop-1",
      flushInterval: 3_600_000,
      fetch: (async (_url: string, init: RequestInit) => {
        sent.push(JSON.parse(init.body as string))
        return new Response(`{"accepted": 1, "errors": []}`, { status: 202 })
      }) as typeof fetch,
    })
    const app = new H3()
      .use(defineYuvaAnalytics({ analytics }))
      .post("/signup", (event) => {
        event.context.yuva?.track({ name: "Signup", props: { plan: "pro" } })
        return "ok"
      })
      .get("/", () => "home")
    return { app, analytics, sent }
  }

  test("tracks from the handler and flushes through waitUntil", async () => {
    const { app, sent } = setup()
    const waiting: Promise<unknown>[] = []
    const request = new Request("http://app.test/signup", {
      method: "POST",
      headers: { "user-agent": "test-agent", "accept-language": "tr" },
    })
    Object.defineProperty(request, "waitUntil", { value: (promise: Promise<unknown>) => waiting.push(promise) })
    const res = await app.fetch(request)
    expect(await res.text()).toBe("ok")
    expect(waiting).toHaveLength(1)
    expect(await waiting[0]).toBeUndefined()
    expect(sent).toHaveLength(1)
    expect(sent[0]!.events[0]).toMatchObject({
      name: "Signup",
      url: "http://app.test/signup",
      props: { plan: "pro" },
      user_agent: "test-agent",
      accept_language: "tr",
    })
  })

  test("leaves the queue to the timer without waitUntil or events", async () => {
    const { app, analytics, sent } = setup()
    const waiting: Promise<unknown>[] = []
    const home = new Request("http://app.test/")
    Object.defineProperty(home, "waitUntil", { value: (promise: Promise<unknown>) => waiting.push(promise) })
    await app.fetch(home)
    await app.fetch(new Request("http://app.test/signup", { method: "POST" }))
    expect(waiting).toHaveLength(0)
    expect(sent).toHaveLength(0)
    await analytics.close()
    expect(sent).toHaveLength(1)
  })
})

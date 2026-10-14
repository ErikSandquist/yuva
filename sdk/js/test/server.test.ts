import { describe, expect, test } from "bun:test"
import {
  AnalyticsBatchError,
  AnalyticsEventError,
  createAnalytics,
  DEFAULT_WEBHOOK_TOLERANCE,
  MAX_BATCH,
  signIdentityToken,
  signWebhook,
  verifyWebhook,
  WebhookVerificationError,
  type ServerAnalyticsConfig,
  type WebhookErrorCode,
} from "../src/server"

const decode = (part: string) => JSON.parse(Buffer.from(part, "base64url").toString("utf8"))

describe("signIdentityToken", () => {
  const secret = "yuva_is_test-secret"
  const at = new Date(1_800_000_000_000)

  test("signs the same tokens as sdk/go/identity", async () => {
    expect(
      await signIdentityToken(secret, {
        subject: "user-42",
        email: "success@simulator.amazonses.com",
        emailVerified: true,
        name: "Ada",
        id: "t-1",
        locale: "tr",
        attrs: { plan: "pro" },
        issuedAt: at,
      })
    ).toBe(
      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyLTQyIiwiZW1haWwiOiJzdWNjZXNzQHNpbXVsYXRvci5hbWF6b25zZXMuY29tIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsIm5hbWUiOiJBZGEiLCJsb2NhbGUiOiJ0ciIsImF0dHJzIjp7InBsYW4iOiJwcm8ifSwianRpIjoidC0xIiwiaWF0IjoxODAwMDAwMDAwLCJleHAiOjE4MDAwMDAzMDB9.sTvDhWhmKXUvNAT5puC4wwz62mYjWt4Kk3dU3tnYrbQ"
    )
    expect(
      await signIdentityToken(secret, {
        subject: " Ayşe Çelik ",
        name: "<Ayşe & Co>",
        issuedAt: at,
        ttl: 600,
        attrs: { z: 1, "10": true, "2": ["x", { b: 1.5, a: null }], a: { y: "é", x: "\u2028" } },
      })
    ).toBe(
      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJBecWfZSDDh2VsaWsiLCJuYW1lIjoiXHUwMDNjQXnFn2UgXHUwMDI2IENvXHUwMDNlIiwiYXR0cnMiOnsiMTAiOnRydWUsIjIiOlsieCIseyJhIjpudWxsLCJiIjoxLjV9XSwiYSI6eyJ4IjoiXHUyMDI4IiwieSI6IsOpIn0sInoiOjF9LCJpYXQiOjE4MDAwMDAwMDAsImV4cCI6MTgwMDAwMDYwMH0.b2cyrnDWWH4JuiftSyGmX9-HsiBRJu9VszldY6BYxXs"
    )
  })

  test("defaults and limits", async () => {
    const before = Math.floor(Date.now() / 1000)
    const claims = decode((await signIdentityToken("s", { subject: "u" })).split(".")[1]!)
    expect(claims.iat).toBeGreaterThanOrEqual(before)
    expect(claims.exp).toBe(claims.iat + 300)
    expect(Object.keys(claims).sort()).toEqual(["exp", "iat", "sub"])
    for (const [secret, c] of [
      ["", { subject: "u" }],
      ["s", { subject: " " }],
      ["s", { subject: "x".repeat(201) }],
      ["s", { subject: "u", ttl: 660 }],
      ["s", { subject: "u", ttl: -60 }],
    ] as const) {
      await expect(signIdentityToken(secret, c)).rejects.toThrow()
    }
  })
})

describe("verifyWebhook", () => {
  const vectorSecret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"
  const vectorID = "msg_p5jXN8AQM9LWM0D4loKWxJek"
  const vectorTimestamp = 1614265330
  const vectorBody = `{"test": 2432232314}`
  const vectorSignature = "v1,g0hM9SsE+OTPJTGt/tmIKtSyZlE3uFJELVlNIOLJ1OE="
  const at = new Date(vectorTimestamp * 1000)
  const wrong = "v1,Ceo5qEr07ixe2NLpvHk3FH9bwy/WavXrAFQ/9tdO6mc="

  const request = (headers: Record<string, string>, body = vectorBody) =>
    new Request("https://host.test/webhooks/yuva", { method: "POST", headers, body })
  const valid = {
    "webhook-id": vectorID,
    "webhook-timestamp": String(vectorTimestamp),
    "webhook-signature": vectorSignature,
  }
  const code = (promise: Promise<unknown>) =>
    promise.then(
      () => undefined,
      (error: unknown) => (error instanceof WebhookVerificationError ? error.code : error)
    )

  test("signs the Standard Webhooks vector", async () => {
    expect(await signWebhook(vectorSecret, vectorID, at, vectorBody)).toBe(vectorSignature)
    expect(await signWebhook(vectorSecret.slice("whsec_".length), vectorID, at, vectorBody)).toBe(vectorSignature)
  })

  test("verifies and returns the event", async () => {
    expect(await verifyWebhook(vectorSecret, request(valid), { now: at })).toEqual({ test: 2432232314 } as never)
    expect(
      await code(
        verifyWebhook(vectorSecret, request(valid), { now: new Date(at.getTime() + DEFAULT_WEBHOOK_TOLERANCE * 1000) })
      )
    ).toBeUndefined()
  })

  test("refuses what sdk/go/webhook refuses", async () => {
    const cases: [string, Record<string, string>, string, number, number | undefined, WebhookErrorCode][] = [
      ["missing id", { ...valid, "webhook-id": "" }, vectorBody, 0, undefined, "missing_headers"],
      [
        "missing timestamp",
        { "webhook-id": vectorID, "webhook-signature": vectorSignature },
        vectorBody,
        0,
        undefined,
        "missing_headers",
      ],
      ["missing signature", { ...valid, "webhook-signature": "" }, vectorBody, 0, undefined, "missing_headers"],
      ["bad timestamp", { ...valid, "webhook-timestamp": "soon" }, vectorBody, 0, undefined, "invalid_timestamp"],
      ["too old", valid, vectorBody, (DEFAULT_WEBHOOK_TOLERANCE + 1) * 1000, undefined, "timestamp_too_old"],
      ["too new", valid, vectorBody, -(DEFAULT_WEBHOOK_TOLERANCE + 1) * 1000, undefined, "timestamp_too_new"],
      ["too old for a short tolerance", valid, vectorBody, 61_000, 60, "timestamp_too_old"],
      ["wrong signature", { ...valid, "webhook-signature": wrong }, vectorBody, 0, undefined, "invalid_signature"],
      ["partial signature", { ...valid, "webhook-signature": "v1," }, vectorBody, 0, undefined, "invalid_signature"],
      [
        "other version",
        { ...valid, "webhook-signature": vectorSignature.replace("v1,", "v2,") },
        vectorBody,
        0,
        undefined,
        "invalid_signature",
      ],
      ["tampered body", valid, `{"test": 2432232315}`, 0, undefined, "invalid_signature"],
      ["other id", { ...valid, "webhook-id": "msg_other" }, vectorBody, 0, undefined, "invalid_signature"],
    ]
    for (const [name, headers, body, offset, tolerance, want] of cases) {
      const got = await code(
        verifyWebhook(vectorSecret, request(headers, body), { now: new Date(at.getTime() + offset), tolerance })
      )
      expect(`${name}: ${String(got)}`).toBe(`${name}: ${want}`)
    }
    const multi = [wrong, "v1a,abc", vectorSignature, wrong].join(" ")
    expect(
      await code(verifyWebhook(vectorSecret, request({ ...valid, "webhook-signature": multi }), { now: at }))
    ).toBeUndefined()
    expect(await code(verifyWebhook("whsec_", request(valid), { now: at }))).toBe("invalid_secret")
    expect(await code(verifyWebhook("not base64!", request(valid), { now: at }))).toBe("invalid_secret")
  })

  test("verifies a signature made now", async () => {
    const now = new Date()
    const body = `{"type":"message.created"}`
    const signature = await signWebhook(vectorSecret, "msg_now", now, body)
    const event = await verifyWebhook(
      vectorSecret,
      request(
        {
          "webhook-id": "msg_now",
          "webhook-timestamp": String(Math.floor(now.getTime() / 1000)),
          "webhook-signature": signature,
        },
        body
      )
    )
    expect(event.type).toBe("message.created")
  })
})

describe("createAnalytics", () => {
  type Answer = () => Response | Promise<Response>
  interface Recorded {
    url: string
    headers: Headers
    body: { events: Record<string, unknown>[] }
    at: number
  }

  function setup(config: Partial<ServerAnalyticsConfig> = {}, answers: Answer[] = []) {
    const requests: Recorded[] = []
    const errors: Error[] = []
    const analytics = createAnalytics({
      server: "https://yuva.test/",
      apiKey: "yuva_key",
      propertyId: "prop-1",
      flushInterval: 3_600_000,
      minBackoff: 1,
      onError: (error) => errors.push(error),
      fetch: (async (url: string, init: RequestInit) => {
        requests.push({
          url,
          headers: new Headers(init.headers),
          body: JSON.parse(init.body as string),
          at: Date.now(),
        })
        const answer = answers.shift()
        if (answer) return answer()
        return new Response(`{"accepted": 1, "errors": []}`, { status: 202 })
      }) as typeof fetch,
      ...config,
    })
    return { analytics, requests, errors, answers }
  }

  // happy-dom's Request drops the headers browsers may not set (Referer, DNT, Sec-*); servers receive them.
  const visitorRequest = (extra: Record<string, string> = {}) =>
    ({
      url: "https://example.com/signup?utm_source=news&secret=1",
      ip: "203.0.113.7",
      headers: new Headers({
        "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605.1.15",
        "Sec-CH-UA-Platform": `"macOS"`,
        "Sec-CH-UA-Mobile": "?0",
        "Accept-Language": "tr-TR,tr;q=0.9",
        Referer: "https://www.google.com/",
        ...extra,
      }),
    }) as unknown as Request

  test("track forwards the visitor and batches", async () => {
    const { analytics, requests, errors } = setup()
    analytics.track(visitorRequest(), {
      name: "Signup",
      time: new Date("2026-10-12T09:00:00+03:00"),
      props: { plan: "pro" },
      revenue: { amount: "29.90", currency: "EUR" },
    })
    analytics.send({
      name: "Renewal",
      url: "https://example.com/billing",
      visitorKey: "session-42",
      nonInteractive: true,
    })
    expect(await analytics.flush()).toBeUndefined()
    expect(requests).toHaveLength(1)
    expect(requests[0]!.url).toBe("https://yuva.test/v1/analytics/properties/prop-1/events")
    expect(requests[0]!.headers.get("authorization")).toBe("Bearer yuva_key")
    const [first, second] = requests[0]!.body.events
    expect(first).toEqual({
      name: "Signup",
      url: "https://example.com/signup?utm_source=news&secret=1",
      referrer: "https://www.google.com/",
      timestamp: "2026-10-12T06:00:00Z",
      props: { plan: "pro" },
      revenue: { a: "29.90", c: "EUR" },
      ip: "203.0.113.7",
      user_agent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605.1.15",
      sec_ch_ua_platform: `"macOS"`,
      sec_ch_ua_mobile: "?0",
      accept_language: "tr-TR,tr;q=0.9",
    })
    expect(second).toMatchObject({ visitor_key: "session-42", interactive: false })
    expect(second).not.toHaveProperty("ip")
    expect(second).not.toHaveProperty("user_agent")
    expect(errors).toEqual([])
  })

  test("the client IP comes from the option, not from headers", async () => {
    const { analytics, requests } = setup({ clientIP: (request) => request.headers.get("cf-connecting-ip") })
    analytics.track(new Request("https://example.com/", { headers: { "CF-Connecting-IP": "198.51.100.4" } }), {
      name: "Signup",
    })
    const plain = setup()
    plain.analytics.track(new Request("https://example.com/", { headers: { "X-Forwarded-For": "198.51.100.4" } }), {
      name: "Signup",
    })
    await analytics.flush()
    await plain.analytics.flush()
    expect(requests[0]!.body.events[0]!.ip).toBe("198.51.100.4")
    expect(plain.requests[0]!.body.events[0]).not.toHaveProperty("ip")
  })

  test("track sends nothing for DNT or GPC", async () => {
    const { analytics, requests } = setup()
    for (const header of ["DNT", "Sec-GPC"]) analytics.track(visitorRequest({ [header]: "1" }), { name: "Signup" })
    await analytics.flush()
    expect(requests).toHaveLength(0)
  })

  test("full batches go out at once and close sends the rest", async () => {
    const { analytics, requests } = setup()
    for (let i = 0; i < MAX_BATCH; i++) analytics.send({ name: "Tick", url: "https://example.com/", visitorKey: "k" })
    await Bun.sleep(5)
    expect(requests).toHaveLength(1)
    expect(requests[0]!.body.events).toHaveLength(MAX_BATCH)
    analytics.send({ name: "Tick", url: "https://example.com/", visitorKey: "k" })
    expect(await analytics.close()).toBeUndefined()
    expect(requests).toHaveLength(2)
    expect(requests[1]!.body.events).toHaveLength(1)
    expect(() => analytics.send({ name: "Tick", url: "https://example.com/", visitorKey: "k" })).toThrow("closed")
  })

  test("queued events are sent after the flush interval", async () => {
    const { analytics, requests } = setup({ flushInterval: 10 })
    analytics.send({ name: "Tick", url: "https://example.com/", visitorKey: "k" })
    expect(requests).toHaveLength(0)
    await Bun.sleep(40)
    expect(requests).toHaveLength(1)
  })

  test("retries honour Retry-After; refused batches and events are reported", async () => {
    const busy = (retryAfter?: string) => () =>
      new Response(null, { status: 503, headers: retryAfter ? { "Retry-After": retryAfter } : {} })
    const { analytics, requests, errors, answers } = setup({}, [busy("1"), busy()])
    analytics.send({ name: "Signup", url: "https://example.com/", visitorKey: "k" })
    expect(await analytics.flush()).toBeUndefined()
    expect(requests).toHaveLength(3)
    expect(requests[1]!.at - requests[0]!.at).toBeGreaterThanOrEqual(990)
    expect(requests[2]!.at - requests[1]!.at).toBeLessThan(500)

    answers.push(() => new Response(`{"code":"insufficient_scope"}`, { status: 403 }))
    analytics.send({ name: "Signup", url: "https://example.com/", visitorKey: "k" })
    const refused = await analytics.flush()
    expect(refused).toBeInstanceOf(AnalyticsBatchError)
    expect((refused as AnalyticsBatchError).status).toBe(403)
    expect(requests).toHaveLength(4)
    expect(errors).toHaveLength(1)

    answers.push(
      () =>
        new Response(`{"accepted": 1, "errors": [{"index": 1, "detail": "engagement is sent by browsers only"}]}`, {
          status: 202,
        })
    )
    analytics.send({ name: "Signup", url: "https://example.com/", visitorKey: "k" })
    analytics.send({ name: "engagement", url: "https://example.com/", visitorKey: "k" })
    expect(await analytics.flush()).toBeUndefined()
    expect(errors).toHaveLength(2)
    expect(errors[1]).toBeInstanceOf(AnalyticsEventError)
    expect((errors[1] as AnalyticsEventError).event.name).toBe("engagement")
  })

  test("network errors are retried, then reported", async () => {
    const down = () => Promise.reject(new TypeError("fetch failed"))
    const { analytics, requests, errors } = setup({ maxRetries: 2 }, [down, down, down])
    analytics.send({ name: "Signup", url: "https://example.com/", visitorKey: "k" })
    const error = await analytics.flush()
    expect(requests).toHaveLength(3)
    expect(error).toBeInstanceOf(AnalyticsBatchError)
    expect((error as AnalyticsBatchError).cause).toBeInstanceOf(TypeError)
    expect(errors).toEqual([error!])
  })

  test("queue limit", async () => {
    const { analytics, errors } = setup({ queueSize: 1 })
    analytics.send({ name: "A", url: "https://example.com/", visitorKey: "k" })
    analytics.send({ name: "B", url: "https://example.com/", visitorKey: "k" })
    expect(errors.map((error) => error.message)).toEqual(["analytics: the queue is full; event dropped"])
    expect(() => analytics.send({ name: "C" })).toThrow()
  })
})

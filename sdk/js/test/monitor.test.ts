import { afterAll, afterEach, beforeAll, describe, expect, spyOn, test } from "bun:test"
import { GlobalRegistrator } from "@happy-dom/global-registrator"
import { createServer, type Server } from "node:http"
import type { AddressInfo } from "node:net"
import express from "express"
import Fastify from "fastify"
import { H3, HTTPError } from "h3"
import { Hono } from "hono"
import { serve } from "srvx"
import { yuvaMonitor as expressMonitor } from "../src/express"
import { yuvaMonitor as fastifyMonitor } from "../src/fastify"
import { defineYuvaMonitor } from "../src/h3"
import { yuvaMonitor as honoMonitor } from "../src/hono"
import { nextRoute } from "../src/next"
import {
  AnalyticsBatchError,
  createMonitor,
  MAX_RECORD_BATCH,
  MonitorRecordError,
  routePattern,
  type MonitorConfig,
} from "../src/server"
import { yuvaMonitor as srvxMonitor } from "../src/srvx"

type Wire = Record<string, unknown>
type Answer = { status: number; body?: string; headers?: Record<string, string> }

function sink(answer?: (attempt: number) => Answer) {
  const batches: Wire[][] = []
  const posts: { url: string; headers: Record<string, string>; at: number }[] = []
  const fetch: typeof globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    posts.push({ url: String(input), headers: init!.headers as Record<string, string>, at: performance.now() })
    const body = JSON.parse(new TextDecoder().decode(Bun.gunzipSync(init!.body as Uint8Array<ArrayBuffer>)))
    batches.push(body.records)
    const a = answer?.(posts.length) ?? { status: 202, body: `{"accepted": 1, "errors": []}` }
    return new Response(a.body ?? "", { status: a.status, headers: a.headers })
  }) as typeof fetch
  return { fetch, batches, posts, all: () => batches.flat() }
}

const base = { server: "https://yuva.test/", apiKey: "yuva_key", propertyId: "prop-1" }

function monitorWith(s: ReturnType<typeof sink>, config: Partial<MonitorConfig> = {}) {
  const errors: Error[] = []
  const monitor = createMonitor({
    ...base,
    fetch: s.fetch,
    flushInterval: 3_600_000,
    onError: (error) => errors.push(error),
    ...config,
  })
  return { monitor, errors }
}

function expectClean(records: Wire[]) {
  for (const r of records) {
    expect(typeof r.duration_us).toBe("number")
    expect(typeof r.time).toBe("string")
    for (const key of ["query", "headers", "user_agent", "body"]) expect(r).not.toHaveProperty(key)
  }
  expect(JSON.stringify(records)).not.toContain("secret")
}

describe("createMonitor", () => {
  afterEach(() => {
    ;(Math.random as unknown as { mockRestore?: () => void }).mockRestore?.()
  })

  test("sends gzip'd records to the property's requests endpoint", async () => {
    const s = sink()
    const { monitor, errors } = monitorWith(s, { environment: "staging", service: "shop" })
    const time = new Date("2026-10-14T09:00:00.250Z")
    monitor.record({ method: "GET", route: "/users/{id}", status: 200, duration: 1.5, time, responseSize: 11 })
    monitor.record({ method: "GET", path: "/wp-login.php", status: 404, duration: 0.2, requestSize: -1 })
    monitor.send({ method: "POST", route: "/orders", status: 201, duration: 3, ip: "203.0.113.7", consumer: "acme" })
    expect(await monitor.flush()).toBeUndefined()
    expect(s.posts).toHaveLength(1)
    expect(s.posts[0]!.url).toBe("https://yuva.test/v1/analytics/properties/prop-1/requests")
    expect(s.posts[0]!.headers).toMatchObject({ Authorization: "Bearer yuva_key", "Content-Encoding": "gzip" })
    const [a, b, c] = s.all()
    expect(a).toEqual({
      method: "GET",
      route: "/users/{id}",
      status: 200,
      duration_us: 1500,
      response_size: 11,
      time: "2026-10-14T09:00:00.250Z",
      environment: "staging",
      service: "shop",
    })
    expect(b).toMatchObject({ method: "GET", path: "/wp-login.php", status: 404, duration_us: 200 })
    expect(b).not.toHaveProperty("request_size")
    expect(c).toMatchObject({ method: "POST", route: "/orders", ip: "203.0.113.7", consumer: "acme" })
    expect(c).not.toHaveProperty("service")
    expect(errors).toEqual([])
    await monitor.close()
  })

  test("a full batch goes out at once and only successful requests are sampled", async () => {
    const s = sink()
    const { monitor } = monitorWith(s, { sampleRate: 4 })
    let n = 0
    spyOn(Math, "random").mockImplementation(() => (++n % 4 === 0 ? 0 : 0.9))
    for (let i = 0; i < 2000; i++) monitor.record({ method: "GET", route: "/things/{id}", status: 200, duration: 1 })
    for (let i = 0; i < 50 && s.all().length < MAX_RECORD_BATCH; i++) await Bun.sleep(5)
    expect(s.all()).toHaveLength(MAX_RECORD_BATCH)
    for (let i = 0; i < 3; i++) monitor.record({ method: "GET", route: "/things/{id}", status: 502, duration: 1 })
    await monitor.flush()
    const records = s.all()
    expect(records).toHaveLength(503)
    expect(records.filter((r) => r.status === 200).every((r) => r.sample_rate === 4)).toBe(true)
    const failed = records.filter((r) => r.status === 502)
    expect(failed).toHaveLength(3)
    for (const r of failed) expect(r).not.toHaveProperty("sample_rate")
    await monitor.close()
  })

  test("refused records, a full queue and records without a route or path", async () => {
    const s = sink(() => ({
      status: 202,
      body: `{"accepted": 1, "errors": [{"index": 1, "detail": "status must be 100 to 599, or 0 when no response was sent"}]}`,
    }))
    const { monitor, errors } = monitorWith(s, { queueSize: 2 })
    monitor.send({ method: "GET", route: "/a", status: 200, duration: 1 })
    monitor.send({ method: "GET", route: "/b", status: 900, duration: 1 })
    monitor.send({ method: "GET", route: "/c", status: 200, duration: 1 })
    expect(() => monitor.send({ method: "GET", status: 200, duration: 1 })).toThrow()
    await monitor.flush()
    expect(errors).toHaveLength(2)
    expect(errors[0]!.message).toContain("queue is full")
    expect(errors[1]).toBeInstanceOf(MonitorRecordError)
    expect((errors[1] as MonitorRecordError).record.route).toBe("/b")
    await monitor.close()
    expect(() => monitor.send({ method: "GET", route: "/a", status: 200, duration: 1 })).toThrow()
    monitor.record({ method: "GET", route: "/a", status: 200, duration: 1 })
  })

  test("retries 429 and 5xx with backoff, never sooner than Retry-After", async () => {
    const s = sink((attempt) =>
      attempt === 1
        ? { status: 503 }
        : attempt === 2
          ? { status: 429, headers: { "retry-after": "1" } }
          : { status: 202, body: `{"accepted": 1, "errors": []}` }
    )
    const { monitor, errors } = monitorWith(s, { minBackoff: 10 })
    monitor.send({ method: "GET", route: "/a", status: 200, duration: 1 })
    expect(await monitor.flush()).toBeUndefined()
    expect(s.posts).toHaveLength(3)
    expect(s.posts[1]!.at - s.posts[0]!.at).toBeGreaterThanOrEqual(9)
    expect(s.posts[2]!.at - s.posts[1]!.at).toBeGreaterThanOrEqual(990)
    expect(errors).toEqual([])

    const refused = sink(() => ({ status: 400, body: "bad" }))
    const other = monitorWith(refused, { minBackoff: 10 })
    other.monitor.send({ method: "GET", route: "/a", status: 200, duration: 1 })
    const error = await other.monitor.flush()
    expect(error).toBeInstanceOf(AnalyticsBatchError)
    expect((error as AnalyticsBatchError).status).toBe(400)
    expect(refused.posts).toHaveLength(1)
  })

  test("flushed() resolves once the queued records are sent", async () => {
    const s = sink()
    const { monitor } = monitorWith(s, { flushInterval: 20 })
    await monitor.flushed()
    monitor.send({ method: "GET", route: "/a", status: 200, duration: 1 })
    await monitor.flushed()
    expect(s.all()).toHaveLength(1)
    monitor.send({ method: "GET", route: "/a", status: 200, duration: 1 })
    const waiting = monitor.flushed()
    await monitor.flush()
    await waiting
    expect(s.all()).toHaveLength(2)
  })

  test("router patterns take one placeholder syntax", () => {
    for (const [pattern, route] of [
      ["/users/:id", "/users/{id}"],
      ["/users/{id}", "/users/{id}"],
      ["/files/**:path", "/files/{path...}"],
      ["/files/*splat", "/files/{splat...}"],
      ["/static/*", "/static/*"],
      ["/static/**", "/static/*"],
      ["/post/:date{[0-9]+}", "/post/{date}"],
      ["/u/:id(^\\d+)", "/u/{id}"],
      ["/:name.:ext", "/{name}.{ext}"],
      ["/users/:id?", "/users/{id}"],
      ["", "/"],
    ]) {
      expect(routePattern(pattern!)).toBe(route!)
    }
    for (const [segment, route] of [
      ["/users/[id]", "/users/{id}"],
      ["/(shop)/docs/[...slug]", "/docs/{slug...}"],
      ["/[[...all]]", "/{all...}"],
      ["/@modal/photos/[id]", "/photos/{id}"],
      ["/", "/"],
    ]) {
      expect(nextRoute(segment!)).toBe(route!)
    }
  })
})

const client = (request: { headers: { get(name: string): string | null } }) => request.headers.get("x-client")

function expectRecorded(records: Wire[]) {
  expect(records).toHaveLength(4)
  expect(records[0]).toMatchObject({ method: "GET", route: "/users/{id}", status: 200, consumer: "acme" })
  expect(records[1]).toMatchObject({ method: "POST", route: "/orders", status: 201 })
  expect(records[2]).toMatchObject({ method: "GET", path: "/wp-login.php", status: 404 })
  expect(records[2]).not.toHaveProperty("route")
  expect(records[3]).toMatchObject({ method: "GET", route: "/boom", status: 500 })
  expectClean(records)
}

const calls = [
  ["GET", "/users/42?token=secret", { "x-client": "acme" }],
  ["POST", "/orders", {}],
  ["GET", "/wp-login.php?secret=1", {}],
  ["GET", "/boom", {}],
] as const

async function callAll(fetcher: (request: Request) => Promise<Response>, origin: string) {
  for (const [method, path, headers] of calls) {
    const res = await fetcher(
      new Request(`${origin}${path}`, { method, headers, body: method === "POST" ? `{"sku":"A-1"}` : undefined })
    )
    await res.arrayBuffer()
  }
}

function nodeFetch(origin: string) {
  return async (request: Request) => {
    const url = new URL(request.url)
    const headers = Object.fromEntries(request.headers)
    return Bun.fetch(`${origin}${url.pathname}${url.search}`, {
      method: request.method,
      headers: { ...headers, connection: "close" },
      body: request.method === "GET" ? undefined : await request.text(),
    })
  }
}

describe("h3", () => {
  test("records the matched route and flushes through waitUntil", async () => {
    const s = sink()
    const { monitor } = monitorWith(s, { flushInterval: 20 })
    const app = new H3({ silent: true })
      .use(defineYuvaMonitor({ monitor, consumer: (event) => client(event.req) }))
      .get("/users/:id", (event) => ({ id: event.context.params?.id }))
      .post("/orders", (event) => {
        event.res.status = 201
        return "created"
      })
      .get("/boom", () => {
        throw new Error("boom")
      })
      .get("/teapot", () => {
        throw new HTTPError({ status: 418 })
      })
    const waiting: Promise<unknown>[] = []
    await callAll(async (request) => {
      Object.defineProperty(request, "waitUntil", { value: (p: Promise<unknown>) => waiting.push(p) })
      return app.fetch(request)
    }, "http://api.test")
    expect(waiting).toHaveLength(4)
    await Promise.all(waiting)
    expectRecorded(s.all())
    const teapot = await app.fetch(new Request("http://api.test/teapot"))
    expect(teapot.status).toBe(418)
    await monitor.flush()
    expect(s.all()[4]).toMatchObject({ route: "/teapot", status: 418 })
  })
})

describe("srvx", () => {
  // Bun.serve takes only the runtime's own Response, which happy-dom replaces.
  beforeAll(() => GlobalRegistrator.unregister())
  afterAll(() => GlobalRegistrator.register())

  test("takes the matched route from the request context or the host, and flushes when the server closes", async () => {
    const s = sink()
    const { monitor } = monitorWith(s)
    const server = serve({
      port: 0,
      hostname: "127.0.0.1",
      silent: true,
      gracefulShutdown: false,
      plugins: [
        srvxMonitor(monitor, {
          consumer: client,
          route: (request) => (new URL(request.url).pathname === "/orders" ? "/orders" : undefined),
        }),
      ],
      fetch(request) {
        const { pathname } = new URL(request.url)
        if (pathname === "/orders") return new Response("created", { status: 201 })
        if (pathname === "/boom") {
          request.context = { ...request.context, matchedRoute: { route: "/boom" } }
          throw new Error("boom")
        }
        const id = /^\/users\/([^/]+)$/.exec(pathname)?.[1]
        if (!id) return new Response("not found", { status: 404 })
        request.context = { ...request.context, matchedRoute: { route: "/users/:id" } }
        return Response.json({ id })
      },
      error: () => new Response("error", { status: 500 }),
    })
    await server.ready()
    const origin = server.url!.replace(/\/$/, "")
    for (const [method, path, headers] of calls) {
      const res = await Bun.fetch(`${origin}${path}`, { method, headers, body: method === "POST" ? "{}" : undefined })
      expect(res.status).toBe(path.startsWith("/users") ? 200 : path === "/orders" ? 201 : path === "/boom" ? 500 : 404)
      await res.arrayBuffer()
    }
    expect(s.all()).toHaveLength(0)
    await server.close(true)
    expectRecorded(s.all())
  })
})

describe("hono", () => {
  test("records the handler's route and flushes through the execution context", async () => {
    const s = sink()
    const { monitor } = monitorWith(s, { flushInterval: 20 })
    const api = new Hono().get("/items/:itemId", (c) => c.json({ id: c.req.param("itemId") }))
    const app = new Hono()
      .onError((_error, c) => c.text("error", 500))
      .use(honoMonitor(monitor, { consumer: (c) => c.req.header("x-client") }))
      .use("/users/*", async (_c, next) => next())
      .get("/users/:id", (c) => c.json({ id: c.req.param("id") }))
      .post("/orders", (c) => c.text("created", 201))
      .get("/boom", () => {
        throw new Error("boom")
      })
      .route("/api", api)
    const waiting: Promise<unknown>[] = []
    const ctx = { waitUntil: (p: Promise<unknown>) => waiting.push(p), passThroughOnException() {}, props: {} }
    await callAll(async (request) => app.fetch(request, {}, ctx), "http://api.test")
    await (await app.fetch(new Request("http://api.test/api/items/9"), {}, ctx)).arrayBuffer()
    expect(waiting).toHaveLength(5)
    await Promise.all(waiting)
    const records = s.all()
    expectRecorded(records.slice(0, 4))
    expect(records[4]).toMatchObject({ route: "/api/items/{itemId}", status: 200 })
  })
})

function listen(server: Server): Promise<string> {
  return new Promise((resolve) =>
    server.listen(0, "127.0.0.1", () => resolve(`http://127.0.0.1:${(server.address() as AddressInfo).port}`))
  )
}

describe("express", () => {
  test("records req.route under req.baseUrl and flushes when the server closes", async () => {
    const s = sink()
    const { monitor } = monitorWith(s)
    const api = express.Router().get("/items/:itemId", (req, res) => {
      res.json({ id: req.params.itemId })
    })
    const app = express()
      .use(expressMonitor(monitor, { consumer: (req) => req.get("x-client") }))
      .get("/users/:id", (req, res) => {
        res.json({ id: req.params.id })
      })
      .post("/orders", express.json(), (_req, res) => {
        res.status(201).send("created")
      })
      .get("/boom", () => {
        throw new Error("boom")
      })
      .get("/hang-up", (req) => {
        req.socket.destroy()
      })
      .use("/api", api)
      .use(((_error, _req, res, _next) => {
        res.status(500).send("error")
      }) as express.ErrorRequestHandler)
    const server = createServer(app)
    const origin = await listen(server)
    await callAll(nodeFetch(origin), "http://api.test")
    await (await nodeFetch(origin)(new Request("http://api.test/api/items/9"))).arrayBuffer()
    await nodeFetch(origin)(new Request("http://api.test/hang-up")).catch(() => undefined)
    for (let i = 0; i < 50 && s.all().length === 0; i++) await Bun.sleep(5)
    expect(s.all()).toHaveLength(0)
    await new Promise((resolve) => server.close(resolve))
    for (let i = 0; i < 50 && s.all().length < 6; i++) await Bun.sleep(5)
    const records = s.all()
    expectRecorded(records.slice(0, 4))
    expect(records[4]).toMatchObject({ route: "/api/items/{itemId}", status: 200 })
    expect(records[5]).toMatchObject({ route: "/hang-up", status: 0 })
    expect(records[1]).toMatchObject({ request_size: 13 })
  })
})

describe("fastify", () => {
  test("records routeOptions.url across plugins and flushes on close", async () => {
    const s = sink()
    const { monitor } = monitorWith(s)
    const app = Fastify()
    await app.register(fastifyMonitor(monitor, { consumer: (request) => request.headers["x-client"] as string }))
    app.get("/users/:id", async (request) => ({ id: (request.params as { id: string }).id }))
    app.post("/orders", async (_request, reply) => reply.code(201).send("created"))
    app.get("/boom", async () => {
      throw new Error("boom")
    })
    await app.register(
      async (child) => {
        child.get("/items/:itemId", async () => ({ ok: true }))
      },
      { prefix: "/api" }
    )
    for (const [method, url, headers] of calls) {
      await app.inject({ method, url, headers, payload: method === "POST" ? { sku: "A-1" } : undefined })
    }
    await app.inject({ method: "GET", url: "/api/items/9" })
    expect(s.all()).toHaveLength(0)
    await app.close()
    const records = s.all()
    expectRecorded(records.slice(0, 4))
    expect(records[4]).toMatchObject({ route: "/api/items/{itemId}", status: 200 })
  })
})

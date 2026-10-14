# API monitoring in JavaScript

The `useyuva` package reports every request your Node, Bun or Deno API answers, with one middleware and no code per
route. Each framework's router tells it which route matched, so `/users/42` is counted as `/users/{id}`. You need an
API property and a key with the scope `analytics:write`: see
[Make an API property](analytics-api.md#make-an-api-property). Keep the key on the server.

```sh
npm install useyuva
```

Create one monitor and give it to the middleware of your framework. It needs Node 22 or later, Bun, Deno or a worker
runtime, and uses only `fetch`.

```ts
// monitor.ts
import { createMonitor } from "useyuva/server"

export const monitor = createMonitor({
  server: "https://support.example.com",
  apiKey: process.env.YUVA_API_KEY!,
  propertyId: process.env.YUVA_API_PROPERTY!,
  service: "shop-api",
})
```

| Option | Default | |
|---|---|---|
| `server`, `apiKey`, `propertyId` | required | Your Yuva address, the key and the API property's id. |
| `environment` | `production` | Tells apart `production`, `staging` and the like. |
| `service`, `release` | none | The service that served the request and its release. The release is accepted but not shown yet. |
| `sampleRate` | every request | `10` sends one in ten successful requests (status 1 to 399), each standing for ten. Errors, client errors and status `0` are always sent. |
| `flushInterval` | 5000 | Milliseconds between batches. A batch of 500 goes at once. |
| `queueSize` | 50,000 | Records that wait at most; more are dropped and reach `onError`. |
| `maxRetries`, `minBackoff` | 5, 1000 | Retries of a batch after a network error, `429` or `5xx`, and the first delay in milliseconds, doubled on each retry. A longer `Retry-After` wins. |
| `onError` | none | Called with an `Error`: a `MonitorRecordError` for a record Yuva left out as invalid, or the failure of a batch. |
| `fetch` | global `fetch` | Sends the requests. |

Records go out gzip-compressed. Every middleware also takes these functions, each called with the framework's request:

| Option | |
|---|---|
| `consumer(request)` | Who called: an API key's name or a client id. Never a person's address; a value shaped like an e-mail address is stored as `(scrubbed)`. |
| `apiVersion(request)` | The API version the request used. |
| `clientIP(request)` | The caller's address, so Yuva can count the request's country. It is shortened on arrival and never stored. Without it no address is sent. |

A middleware records the method, the matched route, the status (`0` when no response was sent), the duration, the body
sizes from `Content-Length` and the start time. Never a body, a header, a query string or an address (except what
`clientIP` returns). A request no route matched is sent with its path, and Yuva counts a `404` or `405` of that kind
as `(unmatched)`; see [Routes](analytics-api.md#routes).

## One route syntax

Whatever the router writes, the route is stored the same way: `:id` and `[id]` become `{id}`, and a catch-all becomes
`{path...}`. The same API in any framework, or in [Go](analytics-api-go.md) or
[OpenTelemetry](analytics-api-otel.md), shows the same routes.

## When records are sent

- **Serverless platforms** (Cloudflare Workers, Vercel, Lambda and the like): the middleware hands a flush to the
  request's `waitUntil` after each response, so nothing waits for the interval.
- **Long-running servers**: records are batched by `flushInterval` and flushed when the server closes. Call
  `await monitor.close()` before the process exits, or the records still waiting are lost. srvx offers `waitUntil` on
  Node, Bun and Deno too, but there the queue batches by interval all the same.

`monitor.record(record)` records one request the way the middleware do, and `monitor.send(record)` queues one as it is.

## srvx

```ts
import { serve } from "srvx"
import { yuvaMonitor } from "useyuva/srvx"

serve({ plugins: [yuvaMonitor(monitor, { route: (request) => routeOf(request) })], fetch: app.fetch })
```

srvx has no router, so the route is what `route(request)` returns, else the one an h3 app put on the request's
context. `server.close()` flushes.

## h3

```ts
import { H3 } from "h3"
import { defineYuvaMonitor } from "useyuva/h3"

const app = new H3().use(defineYuvaMonitor({ monitor })).get("/users/:id", getUser)
```

Register it first, before other middleware.

## Nitro and Nuxt

Both read the key from the server runtime config, never the public one, so it cannot reach the browser. Set the
Yuva address and the two values:

| | Nuxt | Nitro |
|---|---|---|
| Address | `yuva.server`, or `NUXT_YUVA_SERVER` | `NITRO_YUVA_SERVER` |
| Key | `NUXT_YUVA_MONITOR_API_KEY` | `NITRO_YUVA_MONITOR_API_KEY` |
| Property | `NUXT_YUVA_MONITOR_PROPERTY_ID` | `NITRO_YUVA_MONITOR_PROPERTY_ID` |

In Nuxt, add the module and set the address in `nuxt.config`:

```ts
export default defineNuxtConfig({
  modules: ["useyuva/nuxt"],
  yuva: { server: "https://support.example.com", monitor: { service: "shop-api" } },
})
```

In Nitro, add `yuva` from `useyuva/nitro` to `modules`, with `yuva({ server, monitor: { service: "shop-api" } })`.
`monitor` takes `apiKey`, `propertyId`, `environment`, `service`, `release`, `sampleRate` and `flushInterval`; the key and
the property id can come from the variables instead. The monitor is on when both are known at build time, from the
config or the environment; a value that is only in the build environment is not written into the build, so set it at
runtime too. Routes come from Nitro's router. Requests for public assets and, in Nuxt, for `/_nuxt/` (the build assets
directory) are not recorded, so asset paths never become routes. Records are flushed through `waitUntil` where the
platform has one, and when Nitro closes. It works on Nitro v2 and v3.

## Hono

```ts
import { Hono } from "hono"
import { yuvaMonitor } from "useyuva/hono"

const app = new Hono().use(yuvaMonitor(monitor)).get("/users/:id", getUser)
```

The route is the path of the handler that answered, sub-apps included. On Cloudflare Workers records are flushed
through `executionCtx.waitUntil`; with `@hono/node-server` when the server closes.

## Express

```ts
import { yuvaMonitor } from "useyuva/express"

app.use(yuvaMonitor(monitor))
app.get("/users/:id", getUser)
```

The route is the router's base path and the route's path together. A connection closed before the response is
status `0`. The HTTP server's `close` flushes.

## Fastify

```ts
import { yuvaMonitor } from "useyuva/fastify"

await app.register(yuvaMonitor(monitor))
app.get("/users/:id", getUser)
```

Every route of the app is recorded, prefixed plugins included. `app.close()` flushes.

## Next.js route handlers

A route handler does not know its pattern at run time, so you pass it as the second argument, written like the folder:
`[id]` and `[...slug]` become `{id}` and `{slug...}`, and route groups and slots are left out.

```ts
// app/users/[id]/route.ts
import { withYuvaMonitor } from "useyuva/next"
import { monitor } from "@/monitor"

export const GET = withYuvaMonitor(monitor, "/users/[id]", async (request, { params }) => {
  return Response.json(await findUser((await params).id))
})
```

`notFound()` and `redirect()` count with their status. Where the platform gives Next.js a `waitUntil` (Vercel), the
flush goes through `after()`; `next start` batches by interval.

## Read the numbers

Records show up in a query within about 15 seconds. Hiding a noisy route, route rules and the query are in
[API monitoring](analytics-api.md#list-and-hide-routes). Without a framework, send records yourself with
[curl](analytics-api.md#send-requests-with-curl) or `monitor.send`.

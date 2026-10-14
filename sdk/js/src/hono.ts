import type { Context, MiddlewareHandler } from "hono"
import { matchedRoutes } from "hono/route"
import {
  contentLength,
  errorStatus,
  flushOnClose,
  observe,
  started,
  type Monitor,
  type MonitorOptions,
} from "./server/monitor"

export type YuvaMonitorOptions = MonitorOptions<Context>

type NodeBindings = { incoming?: { socket?: { server?: Parameters<typeof flushOnClose>[0] } } }

function handlerRoute(c: Context, self: MiddlewareHandler): string | undefined {
  const routes = matchedRoutes(c)
  for (let i = routes.length - 1; i >= 0; i--) {
    const route = routes[i]!
    if (route.handler === self || (route.method === "ALL" && route.path.endsWith("*"))) continue
    return route.path
  }
  return undefined
}

function waitUntil(c: Context, monitor: Monitor): void {
  let ctx: { waitUntil(promise: Promise<unknown>): void }
  try {
    ctx = c.executionCtx
  } catch {
    return
  }
  ctx.waitUntil(monitor.flush())
}

export function yuvaMonitor(monitor: Monitor, options: YuvaMonitorOptions = {}): MiddlewareHandler {
  const middleware: MiddlewareHandler = async (c, next) => {
    const start = started()
    flushOnClose((c.env as NodeBindings | undefined)?.incoming?.socket?.server, monitor)
    let status: number | undefined
    try {
      await next()
    } catch (error) {
      status = errorStatus(error)
      throw error
    } finally {
      observe(monitor, options, c, {
        ...start,
        method: c.req.method,
        route: handlerRoute(c, middleware),
        path: c.req.path,
        status: status ?? c.res.status,
        requestSize: contentLength(c.req.header("content-length")),
        responseSize: status ? undefined : contentLength(c.res.headers.get("content-length")),
      })
      waitUntil(c, monitor)
    }
  }
  return middleware
}

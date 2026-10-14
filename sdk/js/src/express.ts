import type { Request, RequestHandler } from "express"
import { contentLength, flushOnClose, observe, started, type Monitor, type MonitorOptions } from "./server/monitor"

export type YuvaMonitorOptions = MonitorOptions<Request>

function routeOf(req: Request): string | undefined {
  const path: unknown = req.route?.path
  if (typeof path !== "string") return undefined
  const base = req.baseUrl || ""
  return base && path === "/" ? base : base + path
}

const header = (value: string | number | string[] | undefined) => (Array.isArray(value) ? undefined : value)

export function yuvaMonitor(monitor: Monitor, options: YuvaMonitorOptions = {}): RequestHandler {
  return (req, res, next) => {
    const start = started()
    flushOnClose((req.socket as { server?: Parameters<typeof flushOnClose>[0] } | undefined)?.server, monitor)
    let done = false
    const finish = (sent: boolean) => {
      if (done) return
      done = true
      observe(monitor, options, req, {
        ...start,
        method: req.method,
        route: routeOf(req),
        path: req.originalUrl,
        status: sent ? res.statusCode : 0,
        requestSize: contentLength(header(req.headers["content-length"])),
        responseSize: sent ? contentLength(header(res.getHeader("content-length"))) : undefined,
      })
    }
    res.once("finish", () => finish(true))
    res.once("close", () => finish(res.writableFinished))
    next()
  }
}

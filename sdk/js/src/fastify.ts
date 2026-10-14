import type { FastifyPluginCallback, FastifyRequest } from "fastify"
import { contentLength, observe, started, type Monitor, type MonitorOptions, type Started } from "./server/monitor"

export type YuvaMonitorOptions = MonitorOptions<FastifyRequest>

const header = (value: string | number | string[] | undefined) => (Array.isArray(value) ? undefined : value)

export function yuvaMonitor(monitor: Monitor, options: YuvaMonitorOptions = {}): FastifyPluginCallback {
  const plugin: FastifyPluginCallback = (app, _opts, done) => {
    const starts = new WeakMap<FastifyRequest, Started>()
    const record = (request: FastifyRequest, status: number, responseSize?: number) => {
      const start = starts.get(request)
      if (!start) return
      starts.delete(request)
      observe(monitor, options, request, {
        ...start,
        method: request.method,
        route: request.routeOptions.url,
        path: request.url,
        status,
        requestSize: contentLength(header(request.headers["content-length"])),
        responseSize,
      })
    }
    app.addHook("onRequest", (request, _reply, next) => {
      starts.set(request, started())
      next()
    })
    app.addHook("onResponse", (request, reply, next) => {
      record(request, reply.statusCode, contentLength(header(reply.getHeader("content-length"))))
      next()
    })
    app.addHook("onRequestAbort", (request, next) => {
      record(request, 0)
      next()
    })
    app.addHook("onClose", (_app, next) => {
      void monitor.flush().then(() => next())
    })
    done()
  }
  // Fastify reads these symbols: the hooks apply to the whole app instead of an encapsulated context.
  Object.assign(plugin, { [Symbol.for("skip-override")]: true, [Symbol.for("fastify.display-name")]: "useyuva" })
  return plugin
}

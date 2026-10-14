import type { ServerPlugin, ServerRequest } from "srvx"
import { contentLength, errorStatus, observe, started, type Monitor, type MonitorOptions } from "./server/monitor"

export interface YuvaMonitorOptions extends MonitorOptions<ServerRequest> {
  route?: (request: ServerRequest) => string | null | undefined
}

type Routed = { context?: { matchedRoute?: { route?: unknown } } }

const matchedRoute = (request: ServerRequest) => {
  const route = (request as Routed).context?.matchedRoute?.route
  return typeof route === "string" ? route : undefined
}

export function yuvaMonitor(monitor: Monitor, options: YuvaMonitorOptions = {}): ServerPlugin {
  return (server) => {
    server.options.middleware.unshift(async (request, next) => {
      const start = started()
      let status = 500
      let response: Response | undefined
      try {
        response = await next()
        status = response.status
        return response
      } catch (error) {
        status = errorStatus(error)
        throw error
      } finally {
        observe(monitor, options, request, {
          ...start,
          method: request.method,
          route: options.route?.(request) || matchedRoute(request),
          path: new URL(request.url).pathname,
          status,
          requestSize: contentLength(request.headers.get("content-length")),
          responseSize: contentLength(response?.headers.get("content-length")),
        })
        request.waitUntil?.(monitor.flushed())
      }
    })
    const close = server.close.bind(server)
    server.close = async (closeActiveConnections) => {
      await Promise.all([close(closeActiveConnections), monitor.flush()])
      await monitor.flush()
    }
  }
}

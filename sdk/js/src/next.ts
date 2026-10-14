import { after } from "next/server"
import { contentLength, observe, started, type Monitor, type MonitorOptions } from "./server/monitor"

export type YuvaMonitorOptions = MonitorOptions<Request>

export function nextRoute(route: string): string {
  const segments = route
    .split("/")
    .filter((segment) => !/^\(.*\)$/.test(segment) && !segment.startsWith("@"))
    .map((segment) => segment.replace(/^\[\[?\.\.\.(\w+)\]\]?$/, "{$1...}").replace(/\[(\w+)\]/g, "{$1}"))
  return segments.join("/") || "/"
}

function thrownStatus(error: unknown): number {
  const digest = (error as { digest?: unknown } | null)?.digest
  if (typeof digest === "string") {
    const parts = digest.split(";")
    if (parts[0] === "NEXT_HTTP_ERROR_FALLBACK") return Number(parts[1]) || 500
    if (parts[0] === "NEXT_REDIRECT") return Number(parts.at(-2)) || 307
  }
  return 500
}

export function withYuvaMonitor<R extends Request, A extends unknown[]>(
  monitor: Monitor,
  route: string,
  handler: (request: R, ...rest: A) => Response | Promise<Response>,
  options: YuvaMonitorOptions = {}
): (request: R, ...rest: A) => Promise<Response> {
  const pattern = nextRoute(route)
  return async (request, ...rest) => {
    const start = started()
    let status = 500
    let response: Response | undefined
    try {
      response = await handler(request, ...rest)
      status = response.status
      return response
    } catch (error) {
      status = thrownStatus(error)
      throw error
    } finally {
      observe(monitor, options, request, {
        ...start,
        method: request.method,
        route: pattern,
        path: new URL(request.url).pathname,
        status,
        requestSize: contentLength(request.headers.get("content-length")),
        responseSize: contentLength(response?.headers.get("content-length")),
      })
      try {
        after(() => monitor.flushed())
      } catch {}
    }
  }
}

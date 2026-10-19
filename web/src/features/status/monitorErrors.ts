import { useLingui } from "@lingui/react/macro"

import { ApiError, type StatusMonitorKind } from "@/lib/api"

export type MonitorField =
  | "url"
  | "host"
  | "port"
  | "headers"
  | "body"
  | "keyword"
  | "status"
  | "cert"
  | "interval"
  | "timeout"

export type FieldErrors = Partial<Record<MonitorField, string>>

const PREFIXES: [string, MonitorField][] = [
  ["http.url", "url"],
  ["tcp.host", "host"],
  ["tcp.port", "port"],
  ["http.headers", "headers"],
  ["http.body", "body"],
  ["http.keyword", "keyword"],
  ["http.expected_status", "status"],
  ["http.cert_warn_days", "cert"],
  ["interval must", "interval"],
  ["timeout must", "timeout"],
]

export function useMonitorFieldErrors() {
  const { t } = useLingui()
  return (err: unknown, kind: StatusMonitorKind): { fields: FieldErrors; rest: unknown } => {
    if (!(err instanceof ApiError) || err.code !== "validation_failed" || !err.detail) return { fields: {}, rest: err }
    const detail = err.detail
    if (detail.startsWith("the target points to")) {
      const message = t`This address is private, loopback or link-local. Checks only reach public addresses.`
      return { fields: kind === "tcp" ? { host: message } : { url: message }, rest: null }
    }
    const hit = PREFIXES.find(([p]) => detail.startsWith(p))
    if (!hit) return { fields: {}, rest: err }
    const [, field] = hit
    if (detail.startsWith("tcp.host") && detail.includes("without a port"))
      return { fields: { host: t`Enter the host without a port. The port has its own field.` }, rest: null }
    if (field === "host") return { fields: { host: t`Enter a host name or an IP address.` }, rest: null }
    if (field === "port") return { fields: { port: t`The port must be 1 to 65535.` }, rest: null }
    if (field === "url")
      return {
        fields: { url: t`Enter an http or https address with a host and without a user name or password.` },
        rest: null,
      }
    if (field === "timeout") return { fields: { timeout: t`The timeout must be 1 to 30 seconds.` }, rest: null }
    if (field === "interval") {
      const m = /(\d+) to (\d+) seconds/.exec(detail)
      const lo = m?.[1]
      const hi = m?.[2]
      return {
        fields: { interval: lo && hi ? t`The interval must be ${lo} to ${hi} seconds.` : detail },
        rest: null,
      }
    }
    return { fields: { [field]: detail }, rest: null }
  }
}

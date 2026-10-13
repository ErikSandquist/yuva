import { createAnalytics, type Analytics } from "./analytics/core"

type Call = [keyof Analytics, ...unknown[]]
type Host = Window & { yuva?: Record<string, unknown>; yuvaq?: Call[] | { push(call: Call): void } }

const script = document.currentScript as HTMLScriptElement | null
const data = script?.dataset ?? {}
const host = window as Host
const analytics = createAnalytics({
  key: data.key ?? "",
  server: data.api ?? new URL(script?.src ?? "/", location.href).origin,
  hash: "hash" in data,
  consent: data.consent as "required" | undefined,
  manual: "manual" in data,
  outbound: "outbound" in data,
  files: "files" in data,
  chat: "chat" in data,
})
const run = ([name, ...args]: Call) => (analytics[name] as (...args: unknown[]) => void)?.(...args)
const queued = host.yuvaq

host.yuva = Object.assign(host.yuva ?? {}, analytics)
host.yuvaq = { push: run }
;(queued as Call[] | undefined)?.forEach?.(run)

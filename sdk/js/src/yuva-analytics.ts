import { createAnalytics, type AnalyticsMethods } from "./analytics/core"

type Call = [keyof AnalyticsMethods, ...unknown[]]
type Host = Window & { yuva?: Record<string, unknown>; yuvaq?: Call[] | { push(call: Call): void } }

const script = document.currentScript as HTMLScriptElement | null
const data = script?.dataset ?? {}
const host = window as Host
const analytics = createAnalytics({
  key: data.key ?? "",
  server: data.api ?? new URL(script?.src ?? "/", location.href).origin,
  hash: "hash" in data,
  consent: data.consent == "required" ? "required" : undefined,
  manual: "manual" in data,
})
const { pageview, consent, optOut, optIn } = analytics
const methods: AnalyticsMethods = { pageview, consent, optOut, optIn }
const run = ([name, ...args]: Call) => (methods[name] as (...args: unknown[]) => void)?.(...args)
const queued = host.yuvaq

host.yuva = Object.assign(host.yuva ?? {}, methods)
host.yuvaq = { push: run }
if (Array.isArray(queued)) queued.forEach(run)

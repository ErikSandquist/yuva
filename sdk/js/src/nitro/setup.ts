import { existsSync } from "node:fs"
import { join } from "node:path"
import { fileURLToPath } from "node:url"
import type { YuvaMonitorRuntimeConfig, YuvaServerRuntimeConfig } from "./runtime/core"

export type { YuvaMonitorRuntimeConfig, YuvaServerRuntimeConfig }

export const IDENTITY_ROUTE = "/_yuva/identity"
export const WEBHOOK_ROUTE = "/_yuva/webhook"
export const SERVER_MODULE = "#yuva/server"

export interface YuvaServerOptions {
  identitySecret?: string
  webhookSecret?: string
  analytics?: { apiKey?: string; propertyId?: string }
  monitor?: YuvaMonitorRuntimeConfig
}

export interface YuvaServerParts {
  identity: boolean
  webhook: boolean
  analytics: boolean
  monitor: boolean
}

export function serverRuntimeConfig(
  current: YuvaServerRuntimeConfig | undefined,
  options: YuvaServerOptions & { server?: string }
): Required<Omit<YuvaServerRuntimeConfig, "analytics" | "monitor">> & {
  analytics: { apiKey: string; propertyId: string }
  monitor: Required<YuvaMonitorRuntimeConfig>
} {
  const monitor = (key: "apiKey" | "propertyId" | "environment" | "service" | "release") =>
    current?.monitor?.[key] || options.monitor?.[key] || ""
  return {
    ...current,
    server: current?.server || options.server || "",
    identitySecret: current?.identitySecret || options.identitySecret || "",
    webhookSecret: current?.webhookSecret || options.webhookSecret || "",
    analytics: {
      ...current?.analytics,
      apiKey: current?.analytics?.apiKey || options.analytics?.apiKey || "",
      propertyId: current?.analytics?.propertyId || options.analytics?.propertyId || "",
    },
    monitor: {
      apiKey: monitor("apiKey"),
      propertyId: monitor("propertyId"),
      environment: monitor("environment"),
      service: monitor("service"),
      release: monitor("release"),
      sampleRate: current?.monitor?.sampleRate || options.monitor?.sampleRate || 1,
      flushInterval: current?.monitor?.flushInterval || options.monitor?.flushInterval || 5000,
    },
  }
}

export function serverParts(
  config: YuvaServerRuntimeConfig,
  env: Record<string, string | undefined>,
  prefix: string
): YuvaServerParts {
  const set = (value: string | undefined, name: string) => !!(value || env[`${prefix}YUVA_${name}`])
  return {
    identity: set(config.identitySecret, "IDENTITY_SECRET"),
    webhook: set(config.webhookSecret, "WEBHOOK_SECRET"),
    analytics:
      set(config.analytics?.apiKey, "ANALYTICS_API_KEY") && set(config.analytics?.propertyId, "ANALYTICS_PROPERTY_ID"),
    monitor: set(config.monitor?.apiKey, "MONITOR_API_KEY") && set(config.monitor?.propertyId, "MONITOR_PROPERTY_ID"),
  }
}

export function findServerFile(dir: string): string | undefined {
  for (const ext of ["ts", "mts", "js", "mjs"]) {
    const path = join(dir, `yuva.${ext}`)
    if (existsSync(path)) return path
  }
  return undefined
}

export function runtimePath(name: "identity" | "webhook" | "plugin" | "monitor"): string {
  return fileURLToPath(new URL(`./runtime/${name}.js`, import.meta.url))
}

export interface ServerModuleOptions {
  nitro: number
  server: string | undefined
  runtimeConfig: string
  h3?: string
  publicAssets?: string
  assetPrefixes?: string[]
}

export function serverModule({
  nitro,
  server,
  runtimeConfig,
  h3 = "h3",
  publicAssets,
  assetPrefixes = [],
}: ServerModuleOptions): string {
  const v3 = nitro >= 3
  const assets = publicAssets ?? (v3 ? "#nitro/virtual/public-assets" : "#nitro-internal-virtual/public-assets")
  const lines = [
    `import { useRuntimeConfig } from ${JSON.stringify(runtimeConfig)}`,
    `import { isPublicAssetURL } from ${JSON.stringify(assets)}`,
    ...(v3
      ? []
      : [`import { defineEventHandler, getRequestIP, getRequestURL, toWebRequest } from ${JSON.stringify(h3)}`]),
    server ? `import server from ${JSON.stringify(server)}` : "const server = {}",
    "export { server }",
    "export const config = () => useRuntimeConfig()",
    `const assetPrefixes = ${JSON.stringify(assetPrefixes)}`,
    "export const isAsset = (path) => {",
    "  let id = path",
    "  try {",
    "    id = decodeURI(path)",
    "  } catch {}",
    "  return isPublicAssetURL(id) || assetPrefixes.some((prefix) => id.startsWith(prefix))",
    "}",
    ...(v3
      ? [
          "export const defineHandler = (handler) => handler",
          "export const toRequest = (event) => event.req",
          "export const trackRequest = (event) => event.req",
          'export const onResponse = (hooks, fn) => hooks.hook("response", (_response, event) => fn(event))',
          "export const waitUntil = (event) => (event.req.waitUntil ? (promise) => event.waitUntil(promise) : undefined)",
          "export const serverlessWaitUntil = (event) =>",
          '  ["node", "bun", "deno"].includes(event.req.runtime?.name) ? undefined : waitUntil(event)',
          "export const requestInfo = (event) => ({",
          "  method: event.req.method,",
          "  path: event.url.pathname,",
          "  route: event.context.matchedRoute?.route,",
          '  size: event.req.headers.get("content-length"),',
          "})",
          "export const onResponded = (hooks, fn) =>",
          '  hooks.hook("response", (response, event) => fn(event, response.status, response.headers.get("content-length")))',
        ]
      : [
          "export const defineHandler = defineEventHandler",
          "export const toRequest = (event) => toWebRequest(event)",
          "export const trackRequest = (event) =>",
          "  Object.assign(new Request(getRequestURL(event), { headers: event.headers }), { ip: getRequestIP(event) })",
          'export const onResponse = (hooks, fn) => hooks.hook("afterResponse", (event) => fn(event))',
          "export const waitUntil = (event) => (event.context.waitUntil ? (promise) => event.waitUntil(promise) : undefined)",
          "export const serverlessWaitUntil = waitUntil",
          "export const requestInfo = (event) => ({",
          "  method: event.method,",
          "  path: event.path,",
          "  route: event.context.matchedRoute?.path,",
          '  size: event.headers.get("content-length"),',
          "})",
          "export const onResponded = (hooks, fn) =>",
          '  hooks.hook("afterResponse", (event) => fn(event, event.node.res.statusCode, event.node.res.getHeader("content-length")))',
        ]),
  ]
  return `${lines.join("\n")}\n`
}

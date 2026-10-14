import { existsSync } from "node:fs"
import { join } from "node:path"
import { fileURLToPath } from "node:url"
import type { YuvaServerRuntimeConfig } from "./runtime/core"

export type { YuvaServerRuntimeConfig }

export const IDENTITY_ROUTE = "/_yuva/identity"
export const WEBHOOK_ROUTE = "/_yuva/webhook"
export const SERVER_MODULE = "#yuva/server"

export interface YuvaServerOptions {
  identitySecret?: string
  webhookSecret?: string
  analytics?: { apiKey?: string; propertyId?: string }
}

export interface YuvaServerParts {
  identity: boolean
  webhook: boolean
  analytics: boolean
}

export function serverRuntimeConfig(
  current: YuvaServerRuntimeConfig | undefined,
  options: YuvaServerOptions & { server?: string }
): Required<Omit<YuvaServerRuntimeConfig, "analytics">> & { analytics: { apiKey: string; propertyId: string } } {
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
  }
}

export function findServerFile(dir: string): string | undefined {
  for (const ext of ["ts", "mts", "js", "mjs"]) {
    const path = join(dir, `yuva.${ext}`)
    if (existsSync(path)) return path
  }
  return undefined
}

export function runtimePath(name: "identity" | "webhook" | "plugin"): string {
  return fileURLToPath(new URL(`./runtime/${name}.js`, import.meta.url))
}

export interface ServerModuleOptions {
  nitro: number
  server: string | undefined
  runtimeConfig: string
  h3?: string
}

export function serverModule({ nitro, server, runtimeConfig, h3 = "h3" }: ServerModuleOptions): string {
  const v3 = nitro >= 3
  const lines = [
    `import { useRuntimeConfig } from ${JSON.stringify(runtimeConfig)}`,
    ...(v3
      ? []
      : [`import { defineEventHandler, getRequestIP, getRequestURL, toWebRequest } from ${JSON.stringify(h3)}`]),
    server ? `import server from ${JSON.stringify(server)}` : "const server = {}",
    "export { server }",
    "export const config = () => useRuntimeConfig()",
    ...(v3
      ? [
          "export const defineHandler = (handler) => handler",
          "export const toRequest = (event) => event.req",
          "export const trackRequest = (event) => event.req",
          'export const onResponse = (hooks, fn) => hooks.hook("response", (_response, event) => fn(event))',
          "export const waitUntil = (event) => (event.req.waitUntil ? (promise) => event.waitUntil(promise) : undefined)",
        ]
      : [
          "export const defineHandler = defineEventHandler",
          "export const toRequest = (event) => toWebRequest(event)",
          "export const trackRequest = (event) =>",
          "  Object.assign(new Request(getRequestURL(event), { headers: event.headers }), { ip: getRequestIP(event) })",
          'export const onResponse = (hooks, fn) => hooks.hook("afterResponse", (event) => fn(event))',
          "export const waitUntil = (event) => (event.context.waitUntil ? (promise) => event.waitUntil(promise) : undefined)",
        ]),
  ]
  return `${lines.join("\n")}\n`
}

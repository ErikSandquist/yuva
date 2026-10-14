import * as kit from "@nuxt/kit"
import {
  addComponent,
  addImports,
  addPlugin,
  addServerHandler,
  addServerImports,
  addServerPlugin,
  addTypeTemplate,
  defineNuxtModule,
  useLogger,
} from "@nuxt/kit"
import type { AnalyticsOptions } from "../analytics"
import {
  findServerFile,
  IDENTITY_ROUTE,
  runtimePath,
  SERVER_MODULE,
  serverModule,
  serverParts,
  serverRuntimeConfig,
  WEBHOOK_ROUTE,
  type YuvaMonitorRuntimeConfig,
  type YuvaServerRuntimeConfig,
} from "../nitro/setup"

export interface AnalyticsModuleOptions extends Omit<AnalyticsOptions, "server"> {
  server?: string
  apiKey?: string
  propertyId?: string
}

export type PublicAnalyticsOptions = Omit<AnalyticsModuleOptions, "apiKey" | "propertyId">

export interface ModuleOptions {
  server?: string
  channel?: string
  locale?: string
  analytics?: AnalyticsModuleOptions
  identitySecret?: string
  webhookSecret?: string
  monitor?: YuvaMonitorRuntimeConfig
}

export interface PublicModuleConfig {
  server: string
  channel: string
  locale: string
  identity: string
  analytics?: PublicAnalyticsOptions
}

declare module "@nuxt/schema" {
  interface PublicRuntimeConfig {
    yuva: PublicModuleConfig
  }
  interface RuntimeConfig {
    yuva?: YuvaServerRuntimeConfig
  }
}

const components = ["YuvaChat", "YuvaDiscussions", "YuvaPageFeedback", "YuvaSubscribe"]

const serverTypes = `import type { H3Event } from "h3"

declare module "useyuva/nitro/runtime" {
  interface YuvaServerTypes {
    event: H3Event
  }
}

export {}
`

export default defineNuxtModule<ModuleOptions>({
  meta: { name: "useyuva", configKey: "yuva" },
  defaults: { server: "", channel: "", locale: "" },
  setup(options, nuxt) {
    const { apiKey, propertyId, ...clientAnalytics } = options.analytics ?? { key: "" }
    const secrets = serverRuntimeConfig(nuxt.options.runtimeConfig.yuva, {
      identitySecret: options.identitySecret,
      webhookSecret: options.webhookSecret,
      analytics: { apiKey, propertyId },
      monitor: options.monitor,
    })
    const parts = serverParts(secrets, process.env, "NUXT_")

    const current = (nuxt.options.runtimeConfig.public.yuva ?? {}) as Partial<PublicModuleConfig>
    const base = nuxt.options.app.baseURL.replace(/\/+$/, "")
    nuxt.options.runtimeConfig.public.yuva = {
      server: current.server || options.server || "",
      channel: current.channel || options.channel || "",
      locale: current.locale || options.locale || "",
      identity: current.identity || (parts.identity ? `${base}${IDENTITY_ROUTE}` : ""),
      analytics: current.analytics || (options.analytics ? clientAnalytics : undefined),
    }

    const compilerOptions = (nuxt.options.vue.compilerOptions ??= {})
    const isCustomElement = compilerOptions.isCustomElement
    compilerOptions.isCustomElement = (tag) => tag.startsWith("yuva-") || !!isCustomElement?.(tag)

    nuxt.options.build.transpile.push("useyuva")

    for (const name of components) addComponent({ name, export: name, filePath: "useyuva/nuxt/components" })

    addImports({ name: "useYuvaAnalytics", from: "useyuva/nuxt/analytics" })
    if (nuxt.options.runtimeConfig.public.yuva.analytics) addPlugin({ src: "useyuva/nuxt/plugin", mode: "client" })

    addServerImports([{ name: "defineYuvaServer", from: "useyuva/nitro/runtime" }])
    addTypeTemplate({ filename: "types/useyuva-server.d.ts", getContents: () => serverTypes }, { nitro: true })

    if (!parts.identity && !parts.webhook && !parts.analytics && !parts.monitor) return
    nuxt.options.runtimeConfig.yuva = secrets

    const server = findServerFile(nuxt.options.serverDir)
    if (!server && parts.identity) {
      useLogger("useyuva").warn(`No yuva.ts in ${nuxt.options.serverDir}, so ${IDENTITY_ROUTE} answers 404`)
    }
    const nitro = kit.getNitroVersion?.(nuxt) ?? 2
    nuxt.hook("nitro:config", (config) => {
      config.virtual = {
        ...config.virtual,
        [SERVER_MODULE]: serverModule({
          nitro,
          server,
          runtimeConfig: "#imports",
          assetPrefixes: [`${base}/${nuxt.options.app.buildAssetsDir.replace(/^\/+|\/+$/g, "")}/`],
        }),
      }
    })
    if (parts.identity) addServerHandler({ route: IDENTITY_ROUTE, method: "GET", handler: runtimePath("identity") })
    if (parts.webhook) addServerHandler({ route: WEBHOOK_ROUTE, method: "POST", handler: runtimePath("webhook") })
    if (parts.analytics) addServerPlugin(runtimePath("plugin"))
    if (parts.monitor) addServerPlugin(runtimePath("monitor"))
  },
})

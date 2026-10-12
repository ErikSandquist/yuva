import { addComponent, addImports, addPlugin, defineNuxtModule } from "@nuxt/kit"
import type { AnalyticsOptions } from "../analytics"

export interface AnalyticsModuleOptions extends Omit<AnalyticsOptions, "server"> {
  server?: string
}

export interface ModuleOptions {
  server?: string
  channel?: string
  locale?: string
  analytics?: AnalyticsModuleOptions
}

declare module "@nuxt/schema" {
  interface PublicRuntimeConfig {
    yuva: Required<Omit<ModuleOptions, "analytics">> & { analytics?: AnalyticsModuleOptions }
  }
}

const components = ["YuvaChat", "YuvaDiscussions", "YuvaPageFeedback", "YuvaSubscribe"]

export default defineNuxtModule<ModuleOptions>({
  meta: { name: "useyuva", configKey: "yuva" },
  defaults: { server: "", channel: "", locale: "" },
  setup(options, nuxt) {
    const current = (nuxt.options.runtimeConfig.public.yuva ?? {}) as Partial<ModuleOptions>
    nuxt.options.runtimeConfig.public.yuva = {
      server: current.server || options.server || "",
      channel: current.channel || options.channel || "",
      locale: current.locale || options.locale || "",
      analytics: current.analytics || options.analytics,
    }

    const compilerOptions = (nuxt.options.vue.compilerOptions ??= {})
    const isCustomElement = compilerOptions.isCustomElement
    compilerOptions.isCustomElement = (tag) => tag.startsWith("yuva-") || !!isCustomElement?.(tag)

    nuxt.options.build.transpile.push("useyuva")

    for (const name of components) addComponent({ name, export: name, filePath: "useyuva/nuxt/components" })

    addImports({ name: "useYuvaAnalytics", from: "useyuva/nuxt/analytics" })
    if (nuxt.options.runtimeConfig.public.yuva.analytics) addPlugin({ src: "useyuva/nuxt/plugin", mode: "client" })
  },
})

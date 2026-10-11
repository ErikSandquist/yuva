import { addComponent, defineNuxtModule } from "@nuxt/kit"

export interface ModuleOptions {
  server?: string
  channel?: string
  locale?: string
}

declare module "@nuxt/schema" {
  interface PublicRuntimeConfig {
    yuva: Required<ModuleOptions>
  }
}

const components = ["YuvaChat", "YuvaDiscussions", "YuvaPageFeedback"]

export default defineNuxtModule<ModuleOptions>({
  meta: { name: "useyuva", configKey: "yuva" },
  defaults: { server: "", channel: "", locale: "" },
  setup(options, nuxt) {
    const current = (nuxt.options.runtimeConfig.public.yuva ?? {}) as Partial<ModuleOptions>
    nuxt.options.runtimeConfig.public.yuva = {
      server: current.server || options.server || "",
      channel: current.channel || options.channel || "",
      locale: current.locale || options.locale || "",
    }

    const compilerOptions = (nuxt.options.vue.compilerOptions ??= {})
    const isCustomElement = compilerOptions.isCustomElement
    compilerOptions.isCustomElement = (tag) => tag.startsWith("yuva-") || !!isCustomElement?.(tag)

    nuxt.options.build.transpile.push("useyuva")

    for (const name of components) addComponent({ name, export: name, filePath: "useyuva/nuxt/components" })
  },
})

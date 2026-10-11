export default defineNuxtConfig({
  compatibilityDate: "2026-10-01",
  modules: ["@nuxt/content", "useyuva/nuxt"],
  content: {
    experimental: { sqliteConnector: "native" },
  },
})

import { defineNuxtPlugin, useRuntimeConfig } from "#imports"
import { createAnalytics } from "../../analytics"

export default defineNuxtPlugin(() => {
  const config = useRuntimeConfig().public.yuva
  const options = config?.analytics
  if (!options?.key) return
  return {
    provide: { yuvaAnalytics: createAnalytics({ ...options, server: options.server || config?.server || "" }) },
  }
})

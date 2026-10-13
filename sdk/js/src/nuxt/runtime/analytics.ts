import { useNuxtApp } from "#imports"
import type { AnalyticsMethods } from "../../analytics"

const noop = (): undefined => {}
const idle: AnalyticsMethods = {
  pageview: noop,
  track: noop,
  notFound: noop,
  consent: noop,
  identify: noop,
  persistentId: noop,
  optOut: noop,
  optIn: noop,
}

export function useYuvaAnalytics(): AnalyticsMethods {
  return (useNuxtApp().$yuvaAnalytics as AnalyticsMethods | undefined) ?? idle
}

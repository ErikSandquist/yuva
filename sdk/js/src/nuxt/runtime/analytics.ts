import { useNuxtApp } from "#imports"
import type { AnalyticsMethods } from "../../analytics"

const noop = () => {}
const idle: AnalyticsMethods = { pageview: noop, track: noop, notFound: noop, consent: noop, optOut: noop, optIn: noop }

export function useYuvaAnalytics(): AnalyticsMethods {
  return (useNuxtApp().$yuvaAnalytics as AnalyticsMethods | undefined) ?? idle
}

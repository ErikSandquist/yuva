import type { Analytics, AnalyticsMethods } from "./core"

let current: Analytics | undefined
const pending: [keyof AnalyticsMethods, unknown[]][] = []

const call =
  (name: keyof AnalyticsMethods) =>
  (...args: unknown[]) =>
    current ? (current[name] as (...args: unknown[]) => void)(...args) : void pending.push([name, args])

export const analytics: AnalyticsMethods = {
  pageview: call("pageview"),
  track: call("track"),
  notFound: call("notFound"),
  consent: call("consent"),
  optOut: call("optOut"),
  optIn: call("optIn"),
}

export function attachAnalytics(instance: Analytics | undefined): void {
  current = instance
  if (instance) for (const [name, args] of pending.splice(0)) (instance[name] as (...args: unknown[]) => void)(...args)
}

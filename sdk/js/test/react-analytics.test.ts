import { afterEach, expect, test } from "bun:test"
import { act, createElement } from "react"
import { createRoot, type Root } from "react-dom/client"
import { YuvaAnalytics } from "../src/react"

const sent: string[] = []
let root: Root | undefined

afterEach(() => {
  act(() => root?.unmount())
  root = undefined
})

test("<YuvaAnalytics publicKey> sends a pageview with that key", async () => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  ;(window as unknown as { happyDOM: { setURL(url: string): void } }).happyDOM.setURL("https://site.test/")
  localStorage.clear()
  for (const name of ["globalPrivacyControl", "doNotTrack", "webdriver"])
    Object.defineProperty(navigator, name, { value: undefined, configurable: true, writable: true })
  const record = (url: string) => (sent.push(url), true)
  Object.defineProperty(navigator, "sendBeacon", { value: record, configurable: true, writable: true })
  globalThis.fetch = (async (url: string) => (record(url), new Response(null, { status: 202 }))) as typeof fetch

  const container = document.createElement("div")
  document.body.append(container)
  root = createRoot(container)
  await act(async () => {
    root!.render(createElement(YuvaAnalytics, { server: "https://yuva.test", publicKey: "yuva_ak_react" }))
    await new Promise((resolve) => setTimeout(resolve, 20))
  })
  for (let i = 0; i < 50 && !sent.length; i++) await new Promise((resolve) => setTimeout(resolve, 10))

  expect(sent).toContain("https://yuva.test/client/v1/analytics/yuva_ak_react")
})

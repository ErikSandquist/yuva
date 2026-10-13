import { useEffect, useRef } from "react"
import { useLocation } from "react-router"

type Analytics = { pageview?: (options: { url: string }) => void }

const id = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

export function PageViews() {
  const { pathname } = useLocation()
  const last = useRef("")
  useEffect(() => {
    const path = pathname
      .split("/")
      .map((segment, i, all) => (id.test(segment) ? ":id" : all[i - 1] === "share" && i === 2 ? ":token" : segment))
      .join("/")
    if (path === last.current) return
    last.current = path
    ;(window as Window & { yuva?: Analytics }).yuva?.pageview?.({ url: location.origin + path })
  }, [pathname])
  return null
}

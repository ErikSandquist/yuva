import { useCallback, useSyncExternalStore } from "react"

import { useMediaQuery } from "@/hooks/use-media-query"

const KEY = "sidebar"
const listeners = new Set<() => void>()
let memory = ""

function read(): string {
  try {
    return localStorage.getItem(KEY) ?? memory
  } catch {
    return memory
  }
}

export function useSidebarCollapsed() {
  const narrow = useMediaQuery("(max-width: 1099px)")
  const stored = useSyncExternalStore((l) => {
    listeners.add(l)
    return () => listeners.delete(l)
  }, read)
  const collapsed = stored ? stored === "collapsed" : narrow
  const set = useCallback((next: boolean) => {
    memory = next ? "collapsed" : "open"
    try {
      localStorage.setItem(KEY, memory)
    } catch {
      // Storage can be blocked; the choice then lasts until the page reloads.
    }
    listeners.forEach((l) => l())
  }, [])
  const toggle = useCallback(() => set(!collapsed), [set, collapsed])
  return [collapsed, toggle] as const
}

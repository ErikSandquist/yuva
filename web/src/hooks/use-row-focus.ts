import { useRef } from "react"

import { useHotkeys } from "@/hooks/use-hotkeys"

export function useRowFocus<T extends HTMLElement>(enabled = true) {
  const ref = useRef<T>(null)
  const move = (step: number) => {
    const rows = Array.from(ref.current?.querySelectorAll<HTMLElement>(":scope li > a") ?? [])
    if (rows.length === 0) return
    const i = rows.indexOf(document.activeElement as HTMLElement)
    const next = i < 0 ? (step > 0 ? 0 : rows.length - 1) : Math.min(rows.length - 1, Math.max(0, i + step))
    rows[next]?.focus()
    rows[next]?.scrollIntoView({ block: "nearest" })
  }
  useHotkeys({ j: () => move(1), k: () => move(-1), ArrowDown: () => move(1), ArrowUp: () => move(-1) }, enabled)
  return ref
}

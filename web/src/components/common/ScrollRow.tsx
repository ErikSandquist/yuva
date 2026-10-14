import { useEffect, useRef, useState } from "react"

import { cn } from "@/lib/utils"

export function ScrollRow({ children, className }: { children: React.ReactNode; className?: string }) {
  const ref = useRef<HTMLDivElement>(null)
  const [edges, setEdges] = useState({ left: false, right: false })
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const update = () => {
      const max = el.scrollWidth - el.clientWidth
      const at = Math.abs(el.scrollLeft)
      const rtl = getComputedStyle(el).direction === "rtl"
      const before = at > 2
      const after = max - at > 2
      setEdges(rtl ? { left: after, right: before } : { left: before, right: after })
    }
    update()
    el.addEventListener("scroll", update, { passive: true })
    const ro = new ResizeObserver(update)
    ro.observe(el)
    return () => {
      el.removeEventListener("scroll", update)
      ro.disconnect()
    }
  }, [])
  const fade = (on: boolean) => (on ? "transparent" : "black")
  return (
    <div
      ref={ref}
      className={cn("-mx-1 max-w-[calc(100%+0.5rem)] overflow-x-auto px-1 py-1 [scrollbar-width:none]", className)}
      style={{
        maskImage: `linear-gradient(to right, ${fade(edges.left)}, black 24px, black calc(100% - 24px), ${fade(edges.right)})`,
      }}
      data-scroll-row
    >
      {children}
    </div>
  )
}

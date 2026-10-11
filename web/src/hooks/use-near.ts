import { useEffect, useState } from "react"

export function useNear(el: Element | null, active: boolean, margin = "800px") {
  const [near, setNear] = useState(false)
  const observable = typeof IntersectionObserver !== "undefined"
  useEffect(() => {
    if (!active || near || !el || !observable) return
    const io = new IntersectionObserver((entries) => entries.some((e) => e.isIntersecting) && setNear(true), {
      rootMargin: margin,
    })
    io.observe(el)
    return () => io.disconnect()
  }, [el, active, near, margin, observable])
  return near || (!observable && active && !!el)
}

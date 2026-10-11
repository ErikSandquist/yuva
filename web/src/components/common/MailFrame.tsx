import { useEffect, useMemo, useRef, useState } from "react"

import { cn } from "@/lib/utils"

const FRAME_CSP =
  "default-src 'none'; style-src 'unsafe-inline'; img-src data: https: http:; font-src data:; form-action 'none'"

export function frameDoc(html: string) {
  const head = `<meta http-equiv="Content-Security-Policy" content="${FRAME_CSP}"><base target="_blank"><style>html,body{overflow:hidden}</style>`
  if (/<head[^>]*>/i.test(html)) return html.replace(/<head[^>]*>/i, (m) => `${m}${head}`)
  return `<!doctype html><html><head><meta charset="utf-8">${head}</head><body>${html}</body></html>`
}

export function MailFrame({
  html,
  width,
  dark,
  className,
  title,
}: {
  html: string
  width?: number
  dark?: boolean
  className?: string
  title: string
}) {
  const frame = useRef<HTMLIFrameElement>(null)
  const [height, setHeight] = useState(600)
  const srcDoc = useMemo(() => frameDoc(html), [html])
  useEffect(() => {
    const el = frame.current
    if (!el) return
    let ro: ResizeObserver | undefined
    const measure = () => {
      const doc = el.contentDocument
      if (!doc?.documentElement) return
      setHeight(Math.max(200, doc.documentElement.scrollHeight))
    }
    const start = () => {
      const doc = el.contentDocument
      const win = el.contentWindow as (Window & typeof globalThis) | null
      if (!doc || !win) return
      measure()
      ro?.disconnect()
      ro = new win.ResizeObserver(measure)
      ro.observe(doc.documentElement)
      for (const img of doc.images) img.addEventListener("load", measure)
    }
    el.addEventListener("load", start)
    start()
    return () => {
      el.removeEventListener("load", start)
      ro?.disconnect()
    }
  }, [srcDoc])
  return (
    <iframe
      ref={frame}
      title={title}
      srcDoc={srcDoc}
      sandbox="allow-same-origin allow-popups allow-popups-to-escape-sandbox"
      referrerPolicy="no-referrer"
      className={cn("block border-0 bg-white", className)}
      style={{ height, width: width ?? "100%", colorScheme: dark ? "dark" : "light" }}
      data-testid="mail-frame"
    />
  )
}

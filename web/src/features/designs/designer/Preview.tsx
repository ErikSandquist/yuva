import { Trans, useLingui } from "@lingui/react/macro"
import { LoaderIcon, TriangleAlertIcon } from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"

import { ErrorLine, Notice } from "@/components/common"
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

export type PreviewMode = "desktop" | "phone" | "dark"

export function Preview({
  mode,
  html,
  loading,
  error,
  stale,
}: {
  mode: PreviewMode
  html: string | undefined
  loading: boolean
  error: unknown
  stale: boolean
}) {
  const { t } = useLingui()
  const phone = mode === "phone"
  return (
    <div
      className={cn("flex min-h-full flex-col items-center px-4 py-6", mode === "dark" ? "bg-[#0d0d10]" : "bg-surface")}
      data-testid={`preview-${mode}`}
    >
      {error ? (
        <Notice tone="danger" icon={TriangleAlertIcon} className="mb-4 w-full max-w-xl">
          <ErrorLine error={error} />
        </Notice>
      ) : null}
      {html ? (
        <div
          className={cn(
            "relative w-full transition-opacity",
            phone
              ? "w-[391px] max-w-full rounded-[44px] border-[8px] border-[#1c1c21] bg-[#1c1c21] shadow-xl"
              : "max-w-[960px] overflow-hidden rounded-xl border shadow-sm",
            stale && "opacity-70"
          )}
        >
          <div className={cn("overflow-hidden", phone && "rounded-[36px]")}>
            <MailFrame html={html} dark={mode === "dark"} title={t`Preview`} />
          </div>
        </div>
      ) : loading ? (
        <div className="flex flex-1 items-center gap-2 text-body text-faint">
          <LoaderIcon className="size-4 animate-spin" />
          <Trans>Rendering the mail…</Trans>
        </div>
      ) : null}
      {html && stale && (
        <span className="fixed bottom-6 inline-flex items-center gap-1.5 rounded-full bg-foreground px-3 py-1.5 text-caption text-background shadow-lg">
          <LoaderIcon className="size-3 animate-spin" />
          <Trans>Updating…</Trans>
        </span>
      )}
    </div>
  )
}

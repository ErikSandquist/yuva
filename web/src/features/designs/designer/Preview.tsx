import { Trans, useLingui } from "@lingui/react/macro"
import { LoaderIcon, TriangleAlertIcon } from "lucide-react"

import { ErrorLine, Notice } from "@/components/common"
import { MailFrame } from "@/components/common/MailFrame"
import { cn } from "@/lib/utils"

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

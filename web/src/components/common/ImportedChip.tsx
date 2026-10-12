import { Trans, useLingui } from "@lingui/react/macro"
import { HistoryIcon } from "lucide-react"

import { formatDate } from "@/components/common/text"
import { cn } from "@/lib/utils"

export function ImportedChip({ at, className }: { at: string; className?: string }) {
  const { t, i18n } = useLingui()
  const date = formatDate(at, i18n.locale)
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-1 rounded-full border bg-card px-2 py-px text-caption text-muted-foreground",
        className
      )}
      title={t`Imported into Yuva on ${date}`}
      data-testid="imported-chip"
    >
      <HistoryIcon className="size-3 shrink-0" aria-hidden />
      <Trans>Imported</Trans>
    </span>
  )
}

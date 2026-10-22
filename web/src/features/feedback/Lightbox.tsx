import { useLingui } from "@lingui/react/macro"
import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react"
import { useEffect } from "react"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog"

export type Shot = { id: string; url: string; name: string }

export function Lightbox({
  shots,
  index,
  onIndex,
  onClose,
}: {
  shots: Shot[]
  index: number
  onIndex: (i: number) => void
  onClose: () => void
}) {
  const { t } = useLingui()
  const shot = shots[index]
  const step = (d: number) => onIndex((index + d + shots.length) % shots.length)
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (shots.length < 2) return
      if (e.key === "ArrowLeft") step(-1)
      else if (e.key === "ArrowRight") step(1)
      else return
      e.preventDefault()
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  })
  if (!shot) return null
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="flex max-h-[90svh] flex-col items-center gap-3 sm:max-w-3xl" data-testid="lightbox">
        <DialogTitle className="sr-only">{t`Screenshot`}</DialogTitle>
        <DialogDescription className="sr-only">{shot.name}</DialogDescription>
        <img src={shot.url} alt={shot.name} className="max-h-[72svh] max-w-full rounded-lg object-contain" />
        <div className="flex items-center gap-3 text-small text-muted-foreground">
          <Button
            variant="outline"
            size="icon-sm"
            onClick={() => step(-1)}
            disabled={shots.length < 2}
            aria-label={t`Previous screenshot`}
            data-testid="lightbox-prev"
          >
            <ChevronLeftIcon className="rtl:rotate-180" />
          </Button>
          <span className="tabular-nums">
            {index + 1} / {shots.length}
          </span>
          <Button
            variant="outline"
            size="icon-sm"
            onClick={() => step(1)}
            disabled={shots.length < 2}
            aria-label={t`Next screenshot`}
            data-testid="lightbox-next"
          >
            <ChevronRightIcon className="rtl:rotate-180" />
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

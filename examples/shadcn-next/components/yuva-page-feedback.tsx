"use client"

import * as React from "react"
import { ThumbsDown, ThumbsUp } from "lucide-react"
import { Button } from "@/components/ui/button"
import { usePageRating, useYuvaLabels } from "@/hooks/use-yuva-discussions"
import { cn } from "@/lib/utils"
import { currentPage, yuvaErrorMessage, type YuvaDiscussionsLabels } from "@/lib/yuva-utils"

export interface YuvaPageFeedbackProps {
  page?: string
  pageTitle?: string
  labels?: Partial<YuvaDiscussionsLabels>
  className?: string
}

export function YuvaPageFeedback({ page: pageProp, pageTitle, labels: overrides, className }: YuvaPageFeedbackProps) {
  const labels = useYuvaLabels(overrides)
  const [page, setPage] = React.useState<{ url: string; title: string } | null>(null)
  const [error, setError] = React.useState<string | null>(null)
  const [thanked, setThanked] = React.useState(false)

  React.useEffect(() => {
    const here = currentPage()
    // oxlint-disable-next-line react/set-state-in-effect -- the page comes from location, read after hydration so the server render matches
    setPage({ url: pageProp ?? here.url, title: pageTitle ?? here.title })
    setThanked(false)
  }, [pageProp, pageTitle])

  const { rating, busy, rate } = usePageRating(page?.url, page?.title)
  const choose = (value: "up" | "down") => {
    setError(null)
    rate(value).then(
      () => setThanked(true),
      (failure) => setError(yuvaErrorMessage(failure, labels))
    )
  }

  return (
    <div
      role="group"
      aria-label={labels.feedbackQuestion}
      className={cn("flex flex-wrap items-center gap-x-3 gap-y-2 text-sm", className)}
    >
      <span className="font-medium">{labels.feedbackQuestion}</span>
      <div className="flex gap-2">
        <Button
          type="button"
          variant={rating === "up" ? "secondary" : "outline"}
          size="sm"
          aria-pressed={rating === "up"}
          disabled={busy}
          onClick={() => choose("up")}
        >
          <ThumbsUp data-icon="inline-start" />
          {labels.feedbackYes}
        </Button>
        <Button
          type="button"
          variant={rating === "down" ? "secondary" : "outline"}
          size="sm"
          aria-pressed={rating === "down"}
          disabled={busy}
          onClick={() => choose("down")}
        >
          <ThumbsDown data-icon="inline-start" />
          {labels.feedbackNo}
        </Button>
      </div>
      <span aria-live="polite" className={cn("text-muted-foreground", error && "text-destructive")}>
        {error ?? (thanked ? labels.feedbackThanks : "")}
      </span>
    </div>
  )
}

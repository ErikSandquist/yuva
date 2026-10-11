import { Trans } from "@lingui/react/macro"
import { GlobeIcon, ThumbsDownIcon, ThumbsUpIcon } from "lucide-react"

import type { Conversation, PageRating, SourcePage } from "@/lib/api"
import { cn } from "@/lib/utils"

export function pageLabel(url: string, title?: string) {
  if (title) return title
  try {
    const u = new URL(url)
    return u.host + (u.pathname === "/" ? "" : u.pathname)
  } catch {
    return url
  }
}

export function PageLink({ url, title, className }: { url: string; title?: string; className?: string }) {
  return (
    <a
      href={url}
      target="_blank"
      rel="noreferrer"
      title={url}
      className={cn(
        "min-w-0 truncate text-muted-foreground underline-offset-4 hover:text-foreground hover:underline",
        className
      )}
      data-testid="page-link"
    >
      {pageLabel(url, title)}
    </a>
  )
}

export function PageRatingMark({ rating }: { rating: PageRating }) {
  return rating === "up" ? (
    <span className="inline-flex shrink-0 items-center gap-1 text-success" data-testid="page-rating" data-rating="up">
      <ThumbsUpIcon className="size-3.5" />
      <Trans>Helpful</Trans>
    </span>
  ) : (
    <span
      className="inline-flex shrink-0 items-center gap-1 text-destructive"
      data-testid="page-rating"
      data-rating="down"
    >
      <ThumbsDownIcon className="size-3.5" />
      <Trans>Not helpful</Trans>
    </span>
  )
}

export function conversationPage(c: Conversation): SourcePage | undefined {
  if (c.page) return c.page
  if (c.feedback?.page_url) return { page_url: c.feedback.page_url, page_title: c.feedback.page_title }
  return undefined
}

export function PageLine({ c }: { c: Conversation }) {
  const page = conversationPage(c)
  if (!page) return null
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5" data-testid="conversation-page">
      <GlobeIcon className="size-3.5 shrink-0" />
      <PageLink url={page.page_url} title={page.page_title} />
      {c.feedback?.rating && (
        <>
          <span aria-hidden>·</span>
          <PageRatingMark rating={c.feedback.rating} />
        </>
      )}
    </span>
  )
}

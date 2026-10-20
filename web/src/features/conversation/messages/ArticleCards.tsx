import { useLingui } from "@lingui/react/macro"
import { ExternalLinkIcon, LifeBuoyIcon } from "lucide-react"

import type { MessageCard } from "@/lib/api"
import { cn } from "@/lib/utils"

export function withoutCardLines(body: string, cards?: MessageCard[]) {
  if (!cards?.length) return body
  const lines = body.split("\n")
  while (lines.length > 0) {
    const last = lines[lines.length - 1]
    if (last.trim() === "" || cards.some((c) => last.includes(c.url))) lines.pop()
    else break
  }
  return lines.join("\n")
}

const safe = (url: string) => /^https?:\/\//i.test(url)

export function ArticleCards({
  items,
  outgoing,
  className,
}: {
  items?: MessageCard[]
  outgoing?: boolean
  className?: string
}) {
  const { t } = useLingui()
  if (!items?.length) return null
  return (
    <div className={cn("flex flex-col gap-1.5", outgoing && "items-end", className)} data-testid="article-cards">
      {items.map((card) => (
        <a
          key={card.article_id}
          href={safe(card.url) ? card.url : undefined}
          target="_blank"
          rel="noreferrer noopener"
          className="flex w-72 max-w-full items-start gap-2.5 rounded-xl border bg-card px-3 py-2.5 text-start outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
          title={t`Open the help article`}
          data-testid="article-card"
        >
          <LifeBuoyIcon className="mt-0.5 size-4 shrink-0 text-brand" />
          <span className="min-w-0 flex-1">
            <span className="block truncate text-body font-medium" dir="auto">
              {card.title}
            </span>
            {card.excerpt && (
              <span className="mt-0.5 line-clamp-2 block text-small text-muted-foreground" dir="auto">
                {card.excerpt}
              </span>
            )}
          </span>
          <ExternalLinkIcon className="mt-0.5 size-3.5 shrink-0 text-faint" />
        </a>
      ))}
    </div>
  )
}

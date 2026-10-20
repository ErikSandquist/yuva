"use client"

import * as React from "react"
import { ChevronRightIcon, ThumbsDownIcon, ThumbsUpIcon } from "lucide-react"
import type { ClientHelpArticleSummary, IdentityTokenSource, YuvaClient } from "useyuva"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useYuvaHelp, useYuvaHelpArticle, type YuvaHelpState } from "@/hooks/use-yuva-help"
import { cn } from "@/lib/utils"
import { useYuvaHelpLabels, type YuvaHelpLabels } from "@/lib/yuva-help-utils"

export interface YuvaHelpArticleProps {
  id: string
  server?: string
  channel?: string
  client?: YuvaClient
  identityToken?: IdentityTokenSource | null
  locale?: string
  help?: YuvaHelpState
  search?: string
  onSelect?: (article: ClientHelpArticleSummary) => void
  onAsk?: (articleId: string) => void
  labels?: Partial<YuvaHelpLabels>
  className?: string
}

const prose = cn(
  "max-w-none break-words leading-relaxed [&_a]:text-primary [&_a]:underline [&_a]:underline-offset-4",
  "[&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-[0.85em]",
  "[&_h2]:mt-6 [&_h2]:mb-2 [&_h2]:text-lg [&_h2]:font-semibold [&_h3]:mt-5 [&_h3]:mb-2 [&_h3]:font-semibold",
  "[&_img]:h-auto [&_img]:max-w-full [&_img]:rounded-lg [&_ol]:mb-3 [&_ol]:list-decimal [&_ol]:ps-5",
  "[&_p]:mb-3 [&_pre]:mb-3 [&_pre]:overflow-x-auto [&_pre]:rounded-lg [&_pre]:bg-muted [&_pre]:p-3 [&_pre_code]:bg-transparent [&_pre_code]:p-0",
  "[&_table]:mb-3 [&_table]:block [&_table]:overflow-x-auto [&_td]:border [&_td]:px-2 [&_td]:py-1 [&_th]:border [&_th]:px-2 [&_th]:py-1",
  "[&_ul]:mb-3 [&_ul]:list-disc [&_ul]:ps-5"
)

export function YuvaHelpArticle({
  id,
  server,
  channel,
  client,
  identityToken,
  locale,
  help: shared,
  search,
  onSelect,
  onAsk,
  labels: overrides,
  className,
}: YuvaHelpArticleProps) {
  const labels = useYuvaHelpLabels(overrides)
  const own = useYuvaHelp({
    server: shared ? undefined : server,
    channel,
    client: shared ? undefined : client,
    identityToken,
    locale,
  })
  const help = shared ?? own
  const { article, loading, error, rating, rate, reload } = useYuvaHelpArticle(help, id)
  const [failed, setFailed] = React.useState(false)
  const body = React.useRef<HTMLDivElement>(null)

  React.useEffect(() => {
    for (const link of body.current?.querySelectorAll("a") ?? []) {
      link.target = "_blank"
      link.rel = "noopener noreferrer"
    }
  }, [article])

  if (error)
    return (
      <div className={cn("flex flex-col items-start gap-3", className)}>
        <p className="font-medium">{labels.loadError}</p>
        <Button type="button" onClick={reload}>
          {labels.retry}
        </Button>
      </div>
    )
  if (loading || !article)
    return (
      <div role="status" aria-label={labels.loading} className={cn("flex flex-col gap-3", className)}>
        <Skeleton className="h-7 w-2/3" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-5/6" />
      </div>
    )

  const choose = (value: "up" | "down") => {
    setFailed(false)
    rate(value, search).catch(() => setFailed(true))
  }

  return (
    <article className={cn("flex flex-col gap-8", className)}>
      <div>
        <h1 dir="auto" className="mb-4 text-2xl font-semibold tracking-tight">
          {article.title}
        </h1>
        {/* The server returns sanitized HTML. */}
        <div ref={body} dir="auto" className={prose} dangerouslySetInnerHTML={{ __html: article.body_html }} />
      </div>
      <div
        role="group"
        aria-label={labels.helpfulQuestion}
        className="flex flex-wrap items-center gap-x-3 gap-y-2 text-sm"
      >
        <span className="font-medium">{labels.helpfulQuestion}</span>
        <div className="flex gap-2">
          <Button
            type="button"
            size="sm"
            variant={rating === "up" ? "secondary" : "outline"}
            aria-pressed={rating === "up"}
            onClick={() => choose("up")}
          >
            <ThumbsUpIcon data-icon="inline-start" />
            {labels.yes}
          </Button>
          <Button
            type="button"
            size="sm"
            variant={rating === "down" ? "secondary" : "outline"}
            aria-pressed={rating === "down"}
            onClick={() => choose("down")}
          >
            <ThumbsDownIcon data-icon="inline-start" />
            {labels.no}
          </Button>
        </div>
        {rating && !failed && <span className="text-muted-foreground">{labels.thanks}</span>}
        {failed && <span className="text-destructive">{labels.loadError}</span>}
      </div>
      {article.related.length > 0 && (
        <section className="flex flex-col gap-2">
          <h2 className="font-mono text-[11px] tracking-[0.08em] text-muted-foreground uppercase">{labels.related}</h2>
          <ul className="flex flex-col divide-y border-y">
            {article.related.map((item) => (
              <li key={item.id}>
                <a
                  href={item.url}
                  onClick={(event) => {
                    if (!onSelect) return
                    event.preventDefault()
                    onSelect(item)
                  }}
                  className="flex min-h-12 items-center gap-3 py-2 hover:bg-muted/50"
                >
                  <span dir="auto" className="flex-1 font-medium">
                    {item.title}
                  </span>
                  <ChevronRightIcon aria-hidden className="size-4 shrink-0 text-muted-foreground rtl:rotate-180" />
                </a>
              </li>
            ))}
          </ul>
        </section>
      )}
      {onAsk && (
        <div className="flex items-center gap-3 border-t pt-4">
          <div className="flex flex-1 flex-col">
            <span className="font-medium">{labels.stillStuck}</span>
            <span className="text-xs text-muted-foreground">{labels.askText}</span>
          </div>
          <Button type="button" onClick={() => onAsk(article.id)}>
            {labels.messageUs}
          </Button>
        </div>
      )}
    </article>
  )
}

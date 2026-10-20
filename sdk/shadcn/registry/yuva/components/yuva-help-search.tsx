"use client"

import * as React from "react"
import { ChevronRightIcon, SearchIcon } from "lucide-react"
import type { ClientHelpArticleSummary, IdentityTokenSource, YuvaClient } from "useyuva"
import { Input } from "@/components/ui/input"
import { useYuvaHelp, useYuvaHelpSearch, type YuvaHelpState } from "@/hooks/use-yuva-help"
import { cn } from "@/lib/utils"
import { highlighted, useYuvaHelpLabels, type YuvaHelpLabels } from "@/lib/yuva-help-utils"

export interface YuvaHelpSearchProps {
  server?: string
  channel?: string
  client?: YuvaClient
  identityToken?: IdentityTokenSource | null
  locale?: string
  help?: YuvaHelpState
  query?: string
  onQueryChange?: (query: string) => void
  onSelect?: (article: ClientHelpArticleSummary, query: string) => void
  labels?: Partial<YuvaHelpLabels>
  className?: string
}

export function YuvaHelpSearch({
  server,
  channel,
  client,
  identityToken,
  locale,
  help: shared,
  query: controlled,
  onQueryChange,
  onSelect,
  labels: overrides,
  className,
}: YuvaHelpSearchProps) {
  const labels = useYuvaHelpLabels(overrides)
  const own = useYuvaHelp({
    server: shared ? undefined : server,
    channel,
    client: shared ? undefined : client,
    identityToken,
    locale,
  })
  const help = shared ?? own
  const [typed, setTyped] = React.useState("")
  const query = controlled ?? typed
  const { results, searching, error, searched, submit } = useYuvaHelpSearch(help, query)
  const id = React.useId()

  const change = (value: string) => {
    setTyped(value)
    onQueryChange?.(value)
  }
  const choose = (event: React.MouseEvent, article: ClientHelpArticleSummary) => {
    void help.api?.click({ q: searched, locale: article.locale, article_id: article.id }).catch(() => undefined)
    if (!onSelect) return
    event.preventDefault()
    onSelect(article, searched)
  }

  return (
    <div className={cn("flex flex-col gap-3", className)}>
      <form
        role="search"
        onSubmit={(event) => {
          event.preventDefault()
          submit()
        }}
        className="relative"
      >
        <SearchIcon
          aria-hidden
          className="pointer-events-none absolute start-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
        />
        <Input
          type="search"
          value={query}
          onChange={(event) => change(event.target.value)}
          placeholder={labels.searchPlaceholder}
          aria-label={labels.searchLabel}
          aria-controls={id}
          maxLength={200}
          enterKeyHint="search"
          className="h-11 ps-9"
        />
      </form>
      <div id={id} aria-live="polite">
        {searched && !error && results && results.items.length === 0 && (
          <p className="text-sm text-muted-foreground">{labels.noResults}</p>
        )}
        {error ? <p className="text-sm text-destructive">{labels.loadError}</p> : null}
        {searching && !results ? <p className="text-sm text-muted-foreground">{labels.searching}</p> : null}
        {results && results.items.length > 0 && (
          <ul className="flex flex-col divide-y border-y">
            {results.items.map((article) => (
              <li key={article.id}>
                <a
                  href={article.url}
                  onClick={(event) => choose(event, article)}
                  className="flex min-h-14 items-center gap-3 py-2 hover:bg-muted/50"
                >
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span dir="auto" className="font-medium">
                      {highlighted(article.title, article.highlights, "title")}
                    </span>
                    {article.excerpt && (
                      <span dir="auto" className="line-clamp-2 text-xs text-muted-foreground">
                        {highlighted(article.excerpt, article.highlights, "excerpt")}
                      </span>
                    )}
                  </span>
                  <ChevronRightIcon aria-hidden className="size-4 shrink-0 text-muted-foreground rtl:rotate-180" />
                </a>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

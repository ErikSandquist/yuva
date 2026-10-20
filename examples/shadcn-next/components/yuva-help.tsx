"use client"

import * as React from "react"
import { ArrowLeftIcon, ChevronRightIcon } from "lucide-react"
import type { ClientHelpArticleSummary, ClientHelpCollectionDetail, IdentityTokenSource, YuvaClient } from "useyuva"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { YuvaHelpArticle } from "@/components/yuva-help-article"
import { YuvaHelpSearch } from "@/components/yuva-help-search"
import { useYuvaHelp } from "@/hooks/use-yuva-help"
import { cn } from "@/lib/utils"
import { useYuvaHelpLabels, type YuvaHelpLabels } from "@/lib/yuva-help-utils"

export interface YuvaHelpProps {
  server?: string
  channel?: string
  client?: YuvaClient
  identityToken?: IdentityTokenSource | null
  locale?: string
  page?: string
  articleId?: string
  onAsk?: (articleId?: string) => void
  labels?: Partial<YuvaHelpLabels>
  className?: string
}

type Screen = { kind: "home" } | { kind: "collection"; id: string } | { kind: "article"; id: string; search?: string }

function Eyebrow({ index, children }: { index: string; children: React.ReactNode }) {
  return (
    <h2 className="font-mono text-[11px] tracking-[0.08em] text-muted-foreground uppercase">
      {index} — {children}
    </h2>
  )
}

function Rows({
  items,
  onSelect,
}: {
  items: ClientHelpArticleSummary[]
  onSelect: (item: ClientHelpArticleSummary) => void
}) {
  return (
    <ul className="flex flex-col divide-y border-y">
      {items.map((item) => (
        <li key={item.id}>
          <a
            href={item.url}
            onClick={(event) => {
              event.preventDefault()
              onSelect(item)
            }}
            className="flex min-h-14 items-center gap-3 py-2 hover:bg-muted/50"
          >
            <span className="flex min-w-0 flex-1 flex-col gap-0.5">
              <span dir="auto" className="font-medium">
                {item.title}
              </span>
              {item.excerpt && (
                <span dir="auto" className="line-clamp-2 text-xs text-muted-foreground">
                  {item.excerpt}
                </span>
              )}
            </span>
            <ChevronRightIcon aria-hidden className="size-4 shrink-0 text-muted-foreground rtl:rotate-180" />
          </a>
        </li>
      ))}
    </ul>
  )
}

function Collection({
  id,
  help,
  labels,
  onCollection,
  onSelect,
}: {
  id: string
  help: ReturnType<typeof useYuvaHelp>
  labels: YuvaHelpLabels
  onCollection: (id: string) => void
  onSelect: (item: ClientHelpArticleSummary) => void
}) {
  const [data, setData] = React.useState<ClientHelpCollectionDetail | null>(null)
  const [failed, setFailed] = React.useState(false)
  const [version, setVersion] = React.useState(0)
  const { api, locale } = help
  React.useEffect(() => {
    if (!api) return
    let live = true
    // oxlint-disable-next-line react/set-state-in-effect -- loading from the client is syncing with an external system
    setData(null)
    setFailed(false)
    api.collection(id, { locale }).then(
      (result) => live && setData(result),
      () => live && setFailed(true)
    )
    return () => {
      live = false
    }
  }, [api, id, locale, version])

  if (failed)
    return (
      <div className="flex flex-col items-start gap-3">
        <p className="font-medium">{labels.loadError}</p>
        <Button type="button" onClick={() => setVersion((current) => current + 1)}>
          {labels.retry}
        </Button>
      </div>
    )
  if (!data) return <Skeleton role="status" aria-label={labels.loading} className="h-24 w-full" />
  return (
    <div className="flex flex-col gap-4">
      <div>
        <h1 dir="auto" className="text-2xl font-semibold tracking-tight">
          {data.collection.name}
        </h1>
        {data.collection.description && (
          <p dir="auto" className="mt-1 text-muted-foreground">
            {data.collection.description}
          </p>
        )}
      </div>
      {data.collections.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {data.collections.map((child) => (
            <Button
              key={child.id}
              type="button"
              variant="outline"
              className="rounded-full"
              onClick={() => onCollection(child.id)}
            >
              {child.name}
            </Button>
          ))}
        </div>
      )}
      {data.articles.length > 0 && <Rows items={data.articles} onSelect={onSelect} />}
    </div>
  )
}

export function YuvaHelp({
  server,
  channel,
  client,
  identityToken,
  locale,
  page,
  articleId,
  onAsk,
  labels: overrides,
  className,
}: YuvaHelpProps) {
  const labels = useYuvaHelpLabels(overrides)
  const help = useYuvaHelp({ server, channel, client, identityToken, locale, page })
  const [stack, setStack] = React.useState<Screen[]>([])
  const [screen, setScreen] = React.useState<Screen>(articleId ? { kind: "article", id: articleId } : { kind: "home" })
  const [query, setQuery] = React.useState("")
  const { center, loading, error, reload } = help

  const go = (next: Screen) => {
    setStack((current) => [...current, screen])
    setScreen(next)
  }
  const back = () => {
    setScreen(stack[stack.length - 1] ?? { kind: "home" })
    setStack((current) => current.slice(0, -1))
  }
  const open = (item: ClientHelpArticleSummary, search?: string) => go({ kind: "article", id: item.id, search })
  const roots = center?.collections.filter((collection) => !collection.parent_id) ?? []
  const best = center ? (center.suggested.length > 0 ? center.suggested : center.popular) : []

  return (
    <section aria-label={labels.heading} className={cn("flex w-full flex-col gap-6", className)}>
      {screen.kind !== "home" && (
        <div>
          <Button type="button" variant="ghost" size="sm" onClick={back}>
            <ArrowLeftIcon data-icon="inline-start" className="rtl:rotate-180" />
            {labels.back}
          </Button>
        </div>
      )}
      {screen.kind === "home" && (
        <>
          <YuvaHelpSearch
            help={help}
            query={query}
            onQueryChange={setQuery}
            onSelect={(item, search) => open(item, search)}
            labels={overrides}
          />
          {query.trim().length < 2 && error ? (
            <div className="flex flex-col items-start gap-3">
              <p className="font-medium">{labels.loadError}</p>
              <Button type="button" onClick={reload}>
                {labels.retry}
              </Button>
            </div>
          ) : null}
          {query.trim().length < 2 && loading && !error && (
            <Skeleton role="status" aria-label={labels.loading} className="h-40 w-full" />
          )}
          {query.trim().length < 2 && center && (
            <>
              {best.length > 0 && (
                <div className="flex flex-col gap-2">
                  <Eyebrow index="01">{center.suggested.length > 0 ? labels.bestMatches : labels.popular}</Eyebrow>
                  <Rows items={best} onSelect={(item) => open(item)} />
                </div>
              )}
              {roots.length > 0 && (
                <div className="flex flex-col gap-3">
                  <Eyebrow index={best.length > 0 ? "02" : "01"}>{labels.browse}</Eyebrow>
                  <div className="flex flex-wrap gap-2">
                    {roots.map((collection) => (
                      <Button
                        key={collection.id}
                        type="button"
                        variant="outline"
                        className="rounded-full"
                        onClick={() => go({ kind: "collection", id: collection.id })}
                      >
                        {collection.name}
                        <span className="font-mono text-[11px] text-muted-foreground">{collection.article_count}</span>
                      </Button>
                    ))}
                  </div>
                </div>
              )}
            </>
          )}
          {onAsk && (
            <div className="flex items-center gap-3 border-t pt-4">
              <div className="flex flex-1 flex-col">
                <span className="font-medium">{labels.stillStuck}</span>
                <span className="text-xs text-muted-foreground">{labels.askText}</span>
              </div>
              <Button type="button" onClick={() => onAsk()}>
                {labels.messageUs}
              </Button>
            </div>
          )}
        </>
      )}
      {screen.kind === "collection" && (
        <Collection
          id={screen.id}
          help={help}
          labels={labels}
          onCollection={(id) => go({ kind: "collection", id })}
          onSelect={(item) => open(item)}
        />
      )}
      {screen.kind === "article" && (
        <YuvaHelpArticle
          key={screen.id}
          id={screen.id}
          help={help}
          search={screen.search}
          onSelect={(item) => open(item)}
          onAsk={onAsk}
          labels={overrides}
        />
      )}
    </section>
  )
}

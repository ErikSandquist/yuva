import { Trans, useLingui } from "@lingui/react/macro"
import { useState } from "react"

import { Command, CommandEmpty, CommandInput, CommandItem, CommandList } from "@/components/ui/command"
import { useHelpArticles, useHelpSearch } from "@/features/help/queries"
import { useDebounced } from "@/hooks/use-debounced"
import type { HelpCenter } from "@/lib/api"

export type HelpPick = { id: string; title: string }

type Option = { id: string; title: string; excerpt?: string; locale?: string }

export function ArticlePicker({
  center,
  chosen,
  onPick,
}: {
  center: HelpCenter
  chosen: HelpPick[]
  onPick: (a: HelpPick) => void
}) {
  const { t } = useLingui()
  const [q, setQ] = useState("")
  const query = useDebounced(q.trim(), 250)
  const found = useHelpSearch(center.id, query, "", false)
  const browse = useHelpArticles(center.id, { state: "published" })
  const options: Option[] = query
    ? (found.data?.items ?? []).map((r) => ({ id: r.article_id, title: r.title, excerpt: r.excerpt, locale: r.locale }))
    : (browse.data?.pages[0]?.items ?? []).map((a) => {
        const tr = a.translations.find((x) => x.locale === center.default_locale && x.state !== "draft")
        const any = tr ?? a.translations.find((x) => x.state !== "draft")
        return { id: a.id, title: any?.title ?? "", locale: any?.locale }
      })
  const shown = options.filter((o) => o.title && !chosen.some((c) => c.id === o.id))
  const full = chosen.length >= 3
  return (
    <Command shouldFilter={false} className="bg-transparent p-1.5" label={t`Help articles`}>
      <CommandInput
        value={q}
        onValueChange={setQ}
        placeholder={t`Search help articles…`}
        data-testid="article-search"
      />
      <CommandList className="max-h-72">
        <CommandEmpty className="px-3 py-6 text-faint">
          {full ? <Trans>A message takes at most 3 articles.</Trans> : <Trans>No published article found.</Trans>}
        </CommandEmpty>
        {!full &&
          shown.map((o) => (
            <CommandItem
              key={o.id}
              value={o.id}
              onSelect={() => onPick({ id: o.id, title: o.title })}
              className="rounded-lg px-2.5 py-2 data-selected:bg-muted"
              data-testid="article-option"
            >
              <span className="min-w-0 flex-1">
                <span className="block truncate font-medium">{o.title}</span>
                {o.excerpt && <small className="line-clamp-2 block text-caption text-faint">{o.excerpt}</small>}
              </span>
            </CommandItem>
          ))}
      </CommandList>
    </Command>
  )
}

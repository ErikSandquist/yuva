import * as React from "react"
import type { HelpHighlight } from "useyuva"

export interface YuvaHelpLabels {
  heading: string
  searchLabel: string
  searchPlaceholder: string
  bestMatches: string
  popular: string
  browse: string
  noResults: string
  searching: string
  helpfulQuestion: string
  yes: string
  no: string
  thanks: string
  related: string
  back: string
  stillStuck: string
  askText: string
  messageUs: string
  loadError: string
  retry: string
  loading: string
  articles: (count: number) => string
}

export const defaultYuvaHelpLabels: YuvaHelpLabels = {
  heading: "Help center",
  searchLabel: "Search for help",
  searchPlaceholder: "Search for help",
  bestMatches: "Best matches",
  popular: "Popular articles",
  browse: "Browse",
  noResults: "No articles found.",
  searching: "Searching…",
  helpfulQuestion: "Was this helpful?",
  yes: "Yes",
  no: "No",
  thanks: "Thanks for your feedback",
  related: "Related articles",
  back: "Back",
  stillStuck: "Still stuck?",
  askText: "Ask the team, we read every message.",
  messageUs: "Message us",
  loadError: "Could not load this.",
  retry: "Try again",
  loading: "Loading…",
  articles: (count) => `${count} ${count === 1 ? "article" : "articles"}`,
}

export function useYuvaHelpLabels(overrides?: Partial<YuvaHelpLabels>): YuvaHelpLabels {
  return React.useMemo(() => ({ ...defaultYuvaHelpLabels, ...overrides }), [overrides])
}

export function highlighted(text: string, highlights: HelpHighlight[], field: HelpHighlight["field"]): React.ReactNode {
  const ranges = highlights.filter((h) => h.field === field && h.end > h.start).sort((a, b) => a.start - b.start)
  const parts: React.ReactNode[] = []
  let at = 0
  for (const { start, end } of ranges) {
    if (start < at) continue
    if (start > at) parts.push(text.slice(at, start))
    parts.push(
      <mark key={start} className="rounded-sm bg-primary/15 text-inherit">
        {text.slice(start, end)}
      </mark>
    )
    at = end
  }
  if (at < text.length) parts.push(text.slice(at))
  return parts
}

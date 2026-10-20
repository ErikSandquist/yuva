import type { components } from "./schema.gen"
import { readStored } from "./types"

const freshWindow = 30 * 24 * 60 * 60 * 1000

export function updatesSince(seen: string | undefined): number {
  return (seen && Date.parse(seen)) || Date.now() - freshWindow
}

export async function countUpdates(
  server: string,
  channel: string,
  inbox: components["schemas"]["ClientInbox"],
  locale: string
): Promise<number> {
  const latest = inbox.whats_new?.latest_published_at
  const since = updatesSince(readStored(channel).whats_new_seen)
  if (!latest || Date.parse(latest) <= since) return 0
  const response = await fetch(
    `${server}/client/v1/channels/${encodeURIComponent(channel)}/whats-new?limit=10&locale=${encodeURIComponent(locale)}`,
    { credentials: "omit" }
  )
  if (!response.ok) return 0
  const feed = (await response.json()) as components["schemas"]["ClientWhatsNew"]
  return feed.items.filter((entry) => Date.parse(entry.published_at) > since).length
}

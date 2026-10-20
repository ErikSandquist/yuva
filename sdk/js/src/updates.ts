import type { components } from "./schema.gen"
import { readStored } from "./types"

const freshWindow = 30 * 24 * 60 * 60 * 1000

export function updatesSince(seen: string | undefined): number {
  return (seen && Date.parse(seen)) || Date.now() - freshWindow
}

async function loadFeed(
  server: string,
  channel: string,
  locale: string,
  cache: RequestCache
): Promise<components["schemas"]["ClientWhatsNew"] | null> {
  const response = await fetch(
    `${server}/client/v1/channels/${encodeURIComponent(channel)}/whats-new?limit=10&locale=${encodeURIComponent(locale)}`,
    { credentials: "omit", cache }
  )
  return response.ok ? ((await response.json()) as components["schemas"]["ClientWhatsNew"]) : null
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
  let feed = await loadFeed(server, channel, locale, "default")
  const held = feed?.changelog.latest_published_at
  if (!held || Date.parse(held) < Date.parse(latest)) feed = await loadFeed(server, channel, locale, "no-cache")
  if (!feed) return 0
  return feed.items.filter((entry) => Date.parse(entry.published_at) > since).length
}

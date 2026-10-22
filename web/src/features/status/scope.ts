import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { usePagesIncidents, useStatusPages } from "@/features/status/queries"
import type { StatusPage } from "@/lib/api"
import { useInboxes } from "@/lib/workspace"

export function useStatusScope() {
  const [inboxId] = useInboxFilter()
  const pages = useStatusPages(inboxId)
  const inboxes = useInboxes().data ?? []
  const byInbox = new Map(inboxes.map((i) => [i.id, i]))
  const items = pages.data ?? []
  const shown = inboxId ? inboxes.filter((i) => i.id === inboxId) : inboxes
  return {
    pages,
    items,
    inboxes: shown,
    inboxOf: (p?: StatusPage) => (p ? byInbox.get(p.inbox_id) : undefined),
  }
}

export function useScopeIncidents(items: StatusPage[]) {
  const ids = items.map((p) => p.id)
  const open = usePagesIncidents(ids, true)
  const all = usePagesIncidents(ids, false)
  const seen = new Set<string>()
  const merged = [...open.data, ...all.data].filter((i) => !seen.has(i.id) && !!seen.add(i.id))
  return { data: merged, isPending: open.isPending || all.isPending, error: open.error ?? all.error }
}

import { useInfiniteQuery, useQuery } from "@tanstack/react-query"
import { useSyncExternalStore } from "react"

import { api, unwrap, type DocsPageSort } from "@/lib/api"
import { keys, type DocsDays } from "@/lib/keys"
import { useSession } from "@/lib/session"

export const WINDOWS: DocsDays[] = [7, 30, 90]

export const SORTS: DocsPageSort[] = ["down", "up", "helpful", "activity"]

function store<T>(initial: T) {
  let value = initial
  const listeners = new Set<() => void>()
  const subscribe = (l: () => void) => {
    listeners.add(l)
    return () => listeners.delete(l)
  }
  const set = (v: T) => {
    value = v
    listeners.forEach((l) => l())
  }
  return () => [useSyncExternalStore(subscribe, () => value), set] as const
}

export const useDocsDays = store<DocsDays>(30)
export const useDocsSort = store<DocsPageSort>("down")
export const useDocsSite = store<string>("")

export function docsPageLink(inboxId: string, page: string) {
  return `/docs/page?${new URLSearchParams({ inbox: inboxId, url: page })}`
}

export function useDocsPages(inboxId: string, site: string, d: DocsDays, sort: DocsPageSort) {
  const { workspaceId: ws } = useSession()
  return useInfiniteQuery({
    queryKey: keys.docsPages(ws, inboxId, site, d, sort),
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET("/v1/docs/pages", {
          params: {
            query: {
              inbox_id: inboxId || undefined,
              site: site || undefined,
              days: d,
              sort,
              cursor: pageParam,
              limit: 100,
            },
          },
        })
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
  })
}

export function useDocsSummary(inboxId: string, site: string, d: DocsDays) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.docsSummary(ws, inboxId, site, d),
    queryFn: () =>
      unwrap(
        api.GET("/v1/docs/summary", {
          params: { query: { inbox_id: inboxId || undefined, site: site || undefined, days: d } },
        })
      ),
  })
}

export function useDocsPage(inboxId: string, page: string, site: string, d: DocsDays) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.docsPage(ws, inboxId, page, site, d),
    queryFn: () =>
      unwrap(
        api.GET("/v1/docs/page", { params: { query: { inbox_id: inboxId, page, site: site || undefined, days: d } } })
      ),
    enabled: !!inboxId && !!page,
  })
}

export function useDocsSites(inboxId: string, d: DocsDays) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.docsSites(ws, inboxId, d),
    queryFn: () =>
      unwrap(api.GET("/v1/docs/sites", { params: { query: { inbox_id: inboxId || undefined, days: d } } })),
  })
}

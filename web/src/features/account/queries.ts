import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { api, unwrap, type ReaderAccount } from "@/lib/api"

const root = ["account"] as const

export const accountKeys = {
  root,
  account: [...root, "me"] as const,
  discussions: [...root, "discussions"] as const,
  following: [...root, "following"] as const,
  notifications: (unread: boolean) => [...root, "notifications", unread] as const,
}

export function threadOnPage(page: string, id: string) {
  return `${page}#yuva-discussion-${id}`
}

export function useReaderAccount(enabled = true) {
  return useQuery({
    queryKey: accountKeys.account,
    queryFn: () => unwrap(api.GET("/v1/account")),
    enabled,
  })
}

export function useUpdateReaderAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { name?: string; email_notifications?: boolean }) => unwrap(api.PATCH("/v1/account", { body })),
    onSuccess: (data: ReaderAccount) => {
      qc.setQueryData(accountKeys.account, data)
      void qc.invalidateQueries({ queryKey: ["me"] })
    },
  })
}

export function useReaderDiscussions(kind: "discussions" | "following") {
  return useInfiniteQuery({
    queryKey: accountKeys[kind],
    queryFn: ({ pageParam }) =>
      unwrap(
        kind === "discussions"
          ? api.GET("/v1/account/discussions", { params: { query: { cursor: pageParam, limit: 50 } } })
          : api.GET("/v1/account/following", { params: { query: { cursor: pageParam, limit: 50 } } })
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
  })
}

export function useUnfollow() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.DELETE("/v1/account/following/{discussionId}", { params: { path: { discussionId: id } } })),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: accountKeys.following })
      void qc.invalidateQueries({ queryKey: accountKeys.discussions })
    },
  })
}

export function useReaderNotifications(unread: boolean) {
  return useInfiniteQuery({
    queryKey: accountKeys.notifications(unread),
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET("/v1/account/notifications", {
          params: { query: { unread: unread || undefined, cursor: pageParam, limit: 50 } },
        })
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
  })
}

export function useMarkNotificationsRead() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (ids?: string[]) => unwrap(api.POST("/v1/account/notifications/read", { body: ids ? { ids } : {} })),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...root, "notifications"] }),
  })
}

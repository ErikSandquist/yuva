import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  api,
  unwrap,
  type DiscussionDetail,
  type IdeaStage,
  type RoadmapBoard,
  type RoadmapBoardCreate,
  type RoadmapBoardUpdate,
} from "@/lib/api"
import { keys, type IdeaFilters } from "@/lib/keys"
import { useSession } from "@/lib/session"

export const boardLink = (boardId: string, tab?: string) => `/board/${boardId}${tab ? `?tab=${tab}` : ""}`

export const ideaLink = (boardId: string, ideaId: string) => `/board/${boardId}/ideas/${ideaId}`

export const progressOf = {
  planned: "planned",
  in_progress: "in_progress",
  shipped: "done",
  closed: "wont_do",
} as const

function useRefresh() {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: keys.roadmap(ws) }),
      qc.invalidateQueries({ queryKey: keys.discussions(ws) }),
      qc.invalidateQueries({ queryKey: keys.conversationLists(ws) }),
    ])
}

export function useBoards(enabled = true) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.roadmapBoards(ws),
    queryFn: () => unwrap(api.GET("/v1/roadmap-boards")).then((r) => r.items),
    enabled,
    retry: false,
  })
}

export function useBoard(id: string) {
  const boards = useBoards(!!id)
  return { ...boards, data: boards.data?.find((b) => b.id === id) }
}

export function useCreateBoard() {
  const refresh = useRefresh()
  return useMutation({
    mutationFn: (body: RoadmapBoardCreate) => unwrap(api.POST("/v1/roadmap-boards", { body })),
    onSuccess: refresh,
  })
}

export function useUpdateBoard(id: string) {
  const refresh = useRefresh()
  return useMutation({
    mutationFn: (body: RoadmapBoardUpdate) =>
      unwrap(api.PATCH("/v1/roadmap-boards/{boardId}", { params: { path: { boardId: id } }, body })),
    onSuccess: refresh,
  })
}

export function useDeleteBoard(id: string) {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return useMutation({
    mutationFn: () => unwrap(api.DELETE("/v1/roadmap-boards/{boardId}", { params: { path: { boardId: id } } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.roadmap(ws) }),
  })
}

export function useIdeas(f: IdeaFilters, enabled = true) {
  const { workspaceId: ws } = useSession()
  return useInfiniteQuery({
    queryKey: keys.roadmapIdeas(ws, f),
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET("/v1/discussions", {
          params: { query: { kind: "idea", sort: "votes", ...f, cursor: pageParam, limit: 25 } },
        })
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
    enabled,
  })
}

export function useScopeIdeas(boardId: string | undefined, f: { stage?: IdeaStage; state: "published" | "pending" }) {
  const { workspaceId: ws } = useSession()
  return useInfiniteQuery({
    queryKey: [...keys.roadmap(ws), "scope", boardId ?? "", f],
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET("/v1/discussions", {
          params: { query: { kind: "idea", sort: "votes", board_id: boardId, ...f, cursor: pageParam, limit: 25 } },
        })
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
  })
}

export function useCreateIdea() {
  const refresh = useRefresh()
  return useMutation({
    mutationFn: (body: { board_id: string; title: string; body: string }) =>
      unwrap(api.POST("/v1/discussions", { body: { kind: "idea", ...body } })),
    onSuccess: refresh,
  })
}

export function useBoardIdeas(boardId: string) {
  const f = (stage: IdeaStage) => ({ board_id: boardId, stage, state: "published" as const })
  return {
    ideas: useIdeas(f("ideas")),
    planned: useIdeas(f("planned")),
    in_progress: useIdeas(f("in_progress")),
    shipped: useIdeas(f("shipped")),
    closed: useIdeas(f("closed")),
  }
}

export function useIdeaSearch(boardId: string, q: string) {
  const { workspaceId: ws } = useSession()
  const text = q.trim()
  return useQuery({
    queryKey: keys.roadmapIdeas(ws, { board_id: boardId, state: "published", q: text }),
    queryFn: () =>
      unwrap(
        api.GET("/v1/discussions", {
          params: {
            query: {
              kind: "idea",
              board_id: boardId,
              state: "published",
              sort: "votes",
              q: text || undefined,
              limit: 20,
            },
          },
        })
      ).then((r) => r.items),
    enabled: !!boardId,
    placeholderData: (old) => old,
  })
}

export function useMoveStage(id: string) {
  const refresh = useRefresh()
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  return useMutation({
    mutationFn: (v: { stage: IdeaStage; comment?: string }) =>
      unwrap(api.POST("/v1/discussions/{discussionId}/stage", { params: { path: { discussionId: id } }, body: v })),
    onSuccess: (r) => {
      qc.setQueryData<DiscussionDetail>(keys.discussion(ws, id), (old) => old && { ...old, discussion: r.discussion })
      return refresh()
    },
  })
}

export function useStageChanges(id: string) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.roadmapChanges(ws, id),
    queryFn: () =>
      unwrap(api.GET("/v1/discussions/{discussionId}/stage-changes", { params: { path: { discussionId: id } } })).then(
        (r) => r.items
      ),
    enabled: !!id,
  })
}

export function useLinkIdea() {
  const refresh = useRefresh()
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  const done = (conversationId: string) => {
    void qc.invalidateQueries({ queryKey: keys.conversation(ws, conversationId) })
    return refresh()
  }
  return {
    link: useMutation({
      mutationFn: (v: { ideaId: string; conversationId: string }) =>
        unwrap(
          api.PUT("/v1/discussions/{discussionId}/conversations/{conversationId}", {
            params: { path: { discussionId: v.ideaId, conversationId: v.conversationId } },
          })
        ),
      onSuccess: (_, v) => done(v.conversationId),
    }),
    unlink: useMutation({
      mutationFn: (v: { ideaId: string; conversationId: string }) =>
        unwrap(
          api.DELETE("/v1/discussions/{discussionId}/conversations/{conversationId}", {
            params: { path: { discussionId: v.ideaId, conversationId: v.conversationId } },
          })
        ),
      onSuccess: (_, v) => done(v.conversationId),
    }),
  }
}

export function useStartIdea(conversationId: string) {
  const refresh = useRefresh()
  return useMutation({
    mutationFn: (body: { title: string; body: string }) =>
      unwrap(
        api.POST("/v1/conversations/{conversationId}/discussion", {
          params: { path: { conversationId } },
          body: { kind: "idea", ...body },
        })
      ),
    onSuccess: refresh,
  })
}

export function useLinkedFeedback(ideaId: string) {
  const { workspaceId: ws } = useSession()
  return useInfiniteQuery({
    queryKey: keys.roadmapLinkedList(ws, ideaId),
    queryFn: ({ pageParam }) =>
      unwrap(api.GET("/v1/conversations", { params: { query: { idea_id: ideaId, cursor: pageParam, limit: 100 } } })),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
    enabled: !!ideaId,
  })
}

export function useReplyToLinked(ideaId: string) {
  return useMutation({
    mutationFn: (body: { body: string; close: boolean }) =>
      unwrap(
        api.POST("/v1/discussions/{discussionId}/linked-replies", { params: { path: { discussionId: ideaId } }, body })
      ),
  })
}

export function useReplyCounts(ideaId: string, replyId: string) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.roadmapReply(ws, ideaId, replyId, "counts"),
    queryFn: () =>
      unwrap(
        api.GET("/v1/discussions/{discussionId}/linked-replies/{replyId}", {
          params: { path: { discussionId: ideaId, replyId }, query: { limit: 1 } },
        })
      ),
    enabled: !!replyId,
    retry: false,
    refetchInterval: (q) => (q.state.data && q.state.data.counts.pending === 0 ? false : 2000),
  })
}

export function useReplyOutcomes(ideaId: string, replyId: string, enabled: boolean) {
  const { workspaceId: ws } = useSession()
  return useInfiniteQuery({
    queryKey: keys.roadmapReply(ws, ideaId, replyId, "items"),
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET("/v1/discussions/{discussionId}/linked-replies/{replyId}", {
          params: { path: { discussionId: ideaId, replyId }, query: { cursor: pageParam, limit: 100 } },
        })
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor,
    enabled,
  })
}

export function boardOfInbox(boards: RoadmapBoard[] | undefined, inboxId: string) {
  return boards?.find((b) => b.inbox_id === inboxId)
}

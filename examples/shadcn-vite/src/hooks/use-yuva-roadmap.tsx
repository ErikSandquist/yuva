"use client"

import * as React from "react"
import type { ClientRoadmap, Discussion, IdeaInput, IdeaStage, YuvaClient } from "useyuva"
import { useYuvaDiscussionsContext, useYuvaReader } from "@/hooks/use-yuva-discussions"
import { defaultYuvaRoadmapLabels, type YuvaRoadmapLabels } from "@/lib/yuva-roadmap-utils"

export function useYuvaRoadmapLabels(overrides?: Partial<YuvaRoadmapLabels>): YuvaRoadmapLabels {
  return React.useMemo(() => ({ ...defaultYuvaRoadmapLabels, ...overrides }), [overrides])
}

export interface RoadmapResult {
  board: ClientRoadmap | null
  loading: boolean
  error: unknown
  reload(): Promise<void>
}

const boards = new WeakMap<YuvaClient, Promise<ClientRoadmap>>()

export function useRoadmap(): RoadmapResult {
  const { client } = useYuvaDiscussionsContext()
  const [state, setState] = React.useState<{ board: ClientRoadmap | null; loading: boolean; error: unknown }>({
    board: null,
    loading: true,
    error: null,
  })

  const load = React.useCallback(
    async (fresh = true) => {
      if (!client) return
      setState((s) => ({ ...s, loading: true, error: null }))
      let request = fresh ? undefined : boards.get(client)
      if (!request) {
        request = client.roadmap()
        request.catch(() => boards.delete(client))
        boards.set(client, request)
      }
      try {
        setState({ board: await request, loading: false, error: null })
      } catch (error) {
        setState((s) => ({ ...s, loading: false, error }))
      }
    },
    [client]
  )

  React.useEffect(() => {
    // oxlint-disable-next-line react/set-state-in-effect -- loading from the client is syncing with an external system
    void load(false)
  }, [load])

  return { ...state, reload: () => load(true) }
}

export type IdeaSort = "top" | "new"

export interface IdeasResult {
  ideas: Discussion[]
  loading: boolean
  error: unknown
  hasMore: boolean
  loadMore(): Promise<void>
  reload(): Promise<void>
  suggest(input: IdeaInput): Promise<Discussion>
  vote(idea: Discussion): Promise<void>
}

export function useIdeas(stage: IdeaStage | undefined, sort: IdeaSort = "top", q?: string): IdeasResult {
  const { client } = useYuvaDiscussionsContext()
  const { writer, signIn } = useYuvaReader()
  const [state, setState] = React.useState<{ items: Discussion[]; cursor?: string; loading: boolean; error: unknown }>({
    items: [],
    loading: true,
    error: null,
  })
  const cursor = React.useRef<string | undefined>(undefined)
  const request = React.useRef(0)

  const load = React.useCallback(
    async (more: boolean) => {
      if (!client) return
      const id = ++request.current
      setState((s) => ({ ...s, items: more ? s.items : [], loading: true, error: null }))
      try {
        const result = await client.listIdeas({ stage, sort, q, cursor: more ? cursor.current : undefined })
        if (id !== request.current) return
        cursor.current = result.next_cursor
        setState((s) => {
          const items = more ? [...s.items] : []
          for (const item of result.items) if (!items.some((d) => d.id === item.id)) items.push(item)
          return { items, cursor: result.next_cursor, loading: false, error: null }
        })
      } catch (error) {
        if (id === request.current) setState((s) => ({ ...s, loading: false, error }))
      }
    },
    [client, stage, sort, q]
  )

  React.useEffect(() => {
    cursor.current = undefined
    // oxlint-disable-next-line react/set-state-in-effect -- loading from the client is syncing with an external system
    void load(false)
    if (!client) return
    return client.on("reader", () => void load(false))
  }, [client, load])

  const suggest = React.useCallback(
    async (input: IdeaInput) => {
      if (!client) throw new Error("Yuva: the client is not ready")
      const idea = await client.suggestIdea(input)
      setState((s) => ({ ...s, items: [idea, ...s.items.filter((d) => d.id !== idea.id)] }))
      return idea
    },
    [client]
  )

  const vote = React.useCallback(
    async (idea: Discussion) => {
      if (!client) throw new Error("Yuva: the client is not ready")
      if (!writer) await signIn()
      const post = await client.voteDiscussionPost(idea.opening_post_id, !idea.voted)
      setState((s) => ({
        ...s,
        items: s.items.map((d) => (d.id === idea.id ? { ...d, votes: post.votes, voted: post.voted } : d)),
      }))
    },
    [client, writer, signIn]
  )

  return {
    ideas: state.items,
    loading: state.loading,
    error: state.error,
    hasMore: !!state.cursor,
    loadMore: () => load(true),
    reload: () => load(false),
    suggest,
    vote,
  }
}

"use client"

import * as React from "react"
import {
  createYuvaClient,
  type ClientDiscussionCreate,
  type ClientDiscussionPostUpdate,
  type Discussion,
  type DiscussionDetail,
  type DiscussionMode,
  type DiscussionPost,
  type DiscussionSort,
  type IdentityTokenSource,
  type ReaderSession,
  type YuvaClient,
} from "useyuva"
import { defaultYuvaDiscussionsLabels, type YuvaDiscussionsLabels } from "@/lib/yuva-utils"

interface YuvaDiscussionsContextValue {
  client: YuvaClient | null
  labels: YuvaDiscussionsLabels
  locale?: string
}

const YuvaDiscussionsContext = React.createContext<YuvaDiscussionsContextValue | null>(null)

export interface YuvaDiscussionsProviderProps {
  server?: string
  channel?: string
  identityToken?: IdentityTokenSource | null
  client?: YuvaClient
  labels?: Partial<YuvaDiscussionsLabels>
  locale?: string
  children?: React.ReactNode
}

export function YuvaDiscussionsProvider({
  server,
  channel,
  identityToken,
  client,
  labels,
  locale,
  children,
}: YuvaDiscussionsProviderProps) {
  const identity = React.useRef(identityToken)
  React.useEffect(() => {
    identity.current = identityToken
  })
  const [own, setOwn] = React.useState<YuvaClient | null>(null)

  React.useEffect(() => {
    if (client || !server || !channel) {
      // oxlint-disable-next-line react/set-state-in-effect -- the client is made after mount, so the server and the first client render agree
      setOwn(null)
      return
    }
    setOwn(createYuvaClient({ server, channel, identityToken: () => identity.current?.() ?? null }))
  }, [client, server, channel])

  const value = React.useMemo(
    () => ({ client: client ?? own, labels: { ...defaultYuvaDiscussionsLabels, ...labels }, locale }),
    [client, own, labels, locale]
  )
  return <YuvaDiscussionsContext.Provider value={value}>{children}</YuvaDiscussionsContext.Provider>
}

export function useYuvaDiscussionsContext(): YuvaDiscussionsContextValue {
  const context = React.useContext(YuvaDiscussionsContext)
  if (!context) throw new Error("Yuva: wrap the discussion components in <YuvaDiscussionsProvider>")
  return context
}

export function useYuvaLabels(overrides?: Partial<YuvaDiscussionsLabels>): YuvaDiscussionsLabels {
  const { labels } = useYuvaDiscussionsContext()
  return React.useMemo(() => (overrides ? { ...labels, ...overrides } : labels), [labels, overrides])
}

export type YuvaWriter = "reader" | "contact" | null

export interface YuvaReaderState {
  reader: ReaderSession | null
  writer: YuvaWriter
  ready: boolean
  signIn(): Promise<ReaderSession>
  signOut(): Promise<void>
}

export function useYuvaReader(): YuvaReaderState {
  const { client } = useYuvaDiscussionsContext()
  const [reader, setReader] = React.useState<ReaderSession | null>(null)
  const [writer, setWriter] = React.useState<YuvaWriter>(null)
  const [ready, setReady] = React.useState(false)

  React.useEffect(() => {
    if (!client) return
    let live = true
    const refresh = () =>
      client
        .discussionWriter()
        .catch(() => null)
        .then((value) => {
          if (!live) return
          setWriter(value)
          setReady(true)
        })
    // oxlint-disable-next-line react/set-state-in-effect -- the reader comes from the client's storage, read after hydration so the server render matches
    setReader(client.reader)
    void refresh()
    const off = client.on("reader", (value) => {
      setReader(value)
      void refresh()
    })
    return () => {
      live = false
      off()
    }
  }, [client])

  const signIn = React.useCallback(() => {
    if (!client) return Promise.reject(new Error("Yuva: the client is not ready"))
    return client.readerSignIn()
  }, [client])
  const signOut = React.useCallback(() => client?.readerSignOut() ?? Promise.resolve(), [client])

  return { reader, writer, ready, signIn, signOut }
}

const modes = new WeakMap<YuvaClient, Promise<DiscussionMode>>()

export function useDiscussionMode(): DiscussionMode | null {
  const { client } = useYuvaDiscussionsContext()
  const [mode, setMode] = React.useState<DiscussionMode | null>(null)
  React.useEffect(() => {
    if (!client) return
    let live = true
    let request = modes.get(client)
    if (!request) {
      request = client.channelSettings().then((settings) => settings.chat.discussions)
      request.catch(() => modes.delete(client))
      modes.set(client, request)
    }
    request.then(
      (value) => live && setMode(value),
      () => live && setMode("off")
    )
    return () => {
      live = false
    }
  }, [client])
  return mode
}

export interface DiscussionsResult {
  discussions: Discussion[]
  loading: boolean
  error: unknown
  hasMore: boolean
  loadMore(): Promise<void>
  reload(): Promise<void>
  start(input: Omit<ClientDiscussionCreate, "page_url"> & { page_url?: string }): Promise<Discussion>
}

export function useDiscussions(page: string | null | undefined, sort: DiscussionSort = "new"): DiscussionsResult {
  const { client } = useYuvaDiscussionsContext()
  const [state, setState] = React.useState<{ items: Discussion[]; cursor?: string; loading: boolean; error: unknown }>({
    items: [],
    loading: true,
    error: null,
  })
  const cursor = React.useRef<string | undefined>(undefined)
  const request = React.useRef(0)

  const load = React.useCallback(
    async (more: boolean) => {
      if (!client || !page) return
      const id = ++request.current
      setState((s) => ({ ...s, loading: true, error: null }))
      try {
        const result = await client.listDiscussions(page, { sort, cursor: more ? cursor.current : undefined })
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
    [client, page, sort]
  )

  React.useEffect(() => {
    cursor.current = undefined
    // oxlint-disable-next-line react/set-state-in-effect -- loading from the client is syncing with an external system
    void load(false)
    if (!client) return
    return client.on("reader", () => void load(false))
  }, [client, load])

  const start = React.useCallback(
    async (input: Omit<ClientDiscussionCreate, "page_url"> & { page_url?: string }) => {
      if (!client) throw new Error("Yuva: the client is not ready")
      const page_url = input.page_url ?? page
      if (!page_url) throw new Error("Yuva: no page")
      const discussion = await client.startDiscussion({ ...input, page_url })
      setState((s) => ({ ...s, items: [discussion, ...s.items.filter((d) => d.id !== discussion.id)] }))
      return discussion
    },
    [client, page]
  )

  return {
    discussions: state.items,
    loading: state.loading,
    error: state.error,
    hasMore: !!state.cursor,
    loadMore: () => load(true),
    reload: () => load(false),
    start,
  }
}

export interface DiscussionResult {
  discussion: Discussion | null
  opening: DiscussionPost | null
  posts: DiscussionPost[]
  loading: boolean
  error: unknown
  reload(): Promise<void>
  reply(body: string, parentPostId?: string): Promise<DiscussionPost>
  vote(post: DiscussionPost): Promise<void>
  follow(following: boolean): Promise<void>
  accept(postId: string | null): Promise<void>
  edit(postId: string, input: ClientDiscussionPostUpdate): Promise<void>
  remove(postId: string): Promise<boolean>
}

export function useDiscussion(id: string | null | undefined): DiscussionResult {
  const { client } = useYuvaDiscussionsContext()
  const [state, setState] = React.useState<{ detail: DiscussionDetail | null; loading: boolean; error: unknown }>({
    detail: null,
    loading: true,
    error: null,
  })
  const request = React.useRef(0)

  const load = React.useCallback(async () => {
    if (!client || !id) return
    const current = ++request.current
    setState((s) => ({ ...s, loading: true, error: null }))
    try {
      const detail = await client.getDiscussion(id)
      if (current === request.current) setState({ detail, loading: false, error: null })
    } catch (error) {
      if (current === request.current) setState((s) => ({ ...s, loading: false, error }))
    }
  }, [client, id])

  const [loadedFor, setLoadedFor] = React.useState({ client, id })
  if (loadedFor.client !== client || loadedFor.id !== id) {
    setLoadedFor({ client, id })
    setState({ detail: null, loading: true, error: null })
  }

  React.useEffect(() => {
    // oxlint-disable-next-line react/set-state-in-effect -- loading from the client is syncing with an external system
    void load()
    if (!client) return
    return client.on("reader", () => void load())
  }, [client, load])

  const need = () => {
    if (!client || !id) throw new Error("Yuva: the client is not ready")
    return client
  }

  const replacePost = (post: DiscussionPost) =>
    setState((s) => {
      if (!s.detail) return s
      const { discussion, posts } = s.detail
      const opening = post.id === discussion.opening_post_id
      return {
        ...s,
        detail: {
          discussion: opening ? { ...discussion, votes: post.votes, voted: post.voted } : discussion,
          posts: posts.map((p) => (p.id === post.id ? post : p)),
        },
      }
    })

  const reply = async (body: string, parentPostId?: string) => {
    const post = await need().replyToDiscussion(id!, { body, parent_post_id: parentPostId })
    setState((s) =>
      s.detail
        ? {
            ...s,
            detail: {
              discussion: { ...s.detail.discussion, reply_count: s.detail.discussion.reply_count + 1, following: true },
              posts: [...s.detail.posts, post],
            },
          }
        : s
    )
    return post
  }

  const vote = async (post: DiscussionPost) => {
    replacePost(await need().voteDiscussionPost(post.id, !post.voted))
  }

  const follow = async (following: boolean) => {
    await need().followDiscussion(id!, following)
    setState((s) =>
      s.detail ? { ...s, detail: { ...s.detail, discussion: { ...s.detail.discussion, following } } } : s
    )
  }

  const accept = async (postId: string | null) => {
    await need().acceptDiscussionAnswer(id!, postId)
    await load()
  }

  const edit = async (postId: string, input: ClientDiscussionPostUpdate) => {
    const post = await need().editDiscussionPost(postId, input)
    if (input.title) await load()
    else replacePost(post)
  }

  const remove = async (postId: string) => {
    await need().deleteDiscussionPost(postId)
    try {
      const detail = await need().getDiscussion(id!)
      setState({ detail, loading: false, error: null })
      return true
    } catch {
      setState({ detail: null, loading: false, error: null })
      return false
    }
  }

  const detail = state.detail
  return {
    discussion: detail?.discussion ?? null,
    opening: detail?.posts.find((p) => p.opening) ?? null,
    posts: detail?.posts.filter((p) => !p.opening) ?? [],
    loading: state.loading,
    error: state.error,
    reload: load,
    reply,
    vote,
    follow,
    accept,
    edit,
    remove,
  }
}

export function usePageRating(page: string | null | undefined, title?: string) {
  const { client } = useYuvaDiscussionsContext()
  const [rating, setRating] = React.useState<"up" | "down" | null>(null)
  const [busy, setBusy] = React.useState(false)
  const key = client ? `yuva:${client.channel}:page-ratings` : null

  React.useEffect(() => {
    if (!key || !page) return
    try {
      // oxlint-disable-next-line react/set-state-in-effect -- localStorage is read after hydration so the server render matches
      setRating((JSON.parse(localStorage.getItem(key) ?? "{}") as Record<string, "up" | "down">)[page] ?? null)
    } catch {
      setRating(null)
    }
  }, [key, page])

  const rate = async (value: "up" | "down") => {
    if (!client || !key || !page || busy || value === rating) return
    setBusy(true)
    try {
      await client.ratePage({ page, title: title || undefined, rating: value, previous: rating ?? undefined })
      setRating(value)
      try {
        const stored = JSON.parse(localStorage.getItem(key) ?? "{}") as Record<string, string>
        delete stored[page]
        stored[page] = value
        const pages = Object.keys(stored)
        for (const old of pages.slice(0, Math.max(0, pages.length - 200))) delete stored[old]
        localStorage.setItem(key, JSON.stringify(stored))
      } catch {}
    } finally {
      setBusy(false)
    }
  }

  return { rating, busy, rate }
}

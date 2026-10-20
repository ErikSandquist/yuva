"use client"

import * as React from "react"
import {
  createYuvaClient,
  type ClientHelp,
  type ClientHelpArticle,
  type ClientHelpSearchResults,
  type Help,
  type IdentityTokenSource,
  type YuvaClient,
} from "useyuva"

const SEARCH_DELAY_MS = 250

export interface UseYuvaHelpOptions {
  server?: string
  channel?: string
  client?: YuvaClient
  identityToken?: IdentityTokenSource | null
  locale?: string
  page?: string
}

export interface YuvaHelpState {
  client: YuvaClient | null
  locale: string | undefined
  center: ClientHelp | null
  loading: boolean
  error: unknown
  reload(): void
  api: Help | null
}

export function useYuvaHelp({
  server,
  channel,
  client: given,
  identityToken,
  locale,
  page,
}: UseYuvaHelpOptions): YuvaHelpState {
  const identity = React.useRef(identityToken)
  React.useEffect(() => {
    identity.current = identityToken
  })
  const [own, setOwn] = React.useState<YuvaClient | null>(null)
  React.useEffect(() => {
    if (given || !server || !channel) {
      // oxlint-disable-next-line react/set-state-in-effect -- the client is made after mount, so the server and the first client render agree
      setOwn(null)
      return
    }
    setOwn(createYuvaClient({ server, channel, identityToken: () => identity.current?.() ?? null }))
  }, [given, server, channel])
  const client = given ?? own

  const [center, setCenter] = React.useState<ClientHelp | null>(null)
  const [loading, setLoading] = React.useState(true)
  const [error, setError] = React.useState<unknown>(null)
  const [version, setVersion] = React.useState(0)

  React.useEffect(() => {
    if (!client) return
    let live = true
    // oxlint-disable-next-line react/set-state-in-effect -- loading from the client is syncing with an external system
    setLoading(true)
    client.help.get({ locale, page }).then(
      (result) => {
        if (!live) return
        setCenter(result)
        setError(null)
        setLoading(false)
      },
      (failure: unknown) => {
        if (!live) return
        setError(failure)
        setLoading(false)
      }
    )
    return () => {
      live = false
    }
  }, [client, locale, page, version])

  const reload = React.useCallback(() => setVersion((current) => current + 1), [])
  return { client, locale, center, loading, error, reload, api: client?.help ?? null }
}

export interface YuvaHelpSearchState {
  results: ClientHelpSearchResults | null
  searching: boolean
  error: unknown
  searched: string
  submit(): void
}

export function useYuvaHelpSearch(
  help: Pick<YuvaHelpState, "api" | "locale">,
  query: string,
  { minLength = 2, limit = 8 }: { minLength?: number; limit?: number } = {}
): YuvaHelpSearchState {
  const { api, locale } = help
  const [results, setResults] = React.useState<ClientHelpSearchResults | null>(null)
  const [searching, setSearching] = React.useState(false)
  const [error, setError] = React.useState<unknown>(null)
  const [searched, setSearched] = React.useState("")
  const [submitted, setSubmitted] = React.useState(0)
  const sequence = React.useRef(0)
  const final = React.useRef(false)
  const term = query.trim()

  React.useEffect(() => {
    const mine = ++sequence.current
    if (!api || term.length < minLength) {
      // oxlint-disable-next-line react/set-state-in-effect -- an emptied query clears what the last one found
      setResults(null)
      setSearching(false)
      setSearched("")
      return
    }
    const run = () => {
      setSearching(true)
      const isFinal = final.current
      final.current = false
      api.search(term, { locale, limit, final: isFinal }).then(
        (found) => {
          if (mine !== sequence.current) return
          setResults(found)
          setSearched(term)
          setError(null)
          setSearching(false)
        },
        (failure: unknown) => {
          if (mine !== sequence.current) return
          setError(failure)
          setSearched(term)
          setSearching(false)
        }
      )
    }
    if (final.current) {
      run()
      return
    }
    const timer = setTimeout(run, SEARCH_DELAY_MS)
    return () => clearTimeout(timer)
  }, [api, locale, term, minLength, limit, submitted])

  const submit = React.useCallback(() => {
    final.current = true
    setSubmitted((current) => current + 1)
  }, [])
  return { results, searching, error, searched, submit }
}

export interface YuvaHelpArticleState {
  article: ClientHelpArticle | null
  loading: boolean
  error: unknown
  rating: "up" | "down" | null
  rate(rating: "up" | "down", search?: string): Promise<void>
  reload(): void
}

export function useYuvaHelpArticle(
  help: Pick<YuvaHelpState, "api" | "locale">,
  id: string | null | undefined
): YuvaHelpArticleState {
  const { api, locale } = help
  const [article, setArticle] = React.useState<ClientHelpArticle | null>(null)
  const [loading, setLoading] = React.useState(true)
  const [error, setError] = React.useState<unknown>(null)
  const [rating, setRating] = React.useState<"up" | "down" | null>(null)
  const [version, setVersion] = React.useState(0)

  React.useEffect(() => {
    if (!api || !id) return
    let live = true
    // oxlint-disable-next-line react/set-state-in-effect -- loading from the client is syncing with an external system
    setLoading(true)
    setArticle(null)
    setRating(null)
    api.article(id, { locale }).then(
      (result) => {
        if (!live) return
        setArticle(result)
        setError(null)
        setLoading(false)
      },
      (failure: unknown) => {
        if (!live) return
        setError(failure)
        setLoading(false)
      }
    )
    return () => {
      live = false
    }
  }, [api, id, locale, version])

  const rate = React.useCallback(
    async (next: "up" | "down", search?: string) => {
      if (!api || !article || next === rating) return
      const previous = rating
      setRating(next)
      try {
        await api.react(article.id, { rating: next, previous: previous ?? undefined, locale: article.locale, search })
      } catch (failure) {
        setRating(previous)
        throw failure
      }
    },
    [api, article, rating]
  )

  const reload = React.useCallback(() => setVersion((current) => current + 1), [])
  return { article, loading, error, rating, rate, reload }
}

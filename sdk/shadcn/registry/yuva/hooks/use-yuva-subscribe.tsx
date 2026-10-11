"use client"

import * as React from "react"
import {
  ApiError,
  createYuvaClient,
  type ClientMailingList,
  type IdentityTokenSource,
  type SubscriptionState,
  type YuvaClient,
} from "useyuva"

const FORM_TOKEN_MIN_AGE_MS = 2100

export interface UseYuvaSubscribeOptions {
  server?: string
  channel?: string
  client?: YuvaClient
  identityToken?: IdentityTokenSource | null
  lists?: string[]
}

export interface YuvaSubscribeInput {
  email: string
  name?: string
  listIds: string[]
  consentText: string
  locale?: string
  website?: string
}

export interface YuvaSubscribeState {
  inboxName: string
  lists: ClientMailingList[]
  states: Record<string, SubscriptionState | undefined>
  identified: boolean
  loading: boolean
  error: unknown
  subscribe(input: YuvaSubscribeInput): Promise<{ confirmation: boolean }>
  setSubscribed(listId: string, on: boolean, consentText: string): Promise<SubscriptionState>
}

function pageUrl(): string {
  return window.location.href.split("#")[0]!.slice(0, 2000)
}

export function useYuvaSubscribe({
  server,
  channel,
  client: given,
  identityToken,
  lists: only,
}: UseYuvaSubscribeOptions): YuvaSubscribeState {
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

  const [inboxName, setInboxName] = React.useState("")
  const [lists, setLists] = React.useState<ClientMailingList[]>([])
  const [states, setStates] = React.useState<Record<string, SubscriptionState | undefined>>({})
  const [identified, setIdentified] = React.useState(false)
  const [loading, setLoading] = React.useState(true)
  const [error, setError] = React.useState<unknown>(null)
  const token = React.useRef({ value: "", at: 0 })
  const wanted = only?.join(",") ?? ""

  React.useEffect(() => {
    if (!client) return
    let live = true
    const pick = (all: ClientMailingList[]) => {
      const ids = wanted.split(",").filter(Boolean)
      return ids.length ? ids.map((id) => all.find((list) => list.id === id)).filter((list) => !!list) : all
    }
    const load = async () => {
      const inbox = client.channelSettings()
      if (await identity.current?.()) {
        try {
          const { items } = await client.subscriptions()
          const offered = pick(items.map((item) => item.list))
          return {
            inbox: await inbox,
            lists: offered,
            states: Object.fromEntries(items.map((item) => [item.list.id, item.state])),
            identified: true,
          }
        } catch (failure) {
          if (!(failure instanceof ApiError) || failure.code !== "identity_required") throw failure
        }
      }
      const { lists: all, form_token } = await client.lists()
      token.current = { value: form_token, at: Date.now() }
      return { inbox: await inbox, lists: pick(all), states: {}, identified: false }
    }
    // oxlint-disable-next-line react/set-state-in-effect -- loading from the client is syncing with an external system
    setLoading(true)
    load().then(
      (result) => {
        if (!live) return
        setInboxName(result.inbox.name)
        setLists(result.lists)
        setStates(result.states)
        setIdentified(result.identified)
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
  }, [client, wanted])

  const subscribe = React.useCallback(
    async (input: YuvaSubscribeInput) => {
      if (!client) throw new ApiError(0, "network")
      const send = async (retry: boolean): Promise<{ confirmation: boolean }> => {
        const wait = token.current.at + FORM_TOKEN_MIN_AGE_MS - Date.now()
        if (wait > 0) await new Promise((resolve) => setTimeout(resolve, wait))
        try {
          return await client.subscribe({
            email: input.email,
            name: input.name || undefined,
            list_ids: input.listIds,
            locale: input.locale,
            page_url: pageUrl(),
            consent_text: input.consentText,
            form_token: token.current.value,
            website: input.website || undefined,
          })
        } catch (failure) {
          if (!retry || !(failure instanceof ApiError) || failure.code !== "form_token") throw failure
          const { form_token } = await client.lists()
          token.current = { value: form_token, at: Date.now() }
          return send(false)
        }
      }
      return send(true)
    },
    [client]
  )

  const setSubscribed = React.useCallback(
    async (listId: string, on: boolean, consentText: string) => {
      if (!client) throw new ApiError(0, "network")
      const state: SubscriptionState = on
        ? (await client.subscribeList(listId, { consent_text: consentText, page_url: pageUrl() })).state
        : (await client.unsubscribeList(listId), "unsubscribed")
      setStates((current) => ({ ...current, [listId]: state }))
      return state
    },
    [client]
  )

  return { inboxName, lists, states, identified, loading, error, subscribe, setSubscribed }
}

"use client"

import * as React from "react"
import type { DiscussionSort, IdentityTokenSource, YuvaClient } from "useyuva"
import { LogOut, MessagesSquare, Plus } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { YuvaDiscussionComposer, YuvaSignIn } from "@/components/yuva-discussion-composer"
import { YuvaDiscussionList } from "@/components/yuva-discussion-list"
import { YuvaDiscussionThread } from "@/components/yuva-discussion-thread"
import {
  YuvaDiscussionsProvider,
  useDiscussionMode,
  useYuvaDiscussionsContext,
  useYuvaLabels,
  useYuvaReader,
} from "@/hooks/use-yuva-discussions"
import { cn } from "@/lib/utils"
import { currentPage, type YuvaDiscussionsLabels } from "@/lib/yuva-utils"

export interface YuvaDiscussionsProps {
  server?: string
  channel?: string
  identityToken?: IdentityTokenSource | null
  client?: YuvaClient
  page?: string
  pageTitle?: string
  defaultSort?: DiscussionSort
  labels?: Partial<YuvaDiscussionsLabels>
  locale?: string
  className?: string
}

export function YuvaDiscussions({
  server,
  channel,
  identityToken,
  client,
  labels,
  locale,
  ...props
}: YuvaDiscussionsProps) {
  if (server || channel || client) {
    return (
      <YuvaDiscussionsProvider
        server={server}
        channel={channel}
        identityToken={identityToken}
        client={client}
        labels={labels}
        locale={locale}
      >
        <Discussions {...props} />
      </YuvaDiscussionsProvider>
    )
  }
  return <Discussions labels={labels} {...props} />
}

type View = { kind: "list" } | { kind: "new" } | { kind: "thread"; id: string }

const hashPrefix = "#yuva-discussion-"

function linkedDiscussion(): string {
  const hash = window.location.hash
  return hash.startsWith(hashPrefix) ? decodeURIComponent(hash.slice(hashPrefix.length)) : ""
}

function clearLink() {
  if (!linkedDiscussion()) return
  const url = new URL(window.location.href)
  url.hash = ""
  window.history.replaceState(window.history.state, "", url.href)
}

function Discussions({
  page: pageProp,
  pageTitle,
  defaultSort = "new",
  labels: overrides,
  className,
}: Omit<YuvaDiscussionsProps, "server" | "channel" | "identityToken" | "client" | "locale">) {
  const labels = useYuvaLabels(overrides)
  const { client } = useYuvaDiscussionsContext()
  const mode = useDiscussionMode()
  const { reader, writer, signOut } = useYuvaReader()
  const [sort, setSort] = React.useState<DiscussionSort>(defaultSort)
  const [view, setView] = React.useState<View>({ kind: "list" })
  const [page, setPage] = React.useState<{ url: string; title: string } | null>(null)
  const top = React.useRef<HTMLDivElement>(null)

  React.useEffect(() => {
    const here = currentPage()
    setPage({ url: pageProp ?? here.url, title: pageTitle ?? here.title })
    setView({ kind: "list" })
  }, [pageProp, pageTitle])

  React.useEffect(() => {
    const open = () => {
      const id = linkedDiscussion()
      if (!id) return
      setView({ kind: "thread", id })
      top.current?.scrollIntoView({ block: "start" })
    }
    open()
    window.addEventListener("hashchange", open)
    return () => window.removeEventListener("hashchange", open)
  }, [pageProp])

  const go = (next: View) => {
    if (next.kind !== "thread") clearLink()
    setView(next)
    top.current?.scrollIntoView({ block: "nearest" })
  }

  if (mode === "off") return null

  return (
    <Card ref={top} className={cn("w-full", className)}>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <MessagesSquare className="size-5" aria-hidden />
          <h2>{labels.heading}</h2>
        </CardTitle>
        {reader && (
          <CardDescription className="flex flex-wrap items-center gap-x-2">
            {labels.signedInAs(reader.reader.name || reader.reader.initials)}
            <Button variant="link" size="sm" className="h-auto px-0" onClick={() => void signOut()}>
              <LogOut data-icon="inline-start" className="rtl:rotate-180" />
              {labels.signOut}
            </Button>
          </CardDescription>
        )}
        {view.kind === "list" && client && (
          <CardAction>
            <Button size="sm" onClick={() => go({ kind: "new" })}>
              <Plus data-icon="inline-start" />
              {labels.newDiscussion}
            </Button>
          </CardAction>
        )}
      </CardHeader>
      <CardContent>
        {view.kind === "thread" ? (
          <YuvaDiscussionThread
            id={view.id}
            labels={overrides}
            onBack={() => go({ kind: "list" })}
            onGone={() => go({ kind: "list" })}
          />
        ) : view.kind === "new" ? (
          <div className="flex flex-col gap-4">
            <h3 className="text-base font-semibold">{labels.newDiscussion}</h3>
            {writer ? (
              <YuvaDiscussionComposer
                withTitle
                autoFocus
                labels={overrides}
                onCancel={() => go({ kind: "list" })}
                onSubmit={async ({ title, body }) => {
                  if (!client || !page) return
                  const discussion = await client.startDiscussion({
                    page_url: page.url,
                    page_title: page.title || undefined,
                    title,
                    body,
                  })
                  go({ kind: "thread", id: discussion.id })
                }}
              />
            ) : (
              <>
                <YuvaSignIn labels={overrides} />
                <Button variant="ghost" size="sm" className="self-start" onClick={() => go({ kind: "list" })}>
                  {labels.cancel}
                </Button>
              </>
            )}
          </div>
        ) : (
          <YuvaDiscussionList
            page={page?.url}
            sort={sort}
            onSortChange={setSort}
            labels={overrides}
            onSelect={(discussion) => go({ kind: "thread", id: discussion.id })}
          />
        )}
      </CardContent>
    </Card>
  )
}

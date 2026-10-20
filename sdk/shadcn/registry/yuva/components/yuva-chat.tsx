"use client"

import * as React from "react"
import type { IdentityTokenSource, YuvaClient } from "useyuva"
import { ArrowLeftIcon, ListIcon, PlusIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Spinner } from "@/components/ui/spinner"
import { useYuvaChat } from "@/hooks/use-yuva-chat"
import { cn } from "@/lib/utils"
import { YuvaChatComposer } from "@/components/yuva-chat-composer"
import { YuvaChatConversations } from "@/components/yuva-chat-conversations"
import { YuvaChatThread } from "@/components/yuva-chat-thread"
import { chatErrorText, defaultYuvaChatLabels, type YuvaChatLabels } from "@/lib/yuva-chat-utils"

export interface YuvaChatProps {
  server?: string
  channel?: string
  client?: YuvaClient
  identityToken?: IdentityTokenSource | null
  labels?: Partial<YuvaChatLabels>
  locale?: string
  confirmation?: React.ReactNode
  className?: string
}

export function YuvaChat({
  server,
  channel,
  client,
  identityToken,
  labels: custom,
  locale,
  confirmation,
  className,
}: YuvaChatProps) {
  const labels = React.useMemo(() => ({ ...defaultYuvaChatLabels, ...custom }), [custom])
  const chat = useYuvaChat({ server, channel, client, identityToken })
  const [view, setView] = React.useState<"list" | "thread">("thread")
  const decided = React.useRef(false)
  const { loading, conversations, open } = chat

  React.useEffect(() => {
    if (loading || decided.current) return
    decided.current = true
    const latest = conversations[0]
    if (latest && latest.status !== "closed") void open(latest.id)
    // oxlint-disable-next-line react/set-state-in-effect -- the first view depends on what the client loaded
    else setView(latest ? "list" : "thread")
  }, [loading, conversations, open])

  const inbox = chat.inbox
  const presence = inbox?.presence
  const status = presence
    ? presence.available
      ? labels.online
      : labels.away
    : inbox?.expected_reply_minutes
      ? labels.repliesWithin(inbox.expected_reply_minutes)
      : null

  return (
    <section
      aria-label={labels.title}
      className={cn(
        "flex h-[36rem] max-h-[80dvh] w-full max-w-md flex-col overflow-hidden rounded-xl border bg-card text-card-foreground",
        className
      )}
    >
      <header className="flex items-center gap-2 border-b px-3 py-2">
        {view === "thread" && conversations.length > 0 && (
          <Button variant="ghost" size="icon-sm" aria-label={labels.back} onClick={() => setView("list")}>
            <ArrowLeftIcon className="rtl:rotate-180" />
          </Button>
        )}
        <div className="flex min-w-0 flex-1 flex-col">
          <h2 className="truncate text-sm font-medium">
            {view === "list" ? labels.conversations : (inbox?.name ?? labels.title)}
          </h2>
          {view === "thread" && status && (
            <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
              {presence && (
                <span
                  aria-hidden
                  className={cn("size-1.5 rounded-full", presence.available ? "bg-primary" : "bg-muted-foreground/50")}
                />
              )}
              {status}
            </p>
          )}
        </div>
        {view === "list" ? (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              void chat.open(null)
              setView("thread")
            }}
          >
            <PlusIcon />
            {labels.newConversation}
          </Button>
        ) : (
          conversations.length > 0 && (
            <Button variant="ghost" size="icon-sm" aria-label={labels.conversations} onClick={() => setView("list")}>
              <ListIcon />
            </Button>
          )
        )}
      </header>
      {chat.connection === "reconnecting" && (
        <p role="status" className="flex items-center gap-2 border-b bg-muted px-3 py-1 text-xs text-muted-foreground">
          <Spinner className="size-3" />
          {labels.reconnecting}
        </p>
      )}
      {loading ? (
        <div className="flex flex-1 items-center justify-center">
          <Spinner aria-label={labels.loading} />
        </div>
      ) : chat.error !== null ? (
        <div role="alert" className="flex flex-1 flex-col items-center justify-center gap-2 p-6 text-center text-sm">
          <p>{chat.error ? chatErrorText(chat.error, labels) : labels.loadError}</p>
        </div>
      ) : view === "list" ? (
        <YuvaChatConversations chat={chat} labels={labels} locale={locale} onOpen={() => setView("thread")} />
      ) : (
        <>
          <YuvaChatThread chat={chat} labels={labels} locale={locale} greeting={inbox?.chat.greeting} />
          {confirmation}
          <YuvaChatComposer
            chat={chat}
            labels={labels}
            locale={locale}
            className="border-t p-3"
            disabled={chat.threadLoading}
          />
        </>
      )}
    </section>
  )
}

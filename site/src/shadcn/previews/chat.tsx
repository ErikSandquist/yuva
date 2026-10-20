import * as React from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { YuvaChat } from "@/components/yuva-chat"
import { YuvaChatComposer } from "@/components/yuva-chat-composer"
import { YuvaChatEmail } from "@/components/yuva-chat-email"
import { YuvaChatConversations } from "@/components/yuva-chat-conversations"
import { YuvaChatThread } from "@/components/yuva-chat-thread"
import { useYuvaChat, type YuvaChatState } from "@/hooks/use-yuva-chat"
import { createChatFixture } from "../fixtures"

function useFixtureChat(openFirst = false): YuvaChatState {
  const [client] = React.useState(createChatFixture)
  const chat = useYuvaChat({ client })
  const { loading, conversations, open } = chat
  const opened = React.useRef(false)
  React.useEffect(() => {
    if (!openFirst || loading || opened.current || !conversations[0]) return
    opened.current = true
    void open(conversations[0].id)
  }, [openFirst, loading, conversations, open])
  return chat
}

export function YuvaChatPreview() {
  const [client] = React.useState(createChatFixture)
  const [variant, setVariant] = React.useState<"default" | "lined">("default")
  return (
    <div className="flex w-full flex-col items-center gap-4">
      <div role="group" aria-label="variant" className="flex gap-1 rounded-lg border bg-card p-1">
        {(["default", "lined"] as const).map((v) => (
          <Button key={v} size="sm" variant={v === variant ? "secondary" : "ghost"} onClick={() => setVariant(v)}>
            variant=&quot;{v}&quot;
          </Button>
        ))}
      </div>
      <YuvaChat client={client} variant={variant} className="h-[34rem]" />
    </div>
  )
}

const states = ["ask", "sent", "bad", "confirmed"] as const
const noMessage = [{ id: "m" }]

function emailChat(state: "ask" | "sent" | "bad" | "confirmed"): YuvaChatState {
  const address = "ada@example.com"
  const confirmation =
    state === "sent"
      ? { state: "sent", email: address, expires_at: new Date(Date.now() + 86_400_000).toISOString() }
      : state === "bad"
        ? { state: "undeliverable", email: address }
        : state === "confirmed"
          ? { state: "confirmed", email: address }
          : undefined
  return {
    activeId: "c",
    messages: noMessage,
    inbox: {
      mode: "live",
      presence: { available: false, members: [] },
      chat: { ask_email_offline: true },
    },
    contact: {
      id: "x",
      name: "You",
      identified: false,
      typed_email: state === "ask" ? undefined : address,
      email: state === "confirmed" ? address : undefined,
      email_confirmation: confirmation,
    },
    saveEmail: async () => undefined,
  } as unknown as YuvaChatState
}

export function YuvaChatEmailPreview() {
  const chats = React.useMemo(() => states.map((s) => [s, emailChat(s)] as const), [])
  return (
    <div className="grid w-full max-w-2xl gap-6 sm:grid-cols-2">
      {chats.map(([state, chat]) => (
        <div key={state} className="flex flex-col gap-2 rounded-xl border bg-card p-3 text-sm">
          <Badge variant="outline" className="self-start">
            {state === "bad" ? "undeliverable" : state}
          </Badge>
          <YuvaChatEmail chat={chat} />
        </div>
      ))}
    </div>
  )
}

export function YuvaChatThreadPreview() {
  const chat = useFixtureChat(true)
  return (
    <div className="flex h-96 w-full max-w-md flex-col overflow-hidden rounded-xl border bg-card">
      <YuvaChatThread chat={chat} greeting="Hi, ask us anything about Acme." />
    </div>
  )
}

export function YuvaChatComposerPreview() {
  const chat = useFixtureChat()
  return (
    <div className="w-full max-w-md rounded-xl border bg-card p-3">
      <YuvaChatComposer chat={chat} />
    </div>
  )
}

export function YuvaChatConversationsPreview() {
  const chat = useFixtureChat()
  return (
    <div className="w-full max-w-md overflow-hidden rounded-xl border bg-card">
      <YuvaChatConversations chat={chat} />
    </div>
  )
}

export function UseYuvaChatPreview() {
  const chat = useFixtureChat()
  return (
    <div className="flex w-full max-w-md flex-col gap-3 rounded-xl border bg-card p-4 text-sm">
      <p className="flex items-center gap-2">
        <Badge variant="outline">{chat.loading ? "loading" : chat.connection}</Badge>
        <span className="text-muted-foreground">{chat.inbox?.name}</span>
      </p>
      <ul className="flex flex-col gap-1">
        {chat.conversations.map((c) => (
          <li key={c.id} className="flex items-center gap-2">
            <Button
              variant={c.id === chat.activeId ? "secondary" : "ghost"}
              size="sm"
              className="flex-1 justify-start"
              onClick={() => void chat.open(c.id)}
            >
              {c.subject}
            </Button>
            {c.unread && <Badge>unread</Badge>}
          </li>
        ))}
      </ul>
      {chat.activeId && (
        <>
          <ul className="flex flex-col gap-1 border-t pt-3">
            {chat.messages.map((m) => (
              <li key={m.id} className="flex gap-2">
                <span className="w-12 shrink-0 text-muted-foreground">{m.direction === "in" ? "you" : "team"}</span>
                <span>{m.body}</span>
              </li>
            ))}
          </ul>
          <Button size="sm" className="self-start" onClick={() => void chat.send({ body: "Thanks!" })}>
            send("Thanks!")
          </Button>
        </>
      )}
    </div>
  )
}

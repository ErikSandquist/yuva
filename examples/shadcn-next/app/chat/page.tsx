import type { Metadata } from "next"
import { YuvaChat } from "@/components/yuva-chat"

export const metadata: Metadata = { title: "Chat" }

export default function ChatPage() {
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-3xl font-semibold tracking-tight">Chat</h1>
      <YuvaChat
        server={process.env.NEXT_PUBLIC_YUVA_SERVER}
        channel={process.env.NEXT_PUBLIC_YUVA_CHANNEL}
      />
    </div>
  )
}

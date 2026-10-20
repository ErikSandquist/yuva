"use client"

import * as React from "react"
import { YuvaChat } from "@/components/yuva-chat"
import { YuvaHelp } from "@/components/yuva-help"

export function HelpCenter({ server, channel }: { server?: string; channel?: string }) {
  const [asked, setAsked] = React.useState<string | undefined>()
  const [chat, setChat] = React.useState(false)
  return (
    <div className="flex flex-col gap-10">
      <YuvaHelp
        server={server}
        channel={channel}
        onAsk={(articleId) => {
          setAsked(articleId)
          setChat(true)
        }}
      />
      {chat && <YuvaChat server={server} channel={channel} helpArticleId={asked} />}
    </div>
  )
}

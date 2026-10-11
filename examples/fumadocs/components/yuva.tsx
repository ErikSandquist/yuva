"use client"

import { YuvaDiscussions, YuvaPageFeedback } from "useyuva/react"

const channel = process.env.NEXT_PUBLIC_YUVA_CHANNEL!
const server = process.env.NEXT_PUBLIC_YUVA_SERVER

export function PageDiscussions({ title }: { title: string }) {
  return <YuvaDiscussions channel={channel} server={server} pageTitle={title} />
}

export function PageFeedback({ title }: { title: string }) {
  return <YuvaPageFeedback channel={channel} server={server} pageTitle={title} />
}

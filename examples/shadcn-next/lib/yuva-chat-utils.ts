import { ApiError } from "useyuva"

export interface YuvaChatLabels {
  title: string
  online: string
  away: string
  repliesWithin: (minutes: number) => string
  reconnecting: string
  loading: string
  loadError: string
  retry: string
  back: string
  conversations: string
  newConversation: string
  noConversations: string
  noConversationsHint: string
  loadMore: string
  conversationFallback: string
  unread: string
  emptyTitle: string
  emptyDescription: string
  loadEarlier: string
  messages: string
  today: string
  yesterday: string
  team: string
  bot: string
  seen: string
  sending: string
  sendFailed: string
  remove: string
  download: string
  typing: (name: string) => string
  typingSomeone: string
  jumpToLatest: (unread: number) => string
  closed: string
  rateQuestion: string
  rateGood: string
  rateBad: string
  rated: string
  placeholder: string
  attach: string
  send: string
  enterKey: string
  toSend: string
  generic: string
  offline: string
  rateLimited: string
}

export const defaultYuvaChatLabels: YuvaChatLabels = {
  title: "Chat with us",
  online: "Online",
  away: "Away",
  repliesWithin: (minutes) => `We usually reply within ${minutes} min`,
  reconnecting: "Reconnecting…",
  loading: "Loading",
  loadError: "Could not load the chat.",
  retry: "Try again",
  back: "Back",
  conversations: "Conversations",
  newConversation: "New conversation",
  noConversations: "No conversations yet",
  noConversationsHint: "Start one and we will answer here.",
  loadMore: "Show more",
  conversationFallback: "Conversation",
  unread: "Unread",
  emptyTitle: "Say hello",
  emptyDescription: "Ask us anything. We reply here.",
  loadEarlier: "Show earlier messages",
  messages: "Messages",
  today: "Today",
  yesterday: "Yesterday",
  team: "Support",
  bot: "Assistant",
  seen: "Seen",
  sending: "Sending…",
  sendFailed: "Not sent",
  remove: "Remove",
  download: "Download",
  typing: (name) => `${name} is typing`,
  typingSomeone: "Typing",
  jumpToLatest: (unread) => (unread > 0 ? `${unread} new, jump to latest` : "Jump to latest"),
  closed: "Conversation closed",
  rateQuestion: "How was this conversation?",
  rateGood: "Good",
  rateBad: "Bad",
  rated: "Thanks for your feedback.",
  placeholder: "Write a message",
  attach: "Attach a file",
  send: "Send",
  enterKey: "Enter",
  toSend: "to send",
  generic: "Something went wrong. Try again.",
  offline: "You seem to be offline.",
  rateLimited: "Too many messages. Wait a moment.",
}

export function chatErrorText(error: unknown, labels: YuvaChatLabels): string {
  const code = error instanceof ApiError ? error.code : ""
  if (code === "network") return labels.offline
  if (code === "rate_limited") return labels.rateLimited
  return labels.generic
}

export function initials(name: string | undefined): string {
  const words = (name ?? "").trim().split(/\s+/).filter(Boolean)
  return words
    .slice(0, 2)
    .map((word) => Array.from(word)[0]!.toUpperCase())
    .join("")
}

export function dayKey(iso: string): string {
  const date = new Date(iso)
  return `${date.getFullYear()}-${date.getMonth() + 1}-${date.getDate()}`
}

export function dayLabel(iso: string, labels: YuvaChatLabels, locale?: string): string {
  const key = dayKey(iso)
  const now = new Date()
  if (key === dayKey(now.toISOString())) return labels.today
  const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1)
  if (key === dayKey(yesterday.toISOString())) return labels.yesterday
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium" }).format(new Date(iso))
}

export function timeLabel(iso: string, locale?: string): string {
  return new Intl.DateTimeFormat(locale, { timeStyle: "short" }).format(new Date(iso))
}

export function whenLabel(iso: string, locale?: string): string {
  const today = dayKey(iso) === dayKey(new Date().toISOString())
  return new Intl.DateTimeFormat(locale, today ? { timeStyle: "short" } : { dateStyle: "medium" }).format(new Date(iso))
}

export function sizeLabel(bytes: number, locale?: string): string {
  const units = ["B", "KB", "MB", "GB"]
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit++
  }
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: value < 10 && unit > 0 ? 1 : 0 }).format(value)} ${units[unit]}`
}

export function soleLink(body: string): URL | null {
  const text = body.trim()
  if (!/^https?:\/\/\S+$/i.test(text)) return null
  try {
    return new URL(text)
  } catch {
    return null
  }
}

export function linkParts(body: string): { text: string; href?: string }[] {
  const parts: { text: string; href?: string }[] = []
  let last = 0
  for (const match of body.matchAll(/https?:\/\/[^\s<>"]+[^\s<>".,;:!?)\]]/gi)) {
    if (match.index > last) parts.push({ text: body.slice(last, match.index) })
    parts.push({ text: match[0], href: match[0] })
    last = match.index + match[0].length
  }
  if (last < body.length) parts.push({ text: body.slice(last) })
  return parts
}

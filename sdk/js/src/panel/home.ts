import type { MessageDescriptor } from "@lingui/core"
import { msg } from "@lingui/core/macro"
import type { ClientConversation, ClientInbox } from "../client/api"
import { el, textButton } from "./helpers"
import { icons } from "./icons"

export interface HomeData {
  inbox: ClientInbox
  conversations: ClientConversation[]
  locale: string
  t(descriptor: MessageDescriptor): string
  notice: HTMLElement
  openConversation(id: string): void
  allConversations(): void
  newConversation(): void
}

function section(label: string): HTMLElement {
  const section = el("section", "sec")
  section.append(el("h2", "label", label))
  return section
}

export function buildHome(data: HomeData): HTMLElement {
  const { inbox, t, locale } = data
  const page = el("div", "page home")

  const hero = el("section", "sec hero")
  const members = inbox.mode === "live" && inbox.open_now && inbox.presence?.available ? inbox.presence.members : []
  if (members.length > 0) {
    const avatars = el("div", "avatars")
    for (const member of members.slice(0, 3)) {
      const avatar = el("span", "avatar", member.initials || "•")
      avatar.title = member.name
      avatars.append(avatar)
    }
    hero.append(avatars)
  }
  hero.append(el("h1", "", t(msg`How can we help?`)))
  page.append(data.notice, hero)

  const latest = data.conversations[0]
  const talk = section(`01 — ${latest ? t(msg`Continue`) : t(msg`Talk to us`)}`)
  if (latest) {
    const item = el("button", latest.unread ? "item unread" : "item")
    item.type = "button"
    const at = new Date(latest.last_message_at ?? latest.created_at)
    const main = el("div", "item-main")
    const top = el("div", "item-top")
    top.append(el("span", "item-subject", latest.subject || inbox.name || t(msg`Conversation`)))
    top.append(
      el(
        "span",
        "item-time",
        new Intl.DateTimeFormat(
          locale,
          at.toDateString() === new Date().toDateString() ? { timeStyle: "short" } : { month: "short", day: "numeric" }
        ).format(at)
      )
    )
    const preview = latest.last_message
      ? `${latest.last_message.author_type === "contact" ? `${t(msg`You:`)} ` : ""}${latest.last_message.text || t(msg`Attachment`)}`
      : ""
    const line = el("span", "item-preview", preview.replace(/\*\*|`/g, ""))
    line.dir = "auto"
    main.append(top, line)
    const mark = el("span", "avatar lg", (inbox.name || "").slice(0, 2))
    item.append(mark, main)
    if (latest.unread) item.append(el("span", "unread-dot"))
    item.addEventListener("click", () => data.openConversation(latest.id))
    talk.append(item)
    if (data.conversations.length > 1)
      talk.append(textButton(t(msg`All conversations`), "link brand", () => data.allConversations()))
  }
  const send = el("button", "primary")
  send.type = "button"
  send.append(t(msg`Send us a message`))
  send.insertAdjacentHTML("beforeend", icons.arrow)
  send.addEventListener("click", () => data.newConversation())
  talk.append(send)
  page.append(talk)

  page.append(el("div", "fill"))
  const brand = "Yuva"
  const foot = el("section", "sec foot", t(msg`Powered by ${brand}`))
  page.append(foot)
  return page
}

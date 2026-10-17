import { useLingui } from "@lingui/react/macro"
import { useEffect } from "react"
import { Link, Navigate, useLocation, useNavigate, useParams } from "react-router"

import { ErrorLine } from "@/components/common"
import { useEnumText } from "@/components/common/text"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { AppAccessSection, AppInstallSection } from "@/features/settings/channels/app/AppSections"
import {
  ChatInstallSection,
  DiscussionsSection,
  LookSection,
  SitesSection,
  VisitorsSection,
} from "@/features/settings/channels/chat/ChatSections"
import { DeliverabilityCard, DkimSection } from "@/features/settings/channels/email/Deliverability"
import {
  AutoReplySection,
  BroadcastsSection,
  RepliesSection,
  ReceivingSection,
  SesSection,
} from "@/features/settings/channels/email/EmailSections"
import { emailForm } from "@/features/settings/channels/email/form"
import { sesShown } from "@/features/settings/channels/email/EmailFields"
import { ApiSettingsSection, NameSection, RemoveSection } from "@/features/settings/channels/sections"
import { InboxProvider } from "@/features/settings/inboxes/InboxPage"
import { PageHeader } from "@/features/settings/ui"
import type { Channel } from "@/lib/api"
import { useSession } from "@/lib/session"
import { useChannel } from "@/lib/workspace"

function useHashScroll(ready: boolean) {
  const { hash } = useLocation()
  useEffect(() => {
    if (!ready || !hash) return
    document.getElementById(hash.slice(1))?.scrollIntoView({ block: "start" })
  }, [ready, hash])
}

function address(ch: Channel) {
  if (ch.email) return ch.email.from_address || ch.email.address
  if (ch.chat) return ch.chat.allowed_origins[0]
  return undefined
}

function useSections(ch: Channel) {
  const { t } = useLingui()
  const ses = ch.kind === "email" && sesShown({ f: emailForm(ch.email), ses: ch.email?.ses })
  const items: [string, string][] = [["general", t`General`]]
  if (ch.kind === "email") {
    items.push(["receiving", t`Receiving`], ["replies", t`Replies`])
    if (ses) items.push(["ses", t`Amazon SES`])
    items.push(
      ["broadcasts", t`Broadcast sending`],
      ["deliverability", t`Deliverability`],
      ["dkim", t`DKIM`],
      ["auto-reply", t`Auto-reply`]
    )
  }
  if (ch.kind === "chat")
    items.push(
      ["sites", t`Sites`],
      ["visitors", t`Visitors`],
      ["look", t`Look`],
      ["discussions", t`Discussions`],
      ["install", t`Install`]
    )
  if (ch.kind === "app") items.push(["access", t`Access`], ["install", t`Install`])
  if (ch.kind === "api") items.push(["settings", t`Settings`])
  items.push(["remove", t`Remove`])
  return items
}

function OnThisPage({ items }: { items: [string, string][] }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { hash } = useLocation()
  const labels = Object.fromEntries(items)
  const current = items.some(([id]) => `#${id}` === hash) ? hash.slice(1) : items[0][0]
  return (
    <nav aria-label={t`On this page`} data-testid="channel-sections">
      <ul className="hidden flex-wrap gap-x-4 gap-y-1 sm:flex">
        {items.map(([id, label]) => (
          <li key={id}>
            <Link to={{ hash: id }} replace className="text-body text-muted-foreground hover:text-foreground">
              {label}
            </Link>
          </li>
        ))}
      </ul>
      <div className="sm:hidden">
        <Select value={current} onValueChange={(id) => id && navigate({ hash: id }, { replace: true })} items={labels}>
          <SelectTrigger className="w-full" aria-label={t`On this page`}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {items.map(([id, label]) => (
              <SelectItem key={id} value={id}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </nav>
  )
}

function Sections({ ch }: { ch: Channel }) {
  const items = useSections(ch)
  return (
    <>
      <OnThisPage items={items} />
      <NameSection key={`name:${ch.id}`} channel={ch} />
      {ch.kind === "email" && (
        <>
          <ReceivingSection channel={ch} />
          <RepliesSection channel={ch} />
          <SesSection channel={ch} />
          <BroadcastsSection channel={ch} />
          <DeliverabilityCard channel={ch} />
          <DkimSection key={ch.email?.dkim?.selector ?? "none"} channel={ch} />
          <AutoReplySection channel={ch} />
        </>
      )}
      {ch.kind === "chat" && (
        <>
          <SitesSection channel={ch} />
          <VisitorsSection channel={ch} />
          <LookSection channel={ch} />
          <DiscussionsSection channel={ch} />
          <ChatInstallSection channel={ch} />
        </>
      )}
      {ch.kind === "app" && (
        <>
          <AppAccessSection channel={ch} />
          <AppInstallSection channel={ch} />
        </>
      )}
      {ch.kind === "api" && <ApiSettingsSection channel={ch} />}
      <RemoveSection channel={ch} />
    </>
  )
}

export function ChannelPage() {
  const { inboxId = "", channelId = "" } = useParams()
  const text = useEnumText()
  const { canManage } = useSession()
  const channel = useChannel(channelId)
  const ch = channel.data
  useHashScroll(!!ch)
  if (!canManage || (ch && ch.inbox_id !== inboxId)) return <Navigate to={`/settings/inboxes/${inboxId}`} replace />
  return (
    <InboxProvider>
      {(inbox) =>
        ch ? (
          <>
            <PageHeader
              title={ch.name}
              eyebrow={text.channel[ch.kind]}
              description={<span dir="ltr">{address(ch)}</span>}
              back={{ to: `/settings/inboxes/${inbox.id}`, label: inbox.name }}
            />
            <Sections ch={ch} />
          </>
        ) : channel.isPending ? (
          <Skeleton className="h-64 w-full rounded-2xl" />
        ) : (
          <ErrorLine error={channel.error} />
        )
      }
    </InboxProvider>
  )
}

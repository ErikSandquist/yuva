import { Trans, useLingui } from "@lingui/react/macro"
import { PlusIcon } from "lucide-react"
import { Link, useSearchParams } from "react-router"

import { ChannelIcon, InboxMark } from "@/components/common"
import { Pane } from "@/components/common/Column"
import { formatDuration, useEnumText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { ChannelSummary } from "@/features/settings/channels/ChannelSummary"
import { channelLink } from "@/features/settings/channels/email/deliverability"
import { ChannelStatus } from "@/features/settings/inboxes/InboxPage"
import { useInboxChannels } from "@/features/settings/SettingsColumn"
import { Card, CardHead, EmptyRow, PageHeader, PageTabs, RowIcon } from "@/features/settings/ui"
import type { Channel, Inbox } from "@/lib/api"
import { useSession } from "@/lib/session"
import { useInboxMembers } from "@/lib/workspace"

function language(tag: string, locale: string) {
  try {
    return new Intl.DisplayNames([locale], { type: "language" }).of(tag) ?? tag
  } catch {
    return tag
  }
}

function Fact({ label, children }: { label: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col">
      <dt className="text-caption text-faint">{label}</dt>
      <dd className="truncate text-body">{children}</dd>
    </div>
  )
}

function ProductCard({ inbox, channels }: { inbox: Inbox; channels?: Channel[] }) {
  const { i18n } = useLingui()
  const text = useEnumText()
  const members = useInboxMembers(inbox.id).data
  const reply = inbox.expected_reply_minutes
  const within = reply !== undefined ? formatDuration(reply * 60, i18n.locale) : ""
  const kinds = [...new Set((channels ?? []).map((c) => c.kind))]
  return (
    <li className="flex flex-col overflow-hidden rounded-2xl border bg-card" data-testid="product-card">
      <div className="flex items-center gap-3 border-b px-4 py-3.5">
        <InboxMark name={inbox.name} color={inbox.branding.color} className="size-10 rounded-xl text-body" />
        <div className="min-w-0 flex-1">
          <p className="truncate text-reading font-medium">{inbox.name}</p>
          <p className="truncate text-small text-faint">
            {text.mode[inbox.mode]}
            {inbox.mode === "async" && within && (
              <>
                {" · "}
                <Trans>replies within {within}</Trans>
              </>
            )}
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          render={<Link to={`/settings/inboxes/${inbox.id}`} />}
          data-testid="product-settings"
        >
          <Trans>Settings</Trans>
        </Button>
      </div>
      <dl className="grid grid-cols-2 gap-x-4 gap-y-2.5 px-4 py-3.5">
        <Fact label={<Trans>Language</Trans>}>{language(inbox.default_locale, i18n.locale)}</Fact>
        <Fact label={<Trans>Hours</Trans>}>
          {inbox.business_hours.enabled ? <Trans>Business hours</Trans> : <Trans>Always open</Trans>}
        </Fact>
        <Fact label={<Trans>Members</Trans>}>{members ? members.length : "–"}</Fact>
        <Fact label={<Trans>Ratings</Trans>}>
          {inbox.ask_for_rating ? <Trans>Asked after closing</Trans> : <Trans>Off</Trans>}
        </Fact>
      </dl>
      <div className="mt-auto flex flex-wrap gap-1.5 px-4 pb-4">
        {channels === undefined ? (
          <Skeleton className="h-5 w-24" />
        ) : kinds.length === 0 ? (
          <span className="text-small text-faint">
            <Trans>No channels yet</Trans>
          </span>
        ) : (
          kinds.map((k) => (
            <span
              key={k}
              className="inline-flex items-center gap-1 rounded-md border bg-surface px-1.5 text-caption text-muted-foreground"
            >
              <ChannelIcon kind={k} className="size-3" />
              {text.channel[k]}
            </span>
          ))
        )}
      </div>
    </li>
  )
}

function ChannelGroup({ inbox, channels }: { inbox: Inbox; channels?: Channel[] }) {
  const text = useEnumText()
  const { canManage } = useSession()
  return (
    <Card flush data-testid="channel-group">
      <CardHead
        action={
          canManage && (
            <Button
              variant="ghost"
              size="sm"
              render={<Link to={`/settings/inboxes/${inbox.id}`} />}
              data-testid="channel-group-add"
            >
              <PlusIcon />
              <Trans>Add channel</Trans>
            </Button>
          )
        }
      >
        <span className="flex items-center gap-2">
          <InboxMark name={inbox.name} color={inbox.branding.color} />
          {inbox.name}
        </span>
      </CardHead>
      {channels === undefined ? (
        <div className="p-4">
          <Skeleton className="h-9 w-full" />
        </div>
      ) : channels.length === 0 ? (
        <EmptyRow>
          <Trans>No channels yet. Add one so customers can write to {inbox.name}.</Trans>
        </EmptyRow>
      ) : (
        <ul className="divide-y">
          {channels.map((ch) => (
            <li key={ch.id} className="flex min-h-[56px] items-center gap-3 px-4 py-2.5" data-testid="channel-row">
              <RowIcon>
                <ChannelIcon kind={ch.kind} />
              </RowIcon>
              <div className="min-w-0 flex-1">
                <p className="truncate text-body font-medium">
                  {ch.name}
                  <span className="ms-2 font-normal text-faint">{text.channel[ch.kind]}</span>
                </p>
                <p className="mt-px truncate text-small text-faint">
                  <ChannelSummary ch={ch} />
                </p>
              </div>
              <ChannelStatus inboxId={inbox.id} ch={ch} />
              {canManage && (
                <Button
                  variant="outline"
                  size="sm"
                  render={<Link to={channelLink(inbox.id, ch.id)} />}
                  data-testid="channel-open"
                >
                  <Trans>Open</Trans>
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}

export function ProductsHome() {
  const { t } = useLingui()
  const { canManage } = useSession()
  const [params] = useSearchParams()
  const view = params.get("view") === "channels" ? "channels" : "products"
  const inboxes = useInboxChannels()
  const channelCount = inboxes.reduce((n, i) => n + (i.channels?.length ?? 0), 0)
  const tabs = [
    { key: "products", to: "/settings", label: t`Products`, count: inboxes.length, testId: "settings-tab-products" },
    {
      key: "channels",
      to: "/settings?view=channels",
      label: t`Channels`,
      count: inboxes.every((i) => i.channels) ? channelCount : undefined,
      testId: "settings-tab-channels",
    },
  ]
  return (
    <Pane wide testId="settings-home" className="gap-6">
      <PageHeader
        back={false}
        title={view === "channels" ? <Trans>Channels</Trans> : <Trans>Products</Trans>}
        description={
          view === "channels" ? (
            <Trans>How messages reach each product.</Trans>
          ) : (
            <Trans>Each product has its own inbox, branding, hours and channels.</Trans>
          )
        }
        action={
          canManage && (
            <Button
              size="sm"
              render={<Link to="/setup/new" state={{ back: "/settings" }} />}
              data-testid="products-add"
            >
              <PlusIcon />
              <Trans>Add product</Trans>
            </Button>
          )
        }
        tabs={<PageTabs label={t`Settings`} items={tabs} current={view} />}
      />
      {inboxes.length === 0 ? (
        <Card flush>
          <EmptyRow>
            <Trans>No products yet. Add one for each product or brand you answer for.</Trans>
          </EmptyRow>
        </Card>
      ) : view === "channels" ? (
        <div className="flex flex-col gap-4" data-testid="settings-channels">
          {inboxes.map(({ inbox, channels }) => (
            <ChannelGroup key={inbox.id} inbox={inbox} channels={channels} />
          ))}
        </div>
      ) : (
        <ul className="grid gap-4 lg:grid-cols-2" data-testid="settings-products">
          {inboxes.map(({ inbox, channels }) => (
            <ProductCard key={inbox.id} inbox={inbox} channels={channels} />
          ))}
        </ul>
      )}
    </Pane>
  )
}

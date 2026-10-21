import { Trans, useLingui } from "@lingui/react/macro"
import { Link, useSearchParams } from "react-router"

import { ChannelIcon, InboxMark } from "@/components/common"
import { formatDuration, useEnumText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { ChannelSummary } from "@/features/settings/channels/ChannelSummary"
import { channelLink } from "@/features/settings/channels/email/deliverability"
import { channelState } from "@/features/settings/inboxes/InboxPage"
import { useInboxChannels } from "@/features/settings/SettingsColumn"
import { Card, CardHead, Chip, EmptyRow, headButton, HeadLink, Page, PageHeader, Pill } from "@/features/settings/ui"
import type { BusinessHours, Channel, Inbox, Weekday } from "@/lib/api"
import { useSession } from "@/lib/session"
import { useInboxMembers } from "@/lib/workspace"

function language(tag: string, locale: string) {
  try {
    return new Intl.DisplayNames([locale], { type: "language" }).of(tag) ?? tag
  } catch {
    return tag
  }
}

const DAYS: Weekday[] = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]

function useHours() {
  const { t, i18n } = useLingui()
  const day = new Intl.DateTimeFormat(i18n.locale, { weekday: "short", timeZone: "UTC" })
  const name = (d: Weekday) => day.format(new Date(Date.UTC(2024, 0, 1 + DAYS.indexOf(d))))
  const clock = (v: string) => (v.endsWith(":00") ? v.slice(0, 2) : v)
  return (h: BusinessHours) => {
    if (!h.enabled) return t`Always`
    if (h.intervals.length === 0) return t`not set`
    const spans = [...new Set(h.intervals.map((i) => `${clock(i.start)}–${clock(i.end)}`))]
    const days = DAYS.filter((d) => h.intervals.some((i) => i.day === d))
    const run = days.every((d, i) => i === 0 || DAYS.indexOf(d) === DAYS.indexOf(days[i - 1]) + 1)
    const which =
      days.length === 1
        ? name(days[0])
        : run
          ? `${name(days[0])}–${name(days[days.length - 1])}`
          : days.map(name).join(", ")
    return spans.length === 1 ? `${which} ${spans[0]}` : which
  }
}

function Fact({ label, children }: { label: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col">
      <dt className="text-caption text-faint">{label}</dt>
      <dd className="truncate">{children}</dd>
    </div>
  )
}

function ProductCard({ inbox, channels }: { inbox: Inbox; channels?: Channel[] }) {
  const { i18n } = useLingui()
  const text = useEnumText()
  const hours = useHours()
  const members = useInboxMembers(inbox.id).data
  const reply = inbox.expected_reply_minutes
  const within = reply !== undefined ? formatDuration(reply * 60, i18n.locale) : ""
  const kinds = [...new Set((channels ?? []).map((c) => c.kind))]
  return (
    <li className="flex flex-col overflow-hidden rounded-xl border bg-card" data-testid="product-card">
      <div className="flex items-center gap-3 border-b px-[18px] py-4">
        <InboxMark name={inbox.name} color={inbox.branding.color} className="size-10 rounded-[10px] text-[18px]" />
        <div className="flex min-w-0 flex-1 flex-col leading-[1.3]">
          <span className="truncate text-[16px] font-semibold">{inbox.name}</span>
          <span className="truncate text-caption text-faint">
            {text.mode[inbox.mode]}
            {within && (
              <>
                {" · "}
                <Trans>replies within {within}</Trans>
              </>
            )}
          </span>
        </div>
        <Button
          variant="outline"
          className={headButton.secondary}
          render={<Link to={`/settings/inboxes/${inbox.id}`} />}
          data-testid="product-settings"
        >
          <Trans>Settings</Trans>
        </Button>
      </div>
      <dl className="grid grid-cols-2 gap-x-4 gap-y-2.5 px-[18px] pt-3.5 pb-3">
        <Fact label={<Trans>Language</Trans>}>{language(inbox.default_locale, i18n.locale)}</Fact>
        <Fact label={<Trans>Hours</Trans>}>{hours(inbox.business_hours)}</Fact>
        <Fact label={<Trans>Members</Trans>}>{members ? members.length : "–"}</Fact>
        <Fact label={<Trans>Ratings</Trans>}>
          {inbox.ask_for_rating ? <Trans>asked after closing</Trans> : <Trans>off</Trans>}
        </Fact>
      </dl>
      <div className="mt-auto flex flex-wrap gap-1.5 px-[18px] pb-4">
        {channels === undefined ? (
          <Skeleton className="h-5 w-24" />
        ) : kinds.length === 0 ? (
          <span className="text-caption text-faint">
            <Trans>No channels yet</Trans>
          </span>
        ) : (
          kinds.map((k) => <Chip key={k}>{text.channel[k]}</Chip>)
        )}
      </div>
    </li>
  )
}

function ChannelState({ inboxId, ch }: { inboxId: string; ch: Channel }) {
  const { canManage } = useSession()
  const state = channelState(ch)
  const pill = {
    ok: (
      <Pill tone="success">
        <Trans>Live</Trans>
      </Pill>
    ),
    receive: (
      <Pill tone="warning">
        <Trans>Receives only</Trans>
      </Pill>
    ),
    paused: (
      <Pill tone="danger">
        <Trans>Paused</Trans>
      </Pill>
    ),
    identity_missing: (
      <Pill tone="warning">
        <Trans>Check DNS</Trans>
      </Pill>
    ),
    error: (
      <Pill tone="danger">
        <Trans>Sending error</Trans>
      </Pill>
    ),
  }[state]
  const act =
    state !== "ok" ? (
      <Trans>Fix</Trans>
    ) : ch.kind === "email" ? (
      <Trans>Settings</Trans>
    ) : ch.kind === "api" ? (
      <Trans>Keys</Trans>
    ) : (
      <Trans>Install</Trans>
    )
  const hash =
    state === "receive"
      ? "replies"
      : state === "ok"
        ? ch.kind === "chat" || ch.kind === "app"
          ? "install"
          : undefined
        : "ses"
  return (
    <>
      <span data-testid="channel-status">{pill}</span>
      {canManage && (
        <Button
          variant="outline"
          className="h-[30px] rounded-lg border-input px-2.5 font-normal"
          render={<Link to={channelLink(inboxId, ch.id, hash)} />}
          data-testid="channel-open"
        >
          {act}
        </Button>
      )}
    </>
  )
}

function ChannelGroup({ inbox, channels }: { inbox: Inbox; channels?: Channel[] }) {
  const { canManage } = useSession()
  return (
    <Card flush data-testid="channel-group">
      <CardHead
        action={
          canManage && (
            <HeadLink to={`/settings/inboxes/${inbox.id}`} testId="channel-group-add">
              <Trans>+ Add channel</Trans>
            </HeadLink>
          )
        }
      >
        <InboxMark name={inbox.name} color={inbox.branding.color} className="size-[22px]" />
        {inbox.name}
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
        <ul>
          {channels.map((ch) => (
            <li
              key={ch.id}
              className="flex items-center gap-3 border-b px-4 py-[11px] last:border-b-0"
              data-testid="channel-row"
            >
              <span className="grid size-8 shrink-0 place-items-center rounded-lg border bg-surface text-muted-foreground">
                <ChannelIcon kind={ch.kind} className="size-4" />
              </span>
              <div className="flex min-w-0 flex-1 flex-col leading-[1.3]">
                <span className="truncate font-medium">{ch.name}</span>
                <span className="truncate text-caption text-faint">
                  <ChannelSummary ch={ch} />
                </span>
              </div>
              <ChannelState inboxId={inbox.id} ch={ch} />
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}

export function ProductsHome() {
  const { canManage } = useSession()
  const [params] = useSearchParams()
  const [inboxId] = useInboxFilter()
  const view = params.get("view") === "channels" ? "channels" : "products"
  const inboxes = useInboxChannels().filter(({ inbox }) => !inboxId || inbox.id === inboxId)
  const first = inboxes[0]?.inbox
  return (
    <Page testId="settings-home">
      <PageHeader
        back={false}
        title={view === "channels" ? <Trans>Channels</Trans> : <Trans>Products</Trans>}
        description={
          view === "channels" ? (
            <Trans>How messages reach each product</Trans>
          ) : (
            <Trans>Each product has its own name, branding, hours and channels</Trans>
          )
        }
        action={
          canManage &&
          (view === "channels" ? (
            first && (
              <Button
                className={headButton.primary}
                render={<Link to={`/settings/inboxes/${first.id}`} />}
                data-testid="channels-add"
              >
                <Trans>Add channel</Trans>
              </Button>
            )
          ) : (
            <Button
              className={headButton.primary}
              render={<Link to="/setup/new" state={{ back: "/settings" }} />}
              data-testid="products-add"
            >
              <Trans>Add product</Trans>
            </Button>
          ))
        }
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
    </Page>
  )
}

import { Trans, useLingui } from "@lingui/react/macro"
import { CheckIcon, CircleIcon, PlusIcon } from "lucide-react"
import { Link } from "react-router"

import { ContactAvatar, ErrorLine } from "@/components/common"
import { Pane } from "@/components/common/Column"
import { formatDateTime, formatDuration } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { PropertyCard } from "@/features/home/PropertyCard"
import { useHome } from "@/features/home/queries"
import { Card, EmptyRow, LinkRow, PageHeader, Rows, RowText, Section } from "@/features/settings/ui"
import { hubTarget, useHubPlaces, useHubText } from "@/features/setup/SetupHub"
import type { SetupState } from "@/features/setup/setup"
import { useNow } from "@/hooks/use-now"
import type { Home } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"

const tileClass = "rounded-2xl border bg-card px-4 py-3.5 transition-colors hover:border-faint"

function More({ to, children, testId }: { to: string; children: React.ReactNode; testId?: string }) {
  return (
    <Link to={to} className="text-small text-brand underline-offset-4 hover:underline" data-testid={testId}>
      {children}
    </Link>
  )
}

function InboxBlock({ inbox }: { inbox: Home["inbox"] }) {
  const { t, i18n } = useLingui()
  const now = useNow(60_000)
  const fmt = new Intl.NumberFormat(i18n.locale)
  const tiles: [number, string, string][] = [
    [inbox.open, t`open`, "open"],
    [inbox.unassigned, t`unassigned`, "unassigned"],
    [inbox.waiting, t`waiting for a reply`, "waiting"],
  ]
  const w = inbox.oldest_waiting
  const name = w ? w.contact.name || w.contact.email || t`Unnamed contact` : ""
  const waited = w ? formatDuration(Math.max(0, (now - new Date(w.since).getTime()) / 1000), i18n.locale) : ""
  return (
    <Section
      title={<Trans>Inbox today</Trans>}
      action={
        <More to="/conversations" testId="home-inbox-open">
          <Trans>Open the inbox</Trans>
        </More>
      }
    >
      <div className="grid grid-cols-3 gap-2" data-testid="home-inbox">
        {tiles.map(([n, label, id]) => (
          <Link key={id} to="/conversations" className={tileClass} data-testid={`home-inbox-${id}`}>
            <b className="block text-page tabular-nums">{fmt.format(n)}</b>
            <span className="text-small text-faint">{label}</span>
          </Link>
        ))}
      </div>
      <Card flush>
        {w ? (
          <Rows>
            <LinkRow to={`/conversations/${w.id}`} value={<Trans>for {waited}</Trans>} testId="home-oldest-waiting">
              <ContactAvatar id={w.contact.id} name={name} className="size-8 shrink-0" />
              <div className="min-w-0 flex-1">
                <p className="text-body break-words">
                  <Trans>
                    Waiting longest: <span className="font-medium">{name}</span>
                  </Trans>
                </p>
                {w.subject && <p className="mt-px text-small break-words text-faint">{w.subject}</p>}
              </div>
            </LinkRow>
          </Rows>
        ) : (
          <EmptyRow>
            {inbox.open > 0 ? (
              <Trans>Nobody is waiting for a reply. Everyone who wrote has an answer.</Trans>
            ) : (
              <Trans>No open conversations. New ones show up here and in the inbox as they arrive.</Trans>
            )}
          </EmptyRow>
        )}
      </Card>
    </Section>
  )
}

function AnalyticsBlock({ analytics }: { analytics: NonNullable<Home["analytics"]> }) {
  return (
    <Section
      title={<Trans>Websites and apps</Trans>}
      action={
        analytics.properties.length > 0 && (
          <More to="/analytics">
            <Trans>Open analytics</Trans>
          </More>
        )
      }
    >
      {analytics.properties.length === 0 ? (
        <Card>
          <div className="flex flex-col items-start gap-3" data-testid="home-analytics-empty">
            <p className="text-body text-muted-foreground">
              <Trans>
                No website or app is counted yet. Add one to see its visitors here every day, without cookies.
              </Trans>
            </p>
            <Button
              size="sm"
              variant="outline"
              render={<Link to="/setup/analytics" state={{ back: "/" } satisfies SetupState} />}
            >
              <PlusIcon />
              <Trans>Add a website or app</Trans>
            </Button>
          </div>
        </Card>
      ) : (
        <div className="grid gap-2 sm:grid-cols-2" data-testid="home-analytics">
          {analytics.properties.map((p) => (
            <Link
              key={p.id}
              to={`/analytics/${p.id}`}
              className="flex rounded-2xl outline-none focus-visible:ring-2 focus-visible:ring-ring [&>div]:transition-colors [&>div]:hover:border-faint"
              data-testid={`home-property-${p.id}`}
            >
              <PropertyCard property={p} />
            </Link>
          ))}
        </div>
      )}
    </Section>
  )
}

function BroadcastBlock({ broadcasts }: { broadcasts: NonNullable<Home["broadcasts"]> }) {
  const { t, i18n } = useLingui()
  const fmt = new Intl.NumberFormat(i18n.locale)
  const b = broadcasts.last_sent
  const sentAt = b ? formatDateTime(b.sent_at, i18n.locale) : ""
  return (
    <Section
      title={<Trans>Last broadcast</Trans>}
      action={
        <More to="/broadcasts">
          <Trans>Open broadcasts</Trans>
        </More>
      }
    >
      {!b ? (
        <Card>
          <div className="flex flex-col items-start gap-3" data-testid="home-broadcast-empty">
            <p className="text-body text-muted-foreground">
              <Trans>
                No broadcast has gone out yet. When one has, its recipients, clicks and replies show up here.
              </Trans>
            </p>
            <Button size="sm" variant="outline" render={<Link to="/broadcasts" />}>
              <Trans>Write a broadcast</Trans>
            </Button>
          </div>
        </Card>
      ) : (
        <Link
          to={`/broadcasts/${b.id}`}
          className="flex flex-col gap-4 rounded-2xl border bg-card p-4 transition-colors hover:border-faint"
          data-testid="home-broadcast"
        >
          <div className="min-w-0">
            <p className="truncate font-medium">{b.name || b.subject}</p>
            <p className="truncate text-small text-faint">
              {b.name && b.subject ? <>{b.subject} · </> : null}
              <Trans>Sent {sentAt}</Trans>
            </p>
          </div>
          <div className="grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-4">
            {(
              [
                [fmt.format(b.recipients), t`recipients`],
                [fmt.format(b.sent), t`sent`],
                [b.click_tracking ? fmt.format(b.clicked) : "–", b.click_tracking ? t`clicked` : t`clicks not tracked`],
                [fmt.format(b.replied), t`replied`],
              ] as const
            ).map(([value, label]) => (
              <div key={label}>
                <b className="block text-title tabular-nums">{value}</b>
                <span className="text-small text-faint">{label}</span>
              </div>
            ))}
          </div>
        </Link>
      )}
    </Section>
  )
}

function SetupBlock({ setup }: { setup: NonNullable<Home["setup"]> }) {
  const text = useHubText()
  const places = useHubPlaces()
  const left = setup.items.filter((i) => !i.done).length
  return (
    <Section
      title={left > 0 ? <Trans>Not set up yet</Trans> : <Trans>Setup</Trans>}
      action={
        <More to="/setup" testId="home-setup-open">
          <Trans>Open setup</Trans>
        </More>
      }
    >
      <Card flush>
        <Rows data-testid="home-setup">
          {setup.items.map((item) => (
            <LinkRow
              key={item.id}
              to={hubTarget(item, places)}
              state={item.done ? undefined : ({ back: "/" } satisfies SetupState)}
              testId={`home-setup-${item.id}`}
            >
              <span
                className={cn(
                  "grid size-6 shrink-0 place-items-center rounded-full [&_svg]:size-3.5",
                  item.done ? "bg-success/12 text-success" : "text-faint"
                )}
              >
                {item.done ? <CheckIcon /> : <CircleIcon className="size-5!" />}
              </span>
              <RowText
                title={<span className={cn(item.done && "text-muted-foreground")}>{text.title[item.id]}</span>}
                detail={item.done ? text.count(item.id, item.count) : text.hint[item.id]}
              />
            </LinkRow>
          ))}
        </Rows>
      </Card>
    </Section>
  )
}

export function HomePage() {
  const { membership } = useSession()
  const home = useHome()
  const h = home.data
  const setupLeft = !!h?.setup?.items.some((i) => !i.done)
  return (
    <Pane wide testId="home">
      <PageHeader
        back={false}
        title={membership.workspace.name}
        description={<Trans>Today, across the inboxes, websites and apps you can see.</Trans>}
      />
      <ErrorLine error={home.error} />
      {!h ? (
        !home.error && (
          <div className="flex flex-col gap-3">
            <Skeleton className="h-24 rounded-2xl" />
            <Skeleton className="h-36 rounded-2xl" />
            <Skeleton className="h-36 rounded-2xl" />
          </div>
        )
      ) : (
        <>
          {h.setup && setupLeft && <SetupBlock setup={h.setup} />}
          <InboxBlock inbox={h.inbox} />
          {h.analytics && <AnalyticsBlock analytics={h.analytics} />}
          {h.broadcasts && <BroadcastBlock broadcasts={h.broadcasts} />}
          {h.setup && !setupLeft && <SetupBlock setup={h.setup} />}
        </>
      )}
    </Pane>
  )
}

import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { CheckIcon, CircleIcon, PlusIcon } from "lucide-react"
import { Link } from "react-router"

import { byPresence, useMemberStatus } from "@/app/TeamMenu"
import { ContactAvatar, Dot, ErrorLine, MemberAvatar, ProgressBar } from "@/components/common"
import { Pane } from "@/components/common/Column"
import { formatDateTime, formatDuration } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { PropertyCard } from "@/features/home/PropertyCard"
import { useHome } from "@/features/home/queries"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { useConversations, useCounts, useStats } from "@/features/inbox/queries"
import { Card, LinkRow, PageHeader, Rows, RowText, Section } from "@/features/settings/ui"
import { hubTarget, useHubPlaces, useHubText } from "@/features/setup/SetupHub"
import type { SetupState } from "@/features/setup/setup"
import { useNow } from "@/hooks/use-now"
import type { ConversationListItem, Home } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes, useMembers } from "@/lib/workspace"

const tileClass =
  "flex flex-col gap-1 rounded-2xl border bg-card px-4 py-3.5 outline-none transition-colors hover:border-faint focus-visible:ring-2 focus-visible:ring-ring"

function More({ to, children, testId }: { to: string; children: React.ReactNode; testId?: string }) {
  return (
    <Link to={to} className="text-small text-brand underline-offset-4 hover:underline" data-testid={testId}>
      {children}
    </Link>
  )
}

type Waiting = { c: ConversationListItem; since: string }

function useInboxNumbers(h: Home | undefined) {
  const [inboxId] = useInboxFilter()
  const { membership } = useSession()
  const counts = useCounts().data
  const open = useConversations({ status: ["open"], inbox_id: inboxId || undefined })
  const rows = open.data?.pages.flatMap((p) => p.items) ?? []
  const more = !!open.hasNextPage
  const oldest = h?.inbox.oldest_waiting
  const waiting: Waiting[] = rows
    .filter((c) => c.last_message?.author_type === "contact")
    .map((c) => ({ c, since: oldest?.id === c.id ? oldest.since : c.last_message!.created_at }))
    .sort((a, b) => a.since.localeCompare(b.since))
  const listed = (n: number) => ({ n, more })
  const filtered = !!inboxId
  return {
    filtered,
    loading: open.isPending,
    error: open.error,
    waiting,
    total: {
      waiting: filtered || !h ? listed(waiting.length) : { n: h.inbox.waiting, more: false },
      unassigned:
        filtered || !h ? listed(rows.filter((c) => !c.assignee_id).length) : { n: h.inbox.unassigned, more: false },
      open: filtered
        ? { n: Number(counts?.inboxes.find((i) => i.id === inboxId)?.count ?? 0), more: false }
        : { n: h?.inbox.open ?? 0, more: false },
      mine: filtered
        ? listed(rows.filter((c) => c.assignee_id === membership.member_id).length)
        : { n: Number(counts?.mine ?? 0), more: false },
    },
  }
}

function Tile({
  to,
  label,
  value,
  hint,
  testId,
}: {
  to: string
  label: React.ReactNode
  value: string
  hint?: React.ReactNode
  testId: string
}) {
  return (
    <Link to={to} className={tileClass} data-testid={testId}>
      <span className="text-small text-faint">{label}</span>
      <b className="text-page leading-tight tabular-nums">{value}</b>
      {hint && <span className="truncate text-caption text-faint">{hint}</span>}
    </Link>
  )
}

function Kpis({ numbers }: { numbers: ReturnType<typeof useInboxNumbers> }) {
  const { i18n } = useLingui()
  const [inboxId] = useInboxFilter()
  const stats = useStats(inboxId).data
  const fmt = new Intl.NumberFormat(i18n.locale)
  const show = ({ n, more }: { n: number; more: boolean }) => (more ? `${fmt.format(n)}+` : fmt.format(n))
  const median = stats?.median_first_reply_seconds
  const total = numbers.total
  return (
    <div className="grid grid-cols-2 gap-2 @3xl:grid-cols-4" data-testid="home-inbox">
      <Tile
        to="/conversations"
        label={<Trans>Waiting</Trans>}
        value={show(total.waiting)}
        testId="home-inbox-waiting"
      />
      <Tile
        to="/conversations"
        label={<Trans>Unassigned</Trans>}
        value={show(total.unassigned)}
        testId="home-inbox-unassigned"
      />
      <Tile to="/conversations" label={<Trans>Open</Trans>} value={show(total.open)} testId="home-inbox-open" />
      {median !== undefined ? (
        <Tile
          to="/reports/today"
          label={<Trans>Median first reply</Trans>}
          value={formatDuration(median, i18n.locale)}
          testId="home-first-reply"
        />
      ) : (
        <Tile to="/conversations" label={<Trans>Mine</Trans>} value={show(total.mine)} testId="home-inbox-mine" />
      )}
    </div>
  )
}

function NeedsYou({ numbers }: { numbers: ReturnType<typeof useInboxNumbers> }) {
  const { t, i18n } = useLingui()
  const now = useNow(60_000)
  const inboxes = useInboxes().data ?? []
  const color = new Map(inboxes.map((i) => [i.id, i.branding.color]))
  const shown = numbers.waiting.slice(0, 6)
  return (
    <Section
      title={<Trans>Needs you</Trans>}
      action={
        <More to="/conversations" testId="home-inbox-link">
          <Trans>Open the inbox</Trans>
        </More>
      }
    >
      <ErrorLine error={numbers.error} />
      <Card flush>
        {numbers.loading ? (
          <div className="flex flex-col gap-3 p-4">
            <Skeleton className="h-9 rounded-xl" />
            <Skeleton className="h-9 rounded-xl" />
            <Skeleton className="h-9 rounded-xl" />
          </div>
        ) : shown.length === 0 ? (
          <div className="flex flex-col items-center gap-2 px-6 py-10 text-center" data-testid="home-needs-empty">
            <span className="grid size-10 place-items-center rounded-full bg-success/12 text-success">
              <CheckIcon className="size-5" />
            </span>
            <p className="text-body font-medium">
              <Trans>Nothing needs you</Trans>
            </p>
            <p className="max-w-xs text-small text-faint">
              <Trans>Nobody is waiting for a reply. Everyone who wrote has an answer.</Trans>
            </p>
          </div>
        ) : (
          <Rows data-testid="home-needs">
            {shown.map(({ c, since }, i) => {
              const name = c.contact.name || c.contact.email || t`Unnamed contact`
              const waited = formatDuration(Math.max(0, (now - new Date(since).getTime()) / 1000), i18n.locale)
              return (
                <LinkRow
                  key={c.id}
                  to={`/conversations/${c.id}`}
                  value={<Trans>for {waited}</Trans>}
                  testId={i === 0 ? "home-oldest-waiting" : `home-waiting-${c.id}`}
                >
                  <ContactAvatar id={c.contact.id} name={name} className="size-8 shrink-0" />
                  <div className="min-w-0 flex-1">
                    <p className="flex items-center gap-1.5 text-body">
                      {inboxes.length > 1 && <Dot color={color.get(c.inbox_id)} className="size-2 rounded-[3px]" />}
                      <span className="truncate font-medium">{name}</span>
                    </p>
                    <p className="mt-px truncate text-small text-faint">{c.subject || c.last_message?.text}</p>
                  </div>
                </LinkRow>
              )
            })}
          </Rows>
        )}
      </Card>
    </Section>
  )
}

function TeamBlock() {
  const members = useMembers().data
  const counts = useCounts().data
  const status = useMemberStatus()
  if (!members) return null
  const load = new Map((counts?.assignees ?? []).map((a) => [a.id, Number(a.count)]))
  const order = [...members].sort(byPresence).slice(0, 8)
  return (
    <Section
      title={<Trans>Team</Trans>}
      action={
        <More to="/team">
          <Trans>Open the team</Trans>
        </More>
      }
    >
      <Card flush>
        <Rows data-testid="home-team">
          {order.map((m) => {
            const name = m.name || m.email
            const open = load.get(m.id) ?? 0
            return (
              <LinkRow
                key={m.id}
                to={`/team/${m.id}`}
                value={open > 0 ? <Plural value={open} one="# open" other="# open" /> : undefined}
                testId={`home-member-${m.id}`}
              >
                <MemberAvatar
                  name={name}
                  online={m.online}
                  away={m.availability === "away"}
                  className="size-8"
                  ring="ring-card"
                />
                <RowText title={name} detail={status(m).text} />
              </LinkRow>
            )
          })}
        </Rows>
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
  const { t, i18n } = useLingui()
  const text = useHubText()
  const places = useHubPlaces()
  const count = setup.items.length
  const left = setup.items.filter((i) => !i.done).length
  const fmt = new Intl.NumberFormat(i18n.locale)
  const doneText = fmt.format(count - left)
  const total = fmt.format(count)
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
        <div className="flex items-center gap-3 border-b px-4 py-3">
          <ProgressBar
            value={count ? ((count - left) / count) * 100 : 0}
            label={t`Setup`}
            text={t`${doneText} of ${total}`}
            thin
            className="flex-1"
            testId="home-setup-progress"
          />
          <span className="shrink-0 text-small text-faint tabular-nums">{t`${doneText} of ${total}`}</span>
        </div>
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
  const { t } = useLingui()
  const { me, membership } = useSession()
  const home = useHome()
  const h = home.data
  const numbers = useInboxNumbers(h)
  const [inboxId] = useInboxFilter()
  const product = useInboxes().data?.find((i) => i.id === inboxId)?.name
  const setupLeft = !!h?.setup?.items.some((i) => !i.done)
  const first = (me.person.name || me.person.email).split(/[\s@]/)[0]
  const waiting = numbers.total.waiting.n
  const workspace = membership.workspace.name
  const hour = new Date(useNow(60_000)).getHours()
  const greeting =
    hour < 12 ? t`Good morning, ${first}` : hour < 18 ? t`Good afternoon, ${first}` : t`Good evening, ${first}`
  return (
    <Pane wide testId="home" className="@container">
      <PageHeader
        back={false}
        eyebrow={product ? `${workspace} · ${product}` : workspace}
        title={greeting}
        description={
          !h && !numbers.filtered ? undefined : waiting > 0 ? (
            <Plural value={waiting} one="# person is waiting for a reply." other="# people are waiting for a reply." />
          ) : (
            <Trans>Nothing is waiting.</Trans>
          )
        }
      />
      <ErrorLine error={home.error} />
      {!h ? (
        !home.error && (
          <div className="flex flex-col gap-3">
            <div className="grid grid-cols-2 gap-2 @3xl:grid-cols-4">
              {[0, 1, 2, 3].map((i) => (
                <Skeleton key={i} className="h-22 rounded-2xl" />
              ))}
            </div>
            <Skeleton className="h-64 rounded-2xl" />
          </div>
        )
      ) : (
        <>
          <Kpis numbers={numbers} />
          <div className="grid grid-cols-[minmax(0,1fr)] items-start gap-8 @3xl:grid-cols-[minmax(0,1fr)_minmax(0,320px)]">
            <NeedsYou numbers={numbers} />
            <div className="flex flex-col gap-8">
              <TeamBlock />
              {h.setup && setupLeft && <SetupBlock setup={h.setup} />}
            </div>
          </div>
          {h.analytics && <AnalyticsBlock analytics={h.analytics} />}
          {h.broadcasts && <BroadcastBlock broadcasts={h.broadcasts} />}
        </>
      )}
    </Pane>
  )
}

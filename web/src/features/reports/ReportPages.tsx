import { Trans, useLingui } from "@lingui/react/macro"
import { useQueries, useQuery } from "@tanstack/react-query"
import { ThumbsDownIcon, ThumbsUpIcon } from "lucide-react"
import { Outlet, useLocation, useSearchParams } from "react-router"

import { ErrorLine, InboxMark, MemberAvatar, Segmented } from "@/components/common"
import { Pane } from "@/components/common/Column"
import { formatDuration } from "@/components/common/text"
import { Skeleton } from "@/components/ui/skeleton"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { Card, CardHead, EmptyRow, PageHeader, PageTabs, StatTile } from "@/features/settings/ui"
import { api, unwrap, type Stats } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"
import { useInboxes, useMembers } from "@/lib/workspace"

const RANGES = ["today", "7d", "30d"] as const
type Range = (typeof RANGES)[number]

function useRange(): Range {
  const [params] = useSearchParams()
  const raw = params.get("range") as Range | null
  return raw && RANGES.includes(raw) ? raw : "today"
}

function sinceOf(range: Range) {
  if (range === "today") return undefined
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  d.setDate(d.getDate() - (range === "7d" ? 6 : 29))
  return d.toISOString()
}

function statsQuery(ws: string, inboxId: string, since: string | undefined) {
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone
  return {
    queryKey: since ? [...keys.stats(ws), inboxId, timezone, since] : [...keys.stats(ws), inboxId, timezone],
    queryFn: () =>
      unwrap(api.GET("/v1/stats", { params: { query: { inbox_id: inboxId || undefined, timezone, since } } })),
    staleTime: 60_000,
  }
}

function useReportStats() {
  const { workspaceId: ws } = useSession()
  const [inboxId] = useInboxFilter()
  const since = sinceOf(useRange())
  return useQuery(statsQuery(ws, inboxId, since))
}

function useRangeText() {
  const { t } = useLingui()
  return { today: t`Today`, "7d": t`Last 7 days`, "30d": t`Last 30 days` } satisfies Record<Range, string>
}

function RangePicker() {
  const { t } = useLingui()
  const range = useRange()
  const names = useRangeText()
  const [, setParams] = useSearchParams()
  return (
    <Segmented
      value={range}
      onChange={(v) =>
        setParams(
          (prev) => {
            const next = new URLSearchParams(prev)
            if (v === "today") next.delete("range")
            else next.set("range", v)
            return next
          },
          { replace: true }
        )
      }
      label={t`Period`}
      items={RANGES.map((r) => ({ value: r, label: names[r], testId: `report-range-${r}` }))}
    />
  )
}

export function ReportsPane() {
  const { t } = useLingui()
  const { pathname, search } = useLocation()
  const current = pathname.split("/")[2] ?? "today"
  const tabs = [
    { key: "today", to: { pathname: "/reports/today", search }, label: t`Activity`, testId: "report-tab-today" },
    { key: "ratings", to: { pathname: "/reports/ratings", search }, label: t`Ratings`, testId: "report-tab-ratings" },
    { key: "members", to: { pathname: "/reports/members", search }, label: t`Team`, testId: "report-tab-members" },
  ]
  return (
    <Pane wide testId="reports" className="gap-6">
      <PageHeader
        back={false}
        title={<Trans>Reports</Trans>}
        description={<Trans>How fast and how well your team answers.</Trans>}
        action={<RangePicker />}
        tabs={<PageTabs label={t`Reports`} items={tabs} current={current} />}
      />
      <Outlet />
    </Pane>
  )
}

function Report({ children }: { children: (s: Stats) => React.ReactNode }) {
  const stats = useReportStats()
  return (
    <>
      {stats.isPending && <Skeleton className="h-28 rounded-2xl" />}
      <ErrorLine error={stats.error} />
      {stats.data && children(stats.data)}
    </>
  )
}

function ByInbox() {
  const { t, i18n } = useLingui()
  const { workspaceId: ws } = useSession()
  const [inboxId] = useInboxFilter()
  const since = sinceOf(useRange())
  const inboxes = useInboxes().data ?? []
  const shown = inboxId ? [] : inboxes
  const results = useQueries({ queries: shown.map((i) => statsQuery(ws, i.id, since)) })
  if (shown.length < 2) return null
  const fmt = new Intl.NumberFormat(i18n.locale)
  return (
    <Card flush>
      <CardHead
        action={
          <span className="text-caption text-faint">
            <Trans>median first reply</Trans>
          </span>
        }
      >
        <Trans>By inbox</Trans>
      </CardHead>
      <ul className="divide-y" data-testid="report-inboxes">
        {shown.map((inbox, i) => {
          const s = results[i]?.data
          const median = s?.median_first_reply_seconds
          return (
            <li key={inbox.id} className="flex min-h-12 items-center gap-3 px-4 py-2.5 text-body">
              <InboxMark name={inbox.name} color={inbox.branding.color} className="size-6" />
              <span className="min-w-0 flex-1 truncate font-medium">{inbox.name}</span>
              {s ? (
                <>
                  <span className="shrink-0 text-small text-faint tabular-nums">
                    {t`${fmt.format(s.replies)} replies · ${fmt.format(s.closed)} closed`}
                  </span>
                  <span className="w-20 shrink-0 text-end text-muted-foreground tabular-nums">
                    {median !== undefined ? formatDuration(median, i18n.locale) : "–"}
                  </span>
                </>
              ) : (
                <Skeleton className="h-4 w-32" />
              )}
            </li>
          )
        })}
      </ul>
    </Card>
  )
}

export function TodayReport() {
  const { t, i18n } = useLingui()
  const fmt = new Intl.NumberFormat(i18n.locale)
  return (
    <Report>
      {(s) => {
        const tiles: [string, string, string][] = [
          [fmt.format(s.replies), t`Replies`, t`messages sent`],
          [fmt.format(s.closed), t`Closed`, t`conversations`],
          [fmt.format(s.first_replies), t`First replies`, t`people answered`],
          [
            s.median_first_reply_seconds !== undefined
              ? formatDuration(s.median_first_reply_seconds, i18n.locale)
              : "–",
            t`First reply`,
            t`median`,
          ],
        ]
        return (
          <>
            <div className="grid grid-cols-2 gap-3 md:grid-cols-4" data-testid="report-tiles">
              {tiles.map(([value, label, hint]) => (
                <StatTile key={label} label={label} value={value} hint={hint} />
              ))}
            </div>
            <ByInbox />
          </>
        )
      }}
    </Report>
  )
}

function Split({ good, bad }: { good: number; bad: number }) {
  const rated = good + bad
  return (
    <div className="flex h-2 overflow-hidden rounded-full bg-destructive/25" aria-hidden>
      <span className="bg-success" style={{ width: `${rated ? (good / rated) * 100 : 0}%` }} />
    </div>
  )
}

export function RatingsReport() {
  const { t, i18n } = useLingui()
  const inboxes = new Map((useInboxes().data ?? []).map((i) => [i.id, i]))
  const fmt = new Intl.NumberFormat(i18n.locale)
  const pct = new Intl.NumberFormat(i18n.locale, { style: "percent" })
  return (
    <Report>
      {(s) => {
        const rated = s.ratings.good + s.ratings.bad
        if (rated === 0) {
          return (
            <Card flush>
              <EmptyRow>
                <Trans>No ratings in this period.</Trans>
              </EmptyRow>
            </Card>
          )
        }
        return (
          <div className="grid items-start gap-4 lg:grid-cols-[300px_minmax(0,1fr)]" data-testid="report-ratings">
            <Card>
              <div className="flex flex-col gap-3">
                <div className="flex items-baseline gap-2">
                  <b className="text-[2.5rem] leading-none font-semibold tracking-tight tabular-nums">
                    {pct.format(s.ratings.good / rated)}
                  </b>
                  <span className="text-body text-muted-foreground">
                    <Trans>satisfied</Trans>
                  </span>
                </div>
                <Split good={s.ratings.good} bad={s.ratings.bad} />
                <div className="flex items-center gap-4 text-body">
                  <span className="inline-flex items-center gap-1.5 text-success">
                    <ThumbsUpIcon className="size-3.5" />
                    {fmt.format(s.ratings.good)}
                  </span>
                  <span className="inline-flex items-center gap-1.5 text-destructive">
                    <ThumbsDownIcon className="size-3.5" />
                    {fmt.format(s.ratings.bad)}
                  </span>
                </div>
              </div>
            </Card>
            {s.ratings.inboxes.length > 1 && (
              <Card flush>
                <CardHead>
                  <Trans>By inbox</Trans>
                </CardHead>
                <ul className="divide-y">
                  {s.ratings.inboxes.map((r) => {
                    const inbox = inboxes.get(r.inbox_id)
                    const name = inbox?.name ?? t`Deleted inbox`
                    return (
                      <li key={r.inbox_id} className="flex items-center gap-3 px-4 py-2.5 text-body">
                        <InboxMark name={name} color={inbox?.branding.color} className="size-6" />
                        <span className="w-32 min-w-0 truncate font-medium">{name}</span>
                        <span className="min-w-0 flex-1">
                          <Split good={r.good} bad={r.bad} />
                        </span>
                        <span className="w-10 text-end text-success tabular-nums">{fmt.format(r.good)}</span>
                        <span className="w-10 text-end text-destructive tabular-nums">{fmt.format(r.bad)}</span>
                      </li>
                    )
                  })}
                </ul>
              </Card>
            )}
          </div>
        )
      }}
    </Report>
  )
}

export function MembersReport() {
  const { t, i18n } = useLingui()
  const members = new Map((useMembers().data ?? []).map((m) => [m.id, m]))
  const fmt = new Intl.NumberFormat(i18n.locale)
  return (
    <Report>
      {(s) => {
        const people = [...s.members].sort((a, b) => b.replies - a.replies || b.closed - a.closed)
        if (people.length === 0) {
          return (
            <Card flush>
              <EmptyRow>
                <Trans>Nobody has replied or closed anything in this period.</Trans>
              </EmptyRow>
            </Card>
          )
        }
        const most = Math.max(1, ...people.map((p) => p.replies))
        return (
          <Card flush>
            <table className="w-full table-fixed text-body" data-testid="report-members">
              <thead className="bg-surface text-caption text-faint">
                <tr>
                  <th className="px-4 py-2.5 text-start font-medium">
                    <Trans>Member</Trans>
                  </th>
                  <th className="w-[38%] px-2 py-2.5 text-start font-medium">
                    <Trans>Replies</Trans>
                  </th>
                  <th className="w-24 px-4 py-2.5 text-end font-medium">
                    <Trans>Closed</Trans>
                  </th>
                </tr>
              </thead>
              <tbody>
                {people.map((p) => {
                  const m = members.get(p.member_id)
                  const name = m ? m.name || m.email : t`Deleted member`
                  return (
                    <tr key={p.member_id} className="border-t">
                      <td className="px-4 py-2.5">
                        <span className="flex min-w-0 items-center gap-2.5">
                          <MemberAvatar name={name} className="size-7" />
                          <span className="truncate font-medium">{name}</span>
                        </span>
                      </td>
                      <td className="px-2 py-2.5">
                        <span className="flex items-center gap-2">
                          <span className="flex h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-muted">
                            <span className="rounded-full bg-brand" style={{ width: `${(p.replies / most) * 100}%` }} />
                          </span>
                          <span className="w-10 shrink-0 text-end tabular-nums">{fmt.format(p.replies)}</span>
                        </span>
                      </td>
                      <td className="px-4 py-2.5 text-end tabular-nums">{fmt.format(p.closed)}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </Card>
        )
      }}
    </Report>
  )
}

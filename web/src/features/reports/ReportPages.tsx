import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { ThumbsDownIcon, ThumbsUpIcon } from "lucide-react"
import { Outlet, useLocation, useSearchParams } from "react-router"

import { ErrorLine, InboxMark } from "@/components/common"
import { formatDuration } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import {
  createdSince,
  dayKey,
  dayStart,
  ratedSince,
  share,
  useInboxStats,
  useRecentConversations,
  useWindowStats,
} from "@/features/reports/data"
import {
  Card,
  CardHead,
  EmptyRow,
  Face,
  headButton,
  HeadNote,
  Page,
  PageHeader,
  PageTabs,
  ProductTag,
  StatTile,
  tableHeadClass,
} from "@/features/settings/ui"
import { useNow } from "@/hooks/use-now"
import type { ConversationListItem, Stats } from "@/lib/api"
import { cn } from "@/lib/utils"
import { useInboxes, useMembers } from "@/lib/workspace"

const RANGES = ["today", "7d", "30d"] as const
type Range = (typeof RANGES)[number]

function useRange(): Range {
  const [params] = useSearchParams()
  const raw = params.get("range") as Range | null
  return raw && RANGES.includes(raw) ? raw : "today"
}

function startOf(range: Range) {
  return dayStart(range === "today" ? 0 : range === "7d" ? 6 : 29)
}

function sinceOf(range: Range) {
  return range === "today" ? undefined : startOf(range).toISOString()
}

function useReportStats() {
  const [inboxId] = useInboxFilter()
  return useWindowStats(inboxId, sinceOf(useRange()))
}

function useRecent() {
  const [inboxId] = useInboxFilter()
  const since = startOf(useRange()).toISOString()
  const recent = useRecentConversations(since, inboxId)
  return { since, recent }
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
    <Select
      value={range}
      items={names}
      onValueChange={(v) =>
        setParams(
          (prev) => {
            const next = new URLSearchParams(prev)
            if (!v || v === "today") next.delete("range")
            else next.set("range", v)
            return next
          },
          { replace: true }
        )
      }
    >
      <SelectTrigger aria-label={t`Period`} className={cn(headButton.secondary, "gap-1.5")} data-testid="report-range">
        <SelectValue />
      </SelectTrigger>
      <SelectContent className="min-w-40">
        {RANGES.map((r) => (
          <SelectItem key={r} value={r} data-testid={`report-range-${r}`}>
            {names[r]}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

function csvCell(v: string | number) {
  const s = String(v)
  return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s
}

function ExportButton() {
  const { t } = useLingui()
  const range = useRange()
  const stats = useReportStats().data
  const members = new Map((useMembers().data ?? []).map((m) => [m.id, m]))
  const inboxes = new Map((useInboxes().data ?? []).map((i) => [i.id, i]))
  const download = () => {
    if (!stats) return
    const rows: (string | number)[][] = [
      ["kind", "name", "replies", "closed", "first_replies", "median_first_reply_seconds", "good", "bad"],
      [
        "total",
        range,
        stats.replies,
        stats.closed,
        stats.first_replies,
        stats.median_first_reply_seconds ?? "",
        stats.ratings.good,
        stats.ratings.bad,
      ],
      ...stats.ratings.inboxes.map((r) => [
        "inbox",
        inboxes.get(r.inbox_id)?.name ?? r.inbox_id,
        "",
        "",
        "",
        "",
        r.good,
        r.bad,
      ]),
      ...stats.members.map((m) => {
        const member = members.get(m.member_id)
        return ["member", member ? member.name || member.email : m.member_id, m.replies, m.closed, "", "", "", ""]
      }),
    ]
    const blob = new Blob([rows.map((r) => r.map(csvCell).join(",")).join("\n") + "\n"], { type: "text/csv" })
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = `yuva-report-${range}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }
  return (
    <Button
      className={headButton.primary}
      disabled={!stats}
      onClick={download}
      title={t`Download CSV`}
      data-testid="report-export"
    >
      <Trans>Export</Trans>
    </Button>
  )
}

export function ReportsPane() {
  const { t } = useLingui()
  const { pathname, search } = useLocation()
  const current = pathname.split("/")[2] ?? "today"
  const tabs = [
    { key: "today", to: { pathname: "/reports/today", search }, label: t`Today`, testId: "report-tab-today" },
    { key: "ratings", to: { pathname: "/reports/ratings", search }, label: t`Ratings`, testId: "report-tab-ratings" },
    { key: "members", to: { pathname: "/reports/members", search }, label: t`Team`, testId: "report-tab-members" },
  ]
  return (
    <Page testId="reports">
      <PageHeader
        back={false}
        title={<Trans>Reports</Trans>}
        description={<Trans>How fast and how well your team answers</Trans>}
        action={
          <>
            <RangePicker />
            <ExportButton />
          </>
        }
        tabs={<PageTabs label={t`Reports`} items={tabs} current={current} />}
      />
      <Outlet />
    </Page>
  )
}

function Report({ children }: { children: (s: Stats) => React.ReactNode }) {
  const stats = useReportStats()
  return (
    <>
      {stats.isPending && <Skeleton className="h-28 rounded-xl" />}
      <ErrorLine error={stats.error} />
      {stats.data && children(stats.data)}
    </>
  )
}

function NewChart({ items }: { items: ConversationListItem[] }) {
  const { t, i18n } = useLingui()
  const range = useRange()
  const now = new Date(useNow(60_000))
  const hourly = range === "today"
  const fmt = new Intl.NumberFormat(i18n.locale)
  let slots: { key: string; label: string; n: number }[]
  if (hourly) {
    const hours = items.map((c) => new Date(c.created_at!).getHours())
    const from = Math.min(8, ...hours)
    const to = Math.max(now.getHours(), from + 5)
    const time = new Intl.DateTimeFormat(i18n.locale, { hour: "2-digit", minute: "2-digit" })
    slots = Array.from({ length: to - from + 1 }, (_, i) => {
      const at = new Date(now)
      at.setHours(from + i, 0, 0, 0)
      return { key: String(from + i), label: time.format(at), n: hours.filter((h) => h === from + i).length }
    })
  } else {
    const days = range === "7d" ? 7 : 30
    const label = new Intl.DateTimeFormat(i18n.locale, { day: "numeric", month: "short" })
    slots = Array.from({ length: days }, (_, i) => {
      const d = dayStart(days - 1 - i)
      const key = dayKey(d)
      return { key, label: label.format(d), n: items.filter((c) => dayKey(new Date(c.created_at!)) === key).length }
    })
  }
  const top = Math.max(1, ...slots.map((s) => s.n))
  const last = hourly ? slots.findIndex((s) => Number(s.key) === now.getHours()) : slots.length - 1
  const ticks = [0, Math.round((slots.length - 1) / 3), Math.round(((slots.length - 1) * 2) / 3), slots.length - 1]
  return (
    <section className="flex min-w-0 flex-col gap-3 rounded-xl border bg-card p-4" data-testid="report-chart">
      <h2 className="font-semibold">
        {hourly ? <Trans>New conversations by hour</Trans> : <Trans>New conversations by day</Trans>}
      </h2>
      <div className="flex h-40 items-end gap-1.5 border-b" role="img" aria-label={t`New conversations`}>
        {slots.map((s, i) => (
          <span
            key={s.key}
            className="flex-1 rounded-t bg-brand"
            title={`${s.label} · ${fmt.format(s.n)}`}
            style={{ height: s.n ? `${(s.n / top) * 100}%` : 2, opacity: s.n ? (i === last ? 1 : 0.7) : 0.25 }}
          />
        ))}
      </div>
      <div className="flex justify-between text-caption text-faint">
        {[...new Set(ticks)].map((i) => (
          <span key={i}>{slots[i]?.label}</span>
        ))}
      </div>
    </section>
  )
}

function ByProduct({ items }: { items: ConversationListItem[] }) {
  const { i18n } = useLingui()
  const [inboxId] = useInboxFilter()
  const since = sinceOf(useRange())
  const inboxes = (useInboxes().data ?? []).filter((i) => !inboxId || i.id === inboxId)
  const stats = useInboxStats(
    inboxes.map((i) => i.id),
    since
  )
  const fmt = new Intl.NumberFormat(i18n.locale)
  const rows = inboxes
    .map((inbox, i) => ({ inbox, s: stats[i]?.data, n: items.filter((c) => c.inbox_id === inbox.id).length }))
    .sort((a, b) => b.n - a.n)
  return (
    <Card flush>
      <CardHead
        action={
          <HeadNote>
            <Trans>first reply</Trans>
          </HeadNote>
        }
      >
        <Trans>By product</Trans>
      </CardHead>
      <ul data-testid="report-inboxes">
        {rows.map(({ inbox, s, n }) => {
          const median = s?.median_first_reply_seconds
          return (
            <li key={inbox.id} className="flex items-center gap-3 border-b px-4 py-[11px] last:border-b-0">
              <InboxMark name={inbox.name} color={inbox.branding.color} className="size-[22px]" />
              <span className="min-w-0 flex-1 truncate font-medium">{inbox.name}</span>
              <span className="shrink-0 text-caption text-faint tabular-nums">
                <Trans>{fmt.format(n)} new</Trans>
              </span>
              <span className="w-[70px] shrink-0 text-end text-muted-foreground tabular-nums">
                {!s ? (
                  <Skeleton className="ms-auto h-4 w-10" />
                ) : median !== undefined ? (
                  formatDuration(median, i18n.locale)
                ) : (
                  "–"
                )}
              </span>
            </li>
          )
        })}
      </ul>
    </Card>
  )
}

export function TodayReport() {
  const { t, i18n } = useLingui()
  const range = useRange()
  const { since, recent } = useRecent()
  const fmt = new Intl.NumberFormat(i18n.locale)
  const fresh = createdSince(recent.data?.items ?? [], since)
  return (
    <Report>
      {(s) => (
        <>
          <div className="grid grid-cols-4 gap-4 phone:grid-cols-2" data-testid="report-tiles">
            <StatTile
              label={t`New`}
              value={recent.data ? `${fmt.format(fresh.length)}${recent.data.complete ? "" : "+"}` : "–"}
              hint={t`conversations started`}
              testId="report-new"
            />
            <StatTile label={t`Replied`} value={fmt.format(s.replies)} hint={t`messages sent`} />
            <StatTile label={t`Closed`} value={fmt.format(s.closed)} hint={t`conversations`} />
            <StatTile
              label={t`First reply`}
              value={
                s.median_first_reply_seconds !== undefined
                  ? formatDuration(s.median_first_reply_seconds, i18n.locale)
                  : "–"
              }
              hint={range === "today" ? t`median today` : t`median`}
            />
          </div>
          <div className="grid grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)] gap-4 max-lg:grid-cols-1">
            {recent.isPending ? <Skeleton className="h-60 rounded-xl" /> : <NewChart items={fresh} />}
            <ByProduct items={fresh} />
          </div>
        </>
      )}
    </Report>
  )
}

function Score({ good }: { good: boolean }) {
  const { t } = useLingui()
  return good ? (
    <ThumbsUpIcon className="size-4 text-success" aria-label={t`Good`} />
  ) : (
    <ThumbsDownIcon className="size-4 text-destructive" aria-label={t`Bad`} />
  )
}

export function RatingsReport() {
  const { i18n } = useLingui()
  const { since, recent } = useRecent()
  const inboxes = new Map((useInboxes().data ?? []).map((i) => [i.id, i]))
  const members = new Map((useMembers().data ?? []).map((m) => [m.id, m]))
  const fmt = new Intl.NumberFormat(i18n.locale)
  const pct = new Intl.NumberFormat(i18n.locale, { style: "percent" })
  const written = ratedSince(recent.data?.items ?? [], since).filter((c) => c.rating?.comment)
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
        const most = Math.max(s.ratings.good, s.ratings.bad, 1)
        const rows: [boolean, number][] = [
          [true, s.ratings.good],
          [false, s.ratings.bad],
        ]
        return (
          <div className="grid items-start gap-4 lg:grid-cols-[340px_minmax(0,1fr)]" data-testid="report-ratings">
            <section className="flex flex-col gap-3.5 rounded-xl border bg-card p-5">
              <span className="flex items-baseline gap-2.5">
                <b className="text-[44px] leading-none font-semibold tracking-[-0.03em] tabular-nums">
                  {pct.format(share(s.ratings.good, s.ratings.bad) ?? 0)}
                </b>
                <span className="text-muted-foreground">
                  <Plural value={rated} one="# rating" other="# ratings" />
                </span>
              </span>
              {rows.map(([good, n]) => (
                <span key={String(good)} className="flex items-center gap-2.5">
                  <span className="w-7">
                    <Score good={good} />
                  </span>
                  <span className="flex h-2 flex-1 overflow-hidden rounded-full bg-border">
                    <span
                      className={cn("rounded-full", good ? "bg-success" : "bg-destructive")}
                      style={{ width: `${(n / most) * 100}%` }}
                    />
                  </span>
                  <span className="w-7 text-end text-caption text-faint tabular-nums">{fmt.format(n)}</span>
                </span>
              ))}
            </section>
            <Card flush>
              <CardHead>
                <Trans>What people wrote</Trans>
              </CardHead>
              {recent.isPending ? (
                <Skeleton className="m-4 h-24" />
              ) : written.length === 0 ? (
                <EmptyRow>
                  <Trans>Nobody left a comment with a rating in this period.</Trans>
                </EmptyRow>
              ) : (
                <ul data-testid="report-comments">
                  {written.slice(0, 20).map((c) => {
                    const inbox = inboxes.get(c.inbox_id)
                    const by = c.assignee_id ? members.get(c.assignee_id) : undefined
                    const name = by ? by.name || by.email : ""
                    const who = c.contact.name || c.contact.email || ""
                    return (
                      <li key={c.id} className="flex items-start gap-3 border-b px-4 py-[11px] last:border-b-0">
                        <span className="w-11 shrink-0 pt-0.5">
                          <Score good={c.rating!.rating === "good"} />
                        </span>
                        <span className="flex min-w-0 flex-1 flex-col gap-0.5 leading-[1.4]">
                          <span className="break-words">“{c.rating!.comment}”</span>
                          <span className="flex min-w-0 flex-wrap items-center gap-1.5 text-caption text-faint">
                            {inbox && <ProductTag name={inbox.name} color={inbox.branding.color} />}
                            {name ? (
                              <Trans>
                                {who} · answered by {name}
                              </Trans>
                            ) : (
                              who
                            )}
                          </span>
                        </span>
                      </li>
                    )
                  })}
                </ul>
              )}
            </Card>
          </div>
        )
      }}
    </Report>
  )
}

const MEMBER_GRID =
  "grid grid-cols-[minmax(0,1.4fr)_minmax(0,1.2fr)_minmax(0,1fr)_minmax(0,1fr)] items-center gap-4 px-4"

export function MembersReport() {
  const { t, i18n } = useLingui()
  const { since, recent } = useRecent()
  const members = new Map((useMembers().data ?? []).map((m) => [m.id, m]))
  const fmt = new Intl.NumberFormat(i18n.locale)
  const pct = new Intl.NumberFormat(i18n.locale, { style: "percent" })
  const rated = ratedSince(recent.data?.items ?? [], since)
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
          <section className="overflow-x-auto rounded-xl border bg-card" data-testid="report-members">
            <div className="min-w-[560px]">
              <div className={cn(MEMBER_GRID, tableHeadClass, "py-2.5")}>
                <span>
                  <Trans>Member</Trans>
                </span>
                <span>
                  <Trans>Replies</Trans>
                </span>
                <span>
                  <Trans>Closed</Trans>
                </span>
                <span>
                  <Trans>Rating</Trans>
                </span>
              </div>
              {people.map((p) => {
                const m = members.get(p.member_id)
                const name = m ? m.name || m.email : t`Deleted member`
                const mine = rated.filter((c) => c.assignee_id === p.member_id)
                const good = mine.filter((c) => c.rating!.rating === "good").length
                return (
                  <div key={p.member_id} className={cn(MEMBER_GRID, "border-t py-3")}>
                    <span className="flex min-w-0 items-center gap-2.5">
                      <Face id={p.member_id} name={name} className="size-7" />
                      <span className="truncate font-medium">{name}</span>
                    </span>
                    <span className="flex items-center gap-2">
                      <span className="flex h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-border">
                        <span className="rounded-full bg-brand" style={{ width: `${(p.replies / most) * 100}%` }} />
                      </span>
                      <span className="w-7 shrink-0 text-muted-foreground tabular-nums">{fmt.format(p.replies)}</span>
                    </span>
                    <span className="text-muted-foreground tabular-nums">{fmt.format(p.closed)}</span>
                    <span className="text-muted-foreground tabular-nums">
                      {mine.length ? <Trans>{pct.format(good / mine.length)} good</Trans> : "–"}
                    </span>
                  </div>
                )
              })}
            </div>
          </section>
        )
      }}
    </Report>
  )
}

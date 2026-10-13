import { plural } from "@lingui/core/macro"
import { Trans, useLingui } from "@lingui/react/macro"
import { BellIcon, GaugeIcon, GlobeIcon, SendIcon, TargetIcon, TrendingDownIcon, TrendingUpIcon } from "lucide-react"

import { ErrorLine, Segmented } from "@/components/common"
import { formatDate, formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { useAlerts, useSendReport, useSubscription, useUpdateSubscription } from "@/features/analytics/queries"
import { Card, EmptyRow, Rows, Section, ToggleRow } from "@/features/settings/ui"
import {
  ApiError,
  type AnalyticsAlert,
  type AnalyticsAlertKind,
  type AnalyticsProperty,
  type AnalyticsReportFrequency,
} from "@/lib/api"
import { useSession } from "@/lib/session"

export const ALERT_KINDS: AnalyticsAlertKind[] = ["spike", "drop", "goal_drop", "support_load", "new_referrer"]

const ICONS: Record<AnalyticsAlertKind, React.ComponentType<{ className?: string }>> = {
  spike: TrendingUpIcon,
  drop: TrendingDownIcon,
  goal_drop: TargetIcon,
  support_load: GaugeIcon,
  new_referrer: GlobeIcon,
}

export function useAlertText() {
  const { t, i18n } = useLingui()
  const n = (v: number) => new Intl.NumberFormat(i18n.locale, { maximumFractionDigits: 1 }).format(v)
  const kind: Record<AnalyticsAlertKind, string> = {
    spike: t`Traffic spike`,
    drop: t`Traffic drop`,
    goal_drop: t`Goal drop`,
    support_load: t`Support load`,
    new_referrer: t`New referrer`,
  }
  const help: Record<AnalyticsAlertKind, string> = {
    spike: t`Views of the last two hours are at least three times the same hours of the week before.`,
    drop: t`Views of the last two hours are at most half the same hours of the week before.`,
    goal_drop: t`A goal’s conversions yesterday were at most half its daily average of the week before.`,
    support_load: t`Conversations per 1,000 views yesterday were at least twice the rate of the 28 days before.`,
    new_referrer: t`A site sent 10 or more visits yesterday and none in the 28 days before.`,
  }
  const sentence = (a: AnalyticsAlert) => {
    const now = n(a.current)
    const usual = n(a.baseline)
    const count = Math.round(a.current * 10) / 10
    switch (a.kind) {
      case "spike":
        return t`Views rose to ${now} in two hours, against ${usual} usually.`
      case "drop":
        return t`Views fell to ${now} in two hours, against ${usual} usually.`
      case "goal_drop": {
        const goal = a.goal_name ?? ""
        return plural(count, {
          one: `Goal “${goal}” had # conversion, against ${usual} a day on average in the week before.`,
          other: `Goal “${goal}” had # conversions, against ${usual} a day on average in the week before.`,
        })
      }
      case "support_load":
        return plural(count, {
          one: `# conversation per 1,000 views, against ${usual} in the 28 days before.`,
          other: `# conversations per 1,000 views, against ${usual} in the 28 days before.`,
        })
      case "new_referrer": {
        const site = a.referrer ?? ""
        return plural(count, {
          one: `${site} sent # visit, and none in the 28 days before.`,
          other: `${site} sent # visits, and none in the 28 days before.`,
        })
      }
    }
  }
  return { kind, help, sentence }
}

function when(a: AnalyticsAlert, locale: string, timeZone: string) {
  if (!a.hours_from || !a.hours_to) return formatDate(`${a.day}T12:00:00Z`, locale)
  const from = new Date(a.hours_from)
  const to = new Date(a.hours_to)
  try {
    const day = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeZone }).format(from)
    const hour = new Intl.DateTimeFormat(locale, { timeStyle: "short", timeZone })
    return `${day}, ${hour.format(from)}–${hour.format(to)}`
  } catch {
    return formatDateTime(a.hours_from, locale)
  }
}

function NotifyMe({ p }: { p: AnalyticsProperty }) {
  const { t } = useLingui()
  const { me } = useSession()
  const text = useAlertText()
  const sub = useSubscription(p.id)
  const update = useUpdateSubscription(p.id)
  const send = useSendReport(p.id)
  const s = sub.data
  const email = me.person.email
  const zone = p.timezone
  const reports: { value: AnalyticsReportFrequency; label: string; testId: string }[] = [
    { value: "none", label: t`None`, testId: "report-none" },
    { value: "weekly", label: t`Weekly`, testId: "report-weekly" },
    { value: "monthly", label: t`Monthly`, testId: "report-monthly" },
  ]
  const busy = send.error instanceof ApiError && send.error.status === 429
  return (
    <Section
      title={<Trans>Notify me</Trans>}
      description={
        <Trans>
          Only for you, by e-mail to {email}. Reports are plain numbers, with no images and nothing that tracks opening
          or clicks.
        </Trans>
      }
    >
      {sub.isPending ? (
        <Skeleton className="h-64 rounded-2xl" />
      ) : !s ? (
        <ErrorLine error={sub.error} />
      ) : (
        <>
          <Card data-testid="analytics-notify">
            <div className="flex flex-col gap-2">
              <span className="text-body font-medium" id="report-frequency">
                <Trans>Report</Trans>
              </span>
              <div className="max-w-full overflow-x-auto">
                <Segmented
                  value={s.report}
                  onChange={(report) => update.mutate({ report })}
                  label={t`Report`}
                  items={reports}
                />
              </div>
              <p className="text-small text-faint">
                {s.report === "weekly" ? (
                  <Trans>Every Monday at 08:00 in {zone}, about the week before.</Trans>
                ) : s.report === "monthly" ? (
                  <Trans>On the 1st of each month at 08:00 in {zone}, about the month before.</Trans>
                ) : (
                  <Trans>No scheduled reports.</Trans>
                )}
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={send.isPending}
                onClick={() => send.mutate()}
                data-testid="send-report"
              >
                <SendIcon />
                <Trans>Send me one now</Trans>
              </Button>
              {send.isSuccess && (
                <span role="status" className="text-small text-success" data-testid="send-report-done">
                  <Trans>On its way to {email}.</Trans>
                </span>
              )}
              {busy ? (
                <span role="alert" className="text-small text-destructive">
                  <Trans>Five reports an hour at most. Try again later.</Trans>
                </span>
              ) : (
                <ErrorLine error={send.error} className="text-small" />
              )}
            </div>
          </Card>
          <Card data-testid="analytics-alert-kinds">
            <span className="text-body font-medium">
              <Trans>Alerts</Trans>
            </span>
            {ALERT_KINDS.map((k) => {
              const on = s.alerts.includes(k)
              const help = text.help[k]
              const off = k === "support_load" && !p.inbox_id
              return (
                <ToggleRow key={k} title={text.kind[k]} hint={off ? t`Needs a linked inbox. ${help}` : help}>
                  <Switch
                    checked={on}
                    disabled={update.isPending}
                    onCheckedChange={(v) =>
                      update.mutate({
                        alerts: ALERT_KINDS.filter((x) => (x === k ? v : s.alerts.includes(x))),
                      })
                    }
                    aria-label={text.kind[k]}
                    data-testid={`alert-kind-${k}`}
                  />
                </ToggleRow>
              )
            })}
            <p className="text-small text-faint">
              <Trans>Checked every hour. Each alert is sent once and also shows as a note on the chart.</Trans>
            </p>
          </Card>
          <ErrorLine error={update.error} />
        </>
      )}
    </Section>
  )
}

function AlertHistory({ p }: { p: AnalyticsProperty }) {
  const { i18n } = useLingui()
  const text = useAlertText()
  const alerts = useAlerts(p.id)
  const items = alerts.data?.pages.flatMap((x) => x.items) ?? []
  return (
    <Section
      title={<Trans>Past alerts</Trans>}
      description={<Trans>Everything this property raised, newest first, whoever was notified.</Trans>}
    >
      <Card flush>
        {alerts.isPending ? (
          <Skeleton className="m-4 h-10" />
        ) : alerts.error ? (
          <ErrorLine error={alerts.error} className="p-4" />
        ) : items.length === 0 ? (
          <EmptyRow>
            <Trans>No alerts yet.</Trans>
          </EmptyRow>
        ) : (
          <Rows data-testid="analytics-alerts">
            {items.map((a) => {
              const Icon = ICONS[a.kind] ?? BellIcon
              return (
                <li key={a.id} className="flex items-start gap-3 px-4 py-3" data-testid={`alert-${a.kind}`}>
                  <span className="grid size-[34px] shrink-0 place-items-center rounded-[10px] border bg-surface text-warning">
                    <Icon className="size-4" />
                  </span>
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="flex flex-wrap items-baseline gap-x-2">
                      <span className="text-body font-medium">{text.kind[a.kind]}</span>
                      <span className="text-caption text-faint tabular-nums">{when(a, i18n.locale, p.timezone)}</span>
                    </span>
                    <span className="text-small break-words text-muted-foreground" dir="auto">
                      {text.sentence(a)}
                    </span>
                  </span>
                </li>
              )
            })}
          </Rows>
        )}
        {alerts.hasNextPage && (
          <div className="flex justify-center border-t p-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void alerts.fetchNextPage()}
              disabled={alerts.isFetchingNextPage}
            >
              <Trans>Load more</Trans>
            </Button>
          </div>
        )}
      </Card>
    </Section>
  )
}

export function AlertsView({ p }: { p: AnalyticsProperty }) {
  return (
    <div className="flex flex-col gap-6" data-testid="analytics-alerts-view">
      <NotifyMe p={p} />
      <AlertHistory p={p} />
    </div>
  )
}

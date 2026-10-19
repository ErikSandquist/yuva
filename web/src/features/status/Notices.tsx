import { Trans, useLingui } from "@lingui/react/macro"
import { MailCheckIcon, MailIcon, MailMinusIcon, MailXIcon, Undo2Icon } from "lucide-react"

import { ErrorLine } from "@/components/common"
import { formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Card, Section } from "@/features/settings/ui"
import { useCancelNotice } from "@/features/status/queries"
import type { StatusNotice, StatusNoticeSkipReason } from "@/lib/api"
import { useSession } from "@/lib/session"

function useReasons(): Record<StatusNoticeSkipReason, string> {
  const { t } = useLingui()
  return {
    rate: t`The page already sent 10 notices in the last hour`,
    suspended: t`The workspace may not send bulk mail now`,
    email_off: t`E-mail subscribers are off`,
    unpublished: t`The page or the incident is not public`,
    sender_not_ready: t`The sending channel cannot send`,
    no_recipients: t`No subscriber matched`,
  }
}

function Counts({ n }: { n: StatusNotice }) {
  const c = n.counts
  const parts: React.ReactNode[] = []
  if (c.recipients > 0) parts.push(<Trans key="r">Recipients: {c.recipients}</Trans>)
  if (c.sent > 0) parts.push(<Trans key="s">Sent: {c.sent}</Trans>)
  if (c.failed > 0) parts.push(<Trans key="f">Failed: {c.failed}</Trans>)
  if (c.bounced > 0) parts.push(<Trans key="b">Bounced: {c.bounced}</Trans>)
  if (c.skipped > 0) parts.push(<Trans key="k">Skipped: {c.skipped}</Trans>)
  if (parts.length === 0) return null
  return (
    <span className="flex flex-wrap gap-x-3 text-small text-faint">
      {parts.map((p, i) => (
        <span key={i} className="tabular-nums">
          {p}
        </span>
      ))}
    </span>
  )
}

function NoticeLine({ n, pageId }: { n: StatusNotice; pageId: string }) {
  const { i18n } = useLingui()
  const { canManage } = useSession()
  const reasons = useReasons()
  const cancel = useCancelNotice(pageId)
  const due = formatDateTime(n.due_at, i18n.locale)
  const Icon =
    n.state === "sent"
      ? MailCheckIcon
      : n.state === "skipped"
        ? MailMinusIcon
        : n.state === "cancelled"
          ? MailXIcon
          : MailIcon
  return (
    <li
      className="flex flex-wrap items-center gap-x-3 gap-y-1.5 border-b px-4 py-3 last:border-b-0"
      data-testid="status-notice"
    >
      <Icon className="size-4 shrink-0 text-faint" aria-hidden />
      <div className="flex min-w-0 flex-1 basis-56 flex-col gap-0.5">
        <span className="text-body font-medium" data-testid="status-notice-state" data-state={n.state}>
          {n.state === "pending" ? (
            <Trans>Scheduled for {due}</Trans>
          ) : n.state === "sending" ? (
            <Trans>Sending</Trans>
          ) : n.state === "sent" ? (
            <Trans>Sent</Trans>
          ) : n.state === "cancelled" ? (
            <Trans>Cancelled</Trans>
          ) : (
            <Trans>Not sent</Trans>
          )}
        </span>
        {n.state === "skipped" && n.skip_reason && (
          <span className="text-small text-muted-foreground">{reasons[n.skip_reason]}</span>
        )}
        <Counts n={n} />
      </div>
      {n.state === "pending" && canManage && (
        <Button
          variant="outline"
          size="sm"
          disabled={cancel.isPending}
          onClick={() => cancel.mutate(n.id)}
          data-testid="status-notice-undo"
        >
          <Undo2Icon />
          <Trans>Undo</Trans>
        </Button>
      )}
      <ErrorLine error={cancel.error} className="basis-full text-caption" />
    </li>
  )
}

export function NoticesSection({ notices, pageId }: { notices: StatusNotice[]; pageId: string }) {
  if (notices.length === 0) return null
  return (
    <Section title={<Trans>Notices to subscribers</Trans>}>
      <Card flush>
        <ul data-testid="status-notices">
          {notices.map((n) => (
            <NoticeLine key={n.id} n={n} pageId={pageId} />
          ))}
        </ul>
      </Card>
    </Section>
  )
}

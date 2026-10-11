import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { CircleAlertIcon, CircleCheckIcon, LoaderIcon, SendIcon, TriangleAlertIcon } from "lucide-react"
import { useState } from "react"
import { Link } from "react-router"

import { ErrorLine, Notice, Segmented } from "@/components/common"
import { BROADCAST_KEYS, keyLabel } from "@/components/common/ShortcutSheet"
import { formatBytes, formatDateTime } from "@/components/common/text"
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { problemOf, useSchedule } from "@/features/broadcasts/broadcastQueries"
import { useBroadcastsStatus, useNumber, utcLine } from "@/features/broadcasts/BroadcastParts"
import { designLink } from "@/features/designs/queries"
import type { Broadcast, BroadcastReview, BroadcastReviewItem, BroadcastReviewKey } from "@/lib/api"
import { ApiError } from "@/lib/api"
import { cn } from "@/lib/utils"

export type Spot = "sender" | "audience" | "content" | "options" | "test"

type Fix = { spot: Spot } | { to: string }

const ORDER: BroadcastReviewKey[] = [
  "approval",
  "sender_identity",
  "sender_ready",
  "audience_nonempty",
  "design",
  "subject",
  "merge_fields",
  "html_size",
  "links",
  "images_alt",
  "click_tracking",
  "daily_cap",
  "test_sent",
]

function useItemText(review: BroadcastReview) {
  const { t, i18n } = useLingui()
  const status = useBroadcastsStatus()
  const size = review.html_bytes !== undefined ? formatBytes(review.html_bytes, i18n.locale) : ""
  const days = review.days ?? 0
  const titles: Record<BroadcastReviewKey, string> = {
    sender_identity: t`Sender details for the footer`,
    sender_ready: t`Sending channel`,
    approval: t`Permission to send`,
    design: t`Design`,
    audience_nonempty: t`Recipients`,
    merge_fields: t`Merge fields`,
    html_size: t`Mail size`,
    images_alt: t`Image descriptions`,
    links: t`Links`,
    subject: t`Subject`,
    daily_cap: t`Daily limit`,
    test_sent: t`Test mail`,
    click_tracking: t`Click tracking`,
  }
  const problem = (it: BroadcastReviewItem): string | null => {
    const blocking = it.status === "blocking"
    switch (it.key) {
      case "sender_identity":
        return t`The inbox has no legal name or postal address yet. Every broadcast ends with them, as the law asks.`
      case "sender_ready":
        return t`The channel cannot send yet. It needs an SMTP account and an address to send from.`
      case "approval":
        return status?.status === "suspended"
          ? t`The workspace may not send broadcasts for now.`
          : status?.status === "approval_requested"
            ? t`The server’s operator has not answered the request yet.`
            : t`The server’s operator has to approve broadcasts for this workspace first.`
      case "design":
        return t`Pick a design that renders without errors.`
      case "audience_nonempty":
        return t`Nobody would get this mail. Pick lists with subscribers, or loosen the conditions.`
      case "merge_fields":
        return t`A merge field without a fallback would be empty for some recipients. Give it a fallback.`
      case "html_size":
        return blocking
          ? t`The mail is ${size}. Gmail cuts mail off above 102 KB, so make it shorter.`
          : t`The mail is ${size}, close to the 102 KB where Gmail cuts it off.`
      case "images_alt":
        return t`Some images have no description for people who cannot see them.`
      case "links":
        return blocking ? t`Some links are not valid.` : t`Some links use http: instead of https:.`
      case "subject":
        return blocking
          ? t`Write a subject of at most 300 characters.`
          : t`Inboxes cut off subjects longer than 80 characters.`
      case "daily_cap":
        return days > 1 ? t`Under the workspace’s daily limit, sending takes ${days} days.` : null
      case "test_sent":
        return t`No test went out since the last change.`
      case "click_tracking":
        return t`Links will point at this server’s link domain instead of your own. Spam filters look at link domains.`
    }
  }
  return { titles, problem }
}

function fixOf(b: Broadcast, key: BroadcastReviewKey): Fix | null {
  switch (key) {
    case "sender_identity":
      return { to: `/settings/inboxes/${b.inbox_id}` }
    case "sender_ready":
      return { spot: "sender" }
    case "approval":
      return { to: "/settings/workspace" }
    case "audience_nonempty":
      return { spot: "audience" }
    case "design":
    case "subject":
      return { spot: "content" }
    case "merge_fields":
    case "html_size":
    case "links":
    case "images_alt":
      return b.design_id ? { to: designLink(b.design_id) } : { spot: "content" }
    case "click_tracking":
      return { spot: "options" }
    case "test_sent":
      return { spot: "test" }
    case "daily_cap":
      return null
  }
}

const icons = {
  ok: <CircleCheckIcon className="mt-0.5 size-4 shrink-0 text-success" />,
  warning: <TriangleAlertIcon className="mt-0.5 size-4 shrink-0 text-warning" />,
  blocking: <CircleAlertIcon className="mt-0.5 size-4 shrink-0 text-destructive" />,
}

function Checklist({
  b,
  review,
  onFix,
  readOnly,
}: {
  b: Broadcast
  review: BroadcastReview
  onFix: (spot: Spot) => void
  readOnly: boolean
}) {
  const { t } = useLingui()
  const { titles, problem } = useItemText(review)
  const items = [...review.items].sort((x, y) => ORDER.indexOf(x.key) - ORDER.indexOf(y.key))
  const rank = { blocking: 0, warning: 1, ok: 2 }
  items.sort((x, y) => rank[x.status] - rank[y.status])
  return (
    <ul className="flex flex-col divide-y" data-testid="review-items">
      {items.map((it) => {
        const text = it.status === "ok" ? null : problem(it)
        const fix = it.status === "ok" || readOnly ? null : fixOf(b, it.key)
        return (
          <li
            key={it.key}
            className="flex gap-2.5 py-2.5"
            data-testid="review-item"
            data-key={it.key}
            data-status={it.status}
          >
            {icons[it.status]}
            <div className="flex min-w-0 flex-1 flex-col gap-0.5">
              <span className={cn("text-body", it.status === "ok" ? "text-muted-foreground" : "font-medium")}>
                {titles[it.key]}
              </span>
              {text && <span className="text-small text-muted-foreground">{text}</span>}
              {fix &&
                ("to" in fix ? (
                  <Link to={fix.to} className="w-fit text-small text-brand underline-offset-4 hover:underline">
                    {t`Fix it`}
                  </Link>
                ) : (
                  <button
                    type="button"
                    onClick={() => onFix(fix.spot)}
                    className="w-fit text-small text-brand underline-offset-4 hover:underline"
                  >
                    {t`Fix it`}
                  </button>
                ))}
            </div>
          </li>
        )
      })}
    </ul>
  )
}

function toLocalInput(d: Date) {
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function ReviewPanel({
  b,
  review,
  reviewError,
  onFix,
  readOnly,
  confirming,
  setConfirming,
}: {
  b: Broadcast
  review: BroadcastReview | undefined
  reviewError: unknown
  onFix: (spot: Spot) => void
  readOnly: boolean
  confirming: boolean
  setConfirming: (on: boolean) => void
}) {
  const { t, i18n } = useLingui()
  const num = useNumber()
  const [mode, setMode] = useState<"now" | "later">("now")
  const [now] = useState(() => Date.now())
  const [at, setAt] = useState(() => toLocalInput(new Date(now + 60 * 60_000)))
  const [confirmed, setConfirmed] = useState<number | null>(null)
  const schedule = useSchedule(b.id)
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone
  const sendAt = mode === "later" && at ? new Date(at) : null
  const atValid =
    mode === "now" ||
    (!!sendAt && !Number.isNaN(sendAt.getTime()) && sendAt.getTime() > now && sendAt.getTime() < now + 90 * 86400_000)
  const ready = !!review?.ready
  const count = confirmed ?? review?.recipients ?? 0
  const recipients = num(count)
  const name = b.name
  const when = sendAt && atValid ? formatDateTime(sendAt.toISOString(), i18n.locale) : ""
  const utc = sendAt && atValid ? utcLine(sendAt.toISOString(), i18n.locale) : ""
  const problem = problemOf(schedule.error)
  const changed = schedule.error instanceof ApiError && problem?.code === "audience_changed"
  const key = keyLabel(BROADCAST_KEYS.send).join("")
  const submit = () => {
    schedule.mutate(
      { send_at: mode === "later" && sendAt ? sendAt.toISOString() : undefined, confirm_recipients: count },
      {
        onSuccess: () => setConfirming(false),
        onError: (err) => {
          const p = problemOf(err)
          if (p?.code === "audience_changed" && p.recipients !== undefined) setConfirmed(p.recipients)
          else setConfirming(false)
        },
      }
    )
  }
  return (
    <aside className="flex flex-col gap-4 rounded-2xl border bg-card p-4" data-testid="review-panel">
      <div className="flex items-center gap-2">
        <h2 className="min-w-0 flex-1 text-body font-semibold">
          <Trans>Review and send</Trans>
        </h2>
        {review && (
          <span
            className={cn("text-caption font-medium", ready ? "text-success" : "text-destructive")}
            data-testid="review-ready"
          >
            {ready ? <Trans>Ready</Trans> : <Trans>Not ready</Trans>}
          </span>
        )}
      </div>
      {review ? (
        <Checklist b={b} review={review} onFix={onFix} readOnly={readOnly} />
      ) : reviewError ? (
        <ErrorLine error={reviewError} />
      ) : (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-8 w-full" />
        </div>
      )}
      {!readOnly && (
        <div className="flex flex-col gap-3 border-t pt-4">
          <Segmented
            value={mode}
            onChange={setMode}
            label={t`When`}
            className="w-full"
            itemClassName="flex-1"
            items={[
              { value: "now", label: t`Send now`, testId: "send-mode-now" },
              { value: "later", label: t`Schedule`, testId: "send-mode-later" },
            ]}
          />
          {mode === "later" && (
            <div className="flex flex-col gap-1.5">
              <Input
                type="datetime-local"
                value={at}
                min={toLocalInput(new Date(now))}
                onChange={(e) => setAt(e.target.value)}
                aria-label={t`Send on`}
                aria-invalid={!atValid || undefined}
                data-testid="send-at"
              />
              <p className="text-small text-faint" data-testid="send-zone">
                {utc ? (
                  <Trans>
                    Your time zone, {zone}. That is {utc} UTC.
                  </Trans>
                ) : (
                  <Trans>Pick a time in the next 90 days, in your time zone, {zone}.</Trans>
                )}
              </p>
            </div>
          )}
          <Button
            onClick={() => {
              schedule.reset()
              setConfirmed(null)
              setConfirming(true)
            }}
            disabled={!ready || !atValid || count === 0}
            title={t`Review, then send or schedule (${key})`}
            data-testid="open-send"
          >
            <SendIcon />
            {mode === "now" ? (
              <Plural value={count} one="Send to # recipient" other="Send to # recipients" />
            ) : (
              <Plural value={count} one="Schedule for # recipient" other="Schedule for # recipients" />
            )}
          </Button>
          {!confirming && schedule.error && !changed && <ErrorLine error={schedule.error} />}
        </div>
      )}
      <AlertDialog open={confirming && atValid && ready} onOpenChange={(o) => !schedule.isPending && setConfirming(o)}>
        <AlertDialogContent data-testid="send-confirm">
          <AlertDialogHeader>
            <AlertDialogTitle>
              {mode === "now" ? (
                <Trans>Send “{name}” now?</Trans>
              ) : (
                <Trans>
                  Send “{name}” on {when}?
                </Trans>
              )}
            </AlertDialogTitle>
            <AlertDialogDescription className="flex flex-col gap-2">
              <span className="font-medium text-foreground" data-testid="confirm-recipients">
                <Plural value={count} one="It goes to # recipient." other="It goes to # recipients." />
              </span>
              <span>
                <Trans>
                  The recipients are taken when sending starts: people who subscribe later do not get it, and anyone who
                  unsubscribes before their mail goes out is skipped. You can pause or cancel while it sends.
                </Trans>
              </span>
            </AlertDialogDescription>
          </AlertDialogHeader>
          {changed && (
            <Notice tone="warning" icon={TriangleAlertIcon} data-testid="audience-changed">
              <p>
                <Trans>
                  The number of recipients changed while you looked. It is {recipients} now. Confirm again to send to
                  them.
                </Trans>
              </p>
            </Notice>
          )}
          {schedule.error && !changed && <ErrorLine error={schedule.error} />}
          <AlertDialogFooter>
            <AlertDialogCancel>
              <Trans>Cancel</Trans>
            </AlertDialogCancel>
            <Button onClick={submit} disabled={schedule.isPending} data-testid="confirm-send">
              {schedule.isPending && <LoaderIcon className="animate-spin" />}
              {mode === "now" ? <Trans>Send now</Trans> : <Trans>Schedule</Trans>}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </aside>
  )
}

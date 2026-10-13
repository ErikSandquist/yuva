import { Trans, useLingui } from "@lingui/react/macro"
import {
  CalendarClockIcon,
  CircleAlertIcon,
  LoaderIcon,
  MegaphoneIcon,
  PauseIcon,
  PlayIcon,
  Trash2Icon,
  Undo2Icon,
  XIcon,
} from "lucide-react"
import { useState } from "react"
import { Link, useNavigate, useParams, useSearchParams } from "react-router"

import { Dot, ErrorLine, Notice, Segmented, useConfirm } from "@/components/common"
import { Pane, PaneBack, PaneEmpty } from "@/components/common/Column"
import { BROADCAST_KEYS, keyLabel } from "@/components/common/ShortcutSheet"
import { formatDateTime, useEnumText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import {
  FINISHED,
  isActive,
  useBroadcast,
  useBroadcastAction,
  useDeleteBroadcast,
  useReview,
  type BroadcastAction,
} from "@/features/broadcasts/broadcastQueries"
import { BroadcastSteps } from "@/features/broadcasts/BroadcastList"
import {
  progressOf,
  ProgressBar,
  StateBadge,
  StatusNotice,
  useNumber,
  utcLine,
  WaitingLine,
} from "@/features/broadcasts/BroadcastParts"
import { Composer } from "@/features/broadcasts/Composer"
import { DesignThumb } from "@/features/broadcasts/DesignPicker"
import { useInboxChannels, useLists } from "@/features/broadcasts/queries"
import { Deliveries, Report } from "@/features/broadcasts/Report"
import { useTestSend } from "@/features/broadcasts/TestSend"
import { designLink } from "@/features/designs/queries"
import { Card, Section } from "@/features/settings/ui"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { Broadcast } from "@/lib/api"
import { useSession } from "@/lib/session"
import { useInboxes, useMemberMap } from "@/lib/workspace"

export function BroadcastsHome() {
  return (
    <PaneEmpty icon={MegaphoneIcon} title={<Trans>Broadcasts</Trans>} testId="broadcasts-home">
      <BroadcastSteps />
    </PaneEmpty>
  )
}

function hint(label: string, key: string) {
  return `${label} (${keyLabel(key).join("")})`
}

function Actions({
  b,
  run,
  pending,
  onDelete,
}: {
  b: Broadcast
  run: (a: BroadcastAction) => void
  pending: boolean
  onDelete: () => void
}) {
  const { t } = useLingui()
  return (
    <div className="flex flex-wrap items-center gap-2" data-testid="broadcast-actions">
      {b.state === "scheduled" && (
        <Button
          variant="outline"
          size="sm"
          disabled={pending}
          onClick={() => run("unschedule")}
          title={hint(t`Back to draft`, BROADCAST_KEYS.unschedule)}
          data-testid="broadcast-unschedule"
        >
          <Undo2Icon />
          <Trans>Back to draft</Trans>
        </Button>
      )}
      {(b.state === "sending" || b.state === "preparing") && (
        <Button
          variant="outline"
          size="sm"
          disabled={pending}
          onClick={() => run("pause")}
          title={hint(t`Pause`, BROADCAST_KEYS.pause)}
          data-testid="broadcast-pause"
        >
          <PauseIcon />
          <Trans>Pause</Trans>
        </Button>
      )}
      {b.state === "paused" && (
        <Button
          size="sm"
          disabled={pending}
          onClick={() => run("resume")}
          title={hint(t`Resume`, BROADCAST_KEYS.pause)}
          data-testid="broadcast-resume"
        >
          <PlayIcon />
          <Trans>Resume</Trans>
        </Button>
      )}
      {(b.state === "scheduled" || isActive(b)) && (
        <Button
          variant="ghost"
          size="sm"
          disabled={pending}
          onClick={() => run("cancel")}
          title={hint(t`Cancel the broadcast`, BROADCAST_KEYS.cancel)}
          data-testid="broadcast-cancel"
        >
          <XIcon />
          <Trans>Cancel broadcast</Trans>
        </Button>
      )}
      {(b.state === "draft" || FINISHED.includes(b.state)) && (
        <Button variant="ghost" size="sm" onClick={onDelete} data-testid="broadcast-delete">
          <Trash2Icon />
          {b.state === "draft" ? <Trans>Delete draft</Trans> : <Trans>Delete</Trans>}
        </Button>
      )}
    </div>
  )
}

function Scheduled({ b }: { b: Broadcast }) {
  const { i18n } = useLingui()
  if (!b.scheduled_at) return null
  const local = formatDateTime(b.scheduled_at, i18n.locale)
  const utc = utcLine(b.scheduled_at, i18n.locale)
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone
  return (
    <Card data-testid="broadcast-scheduled">
      <div className="flex items-start gap-3">
        <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-brand-wash text-brand">
          <CalendarClockIcon className="size-5" />
        </span>
        <div className="flex min-w-0 flex-col gap-0.5">
          <p className="text-reading font-medium">
            <Trans>Sends on {local}</Trans>
          </p>
          <p className="text-small text-faint">
            <Trans>
              {zone} · {utc} UTC. The recipients are taken then; you can go back to the draft until it starts.
            </Trans>
          </p>
        </div>
      </div>
    </Card>
  )
}

function Sending({ b, at }: { b: Broadcast; at: number }) {
  const { t, i18n } = useLingui()
  const text = useEnumText()
  const num = useNumber()
  const channel = useInboxChannels(b.inbox_id).data?.find((c) => c.id === b.channel_id)
  const { done, total } = progressOf(b)
  const rate = channel?.email?.broadcast_rate
  const left = Math.max(0, total - done)
  const sent = num(done)
  const all = num(total)
  const prepared = num(b.counts.prepared ?? 0)
  const end =
    b.state === "sending" && rate && left > 0 && !b.waiting_until
      ? formatDateTime(new Date(at + (left / rate) * 1000).toISOString(), i18n.locale)
      : null
  const reason = b.pause_reason
  return (
    <Card data-testid="broadcast-sending">
      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
          <p className="text-reading font-medium tabular-nums" data-testid="broadcast-sent-count">
            {b.state === "preparing" ? (
              <Trans>Taking the recipients: {prepared} so far</Trans>
            ) : (
              <Trans>
                {sent} of {all} done
              </Trans>
            )}
          </p>
          <p className="text-small text-faint">
            {rate ? t`${rate} mails a second` : null}
            {end && (
              <>
                {" · "}
                <Trans>about done by {end}</Trans>
              </>
            )}
          </p>
        </div>
        <ProgressBar b={b} className="h-2" />
      </div>
      {b.waiting_until && <WaitingLine until={b.waiting_until} />}
      {b.state === "paused" && reason && (
        <Notice tone={reason === "member" ? "info" : "warning"} icon={PauseIcon} data-testid="broadcast-pause-reason">
          <p>{text.pauseReason[reason]}</p>
          {b.state_detail && (
            <pre
              className="overflow-x-auto rounded-lg bg-muted px-2.5 py-1.5 font-mono text-caption whitespace-pre-wrap"
              dir="ltr"
            >
              {b.state_detail}
            </pre>
          )}
        </Notice>
      )}
    </Card>
  )
}

function Summary({ b }: { b: Broadcast }) {
  const { t, i18n } = useLingui()
  const { canManage } = useSession()
  const channel = useInboxChannels(b.inbox_id).data?.find((c) => c.id === b.channel_id)
  const lists = new Map(
    (useLists(b.inbox_id, false).data?.pages.flatMap((p) => p.items) ?? []).map((l) => [l.id, l.name])
  )
  const members = useMemberMap()
  const by = b.scheduled_by ? members.get(b.scheduled_by) : undefined
  const rows: [string, React.ReactNode][] = [
    [t`Subject`, b.subject || "—"],
    [t`Preview text`, b.preheader || t`The design’s own`],
    [t`Sends from`, channel ? `${channel.name} · ${channel.email?.from_address || channel.email?.address}` : "—"],
    [t`Lists`, b.audience.list_ids.map((id) => lists.get(id) ?? "…").join(", ") || "—"],
  ]
  if (b.audience.conditions.length) rows.push([t`Conditions`, String(b.audience.conditions.length)])
  if (b.audience.locales.length) rows.push([t`Languages`, b.audience.locales.join(", ")])
  rows.push([t`Click counting`, b.click_tracking ? t`On` : t`Off`])
  rows.push([
    t`Campaign in links`,
    b.utm ? (
      <span dir="ltr" key="utm">
        {b.utm_campaign}
      </span>
    ) : (
      t`Off`
    ),
  ])
  if (by) rows.push([t`Scheduled by`, by.name || by.email])
  else if (b.scheduled_by_api_key) rows.push([t`Scheduled by`, t`An API key or assistant`])
  if (b.started_at) rows.push([t`Started`, formatDateTime(b.started_at, i18n.locale)])
  if (b.finished_at) rows.push([t`Finished`, formatDateTime(b.finished_at, i18n.locale)])
  return (
    <Section title={b.started_at ? <Trans>What went out</Trans> : <Trans>What will go out</Trans>}>
      <Card>
        <div className="flex flex-wrap items-start gap-5 phone:flex-col" data-testid="broadcast-summary">
          {b.design_id &&
            (canManage ? (
              <Link to={designLink(b.design_id)} title={t`Open the design`}>
                <DesignThumb designId={b.design_id} inboxId={b.inbox_id} />
              </Link>
            ) : (
              <DesignThumb designId={b.design_id} inboxId={b.inbox_id} />
            ))}
          <dl className="grid min-w-0 flex-1 grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-body">
            {rows.map(([label, value]) => (
              <div key={label} className="contents">
                <dt className="text-faint">{label}</dt>
                <dd className="min-w-0 break-words">{value}</dd>
              </div>
            ))}
          </dl>
        </div>
      </Card>
    </Section>
  )
}

const TABS = ["report", "recipients"] as const
type Tab = (typeof TABS)[number]

export function BroadcastPage() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { broadcastId = "" } = useParams()
  const [params, setParams] = useSearchParams()
  const { canManage } = useSession()
  const broadcast = useBroadcast(broadcastId)
  const b = broadcast.data
  const inbox = (useInboxes().data ?? []).find((i) => i.id === b?.inbox_id)
  const action = useBroadcastAction(broadcastId)
  const remove = useDeleteBroadcast()
  const [confirm, confirmDialog] = useConfirm()
  const [confirming, setConfirming] = useState(false)
  const draft = b?.state === "draft"
  const review = useReview(b ?? ({ id: broadcastId, updated_at: "" } as Broadcast), !!draft)
  const test = useTestSend(broadcastId)
  const raw = params.get("tab")
  const tab: Tab = TABS.includes(raw as Tab) && canManage ? (raw as Tab) : "report"
  const setTab = (v: Tab) => setParams(v === "report" ? {} : { tab: v }, { replace: true })
  const name = b?.name ?? ""

  const run = (a: BroadcastAction) => {
    if (a === "cancel")
      confirm({
        title: <Trans>Cancel “{name}”?</Trans>,
        description: (
          <Trans>
            Sending stops within a few seconds and cannot start again. Mail that already went out stays sent.
          </Trans>
        ),
        confirm: <Trans>Cancel broadcast</Trans>,
        run: () => action.mutate("cancel"),
      })
    else action.mutate(a)
  }
  const onDelete = () =>
    confirm({
      title: <Trans>Delete “{name}”?</Trans>,
      description: (
        <Trans>Its report and the list of recipients are deleted. Conversations opened by replies stay.</Trans>
      ),
      confirm: <Trans>Delete</Trans>,
      run: () => remove.mutate(broadcastId, { onSuccess: () => navigate("/broadcasts") }),
    })

  useHotkeys(
    b
      ? {
          ...(draft && b.design_id ? { [BROADCAST_KEYS.design]: () => navigate(designLink(b.design_id!)) } : {}),
          ...(draft ? { [BROADCAST_KEYS.test]: test.run } : {}),
          ...(draft && review.data?.ready ? { [BROADCAST_KEYS.send]: () => setConfirming(true) } : {}),
          ...(b.state === "scheduled" ? { [BROADCAST_KEYS.unschedule]: () => run("unschedule") } : {}),
          ...(b.state === "sending" || b.state === "preparing" ? { [BROADCAST_KEYS.pause]: () => run("pause") } : {}),
          ...(b.state === "paused" ? { [BROADCAST_KEYS.pause]: () => run("resume") } : {}),
          ...(b.state === "scheduled" || isActive(b) ? { [BROADCAST_KEYS.cancel]: () => run("cancel") } : {}),
          ...(!draft
            ? {
                [BROADCAST_KEYS.report]: () => setTab("report"),
                [BROADCAST_KEYS.deliveries]: () => setTab("recipients"),
              }
            : {}),
        }
      : {},
    canManage && !!b
  )

  if (broadcast.isPending) {
    return (
      <Pane wide className="gap-4">
        <Skeleton className="h-8 w-60" />
        <Skeleton className="h-40 w-full rounded-2xl" />
      </Pane>
    )
  }
  if (!b) {
    return (
      <Pane wide>
        <PaneBack to="/broadcasts" label={t`Broadcasts`} />
        <ErrorLine error={broadcast.error} />
      </Pane>
    )
  }
  return (
    <Pane wide testId="broadcast-page" key={b.id} className="@container">
      <header className="flex flex-col">
        <PaneBack to="/broadcasts" label={t`Broadcasts`} />
        <div className="flex flex-wrap items-start justify-between gap-3">
          <h1 className="min-w-0 text-page break-words" data-testid="broadcast-name-heading">
            {b.name}
          </h1>
          {canManage && <Actions b={b} run={run} pending={action.isPending} onDelete={onDelete} />}
        </div>
        <p className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-body text-muted-foreground">
          <StateBadge state={b.state} />
          {inbox && (
            <span className="inline-flex items-center gap-1.5">
              <Dot color={inbox.branding.color} className="size-[7px] rounded-[2px]" />
              {inbox.name}
            </span>
          )}
        </p>
      </header>
      <ErrorLine error={action.error ?? remove.error} />
      {draft && <StatusNotice />}
      {b.state === "failed" && (
        <Notice tone="danger" icon={CircleAlertIcon} data-testid="broadcast-failed">
          <p>
            <Trans>Sending stopped and cannot go on, for example because its channel was removed.</Trans>
          </p>
          {b.state_detail && <p className="text-muted-foreground">{b.state_detail}</p>}
        </Notice>
      )}
      {draft ? (
        <Composer b={b} readOnly={!canManage} test={test} confirming={confirming} setConfirming={setConfirming} />
      ) : (
        <>
          {b.state === "scheduled" && <Scheduled b={b} />}
          {isActive(b) && <Sending b={b} at={broadcast.dataUpdatedAt} />}
          {b.state !== "scheduled" && canManage && (
            <Segmented
              value={tab}
              onChange={setTab}
              label={t`Broadcast`}
              items={[
                { value: "report", label: t`Report`, testId: "broadcast-tab-report" },
                { value: "recipients", label: t`Recipients`, testId: "broadcast-tab-recipients" },
              ]}
            />
          )}
          {b.state !== "scheduled" && (tab === "report" ? <Report b={b} /> : <Deliveries b={b} />)}
          <Summary b={b} />
        </>
      )}
      {action.isPending && (
        <span className="fixed bottom-6 left-1/2 inline-flex -translate-x-1/2 items-center gap-1.5 rounded-full bg-foreground px-3 py-1.5 text-caption text-background shadow-lg">
          <LoaderIcon className="size-3 animate-spin" />
          <Trans>Working…</Trans>
        </span>
      )}
      {confirmDialog}
    </Pane>
  )
}

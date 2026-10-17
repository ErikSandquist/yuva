import { Trans, useLingui } from "@lingui/react/macro"
import { ChartLineIcon, CheckIcon, LoaderIcon, MailWarningIcon, MousePointerClickIcon } from "lucide-react"
import { useCallback, useEffect, useRef, useState } from "react"
import { Link } from "react-router"

import { ErrorLine, Notice, PersonAvatar } from "@/components/common"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { AudienceBuilder, AudienceCount, conditionValid, type Audience } from "@/features/broadcasts/AudienceBuilder"
import { useAudienceCount, useReview, useUpdateBroadcast } from "@/features/broadcasts/broadcastQueries"
import { senderState, type SenderState } from "@/features/broadcasts/BroadcastParts"
import { DesignPicker } from "@/features/broadcasts/DesignPicker"
import { useInboxChannels, useLists } from "@/features/broadcasts/queries"
import { hasPace, PaceFields, paceInvalid, paceOf, paceTextOf, type PaceText } from "@/features/broadcasts/Pace"
import { ReviewPanel, type Spot } from "@/features/broadcasts/ReviewPanel"
import { TestSend, type useTestSend } from "@/features/broadcasts/TestSend"
import { useDesign } from "@/features/designs/queries"
import { formatDuration } from "@/components/common/text"
import { Card, Field, Section, ToggleRow } from "@/features/settings/ui"
import type { Broadcast, BroadcastUpdate, Channel } from "@/lib/api"
import { cn } from "@/lib/utils"

const NONE = "none"
const SUBJECT_MAX = 300
const SUBJECT_SOFT = 80
const PREHEADER_MAX = 300
const CAMPAIGN = /^[a-z0-9][a-z0-9_.-]*$/

type Draft = {
  name: string
  subject: string
  preheader: string
  channel_id: string
  design_id: string
  audience: Audience
  click_tracking: boolean
  utm: boolean
  utm_campaign: string
  pace_on: boolean
  pace: PaceText
}

function draftOf(b: Broadcast): Draft {
  return {
    name: b.name,
    subject: b.subject,
    preheader: b.preheader,
    channel_id: b.channel_id ?? "",
    design_id: b.design_id ?? "",
    audience: b.audience,
    click_tracking: b.click_tracking,
    utm: b.utm,
    utm_campaign: b.utm_campaign,
    pace_on: hasPace(b.pace),
    pace: paceTextOf(b.pace),
  }
}

function audienceInput(a: Audience) {
  return { list_ids: a.list_ids, conditions: a.conditions.filter(conditionValid), locales: a.locales }
}

function useAutosave(id: string) {
  const update = useUpdateBroadcast(id)
  const pending = useRef<BroadcastUpdate>({})
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const [waiting, setWaiting] = useState(false)
  const { mutate } = update
  const flush = useCallback(() => {
    clearTimeout(timer.current)
    const body = pending.current
    pending.current = {}
    setWaiting(false)
    if (Object.keys(body).length > 0) mutate(body)
  }, [mutate])
  const save = useCallback(
    (patch: BroadcastUpdate, delay = 700) => {
      pending.current = { ...pending.current, ...patch }
      setWaiting(true)
      clearTimeout(timer.current)
      timer.current = setTimeout(flush, delay)
    },
    [flush]
  )
  useEffect(() => flush, [flush])
  return { save, flush, saving: waiting || update.isPending, error: update.error }
}

export function SaveState({ saving, error }: { saving: boolean; error: unknown }) {
  if (error) return <ErrorLine error={error} className="text-small" />
  return (
    <span
      className="inline-flex items-center gap-1.5 text-small text-faint"
      role="status"
      data-testid="draft-save-state"
    >
      {saving ? (
        <>
          <LoaderIcon className="size-3.5 animate-spin" />
          <Trans>Saving…</Trans>
        </>
      ) : (
        <>
          <CheckIcon className="size-3.5" />
          <Trans>Draft saved</Trans>
        </>
      )}
    </span>
  )
}

function useSenderText(): Record<SenderState, string> {
  const { t } = useLingui()
  return {
    ready: t`Ready to send`,
    no_smtp: t`No SMTP account`,
    no_from: t`Catch-all without a From address`,
  }
}

function SenderPicker({
  b,
  value,
  onChange,
  disabled,
}: {
  b: Broadcast
  value: string
  onChange: (id: string | null) => void
  disabled: boolean
}) {
  const { t } = useLingui()
  const stateText = useSenderText()
  const channels = (useInboxChannels(b.inbox_id).data ?? []).filter((c) => c.kind === "email")
  const current = channels.find((c) => c.id === value)
  const items: Record<string, string> = { [NONE]: t`No channel yet` }
  for (const c of channels) items[c.id] = c.email ? `${c.name} · ${c.email.from_address || c.email.address}` : c.name
  const state = current ? senderState(current) : null
  return (
    <div className="flex flex-col gap-3">
      <Field label={<Trans>Sends from</Trans>}>
        <Select
          value={value || NONE}
          items={items}
          disabled={disabled}
          onValueChange={(v) => onChange(!v || v === NONE ? null : String(v))}
        >
          <SelectTrigger className="w-full min-w-0" aria-label={t`Sends from`} data-testid="broadcast-channel">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={NONE}>{t`No channel yet`}</SelectItem>
            {channels.map((c) => {
              const s = senderState(c)
              return (
                <SelectItem key={c.id} value={c.id} disabled={s !== "ready"}>
                  <span className="flex min-w-0 flex-col">
                    <span className="truncate">{c.name}</span>
                    <small className="truncate text-caption text-faint">
                      {c.email?.from_address || c.email?.address}
                      {s !== "ready" && <> · {stateText[s]}</>}
                    </small>
                  </span>
                </SelectItem>
              )
            })}
          </SelectContent>
        </Select>
      </Field>
      {current && state === "ready" && <SenderLine ch={current} />}
      {(channels.length === 0 ||
        (current && state !== "ready") ||
        channels.every((c) => senderState(c) !== "ready")) && (
        <Notice tone="warning" icon={MailWarningIcon} data-testid="broadcast-sender-missing">
          <p>
            {channels.length === 0 ? (
              <Trans>This inbox has no e-mail channel. Add one with an SMTP account to send broadcasts.</Trans>
            ) : (
              <Trans>
                A broadcast goes out through an e-mail channel with an SMTP account, ideally a separate one for
                broadcasts so complaints never reach your support mail.
              </Trans>
            )}
          </p>
          <Link
            to={`/settings/inboxes/${b.inbox_id}`}
            className="w-fit text-brand underline-offset-4 hover:underline"
            data-testid="broadcast-inbox-settings"
          >
            <Trans>Open the inbox settings</Trans>
          </Link>
        </Notice>
      )}
    </div>
  )
}

function SenderLine({ ch }: { ch: Channel }) {
  const { t } = useLingui()
  const e = ch.email
  if (!e) return null
  const rate = e.broadcast_rate
  return (
    <p
      className="flex flex-wrap items-center gap-x-2 gap-y-1 text-small text-muted-foreground"
      data-testid="sender-line"
    >
      <span className="inline-flex items-center gap-1 text-success">
        <CheckIcon className="size-3.5" />
        <Trans>Ready to send</Trans>
      </span>
      <span aria-hidden>·</span>
      <span>
        {e.broadcast_smtp ? (
          <Trans>through its own broadcast account</Trans>
        ) : (
          <Trans>through the support account</Trans>
        )}
      </span>
      <span aria-hidden>·</span>
      <span>{t`${rate} mails a second`}</span>
    </p>
  )
}

const asSeen = (s: string) =>
  s
    .replace(/\{\{\s*[^|}]*?\s*(?:\|\s*"([^"]*)"\s*)?\}\}/g, (_, fallback?: string) => fallback ?? "")
    .replace(/\s{2,}/g, " ")

function InboxRow({
  sender,
  address,
  subject,
  preheader,
}: {
  sender: string
  address: string
  subject: string
  preheader: string
}) {
  const { t } = useLingui()
  return (
    <div className="flex flex-col gap-1.5" data-testid="inbox-preview">
      <span className="text-small font-medium text-faint">
        <Trans>In the inbox</Trans>
      </span>
      <div className="flex items-start gap-3 rounded-xl border bg-background px-3.5 py-3">
        <PersonAvatar name={sender || address || "?"} className="size-9" />
        <div className="min-w-0 flex-1">
          <div className="flex items-baseline gap-2">
            <span className="min-w-0 flex-1 truncate text-body font-semibold">{sender || address || t`Your name`}</span>
            <span className="shrink-0 text-caption text-faint">{t`now`}</span>
          </div>
          <p className="truncate text-body font-medium">{asSeen(subject) || t`The subject`}</p>
          <p className="truncate text-small text-faint">
            {asSeen(preheader) || t`The first line of the mail shows here when there is no preview text.`}
          </p>
        </div>
      </div>
    </div>
  )
}

function Counter({ value, max, soft }: { value: string; max: number; soft?: number }) {
  const n = [...value].length
  return (
    <span
      className={cn(
        "text-caption tabular-nums",
        n > max ? "text-destructive" : soft && n > soft ? "text-warning" : "text-faint"
      )}
    >
      {n}/{max}
    </span>
  )
}

export function Composer({
  b,
  readOnly,
  test,
  confirming,
  setConfirming,
}: {
  b: Broadcast
  readOnly: boolean
  test: ReturnType<typeof useTestSend>
  confirming: boolean
  setConfirming: (on: boolean) => void
}) {
  const { t, i18n } = useLingui()
  const [d, setD] = useState<Draft>(() => draftOf(b))
  const auto = useAutosave(b.id)
  const count = useAudienceCount(b, true)
  const review = useReview(b, true)
  const lists = useLists(b.inbox_id, false)
  const listNames = new Map((lists.data?.pages.flatMap((p) => p.items) ?? []).map((l) => [l.id, l.name]))
  const channel = useInboxChannels(b.inbox_id).data?.find((c) => c.id === d.channel_id)
  const design = useDesign(d.design_id)
  const designPreheader = d.design_id ? (design.data?.document.preheader ?? "") : ""
  const set = (patch: Partial<Draft>, save: BroadcastUpdate, delay?: number) => {
    setD((x) => ({ ...x, ...patch }))
    if (!readOnly) auto.save(save, delay)
  }
  const fix = (spot: Spot) => {
    const el = document.getElementById(`broadcast-${spot}`)
    el?.scrollIntoView({ block: "start", behavior: "smooth" })
    el?.querySelector<HTMLElement>("input, button, [role=combobox]")?.focus({ preventScroll: true })
  }
  const paceSecs = d.pace_on && hasPace(paceOf(d.pace)) ? review.data?.duration_seconds : undefined
  const paceDuration = paceSecs === undefined ? "" : formatDuration(paceSecs, i18n.locale)
  const merge = '{{ contact.first_name | "there" }}'
  const campaignBad = !CAMPAIGN.test(d.utm_campaign)
  return (
    <div className="grid items-start gap-6 @4xl:grid-cols-[minmax(0,1fr)_20rem]" data-testid="broadcast-composer">
      <div className="flex min-w-0 flex-col gap-8">
        {!readOnly && (
          <div className="-mb-4 flex justify-end">
            <SaveState saving={auto.saving} error={auto.error} />
          </div>
        )}
        <Section>
          <Card>
            <Field label={<Trans>Name</Trans>} htmlFor="broadcast-name" hint={<Trans>Only your team sees it.</Trans>}>
              <Input
                id="broadcast-name"
                value={d.name}
                maxLength={200}
                disabled={readOnly}
                onChange={(e) =>
                  set({ name: e.target.value }, e.target.value.trim() ? { name: e.target.value.trim() } : {})
                }
                data-testid="broadcast-name"
              />
            </Field>
          </Card>
        </Section>
        <Section title={<Trans>Sender</Trans>}>
          <Card id="broadcast-sender">
            <SenderPicker
              b={b}
              value={d.channel_id}
              disabled={readOnly}
              onChange={(id) => set({ channel_id: id ?? "" }, { channel_id: id }, 0)}
            />
          </Card>
        </Section>
        <Section
          title={<Trans>Audience</Trans>}
          description={
            <Trans>
              Only people who subscribed get it. Unsubscribed, unconfirmed, bounced and blocked addresses are always
              left out.
            </Trans>
          }
        >
          <Card id="broadcast-audience">
            <div className="grid gap-5 @2xl:grid-cols-[minmax(0,1fr)_15rem]">
              <AudienceBuilder
                inboxId={b.inbox_id}
                value={d.audience}
                disabled={readOnly}
                onChange={(audience) => set({ audience }, { audience: audienceInput(audience) })}
              />
              <div className="@2xl:sticky @2xl:top-4 @2xl:self-start">
                <AudienceCount
                  count={d.audience.list_ids.length ? count.data : undefined}
                  loading={count.isPending && d.audience.list_ids.length > 0}
                  stale={auto.saving || count.isFetching}
                  error={count.error ? <ErrorLine error={count.error} /> : null}
                  listNames={listNames}
                />
              </div>
            </div>
          </Card>
        </Section>
        <Section title={<Trans>Content</Trans>}>
          <Card id="broadcast-content">
            <DesignPicker
              inboxId={b.inbox_id}
              value={d.design_id}
              disabled={readOnly}
              onChange={(id) => set({ design_id: id ?? "" }, { design_id: id }, 0)}
            />
            <Field
              label={
                <span className="flex w-full items-center justify-between gap-2">
                  <Trans>Subject</Trans>
                  <Counter value={d.subject} max={SUBJECT_MAX} soft={SUBJECT_SOFT} />
                </span>
              }
              htmlFor="broadcast-subject"
              hint={<Trans>Merge fields work here too, for example {merge}.</Trans>}
            >
              <Input
                id="broadcast-subject"
                value={d.subject}
                maxLength={SUBJECT_MAX}
                disabled={readOnly}
                placeholder={t`What is new in October`}
                onChange={(e) => set({ subject: e.target.value }, { subject: e.target.value })}
                data-testid="broadcast-subject"
              />
            </Field>
            <Field
              label={
                <span className="flex w-full items-center justify-between gap-2">
                  <Trans>Preview text</Trans>
                  <Counter value={d.preheader} max={PREHEADER_MAX} />
                </span>
              }
              htmlFor="broadcast-preheader"
              hint={<Trans>Shown after the subject in most inboxes. Empty uses the design’s own.</Trans>}
            >
              <Input
                id="broadcast-preheader"
                value={d.preheader}
                maxLength={PREHEADER_MAX}
                disabled={readOnly}
                placeholder={designPreheader || t`A line that makes people open it`}
                onChange={(e) => set({ preheader: e.target.value }, { preheader: e.target.value })}
                data-testid="broadcast-preheader"
              />
            </Field>
            <InboxRow
              sender={channel?.email?.display_name || channel?.name || ""}
              address={channel?.email?.from_address || channel?.email?.address || ""}
              subject={d.subject}
              preheader={d.preheader || designPreheader}
            />
          </Card>
        </Section>
        <Section title={<Trans>Options</Trans>}>
          <Card id="broadcast-options">
            <ToggleRow
              title={
                <span className="inline-flex items-center gap-1.5">
                  <MousePointerClickIcon className="size-4 text-faint" />
                  <Trans>Count clicks</Trans>
                </span>
              }
              hint={
                <Trans>
                  Off by default. When on, every link goes through this server’s link domain first, so the report can
                  count who opened which link. Clicks are approximate: security scanners that open every link are left
                  out. Yuva never counts opens.
                </Trans>
              }
            >
              <Switch
                checked={d.click_tracking}
                disabled={readOnly}
                onCheckedChange={(on) => set({ click_tracking: on }, { click_tracking: on }, 0)}
                aria-label={t`Count clicks`}
                data-testid="broadcast-click-tracking"
              />
            </ToggleRow>
            <ToggleRow
              title={
                <span className="inline-flex items-center gap-1.5">
                  <ChartLineIcon className="size-4 text-faint" />
                  <Trans>Campaign in links</Trans>
                </span>
              }
              hint={
                <Trans>
                  On by default. Links to your sites get utm_source=newsletter, utm_medium=email and the campaign name,
                  so Analytics shows the visits this broadcast brings. Links with their own utm_ parameters keep them.
                </Trans>
              }
            >
              <Switch
                checked={d.utm}
                disabled={readOnly}
                onCheckedChange={(on) => set({ utm: on }, { utm: on }, 0)}
                aria-label={t`Campaign in links`}
                data-testid="broadcast-utm"
              />
            </ToggleRow>
            {d.utm && (
              <Field
                label={<Trans>Campaign name</Trans>}
                htmlFor="broadcast-utm-campaign"
                hint={
                  campaignBad ? undefined : (
                    <Trans>
                      Lowercase letters, digits, dashes, underscores and dots. Fixed once the broadcast is sent.
                    </Trans>
                  )
                }
                error={
                  campaignBad ? (
                    <Trans>
                      Use lowercase letters, digits, dashes, underscores and dots, starting with a letter or digit.
                    </Trans>
                  ) : undefined
                }
              >
                <Input
                  id="broadcast-utm-campaign"
                  dir="ltr"
                  value={d.utm_campaign}
                  maxLength={100}
                  disabled={readOnly}
                  aria-invalid={campaignBad || undefined}
                  onChange={(e) => {
                    const v = e.target.value.toLowerCase().replace(/\s+/g, "-")
                    set({ utm_campaign: v }, CAMPAIGN.test(v) ? { utm_campaign: v } : {})
                  }}
                  data-testid="broadcast-utm-campaign"
                />
              </Field>
            )}
          </Card>
        </Section>
        <Section title={<Trans>Sending pace</Trans>}>
          <Card id="broadcast-pace">
            <ToggleRow
              title={<Trans>Limit the pace</Trans>}
              hint={
                <Trans>
                  Off sends as fast as the channel and the workspace’s daily limit allow. On spreads the mails evenly,
                  so no 60 minutes, 24 hours or 30 days get more than you set.
                </Trans>
              }
            >
              <Switch
                checked={d.pace_on}
                disabled={readOnly}
                onCheckedChange={(on) =>
                  on
                    ? setD((x) => ({ ...x, pace_on: true }))
                    : set({ pace_on: false, pace: paceTextOf(undefined) }, { pace: {} }, 0)
                }
                aria-label={t`Limit the pace`}
                data-testid="broadcast-pace-on"
              />
            </ToggleRow>
            {d.pace_on && (
              <>
                <PaceFields
                  value={d.pace}
                  disabled={readOnly}
                  rate={channel?.email?.broadcast_rate}
                  onChange={(pace) =>
                    set({ pace }, paceInvalid(pace) || !hasPace(paceOf(pace)) ? {} : { pace: paceOf(pace) })
                  }
                />
                {paceSecs !== undefined && (
                  <p className="text-small text-muted-foreground" data-testid="broadcast-pace-duration">
                    <Trans>At this pace, sending takes about {paceDuration}.</Trans>
                  </p>
                )}
              </>
            )}
          </Card>
        </Section>
        {!readOnly && (
          <Section title={<Trans>Test</Trans>}>
            <Card id="broadcast-test">
              <TestSend b={b} state={test} disabled={readOnly} />
            </Card>
          </Section>
        )}
      </div>
      <div className="@4xl:sticky @4xl:top-0 @4xl:self-start">
        <ReviewPanel
          b={b}
          review={review.data}
          reviewError={review.error}
          onFix={fix}
          readOnly={readOnly}
          confirming={confirming}
          setConfirming={(on) => {
            if (on) auto.flush()
            setConfirming(on)
          }}
        />
      </div>
    </div>
  )
}

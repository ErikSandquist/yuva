import { Trans, useLingui } from "@lingui/react/macro"
import { CheckIcon, MoreHorizontalIcon, PencilIcon, SendIcon, SquareStackIcon, Trash2Icon } from "lucide-react"
import { useRef, useState } from "react"
import { useNavigate, useParams, useSearchParams } from "react-router"

import { ErrorLine, toast, useConfirm } from "@/components/common"
import { keyLabel, STATUS_KEYS } from "@/components/common/ShortcutSheet"
import { formatDateTime, formatDuration, formatMs, formatShort } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import {
  IncidentsFrame,
  useIncidentCounts,
  useScopeIncidents,
  useStatusScope,
  viewLink,
  viewOf,
} from "@/features/status/IncidentsPage"
import { IncidentDialog } from "@/features/status/IncidentDialog"
import { LinkedConversations } from "@/features/status/LinkedSection"
import { NotifyCheck } from "@/features/status/NotifyCheck"
import { NoticesSection } from "@/features/status/Notices"
import { FrameButton, Pill, ProductTag } from "@/features/status/PageFrame"
import {
  statusLink,
  useCancelNotice,
  useIncidentMutations,
  useMonitorSummary,
  useStatusComponents,
  useStatusIncident,
  useStatusMonitors,
  useStatusNotices,
  useStatusPage,
} from "@/features/status/queries"
import {
  INCIDENT_STATES,
  isClosed,
  MAINTENANCE_STATES,
  nextStates,
  PlainText,
  useStatusText,
} from "@/features/status/text"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useNow } from "@/hooks/use-now"
import type { StatusComponentStateValue, StatusIncident, StatusIncidentState, StatusIncidentUpdate } from "@/lib/api"
import { cn } from "@/lib/utils"
import { useInboxes, useMemberMap } from "@/lib/workspace"

function updateDot(state: StatusIncidentState) {
  if (state === "resolved" || state === "completed") return "bg-success"
  if (state === "scheduled") return "bg-brand"
  return "bg-warning"
}

function Stepper({
  i,
  value,
  onPick,
}: {
  i: StatusIncident
  value: StatusIncidentState
  onPick?: (s: StatusIncidentState) => void
}) {
  const { t } = useLingui()
  const text = useStatusText()
  const hint: Record<StatusIncidentState, string> = {
    investigating: t`Looking into it`,
    identified: t`Cause found`,
    monitoring: t`Fix is out`,
    resolved: t`All good`,
    scheduled: t`Planned`,
    in_progress: t`Work is going on`,
    completed: t`Done`,
  }
  const stages = i.kind === "incident" ? INCIDENT_STATES : MAINTENANCE_STATES
  const allowed = nextStates(i)
  const at = stages.indexOf(value)
  return (
    <div
      role="radiogroup"
      aria-label={t`Stage`}
      className="grid overflow-hidden rounded-xl border bg-card @2xl:grid-flow-col @2xl:auto-cols-fr"
      data-testid="status-stepper"
    >
      {stages.map((s, n) => {
        const done = n < at
        const here = n === at
        const can = !!onPick && allowed.includes(s)
        return (
          <button
            key={s}
            type="button"
            role="radio"
            aria-checked={here}
            disabled={!can}
            onClick={() => onPick?.(s)}
            className={cn(
              "flex items-center gap-2.5 border-b px-3.5 py-3 text-start outline-none transition-colors last:border-b-0 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset @2xl:border-s @2xl:border-b-0 @2xl:first:border-s-0",
              here ? "bg-warning/8 text-foreground" : done ? "text-foreground" : "text-faint",
              can && !here && "hover:bg-muted",
              !can && "cursor-default"
            )}
            data-testid={`status-stage-${s}`}
          >
            <span
              className={cn(
                "grid size-6 shrink-0 place-items-center rounded-full text-caption font-semibold",
                here ? "bg-warning text-white" : done ? "bg-success text-white" : "border"
              )}
              aria-hidden
            >
              {done ? <CheckIcon className="size-3.5" /> : n + 1}
            </span>
            <span className="flex min-w-0 flex-col leading-tight">
              <span className="truncate text-body font-semibold">{text.incident[s]}</span>
              <span className="truncate text-caption opacity-75">{hint[s]}</span>
            </span>
          </button>
        )
      })}
    </div>
  )
}

function UpdateItem({ i, u, pageId }: { i: StatusIncident; u: StatusIncidentUpdate; pageId: string }) {
  const { t, i18n } = useLingui()
  const text = useStatusText()
  const members = useMemberMap()
  const m = useIncidentMutations(pageId)
  const [editing, setEditing] = useState(false)
  const [body, setBody] = useState(u.body)
  const author = u.author_member_id
    ? members.get(u.author_member_id)?.name
    : u.author_api_key_id
      ? t`API key`
      : undefined
  return (
    <li className="group/update flex gap-3.5" data-testid="status-update">
      <span className="flex shrink-0 flex-col items-center" aria-hidden>
        <span className={cn("mt-1.5 size-2.5 rounded-full", updateDot(u.state))} />
        <span className="w-px flex-1 bg-border" />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-0.5 pb-4.5">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-caption text-faint">
          <b className="text-body font-semibold text-foreground">{text.incident[u.state]}</b>
          <time dateTime={u.created_at} title={formatDateTime(u.created_at, i18n.locale)}>
            {formatShort(u.created_at, i18n.locale)}
          </time>
          {author ? (
            <span>· {author}</span>
          ) : (
            <Pill tone="warning">
              <Trans>automatic</Trans>
            </Pill>
          )}
          {!i.published_at && (
            <Pill tone="muted">
              <Trans>not public</Trans>
            </Pill>
          )}
          {u.edited_at && (
            <span title={formatDateTime(u.edited_at, i18n.locale)} data-testid="status-update-edited">
              · <Trans>edited</Trans>
            </span>
          )}
          {!editing && u.body && (
            <Button
              variant="ghost"
              size="icon-xs"
              className="ms-auto"
              aria-label={t`Edit this update`}
              onClick={() => {
                setBody(u.body)
                setEditing(true)
              }}
              data-testid="status-update-edit"
            >
              <PencilIcon />
            </Button>
          )}
        </div>
        {editing ? (
          <form
            className="flex flex-col gap-2"
            onSubmit={(e) => {
              e.preventDefault()
              if (body.trim())
                m.editUpdate.mutate(
                  { id: i.id, updateId: u.id, body: body.trim() },
                  { onSuccess: () => setEditing(false) }
                )
            }}
          >
            <Textarea
              value={body}
              maxLength={5000}
              rows={3}
              autoFocus
              onChange={(e) => setBody(e.target.value)}
              aria-label={t`Update text`}
              data-testid="status-update-text"
            />
            <ErrorLine error={m.editUpdate.error} className="text-caption" />
            <div className="flex justify-end gap-2">
              <Button type="button" variant="ghost" size="sm" onClick={() => setEditing(false)}>
                <Trans>Cancel</Trans>
              </Button>
              <Button
                type="submit"
                size="sm"
                disabled={!body.trim() || m.editUpdate.isPending}
                data-testid="status-update-save"
              >
                <Trans>Save</Trans>
              </Button>
            </div>
          </form>
        ) : (
          u.body && <PlainText text={u.body} className="text-body text-muted-foreground" />
        )}
      </div>
    </li>
  )
}

function MonitorSeen({ pageId, incident }: { pageId: string; incident: StatusIncident }) {
  const { t, i18n } = useLingui()
  const monitors = useStatusMonitors(pageId).data ?? []
  const touched = new Set(incident.components.map((c) => c.component_id))
  const m =
    monitors.find((x) => x.incident_id === incident.id) ??
    monitors.find((x) => (x.kind === "http" || x.kind === "tcp") && touched.has(x.component_id))
  const summary = useMonitorSummary(pageId, m?.id ?? "", "24h", !!m && (m.kind === "http" || m.kind === "tcp")).data
  if (!m) return null
  const buckets = summary?.buckets ?? []
  const size = Math.max(1, Math.ceil(buckets.length / 24))
  const groups = Array.from({ length: Math.ceil(buckets.length / size) }, (_, n) =>
    buckets.slice(n * size, n * size + size)
  ).map((g) => ({
    ms: Math.max(0, ...g.map((b) => b.p50_ms ?? 0)),
    failed: g.some((b) => b.failures > 0),
    checks: g.reduce((n, b) => n + b.checks, 0),
  }))
  const top = Math.max(1, ...groups.map((g) => g.ms))
  const goal = m.kind === "http" ? m.http?.url : m.tcp ? `${m.tcp.host}:${m.tcp.port}` : m.name
  const median = summary?.p50_ms === undefined ? undefined : formatMs(summary.p50_ms, i18n.locale)
  return (
    <section className="flex flex-col gap-2 p-4" data-testid="status-incident-monitor">
      <h2 className="text-[11px] font-medium tracking-[0.06em] text-faint uppercase">
        <Trans>Seen by the monitor</Trans>
      </h2>
      {groups.length > 0 && (
        <span className="flex h-15 items-end gap-0.5" title={t`Response time, last 24 hours`} aria-hidden>
          {groups.map((g, n) => (
            <span
              key={n}
              className={cn(
                "min-w-px flex-1 rounded-t-[2px]",
                g.checks === 0 ? "bg-muted" : g.failed ? "bg-warning" : "bg-success/70",
                n !== groups.length - 1 && "opacity-70"
              )}
              style={{ height: g.checks === 0 ? 2 : Math.max(3, Math.round((60 * g.ms) / top)) }}
            />
          ))}
        </span>
      )}
      <span className="text-caption break-all text-faint">
        {goal}
        {median && (
          <>
            {" · "}
            <Trans>median {median}</Trans>
          </>
        )}
      </span>
    </section>
  )
}

export function IncidentDetail({ pageId, i }: { pageId: string; i: StatusIncident }) {
  const { t, i18n } = useLingui()
  const navigate = useNavigate()
  const page = useStatusPage(pageId).data
  const inbox = (useInboxes().data ?? []).find((x) => x.id === page?.inbox_id)
  const components = useStatusComponents(pageId).data ?? []
  const text = useStatusText()
  const m = useIncidentMutations(pageId)
  const [confirm, confirmDialog] = useConfirm()
  const [params, setParams] = useSearchParams()
  const replyId = params.get("reply") ?? ""
  const [editing, setEditing] = useState(false)
  const [state, setState] = useState<StatusIncidentState | null>(null)
  const [body, setBody] = useState("")
  const box = useRef<HTMLTextAreaElement>(null)
  const [notifyChoice, setNotifyChoice] = useState<boolean | null>(null)
  const [publishNotify, setPublishNotify] = useState<boolean | null>(null)
  const subscribers = !!page?.email_subscribers.enabled
  const notices = useStatusNotices(pageId, i.id, subscribers).data ?? []
  const cancelNotice = useCancelNotice(pageId)
  const waiting = notices.find((n) => n.state === "pending")

  const closing: StatusIncidentState = i.kind === "incident" ? "resolved" : "completed"
  const open = !isClosed(i)
  const draft = !i.published_at

  const remove = () =>
    confirm({
      title: <Trans>Delete {i.title}?</Trans>,
      description: <Trans>Only a draft can be deleted. It was never shown on the page.</Trans>,
      confirm: <Trans>Delete draft</Trans>,
      run: () =>
        m.remove.mutate(i.id, {
          onSuccess: () => {
            toast(t`Draft deleted`)
            navigate(statusLink(pageId))
          },
        }),
    })
  const focusUpdate = (next?: StatusIncidentState) => {
    if (next) setState(next)
    box.current?.focus()
    box.current?.scrollIntoView({ block: "center" })
  }
  const notifyOnPublish = publishNotify ?? i.notify ?? true
  const publish = () => m.publish.mutate({ id: i.id, notify: subscribers ? notifyOnPublish : undefined })

  useHotkeys({
    ...(open ? { [STATUS_KEYS.update]: () => focusUpdate() } : {}),
    ...(open ? { [STATUS_KEYS.resolve]: () => focusUpdate(closing) } : {}),
    ...(open ? { [STATUS_KEYS.edit]: () => setEditing(true) } : {}),
    ...(draft ? { [STATUS_KEYS.publish]: publish } : {}),
    ...(waiting ? { [STATUS_KEYS.undoNotice]: () => cancelNotice.mutate(waiting.id) } : {}),
    ...(draft ? { [STATUS_KEYS.remove]: remove } : {}),
  })

  const touchedIds = i.components.map((c) => c.component_id)
  const current = state && nextStates(i).includes(state) ? state : i.state
  const notifyUpdate = notifyChoice ?? current !== i.state
  const touched = i.components.map((c) => ({ ...c, name: components.find((x) => x.id === c.component_id)?.name }))
  const updateKey = keyLabel(STATUS_KEYS.update).join("")
  const resolveKey = keyLabel(STATUS_KEYS.resolve).join("")
  const publishKey = keyLabel(STATUS_KEYS.publish).join("")
  const editKey = keyLabel(STATUS_KEYS.edit).join("")
  const removeKey = keyLabel(STATUS_KEYS.remove).join("")
  const now = useNow(60_000)
  const startedAt = i.started_at ?? i.published_at ?? i.created_at
  const started = new Intl.DateTimeFormat(i18n.locale, { timeStyle: "short" }).format(new Date(startedAt))
  const lasted = formatDuration(
    Math.max(60, Math.round(((i.resolved_at ? Date.parse(i.resolved_at) : now) - Date.parse(startedAt)) / 1000)),
    i18n.locale
  )
  const stage = text.incident[current]
  const tone = (c: StatusComponentStateValue) =>
    c === "operational" ? "success" : c === "major_outage" ? "danger" : c === "maintenance" ? "info" : "warning"
  return (
    <div className="@container flex min-h-full phone:flex-col" data-testid="status-incident" key={i.id}>
      <div className="flex min-w-0 flex-1 flex-col gap-4 p-6 pb-16 phone:px-4">
        <div className="flex flex-wrap items-center gap-2.5">
          {page && <ProductTag name={inbox?.name ?? page.name} color={inbox?.branding.color} />}
          <h2 className="min-w-0 flex-1 text-title break-words" data-testid="status-incident-title-heading">
            {i.title}
          </h2>
          <span className="text-caption text-faint">
            {i.kind === "maintenance" && i.scheduled_start && i.scheduled_end ? (
              `${formatDateTime(i.scheduled_start, i18n.locale)} – ${formatDateTime(i.scheduled_end, i18n.locale)}`
            ) : (
              <Trans>
                started {started} · {lasted}
              </Trans>
            )}
          </span>
          <DropdownMenu>
            <DropdownMenuTrigger
              render={<Button variant="ghost" size="icon-sm" aria-label={t`More for ${i.title}`} />}
              data-testid="status-incident-menu"
            >
              <MoreHorizontalIcon />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="min-w-52">
              {open && (
                <DropdownMenuItem
                  onClick={() => setEditing(true)}
                  title={t`Edit details (${editKey})`}
                  data-testid="status-incident-edit"
                >
                  <PencilIcon />
                  <Trans>Edit details</Trans>
                </DropdownMenuItem>
              )}
              {open && (
                <DropdownMenuItem
                  onClick={() => focusUpdate(closing)}
                  title={(closing === "resolved" ? t`Resolve` : t`Complete`) + ` (${resolveKey})`}
                  data-testid="status-incident-resolve"
                >
                  <CheckIcon />
                  {closing === "resolved" ? <Trans>Resolve</Trans> : <Trans>Complete</Trans>}
                </DropdownMenuItem>
              )}
              <DropdownMenuItem onClick={() => navigate(statusLink(pageId, "components"))}>
                <SquareStackIcon />
                <Trans>Components</Trans>
              </DropdownMenuItem>
              {draft && (
                <DropdownMenuItem
                  variant="destructive"
                  disabled={m.remove.isPending}
                  onClick={remove}
                  title={t`Delete draft (${removeKey})`}
                  data-testid="status-incident-delete"
                >
                  <Trash2Icon />
                  <Trans>Delete draft</Trans>
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
        {draft && (
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-xl border border-dashed px-4 py-3">
            <p className="min-w-60 flex-1 text-muted-foreground" data-testid="status-draft-note">
              <Trans>This is a draft. Only your team sees it, and it changes no component until you publish it.</Trans>
            </p>
            {page && (
              <NotifyCheck
                page={page}
                components={touchedIds}
                checked={notifyOnPublish}
                onChange={setPublishNotify}
                testId="status-incident-publish-notify"
              />
            )}
            <FrameButton
              ink
              disabled={m.publish.isPending}
              onClick={publish}
              title={t`Publish (${publishKey})`}
              data-testid="status-incident-publish-draft"
            >
              <SendIcon />
              <Trans>Publish</Trans>
            </FrameButton>
          </div>
        )}
        <ErrorLine error={m.publish.error ?? m.remove.error} />
        <Stepper i={i} value={current} onPick={open ? setState : undefined} />
        {open && (
          <form
            className="flex flex-col gap-3 rounded-xl border bg-card p-4"
            onSubmit={(e) => {
              e.preventDefault()
              if (!body.trim()) return
              m.postUpdate.mutate(
                { id: i.id, body: body.trim(), state: current, ...(subscribers ? { notify: notifyUpdate } : {}) },
                {
                  onSuccess: () => {
                    setBody("")
                    setState(null)
                    setNotifyChoice(null)
                  },
                }
              )
            }}
          >
            <h3 className="font-semibold">
              <Trans>New update · {stage}</Trans>
            </h3>
            <Textarea
              ref={box}
              value={body}
              maxLength={5000}
              rows={3}
              className="rounded-lg"
              onChange={(e) => setBody(e.target.value)}
              placeholder={t`What do customers need to know?`}
              aria-label={t`Update text`}
              data-testid="status-update-body"
            />
            {touched.length > 0 && (
              <p className="flex flex-wrap items-center gap-1.5">
                <span className="text-caption text-faint">
                  <Trans>Affects</Trans>
                </span>
                {touched.map((c) => (
                  <Pill key={c.component_id} tone={tone(c.state)} data-testid="status-incident-touched">
                    {c.name ?? <Trans>Deleted component</Trans>} · {text.component[c.state]}
                  </Pill>
                ))}
              </p>
            )}
            <div className="flex flex-wrap items-center gap-4 border-t pt-2.5">
              {page && (
                <NotifyCheck
                  page={page}
                  components={touchedIds}
                  checked={notifyUpdate}
                  onChange={setNotifyChoice}
                  testId="status-update-notify"
                />
              )}
              <ErrorLine error={m.postUpdate.error} className="min-w-0 flex-1 text-caption" />
              <Button
                type="submit"
                className="ms-auto h-[34px] rounded-lg bg-brand px-3.5 text-white hover:bg-brand/90 disabled:bg-brand disabled:text-white disabled:opacity-60 dark:text-black dark:disabled:text-black"
                disabled={!body.trim() || m.postUpdate.isPending}
                title={t`Post an update (${updateKey})`}
                data-testid="status-update-post"
              >
                <Trans>Publish update</Trans>
              </Button>
            </div>
          </form>
        )}
        {!open && touched.length > 0 && (
          <p className="flex flex-wrap items-center gap-1.5">
            <span className="text-caption text-faint">
              <Trans>Affected</Trans>
            </span>
            {touched.map((c) => (
              <Pill key={c.component_id} tone={tone(c.state)} data-testid="status-incident-touched">
                {c.name ?? <Trans>Deleted component</Trans>} · {text.component[c.state]}
              </Pill>
            ))}
          </p>
        )}
        <ul className="flex flex-col" data-testid="status-updates">
          {i.updates.map((u) => (
            <UpdateItem key={u.id} i={i} u={u} pageId={pageId} />
          ))}
        </ul>
      </div>
      <aside className="flex w-[300px] shrink-0 flex-col border-s phone:w-full phone:border-s-0 phone:border-t">
        <div className="border-b p-4">
          <LinkedConversations
            pageId={pageId}
            incident={i}
            replyId={replyId}
            onReply={(id) => setParams({ reply: id }, { replace: true })}
            onDismissReply={() => setParams({}, { replace: true })}
          />
        </div>
        <MonitorSeen pageId={pageId} incident={i} />
        {notices.length > 0 && (
          <div className="border-t p-4">
            <NoticesSection notices={notices} pageId={pageId} />
          </div>
        )}
      </aside>
      {editing && (
        <IncidentDialog
          pageId={pageId}
          kind={i.kind}
          edit={i}
          onClose={() => setEditing(false)}
          onDone={() => setEditing(false)}
        />
      )}
      {confirmDialog}
    </div>
  )
}

export function IncidentPane() {
  const { t } = useLingui()
  const { pageId = "", incidentId = "" } = useParams()
  const incident = useStatusIncident(pageId, incidentId)
  const { items } = useStatusScope()
  const all = useScopeIncidents(items)
  const i = incident.data
  const view = i ? viewOf(i) : "active"
  return (
    <IncidentsFrame
      view={view}
      pageId={pageId}
      counts={useIncidentCounts(all.data)}
      back={{ to: viewLink(pageId, view), label: t`Incidents` }}
    >
      {incident.isPending ? (
        <div className="flex flex-col gap-4 p-6">
          <Skeleton className="h-8 w-60" />
          <Skeleton className="h-24 w-full rounded-xl" />
        </div>
      ) : !i ? (
        <ErrorLine error={incident.error} className="m-6" />
      ) : (
        <IncidentDetail key={i.id} pageId={pageId} i={i} />
      )}
    </IncidentsFrame>
  )
}

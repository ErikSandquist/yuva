import { Trans, useLingui } from "@lingui/react/macro"
import { CheckIcon, PencilIcon, SendIcon, Trash2Icon } from "lucide-react"
import { useRef, useState } from "react"
import { useNavigate, useParams, useSearchParams } from "react-router"

import { ErrorLine, toast, useConfirm } from "@/components/common"
import { Pane, PaneBack } from "@/components/common/Column"
import { keyLabel, STATUS_KEYS } from "@/components/common/ShortcutSheet"
import { formatDateTime, formatRelative } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { Card, PageHeader, Section } from "@/features/settings/ui"
import { IncidentBadges } from "@/features/status/IncidentsTab"
import { IncidentDialog } from "@/features/status/IncidentDialog"
import { LinkedConversations } from "@/features/status/LinkedSection"
import { NotifyCheck } from "@/features/status/NotifyCheck"
import { NoticesSection } from "@/features/status/Notices"
import {
  statusLink,
  useCancelNotice,
  useIncidentMutations,
  useStatusComponents,
  useStatusIncident,
  useStatusNotices,
  useStatusPage,
} from "@/features/status/queries"
import {
  INCIDENT_STATES,
  isClosed,
  MAINTENANCE_STATES,
  nextStates,
  PlainText,
  StateDot,
  useStatusText,
} from "@/features/status/text"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { StatusIncident, StatusIncidentState, StatusIncidentUpdate } from "@/lib/api"
import { cn } from "@/lib/utils"
import { useMemberMap } from "@/lib/workspace"

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
      className="grid overflow-hidden rounded-2xl border bg-card @2xl:grid-flow-col @2xl:auto-cols-fr"
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
              here ? "bg-warning/10 text-foreground" : done ? "text-foreground" : "text-faint",
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
        <span className="mt-1.5 w-px flex-1 bg-faint/30 group-last/update:hidden" />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-1.5 pb-5">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-caption text-muted-foreground">
          <b className="text-body font-semibold text-foreground">{text.incident[u.state]}</b>
          <time dateTime={u.created_at} title={formatDateTime(u.created_at, i18n.locale)}>
            {formatRelative(u.created_at, i18n.locale)}
          </time>
          {author && <span>· {author}</span>}
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
          u.body && <PlainText text={u.body} className="text-muted-foreground" />
        )}
      </div>
    </li>
  )
}

export function IncidentPane() {
  const { t, i18n } = useLingui()
  const navigate = useNavigate()
  const { pageId = "", incidentId = "" } = useParams()
  const page = useStatusPage(pageId).data
  const incident = useStatusIncident(pageId, incidentId)
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
  const i = incident.data
  const subscribers = !!page?.email_subscribers.enabled
  const notices = useStatusNotices(pageId, incidentId, subscribers).data ?? []
  const cancelNotice = useCancelNotice(pageId)
  const waiting = notices.find((n) => n.state === "pending")

  const closing: StatusIncidentState | undefined = i ? (i.kind === "incident" ? "resolved" : "completed") : undefined
  const open = !!i && !isClosed(i)
  const draft = !!i && !i.published_at

  const remove = () =>
    i &&
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

  useHotkeys(
    {
      ...(open ? { [STATUS_KEYS.update]: () => focusUpdate() } : {}),
      ...(open && closing ? { [STATUS_KEYS.resolve]: () => focusUpdate(closing) } : {}),
      ...(open ? { [STATUS_KEYS.edit]: () => setEditing(true) } : {}),
      ...(draft
        ? {
            [STATUS_KEYS.publish]: () =>
              i &&
              m.publish.mutate({ id: i.id, notify: subscribers ? (publishNotify ?? i.notify ?? true) : undefined }),
          }
        : {}),
      ...(waiting ? { [STATUS_KEYS.undoNotice]: () => cancelNotice.mutate(waiting.id) } : {}),
      ...(draft ? { [STATUS_KEYS.remove]: remove } : {}),
    },
    !!i
  )

  const back = <PaneBack to={statusLink(pageId)} label={page?.name ?? t`Status`} always />
  if (incident.isPending) {
    return (
      <Pane wide className="gap-4">
        <Skeleton className="h-8 w-60" />
        <Skeleton className="h-24 w-full rounded-2xl" />
      </Pane>
    )
  }
  if (!i) {
    return (
      <Pane wide>
        {back}
        <ErrorLine error={incident.error} />
      </Pane>
    )
  }
  const notifyOnPublish = publishNotify ?? i.notify ?? true
  const touchedIds = i.components.map((c) => c.component_id)
  const current = state && nextStates(i).includes(state) ? state : i.state
  const notifyUpdate = notifyChoice ?? current !== i.state
  const touched = i.components.map((c) => ({ ...c, name: components.find((x) => x.id === c.component_id)?.name }))
  const updateKey = keyLabel(STATUS_KEYS.update).join("")
  const resolveKey = keyLabel(STATUS_KEYS.resolve).join("")
  const publishKey = keyLabel(STATUS_KEYS.publish).join("")
  const editKey = keyLabel(STATUS_KEYS.edit).join("")
  const removeKey = keyLabel(STATUS_KEYS.remove).join("")

  const started = formatDateTime(i.started_at ?? i.published_at ?? i.created_at, i18n.locale)
  const stage = text.incident[current]
  return (
    <Pane wide testId="status-incident" key={i.id} className="@container gap-6">
      {back}
      <PageHeader
        back={false}
        title={<span data-testid="status-incident-title-heading">{i.title}</span>}
        description={
          <span className="flex flex-col gap-1.5">
            <IncidentBadges i={i} />
            <span className="text-small text-faint">
              {i.kind === "maintenance" && i.scheduled_start && i.scheduled_end ? (
                `${formatDateTime(i.scheduled_start, i18n.locale)} – ${formatDateTime(i.scheduled_end, i18n.locale)}`
              ) : (
                <Trans>Started {started}</Trans>
              )}
            </span>
          </span>
        }
        action={
          <>
            {draft && (
              <Button
                variant="outline"
                size="sm"
                disabled={m.remove.isPending}
                onClick={remove}
                title={t`Delete draft (${removeKey})`}
                data-testid="status-incident-delete"
              >
                <Trash2Icon />
                <Trans>Delete draft</Trans>
              </Button>
            )}
            {open && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setEditing(true)}
                title={t`Edit details (${editKey})`}
                data-testid="status-incident-edit"
              >
                <PencilIcon />
                <Trans>Edit details</Trans>
              </Button>
            )}
            {open && closing && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => focusUpdate(closing)}
                title={(closing === "resolved" ? t`Resolve` : t`Complete`) + ` (${resolveKey})`}
                data-testid="status-incident-resolve"
              >
                <CheckIcon />
                {closing === "resolved" ? <Trans>Resolve</Trans> : <Trans>Complete</Trans>}
              </Button>
            )}
            {draft && (
              <Button
                size="sm"
                disabled={m.publish.isPending}
                onClick={() => m.publish.mutate({ id: i.id, notify: subscribers ? notifyOnPublish : undefined })}
                title={t`Publish (${publishKey})`}
                data-testid="status-incident-publish-draft"
              >
                <SendIcon />
                <Trans>Publish</Trans>
              </Button>
            )}
          </>
        }
      >
        {draft && (
          <div className="mt-4 flex flex-wrap items-center gap-x-4 gap-y-2 rounded-xl border border-dashed px-4 py-3">
            <p className="min-w-60 flex-1 text-body text-muted-foreground" data-testid="status-draft-note">
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
          </div>
        )}
        <ErrorLine error={m.publish.error ?? m.remove.error} className="mt-2" />
      </PageHeader>
      <Stepper i={i} value={current} onPick={open ? setState : undefined} />
      <div className="grid grid-cols-[minmax(0,1fr)] items-start gap-6 @4xl:grid-cols-[minmax(0,1fr)_300px]">
        <div className="flex min-w-0 flex-col gap-6">
          {open && (
            <form
              className="flex flex-col gap-3 rounded-2xl border bg-card p-4"
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
              <h2 className="text-body font-semibold">
                <Trans>New update · {stage}</Trans>
              </h2>
              <Textarea
                ref={box}
                value={body}
                maxLength={5000}
                rows={3}
                onChange={(e) => setBody(e.target.value)}
                placeholder={t`What do customers need to know?`}
                aria-label={t`Update text`}
                data-testid="status-update-body"
              />
              {touched.length > 0 && (
                <p className="flex flex-wrap items-center gap-1.5 text-small">
                  <span className="text-faint">
                    <Trans>Affects</Trans>
                  </span>
                  {touched.map((c) => (
                    <span
                      key={c.component_id}
                      className="inline-flex items-center gap-1.5 rounded-full border px-2 py-px text-caption"
                      data-testid="status-incident-touched"
                    >
                      <StateDot state={c.state} className="size-1.5" />
                      {c.name ?? <Trans>Deleted component</Trans>} · {text.component[c.state]}
                    </span>
                  ))}
                </p>
              )}
              <div className="flex flex-wrap items-center gap-3 border-t pt-3">
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
                  size="sm"
                  className="ms-auto"
                  disabled={!body.trim() || m.postUpdate.isPending}
                  title={t`Post an update (${updateKey})`}
                  data-testid="status-update-post"
                >
                  <Trans>Post update</Trans>
                </Button>
              </div>
            </form>
          )}
          {!open && touched.length > 0 && (
            <Section title={<Trans>Components</Trans>}>
              <Card flush>
                <ul className="divide-y">
                  {touched.map((c) => (
                    <li
                      key={c.component_id}
                      className="flex items-center gap-3 px-4 py-2.5"
                      data-testid="status-incident-touched"
                    >
                      <span className="min-w-0 flex-1 truncate text-body">
                        {c.name ?? <Trans>Deleted component</Trans>}
                      </span>
                      <span className="inline-flex shrink-0 items-center gap-2 text-small text-muted-foreground">
                        <StateDot state={c.state} />
                        {text.component[c.state]}
                      </span>
                    </li>
                  ))}
                </ul>
              </Card>
            </Section>
          )}
          <Section title={<Trans>Updates</Trans>}>
            <ul className="px-1 pt-2" data-testid="status-updates">
              {i.updates.map((u) => (
                <UpdateItem key={u.id} i={i} u={u} pageId={pageId} />
              ))}
            </ul>
          </Section>
        </div>
        <div className="flex min-w-0 flex-col gap-6">
          <LinkedConversations
            pageId={pageId}
            incident={i}
            replyId={replyId}
            onReply={(id) => setParams({ reply: id }, { replace: true })}
            onDismissReply={() => setParams({}, { replace: true })}
          />
          <NoticesSection notices={notices} pageId={pageId} />
        </div>
      </div>
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
    </Pane>
  )
}

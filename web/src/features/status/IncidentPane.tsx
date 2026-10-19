import { Trans, useLingui } from "@lingui/react/macro"
import { CheckIcon, PencilIcon, SendIcon, Trash2Icon } from "lucide-react"
import { useRef, useState } from "react"
import { useNavigate, useParams } from "react-router"

import { ErrorLine, toast, useConfirm } from "@/components/common"
import { Pane, PaneBack } from "@/components/common/Column"
import { keyLabel, STATUS_KEYS } from "@/components/common/ShortcutSheet"
import { formatDateTime, formatRelative } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { Card, ChoiceSelect, Section } from "@/features/settings/ui"
import { IncidentBadges } from "@/features/status/IncidentsTab"
import { IncidentDialog } from "@/features/status/IncidentDialog"
import {
  statusLink,
  useIncidentMutations,
  useStatusComponents,
  useStatusIncident,
  useStatusPage,
} from "@/features/status/queries"
import { isClosed, nextStates, PlainText, StateDot, useStatusText } from "@/features/status/text"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { StatusIncident, StatusIncidentState, StatusIncidentUpdate } from "@/lib/api"
import { useMemberMap } from "@/lib/workspace"

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
    <li className="flex flex-col gap-1.5 border-b px-4 py-3.5 last:border-b-0" data-testid="status-update">
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
        u.body && <PlainText text={u.body} />
      )}
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
  const [editing, setEditing] = useState(false)
  const [state, setState] = useState<StatusIncidentState | null>(null)
  const [body, setBody] = useState("")
  const box = useRef<HTMLTextAreaElement>(null)
  const i = incident.data

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
      ...(draft ? { [STATUS_KEYS.publish]: () => i && m.publish.mutate(i.id) } : {}),
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
  const states = nextStates(i).map((s) => [s, text.incident[s]] as const)
  const current = state && nextStates(i).includes(state) ? state : i.state
  const touched = i.components.map((c) => ({ ...c, name: components.find((x) => x.id === c.component_id)?.name }))
  const updateKey = keyLabel(STATUS_KEYS.update).join("")
  const resolveKey = keyLabel(STATUS_KEYS.resolve).join("")
  const publishKey = keyLabel(STATUS_KEYS.publish).join("")
  const editKey = keyLabel(STATUS_KEYS.edit).join("")
  const removeKey = keyLabel(STATUS_KEYS.remove).join("")

  return (
    <Pane testId="status-incident" key={i.id}>
      <header className="flex flex-col gap-2">
        {back}
        <h1 className="min-w-0 text-page break-words" data-testid="status-incident-title-heading">
          {i.title}
        </h1>
        <IncidentBadges i={i} />
        {i.kind === "maintenance" && i.scheduled_start && i.scheduled_end && (
          <p className="text-body text-muted-foreground">
            {formatDateTime(i.scheduled_start, i18n.locale)} – {formatDateTime(i.scheduled_end, i18n.locale)}
          </p>
        )}
        {draft && (
          <p className="text-body text-muted-foreground" data-testid="status-draft-note">
            <Trans>This is a draft. Only your team sees it, and it changes no component until you publish it.</Trans>
          </p>
        )}
        <div className="mt-1 flex flex-wrap gap-2">
          {draft && (
            <Button
              size="sm"
              disabled={m.publish.isPending}
              onClick={() => m.publish.mutate(i.id)}
              title={t`Publish (${publishKey})`}
              data-testid="status-incident-publish-draft"
            >
              <SendIcon />
              <Trans>Publish</Trans>
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
        </div>
        <ErrorLine error={m.publish.error ?? m.remove.error} />
      </header>
      {touched.length > 0 && (
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
      {open && (
        <Section title={<Trans>Post an update</Trans>}>
          <form
            className="flex flex-col gap-3 rounded-2xl border bg-card p-4"
            onSubmit={(e) => {
              e.preventDefault()
              if (!body.trim()) return
              m.postUpdate.mutate(
                { id: i.id, body: body.trim(), state: current },
                {
                  onSuccess: () => {
                    setBody("")
                    setState(null)
                  },
                }
              )
            }}
          >
            <ChoiceSelect value={current} onChange={setState} options={states} label={t`State`} className="w-56" />
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
            <div className="flex flex-wrap items-center gap-3">
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
        </Section>
      )}
      <Section title={<Trans>Updates</Trans>}>
        <Card flush>
          <ul data-testid="status-updates">
            {i.updates.map((u) => (
              <UpdateItem key={u.id} i={i} u={u} pageId={pageId} />
            ))}
          </ul>
        </Card>
      </Section>
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

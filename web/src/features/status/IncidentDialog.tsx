import { Trans, useLingui } from "@lingui/react/macro"
import { LoaderIcon } from "lucide-react"
import { useState } from "react"

import { ErrorLine } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { ChoiceSelect, Field } from "@/features/settings/ui"
import { NotifyCheck } from "@/features/status/NotifyCheck"
import { useIncidentMutations, useStatusComponents, useStatusPage } from "@/features/status/queries"
import { StateSelect } from "@/features/status/StateSelect"
import { IMPACTS, INCIDENT_STATES, useStatusText } from "@/features/status/text"
import type {
  StatusComponentStateValue,
  StatusIncident,
  StatusIncidentImpact,
  StatusIncidentKind,
  StatusIncidentState,
} from "@/lib/api"

function toLocalInput(date: Date) {
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function defaultWindow() {
  const start = new Date()
  start.setMinutes(0, 0, 0)
  start.setHours(start.getHours() + 1)
  return [toLocalInput(start), toLocalInput(new Date(start.getTime() + 3600_000))] as const
}

export function IncidentDialog({
  pageId,
  kind,
  edit,
  onClose,
  onDone,
}: {
  pageId: string
  kind: StatusIncidentKind
  edit?: StatusIncident
  onClose: () => void
  onDone: (i: StatusIncident) => void
}) {
  const { t } = useLingui()
  const text = useStatusText()
  const m = useIncidentMutations(pageId)
  const components = useStatusComponents(pageId).data ?? []
  const page = useStatusPage(pageId).data
  const [notify, setNotify] = useState(true)
  const maintenance = kind === "maintenance"
  const [title, setTitle] = useState(edit?.title ?? "")
  const [impact, setImpact] = useState<StatusIncidentImpact>(edit?.impact ?? (maintenance ? "none" : "minor"))
  const [state, setState] = useState<StatusIncidentState>("investigating")
  const [message, setMessage] = useState("")
  const [defaults] = useState(defaultWindow)
  const [start, setStart] = useState(edit?.scheduled_start ? toLocalInput(new Date(edit.scheduled_start)) : defaults[0])
  const [end, setEnd] = useState(edit?.scheduled_end ? toLocalInput(new Date(edit.scheduled_end)) : defaults[1])
  const fallback: StatusComponentStateValue = maintenance ? "maintenance" : "major_outage"
  const [touched, setTouched] = useState<Record<string, StatusComponentStateValue>>(
    Object.fromEntries((edit?.components ?? []).map((c) => [c.component_id, c.state]))
  )
  const windowChanged =
    !!edit &&
    maintenance &&
    (new Date(start).getTime() !== Date.parse(edit.scheduled_start ?? "") ||
      new Date(end).getTime() !== Date.parse(edit.scheduled_end ?? ""))
  const askNotify = !!page && (!edit || windowChanged)
  const pending = m.create.isPending || m.patch.isPending
  const windowOk = !maintenance || (!!start && !!end && new Date(end) > new Date(start))
  const ok = !!title.trim() && (!!edit || !!message.trim()) && windowOk

  const body = () => ({
    title: title.trim(),
    impact,
    components: Object.entries(touched).map(([component_id, s]) => ({ component_id, state: s })),
    ...(maintenance
      ? { scheduled_start: new Date(start).toISOString(), scheduled_end: new Date(end).toISOString() }
      : {}),
  })
  const save = (publish: boolean) => {
    if (!ok) return
    if (edit)
      m.patch.mutate(
        { id: edit.id, ...body(), ...(windowChanged && page?.email_subscribers.enabled ? { notify } : {}) },
        { onSuccess: onDone }
      )
    else
      m.create.mutate(
        {
          kind,
          ...body(),
          message: message.trim(),
          publish,
          ...(maintenance ? {} : { state }),
          ...(page?.email_subscribers.enabled ? { notify } : {}),
        },
        { onSuccess: onDone }
      )
  }
  const states = INCIDENT_STATES.filter((s) => s !== "resolved").map((s) => [s, text.incident[s]] as const)
  const impacts = IMPACTS.map((s) => [s, text.impact[s]] as const)
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-xl" data-testid="status-incident-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            save(false)
          }}
        >
          <DialogHeader>
            <DialogTitle>
              {edit ? (
                <Trans>Edit details</Trans>
              ) : maintenance ? (
                <Trans>Plan maintenance</Trans>
              ) : (
                <Trans>Open an incident</Trans>
              )}
            </DialogTitle>
            {!edit && (
              <DialogDescription>
                <Trans>A draft is seen only by your team and changes no component until you publish it.</Trans>
              </DialogDescription>
            )}
          </DialogHeader>
          <Field label={<Trans>Title</Trans>} htmlFor="status-incident-title">
            <Input
              id="status-incident-title"
              value={title}
              maxLength={200}
              autoFocus
              required
              placeholder={maintenance ? t`Database upgrade` : t`Elevated API errors`}
              onChange={(e) => setTitle(e.target.value)}
              data-testid="status-incident-title"
            />
          </Field>
          <div className="grid gap-5 sm:grid-cols-2">
            <Field label={<Trans>Impact</Trans>} htmlFor="status-incident-impact">
              <ChoiceSelect
                id="status-incident-impact"
                value={impact}
                onChange={setImpact}
                options={impacts}
                label={t`Impact`}
              />
            </Field>
            {!edit && !maintenance && (
              <Field label={<Trans>State</Trans>} htmlFor="status-incident-state">
                <ChoiceSelect
                  id="status-incident-state"
                  value={state}
                  onChange={setState}
                  options={states}
                  label={t`State`}
                />
              </Field>
            )}
          </div>
          {maintenance && (
            <div className="grid gap-5 sm:grid-cols-2">
              <Field label={<Trans>Starts</Trans>} htmlFor="status-incident-start">
                <Input
                  id="status-incident-start"
                  type="datetime-local"
                  value={start}
                  required
                  onChange={(e) => setStart(e.target.value)}
                  data-testid="status-incident-start"
                />
              </Field>
              <Field
                label={<Trans>Ends</Trans>}
                htmlFor="status-incident-end"
                error={!windowOk && start && end ? <Trans>The end must be after the start.</Trans> : undefined}
              >
                <Input
                  id="status-incident-end"
                  type="datetime-local"
                  value={end}
                  required
                  onChange={(e) => setEnd(e.target.value)}
                  data-testid="status-incident-end"
                />
              </Field>
            </div>
          )}
          {components.length > 0 && (
            <fieldset className="flex flex-col gap-2">
              <legend className="mb-2 text-body font-medium">
                <Trans>Components it touches</Trans>
              </legend>
              <ul className="flex flex-col divide-y rounded-xl border">
                {components.map((c) => {
                  const on = c.id in touched
                  return (
                    <li key={c.id} className="flex flex-wrap items-center gap-x-3 gap-y-2 px-3 py-2">
                      <Label className="min-w-0 flex-1 basis-40 font-normal">
                        <Checkbox
                          checked={on}
                          onCheckedChange={(v) =>
                            setTouched((cur) => {
                              const next = { ...cur }
                              if (v) next[c.id] = fallback
                              else delete next[c.id]
                              return next
                            })
                          }
                          data-testid="status-incident-component"
                        />
                        <span className="truncate">{c.name}</span>
                      </Label>
                      {on && (
                        <StateSelect
                          value={touched[c.id]}
                          label={t`State of ${c.name} while it is open`}
                          onChange={(s) => setTouched((cur) => ({ ...cur, [c.id]: s }))}
                          className="w-48 phone:w-full"
                        />
                      )}
                    </li>
                  )
                })}
              </ul>
              <p className="text-small text-faint">
                {maintenance ? (
                  <Trans>Each component gets this state while the maintenance is open.</Trans>
                ) : (
                  <Trans>Each component gets this state while the incident is open.</Trans>
                )}
              </p>
            </fieldset>
          )}
          {!edit && (
            <Field
              label={<Trans>First update</Trans>}
              htmlFor="status-incident-message"
              hint={<Trans>Plain text with links and line breaks, at most 5,000 characters.</Trans>}
            >
              <Textarea
                id="status-incident-message"
                value={message}
                maxLength={5000}
                required
                rows={4}
                onChange={(e) => setMessage(e.target.value)}
                data-testid="status-incident-message"
              />
            </Field>
          )}
          {askNotify && (
            <NotifyCheck
              page={page}
              components={Object.keys(touched)}
              checked={notify}
              onChange={setNotify}
              testId="status-incident-notify"
            />
          )}
          <ErrorLine error={m.create.error ?? m.patch.error} />
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              <Trans>Cancel</Trans>
            </Button>
            {edit ? (
              <Button type="submit" disabled={!ok || pending} data-testid="status-incident-save">
                {pending && <LoaderIcon className="animate-spin" />}
                <Trans>Save</Trans>
              </Button>
            ) : (
              <>
                <Button type="submit" variant="outline" disabled={!ok || pending} data-testid="status-incident-draft">
                  <Trans>Save as draft</Trans>
                </Button>
                <Button
                  type="button"
                  disabled={!ok || pending}
                  onClick={() => save(true)}
                  data-testid="status-incident-publish"
                >
                  {pending && <LoaderIcon className="animate-spin" />}
                  <Trans>Publish now</Trans>
                </Button>
              </>
            )}
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

import { Trans, useLingui } from "@lingui/react/macro"
import { BellIcon, KeyRoundIcon, PencilIcon, StickyNoteIcon, TargetIcon, Trash2Icon } from "lucide-react"
import { useState } from "react"
import { ReferenceArea, ReferenceLine } from "recharts"

import { ErrorLine, useConfirm } from "@/components/common"
import { formatDate } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { useAlertText } from "@/features/analytics/Alerts"
import { useAlerts, useDeleteAnnotation, useSaveAnnotation } from "@/features/analytics/queries"
import { todayIso } from "@/features/analytics/state"
import { Field } from "@/features/settings/ui"
import type { AnalyticsAnnotation, AnalyticsDimension } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"

function bucketOf(buckets: string[], grain: AnalyticsDimension, day: string) {
  const key = grain === "time:hour" ? `${day}T00:00` : day
  let hit: string | undefined
  for (const b of buckets) {
    if (b > key) break
    hit = b
  }
  return hit
}

export function annotationMarks(items: AnalyticsAnnotation[], buckets: string[], grain: AnalyticsDimension) {
  if (buckets.length === 0) return []
  const last = buckets[buckets.length - 1]
  const lastDay = last.slice(0, 10)
  return items.flatMap((a) => {
    if (a.starts_on > lastDay) return []
    const x1 = bucketOf(buckets, grain, a.starts_on) ?? buckets[0]
    const x2 = bucketOf(buckets, grain, a.ends_on) ?? buckets[0]
    const color = a.kind === "note" ? "var(--brand)" : a.kind === "alert" ? "var(--warning)" : "var(--muted-foreground)"
    if (x1 === x2) {
      return [
        <ReferenceLine
          key={a.id}
          x={x1}
          stroke={color}
          strokeDasharray="3 3"
          strokeOpacity={0.7}
          ifOverflow="extendDomain"
        />,
      ]
    }
    return [<ReferenceArea key={a.id} x1={x1} x2={x2} fill={color} fillOpacity={0.06} ifOverflow="extendDomain" />]
  })
}

export function useAnnotationText(propertyId: string, withAlerts: boolean) {
  const { t } = useLingui()
  const alertText = useAlertText()
  const alerts = useAlerts(propertyId, withAlerts)
  const byId = new Map((alerts.data?.pages.flatMap((p) => p.items) ?? []).map((x) => [x.id, x]))
  return (a: AnalyticsAnnotation) => {
    switch (a.kind) {
      case "key_rotated":
        return t`A new key was made`
      case "goal_created":
        return t`Goal “${a.text}” was created`
      case "alert": {
        const alert = a.alert_id ? byId.get(a.alert_id) : undefined
        if (!alert) return a.text
        const kind = alertText.kind[alert.kind]
        const found = alertText.sentence(alert)
        return t`${kind}: ${found}`
      }
      default:
        return a.text
    }
  }
}

function dayRange(a: { starts_on: string; ends_on: string }, locale: string) {
  const from = formatDate(`${a.starts_on}T12:00:00Z`, locale)
  return a.ends_on === a.starts_on ? from : `${from} – ${formatDate(`${a.ends_on}T12:00:00Z`, locale)}`
}

function AnnotationDialog({
  propertyId,
  annotation,
  onClose,
}: {
  propertyId: string
  annotation?: AnalyticsAnnotation
  onClose: () => void
}) {
  const { t } = useLingui()
  const save = useSaveAnnotation(propertyId, annotation?.id)
  const [from, setFrom] = useState(annotation?.starts_on ?? todayIso())
  const [to, setTo] = useState(annotation?.ends_on ?? annotation?.starts_on ?? todayIso())
  const [text, setText] = useState(annotation?.text ?? "")
  const valid = !!from && !!text.trim() && (!to || to >= from)
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent data-testid="annotation-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            if (!valid) return
            save.mutate({ starts_on: from, ends_on: to || from, text: text.trim() }, { onSuccess: onClose })
          }}
        >
          <DialogHeader>
            <DialogTitle>{annotation ? <Trans>Edit note</Trans> : <Trans>Add a note to the chart</Trans>}</DialogTitle>
          </DialogHeader>
          <Field
            label={<Trans>Note</Trans>}
            htmlFor="annotation-text"
            hint={<Trans>Everyone who sees this property sees the note, such as a launch or a campaign.</Trans>}
          >
            <Input
              id="annotation-text"
              required
              maxLength={300}
              value={text}
              placeholder={t`New pricing page`}
              onChange={(e) => setText(e.target.value)}
              data-testid="annotation-text"
            />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label={<Trans>From</Trans>} htmlFor="annotation-from">
              <Input
                id="annotation-from"
                type="date"
                required
                value={from}
                onChange={(e) => {
                  setFrom(e.target.value)
                  if (to < e.target.value) setTo(e.target.value)
                }}
              />
            </Field>
            <Field label={<Trans>To</Trans>} htmlFor="annotation-to">
              <Input id="annotation-to" type="date" value={to} min={from} onChange={(e) => setTo(e.target.value)} />
            </Field>
          </div>
          <ErrorLine error={save.error} />
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              <Trans>Cancel</Trans>
            </Button>
            <Button type="submit" disabled={!valid || save.isPending} data-testid="annotation-save">
              <Trans>Save</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export function AddNoteButton({ propertyId }: { propertyId: string }) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button variant="ghost" size="sm" onClick={() => setOpen(true)} data-testid="annotation-add">
        <StickyNoteIcon />
        <Trans>Add note</Trans>
      </Button>
      {open && <AnnotationDialog propertyId={propertyId} onClose={() => setOpen(false)} />}
    </>
  )
}

export function AnnotationList({ propertyId, items }: { propertyId: string; items: AnalyticsAnnotation[] }) {
  const { t, i18n } = useLingui()
  const { canManage, membership } = useSession()
  const label = useAnnotationText(
    propertyId,
    items.some((a) => a.kind === "alert")
  )
  const remove = useDeleteAnnotation(propertyId)
  const [ask, dialog] = useConfirm()
  const [editing, setEditing] = useState<AnalyticsAnnotation | null>(null)
  if (items.length === 0) return null
  return (
    <>
      <ul className="mt-2 flex flex-col gap-0.5 border-t pt-2" aria-label={t`Notes`} data-testid="annotations">
        {items.map((a) => {
          const Icon =
            a.kind === "key_rotated"
              ? KeyRoundIcon
              : a.kind === "goal_created"
                ? TargetIcon
                : a.kind === "alert"
                  ? BellIcon
                  : StickyNoteIcon
          const mine = a.kind === "note" && (canManage || a.created_by === membership.member_id)
          return (
            <li
              key={a.id}
              className="group flex min-h-8 items-center gap-2 rounded-md px-2 text-small"
              data-testid={`annotation-${a.kind}`}
            >
              <Icon
                className={cn(
                  "size-3.5 shrink-0",
                  a.kind === "note" ? "text-brand" : a.kind === "alert" ? "text-warning" : "text-faint"
                )}
              />
              <span className="shrink-0 text-faint tabular-nums">{dayRange(a, i18n.locale)}</span>
              <span className="min-w-0 flex-1 truncate" dir="auto" title={label(a)}>
                {label(a)}
              </span>
              {mine && (
                <span className="flex shrink-0 items-center">
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={t`Edit note`}
                    title={t`Edit note`}
                    onClick={() => setEditing(a)}
                  >
                    <PencilIcon />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={t`Delete note`}
                    title={t`Delete note`}
                    onClick={() =>
                      ask({
                        title: <Trans>Delete this note?</Trans>,
                        description: label(a),
                        confirm: <Trans>Delete note</Trans>,
                        run: () => remove.mutate(a.id),
                      })
                    }
                  >
                    <Trash2Icon />
                  </Button>
                </span>
              )}
            </li>
          )
        })}
      </ul>
      <ErrorLine error={remove.error} className="px-2" />
      {editing && <AnnotationDialog propertyId={propertyId} annotation={editing} onClose={() => setEditing(null)} />}
      {dialog}
    </>
  )
}

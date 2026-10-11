import { Trans, useLingui } from "@lingui/react/macro"
import { LayoutTemplateIcon, PencilIcon, PlusIcon } from "lucide-react"
import { Link } from "react-router"

import { MailFrame } from "@/components/common/MailFrame"
import { BROADCAST_KEYS, keyLabel } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { designLink, useDesign, useDesigns, useRender } from "@/features/designs/queries"
import { Field } from "@/features/settings/ui"

const NONE = "none"
const WIDTH = 600
const SCALE = 0.3

export function DesignThumb({ designId, inboxId }: { designId: string; inboxId: string }) {
  const { t } = useLingui()
  const design = useDesign(designId)
  const render = useRender(design.data ? { document: design.data.document, inbox_id: inboxId } : null)
  return (
    <div
      className="relative h-60 w-[180px] shrink-0 overflow-hidden rounded-xl border bg-white"
      data-testid="design-thumb"
    >
      {render.data ? (
        <div
          className="pointer-events-none absolute top-0 start-0 origin-top-left rtl:origin-top-right"
          style={{ width: WIDTH, transform: `scale(${SCALE})` }}
          aria-hidden
        >
          <MailFrame html={render.data.html} width={WIDTH} title={t`Preview of the design`} />
        </div>
      ) : render.isError || design.isError ? (
        <div className="grid h-full place-items-center text-faint">
          <LayoutTemplateIcon className="size-6" />
        </div>
      ) : (
        <Skeleton className="h-full w-full rounded-none" />
      )}
      <span className="absolute inset-x-0 bottom-0 h-10 bg-gradient-to-t from-black/10 to-transparent" />
    </div>
  )
}

export function DesignPicker({
  inboxId,
  value,
  onChange,
  disabled,
}: {
  inboxId: string
  value: string
  onChange: (id: string | null) => void
  disabled: boolean
}) {
  const { t } = useLingui()
  const designs = useDesigns("", "", false)
  const options = (designs.data?.pages.flatMap((p) => p.items) ?? []).filter(
    (d) => !d.inbox_id || d.inbox_id === inboxId
  )
  const known = options.some((d) => d.id === value)
  const items: Record<string, string> = { [NONE]: t`No design yet` }
  for (const d of options) items[d.id] = d.name
  if (value && !known) items[value] = t`A design of another inbox or archived`
  const edit = keyLabel(BROADCAST_KEYS.design).join("")
  return (
    <div className="flex flex-wrap items-start gap-4 phone:flex-col" data-testid="design-picker">
      {value ? (
        <DesignThumb designId={value} inboxId={inboxId} />
      ) : (
        <div className="grid h-60 w-[180px] shrink-0 place-items-center rounded-xl border-2 border-dashed text-faint">
          <LayoutTemplateIcon className="size-7" />
        </div>
      )}
      <div className="flex min-w-48 flex-1 flex-col gap-3">
        <Field label={<Trans>Design</Trans>}>
          <Select
            value={value || NONE}
            items={items}
            disabled={disabled}
            onValueChange={(v) => onChange(!v || v === NONE ? null : String(v))}
          >
            <SelectTrigger className="w-full min-w-0" aria-label={t`Design`} data-testid="broadcast-design">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NONE}>{t`No design yet`}</SelectItem>
              {options.map((d) => (
                <SelectItem key={d.id} value={d.id}>
                  {d.name}
                </SelectItem>
              ))}
              {value && !known && <SelectItem value={value}>{items[value]}</SelectItem>}
            </SelectContent>
          </Select>
        </Field>
        <div className="flex flex-wrap gap-2">
          {value && (
            <Button
              variant="outline"
              size="sm"
              render={<Link to={designLink(value)} />}
              title={t`Edit the design (${edit})`}
              data-testid="edit-design"
            >
              <PencilIcon />
              <Trans>Edit design</Trans>
            </Button>
          )}
          {!disabled && (
            <Button variant="ghost" size="sm" render={<Link to="/broadcasts/designs/new" />}>
              <PlusIcon />
              <Trans>New design</Trans>
            </Button>
          )}
        </div>
        <p className="text-small text-faint">
          <Trans>
            The design is frozen when you schedule the broadcast. Changing the design later does not change mail that is
            already scheduled or sent.
          </Trans>
        </p>
      </div>
    </div>
  )
}

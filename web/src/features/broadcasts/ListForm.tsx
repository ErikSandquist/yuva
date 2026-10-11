import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation } from "@tanstack/react-query"
import { MailWarningIcon, TriangleAlertIcon } from "lucide-react"
import { useState } from "react"
import { Link } from "react-router"

import { Dot, ErrorLine, Notice } from "@/components/common"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { useInboxChannels, useSetList } from "@/features/broadcasts/queries"
import { Field, ToggleRow } from "@/features/settings/ui"
import { api, ApiError, unwrap, type Channel, type MailingList, type MailingListCreate } from "@/lib/api"
import { useInboxes } from "@/lib/workspace"

export type ListValues = Required<Omit<MailingListCreate, "channel_id">> & { channel_id: string }

export function listValues(l: MailingList): ListValues {
  return {
    inbox_id: l.inbox_id,
    name: l.name,
    description: l.description,
    double_opt_in: l.double_opt_in,
    public: l.public,
    commercial: l.commercial,
    welcome: l.welcome,
    channel_id: l.channel_id ?? "",
  }
}

const NONE = "none"

export function canSend(ch: Channel) {
  return ch.kind === "email" && !!ch.email?.smtp
}

export function SenderMissing({ inboxId, className }: { inboxId: string; className?: string }) {
  return (
    <Notice tone="warning" icon={MailWarningIcon} className={className} data-testid="list-needs-sender">
      <p>
        <Trans>
          This list has no e-mail channel that can send. Until it has one, forms send no confirmation mail, new form
          subscribers stay waiting for confirmation and nobody gets a welcome mail. Pick an e-mail channel with an SMTP
          account below, or add the SMTP account in the inbox settings.
        </Trans>
      </p>
      <Link
        to={`/settings/inboxes/${inboxId}`}
        className="w-fit text-brand underline-offset-4 hover:underline"
        data-testid="list-inbox-settings"
      >
        <Trans>Open the inbox settings</Trans>
      </Link>
    </Notice>
  )
}

function SenderPicker({
  inboxId,
  value,
  onChange,
  invalid,
}: {
  inboxId: string
  value: string
  onChange: (id: string) => void
  invalid: boolean
}) {
  const { t } = useLingui()
  const channels = (useInboxChannels(inboxId).data ?? []).filter((c) => c.kind === "email")
  const items: Record<string, string> = { [NONE]: t`None` }
  for (const c of channels) items[c.id] = c.email ? `${c.name} · ${c.email.address}` : c.name
  return (
    <Field
      label={<Trans>Sends its mail from</Trans>}
      hint={
        channels.length === 0 ? (
          <Trans>This inbox has no e-mail channel yet. Add one in the inbox settings.</Trans>
        ) : (
          <Trans>The e-mail channel that sends the confirmation and welcome mails. It needs an SMTP account.</Trans>
        )
      }
    >
      <Select value={value || NONE} items={items} onValueChange={(v) => onChange(!v || v === NONE ? "" : String(v))}>
        <SelectTrigger
          className="w-full min-w-0"
          aria-label={t`Sends its mail from`}
          aria-invalid={invalid || undefined}
          data-testid="list-channel"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={NONE}>{t`None`}</SelectItem>
          {channels.map((c) => (
            <SelectItem key={c.id} value={c.id} disabled={!canSend(c)}>
              <span className="flex min-w-0 flex-col">
                <span className="truncate">{c.name}</span>
                <small className="truncate text-caption text-faint">
                  {c.email?.address}
                  {!canSend(c) && <> · {t`no SMTP account`}</>}
                </small>
              </span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </Field>
  )
}

export function ListFields({
  values,
  onChange,
  creating,
  error,
}: {
  values: ListValues
  onChange: (patch: Partial<ListValues>) => void
  creating?: boolean
  error?: unknown
}) {
  const { t } = useLingui()
  const inboxes = useInboxes().data ?? []
  const code = error instanceof ApiError ? error.code : undefined
  const inboxItems = Object.fromEntries(inboxes.map((i) => [i.id, i.name]))
  return (
    <>
      {creating && inboxes.length > 1 && (
        <Field
          label={<Trans>Inbox</Trans>}
          hint={<Trans>The list sends in this inbox's name, and replies to its mail land there.</Trans>}
        >
          <Select
            value={values.inbox_id}
            items={inboxItems}
            onValueChange={(v) => v && onChange({ inbox_id: String(v), channel_id: "" })}
          >
            <SelectTrigger className="w-full min-w-0" aria-label={t`Inbox`} data-testid="list-inbox">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {inboxes.map((i) => (
                <SelectItem key={i.id} value={i.id}>
                  <Dot color={i.branding.color} className="mt-1.5 size-2 rounded-[3px]" />
                  {i.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
      )}
      <Field
        label={<Trans>Name</Trans>}
        htmlFor="list-name"
        error={code === "name_taken" ? <Trans>This inbox already has a list with this name.</Trans> : undefined}
      >
        <Input
          id="list-name"
          required
          maxLength={100}
          value={values.name}
          placeholder={t`Product news`}
          onChange={(e) => onChange({ name: e.target.value })}
          aria-invalid={code === "name_taken" || undefined}
          data-testid="list-name"
        />
      </Field>
      <Field
        label={<Trans>Description (optional)</Trans>}
        htmlFor="list-description"
        hint={
          <Trans>Shown on subscribe forms and the preference page, so say what people will get and how often.</Trans>
        }
      >
        <Textarea
          id="list-description"
          rows={2}
          maxLength={500}
          value={values.description}
          placeholder={t`One e-mail a month about what is new.`}
          onChange={(e) => onChange({ description: e.target.value })}
        />
      </Field>
      {values.inbox_id && (
        <SenderPicker
          key={values.inbox_id}
          inboxId={values.inbox_id}
          value={values.channel_id}
          onChange={(channel_id) => onChange({ channel_id })}
          invalid={code === "email_channel_required"}
        />
      )}
      <div className="flex flex-col gap-4">
        <ToggleRow
          title={<Trans>Ask for a confirmation</Trans>}
          hint={
            <Trans>People who sign up on a form confirm by clicking a link in a mail before they are subscribed.</Trans>
          }
        >
          <Switch
            checked={values.double_opt_in}
            onCheckedChange={(on) => onChange({ double_opt_in: on })}
            aria-label={t`Ask for a confirmation`}
            data-testid="list-double-opt-in"
          />
        </ToggleRow>
        {!values.double_opt_in && (
          <Notice tone="warning" icon={TriangleAlertIcon} className="text-small" data-testid="list-single-opt-in">
            <Trans>
              Without a confirmation anyone can type someone else's address into your form, and you cannot show that the
              owner of the address agreed. Keep it on unless your forms are only shown to people whose address you have
              already verified.
            </Trans>
          </Notice>
        )}
        <ToggleRow
          title={<Trans>Public</Trans>}
          hint={
            <Trans>
              Offered on subscribe forms and the preference page. Private lists are filled by the team and the API.
            </Trans>
          }
        >
          <Switch
            checked={values.public}
            onCheckedChange={(on) => onChange({ public: on })}
            aria-label={t`Public`}
            data-testid="list-public"
          />
        </ToggleRow>
        <ToggleRow
          title={<Trans>Marketing content</Trans>}
          hint={
            <Trans>
              Turn off for service announcements to existing customers. Consent is needed either way; this is recorded
              in the consent export.
            </Trans>
          }
        >
          <Switch
            checked={values.commercial}
            onCheckedChange={(on) => onChange({ commercial: on })}
            aria-label={t`Marketing content`}
            data-testid="list-commercial"
          />
        </ToggleRow>
        <ToggleRow
          title={<Trans>Welcome receipt</Trans>}
          hint={
            <Trans>Right after subscribing, people get a short mail that confirms it, with an unsubscribe link.</Trans>
          }
        >
          <Switch
            checked={values.welcome}
            onCheckedChange={(on) => onChange({ welcome: on })}
            aria-label={t`Welcome receipt`}
            data-testid="list-welcome"
          />
        </ToggleRow>
      </div>
    </>
  )
}

export function NewListDialog({
  inboxId,
  onClose,
  onCreated,
}: {
  inboxId: string
  onClose: () => void
  onCreated: (l: MailingList) => void
}) {
  const inboxes = useInboxes().data ?? []
  const setList = useSetList()
  const [values, setValues] = useState<ListValues>(() => ({
    inbox_id: inboxId || inboxes[0]?.id || "",
    name: "",
    description: "",
    double_opt_in: true,
    public: true,
    commercial: true,
    welcome: true,
    channel_id: "",
  }))
  const [picked, setPicked] = useState<string | null>(null)
  const channels = useInboxChannels(values.inbox_id).data
  const sender = picked ?? channels?.find(canSend)?.id ?? ""
  const create = useMutation({
    mutationFn: () => {
      const body: MailingListCreate = {
        ...values,
        name: values.name.trim(),
        description: values.description.trim(),
        channel_id: sender || undefined,
      }
      return unwrap(api.POST("/v1/lists", { body }))
    },
    onSuccess: (l) => {
      setList(l)
      onCreated(l)
    },
  })
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90svh] p-0 sm:max-w-lg" data-testid="list-dialog">
        <form
          className="flex min-w-0 flex-col"
          onSubmit={(e) => {
            e.preventDefault()
            if (values.name.trim() && values.inbox_id) create.mutate()
          }}
        >
          <div className="flex flex-col gap-5 p-6">
            <DialogHeader>
              <DialogTitle>
                <Trans>New mailing list</Trans>
              </DialogTitle>
              <DialogDescription>
                <Trans>
                  People join a list on a subscribe form or are added with their consent. Broadcasts go to lists.
                </Trans>
              </DialogDescription>
            </DialogHeader>
            <ListFields
              values={{ ...values, channel_id: sender }}
              onChange={({ channel_id, ...patch }) => {
                if (channel_id !== undefined) setPicked(patch.inbox_id ? null : channel_id)
                setValues((v) => ({ ...v, ...patch }))
              }}
              creating
              error={create.error}
            />
            {channels && !sender && <SenderMissing inboxId={values.inbox_id} />}
            <ErrorLine error={create.error} />
          </div>
          <DialogFooter className="sticky bottom-0 border-t bg-card px-6 py-3">
            <Button type="submit" disabled={create.isPending} data-testid="list-create">
              <Trans>Create list</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

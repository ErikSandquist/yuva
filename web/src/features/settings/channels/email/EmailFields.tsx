import { Trans, useLingui } from "@lingui/react/macro"
import { CheckIcon, ShieldCheckIcon } from "lucide-react"
import { Link } from "react-router"

import { CodeLine, Notice } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { SesStatus } from "@/features/settings/channels/email/Ses"
import { isSesHost, SES_NAME, TLS, type EmailField, type EmailForm } from "@/features/settings/channels/email/form"
import { FieldError, FormBlock } from "@/features/settings/channels/parts"
import { Field, ToggleRow } from "@/features/settings/ui"
import type { EmailSes, SmtpTls } from "@/lib/api"

export type Props = {
  f: EmailForm
  set: (patch: Partial<EmailForm>) => void
  error: (field: EmailField) => string | undefined
  taken: boolean
  sendingLink?: string
  ses?: EmailSes
}

export function useInvalid({ error, taken }: Pick<Props, "error" | "taken">) {
  return (field: EmailField) => ({
    "aria-invalid": !!error(field) || (field === "address" && taken) || undefined,
    "aria-describedby": error(field) ? `err-${field}` : undefined,
  })
}

export function ReceivingFields({ f, set, error, taken }: Props) {
  const invalid = useInvalid({ error, taken })
  const ingress = `${window.location.origin}/ingress/email`
  return (
    <>
      <Field
        label={<Trans>Support address</Trans>}
        htmlFor="email-address"
        hint={
          <Trans>
            *@example.com receives every address of the domain that no other channel has; replies then go out from the
            address the customer wrote to.
          </Trans>
        }
      >
        <Input
          id="email-address"
          type="email"
          required
          maxLength={320}
          value={f.address}
          placeholder="support@example.com"
          onChange={(e) => set({ address: e.target.value })}
          {...invalid("address")}
        />
        <FieldError id="err-address">
          {taken ? <Trans>Another channel already receives mail at this address.</Trans> : error("address")}
        </FieldError>
      </Field>
      <Notice className="text-caption" data-testid="ingress-hint">
        <p>
          <Trans>
            Forward mail for this address to the Cloudflare Email Worker in edge/ of the Yuva repository, or to any mail
            server that posts each message to this server's ingress URL, signed with YUVA_INGRESS_SECRET:
          </Trans>
        </p>
        <CodeLine value={ingress} />
      </Notice>
    </>
  )
}

function Receiving(p: Props) {
  return (
    <FormBlock title={<Trans>Receiving</Trans>}>
      <ReceivingFields {...p} />
    </FormBlock>
  )
}

function SmtpPassword({
  id,
  value,
  isSet,
  clearing,
  onChange,
  invalid,
  error,
  errorId,
  testId,
}: {
  id: string
  errorId: string
  value: string | null
  isSet: boolean
  clearing: boolean
  onChange: (patch: { password?: string | null; passwordSet?: boolean; clearing?: boolean }) => void
  invalid: ReturnType<ReturnType<typeof useInvalid>>
  error?: string
  testId: string
}) {
  const { t } = useLingui()
  return (
    <Field label={<Trans>Password</Trans>} htmlFor={id}>
      {value === null ? (
        <div className="flex h-9 items-center gap-2" data-testid={`${testId}-set`}>
          <span className="inline-flex min-w-0 flex-1 items-center gap-1.5 text-body">
            <CheckIcon className="size-4 shrink-0 text-success" />
            <Trans>Password set</Trans>
          </span>
          <Button type="button" variant="outline" size="sm" onClick={() => onChange({ password: "" })}>
            <Trans>Replace</Trans>
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onChange({ password: "", passwordSet: false, clearing: true })}
          >
            <Trans>Remove</Trans>
          </Button>
        </div>
      ) : (
        <div className="flex items-center gap-2">
          <Input
            id={id}
            type="password"
            maxLength={1024}
            value={value}
            autoComplete="new-password"
            placeholder={isSet ? t`New password` : undefined}
            onChange={(e) => onChange({ password: e.target.value })}
            {...invalid}
          />
          {(isSet || clearing) && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => onChange({ password: null, passwordSet: true, clearing: false })}
            >
              <Trans>Keep</Trans>
            </Button>
          )}
        </div>
      )}
      {clearing && value === "" && (
        <p className="text-caption text-muted-foreground" data-testid={`${testId}-cleared`}>
          <Trans>Saving removes the stored password.</Trans>
        </p>
      )}
      <FieldError id={errorId}>{error}</FieldError>
    </Field>
  )
}

function Password({ f, set, error, taken }: Props) {
  const invalid = useInvalid({ error, taken })
  return (
    <SmtpPassword
      id="smtp-password"
      value={f.password}
      isSet={f.passwordSet}
      clearing={f.clearing}
      onChange={(p) => set(p)}
      invalid={invalid("smtp.password")}
      error={error("smtp.password")}
      errorId="err-smtp.password"
      testId="smtp-password"
    />
  )
}

function useTlsText(): Record<SmtpTls, string> {
  const { t } = useLingui()
  return {
    starttls: t`STARTTLS (port 587)`,
    tls: t`TLS (port 465)`,
    none: t`None (local relays and test servers only)`,
  }
}

export function SenderFields(p: Props) {
  const { t } = useLingui()
  const { f, set, error } = p
  const invalid = useInvalid(p)
  return (
    <div className="grid gap-5 sm:grid-cols-2">
      <Field
        label={<Trans>Display name</Trans>}
        htmlFor="email-display-name"
        hint={<Trans>The channel name when empty.</Trans>}
      >
        <Input
          id="email-display-name"
          maxLength={200}
          value={f.displayName}
          placeholder={t`Acme Support`}
          onChange={(e) => set({ displayName: e.target.value })}
          {...invalid("display_name")}
        />
        <FieldError id="err-display_name">{error("display_name")}</FieldError>
      </Field>
      <Field
        label={<Trans>From address (optional)</Trans>}
        htmlFor="email-from"
        hint={<Trans>When your provider sends from another address. Replies still come to the support address.</Trans>}
      >
        <Input
          id="email-from"
          type="email"
          maxLength={320}
          value={f.fromAddress}
          placeholder="no-reply@example.com"
          onChange={(e) => set({ fromAddress: e.target.value })}
          {...invalid("from_address")}
        />
        <FieldError id="err-from_address">{error("from_address")}</FieldError>
      </Field>
    </div>
  )
}

export function SmtpHostField(p: Props) {
  const { f, set, error } = p
  const invalid = useInvalid(p)
  return (
    <Field label={<Trans>SMTP host</Trans>} htmlFor="smtp-host">
      <Input
        id="smtp-host"
        maxLength={253}
        value={f.host}
        placeholder="smtp.example.com"
        autoComplete="off"
        onChange={(e) => set({ host: e.target.value })}
        {...invalid("smtp.host")}
      />
      <FieldError id="err-smtp.host">{error("smtp.host")}</FieldError>
    </Field>
  )
}

export function SmtpPortField(p: Props) {
  const { f, set, error } = p
  const invalid = useInvalid(p)
  return (
    <Field label={<Trans>Port</Trans>} htmlFor="smtp-port">
      <Input
        id="smtp-port"
        type="number"
        min={1}
        max={65535}
        value={f.port}
        placeholder={f.tls === "tls" ? "465" : "587"}
        onChange={(e) => set({ port: e.target.value })}
        {...invalid("smtp.port")}
      />
      <FieldError id="err-smtp.port">{error("smtp.port")}</FieldError>
    </Field>
  )
}

export function SmtpEncryptionField(p: Props) {
  const { t } = useLingui()
  const { f, set, error } = p
  const invalid = useInvalid(p)
  const tlsText = useTlsText()
  return (
    <Field label={<Trans>Encryption</Trans>}>
      <Select value={f.tls} onValueChange={(v) => set({ tls: v as SmtpTls })} items={tlsText}>
        <SelectTrigger className="w-full" aria-label={t`Encryption`} {...invalid("smtp.tls")}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {TLS.map((x) => (
            <SelectItem key={x} value={x}>
              {tlsText[x]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <FieldError id="err-smtp.tls">{error("smtp.tls")}</FieldError>
    </Field>
  )
}

export function SmtpLoginFields(p: Props) {
  const { f, set, error } = p
  const invalid = useInvalid(p)
  return (
    <>
      <div className="grid gap-5 sm:grid-cols-2">
        <Field label={<Trans>Username</Trans>} htmlFor="smtp-username">
          <Input
            id="smtp-username"
            maxLength={320}
            value={f.username}
            autoComplete="off"
            onChange={(e) => set({ username: e.target.value })}
            {...invalid("smtp.username")}
          />
          <FieldError id="err-smtp.username">{error("smtp.username")}</FieldError>
        </Field>
        <Password {...p} />
      </div>
      <p className="text-caption text-muted-foreground">
        <Trans>The password is stored encrypted and never shown again.</Trans>
      </p>
    </>
  )
}

export function SendingFields(p: Props) {
  return (
    <>
      <SenderFields {...p} />
      <p className="text-caption text-muted-foreground">
        <Trans>Replies go out through your own SMTP account. Leave the host empty to only receive mail.</Trans>
      </p>
      <div className="grid gap-5 sm:grid-cols-[1fr_7rem]">
        <SmtpHostField {...p} />
        <SmtpPortField {...p} />
      </div>
      <SmtpEncryptionField {...p} />
      <SmtpLoginFields {...p} />
    </>
  )
}

export function Sending(p: Props) {
  return (
    <FormBlock title={<Trans>Sending</Trans>}>
      <SendingFields {...p} />
    </FormBlock>
  )
}

export function AutoReplyFields(p: Props) {
  const { t } = useLingui()
  const { f, set, error } = p
  const invalid = useInvalid(p)
  return (
    <>
      <ToggleRow
        title={<Trans>Send an auto-reply</Trans>}
        hint={
          <Trans>
            Greets new conversations by e-mail, once per contact within the interval. Never sent to automatic mail or
            spam.
          </Trans>
        }
      >
        <Switch
          checked={f.autoReply}
          onCheckedChange={(autoReply) => set({ autoReply })}
          aria-label={t`Send an auto-reply`}
        />
      </ToggleRow>
      {f.autoReply && (
        <>
          <Field label={<Trans>Text</Trans>} htmlFor="auto-reply-text">
            <Textarea
              id="auto-reply-text"
              rows={4}
              maxLength={5000}
              required
              value={f.autoReplyText}
              placeholder={t`Thanks for writing. We usually answer within a day.`}
              onChange={(e) => set({ autoReplyText: e.target.value })}
              {...invalid("auto_reply.text")}
            />
            <FieldError id="err-auto_reply.text">{error("auto_reply.text")}</FieldError>
          </Field>
          <Field label={<Trans>At most once per contact every (hours)</Trans>} htmlFor="auto-reply-interval">
            <Input
              id="auto-reply-interval"
              type="number"
              min={1}
              max={8760}
              className="w-32"
              value={f.interval}
              onChange={(e) => set({ interval: e.target.value })}
              {...invalid("auto_reply.interval_hours")}
            />
            <FieldError id="err-auto_reply.interval_hours">{error("auto_reply.interval_hours")}</FieldError>
          </Field>
        </>
      )}
    </>
  )
}

function AutoReply(p: Props) {
  return (
    <FormBlock title={<Trans>Auto-reply</Trans>}>
      <AutoReplyFields {...p} />
    </FormBlock>
  )
}

export function BroadcastFields(p: Props) {
  const { t } = useLingui()
  const { f, set, error } = p
  const invalid = useInvalid(p)
  const tlsText = useTlsText()
  return (
    <>
      <p className="-mt-2 text-caption text-muted-foreground">
        <Trans>
          Broadcasts from this channel go out through its SMTP account for replies, or through a separate account. A
          separate account, ideally on its own subdomain, keeps spam complaints about newsletters away from your support
          mail.
        </Trans>
      </p>
      <ToggleRow
        title={<Trans>Use a separate account for broadcasts</Trans>}
        hint={
          f.bcHad && !f.bc ? (
            <Trans>Saving removes the broadcast account; broadcasts then use the account for replies.</Trans>
          ) : undefined
        }
      >
        <Switch
          checked={f.bc}
          onCheckedChange={(bc) => set({ bc })}
          aria-label={t`Use a separate account for broadcasts`}
          data-testid="broadcast-smtp-toggle"
        />
      </ToggleRow>
      {f.bc && (
        <>
          <div className="grid gap-5 sm:grid-cols-[1fr_7rem]">
            <Field label={<Trans>SMTP host</Trans>} htmlFor="broadcast-smtp-host">
              <Input
                id="broadcast-smtp-host"
                required
                maxLength={253}
                value={f.bcHost}
                placeholder="smtp.news.example.com"
                autoComplete="off"
                onChange={(e) => set({ bcHost: e.target.value })}
                {...invalid("broadcast_smtp.host")}
              />
              <FieldError id="err-broadcast_smtp.host">{error("broadcast_smtp.host")}</FieldError>
            </Field>
            <Field label={<Trans>Port</Trans>} htmlFor="broadcast-smtp-port">
              <Input
                id="broadcast-smtp-port"
                type="number"
                min={1}
                max={65535}
                value={f.bcPort}
                placeholder={f.bcTls === "tls" ? "465" : "587"}
                onChange={(e) => set({ bcPort: e.target.value })}
                {...invalid("broadcast_smtp.port")}
              />
              <FieldError id="err-broadcast_smtp.port">{error("broadcast_smtp.port")}</FieldError>
            </Field>
          </div>
          <Field label={<Trans>Encryption</Trans>}>
            <Select value={f.bcTls} onValueChange={(v) => set({ bcTls: v as SmtpTls })} items={tlsText}>
              <SelectTrigger className="w-full" aria-label={t`Encryption`} {...invalid("broadcast_smtp.tls")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {TLS.map((x) => (
                  <SelectItem key={x} value={x}>
                    {tlsText[x]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <FieldError id="err-broadcast_smtp.tls">{error("broadcast_smtp.tls")}</FieldError>
          </Field>
          <div className="grid gap-5 sm:grid-cols-2">
            <Field label={<Trans>Username</Trans>} htmlFor="broadcast-smtp-username">
              <Input
                id="broadcast-smtp-username"
                maxLength={320}
                value={f.bcUsername}
                autoComplete="off"
                onChange={(e) => set({ bcUsername: e.target.value })}
                {...invalid("broadcast_smtp.username")}
              />
              <FieldError id="err-broadcast_smtp.username">{error("broadcast_smtp.username")}</FieldError>
            </Field>
            <SmtpPassword
              id="broadcast-smtp-password"
              errorId="err-broadcast_smtp.password"
              value={f.bcPassword}
              isSet={f.bcPasswordSet}
              clearing={f.bcClearing}
              onChange={(x) =>
                set({
                  ...(x.password !== undefined ? { bcPassword: x.password } : {}),
                  ...(x.passwordSet !== undefined ? { bcPasswordSet: x.passwordSet } : {}),
                  ...(x.clearing !== undefined ? { bcClearing: x.clearing } : {}),
                })
              }
              invalid={invalid("broadcast_smtp.password")}
              error={error("broadcast_smtp.password")}
              testId="broadcast-smtp-password"
            />
          </div>
        </>
      )}
      <Field
        label={<Trans>Pace (mails a second)</Trans>}
        htmlFor="broadcast-rate"
        hint={
          <span className="flex flex-col gap-1">
            <span>
              <Trans>
                Stay under your provider’s sending rate. Support replies are never slowed down by a broadcast.
              </Trans>
            </span>
            <span data-testid="broadcast-rate-hints">
              <Trans>
                Amazon SES shows your account’s maximum send rate in its console; new accounts often start at 14. A
                mailbox account such as Google Workspace or Microsoft 365 is not made for bulk mail: keep it at 1.
              </Trans>
            </span>
          </span>
        }
      >
        <Input
          id="broadcast-rate"
          type="number"
          min={1}
          max={100}
          className="w-32"
          value={f.rate}
          onChange={(e) => set({ rate: e.target.value })}
          {...invalid("broadcast_rate")}
          data-testid="broadcast-rate"
        />
        <FieldError id="err-broadcast_rate">{error("broadcast_rate")}</FieldError>
      </Field>
    </>
  )
}

function BroadcastSending(p: Props) {
  return (
    <FormBlock title={<Trans>Broadcast sending</Trans>} data-testid="broadcast-sending">
      <BroadcastFields {...p} />
      {p.sendingLink && (
        <Link
          to={p.sendingLink}
          className="inline-flex w-fit items-center gap-1.5 text-body text-brand underline-offset-4 hover:underline"
          data-testid="channel-deliverability-link"
        >
          <ShieldCheckIcon className="size-4" />
          <Trans>Check deliverability and set up DKIM</Trans>
        </Link>
      )}
    </FormBlock>
  )
}

export function sesShown({ f, ses }: Pick<Props, "f" | "ses">) {
  return !!ses || isSesHost(f.host) || (f.bc && isSesHost(f.bcHost))
}

export function SesFields(p: Props) {
  const { f, set, error, ses } = p
  const invalid = useInvalid(p)
  const { t } = useLingui()
  const errorText = t`Use letters, digits, - and _, at most 64.`
  const typed = ses?.source !== "server"
  const bad = (v: string) => v !== "" && !/^[A-Za-z0-9_-]{1,64}$/.test(v.trim())
  const nameError = (field: "ses.tenant" | "ses.configuration_set", v: string) =>
    error(field) ?? (bad(v) ? errorText : undefined)
  const invalidName = (field: "ses.tenant" | "ses.configuration_set", v: string) => ({
    ...invalid(field),
    "aria-invalid": bad(v) || !!error(field) || undefined,
    "aria-describedby": bad(v) || error(field) ? `err-${field}` : undefined,
  })
  return (
    <>
      {ses && <SesStatus ses={ses} address={f.fromAddress || f.address} />}
      {typed && (
        <>
          <p className="-mt-2 text-caption text-muted-foreground">
            <Trans>
              If your Amazon SES account uses tenants, name the tenant this channel’s mail belongs to. Yuva adds it as
              X-SES-TENANT; the tenant must have this sending domain and the configuration set.
            </Trans>
          </p>
          <div className="grid gap-5 sm:grid-cols-2">
            <Field label={<Trans>SES tenant</Trans>} htmlFor="ses-tenant">
              <Input
                id="ses-tenant"
                maxLength={64}
                pattern={SES_NAME}
                dir="ltr"
                autoComplete="off"
                value={f.sesTenant}
                onChange={(e) => set({ sesTenant: e.target.value })}
                {...invalidName("ses.tenant", f.sesTenant)}
              />
              <FieldError id="err-ses.tenant">{nameError("ses.tenant", f.sesTenant)}</FieldError>
            </Field>
            <Field label={<Trans>Configuration set</Trans>} htmlFor="ses-configuration-set">
              <Input
                id="ses-configuration-set"
                maxLength={64}
                pattern={SES_NAME}
                dir="ltr"
                autoComplete="off"
                value={f.sesSet}
                onChange={(e) => set({ sesSet: e.target.value })}
                {...invalidName("ses.configuration_set", f.sesSet)}
              />
              <FieldError id="err-ses.configuration_set">{nameError("ses.configuration_set", f.sesSet)}</FieldError>
            </Field>
          </div>
        </>
      )}
    </>
  )
}

function SesTenant(p: Props) {
  if (!sesShown(p)) return null
  return (
    <FormBlock title={<Trans>Amazon SES tenant</Trans>} data-testid="ses-tenant">
      <SesFields {...p} />
    </FormBlock>
  )
}

export function EmailFields(p: Props) {
  return (
    <>
      <Receiving {...p} />
      <Sending {...p} />
      <BroadcastSending {...p} />
      <SesTenant {...p} />
      <AutoReply {...p} />
    </>
  )
}

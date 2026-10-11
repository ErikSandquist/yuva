import { useLingui } from "@lingui/react/macro"

import { ApiError, type EmailChannel, type EmailChannelInput, type SmtpTls } from "@/lib/api"

export const TLS: SmtpTls[] = ["starttls", "tls", "none"]

export type EmailForm = {
  address: string
  displayName: string
  fromAddress: string
  host: string
  port: string
  username: string
  tls: SmtpTls
  password: string | null
  passwordSet: boolean
  clearing: boolean
  autoReply: boolean
  autoReplyText: string
  interval: string
  bc: boolean
  bcHad: boolean
  bcHost: string
  bcPort: string
  bcUsername: string
  bcTls: SmtpTls
  bcPassword: string | null
  bcPasswordSet: boolean
  bcClearing: boolean
  rate: string
  rateWas: string
}

export function emailForm(e?: EmailChannel): EmailForm {
  return {
    address: e?.address ?? "",
    displayName: e?.display_name ?? "",
    fromAddress: e?.from_address ?? "",
    host: e?.smtp?.host ?? "",
    port: e?.smtp ? String(e.smtp.port) : "",
    username: e?.smtp?.username ?? "",
    tls: e?.smtp?.tls ?? "starttls",
    password: e?.smtp?.password_set ? null : "",
    passwordSet: !!e?.smtp?.password_set,
    clearing: false,
    autoReply: e?.auto_reply.enabled ?? false,
    autoReplyText: e?.auto_reply.text ?? "",
    interval: String(e?.auto_reply.interval_hours ?? 24),
    bc: !!e?.broadcast_smtp,
    bcHad: !!e?.broadcast_smtp,
    bcHost: e?.broadcast_smtp?.host ?? "",
    bcPort: e?.broadcast_smtp ? String(e.broadcast_smtp.port) : "",
    bcUsername: e?.broadcast_smtp?.username ?? "",
    bcTls: e?.broadcast_smtp?.tls ?? "starttls",
    bcPassword: e?.broadcast_smtp?.password_set ? null : "",
    bcPasswordSet: !!e?.broadcast_smtp?.password_set,
    bcClearing: false,
    rate: String(e?.broadcast_rate ?? 5),
    rateWas: String(e?.broadcast_rate ?? 5),
  }
}

export function emailInput(f: EmailForm): EmailChannelInput {
  const out: EmailChannelInput = {
    address: f.address.trim(),
    display_name: f.displayName.trim(),
    auto_reply: { enabled: f.autoReply, text: f.autoReplyText.trim(), interval_hours: Number(f.interval) || 24 },
  }
  if (f.fromAddress.trim()) out.from_address = f.fromAddress.trim()
  if (f.host.trim()) {
    out.smtp = { host: f.host.trim(), username: f.username.trim(), tls: f.tls }
    if (f.port.trim()) out.smtp.port = Number(f.port)
    if (f.password !== null) out.smtp.password = f.password
  }
  if (f.bc && f.bcHost.trim()) {
    out.broadcast_smtp = { host: f.bcHost.trim(), username: f.bcUsername.trim(), tls: f.bcTls }
    if (f.bcPort.trim()) out.broadcast_smtp.port = Number(f.bcPort)
    if (f.bcPassword !== null) out.broadcast_smtp.password = f.bcPassword
  } else if (f.bcHad) {
    out.broadcast_smtp_remove = true
  }
  if (f.rate !== f.rateWas && f.rate.trim()) out.broadcast_rate = Number(f.rate)
  return out
}

export type EmailField =
  | "address"
  | "display_name"
  | "from_address"
  | "smtp.host"
  | "smtp.port"
  | "smtp.username"
  | "smtp.password"
  | "smtp.tls"
  | "auto_reply.text"
  | "auto_reply.interval_hours"
  | "broadcast_smtp.host"
  | "broadcast_smtp.port"
  | "broadcast_smtp.username"
  | "broadcast_smtp.password"
  | "broadcast_smtp.tls"
  | "broadcast_rate"

export function emailErrorField(err: unknown): EmailField | null {
  if (!(err instanceof ApiError)) return null
  if (err.code === "email_address_taken") return "address"
  if (err.status !== 400 || !err.detail) return null
  const m = /\bemail\.([a-z_]+(?:\.[a-z_]+)?)/.exec(err.detail)
  return (m?.[1] as EmailField | undefined) ?? null
}

export function useEmailFieldErrors() {
  const { t } = useLingui()
  return {
    address: t`Enter a valid e-mail address.`,
    display_name: t`Use at most 200 characters.`,
    from_address: t`Enter a valid e-mail address, or leave it empty.`,
    "smtp.host": t`Enter a host name or IP address.`,
    "smtp.port": t`Use a port from 1 to 65535.`,
    "smtp.username": t`A username needs TLS or STARTTLS: the password is never sent unencrypted.`,
    "smtp.password": t`The password is too long.`,
    "smtp.tls": t`Pick a TLS mode.`,
    "auto_reply.text": t`Write the auto-reply, or turn it off.`,
    "auto_reply.interval_hours": t`Use 1 to 8760 hours.`,
    "broadcast_smtp.host": t`Enter a host name or IP address.`,
    "broadcast_smtp.port": t`Use a port from 1 to 65535.`,
    "broadcast_smtp.username": t`A username needs TLS or STARTTLS: the password is never sent unencrypted.`,
    "broadcast_smtp.password": t`The password is too long.`,
    "broadcast_smtp.tls": t`Pick a TLS mode.`,
    broadcast_rate: t`Use 1 to 100 mails a second.`,
  } satisfies Record<EmailField, string>
}

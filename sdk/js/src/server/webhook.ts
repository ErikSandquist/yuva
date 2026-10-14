import type { webhooks } from "../schema.gen"
import { base64, decodeBase64, hmacKey, utf8 } from "./encoding"

type Payload<K extends keyof webhooks> = webhooks[K]["post"] extends {
  requestBody: { content: { "application/json": infer Body } }
}
  ? Body
  : never

export type WebhookEventType = keyof webhooks
export type WebhookEvent = { [K in WebhookEventType]: Omit<Payload<K>, "type"> & { type: K } }[WebhookEventType]

export const DEFAULT_WEBHOOK_TOLERANCE = 300

export type WebhookErrorCode =
  | "invalid_secret"
  | "missing_headers"
  | "invalid_timestamp"
  | "timestamp_too_old"
  | "timestamp_too_new"
  | "invalid_signature"

const messages: Record<WebhookErrorCode, string> = {
  invalid_secret: "webhook: the secret is not a whsec_ base64 secret",
  missing_headers: "webhook: webhook-id, webhook-timestamp or webhook-signature is missing",
  invalid_timestamp: "webhook: webhook-timestamp is not a unix time",
  timestamp_too_old: "webhook: webhook-timestamp is too old",
  timestamp_too_new: "webhook: webhook-timestamp is too far in the future",
  invalid_signature: "webhook: no signature matches",
}

export class WebhookVerificationError extends Error {
  readonly code: WebhookErrorCode

  constructor(code: WebhookErrorCode) {
    super(messages[code])
    this.name = "WebhookVerificationError"
    this.code = code
  }
}

export interface VerifyWebhookOptions {
  tolerance?: number
  now?: Date
}

function key(secret: string): Uint8Array<ArrayBuffer> {
  const bytes = decodeBase64(secret.trim().replace(/^whsec_/, ""))
  if (!bytes || bytes.length === 0) throw new WebhookVerificationError("invalid_secret")
  return bytes
}

function signed(id: string, timestamp: string, body: Uint8Array): Uint8Array<ArrayBuffer> {
  const prefix = utf8(`${id}.${timestamp}.`)
  const data = new Uint8Array(prefix.length + body.length)
  data.set(prefix)
  data.set(body, prefix.length)
  return data
}

export async function signWebhook(
  secret: string,
  id: string,
  timestamp: Date,
  body: string | Uint8Array
): Promise<string> {
  const k = await hmacKey(key(secret), ["sign"])
  const data = signed(id, String(Math.floor(timestamp.getTime() / 1000)), typeof body === "string" ? utf8(body) : body)
  return `v1,${base64(new Uint8Array(await crypto.subtle.sign("HMAC", k, data)))}`
}

export async function verifyWebhookBody(
  secret: string,
  headers: Headers,
  body: Uint8Array,
  { tolerance, now = new Date() }: VerifyWebhookOptions = {}
): Promise<void> {
  const raw = key(secret)
  const id = headers.get("webhook-id")
  const ts = headers.get("webhook-timestamp")
  const sigs = headers.get("webhook-signature")
  if (!id || !ts || !sigs) throw new WebhookVerificationError("missing_headers")
  if (!/^[+-]?\d{1,19}$/.test(ts)) throw new WebhookVerificationError("invalid_timestamp")
  const window = (tolerance && tolerance > 0 ? tolerance : DEFAULT_WEBHOOK_TOLERANCE) * 1000
  const at = Number(ts) * 1000
  if (now.getTime() - at > window) throw new WebhookVerificationError("timestamp_too_old")
  if (at - now.getTime() > window) throw new WebhookVerificationError("timestamp_too_new")
  const k = await hmacKey(raw, ["verify"])
  const data = signed(id, ts, body)
  for (const sig of sigs.split(/\s+/)) {
    const comma = sig.indexOf(",")
    if (comma < 0 || sig.slice(0, comma) !== "v1") continue
    const got = decodeBase64(sig.slice(comma + 1))
    if (got && got.length > 0 && (await crypto.subtle.verify("HMAC", k, got, data))) return
  }
  throw new WebhookVerificationError("invalid_signature")
}

export async function verifyWebhook<T = WebhookEvent>(
  secret: string,
  request: Request,
  options?: VerifyWebhookOptions
): Promise<T> {
  const body = new Uint8Array(await request.arrayBuffer())
  await verifyWebhookBody(secret, request.headers, body, options)
  return JSON.parse(new TextDecoder().decode(body)) as T
}

import { base64url, goJSON, hmacKey, utf8 } from "./encoding"

export const DEFAULT_IDENTITY_TTL = 300
export const MAX_IDENTITY_TTL = 600

export interface IdentityClaims {
  subject: string
  email?: string
  emailVerified?: boolean
  name?: string
  locale?: string
  attrs?: Record<string, unknown>
  id?: string
  issuedAt?: Date
  ttl?: number
}

const header = base64url(utf8(`{"alg":"HS256","typ":"JWT"}`))

export async function signIdentityToken(secret: string, claims: IdentityClaims): Promise<string> {
  if (!secret) throw new Error("identity: the inbox identity secret is empty")
  const sub = (claims.subject ?? "").trim()
  if (!sub || [...sub].length > 200) throw new Error("identity: subject must be 1 to 200 characters")
  const ttl = claims.ttl || DEFAULT_IDENTITY_TTL
  if (!(ttl > 0 && ttl <= MAX_IDENTITY_TTL)) throw new Error("identity: ttl must be at most 10 minutes")
  const at = (claims.issuedAt ?? new Date()).getTime()
  const payload = {
    sub,
    email: claims.email || undefined,
    email_verified: claims.emailVerified || undefined,
    name: claims.name || undefined,
    locale: claims.locale || undefined,
    attrs: claims.attrs && Object.keys(claims.attrs).length > 0 ? claims.attrs : undefined,
    jti: claims.id || undefined,
    iat: Math.floor(at / 1000),
    exp: Math.floor((at + ttl * 1000) / 1000),
  }
  const fields = Object.entries(payload).flatMap(([key, value]) => {
    const encoded = goJSON(value)
    return encoded === undefined ? [] : [`"${key}":${encoded}`]
  })
  const signing = `${header}.${base64url(utf8(`{${fields.join(",")}}`))}`
  const key = await hmacKey(utf8(secret), ["sign"])
  return `${signing}.${base64url(new Uint8Array(await crypto.subtle.sign("HMAC", key, utf8(signing))))}`
}

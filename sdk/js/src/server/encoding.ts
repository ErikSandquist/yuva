const encoder = new TextEncoder()

export const utf8 = (text: string): Uint8Array<ArrayBuffer> => encoder.encode(text)

export function base64(bytes: Uint8Array): string {
  let binary = ""
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary)
}

export const base64url = (bytes: Uint8Array): string =>
  base64(bytes).replace(/=+$/, "").replace(/\+/g, "-").replace(/\//g, "_")

const strictBase64 = /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/

export function decodeBase64(text: string): Uint8Array<ArrayBuffer> | undefined {
  if (!strictBase64.test(text)) return undefined
  return Uint8Array.from(atob(text), (char) => char.charCodeAt(0))
}

export const hmacKey = (key: Uint8Array<ArrayBuffer>, usages: KeyUsage[]): Promise<CryptoKey> =>
  crypto.subtle.importKey("raw", key, { name: "HMAC", hash: "SHA-256" }, false, usages)

const goEscapes: Record<string, string> = {
  "<": "\\u003c",
  ">": "\\u003e",
  "&": "\\u0026",
  "\u2028": "\\u2028",
  "\u2029": "\\u2029",
}

// Go's encoding/json sorts map keys and escapes HTML characters; matching it keeps tokens byte-identical to sdk/go.
export function goJSON(value: unknown): string | undefined {
  if (value === null || typeof value !== "object" || typeof (value as { toJSON?: unknown }).toJSON === "function") {
    return JSON.stringify(value)?.replace(/[<>&\u2028\u2029]/g, (char) => goEscapes[char]!)
  }
  if (Array.isArray(value)) return `[${value.map((item) => goJSON(item) ?? "null").join(",")}]`
  const fields: string[] = []
  for (const key of Object.keys(value).sort()) {
    const encoded = goJSON((value as Record<string, unknown>)[key])
    if (encoded !== undefined) fields.push(`${goJSON(key)}:${encoded}`)
  }
  return `{${fields.join(",")}}`
}

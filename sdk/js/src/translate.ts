import type { MessageDescriptor, Messages } from "@lingui/core"

// Keeps @lingui/core out of the loader; only plain messages and simple placeholders (no plurals) belong here.
export function translate(messages: Messages, descriptor: MessageDescriptor, fallback: Messages): string {
  const entry = messages[descriptor.id] ?? fallback[descriptor.id]
  const values = (descriptor.values ?? {}) as Record<string, unknown>
  if (typeof entry === "string") return entry
  if (Array.isArray(entry)) {
    return entry
      .map((part) =>
        typeof part === "string" ? part : Array.isArray(part) ? String(values[part[0] as string] ?? "") : ""
      )
      .join("")
  }
  return descriptor.message ?? descriptor.id
}

const rtlLanguages = new Set(["ar", "fa", "he", "ur"])

export const sourceLocale = "en"

export function localeIn(catalogs: Record<string, Messages>, candidates: (string | null | undefined)[]): string {
  for (const candidate of candidates) {
    if (!candidate) continue
    const tag = candidate.toLowerCase()
    if (catalogs[tag]) return tag
    const language = tag.split(/[-_]/)[0] ?? ""
    if (catalogs[language]) return language
  }
  return sourceLocale
}

export function directionOf(locale: string): "ltr" | "rtl" {
  return rtlLanguages.has(locale.split(/[-_]/)[0] ?? "") ? "rtl" : "ltr"
}

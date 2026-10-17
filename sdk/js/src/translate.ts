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

const latinAmericanSpanish = new Set([
  "419",
  "us",
  "ar",
  "bo",
  "cl",
  "co",
  "cr",
  "cu",
  "do",
  "ec",
  "gt",
  "hn",
  "mx",
  "ni",
  "pa",
  "pe",
  "pr",
  "py",
  "sv",
  "uy",
  "ve",
])
const languageAliases: Record<string, string> = { iw: "he", in: "id", no: "nb", nn: "nb" }

function variantsOf(tag: string): string[] {
  const [language = "", ...rest] = tag.split("-")
  const base = languageAliases[language] ?? language
  const region = rest.find((part) => part.length === 2 || /^\d{3}$/.test(part))
  const out: string[] = []
  if (base === "zh") {
    const traditional = rest.includes("hant") || (!rest.includes("hans") && ["tw", "hk", "mo"].includes(region ?? ""))
    out.push(traditional ? "zh-hant" : "zh-hans")
  } else if (base === "pt") {
    out.push(region === "pt" ? "pt-pt" : "pt-br")
  } else if (base === "es") {
    out.push(region && latinAmericanSpanish.has(region) ? "es-mx" : "es")
  } else if (base === "fr") {
    out.push(region === "ca" ? "fr-ca" : "fr")
  }
  out.push(region ? `${base}-${region}` : base, base)
  return out
}

export function localeIn(catalogs: Record<string, Messages>, candidates: (string | null | undefined)[]): string {
  for (const candidate of candidates) {
    if (!candidate) continue
    const tag = candidate.toLowerCase().replace(/_/g, "-")
    if (catalogs[tag]) return tag
    for (const variant of variantsOf(tag)) if (catalogs[variant]) return variant
  }
  return sourceLocale
}

export function directionOf(locale: string): "ltr" | "rtl" {
  return rtlLanguages.has(locale.split(/[-_]/)[0] ?? "") ? "rtl" : "ltr"
}

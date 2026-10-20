import type { MessageDescriptor, Messages } from "@lingui/core"

type Part = string | [string, string?, Record<string, Part[]>?]

function render(parts: Part[], values: Record<string, unknown>, locale: string, hash?: number): string {
  return parts
    .map((part) => {
      if (typeof part === "string") return hash === undefined ? part : part.replaceAll("#", String(hash))
      const [name = "", kind, options] = part
      const value = values[name]
      if (kind !== "plural" || !options) return String(value ?? "")
      const count = Number(value)
      const branch = options[`=${count}`] ?? options[new Intl.PluralRules(locale).select(count)] ?? options.other ?? []
      return render(branch, values, locale, count)
    })
    .join("")
}

// Keeps @lingui/core out of the loader; plain messages, simple placeholders and plurals only.
export function translate(
  messages: Messages,
  descriptor: MessageDescriptor,
  fallback: Messages,
  locale = sourceLocale
): string {
  const entry = messages[descriptor.id] ?? fallback[descriptor.id]
  const values = (descriptor.values ?? {}) as Record<string, unknown>
  if (typeof entry === "string") return entry
  if (Array.isArray(entry)) return render(entry as Part[], values, locale)
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

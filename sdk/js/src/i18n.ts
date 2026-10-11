import type { MessageDescriptor, Messages } from "@lingui/core"
import { messages as en } from "./locales/en/loader"
import { messages as tr } from "./locales/tr/loader"
import { translate } from "./translate"

const catalogs: Record<string, Messages> = { en, tr }
const rtlLanguages = new Set(["ar", "fa", "he", "ur"])

export const sourceLocale = "en"

export function resolveLocale(...candidates: (string | null | undefined)[]): string {
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

export function catalogFor(locale: string): Messages {
  return catalogs[locale] ?? en
}

export function translatePlain(messages: Messages, descriptor: MessageDescriptor, fallback: Messages = en): string {
  return translate(messages, descriptor, fallback)
}

import type { MessageDescriptor, Messages } from "@lingui/core"
import { messages as en } from "./locales/en/loader"
import { messages as tr } from "./locales/tr/loader"
import { directionOf, localeIn, sourceLocale, translate } from "./translate"

const catalogs: Record<string, Messages> = { en, tr }

export { directionOf, sourceLocale }

export function resolveLocale(...candidates: (string | null | undefined)[]): string {
  return localeIn(catalogs, candidates)
}

export function catalogFor(locale: string): Messages {
  return catalogs[locale] ?? en
}

export function translatePlain(messages: Messages, descriptor: MessageDescriptor, fallback: Messages = en): string {
  return translate(messages, descriptor, fallback)
}

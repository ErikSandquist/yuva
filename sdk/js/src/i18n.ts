import type { MessageDescriptor, Messages } from "@lingui/core"
import { catalogs } from "./catalogs/loader"
import { messages as en } from "./locales/en/loader"
import { directionOf, localeIn, sourceLocale, translate } from "./translate"

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

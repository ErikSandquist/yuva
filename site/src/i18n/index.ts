import { setupI18n, type I18n, type Messages } from "@lingui/core"
import { bcp47, locales, type Locale } from "@/lib/routes"

const loaded = import.meta.glob<{ messages: Messages }>("../locales/*/messages.po", { eager: true })
const catalogs = Object.fromEntries(locales.map((l) => [l, loaded[`../locales/${l}/messages.po`]!.messages])) as Record<
  Locale,
  Messages
>
const cache = new Map<Locale, I18n>()

export function i18nFor(locale: Locale): I18n {
  let i18n = cache.get(locale)
  if (!i18n) {
    i18n = setupI18n({ locale, locales: [bcp47[locale]], messages: { [locale]: catalogs[locale] } })
    cache.set(locale, i18n)
  }
  return i18n
}

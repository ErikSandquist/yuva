import { i18n, type Messages } from "@lingui/core"

import { localeCodes, rtlLocales } from "@/localeList"

export type Locale = (typeof localeCodes)[number]
export type Catalog = "messages" | "analytics" | "designs"

type Loader = () => Promise<{ messages: Messages }>

const loaders: Record<Catalog, Record<string, Loader>> = {
  messages: import.meta.glob<{ messages: Messages }>("./locales/*/messages.po"),
  analytics: import.meta.glob<{ messages: Messages }>("./locales/*/analytics.po"),
  designs: import.meta.glob<{ messages: Messages }>("./locales/*/designs.po"),
}

const nativeOverrides: Record<string, string> = { id: "Bahasa Indonesia", or: "ଓଡ଼ିଆ" }

function nativeName(code: string) {
  if (nativeOverrides[code]) return nativeOverrides[code]
  try {
    const name = new Intl.DisplayNames([code], { type: "language" }).of(code)
    if (!name || name === code) return code
    return name.charAt(0).toLocaleUpperCase(code) + name.slice(1)
  } catch {
    return code
  }
}

export const locales = Object.fromEntries(localeCodes.map((c) => [c, nativeName(c)])) as Record<Locale, string>

export function isLocale(value: unknown): value is Locale {
  return typeof value === "string" && (localeCodes as readonly string[]).includes(value)
}

const wanted = new Set<Catalog>(["messages"])
const loaded = new Set<string>()

async function loadCatalog(catalog: Catalog, locale: string) {
  const key = `${catalog}/${locale}`
  if (loaded.has(key)) return
  const load = loaders[catalog][`./locales/${locale}/${catalog}.po`]
  if (!load) return
  i18n.load(locale, (await load()).messages)
  loaded.add(key)
}

function loadAll(locale: string) {
  return Promise.all([...wanted].map((c) => loadCatalog(c, locale)))
}

export async function needCatalog(catalog: Catalog) {
  wanted.add(catalog)
  if (i18n.locale) await loadCatalog(catalog, i18n.locale)
}

const aliases: Record<string, Locale> = {
  zh: "zh-Hans",
  "zh-cn": "zh-Hans",
  "zh-sg": "zh-Hans",
  "zh-tw": "zh-Hant",
  "zh-hk": "zh-Hant",
  "zh-mo": "zh-Hant",
  pt: "pt-BR",
  "es-419": "es-MX",
  no: "nb",
  nn: "nb",
  iw: "he",
  in: "id",
  fil: "en",
}

export function matchLocale(tags: readonly string[]): Locale {
  for (const tag of tags) {
    const lower = tag.toLowerCase()
    const exact = localeCodes.find((c) => c.toLowerCase() === lower)
    if (exact) return exact
    const alias = aliases[lower]
    if (alias) return alias
    const base = lower.split("-")[0]!
    const byBase = localeCodes.find((c) => c.toLowerCase() === base) ?? aliases[base]
    if (byBase) return byBase
    const family = localeCodes.find((c) => c.toLowerCase().split("-")[0] === base)
    if (family) return family
  }
  return "en"
}

function localeFromAddress(): Locale | null {
  try {
    const url = new URL(window.location.href)
    const tag = url.searchParams.get("lang")?.trim()
    if (tag === undefined || tag === null) return null
    url.searchParams.delete("lang")
    window.history.replaceState(window.history.state, "", url.pathname + url.search + url.hash)
    if (!tag) return null
    const locale = matchLocale([tag])
    return locale !== "en" || tag.toLowerCase().startsWith("en") ? locale : null
  } catch {
    return null
  }
}

export function initialLocale(): Locale {
  const fromAddress = localeFromAddress()
  if (fromAddress) return fromAddress
  try {
    const saved = localStorage.getItem("locale")
    if (isLocale(saved)) return saved
  } catch {
    // Storage can be blocked; fall back to the browser language.
  }
  return matchLocale(navigator.languages?.length ? navigator.languages : [navigator.language])
}

let requested = ""

export async function activate(tag: string) {
  const locale = isLocale(tag) ? tag : matchLocale([tag])
  requested = locale
  await loadAll(locale)
  if (requested !== locale) return
  i18n.activate(locale)
  const root = document.documentElement
  root.lang = locale
  root.dir = rtlLocales.includes(locale) ? "rtl" : "ltr"
  try {
    localStorage.setItem("locale", locale)
  } catch {
    // Storage can be blocked; the choice then lasts for this page only.
  }
}

export { i18n }

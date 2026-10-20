import { i18n, type Messages } from "@lingui/core"

import { localeCodes, rtlLocales } from "@/localeList"

export type Locale = (typeof localeCodes)[number]
export type Catalog = "messages" | "analytics" | "designs" | "help"

type Loader = () => Promise<{ messages: Messages }>

const loaders: Record<Catalog, Record<string, Loader>> = {
  messages: import.meta.glob<{ messages: Messages }>("./locales/*/messages.po"),
  analytics: import.meta.glob<{ messages: Messages }>("./locales/*/analytics.po"),
  designs: import.meta.glob<{ messages: Messages }>("./locales/*/designs.po"),
  help: import.meta.glob<{ messages: Messages }>("./locales/*/help.po"),
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

let requested = ""
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

async function loadAll(locale: string) {
  let size = 0
  while (size !== wanted.size) {
    size = wanted.size
    await Promise.all([...wanted].map((c) => loadCatalog(c, locale)))
  }
}

export async function needCatalog(catalog: Catalog) {
  wanted.add(catalog)
  const locale = requested || i18n.locale
  if (locale) await loadCatalog(catalog, locale)
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

const cookieName = "yuva_locale"

function localeFromCookie(): Locale | null {
  try {
    for (const part of document.cookie.split(";")) {
      const [name, ...rest] = part.trim().split("=")
      if (name !== cookieName) continue
      const value = decodeURIComponent(rest.join("="))
      if (isLocale(value)) return value
    }
  } catch {
    // Cookies can be blocked.
  }
  return null
}

let cookieReady = false
let cookieDomain = ""

function writeCookie(locale: string) {
  const base = `${cookieName}=${encodeURIComponent(locale)}; Path=/; SameSite=Lax`
  const secure = window.location.protocol === "https:" ? "; Secure" : ""
  try {
    if (cookieDomain) {
      document.cookie = `${cookieName}=; Path=/; Max-Age=0; SameSite=Lax${secure}`
      document.cookie = `${base}; Domain=${cookieDomain}; Max-Age=31536000${secure}`
    } else {
      document.cookie = `${base}; Max-Age=31536000${secure}`
    }
  } catch {
    // Cookies can be blocked; the choice then stays in localStorage.
  }
}

export function setLocaleCookieDomain(domain: string | undefined) {
  cookieDomain = domain?.trim().replace(/^\./, "") ?? ""
  cookieReady = true
  if (i18n.locale) writeCookie(i18n.locale)
}

export function initialLocale(): Locale {
  const fromAddress = localeFromAddress()
  if (fromAddress) return fromAddress
  const fromCookie = localeFromCookie()
  if (fromCookie) return fromCookie
  try {
    const saved = localStorage.getItem("locale")
    if (isLocale(saved)) return saved
  } catch {
    // Storage can be blocked; fall back to the browser language.
  }
  return matchLocale(navigator.languages?.length ? navigator.languages : [navigator.language])
}

export async function activate(tag: string) {
  const locale = isLocale(tag) ? tag : matchLocale([tag])
  requested = locale
  await loadAll(locale)
  if (requested !== locale) return
  i18n.activate(locale)
  const root = document.documentElement
  root.lang = locale
  root.dir = rtlLocales.includes(locale) ? "rtl" : "ltr"
  if (cookieReady) writeCookie(locale)
  try {
    localStorage.setItem("locale", locale)
  } catch {
    // Storage can be blocked; the choice then lasts for this page only.
  }
}

export { i18n }

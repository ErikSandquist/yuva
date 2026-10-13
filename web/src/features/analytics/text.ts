import { useLingui } from "@lingui/react/macro"
import { useParams, useSearchParams } from "react-router"

import { useProperty } from "@/features/analytics/queries"
import { parseFilters, type Period, type FilterOp } from "@/features/analytics/state"
import type {
  AnalyticsDimension,
  AnalyticsFilter,
  AnalyticsFilterDimension,
  AnalyticsMetric,
  AnalyticsMetrics,
  AnalyticsProperty,
} from "@/lib/api"

export const OTHER = "(other)"

export type Wording = "pages" | "screens" | "both"

const APP_PLATFORMS = ["ios", "android"]

export function hasApps(p: Pick<AnalyticsProperty, "platforms">) {
  return p.platforms.some((x) => APP_PLATFORMS.includes(x))
}

export function wordingFor(p: Pick<AnalyticsProperty, "platforms">, filters: AnalyticsFilter[] = []): Wording {
  const platform = filters.find((f) => f.dimension === "platform" && f.op === "is")?.values[0]
  if (platform) return APP_PLATFORMS.includes(platform) ? "screens" : "pages"
  if (!hasApps(p)) return "pages"
  return p.platforms.includes("web") ? "both" : "screens"
}

export function useWording(): Wording {
  const { propertyId = "" } = useParams()
  const [params] = useSearchParams()
  const property = useProperty(propertyId).data
  return wordingFor(property ?? { platforms: ["web"] }, parseFilters(params.getAll("f")))
}

export function useAnalyticsText() {
  const { t } = useLingui()
  const wording = useWording()
  const words = {
    pages: { page: t`Page`, entry: t`Entry page`, exit: t`Exit page`, host: t`Host`, views: t`Pageviews` },
    screens: { page: t`Screen`, entry: t`First screen`, exit: t`Last screen`, host: t`App`, views: t`Screen views` },
    both: {
      page: t`Page or screen`,
      entry: t`First page or screen`,
      exit: t`Last page or screen`,
      host: t`Host or app`,
      views: t`Views`,
    },
  }[wording]
  return {
    wording,
    title: {
      pages: { pages: t`Pages`, screens: t`Screens`, both: t`Pages and screens` }[wording],
      top: { pages: t`Top pages`, screens: t`Top screens`, both: t`Top pages and screens` }[wording],
      perMinute: {
        pages: t`Pageviews per minute`,
        screens: t`Screen views per minute`,
        both: t`Views per minute`,
      }[wording],
      lastHalfHour: {
        pages: t`Pageviews in the last 30 minutes`,
        screens: t`Screen views in the last 30 minutes`,
        both: t`Views in the last 30 minutes`,
      }[wording],
    },
    metric: {
      visitors: t`Visitors`,
      visits: t`Visits`,
      pageviews: words.views,
      views_per_visit: t`Views per visit`,
      bounce_rate: t`Bounce rate`,
      visit_duration: t`Visit duration`,
      time_on_page: t`Time on page`,
      scroll_depth: t`Scroll depth`,
      events: t`Events`,
      conversions: t`Conversions`,
      conversion_rate: t`Conversion rate`,
      completions: t`Completions`,
      revenue: t`Revenue`,
    } satisfies Record<AnalyticsMetric, string>,
    metricHelp: {
      visitors: t`Distinct visitors per day, added up over the days of the range. Someone who comes back on another day counts again.`,
      visits: {
        pages: t`A visitor's page views without a gap of 30 minutes, within one day.`,
        screens: t`A visitor's screen views without a gap of 30 minutes, within one day. Each launch of the app starts a new visit.`,
        both: t`A visitor's page or screen views without a gap of 30 minutes, within one day.`,
      }[wording],
      pageviews: { pages: t`Pages viewed.`, screens: t`Screens viewed.`, both: t`Pages and screens viewed.` }[wording],
      views_per_visit: {
        pages: t`Pageviews divided by visits.`,
        screens: t`Screen views divided by visits.`,
        both: t`Views divided by visits.`,
      }[wording],
      bounce_rate: {
        pages: t`Visits with one page view and nothing else.`,
        screens: t`Visits with one screen view and nothing else.`,
        both: t`Visits with one page or screen view and nothing else.`,
      }[wording],
      visit_duration: t`Time from the first to the last event of a visit, on average. Bounces count as zero.`,
      time_on_page: t`Time the page was visible and in focus, per view.`,
      scroll_depth: t`The deepest point reached on the page, per view.`,
      events: t`Custom and built-in events.`,
      conversions: t`Visitors who reached the goal, per day, added up over the days of the range.`,
      conversion_rate: t`Conversions per 100 visitors.`,
      completions: t`Times the goal was reached, also several times by one visitor.`,
      revenue: t`What the goal's events carried, per currency. Currencies are never converted or added up.`,
    } satisfies Record<AnalyticsMetric, string>,
    dimension: {
      page: words.page,
      entry_page: words.entry,
      exit_page: words.exit,
      hostname: words.host,
      channel: t`Channel`,
      source: t`Source`,
      referrer: t`Referrer`,
      utm_source: t`UTM source`,
      utm_medium: t`UTM medium`,
      utm_campaign: t`UTM campaign`,
      utm_term: t`UTM term`,
      utm_content: t`UTM content`,
      country: t`Country`,
      region: t`Region`,
      city: t`City`,
      device: t`Device`,
      browser: t`Browser`,
      browser_version: t`Browser version`,
      os: t`Operating system`,
      os_version: t`OS version`,
      screen_size: t`Screen size`,
      language: t`Language`,
      platform: t`Platform`,
      app_version: t`App version`,
      event: t`Event`,
      prop: t`Prop`,
    } satisfies Record<AnalyticsFilterDimension, string>,
    event: {
      outbound_click: t`Outbound link`,
      file_download: t`File download`,
      not_found: t`Page not found`,
      chat_open: t`Chat opened`,
    } as Record<string, string>,
    op: {
      is: t`is`,
      is_not: t`is not`,
      contains: t`contains`,
      not_contains: t`does not contain`,
      matches: t`matches`,
    } satisfies Record<FilterOp, string>,
    period: {
      today: t`Today`,
      yesterday: t`Yesterday`,
      "7d": t`Last 7 days`,
      "28d": t`Last 28 days`,
      "30d": t`Last 30 days`,
      "90d": t`Last 90 days`,
      month: t`This month`,
      last_month: t`Last month`,
      "6mo": t`Last 6 months`,
      "12mo": t`Last 12 months`,
      year: t`This year`,
    } satisfies Record<Period, string>,
    channel: {
      direct: t`Direct`,
      organic_search: t`Organic search`,
      paid_search: t`Paid search`,
      social: t`Social`,
      paid_social: t`Paid social`,
      email: t`E-mail`,
      referral: t`Referral`,
      ai_assistants: t`AI assistants`,
      internal: t`Internal`,
    } as Record<string, string>,
    device: { desktop: t`Desktop`, mobile: t`Mobile`, tablet: t`Tablet` } as Record<string, string>,
    screen: { phone: t`Phone`, tablet: t`Tablet`, laptop: t`Laptop`, desktop: t`Desktop` } as Record<string, string>,
    platform: { web: t`Web`, ios: "iOS", android: "Android", server: t`Server` } as Record<string, string>,
    unknown: t`Unknown`,
    notSet: t`(not set)`,
    direct: t`Direct / none`,
    other: t`Other`,
  }
}

export type AnalyticsText = ReturnType<typeof useAnalyticsText>

export function flag(code: string) {
  if (!/^[A-Z]{2}$/.test(code)) return ""
  return String.fromCodePoint(...[...code].map((c) => 0x1f1e6 + c.charCodeAt(0) - 65))
}

export function countryName(code: string, locale: string) {
  try {
    return new Intl.DisplayNames([locale], { type: "region" }).of(code) ?? code
  } catch {
    return code
  }
}

function languageName(tag: string, locale: string) {
  try {
    return new Intl.DisplayNames([locale], { type: "language" }).of(tag) ?? tag
  } catch {
    return tag
  }
}

export function valueText(text: AnalyticsText, dim: AnalyticsDimension, value: string, locale: string) {
  if (value === OTHER) return text.other
  if (value === "") {
    if (dim === "prop") return text.notSet
    return ["source", "referrer", "utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content"].includes(dim)
      ? text.direct
      : text.unknown
  }
  switch (dim) {
    case "channel":
      return text.channel[value] ?? value
    case "device":
      return text.device[value] ?? value
    case "screen_size":
      return text.screen[value] ?? value
    case "platform":
      return text.platform[value] ?? value
    case "country":
      return countryName(value, locale)
    case "language":
      return languageName(value, locale)
    case "event":
      return text.event[value] ?? value
    default:
      return value
  }
}

export function isPathDimension(dim: AnalyticsDimension) {
  return dim === "page" || dim === "entry_page" || dim === "exit_page" || dim === "referrer" || dim === "hostname"
}

function seconds(v: number, locale: string) {
  const s = Math.round(v)
  const unit = (n: number, u: "second" | "minute" | "hour") =>
    new Intl.NumberFormat(locale, { style: "unit", unit: u, unitDisplay: "short" }).format(n)
  if (s < 60) return unit(s, "second")
  if (s < 3600) return `${unit(Math.floor(s / 60), "minute")} ${unit(s % 60, "second")}`
  return `${unit(Math.floor(s / 3600), "hour")} ${unit(Math.floor((s % 3600) / 60), "minute")}`
}

export type NumMetric = Exclude<AnalyticsMetric, "revenue">

export function num(m: AnalyticsMetrics | undefined, k: NumMetric) {
  return m?.[k]
}

export function formatMoney(minor: number, currency: string, digits: number, locale: string) {
  const amount = minor / 10 ** digits
  try {
    return new Intl.NumberFormat(locale, {
      style: "currency",
      currency,
      minimumFractionDigits: digits,
      maximumFractionDigits: digits,
    }).format(amount)
  } catch {
    return `${new Intl.NumberFormat(locale, { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(amount)} ${currency}`
  }
}

export function formatMetric(metric: AnalyticsMetric, v: number | undefined, locale: string) {
  if (v === undefined || Number.isNaN(v)) return "–"
  switch (metric) {
    case "conversion_rate":
      return new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 1 }).format(v / 100)
    case "bounce_rate":
    case "scroll_depth":
      return new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 0 }).format(v / 100)
    case "visit_duration":
    case "time_on_page":
      return seconds(v, locale)
    case "views_per_visit":
      return new Intl.NumberFormat(locale, { maximumFractionDigits: 2 }).format(v)
    default:
      return new Intl.NumberFormat(locale, { maximumFractionDigits: 0 }).format(v)
  }
}

export function formatCompact(v: number, locale: string) {
  return new Intl.NumberFormat(locale, {
    notation: v >= 10_000 ? "compact" : "standard",
    maximumFractionDigits: 1,
  }).format(v)
}

export function lowerIsBetter(metric: AnalyticsMetric) {
  return metric === "bounce_rate"
}

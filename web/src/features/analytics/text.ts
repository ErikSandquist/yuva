import { useLingui } from "@lingui/react/macro"

import type { Period, FilterOp } from "@/features/analytics/state"
import type {
  AnalyticsDimension,
  AnalyticsFilterDimension,
  AnalyticsMetric,
  AnalyticsMetrics,
  AnalyticsRevenue,
} from "@/lib/api"

export const OTHER = "(other)"

export function useAnalyticsText() {
  const { t } = useLingui()
  return {
    metric: {
      visitors: t`Visitors`,
      visits: t`Visits`,
      pageviews: t`Pageviews`,
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
      visits: t`A visitor's page views without a gap of 30 minutes, within one day.`,
      pageviews: t`Pages viewed.`,
      views_per_visit: t`Pageviews divided by visits.`,
      bounce_rate: t`Visits with one page view and nothing else.`,
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
      page: t`Page`,
      entry_page: t`Entry page`,
      exit_page: t`Exit page`,
      hostname: t`Host`,
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
    new Intl.NumberFormat(locale, { style: "unit", unit: u, unitDisplay: "narrow" }).format(n)
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

export function formatRevenue(r: AnalyticsRevenue[] | undefined, locale: string) {
  if (!r || r.length === 0) return "–"
  return r.map((x) => formatMoney(x.total, x.currency, x.digits, locale)).join(" · ")
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

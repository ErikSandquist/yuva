import { useLingui } from "@lingui/react/macro"

import type { MonitorAlertRuleKind, MonitorCheckError, MonitorCheckResult, MonitorCheckState } from "@/lib/api"

export const RULE_KINDS: MonitorAlertRuleKind[] = [
  "error_rate",
  "client_error_rate",
  "p95",
  "p99",
  "apdex",
  "spike",
  "drop",
  "silent",
  "check_state",
  "certificate",
]

export const CHECK_KINDS: MonitorAlertRuleKind[] = ["check_state", "certificate"]

export function isCheckKind(k: MonitorAlertRuleKind) {
  return CHECK_KINDS.includes(k)
}

export function hasThreshold(k: MonitorAlertRuleKind) {
  return !["spike", "drop", "silent", "check_state"].includes(k)
}

export function hasWindow(k: MonitorAlertRuleKind) {
  return ["error_rate", "client_error_rate", "p95", "p99", "apdex"].includes(k)
}

export function useMonitoringText() {
  const { t } = useLingui()
  return {
    state: {
      unknown: t`Not checked yet`,
      up: t`Up`,
      down: t`Down`,
    } satisfies Record<MonitorCheckState, string>,
    runState: {
      up: t`Up`,
      down: t`Down`,
      unknown: t`Not counted`,
      pending: t`Waiting`,
    } satisfies Record<MonitorCheckResult["state"], string>,
    error: {
      dns: t`The name did not resolve`,
      refused_address: t`It led to an address the server will not call`,
      connect: t`Could not connect`,
      tls: t`The TLS handshake or certificate failed`,
      timeout: t`No answer in time`,
      status: t`An unexpected status`,
      keyword: t`The keyword check failed`,
      redirects: t`Too many redirects, or a redirect that is not allowed`,
      other: t`Something else went wrong`,
    } satisfies Record<MonitorCheckError, string>,
    kind: {
      error_rate: t`Error rate`,
      client_error_rate: t`Client error rate`,
      p95: t`p95 latency`,
      p99: t`p99 latency`,
      apdex: t`Apdex`,
      spike: t`Traffic spike`,
      drop: t`Traffic drop`,
      silent: t`Route went silent`,
      check_state: t`Check down`,
      certificate: t`Certificate expiring`,
    } satisfies Record<MonitorAlertRuleKind, string>,
    kindHelp: {
      error_rate: t`Requests with status 500 to 599, or no answer, above a share of all requests.`,
      client_error_rate: t`Requests with status 400 to 499 above a share of all requests.`,
      p95: t`95 out of 100 requests took longer than the threshold.`,
      p99: t`99 out of 100 requests took longer than the threshold.`,
      apdex: t`The Apdex score fell below the threshold.`,
      spike: t`The last 15 minutes had at least three times the requests of the same 15 minutes over the week before.`,
      drop: t`The last 15 minutes had at most half the requests of the same 15 minutes over the week before.`,
      silent: t`A route that got a request a minute on average over the day before got none for 15 minutes.`,
      check_state: t`An uptime check is down.`,
      certificate: t`An uptime check’s certificate expires within the threshold.`,
    } satisfies Record<MonitorAlertRuleKind, string>,
    unit: {
      error_rate: "%",
      client_error_rate: "%",
      p95: t`ms`,
      p99: t`ms`,
      apdex: "0–1",
      spike: "",
      drop: "",
      silent: "",
      check_state: "",
      certificate: t`days`,
    } satisfies Record<MonitorAlertRuleKind, string>,
  }
}

export type MonitoringText = ReturnType<typeof useMonitoringText>

export function formatMs(v: number | undefined, locale: string) {
  if (v === undefined || Number.isNaN(v)) return "–"
  const unit = (n: number, u: "millisecond" | "second", digits: number) =>
    new Intl.NumberFormat(locale, {
      style: "unit",
      unit: u,
      unitDisplay: "short",
      maximumFractionDigits: digits,
    }).format(n)
  if (v < 10) return unit(v, "millisecond", 1)
  if (v < 1000) return unit(v, "millisecond", 0)
  return unit(v / 1000, "second", 2)
}

export function formatInterval(seconds: number, locale: string) {
  const unit = (n: number, u: "second" | "minute" | "hour") =>
    new Intl.NumberFormat(locale, { style: "unit", unit: u, unitDisplay: "long" }).format(n)
  if (seconds % 3600 === 0) return unit(seconds / 3600, "hour")
  if (seconds % 60 === 0) return unit(seconds / 60, "minute")
  return unit(seconds, "second")
}

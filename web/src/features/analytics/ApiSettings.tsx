import { Trans, useLingui } from "@lingui/react/macro"
import { ClockIcon, EyeIcon } from "lucide-react"
import { useState } from "react"
import { useSearchParams } from "react-router"

import { ErrorLine, Notice, Segmented } from "@/components/common"
import { formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { ApiInstallSnippets } from "@/features/analytics/Install"
import { InferredTag } from "@/features/analytics/Monitor"
import { hostOnly, splitLines, useZones } from "@/features/analytics/NewProperty"
import { useCurrentProperty } from "@/features/analytics/parts"
import { useApiRoutes, useSetRouteHidden, useUpdateProperty } from "@/features/analytics/queries"
import { DeletePropertySection, RuleRows } from "@/features/analytics/Settings"
import { useAnalyticsText, valueText } from "@/features/analytics/text"
import { ChoiceSelect, Field, FormActions, FormCard, Section } from "@/features/settings/ui"
import type { AnalyticsPathRule, AnalyticsProperty } from "@/lib/api"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

type Tab = "general" | "routes" | "install" | "data"

const VIEW_PARAMS = ["period"]

const NO_INBOX = "none"

function General({ p }: { p: AnalyticsProperty }) {
  const { t, i18n } = useLingui()
  const inboxes = useInboxes().data ?? []
  const zones = useZones()
  const update = useUpdateProperty(p.id)
  const [name, setName] = useState(p.name)
  const [domains, setDomains] = useState(p.domains.join("\n"))
  const [zone, setZone] = useState(p.timezone)
  const [inbox, setInbox] = useState(p.inbox_id ?? NO_INBOX)
  const [apdex, setApdex] = useState(String(p.apdex_ms))
  const [hourDays, setHourDays] = useState(String(p.monitor_hour_days))
  const hosts = splitLines(domains).map(hostOnly).filter(Boolean)
  const apdexMs = Number(apdex)
  const days = Number(hourDays)
  const apdexOk = Number.isInteger(apdexMs) && apdexMs >= 1 && apdexMs <= 60_000
  const daysOk = Number.isInteger(days) && days >= 7 && days <= 400
  const applies = p.timezone_applies_at ? formatDateTime(p.timezone_applies_at, i18n.locale) : ""
  const current = p.timezone
  return (
    <FormCard
      onSubmit={() =>
        update.mutate({
          name: name.trim(),
          domains: hosts,
          timezone: zone,
          inbox_id: inbox === NO_INBOX ? null : inbox,
          apdex_ms: apdexMs,
          monitor_hour_days: days,
        })
      }
      footer={
        <FormActions
          pending={update.isPending}
          saved={update.isSuccess}
          error={update.error}
          disabled={!name.trim() || !zones.includes(zone) || !apdexOk || !daysOk}
        />
      }
    >
      <Field label={<Trans>Name</Trans>} htmlFor="ap-name">
        <Input id="ap-name" value={name} maxLength={100} onChange={(e) => setName(e.target.value)} />
      </Field>
      <Field
        label={<Trans>Host names</Trans>}
        htmlFor="ap-domains"
        hint={<Trans>Optional, one per line. The host names the API answers on, to tell properties apart.</Trans>}
      >
        <Textarea
          id="ap-domains"
          dir="ltr"
          rows={2}
          value={domains}
          placeholder="api.example.com"
          onChange={(e) => setDomains(e.target.value)}
        />
      </Field>
      <div className="grid gap-5 sm:grid-cols-2">
        <Field
          label={<Trans>Time zone</Trans>}
          htmlFor="ap-zone"
          hint={<Trans>A change applies from the next midnight, so no day is cut in two.</Trans>}
        >
          <Input id="ap-zone" list="ap-zones" value={zone} onChange={(e) => setZone(e.target.value)} />
          <datalist id="ap-zones">
            {zones.map((z) => (
              <option key={z} value={z} />
            ))}
          </datalist>
        </Field>
        <Field
          label={<Trans>Inbox</Trans>}
          htmlFor="ap-inbox"
          hint={<Trans>Agents of this inbox can see the numbers.</Trans>}
        >
          <ChoiceSelect
            id="ap-inbox"
            label={t`Inbox`}
            value={inbox}
            onChange={setInbox}
            options={[[NO_INBOX, t`None, owners and admins only`], ...inboxes.map((i) => [i.id, i.name] as const)]}
            className="w-full"
          />
        </Field>
      </div>
      {applies && (
        <Notice icon={ClockIcon} data-testid="api-zone-pending">
          <Trans>
            {current} applies from {applies}. Until then hours and days are still counted in the time zone before.
          </Trans>
        </Notice>
      )}
      <div className="grid gap-5 sm:grid-cols-2">
        <Field
          label={<Trans>Apdex threshold</Trans>}
          htmlFor="ap-apdex"
          hint={<Trans>In milliseconds. A request is satisfied within it and tolerated within four times it.</Trans>}
          error={apdexOk ? undefined : <Trans>From 1 to 60,000.</Trans>}
        >
          <Input
            id="ap-apdex"
            type="number"
            inputMode="numeric"
            min={1}
            max={60000}
            value={apdex}
            onChange={(e) => setApdex(e.target.value)}
            className="sm:max-w-40"
          />
        </Field>
        <Field
          label={<Trans>Hourly detail</Trans>}
          htmlFor="ap-hour-days"
          hint={<Trans>Days the counts per hour are kept. Daily counts are kept as long as the property’s data.</Trans>}
          error={daysOk ? undefined : <Trans>From 7 to 400 days.</Trans>}
        >
          <Input
            id="ap-hour-days"
            type="number"
            inputMode="numeric"
            min={7}
            max={400}
            value={hourDays}
            onChange={(e) => setHourDays(e.target.value)}
            className="sm:max-w-40"
          />
        </Field>
      </div>
    </FormCard>
  )
}

function PathRules({ p }: { p: AnalyticsProperty }) {
  const update = useUpdateProperty(p.id)
  const [rules, setRules] = useState<AnalyticsPathRule[]>(p.path_rules)
  const complete = rules.filter((r) => r.pattern.trim() && r.replacement.trim())
  return (
    <Section
      title={<Trans>Path rules</Trans>}
      description={
        <Trans>
          Paths or routes that match a pattern are counted under the route you name, from now on. Use them when your
          server sends paths and a pattern made from them is not the one you want.
        </Trans>
      }
    >
      <FormCard
        onSubmit={() =>
          update.mutate({
            path_rules: complete.map((r) => ({ pattern: r.pattern.trim(), replacement: r.replacement.trim() })),
          })
        }
        footer={<FormActions pending={update.isPending} saved={update.isSuccess} error={update.error} />}
      >
        <RuleRows rules={rules} onChange={setRules} />
      </FormCard>
    </Section>
  )
}

function HiddenRoutes({ p }: { p: AnalyticsProperty }) {
  const { i18n } = useLingui()
  const text = useAnalyticsText()
  const { canManage } = useSession()
  const routes = useApiRoutes(p.id)
  const show = useSetRouteHidden(p.id)
  const hidden = (routes.data ?? []).filter((r) => r.hidden)
  return (
    <Section
      title={<Trans>Hidden routes</Trans>}
      description={
        <Trans>
          Hidden routes are left out of every number and list, such as a health check your load balancer calls. Their
          requests are still counted, so showing one again brings them back.
        </Trans>
      }
    >
      <div className="overflow-hidden rounded-2xl border bg-card" data-testid="api-hidden-routes">
        {routes.isPending ? (
          <Skeleton className="m-4 h-12" />
        ) : routes.error ? (
          <ErrorLine error={routes.error} className="p-4" />
        ) : hidden.length === 0 ? (
          <p className="px-4 py-6 text-body text-faint">
            <Trans>No hidden routes. Hide one from the Routes view.</Trans>
          </p>
        ) : (
          <ul className="divide-y">
            {hidden.map((r) => {
              const seen = formatDateTime(r.last_seen_at, i18n.locale)
              return (
                <li key={r.id} className="flex items-center gap-3 px-4 py-2.5" data-testid="api-hidden-route">
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="flex min-w-0 items-center gap-2">
                      <span className="min-w-0 truncate font-mono text-small" dir="ltr">
                        {valueText(text, "route", r.pattern, i18n.locale)}
                      </span>
                      {r.inferred && <InferredTag />}
                    </span>
                    <span className="text-caption text-faint">
                      <Trans>Last request {seen}</Trans>
                    </span>
                  </span>
                  {canManage && (
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={show.isPending}
                      onClick={() => show.mutate({ routeId: r.id, hidden: false })}
                      data-testid="api-show-route"
                    >
                      <EyeIcon />
                      <Trans>Show</Trans>
                    </Button>
                  )}
                </li>
              )
            })}
          </ul>
        )}
      </div>
      <ErrorLine error={show.error} />
    </Section>
  )
}

export function ApiSettingsView() {
  const { t } = useLingui()
  const { canManage } = useSession()
  const p = useCurrentProperty()
  const [params, setParams] = useSearchParams()
  const tabs: Tab[] = canManage ? ["general", "routes", "install", "data"] : ["install", "routes"]
  const raw = params.get("tab") as Tab | null
  const tab = raw && tabs.includes(raw) ? raw : tabs[0]!
  const names: Record<Tab, string> = {
    general: t`General`,
    routes: t`Routes`,
    install: t`Install`,
    data: t`Data`,
  }
  return (
    <div className="flex flex-col gap-5" data-testid="analytics-settings">
      <div className="-mx-1 max-w-[calc(100%+0.5rem)] overflow-x-auto px-1 py-1">
        <Segmented
          value={tab}
          onChange={(v) =>
            setParams(
              (prev) => {
                const next = new URLSearchParams()
                for (const k of VIEW_PARAMS) for (const x of prev.getAll(k)) next.append(k, x)
                if (v !== tabs[0]) next.set("tab", v)
                return next
              },
              { replace: true }
            )
          }
          label={t`Settings`}
          items={tabs.map((v) => ({ value: v, label: names[v], testId: `settings-tab-${v}` }))}
        />
      </div>
      {tab === "general" && <General p={p} />}
      {tab === "routes" && (
        <div className="flex flex-col gap-6">
          <HiddenRoutes p={p} />
          {canManage && <PathRules p={p} />}
        </div>
      )}
      {tab === "install" && (
        <div className="flex flex-col gap-5 rounded-2xl border bg-card p-4">
          <ApiInstallSnippets property={p} />
        </div>
      )}
      {tab === "data" && <DeletePropertySection p={p} />}
    </div>
  )
}

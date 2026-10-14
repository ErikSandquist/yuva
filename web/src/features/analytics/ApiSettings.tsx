import { Trans, useLingui } from "@lingui/react/macro"
import { ClockIcon, EraserIcon, EyeIcon } from "lucide-react"
import { useState } from "react"
import { useSearchParams } from "react-router"

import { ErrorLine, Notice, ScrollRow, Segmented, useConfirm } from "@/components/common"
import { formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { ApiInstallSnippets } from "@/features/analytics/Install"
import { InferredTag } from "@/features/analytics/MonitorParts"
import { hostOnly, splitLines, useZones } from "@/features/analytics/NewProperty"
import { useCurrentProperty } from "@/features/analytics/parts"
import {
  useApiRoutes,
  useConsumers,
  useEraseConsumer,
  useUpdateProperty,
  useUpdateRoute,
} from "@/features/analytics/queries"
import { NotifyMe } from "@/features/analytics/Alerts"
import { Exports } from "@/features/analytics/Exports"
import { DeletePropertySection, RuleRows } from "@/features/analytics/Settings"
import { formatInZone, Sharing } from "@/features/analytics/Sharing"
import { useAnalyticsText, valueText } from "@/features/analytics/text"
import { Card, ChoiceSelect, EmptyRow, Field, FormActions, FormCard, Section } from "@/features/settings/ui"
import type { AnalyticsPathRule, AnalyticsPlatform, AnalyticsProperty } from "@/lib/api"
import { useDebounced } from "@/hooks/use-debounced"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

type Tab = "general" | "routes" | "reports" | "sharing" | "data" | "install"

const VIEW_PARAMS = ["period"]

const NO_INBOX = "none"

const APPS: AnalyticsPlatform[] = ["ios", "android"]

function General({ p }: { p: AnalyticsProperty }) {
  const { t, i18n } = useLingui()
  const text = useAnalyticsText()
  const inboxes = useInboxes().data ?? []
  const zones = useZones()
  const update = useUpdateProperty(p.id)
  const [name, setName] = useState(p.name)
  const [domains, setDomains] = useState(p.domains.join("\n"))
  const [zone, setZone] = useState(p.timezone)
  const [inbox, setInbox] = useState(p.inbox_id ?? NO_INBOX)
  const [apdex, setApdex] = useState(String(p.apdex_ms))
  const [platforms, setPlatforms] = useState<AnalyticsPlatform[]>(p.platforms.filter((x) => APPS.includes(x)))
  const hosts = splitLines(domains).map(hostOnly).filter(Boolean)
  const apdexMs = Number(apdex)
  const apdexOk = Number.isInteger(apdexMs) && apdexMs >= 1 && apdexMs <= 60_000
  const applies = p.timezone_applies_at ? formatInZone(p.timezone_applies_at, i18n.locale, p.timezone) : ""
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
          platforms: APPS.filter((x) => platforms.includes(x)),
        })
      }
      footer={
        <FormActions
          pending={update.isPending}
          saved={update.isSuccess}
          error={update.error}
          disabled={!name.trim() || !zones.includes(zone) || !apdexOk}
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
      <div className="flex min-w-0 flex-col gap-2">
        <span className="text-body font-medium" id="ap-platforms">
          <Trans>Apps</Trans>
        </span>
        <div className="flex flex-wrap gap-x-5 gap-y-2" role="group" aria-labelledby="ap-platforms">
          {APPS.map((x) => (
            <Label key={x} className="font-normal" data-testid={`ap-platform-${x}`}>
              <Checkbox
                checked={platforms.includes(x)}
                onCheckedChange={(on) => setPlatforms(on ? [...platforms, x] : platforms.filter((y) => y !== x))}
              />
              {text.platform[x]}
            </Label>
          ))}
        </div>
        <p className="text-small text-faint">
          <Trans>
            The apps allowed to time their own calls to this API with the Swift or Kotlin library. Records from your
            servers need neither.
          </Trans>
        </p>
      </div>
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
      </div>
    </FormCard>
  )
}

const KEEP = ["1", "3", "6", "12", "24", "36", "60", "120"] as const
const FOREVER = "forever"

function Retention({ p }: { p: AnalyticsProperty }) {
  const { t, i18n } = useLingui()
  const { canManage } = useSession()
  const update = useUpdateProperty(p.id)
  const [hourDays, setHourDays] = useState(String(p.monitor_hour_days))
  const [months, setMonths] = useState(p.retention_months ? String(p.retention_months) : FOREVER)
  const days = Number(hourDays)
  const daysOk = Number.isInteger(days) && days >= 7 && days <= 400
  const monthName = (m: number) =>
    new Intl.NumberFormat(i18n.locale, { style: "unit", unit: "month", unitDisplay: "long" }).format(m)
  const options = [
    ...KEEP.map((m) => [m, monthName(Number(m))] as const),
    ...(months !== FOREVER && !(KEEP as readonly string[]).includes(months)
      ? [[months, monthName(Number(months))] as const]
      : []),
    [FOREVER, t`Forever`] as const,
  ]
  return (
    <Section
      title={<Trans>Retention</Trans>}
      description={
        <Trans>
          Counts per minute are kept 8 days on every server, and older rows of each kind are deleted. Nothing else about
          a request is ever stored.
        </Trans>
      }
    >
      <FormCard
        onSubmit={() => {
          if (canManage && daysOk)
            update.mutate({ monitor_hour_days: days, retention_months: months === FOREVER ? null : Number(months) })
        }}
        footer={
          canManage ? (
            <FormActions pending={update.isPending} saved={update.isSuccess} error={update.error} disabled={!daysOk} />
          ) : undefined
        }
      >
        <div className="grid gap-5 sm:grid-cols-2">
          <Field
            label={<Trans>Counts per hour, in days</Trans>}
            htmlFor="ap-hour-days"
            hint={<Trans>Hourly rows, with consumers, countries and app versions.</Trans>}
            error={daysOk ? undefined : <Trans>From 7 to 400 days.</Trans>}
          >
            <Input
              id="ap-hour-days"
              type="number"
              inputMode="numeric"
              min={7}
              max={400}
              value={hourDays}
              disabled={!canManage}
              onChange={(e) => setHourDays(e.target.value)}
              className="sm:max-w-40"
              data-testid="ap-hour-days"
            />
          </Field>
          <Field
            label={<Trans>Counts per day</Trans>}
            htmlFor="ap-months"
            hint={<Trans>Daily rows, the totals of every older range.</Trans>}
          >
            <ChoiceSelect
              id="ap-months"
              label={t`Counts per day`}
              value={months}
              onChange={setMonths}
              options={options}
              disabled={!canManage}
              className="w-full"
            />
          </Field>
        </div>
      </FormCard>
    </Section>
  )
}

function Consumers({ p }: { p: AnalyticsProperty }) {
  const { t, i18n } = useLingui()
  const { canManage } = useSession()
  const [q, setQ] = useState("")
  const term = useDebounced(q.trim(), 300)
  const list = useConsumers(p.id, term)
  const erase = useEraseConsumer(p.id)
  const [ask, confirm] = useConfirm()
  const items = list.data?.pages.flatMap((x) => x.items) ?? []
  return (
    <Section
      title={<Trans>Consumers</Trans>}
      description={
        <Trans>
          The callers your server named. Erase one when someone asks for their data to be deleted: their rows by hour
          and by day go, and the name is never used again.
        </Trans>
      }
    >
      <Card flush>
        <div className="border-b p-3">
          <Input
            type="search"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder={t`Search consumers`}
            aria-label={t`Search consumers`}
            maxLength={100}
            data-testid="consumers-search"
          />
        </div>
        {list.isPending ? (
          <Skeleton className="m-4 h-10" />
        ) : list.error ? (
          <ErrorLine error={list.error} className="p-4" />
        ) : items.length === 0 ? (
          <EmptyRow>{term ? <Trans>No consumer matches.</Trans> : <Trans>No consumers counted yet.</Trans>}</EmptyRow>
        ) : (
          <ul className="divide-y" data-testid="consumers-list">
            {items.map((c) => {
              const seen = formatDateTime(c.first_seen_at, i18n.locale)
              return (
                <li key={c.id} className="flex items-center gap-3 px-4 py-2.5" data-testid="consumer-row">
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="truncate text-body" dir="auto">
                      {c.name}
                    </span>
                    <span className="text-caption text-faint">
                      <Trans>First seen {seen}</Trans>
                    </span>
                  </span>
                  {canManage && (
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={erase.isPending}
                      onClick={() =>
                        ask({
                          title: <EraseTitle name={c.name} />,
                          description: (
                            <Trans>
                              Its rows by hour and by day are deleted now and again a few minutes later. Totals stay the
                              same, since they never held the consumer. Export files made before keep the name until
                              they are deleted, at most 7 days.
                            </Trans>
                          ),
                          confirm: <Trans>Erase</Trans>,
                          run: () => erase.mutate(c.id),
                        })
                      }
                      data-testid="consumer-erase"
                    >
                      <EraserIcon />
                      <Trans>Erase</Trans>
                    </Button>
                  )}
                </li>
              )
            })}
          </ul>
        )}
        {list.hasNextPage && (
          <div className="flex justify-center border-t p-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void list.fetchNextPage()}
              disabled={list.isFetchingNextPage}
            >
              <Trans>Load more</Trans>
            </Button>
          </div>
        )}
      </Card>
      <ErrorLine error={erase.error} />
      {erase.isSuccess && (
        <p role="status" className="text-small text-muted-foreground">
          <Trans>Erased.</Trans>
        </p>
      )}
      {confirm}
    </Section>
  )
}

function EraseTitle({ name }: { name: string }) {
  return <Trans>Erase the consumer “{name}”?</Trans>
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
  const show = useUpdateRoute(p.id)
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
  const tabs: Tab[] = canManage
    ? ["general", "routes", "reports", "sharing", "data", "install"]
    : ["reports", "routes", "sharing", "data", "install"]
  const raw = params.get("tab") as Tab | null
  const tab = raw && tabs.includes(raw) ? raw : tabs[0]!
  const names: Record<Tab, string> = {
    general: t`General`,
    routes: t`Routes`,
    reports: t`Reports`,
    sharing: t`Sharing`,
    install: t`Install`,
    data: t`Data`,
  }
  return (
    <div className="flex flex-col gap-5" data-testid="analytics-settings">
      <ScrollRow>
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
      </ScrollRow>
      {tab === "general" && <General p={p} />}
      {tab === "routes" && (
        <div className="flex flex-col gap-6">
          <HiddenRoutes p={p} />
          {canManage && <PathRules p={p} />}
        </div>
      )}
      {tab === "reports" && <NotifyMe p={p} />}
      {tab === "install" && (
        <div className="flex flex-col gap-5 rounded-2xl border bg-card p-4">
          <ApiInstallSnippets property={p} />
        </div>
      )}
      {tab === "sharing" && <Sharing p={p} />}
      {tab === "data" && (
        <div className="flex flex-col gap-6">
          <Retention p={p} />
          <Exports p={p} />
          <Consumers p={p} />
          {canManage && <DeletePropertySection p={p} />}
        </div>
      )}
    </div>
  )
}

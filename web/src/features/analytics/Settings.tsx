import { Trans, useLingui } from "@lingui/react/macro"
import {
  CheckCircle2Icon,
  ClockIcon,
  KeyRoundIcon,
  PlusIcon,
  RadioIcon,
  RotateCcwIcon,
  Trash2Icon,
  TriangleAlertIcon,
  XIcon,
} from "lucide-react"
import { useEffect, useState } from "react"
import { useNavigate, useSearchParams } from "react-router"

import { CodeLine, ErrorLine, Notice, Segmented, TypeToConfirmDialog, useConfirm } from "@/components/common"
import { formatDate, formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { Textarea } from "@/components/ui/textarea"
import { AlertsView } from "@/features/analytics/Alerts"
import { Exports } from "@/features/analytics/Exports"
import { InstallSnippets } from "@/features/analytics/Install"
import { hostOnly, splitLines, useZones } from "@/features/analytics/NewProperty"
import { useCurrentProperty } from "@/features/analytics/parts"
import {
  useDeleteData,
  useDeleteProperty,
  useHealth,
  useHealthWatch,
  useRotateKey,
  useUpdateProperty,
} from "@/features/analytics/queries"
import { Sharing } from "@/features/analytics/Sharing"
import { todayIso } from "@/features/analytics/state"
import { ChoiceSelect, Field, FormActions, FormCard, Card, Section } from "@/features/settings/ui"
import { useAnalyticsText } from "@/features/analytics/text"
import {
  ApiError,
  type AnalyticsConsentFeature,
  type AnalyticsDropCounts,
  type AnalyticsHealthDay,
  type AnalyticsPathRule,
  type AnalyticsPlatform,
  type AnalyticsProperty,
} from "@/lib/api"
import { useNow } from "@/hooks/use-now"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

type Tab = "general" | "tracking" | "install" | "test" | "health" | "alerts" | "sharing" | "data"

const VIEW_PARAMS = ["period", "compare", "f", "segment"]

const NO_INBOX = "none"

function lines(s: string) {
  return s
    .split("\n")
    .map((x) => x.trim())
    .filter(Boolean)
}

const PLATFORMS: AnalyticsPlatform[] = ["web", "ios", "android"]

function General({ p }: { p: AnalyticsProperty }) {
  const { t } = useLingui()
  const text = useAnalyticsText()
  const inboxes = useInboxes().data ?? []
  const zones = useZones()
  const update = useUpdateProperty(p.id)
  const [name, setName] = useState(p.name)
  const [domains, setDomains] = useState(p.domains.join("\n"))
  const [zone, setZone] = useState(p.timezone)
  const [currency, setCurrency] = useState(p.currency)
  const [inbox, setInbox] = useState(p.inbox_id ?? NO_INBOX)
  const [audience, setAudience] = useState(p.audience)
  const [collection, setCollection] = useState(p.collection)
  const [platforms, setPlatforms] = useState<AnalyticsPlatform[]>(p.platforms)
  const hosts = splitLines(domains).map(hostOnly).filter(Boolean)
  const save = () =>
    update.mutate({
      name: name.trim(),
      domains: hosts,
      timezone: zone,
      currency: currency.toUpperCase(),
      inbox_id: inbox === NO_INBOX ? null : inbox,
      audience,
      collection,
      platforms: PLATFORMS.filter((x) => platforms.includes(x)),
      ...(audience === "children" ? { consent_features: [], tracked_props: [] } : {}),
    })
  return (
    <FormCard
      onSubmit={save}
      footer={
        <FormActions
          pending={update.isPending}
          saved={update.isSuccess}
          error={update.error}
          disabled={
            !name.trim() ||
            hosts.length === 0 ||
            platforms.length === 0 ||
            !zones.includes(zone) ||
            !/^[A-Za-z]{3}$/.test(currency)
          }
        />
      }
    >
      <Field label={<Trans>Name</Trans>} htmlFor="ap-name">
        <Input id="ap-name" value={name} maxLength={100} onChange={(e) => setName(e.target.value)} />
      </Field>
      <Field
        label={<Trans>Domains</Trans>}
        htmlFor="ap-domains"
        hint={<Trans>One per line. *.example.com covers every subdomain, but not example.com itself.</Trans>}
      >
        <Textarea id="ap-domains" dir="ltr" rows={3} value={domains} onChange={(e) => setDomains(e.target.value)} />
      </Field>
      <div className="flex min-w-0 flex-col gap-2">
        <span className="text-body font-medium" id="ap-platforms">
          <Trans>Platforms</Trans>
        </span>
        <div className="flex flex-wrap gap-x-5 gap-y-2" role="group" aria-labelledby="ap-platforms">
          {PLATFORMS.map((x) => (
            <Label key={x} className="font-normal" data-testid={`ap-platform-${x}`}>
              <Checkbox
                checked={platforms.includes(x)}
                onCheckedChange={(on) => setPlatforms(on ? [...platforms, x] : platforms.filter((y) => y !== x))}
                aria-invalid={platforms.length === 0 || undefined}
              />
              {text.platform[x]}
            </Label>
          ))}
        </div>
        {platforms.length === 0 ? (
          <p role="alert" className="text-small text-destructive">
            <Trans>Pick at least one platform.</Trans>
          </p>
        ) : (
          <p className="text-small text-faint">
            <Trans>
              Apps send without a domain, so only the platforms turned on here are counted. Events from your backend
              need no platform.
            </Trans>
          </p>
        )}
      </div>
      <div className="grid gap-5 sm:grid-cols-2">
        <Field
          label={<Trans>Time zone</Trans>}
          htmlFor="ap-zone"
          hint={<Trans>A change applies to new events. Days already counted keep their zone.</Trans>}
        >
          <Input id="ap-zone" list="ap-zones" value={zone} onChange={(e) => setZone(e.target.value)} />
          <datalist id="ap-zones">
            {zones.map((z) => (
              <option key={z} value={z} />
            ))}
          </datalist>
        </Field>
        <Field
          label={<Trans>Currency</Trans>}
          htmlFor="ap-currency"
          hint={<Trans>For revenue, an ISO code like EUR.</Trans>}
        >
          <Input
            id="ap-currency"
            dir="ltr"
            maxLength={3}
            value={currency}
            onChange={(e) => setCurrency(e.target.value.toUpperCase())}
          />
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
        <Field
          label={<Trans>Audience</Trans>}
          htmlFor="ap-audience"
          hint={
            audience === "children" ? (
              <Trans>Only the country is kept, never the region or city.</Trans>
            ) : (
              <Trans>Choose children for a site made for children.</Trans>
            )
          }
        >
          <ChoiceSelect
            id="ap-audience"
            label={t`Audience`}
            value={audience}
            onChange={setAudience}
            options={[
              ["general", t`General`],
              ["children", t`Children`],
            ]}
            className="w-full"
          />
        </Field>
      </div>
      <Field
        label={<Trans>Collection</Trans>}
        htmlFor="ap-collection"
        hint={
          collection === "consent" ? (
            <Trans>
              Events count only after the page calls yuva.consent with analytics set to true. Install the script with
              data-consent="required".
            </Trans>
          ) : (
            <Trans>
              Counts every visitor without asking: nothing is stored on the device and no IP address is kept.
            </Trans>
          )
        }
      >
        <ChoiceSelect
          id="ap-collection"
          label={t`Collection`}
          value={collection}
          onChange={setCollection}
          options={[
            ["anonymous", t`Without cookies, for everyone`],
            ["consent", t`Only with consent`],
          ]}
          className="w-full sm:max-w-80"
        />
      </Field>
    </FormCard>
  )
}

function RuleRows({ rules, onChange }: { rules: AnalyticsPathRule[]; onChange: (r: AnalyticsPathRule[]) => void }) {
  const { t } = useLingui()
  return (
    <div className="flex flex-col gap-2">
      {rules.map((r, i) => (
        <div key={i} className="flex items-center gap-2" dir="ltr" data-testid="path-rule">
          <Input
            value={r.pattern}
            placeholder="/orders/*"
            aria-label={t`Pattern`}
            onChange={(e) => onChange(rules.map((x, j) => (j === i ? { ...x, pattern: e.target.value } : x)))}
          />
          <span className="shrink-0 text-faint">→</span>
          <Input
            value={r.replacement}
            placeholder="/orders/:id"
            aria-label={t`Counted as`}
            onChange={(e) => onChange(rules.map((x, j) => (j === i ? { ...x, replacement: e.target.value } : x)))}
          />
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={t`Remove rule`}
            onClick={() => onChange(rules.filter((_, j) => j !== i))}
          >
            <XIcon />
          </Button>
        </div>
      ))}
      {rules.length < 50 && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="w-fit"
          onClick={() => onChange([...rules, { pattern: "", replacement: "" }])}
          data-testid="add-path-rule"
        >
          <PlusIcon />
          <Trans>Add a rule</Trans>
        </Button>
      )}
    </div>
  )
}

function Tracking({ p }: { p: AnalyticsProperty }) {
  const update = useUpdateProperty(p.id)
  const [paths, setPaths] = useState(p.excluded_paths.join("\n"))
  const [ips, setIps] = useState(p.excluded_ips.join("\n"))
  const [rules, setRules] = useState<AnalyticsPathRule[]>(p.path_rules)
  const complete = rules.filter((r) => r.pattern.trim() && r.replacement.trim())
  return (
    <FormCard
      onSubmit={() =>
        update.mutate({
          excluded_paths: lines(paths),
          excluded_ips: lines(ips),
          path_rules: complete.map((r) => ({ pattern: r.pattern.trim(), replacement: r.replacement.trim() })),
        })
      }
      footer={<FormActions pending={update.isPending} saved={update.isSuccess} error={update.error} />}
    >
      <Field
        label={<Trans>Pages not counted</Trans>}
        htmlFor="ap-paths"
        hint={<Trans>One path per line, up to 50. * stands for one part of the path, ** for any number of them.</Trans>}
      >
        <Textarea
          id="ap-paths"
          dir="ltr"
          rows={3}
          value={paths}
          placeholder={"/admin/**\n/preview/*"}
          onChange={(e) => setPaths(e.target.value)}
        />
      </Field>
      <Field
        label={<Trans>Addresses not counted</Trans>}
        htmlFor="ap-ips"
        hint={<Trans>One IP address or range per line, up to 50, such as your office or a monitoring service.</Trans>}
      >
        <Textarea
          id="ap-ips"
          dir="ltr"
          rows={3}
          value={ips}
          placeholder={"203.0.113.7\n198.51.100.0/24"}
          onChange={(e) => setIps(e.target.value)}
        />
      </Field>
      <Field
        label={<Trans>Path rules</Trans>}
        hint={
          <Trans>
            Pages that match a pattern are counted under one path, so every order page adds up as one. Addresses in a
            path are always replaced.
          </Trans>
        }
      >
        <RuleRows rules={rules} onChange={setRules} />
      </Field>
    </FormCard>
  )
}

function badProp(k: string) {
  return k.length > 300 || k.startsWith("$") || k.startsWith("yuva_")
}

function TrackedProps({ p }: { p: AnalyticsProperty }) {
  const { canManage } = useSession()
  const update = useUpdateProperty(p.id)
  const [value, setValue] = useState(p.tracked_props.join("\n"))
  const keys = [...new Set(lines(value))]
  const bad = keys.find(badProp)
  const tooMany = keys.length > 10
  const explain = (
    <Trans>
      A tracked prop is also kept in the daily totals, so its breakdown lasts after the detailed records are gone, and
      it shows as a chip on the Events view.
    </Trans>
  )
  return (
    <Section title={<Trans>Tracked props</Trans>} description={explain}>
      {p.audience === "children" ? (
        <Card data-testid="tracked-props-children">
          <p className="text-body text-muted-foreground">
            <Trans>Properties for children keep no props, so none can be tracked.</Trans>
          </p>
        </Card>
      ) : canManage ? (
        <FormCard
          onSubmit={() => update.mutate({ tracked_props: keys })}
          footer={
            <FormActions
              pending={update.isPending}
              saved={update.isSuccess}
              error={update.error}
              disabled={!!bad || tooMany}
            />
          }
        >
          <Field
            label={<Trans>Props</Trans>}
            htmlFor="ap-tracked-props"
            error={
              tooMany ? (
                <Trans>At most 10 props can be tracked.</Trans>
              ) : bad ? (
                <Trans>A prop name has at most 300 characters and does not start with $ or yuva_.</Trans>
              ) : undefined
            }
            hint={<Trans>One prop name per line, up to 10, such as plan.</Trans>}
          >
            <Textarea
              id="ap-tracked-props"
              dir="ltr"
              rows={3}
              value={value}
              placeholder={"plan\nseats"}
              onChange={(e) => setValue(e.target.value)}
              data-testid="tracked-props"
            />
          </Field>
        </FormCard>
      ) : (
        <Card data-testid="tracked-props-read">
          {p.tracked_props.length === 0 ? (
            <p className="text-body text-faint">
              <Trans>No props are tracked.</Trans>
            </p>
          ) : (
            <ul className="flex flex-wrap gap-1.5">
              {p.tracked_props.map((k) => (
                <li key={k} className="rounded-full border bg-card px-2.5 py-0.5 text-small" dir="ltr">
                  {k}
                </li>
              ))}
            </ul>
          )}
        </Card>
      )}
    </Section>
  )
}

function ConsentFeatures({ p }: { p: AnalyticsProperty }) {
  const { canManage } = useSession()
  const update = useUpdateProperty(p.id)
  const [persistent, setPersistent] = useState(p.consent_features.includes("persistent"))
  const [identify, setIdentify] = useState(p.consent_features.includes("identify"))
  const linked = !!p.inbox_id
  const canIdentify = persistent && linked
  const features: AnalyticsConsentFeature[] = [
    ...(persistent ? (["persistent"] as const) : []),
    ...(canIdentify && identify ? (["identify"] as const) : []),
  ]
  const explain = (
    <Trans>
      Offered only to visitors who agree in your consent tool. Everyone else is counted exactly as before, one day at a
      time and without anything stored on the device.
    </Trans>
  )
  if (p.audience === "children") {
    return (
      <Section title={<Trans>Consent features</Trans>} description={explain}>
        <Card data-testid="consent-features-children">
          <p className="text-body text-muted-foreground">
            <Trans>Properties for children never keep an id on the device or link visits to a contact.</Trans>
          </p>
        </Card>
      </Section>
    )
  }
  if (!canManage) {
    return (
      <Section title={<Trans>Consent features</Trans>} description={explain}>
        <Card data-testid="consent-features-read">
          <p className="text-body text-muted-foreground">
            {p.consent_features.includes("identify") ? (
              <Trans>Consented visitors keep a persistent id, and signed-in users are linked to their contact.</Trans>
            ) : p.consent_features.includes("persistent") ? (
              <Trans>Consented visitors keep a persistent id.</Trans>
            ) : (
              <Trans>No consent features are offered.</Trans>
            )}
          </p>
        </Card>
      </Section>
    )
  }
  return (
    <Section title={<Trans>Consent features</Trans>} description={explain}>
      <FormCard
        onSubmit={() => update.mutate({ consent_features: features })}
        footer={<FormActions pending={update.isPending} saved={update.isSuccess} error={update.error} />}
      >
        <div className="flex flex-col gap-1.5">
          <Label className="font-normal" data-testid="consent-persistent">
            <Checkbox
              checked={persistent}
              onCheckedChange={(on) => {
                setPersistent(on)
                if (!on) setIdentify(false)
              }}
            />
            <span className="font-medium">
              <Trans>Persistent id</Trans>
            </span>
          </Label>
          <p className="ps-6 text-small text-faint">
            <Trans>
              With yuva.consent and persistent set to true, the device keeps a random id for 13 months. The visitor
              counts once over several days, is told apart as new or returning, fills retention and is followed across
              days in funnels. Never under Do Not Track or Global Privacy Control.
            </Trans>
          </p>
        </div>
        <div className="flex flex-col gap-1.5">
          <Label className="font-normal" data-testid="consent-identify">
            <Checkbox checked={canIdentify && identify} disabled={!canIdentify} onCheckedChange={setIdentify} />
            <span className={cn("font-medium", !canIdentify && "text-faint")}>
              <Trans>Link signed-in users to their contact</Trans>
            </span>
          </Label>
          <p className="ps-6 text-small text-faint">
            {!persistent ? (
              <Trans>Needs the persistent id: a visitor is linked through it.</Trans>
            ) : !linked ? (
              <Trans>Needs a linked inbox under General: its identity secret signs the tokens.</Trans>
            ) : (
              <Trans>
                yuva.identify with a token from your backend, signed like a chat session, links the visitor’s visits to
                their contact. They show on the contact’s page under Recent visits. Forged or reused tokens are ignored.
              </Trans>
            )}
          </p>
        </div>
      </FormCard>
    </Section>
  )
}

function Install({ p }: { p: AnalyticsProperty }) {
  const { i18n } = useLingui()
  const { canManage } = useSession()
  const rotate = useRotateKey(p.id)
  const [ask, dialog] = useConfirm()
  const now = useNow(60_000)
  const until = p.previous_key_expires_at && Date.parse(p.previous_key_expires_at) > now
  const at = p.previous_key_expires_at ? formatDateTime(p.previous_key_expires_at, i18n.locale) : ""
  return (
    <>
      <Section title={<Trans>Key</Trans>}>
        <Card>
          <CodeLine value={p.public_key} testId="analytics-key" />
          <p className="text-small text-faint">
            <Trans>
              The key is public, like a chat key: every page carries it, and it only works on the property’s domains.
            </Trans>
          </p>
          {until && (
            <Notice icon={ClockIcon} data-testid="previous-key">
              <Trans>The previous key keeps working until {at}. Update every page and app before then.</Trans>
            </Notice>
          )}
          {canManage && (
            <div className="flex flex-col gap-2">
              <Button
                variant="outline"
                size="sm"
                className="w-fit"
                disabled={rotate.isPending}
                onClick={() =>
                  ask({
                    title: <Trans>Make a new key?</Trans>,
                    description: (
                      <Trans>
                        The current key keeps working for 24 hours, so you can update your pages without losing events.
                        Making another key within that time ends the old one at once.
                      </Trans>
                    ),
                    confirm: <Trans>Make a new key</Trans>,
                    run: () => rotate.mutate(),
                  })
                }
                data-testid="rotate-key"
              >
                <KeyRoundIcon />
                <Trans>Make a new key</Trans>
              </Button>
              <ErrorLine error={rotate.error} />
            </div>
          )}
        </Card>
      </Section>
      <Section title={<Trans>Install</Trans>}>
        <Card>
          <InstallSnippets property={p} />
        </Card>
      </Section>
      {dialog}
    </>
  )
}

function dropped(d: AnalyticsDropCounts) {
  return d.bot + d.rate + d.origin + d.invalid + d.late + d.over_limit + d.buffer + d.excluded
}

function Health({ p }: { p: AnalyticsProperty }) {
  const { t, i18n } = useLingui()
  const health = useHealth(p.id, 30)
  const h = health.data
  const fmt = new Intl.NumberFormat(i18n.locale)
  if (!h) return health.error ? <ErrorLine error={health.error} /> : <Skeleton className="h-64 rounded-2xl" />
  const reasons: [keyof AnalyticsDropCounts, string][] = [
    ["bot", t`Bots`],
    ["rate", t`Rate limits`],
    ["origin", t`Other sites`],
    ["excluded", t`Excluded`],
    ["over_limit", t`Monthly limit`],
    ["invalid", t`Invalid`],
    ["late", t`Too late`],
    ["buffer", t`Server busy`],
  ]
  const tiles: [string, string][] = [
    [fmt.format(h.totals.accepted), t`events counted`],
    [fmt.format(dropped(h.totals.dropped)), t`events dropped`],
    [fmt.format(h.totals.scrubbed), t`had an address removed`],
  ]
  const geo = h.geoip
  const sources = { dbip: "DB-IP Lite", file: t`The server’s own database`, off: t`Off` }
  return (
    <div className="flex flex-col gap-6" data-testid="analytics-health">
      <Section title={<Trans>The last 30 days</Trans>}>
        <div className="grid grid-cols-3 gap-2">
          {tiles.map(([v, label]) => (
            <div key={label} className="min-w-0 rounded-2xl border bg-card px-4 py-3.5 phone:px-3">
              <b className="block text-page tabular-nums">{v}</b>
              <span className="block text-small break-words text-faint">{label}</span>
            </div>
          ))}
        </div>
        <Card>
          <dl className="grid gap-x-6 gap-y-3 text-body sm:grid-cols-2">
            <div>
              <dt className="text-small text-faint">
                <Trans>Last event</Trans>
              </dt>
              <dd>{h.last_event_at ? formatDateTime(h.last_event_at, i18n.locale) : t`None yet`}</dd>
            </div>
            <div>
              <dt className="text-small text-faint">
                <Trans>Detailed records kept since</Trans>
              </dt>
              <dd>{formatDate(`${h.raw_window_from}T12:00:00Z`, i18n.locale)}</dd>
            </div>
            <div>
              <dt className="text-small text-faint">
                <Trans>Days waiting to be summed up</Trans>
              </dt>
              <dd>{fmt.format(h.pending_rollup_days)}</dd>
            </div>
            <div>
              <dt className="text-small text-faint">
                <Trans>Location database</Trans>
              </dt>
              <dd>
                {sources[geo.source]}
                {geo.built_at && <span className="text-faint"> · {formatDate(geo.built_at, i18n.locale)}</span>}
              </dd>
              {geo.error && <dd className="text-small text-destructive">{geo.error}</dd>}
              {geo.attribution && (
                <dd className="text-small">
                  {geo.attribution_url ? (
                    <a
                      href={geo.attribution_url}
                      target="_blank"
                      rel="noreferrer"
                      className="text-brand underline-offset-4 hover:underline"
                    >
                      {geo.attribution}
                    </a>
                  ) : (
                    geo.attribution
                  )}
                </dd>
              )}
            </div>
          </dl>
        </Card>
      </Section>
      <Section
        title={<Trans>Per day</Trans>}
        description={
          <Trans>
            Visitors with Do Not Track or Global Privacy Control send nothing, so they are not counted here either.
            Nothing about a dropped event is kept but its count.
          </Trans>
        }
      >
        <div className="overflow-x-auto rounded-2xl border bg-card">
          <table className="w-full text-body tabular-nums" data-testid="health-days">
            <thead>
              <tr className="border-b text-caption text-faint">
                <th className="px-4 py-2.5 text-start font-medium">
                  <Trans>Day</Trans>
                </th>
                <th className="px-2 py-2.5 text-end font-medium">
                  <Trans>Counted</Trans>
                </th>
                <th className="px-2 py-2.5 text-end font-medium">
                  <Trans>Dropped</Trans>
                </th>
                {reasons.map(([k, label]) => (
                  <th key={k} className="px-2 py-2.5 text-end font-medium whitespace-nowrap phone:hidden">
                    {label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y">
              {h.days.map((d) => {
                const n = dropped(d.dropped)
                return (
                  <tr key={d.day}>
                    <td className="px-4 py-2 whitespace-nowrap">{formatDate(`${d.day}T12:00:00Z`, i18n.locale)}</td>
                    <td className="px-2 py-2 text-end">{fmt.format(d.accepted)}</td>
                    <td className={cn("px-2 py-2 text-end", n > d.accepted && n > 0 && "text-warning")}>
                      {fmt.format(n)}
                    </td>
                    {reasons.map(([k]) => (
                      <td key={k} className={cn("px-2 py-2 text-end phone:hidden", d.dropped[k] === 0 && "text-faint")}>
                        {fmt.format(d.dropped[k])}
                      </td>
                    ))}
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </Section>
    </div>
  )
}

type Counts = Pick<AnalyticsHealthDay, "accepted"> & { dropped: AnalyticsDropCounts }

const NO_DROPS: AnalyticsDropCounts = {
  bot: 0,
  rate: 0,
  origin: 0,
  invalid: 0,
  late: 0,
  over_limit: 0,
  buffer: 0,
  excluded: 0,
}

function since(now: AnalyticsHealthDay[], base: AnalyticsHealthDay[]): Counts {
  const out: Counts = { accepted: 0, dropped: { ...NO_DROPS } }
  for (const d of now) {
    const b = base.find((x) => x.day === d.day)
    out.accepted += Math.max(0, d.accepted - (b?.accepted ?? 0))
    for (const k of Object.keys(NO_DROPS) as (keyof AnalyticsDropCounts)[])
      out.dropped[k] += Math.max(0, d.dropped[k] - (b?.dropped[k] ?? 0))
  }
  return out
}

type Baseline = { at: number; days: AnalyticsHealthDay[] }

const BASELINE_FOR = 30 * 60_000

function baselineKey(id: string) {
  return `yuva.analytics.test-events.${id}`
}

function readBaseline(id: string, now: number): Baseline | null {
  try {
    const b = JSON.parse(localStorage.getItem(baselineKey(id)) ?? "null") as Baseline | null
    if (b && typeof b.at === "number" && Array.isArray(b.days) && now - b.at < BASELINE_FOR) return b
  } catch {
    return null
  }
  return null
}

function writeBaseline(id: string, b: Baseline) {
  try {
    localStorage.setItem(baselineKey(id), JSON.stringify(b))
  } catch {
    return
  }
}

function TestEvents({ p }: { p: AnalyticsProperty }) {
  const { t, i18n } = useLingui()
  const watch = useHealthWatch(p.id)
  const h = watch.data
  const [base, setBase] = useState<Baseline | null>(null)
  if (h && !base) setBase(readBaseline(p.id, watch.dataUpdatedAt) ?? { at: watch.dataUpdatedAt, days: h.days })
  useEffect(() => {
    if (base) writeBaseline(p.id, base)
  }, [p.id, base])
  const fmt = new Intl.NumberFormat(i18n.locale)
  if (!h || !base) return watch.error ? <ErrorLine error={watch.error} /> : <Skeleton className="h-64 rounded-2xl" />
  const c = since(h.days, base.days)
  const other = c.dropped.bot + c.dropped.rate + c.dropped.excluded + c.dropped.over_limit + c.dropped.buffer
  const drops = dropped(c.dropped)
  const at = new Intl.DateTimeFormat(i18n.locale, { timeStyle: "short" }).format(new Date(base.at))
  const today = h.days[0]
  const last = h.last_event_at ? formatDateTime(h.last_event_at, i18n.locale) : ""
  const todayCounted = fmt.format(today?.accepted ?? 0)
  const todayDropped = fmt.format(today ? dropped(today.dropped) : 0)
  const tiles: { key: string; value: number; label: string; help: string; bad?: boolean }[] = [
    {
      key: "accepted",
      value: c.accepted,
      label: t`Counted`,
      help: t`Taken and stored. They show up in Realtime within a minute.`,
    },
    {
      key: "late",
      value: c.dropped.late,
      label: t`Too late`,
      help: t`Older than 72 hours. Apps keep events without a connection for at most that long.`,
      bad: true,
    },
    {
      key: "invalid",
      value: c.dropped.invalid,
      label: t`Invalid`,
      help: t`Broken events, such as a screen address that does not start with app://, a reserved name or too many props.`,
      bad: true,
    },
    {
      key: "origin",
      value: c.dropped.origin,
      label: t`Refused`,
      help: t`From a site that is not one of the domains, or from an app whose platform is not turned on.`,
      bad: true,
    },
    {
      key: "other",
      value: other,
      label: t`Other drops`,
      help: t`Bots, rate limits, excluded addresses and paths, the monthly limit or a busy server.`,
      bad: true,
    },
  ]
  return (
    <div className="flex flex-col gap-6" data-testid="analytics-test-events">
      <Section
        title={<Trans>Test events</Trans>}
        description={
          <Trans>
            Send a screen or an event from your app or backend while this page is open, and see whether it reached the
            server. Refreshes every 5 seconds.
          </Trans>
        }
      >
        {c.accepted > 0 && drops === 0 ? (
          <Notice icon={CheckCircle2Icon} className="bg-success/8 [&>svg]:text-success" data-testid="test-events-ok">
            <Trans>Events are arriving.</Trans>
          </Notice>
        ) : drops > 0 ? (
          <Notice tone="warning" icon={TriangleAlertIcon} data-testid="test-events-dropped">
            <Trans>Some events were dropped. The reasons are below.</Trans>
          </Notice>
        ) : (
          <Notice icon={RadioIcon} data-testid="test-events-waiting">
            <span className="flex items-center gap-2">
              <Spinner className="size-3.5" />
              <Trans>Waiting for events since {at}.</Trans>
            </span>
          </Notice>
        )}
        <ul className="grid grid-cols-2 gap-2 sm:grid-cols-5" data-testid="test-events-counts">
          {tiles.map((x) => (
            <li
              key={x.key}
              className={cn(
                "min-w-0 rounded-2xl border bg-card px-4 py-3.5 phone:px-3",
                x.key === "accepted" && "col-span-2 sm:col-span-1"
              )}
              title={x.help}
              data-testid={`test-events-${x.key}`}
            >
              <b
                className={cn(
                  "block text-page tabular-nums",
                  x.value === 0 ? "text-faint" : x.bad ? "text-warning" : "text-success"
                )}
              >
                {fmt.format(x.value)}
              </b>
              <span className="block text-small break-words text-faint">{x.label}</span>
            </li>
          ))}
        </ul>
        <Card>
          <dl className="grid gap-x-6 gap-y-3 text-body sm:grid-cols-2">
            {tiles.slice(1).map((x) => (
              <div key={x.key}>
                <dt className="font-medium">{x.label}</dt>
                <dd className="text-small text-faint">{x.help}</dd>
              </div>
            ))}
          </dl>
        </Card>
        <div className="mx-1 flex flex-wrap items-center gap-x-3 gap-y-2 text-small text-faint">
          <span data-testid="test-events-last">
            {last ? <Trans>Last event: {last}</Trans> : <Trans>No event yet.</Trans>}
          </span>
          {today && (
            <span>
              <Trans>
                Today: {todayCounted} counted, {todayDropped} dropped
              </Trans>
            </span>
          )}
          <span className="flex-1" />
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setBase({ at: watch.dataUpdatedAt, days: h.days })}
            data-testid="test-events-reset"
          >
            <RotateCcwIcon />
            <Trans>Start over</Trans>
          </Button>
        </div>
        <p className="mx-1 text-caption text-faint">
          <Trans>
            Requests with Do Not Track or Global Privacy Control, and events without consent on a property that needs
            it, are taken but neither stored nor counted here.
          </Trans>
        </p>
      </Section>
    </div>
  )
}

function Data({ p }: { p: AnalyticsProperty }) {
  const { i18n } = useLingui()
  const navigate = useNavigate()
  const deleteData = useDeleteData(p.id)
  const deleteProperty = useDeleteProperty(p.id)
  const [ask, dialog] = useConfirm()
  const [from, setFrom] = useState(todayIso())
  const [to, setTo] = useState(todayIso())
  const [removing, setRemoving] = useState(false)
  const span =
    from && to ? `${formatDate(`${from}T12:00:00Z`, i18n.locale)} – ${formatDate(`${to}T12:00:00Z`, i18n.locale)}` : ""
  const mismatch = deleteProperty.error instanceof ApiError && deleteProperty.error.code === "confirmation_mismatch"
  return (
    <div className="flex flex-col gap-6">
      <Section
        title={<Trans>Delete days</Trans>}
        description={<Trans>Days are in the property’s time zone. Their totals are counted again without them.</Trans>}
      >
        <Card tone="danger">
          <div className="grid grid-cols-2 gap-3 sm:max-w-md">
            <Field label={<Trans>From</Trans>} htmlFor="del-from">
              <Input id="del-from" type="date" value={from} max={to} onChange={(e) => setFrom(e.target.value)} />
            </Field>
            <Field label={<Trans>To</Trans>} htmlFor="del-to">
              <Input id="del-to" type="date" value={to} min={from} onChange={(e) => setTo(e.target.value)} />
            </Field>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={!from || !to || from > to || deleteData.isPending}
              onClick={() =>
                ask({
                  title: <Trans>Delete the visits and events of {span}?</Trans>,
                  description: <Trans>This cannot be undone.</Trans>,
                  confirm: <Trans>Delete these days</Trans>,
                  run: () => deleteData.mutate({ from, to }),
                })
              }
              data-testid="delete-range"
            >
              <Trash2Icon />
              <Trans>Delete these days</Trans>
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={deleteData.isPending}
              onClick={() =>
                ask({
                  title: <Trans>Delete all data of {p.name}?</Trans>,
                  description: (
                    <Trans>
                      Every visit, event and total is deleted. The property and its key stay, and counting goes on.
                    </Trans>
                  ),
                  confirm: <Trans>Delete all data</Trans>,
                  run: () => deleteData.mutate({ all: true }),
                })
              }
              data-testid="delete-all-data"
            >
              <Trans>Delete all data</Trans>
            </Button>
          </div>
          {deleteData.isSuccess && (
            <p className="text-body text-muted-foreground" role="status">
              <Trans>Deleting in the background. The numbers update when it is done.</Trans>
            </p>
          )}
          <ErrorLine error={deleteData.error} />
        </Card>
      </Section>
      <Section title={<Trans>Delete the property</Trans>}>
        <Card tone="danger">
          <p className="text-body text-muted-foreground">
            <Trans>
              The key stops working at once, and all visits, events and totals are deleted. This cannot be undone.
            </Trans>
          </p>
          <Button
            variant="destructive"
            size="sm"
            className="w-fit"
            onClick={() => setRemoving(true)}
            data-testid="delete-property"
          >
            <Trans>Delete the property</Trans>
          </Button>
        </Card>
      </Section>
      <TypeToConfirmDialog
        open={removing}
        onOpenChange={setRemoving}
        title={<Trans>Delete {p.name}?</Trans>}
        description={<Trans>Type the property’s name to delete it and all its data.</Trans>}
        label={<Trans>Property name</Trans>}
        expected={p.name}
        confirm={<Trans>Delete the property</Trans>}
        pending={deleteProperty.isPending}
        error={
          mismatch ? (
            <p role="alert" className="text-body text-destructive">
              <Trans>The name does not match. Type it exactly as shown.</Trans>
            </p>
          ) : (
            <ErrorLine error={deleteProperty.error} />
          )
        }
        onConfirm={() =>
          deleteProperty.mutate(p.name, {
            onSuccess: () => {
              setRemoving(false)
              navigate("/analytics", { replace: true })
            },
          })
        }
      />
      {dialog}
    </div>
  )
}

export function SettingsView() {
  const { t } = useLingui()
  const { canManage } = useSession()
  const p = useCurrentProperty()
  const [params, setParams] = useSearchParams()
  const tabs: Tab[] = canManage
    ? ["general", "tracking", "install", "test", "health", "alerts", "sharing", "data"]
    : ["install", "test", "tracking", "health", "alerts", "sharing", "data"]
  const raw = params.get("tab") as Tab | null
  const tab = raw && tabs.includes(raw) ? raw : tabs[0]
  const names: Record<Tab, string> = {
    general: t`General`,
    tracking: t`Tracking`,
    install: t`Install`,
    test: t`Test events`,
    health: t`Health`,
    alerts: t`Alerts and reports`,
    sharing: t`Sharing`,
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
      {tab === "tracking" && (
        <div className="flex flex-col gap-6">
          {canManage && <Tracking p={p} />}
          <TrackedProps p={p} />
          <ConsentFeatures p={p} />
        </div>
      )}
      {tab === "install" && <Install p={p} />}
      {tab === "test" && <TestEvents p={p} />}
      {tab === "health" && <Health p={p} />}
      {tab === "alerts" && <AlertsView p={p} />}
      {tab === "sharing" && <Sharing p={p} />}
      {tab === "data" && (
        <div className="flex flex-col gap-6">
          <Exports p={p} />
          {canManage && <Data p={p} />}
        </div>
      )}
    </div>
  )
}

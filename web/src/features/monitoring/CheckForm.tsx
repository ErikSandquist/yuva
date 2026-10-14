import { Trans, useLingui } from "@lingui/react/macro"
import { PlusIcon, XIcon } from "lucide-react"
import { useState } from "react"
import { Navigate, useNavigate, useParams, useSearchParams } from "react-router"

import { ErrorLine, Segmented } from "@/components/common"
import { Pane } from "@/components/common/Column"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { useAnalyticsProperties } from "@/features/analytics/properties"
import { useCheck, useChecks, useSaveCheck } from "@/features/monitoring/queries"
import { formatInterval } from "@/features/monitoring/text"
import { ChoiceSelect, Field, FormCard, PageHeader } from "@/features/settings/ui"
import type { MonitorCheck } from "@/lib/api"
import { useSession } from "@/lib/session"

const NONE = "none"

const INTERVALS = [30, 60, 120, 300, 600, 900, 1800, 3600, 21600, 43200, 86400]

type Header = { name: string; value: string }

function parseStatuses(s: string) {
  const out: number[] = []
  for (const part of s.split(/[\s,]+/).filter(Boolean)) {
    const range = part.match(/^(\d{3})-(\d{3})$/)
    if (range) {
      for (let n = Number(range[1]); n <= Number(range[2]) && out.length <= 20; n++) out.push(n)
      continue
    }
    if (!/^\d{3}$/.test(part)) return null
    out.push(Number(part))
  }
  if (out.length > 20 || out.some((n) => n < 200 || n > 399)) return null
  return [...new Set(out)]
}

function between(v: string, min: number, max: number) {
  const n = Number(v)
  return Number.isInteger(n) && n >= min && n <= max
}

function CheckFormBody({ check, propertyId }: { check?: MonitorCheck; propertyId?: string }) {
  const { t, i18n } = useLingui()
  const navigate = useNavigate()
  const list = useChecks()
  const properties = useAnalyticsProperties().data?.items ?? []
  const save = useSaveCheck(check?.id)
  const minInterval = list.data?.min_interval_seconds ?? 30
  const [name, setName] = useState(check?.name ?? "")
  const [url, setUrl] = useState(check?.url ?? "https://")
  const [method, setMethod] = useState(check?.method ?? "GET")
  const [replaceHeaders, setReplaceHeaders] = useState(!check || check.header_names.length === 0)
  const [headers, setHeaders] = useState<Header[]>([])
  const [statuses, setStatuses] = useState(check?.expected_statuses.join(", ") ?? "")
  const [keyword, setKeyword] = useState(check?.keyword ?? "")
  const [absent, setAbsent] = useState(check?.keyword_absent ?? false)
  const [timeout, setTimeoutValue] = useState(String(check?.timeout_seconds ?? 10))
  const [interval, setIntervalValue] = useState(String(check?.interval_seconds ?? Math.max(60, minInterval)))
  const [failures, setFailures] = useState(String(check?.failures_before_down ?? 2))
  const [certDays, setCertDays] = useState(String(check?.cert_warning_days ?? 14))
  const [redirects, setRedirects] = useState(String(check?.max_redirects ?? 5))
  const [service, setService] = useState(check?.service ?? "")
  const [property, setProperty] = useState(check?.property_id ?? propertyId ?? NONE)
  const parsed = parseStatuses(statuses)
  const validUrl = /^https?:\/\/[^\s/@]+(\/\S*)?$/i.test(url.trim())
  const intervals = INTERVALS.filter((s) => s >= minInterval)
  if (!intervals.includes(Number(interval))) intervals.unshift(Number(interval))
  const headersOk = headers.every((h) => /^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/.test(h.name) && h.value.length <= 2000)
  const valid =
    !!name.trim() &&
    validUrl &&
    parsed !== null &&
    headersOk &&
    between(timeout, 1, 30) &&
    Number(interval) >= minInterval &&
    between(failures, 1, 10) &&
    between(certDays, 1, 90) &&
    between(redirects, 0, 5)
  const https = url.trim().toLowerCase().startsWith("https://")
  const shortest = formatInterval(minInterval, i18n.locale)
  return (
    <FormCard
      onSubmit={() => {
        if (!valid) return
        save.mutate(
          {
            name: name.trim(),
            url: url.trim(),
            method,
            ...(replaceHeaders ? { headers: headers.filter((h) => h.name) } : {}),
            expected_statuses: parsed ?? [],
            keyword: method === "GET" && keyword.trim() ? keyword.trim() : null,
            keyword_absent: absent,
            timeout_seconds: Number(timeout),
            interval_seconds: Number(interval),
            failures_before_down: Number(failures),
            cert_warning_days: Number(certDays),
            max_redirects: Number(redirects),
            service: service.trim() || null,
            property_id: property === NONE ? null : property,
          },
          { onSuccess: (c) => navigate(`/monitoring/checks/${c.id}`, { replace: true }) }
        )
      }}
      footer={
        <>
          <ErrorLine error={save.error} className="min-w-0 flex-1" />
          <Button type="submit" className="ms-auto" disabled={!valid || save.isPending} data-testid="check-save">
            {check ? <Trans>Save</Trans> : <Trans>Add the check</Trans>}
          </Button>
        </>
      }
    >
      <Field label={<Trans>Name</Trans>} htmlFor="check-name">
        <Input
          id="check-name"
          autoFocus={!check}
          value={name}
          maxLength={100}
          placeholder={t`For example Shop API health`}
          onChange={(e) => setName(e.target.value)}
          data-testid="check-name"
        />
      </Field>
      <div className="grid gap-5 sm:grid-cols-[1fr_auto]">
        <Field
          label={<Trans>URL</Trans>}
          htmlFor="check-url"
          error={
            url !== "https://" && !validUrl ? (
              <Trans>An http or https address, without a user or password.</Trans>
            ) : undefined
          }
          hint={
            <Trans>
              The server calls it from its own network. Loopback, private, link-local and cloud metadata addresses are
              refused.
            </Trans>
          }
        >
          <Input
            id="check-url"
            dir="ltr"
            value={url}
            maxLength={2000}
            onChange={(e) => setUrl(e.target.value)}
            data-testid="check-url"
          />
        </Field>
        <Field label={<Trans>Method</Trans>}>
          <Segmented
            value={method}
            onChange={setMethod}
            label={t`Method`}
            items={[
              { value: "GET", label: "GET" },
              { value: "HEAD", label: "HEAD" },
            ]}
          />
        </Field>
      </div>
      <Field
        label={<Trans>Headers</Trans>}
        hint={<Trans>Up to 5, such as Authorization. Values are stored encrypted and never shown again.</Trans>}
      >
        {!replaceHeaders && check ? (
          <div className="flex flex-wrap items-center gap-2" data-testid="check-header-names">
            {check.header_names.map((h) => (
              <span key={h} className="rounded-full border bg-card px-2.5 py-0.5 font-mono text-small" dir="ltr">
                {h}: ••••
              </span>
            ))}
            <Button type="button" variant="outline" size="sm" onClick={() => setReplaceHeaders(true)}>
              <Trans>Replace the headers</Trans>
            </Button>
          </div>
        ) : (
          <div className="flex flex-col gap-2">
            {headers.map((h, i) => (
              <div key={i} className="flex items-center gap-2" dir="ltr" data-testid="check-header">
                <Input
                  value={h.name}
                  placeholder="Authorization"
                  aria-label={t`Header name`}
                  maxLength={100}
                  onChange={(e) => setHeaders(headers.map((x, j) => (j === i ? { ...x, name: e.target.value } : x)))}
                />
                <Input
                  type="password"
                  autoComplete="off"
                  value={h.value}
                  aria-label={t`Header value`}
                  maxLength={2000}
                  onChange={(e) => setHeaders(headers.map((x, j) => (j === i ? { ...x, value: e.target.value } : x)))}
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  aria-label={t`Remove header`}
                  onClick={() => setHeaders(headers.filter((_, j) => j !== i))}
                >
                  <XIcon />
                </Button>
              </div>
            ))}
            {headers.length < 5 && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="w-fit"
                onClick={() => setHeaders([...headers, { name: "", value: "" }])}
                data-testid="check-add-header"
              >
                <PlusIcon />
                <Trans>Add a header</Trans>
              </Button>
            )}
            {check && check.header_names.length > 0 && (
              <p className="text-small text-faint">
                <Trans>Saving replaces every header the check sends now.</Trans>
              </p>
            )}
          </div>
        )}
      </Field>
      <div className="grid gap-5 sm:grid-cols-2">
        <Field
          label={<Trans>Statuses that count as up</Trans>}
          htmlFor="check-statuses"
          error={
            parsed === null ? <Trans>Codes from 200 to 399, at most 20, such as 200, 204 or 200-204.</Trans> : undefined
          }
          hint={<Trans>Empty for any status from 200 to 399.</Trans>}
        >
          <Input
            id="check-statuses"
            dir="ltr"
            value={statuses}
            placeholder="200, 204"
            onChange={(e) => setStatuses(e.target.value)}
          />
        </Field>
        <Field
          label={<Trans>Check every</Trans>}
          htmlFor="check-interval"
          hint={<Trans>This server allows {shortest} at the shortest.</Trans>}
        >
          <ChoiceSelect
            id="check-interval"
            label={t`Check every`}
            value={interval}
            onChange={setIntervalValue}
            options={intervals.map((s) => [String(s), formatInterval(s, i18n.locale)] as const)}
            className="w-full"
          />
        </Field>
      </div>
      {method === "GET" && (
        <Field
          label={<Trans>Keyword</Trans>}
          htmlFor="check-keyword"
          hint={<Trans>Optional. Looked for in the first 64 KiB of the answer.</Trans>}
        >
          <Input
            id="check-keyword"
            value={keyword}
            maxLength={200}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder={t`For example "status":"ok"`}
          />
          {keyword.trim() && (
            <Label className="font-normal">
              <Checkbox checked={absent} onCheckedChange={(v) => setAbsent(v === true)} />
              <Trans>Down when the keyword is there, up when it is missing</Trans>
            </Label>
          )}
        </Field>
      )}
      <div className="grid gap-5 sm:grid-cols-2">
        <Field
          label={<Trans>Timeout in seconds</Trans>}
          htmlFor="check-timeout"
          error={between(timeout, 1, 30) ? undefined : <Trans>From 1 to 30.</Trans>}
        >
          <Input
            id="check-timeout"
            type="number"
            min={1}
            max={30}
            value={timeout}
            onChange={(e) => setTimeoutValue(e.target.value)}
          />
        </Field>
        <Field
          label={<Trans>Failed runs before down</Trans>}
          htmlFor="check-failures"
          error={between(failures, 1, 10) ? undefined : <Trans>From 1 to 10.</Trans>}
          hint={<Trans>Each failed run is tried again after 20 seconds before it counts.</Trans>}
        >
          <Input
            id="check-failures"
            type="number"
            min={1}
            max={10}
            value={failures}
            onChange={(e) => setFailures(e.target.value)}
          />
        </Field>
        <Field
          label={<Trans>Redirects to follow</Trans>}
          htmlFor="check-redirects"
          error={between(redirects, 0, 5) ? undefined : <Trans>From 0 to 5.</Trans>}
          hint={<Trans>With 0 the redirect’s own status is judged.</Trans>}
        >
          <Input
            id="check-redirects"
            type="number"
            min={0}
            max={5}
            value={redirects}
            onChange={(e) => setRedirects(e.target.value)}
          />
        </Field>
        <Field
          label={<Trans>Certificate warning, in days</Trans>}
          htmlFor="check-cert"
          error={between(certDays, 1, 90) ? undefined : <Trans>From 1 to 90.</Trans>}
          hint={
            https ? (
              <Trans>Alerts when the certificate expires within this many days.</Trans>
            ) : (
              <Trans>For https addresses.</Trans>
            )
          }
        >
          <Input
            id="check-cert"
            type="number"
            min={1}
            max={90}
            value={certDays}
            onChange={(e) => setCertDays(e.target.value)}
          />
        </Field>
        <Field
          label={<Trans>Service</Trans>}
          htmlFor="check-service"
          hint={<Trans>Optional. The service it watches, by the name its records give.</Trans>}
        >
          <Input id="check-service" value={service} maxLength={100} onChange={(e) => setService(e.target.value)} />
        </Field>
        <Field
          label={<Trans>Property</Trans>}
          htmlFor="check-property"
          hint={<Trans>Its uptime then shows on the property and in its reports.</Trans>}
        >
          <ChoiceSelect
            id="check-property"
            label={t`Property`}
            value={property}
            onChange={setProperty}
            options={[[NONE, t`None`], ...properties.map((p) => [p.id, p.name] as const)]}
            className="w-full"
          />
        </Field>
      </div>
    </FormCard>
  )
}

export function NewCheckPage() {
  const { t } = useLingui()
  const { canManage } = useSession()
  const [params] = useSearchParams()
  if (!canManage) return <Navigate to="/monitoring" replace />
  return (
    <Pane testId="check-new">
      <PageHeader
        back={{ to: "/monitoring", label: t`Monitoring` }}
        title={<Trans>Add an uptime check</Trans>}
        description={
          <Trans>
            The server calls an address on an interval and tells you when it stops answering as expected, and when it is
            back.
          </Trans>
        }
      />
      <CheckFormBody propertyId={params.get("property") ?? undefined} />
    </Pane>
  )
}

export function EditCheckPage() {
  const { t } = useLingui()
  const { canManage } = useSession()
  const { checkId = "" } = useParams()
  const check = useCheck(checkId)
  if (!canManage) return <Navigate to={`/monitoring/checks/${checkId}`} replace />
  return (
    <Pane testId="check-edit">
      <PageHeader
        back={{ to: `/monitoring/checks/${checkId}`, label: check.data?.name ?? t`Uptime check` }}
        title={<Trans>Change the check</Trans>}
      />
      {check.data ? (
        <CheckFormBody check={check.data} />
      ) : check.error ? (
        <ErrorLine error={check.error} />
      ) : (
        <Skeleton className="h-96 rounded-2xl" />
      )}
    </Pane>
  )
}

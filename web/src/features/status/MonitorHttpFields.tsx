import { Trans, useLingui } from "@lingui/react/macro"
import { PlusIcon, XIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { Field, ToggleRow } from "@/features/settings/ui"
import { ChoiceChips } from "@/features/status/Choice"
import type { FieldErrors } from "@/features/status/monitorErrors"
import type { StatusMonitorHTTPMethod } from "@/lib/api"

export const HTTP_METHODS: StatusMonitorHTTPMethod[] = ["GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"]

export type HeaderRow = { key: number; name: string; value: string; stored: boolean; replacing: boolean }

export type HttpForm = {
  url: string
  method: StatusMonitorHTTPMethod
  headers: HeaderRow[]
  body: string
  statusMin: number
  statusMax: number
  keyword: string
  follow: boolean
  certWarn: number
}

let nextKey = 0
export const newHeaderKey = () => ++nextKey

function HeadersEditor({
  rows,
  onChange,
  error,
}: {
  rows: HeaderRow[]
  onChange: (rows: HeaderRow[]) => void
  error?: string
}) {
  const { t } = useLingui()
  const set = (key: number, patch: Partial<HeaderRow>) =>
    onChange(rows.map((r) => (r.key === key ? { ...r, ...patch } : r)))
  return (
    <Field
      label={<Trans>Headers</Trans>}
      error={error}
      hint={<Trans>Values are stored encrypted and never shown again. To change one, replace it.</Trans>}
    >
      <div className="flex flex-col gap-2" data-testid="status-monitor-headers">
        {rows.map((r) => {
          const header = r.name
          return (
            <div key={r.key} className="flex flex-wrap items-center gap-2" data-testid="status-monitor-header">
              {r.stored ? (
                <span className="min-w-0 flex-1 basis-40 truncate font-mono text-body" title={r.name}>
                  {r.name}
                </span>
              ) : (
                <Input
                  className="min-w-0 flex-1 basis-40 font-mono"
                  value={r.name}
                  maxLength={100}
                  placeholder={t`Authorization`}
                  aria-label={t`Header name`}
                  onChange={(e) => set(r.key, { name: e.target.value })}
                />
              )}
              {r.stored && !r.replacing ? (
                <>
                  <span className="text-small text-faint">
                    <Trans>Value kept</Trans>
                  </span>
                  <Button type="button" variant="ghost" size="sm" onClick={() => set(r.key, { replacing: true })}>
                    <Trans>Replace</Trans>
                  </Button>
                </>
              ) : (
                <Input
                  className="min-w-0 flex-1 basis-40 font-mono"
                  value={r.value}
                  maxLength={4096}
                  autoComplete="off"
                  placeholder={t`New value`}
                  aria-label={t`Header value`}
                  onChange={(e) => set(r.key, { value: e.target.value })}
                />
              )}
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => onChange(rows.filter((x) => x.key !== r.key))}
                aria-label={t`Remove header ${header}`}
              >
                {r.stored ? <Trans>Remove</Trans> : <XIcon />}
              </Button>
            </div>
          )
        })}
        {rows.length < 20 && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="self-start"
            onClick={() =>
              onChange([...rows, { key: newHeaderKey(), name: "", value: "", stored: false, replacing: true }])
            }
            data-testid="status-monitor-add-header"
          >
            <PlusIcon />
            <Trans>Add header</Trans>
          </Button>
        )}
      </div>
    </Field>
  )
}

export function HttpUrlField({
  value,
  onChange,
  errors,
}: {
  value: HttpForm
  onChange: (patch: Partial<HttpForm>) => void
  errors: FieldErrors
}) {
  return (
    <Field label={<Trans>Address</Trans>} htmlFor="status-monitor-url" error={errors.url}>
      <Input
        id="status-monitor-url"
        type="url"
        inputMode="url"
        value={value.url}
        maxLength={2000}
        required
        placeholder="https://example.com/health"
        aria-invalid={!!errors.url}
        onChange={(e) => onChange({ url: e.target.value })}
        data-testid="status-monitor-url-input"
      />
    </Field>
  )
}

export function HttpDetails({
  value,
  onChange,
  errors,
}: {
  value: HttpForm
  onChange: (patch: Partial<HttpForm>) => void
  errors: FieldErrors
}) {
  const { t } = useLingui()
  const num = (v: string, lo: number, hi: number) => Math.min(hi, Math.max(lo, Number(v) || lo))
  const bodyless = value.method === "GET" || value.method === "HEAD" || value.method === "OPTIONS"
  return (
    <div className="flex flex-col gap-5 rounded-xl border p-4" data-testid="status-monitor-http-details">
      <Field label={<Trans>Method</Trans>}>
        <ChoiceChips
          value={value.method}
          onChange={(method) => onChange({ method })}
          options={HTTP_METHODS.map((m) => [m, m] as const)}
          label={t`Method`}
        />
      </Field>
      <HeadersEditor rows={value.headers} onChange={(headers) => onChange({ headers })} error={errors.headers} />
      {!bodyless && (
        <Field label={<Trans>Body</Trans>} htmlFor="status-monitor-body" error={errors.body}>
          <Textarea
            id="status-monitor-body"
            value={value.body}
            maxLength={16384}
            className="font-mono"
            onChange={(e) => onChange({ body: e.target.value })}
          />
        </Field>
      )}
      <div className="grid grid-cols-2 gap-4 phone:grid-cols-1">
        <Field
          label={<Trans>Expected status from</Trans>}
          htmlFor="status-monitor-status-min"
          error={errors.status}
          hint={<Trans>200 to 299 by default.</Trans>}
        >
          <Input
            id="status-monitor-status-min"
            type="number"
            inputMode="numeric"
            min={100}
            max={599}
            value={value.statusMin}
            onChange={(e) => onChange({ statusMin: num(e.target.value, 100, 599) })}
          />
        </Field>
        <Field label={<Trans>Expected status to</Trans>} htmlFor="status-monitor-status-max">
          <Input
            id="status-monitor-status-max"
            type="number"
            inputMode="numeric"
            min={100}
            max={599}
            value={value.statusMax}
            onChange={(e) => onChange({ statusMax: num(e.target.value, 100, 599) })}
          />
        </Field>
      </div>
      <Field
        label={<Trans>Keyword</Trans>}
        htmlFor="status-monitor-keyword"
        error={errors.keyword}
        hint={<Trans>Optional. The first 64 KiB of the answer must contain it, upper and lower case matter.</Trans>}
      >
        <Input
          id="status-monitor-keyword"
          value={value.keyword}
          maxLength={200}
          onChange={(e) => onChange({ keyword: e.target.value })}
        />
      </Field>
      <Field
        label={<Trans>Warn about the certificate</Trans>}
        htmlFor="status-monitor-cert"
        error={errors.cert}
        hint={<Trans>Days before it expires that the check fails. 0 only checks that it is valid.</Trans>}
      >
        <Input
          id="status-monitor-cert"
          type="number"
          inputMode="numeric"
          min={0}
          max={365}
          value={value.certWarn}
          onChange={(e) => onChange({ certWarn: num(e.target.value, 0, 365) })}
        />
      </Field>
      <ToggleRow
        title={<Trans>Follow redirects</Trans>}
        hint={<Trans>Off, a redirect is judged against the expected status.</Trans>}
      >
        <Switch checked={value.follow} onCheckedChange={(follow) => onChange({ follow })} />
      </ToggleRow>
    </div>
  )
}

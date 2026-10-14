import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { DownloadIcon, FileDownIcon, Trash2Icon } from "lucide-react"
import { useState } from "react"
import { Link } from "react-router"

import { ErrorLine, ProgressBar, Segmented } from "@/components/common"
import { formatBytes, formatDate, formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { FilterBar, RangePicker } from "@/features/analytics/Controls"
import { exportDownloadUrl, useExportActions, useExports, useHealth } from "@/features/analytics/queries"
import { API_FILTER_DIMENSIONS, FILTER_DIMENSIONS, todayIso, useView, useViewSearch } from "@/features/analytics/state"
import { useAnalyticsText } from "@/features/analytics/text"
import { Card, ChoiceSelect, EmptyRow, Field, Row, Rows, Section, StatusTag } from "@/features/settings/ui"
import {
  ApiError,
  type AnalyticsDataset,
  type AnalyticsExport,
  type AnalyticsExportKind,
  type AnalyticsMetric,
  type AnalyticsFilterDimension,
  type AnalyticsProperty,
} from "@/lib/api"
import { useSession } from "@/lib/session"

type Format = "csv" | "ndjson"

const NONE = "none"
const VIEW_DIMS = FILTER_DIMENSIONS.filter((d) => d !== "prop")
const DAILY_DIMS = FILTER_DIMENSIONS.filter((d) => d !== "prop" && d !== "event")
const VIEW_METRICS = ["visitors", "visits", "pageviews", "bounce_rate", "visit_duration"] as const

function daysBetween(from: string, to: string) {
  return Math.round((Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) / 86_400_000) + 1
}

function minusDays(day: string, n: number) {
  return new Date(Date.parse(`${day}T00:00:00Z`) - n * 86_400_000).toISOString().slice(0, 10)
}

function NewExport({ p }: { p: AnalyticsProperty }) {
  const { t, i18n } = useLingui()
  const text = useAnalyticsText()
  const { canManage } = useSession()
  const { view } = useView()
  const search = useViewSearch()
  const { create } = useExportActions(p.id)
  const rawFrom = useHealth(p.id, 1).data?.raw_window_from
  const [kind, setKind] = useState<AnalyticsExportKind>("view")
  const [format, setFormat] = useState<Format>("csv")
  const [dim, setDim] = useState<AnalyticsFilterDimension>("page")
  const [daily, setDaily] = useState<AnalyticsFilterDimension | typeof NONE>(NONE)
  const today = todayIso()
  const [from, setFrom] = useState(minusDays(today, 6))
  const [to, setTo] = useState(today)
  const span = from && to && from <= to ? daysBetween(from, to) : 0
  const tooLong = span > 31
  const early = !!rawFrom && !!from && from < rawFrom
  const valid = kind !== "raw" || (span > 0 && !tooLong && !early)
  const rawDay = rawFrom ? formatDate(`${rawFrom}T12:00:00Z`, i18n.locale) : ""
  const kinds: { value: AnalyticsExportKind; label: string; testId: string }[] = [
    { value: "view", label: t`Current view`, testId: "export-kind-view" },
    { value: "daily", label: t`Day by day`, testId: "export-kind-daily" },
    ...(canManage ? [{ value: "raw" as const, label: t`Raw events`, testId: "export-kind-raw" }] : []),
  ]
  const filters = view.filters.length
  const submit = () => {
    if (kind === "view") {
      create.mutate({
        kind,
        format,
        query: {
          metrics: [...VIEW_METRICS],
          dimensions: [dim],
          range: view.range,
          filters: view.filters,
          limit: 1000,
          insights: false,
        },
      })
    } else if (kind === "daily") {
      create.mutate({ kind, format, range: view.range, ...(daily !== NONE ? { dimension: daily } : {}) })
    } else {
      create.mutate({ kind, format: "ndjson", range: { from, to } })
    }
  }
  const err = create.error instanceof ApiError ? create.error : null
  const busy = err?.status === 429
  const rangeProblem = err?.code === "range_too_large" || err?.code === "needs_raw_window"
  return (
    <Card
      footer={
        <>
          {busy ? (
            <p role="alert" className="min-w-0 flex-1 text-body text-destructive">
              <Trans>Three exports are already running in this workspace. Try again when one finishes.</Trans>
            </p>
          ) : rangeProblem ? (
            <p role="alert" className="min-w-0 flex-1 text-body text-destructive">
              <Trans>This range cannot be exported. Pick a shorter one, or one inside the detailed records.</Trans>
            </p>
          ) : (
            <ErrorLine error={create.error} className="min-w-0 flex-1" />
          )}
          <Button
            size="sm"
            className="ms-auto"
            disabled={!valid || create.isPending}
            onClick={submit}
            data-testid="export-create"
          >
            <FileDownIcon />
            <Trans>Export</Trans>
          </Button>
        </>
      }
      data-testid="export-new"
    >
      <div className="-mx-1 max-w-[calc(100%+0.5rem)] overflow-x-auto px-1 py-1">
        <Segmented value={kind} onChange={setKind} label={t`What to export`} items={kinds} />
      </div>
      {kind === "raw" ? (
        <>
          <p className="text-body text-muted-foreground">
            <Trans>
              Every event with its visit’s page, source, place and device, as NDJSON. Visits are numbered within the
              file; there is no visitor id, address or contact. At most 31 days.
            </Trans>
          </p>
          <div className="grid grid-cols-2 gap-3 sm:max-w-md">
            <Field label={<Trans>From</Trans>} htmlFor="export-from">
              <Input
                id="export-from"
                type="date"
                value={from}
                min={rawFrom}
                max={to}
                onChange={(e) => setFrom(e.target.value)}
              />
            </Field>
            <Field label={<Trans>To</Trans>} htmlFor="export-to">
              <Input
                id="export-to"
                type="date"
                value={to}
                min={from}
                max={today}
                onChange={(e) => setTo(e.target.value)}
              />
            </Field>
          </div>
          {tooLong ? (
            <p role="alert" className="text-small text-destructive">
              <Trans>Raw exports cover at most 31 days.</Trans>
            </p>
          ) : (
            rawFrom && (
              <p className={early ? "text-small text-destructive" : "text-small text-faint"}>
                <Trans>Detailed records are kept since {rawDay}.</Trans>
              </p>
            )
          )}
        </>
      ) : (
        <>
          <p className="text-body text-muted-foreground">
            {kind === "view" ? (
              filters > 0 ? (
                <Trans>Every row of the range and the filters you are looking at, without a comparison.</Trans>
              ) : (
                <Trans>Every row of the range you are looking at, without a comparison.</Trans>
              )
            ) : (
              <Trans>
                One row per day with visitors, visits, views, bounce rate, durations and scroll depth. At most 400 days.
              </Trans>
            )}
          </p>
          <div className="flex flex-wrap items-end gap-3">
            <Field label={<Trans>Range</Trans>}>
              <RangePicker />
            </Field>
            {kind === "view" ? (
              <Field label={<Trans>Rows</Trans>} htmlFor="export-dim">
                <ChoiceSelect
                  id="export-dim"
                  value={dim}
                  onChange={setDim}
                  options={VIEW_DIMS.map((d) => [d, text.dimension[d]] as const)}
                />
              </Field>
            ) : (
              <Field label={<Trans>Broken down by</Trans>} htmlFor="export-daily-dim">
                <ChoiceSelect
                  id="export-daily-dim"
                  value={daily}
                  onChange={setDaily}
                  options={[[NONE, t`Nothing`] as const, ...DAILY_DIMS.map((d) => [d, text.dimension[d]] as const)]}
                />
              </Field>
            )}
            <Field label={<Trans>Format</Trans>}>
              <Segmented
                value={format}
                onChange={setFormat}
                label={t`Format`}
                items={[
                  { value: "csv", label: "CSV" },
                  { value: "ndjson", label: "NDJSON" },
                ]}
              />
            </Field>
          </div>
          {kind === "view" && (
            <Field label={<Trans>Filters</Trans>}>
              {filters > 0 ? (
                <FilterBar />
              ) : (
                <p className="text-body text-muted-foreground" data-testid="export-no-filters">
                  <Trans>None, every visit counts.</Trans>
                </p>
              )}
              <p className="text-small text-faint">
                <Trans>
                  Filters come from the view: click a row there to add one.{" "}
                  <Link
                    to={`/analytics/${p.id}${search}`}
                    className="text-brand underline-offset-4 hover:underline"
                    data-testid="export-change-filters"
                  >
                    Change them in the view
                  </Link>
                </Trans>
              </p>
            </Field>
          )}
        </>
      )}
    </Card>
  )
}

const API_DIMS: AnalyticsFilterDimension[] = API_FILTER_DIMENSIONS
const API_METRICS: AnalyticsMetric[] = [
  "requests",
  "errors",
  "error_rate",
  "client_error_rate",
  "p50",
  "p95",
  "p99",
  "apdex",
]
const UPTIME_METRICS: AnalyticsMetric[] = [
  "runs",
  "failures",
  "uptime",
  "downtime",
  "response_time_p50",
  "response_time_p95",
  "cert_days_left",
]

type ExportDataset = "api" | "uptime"

function ApiNewExport({ p }: { p: AnalyticsProperty }) {
  const { t } = useLingui()
  const text = useAnalyticsText()
  const { view } = useView("api")
  const { create } = useExportActions(p.id)
  const [kind, setKind] = useState<"view" | "daily">("view")
  const [dataset, setDataset] = useState<ExportDataset>("api")
  const [format, setFormat] = useState<Format>("csv")
  const [dim, setDim] = useState<AnalyticsFilterDimension>("route")
  const [daily, setDaily] = useState<AnalyticsFilterDimension | typeof NONE>(NONE)
  const dims: AnalyticsFilterDimension[] = dataset === "api" ? API_DIMS : ["check"]
  const by = dims.includes(dim) ? dim : dims[0]!
  const dailyBy = daily === NONE || dims.includes(daily) ? daily : NONE
  const filters = dataset === "api" ? view.filters : []
  const submit = () => {
    if (kind === "view") {
      create.mutate({
        kind,
        format,
        query: {
          dataset,
          metrics: dataset === "api" ? API_METRICS : UPTIME_METRICS,
          dimensions: [by],
          range: view.range,
          filters,
          limit: 1000,
          insights: false,
        },
      })
    } else {
      create.mutate({ kind, format, dataset, range: view.range, ...(dailyBy !== NONE ? { dimension: dailyBy } : {}) })
    }
  }
  const err = create.error instanceof ApiError ? create.error : null
  return (
    <Card
      footer={
        <>
          {err?.status === 429 ? (
            <p role="alert" className="min-w-0 flex-1 text-body text-destructive">
              <Trans>Three exports are already running in this workspace. Try again when one finishes.</Trans>
            </p>
          ) : (
            <ErrorLine error={create.error} className="min-w-0 flex-1" />
          )}
          <Button
            size="sm"
            className="ms-auto"
            disabled={create.isPending}
            onClick={submit}
            data-testid="export-create"
          >
            <FileDownIcon />
            <Trans>Export</Trans>
          </Button>
        </>
      }
      data-testid="export-new"
    >
      <div className="flex flex-wrap items-end gap-3">
        <Field label={<Trans>What to export</Trans>}>
          <Segmented
            value={kind}
            onChange={setKind}
            label={t`What to export`}
            items={[
              { value: "view", label: t`Current view`, testId: "export-kind-view" },
              { value: "daily", label: t`Day by day`, testId: "export-kind-daily" },
            ]}
          />
        </Field>
        <Field label={<Trans>Numbers</Trans>}>
          <Segmented
            value={dataset}
            onChange={setDataset}
            label={t`Numbers`}
            items={[
              { value: "api", label: t`Requests`, testId: "export-dataset-api" },
              { value: "uptime", label: t`Uptime checks`, testId: "export-dataset-uptime" },
            ]}
          />
        </Field>
      </div>
      <p className="text-body text-muted-foreground">
        {kind === "view" ? (
          dataset === "api" ? (
            <Trans>
              One row per value, with requests, errors, error rates, p50, p95, p99 and Apdex, for the range and filters
              you are looking at.
            </Trans>
          ) : (
            <Trans>
              One row per check, with runs, failures, uptime, downtime, response times and certificate days.
            </Trans>
          )
        ) : dataset === "api" ? (
          <Trans>One row per day with the request numbers, optionally broken down. At most 400 days.</Trans>
        ) : (
          <Trans>One row per day with the uptime numbers, optionally per check. At most 400 days.</Trans>
        )}
      </p>
      <div className="flex flex-wrap items-end gap-3">
        <Field label={<Trans>Range</Trans>}>
          <RangePicker kind="api" />
        </Field>
        {kind === "view" ? (
          <Field label={<Trans>Rows</Trans>} htmlFor="export-dim">
            <ChoiceSelect
              id="export-dim"
              value={by}
              onChange={setDim}
              options={dims.map((d) => [d, text.dimension[d]] as const)}
            />
          </Field>
        ) : (
          <Field label={<Trans>Broken down by</Trans>} htmlFor="export-daily-dim">
            <ChoiceSelect
              id="export-daily-dim"
              value={dailyBy}
              onChange={setDaily}
              options={[[NONE, t`Nothing`] as const, ...dims.map((d) => [d, text.dimension[d]] as const)]}
            />
          </Field>
        )}
        <Field label={<Trans>Format</Trans>}>
          <Segmented
            value={format}
            onChange={setFormat}
            label={t`Format`}
            items={[
              { value: "csv", label: "CSV" },
              { value: "ndjson", label: "NDJSON" },
            ]}
          />
        </Field>
      </div>
      {kind === "view" && dataset === "api" && filters.length > 0 && (
        <Field label={<Trans>Filters</Trans>}>
          <FilterBar kind="api" />
        </Field>
      )}
    </Card>
  )
}

function useFailure() {
  const { t } = useLingui()
  return (e: AnalyticsExport) => {
    const code = (e.error ?? "").split(":")[0]
    switch (code) {
      case "too_large":
        return t`The file would be larger than 1 GiB. Export a shorter range.`
      case "needs_raw_window":
        return t`The range is outside the detailed records.`
      case "invalid_query":
        return t`The query could not be run.`
      default:
        return t`The export failed on the server.`
    }
  }
}

function ExportRow({ e, ws, onDelete }: { e: AnalyticsExport; ws: string; onDelete: () => void }) {
  const { t, i18n } = useLingui()
  const failure = useFailure()
  const datasets: Record<AnalyticsDataset, string> = {
    traffic: t`Visits`,
    support: t`Support`,
    api: t`Requests`,
    uptime: t`Uptime checks`,
  }
  const kinds: Record<AnalyticsExportKind, string> = {
    view: t`Current view`,
    daily: t`Day by day`,
    raw: t`Raw events`,
  }
  const range = `${formatDate(`${e.from}T12:00:00Z`, i18n.locale)} – ${formatDate(`${e.to}T12:00:00Z`, i18n.locale)}`
  const size = formatBytes(e.size_bytes, i18n.locale)
  const expires = formatDateTime(e.expires_at, i18n.locale)
  const pct = Math.round(e.progress)
  return (
    <Row className="items-start" data-testid="export-row">
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
          <p className="min-w-0 text-body font-medium">
            {kinds[e.kind]}{" "}
            <span className="font-normal text-faint" data-testid="export-dataset">
              · {datasets[e.dataset]} · {e.format.toUpperCase()}
            </span>
          </p>
          {e.status === "done" ? (
            <StatusTag tone="success" data-testid="export-status">
              <Trans>Ready</Trans>
            </StatusTag>
          ) : e.status === "failed" ? (
            <StatusTag tone="danger" data-testid="export-status">
              <Trans>Failed</Trans>
            </StatusTag>
          ) : e.status === "running" ? (
            <StatusTag data-testid="export-status">
              <Trans>Running</Trans>
            </StatusTag>
          ) : (
            <StatusTag data-testid="export-status">
              <Trans>Waiting</Trans>
            </StatusTag>
          )}
        </div>
        <p className="text-caption text-muted-foreground">{range}</p>
        {(e.status === "queued" || e.status === "running") && (
          <div className="flex max-w-sm items-center gap-3">
            <ProgressBar
              thin
              value={e.status === "running" ? pct : undefined}
              label={t`Export progress`}
              text={`${pct}%`}
              testId="export-progress"
            />
            <span className="shrink-0 text-caption text-faint tabular-nums">
              <Plural value={e.rows} one="# row" other="# rows" />
            </span>
          </div>
        )}
        {e.status === "failed" && (
          <p className="text-caption text-destructive" data-testid="export-error">
            {failure(e)}
          </p>
        )}
        <p className="text-caption text-faint">
          {e.status === "done" && (
            <>
              <Plural value={e.rows} one="# row" other="# rows" />
              {`, ${size} · `}
            </>
          )}
          <Trans>deleted {expires}</Trans>
        </p>
      </div>
      <div className="-my-1 flex shrink-0 items-center gap-1">
        {e.status === "done" && (
          <Button
            variant="outline"
            size="sm"
            render={<a href={exportDownloadUrl(e.property_id, e.id, ws)} download={e.file_name} />}
            data-testid="export-download"
          >
            <DownloadIcon />
            <Trans>Download</Trans>
          </Button>
        )}
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t`Delete export`}
          title={t`Delete export`}
          onClick={onDelete}
          data-testid="export-delete"
        >
          <Trash2Icon />
        </Button>
      </div>
    </Row>
  )
}

export function Exports({ p }: { p: AnalyticsProperty }) {
  const { workspaceId } = useSession()
  const list = useExports(p.id)
  const { remove } = useExportActions(p.id)
  const items = list.data ?? []
  return (
    <div className="flex flex-col gap-6" data-testid="analytics-exports">
      <Section
        title={<Trans>Export</Trans>}
        description={<Trans>Files are made in the background and deleted seven days later.</Trans>}
      >
        {p.kind === "api" ? <ApiNewExport p={p} /> : <NewExport p={p} />}
      </Section>
      <Section title={<Trans>Exports</Trans>}>
        <Card flush>
          {list.isPending ? null : items.length === 0 ? (
            <EmptyRow>
              <Trans>No exports yet.</Trans>
            </EmptyRow>
          ) : (
            <Rows>
              {items.map((e) => (
                <ExportRow key={e.id} e={e} ws={workspaceId} onDelete={() => remove.mutate(e.id)} />
              ))}
            </Rows>
          )}
          <ErrorLine error={list.error ?? remove.error} className="px-4 py-3" />
        </Card>
      </Section>
    </div>
  )
}

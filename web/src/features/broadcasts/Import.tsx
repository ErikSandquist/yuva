import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { useMutation } from "@tanstack/react-query"
import {
  ChevronRightIcon,
  DownloadIcon,
  FileSpreadsheetIcon,
  FileUpIcon,
  LoaderIcon,
  TriangleAlertIcon,
  XIcon,
} from "lucide-react"
import { useRef, useState } from "react"
import { useSearchParams } from "react-router"

import { ErrorLine, Notice, Stepper, useConfirm } from "@/components/common"
import { formatBytes, formatDate, formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Item } from "@/components/ui/item"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { useNumber } from "@/features/broadcasts/BroadcastParts"
import {
  downloadErrors,
  IMPORT_MAX_BYTES,
  isOpen,
  isRunning,
  REFUSALS,
  refused,
  useCancelImport,
  useListImport,
  useListImports,
  useRemapImport,
  useStartImport,
  useUploadImport,
} from "@/features/broadcasts/importQueries"
import { Card, ChoiceSelect, EmptyRow, Field, Rows, Section, StatusTag } from "@/features/settings/ui"
import {
  ApiError,
  type ListImport,
  type ListImportColumnMapping,
  type ListImportCounts,
  type ListImportField,
  type ListImportState,
  type MailingList,
} from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

const FIELDS: ListImportField[] = [
  "email",
  "name",
  "first_name",
  "last_name",
  "locale",
  "external_id",
  "attribute",
  "skip",
]

const ATTRIBUTE_KEY = /^[A-Za-z0-9_.-]{1,64}$/

function useImportText() {
  const { t } = useLingui()
  return {
    state: {
      validating: t`Checking the file`,
      ready: t`Ready to import`,
      importing: t`Importing`,
      done: t`Done`,
      failed: t`Failed`,
      cancelled: t`Cancelled`,
    } satisfies Record<ListImportState, string>,
    field: {
      email: t`E-mail address`,
      name: t`Full name`,
      first_name: t`First name`,
      last_name: t`Last name`,
      locale: t`Language`,
      external_id: t`External id`,
      attribute: t`Attribute`,
      skip: t`Skip this column`,
    } satisfies Record<ListImportField, string>,
    refusal: {
      invalid_email: t`Not a valid e-mail address`,
      no_mail_domain: t`The domain takes no mail`,
      role_address: t`Role addresses such as postmaster@ or noreply@`,
      duplicate: t`Already earlier in the file`,
      suppressed: t`Bounced or reported as spam before`,
      opted_out: t`Unsubscribed from everything or asked to be forgotten`,
      unsubscribed: t`Unsubscribed from this list`,
    } satisfies Record<(typeof REFUSALS)[number], string>,
    encoding: {
      "utf-8": "UTF-8",
      "utf-16le": "UTF-16",
      "utf-16be": "UTF-16",
      "windows-1254": t`Windows-1254 (Turkish Excel)`,
      "windows-1252": t`Windows-1252 (Western Excel)`,
    },
    delimiter: { comma: t`commas`, semicolon: t`semicolons`, tab: t`tabs` },
  }
}

const tones: Record<ListImportState, "muted" | "success" | "warning" | "danger"> = {
  validating: "warning",
  ready: "warning",
  importing: "warning",
  done: "success",
  failed: "danger",
  cancelled: "muted",
}

function ImportState({ state }: { state: ListImportState }) {
  const text = useImportText()
  return (
    <StatusTag tone={tones[state]} data-testid="import-state" data-state={state}>
      {text.state[state]}
    </StatusTag>
  )
}

function Steps({ step }: { step: number }) {
  const { t } = useLingui()
  const steps = [t`File and consent`, t`Columns`, t`Review`, t`Import`].map((label, i) => ({
    key: String(i + 1),
    label,
  }))
  return <Stepper steps={steps} current={String(step)} label={t`Import steps`} testId="import-step" />
}

function useImportParam() {
  const [params, setParams] = useSearchParams()
  const current = params.get("import")
  const go = (id: string | null) =>
    setParams(
      (p) => {
        const next = new URLSearchParams(p)
        if (id) next.set("import", id)
        else next.delete("import")
        return next
      },
      { replace: !id }
    )
  return [current, go] as const
}

function Upload({
  list,
  onCreated,
  onBack,
}: {
  list: MailingList
  onCreated: (i: ListImport) => void
  onBack?: () => void
}) {
  const { t, i18n } = useLingui()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === list.inbox_id)
  const listName = list.name
  const inboxName = inbox?.name ?? ""
  const statement = t`Everyone in this file agreed to receive ${listName} from ${inboxName}, and I can show how and when if asked.`
  const input = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [fileError, setFileError] = useState<string | null>(null)
  const [over, setOver] = useState(false)
  const [source, setSource] = useState("")
  const [attested, setAttested] = useState(false)
  const [reconfirm, setReconfirm] = useState(false)
  const [touched, setTouched] = useState(false)
  const upload = useUploadImport(list.id)
  const code = upload.error instanceof ApiError ? upload.error.code : undefined
  const max = formatBytes(IMPORT_MAX_BYTES, i18n.locale)
  const pick = (f: File | undefined) => {
    if (!f) return
    upload.reset()
    if (!/\.(csv|txt|tsv)$/i.test(f.name) && f.type && !/csv|text\//.test(f.type)) {
      setFile(null)
      setFileError(t`Pick a CSV file. Save a spreadsheet as CSV first.`)
      return
    }
    if (f.size > IMPORT_MAX_BYTES) {
      setFile(null)
      setFileError(t`The file is larger than ${max}. Split it into smaller files.`)
      return
    }
    setFileError(null)
    setFile(f)
  }
  const size = file ? formatBytes(file.size, i18n.locale) : ""
  const fileName = file?.name ?? ""
  return (
    <form
      className="flex flex-col gap-5"
      onSubmit={(e) => {
        e.preventDefault()
        setTouched(true)
        if (!file || !source.trim() || !attested) return
        upload.mutate(
          { file, attestation: statement, consentSource: source.trim(), reconfirm },
          { onSuccess: onCreated }
        )
      }}
      data-testid="import-upload"
    >
      <Steps step={1} />
      <Card>
        <div className="flex flex-col gap-2">
          <span className="text-body font-medium">
            <Trans>CSV file</Trans>
          </span>
          <input
            ref={input}
            type="file"
            accept=".csv,text/csv,text/plain"
            className="hidden"
            onChange={(e) => {
              pick(e.target.files?.[0])
              e.target.value = ""
            }}
            data-testid="import-file-input"
          />
          <button
            type="button"
            onClick={() => input.current?.click()}
            onDragOver={(e) => {
              e.preventDefault()
              setOver(true)
            }}
            onDragLeave={() => setOver(false)}
            onDrop={(e) => {
              e.preventDefault()
              setOver(false)
              pick(e.dataTransfer.files[0])
            }}
            className={cn(
              "flex min-w-0 items-center gap-3 rounded-2xl border-2 border-dashed px-4 py-5 text-start text-body text-muted-foreground transition-colors hover:border-faint hover:text-foreground",
              over && "border-brand bg-brand-wash text-brand",
              touched && !file && "border-destructive/50"
            )}
            data-testid="import-file"
          >
            {file ? <FileSpreadsheetIcon className="size-5 shrink-0" /> : <FileUpIcon className="size-5 shrink-0" />}
            <span className="flex min-w-0 flex-col">
              {file ? (
                <>
                  <span className="truncate font-medium text-foreground" dir="auto">
                    {fileName}
                  </span>
                  <span className="text-small text-faint">
                    <Trans>{size} · pick another file</Trans>
                  </span>
                </>
              ) : (
                <>
                  <span>
                    <Trans>Pick a CSV file, or drop it here</Trans>
                  </span>
                  <span className="text-small text-faint">
                    <Trans>
                      Up to {max} and 100,000 rows, with a header row. Excel files in Turkish or Western encodings and
                      columns separated by commas, semicolons or tabs are read as they are.
                    </Trans>
                  </span>
                </>
              )}
            </span>
          </button>
          {(fileError || (touched && !file)) && (
            <p role="alert" className="text-small text-destructive">
              {fileError ?? <Trans>Pick the file to import.</Trans>}
            </p>
          )}
        </div>
        <Field
          label={<Trans>Where and when did these people agree?</Trans>}
          htmlFor="import-source"
          hint={<Trans>Kept with every subscription from this file, as the record of their consent.</Trans>}
          error={touched && !source.trim() ? <Trans>Write where and when they agreed.</Trans> : undefined}
        >
          <Textarea
            id="import-source"
            rows={3}
            maxLength={1000}
            value={source}
            placeholder={t`Newsletter checkbox on the checkout page of our shop, 2023 to 2026`}
            onChange={(e) => setSource(e.target.value)}
            aria-invalid={(touched && !source.trim()) || undefined}
            data-testid="import-source"
          />
        </Field>
        <div className="flex flex-col gap-1.5">
          <Label className="items-start gap-3 font-normal">
            <Checkbox
              checked={reconfirm}
              onCheckedChange={setReconfirm}
              disabled={!list.sender_ready}
              className="mt-0.5"
              data-testid="import-reconfirm"
            />
            <span className="flex min-w-0 flex-col gap-0.5">
              <span className="text-body font-medium">
                <Trans>Ask everyone to confirm again</Trans>
              </span>
              <span className="text-small text-faint">
                <Trans>
                  Nobody is subscribed yet: each new address gets the list’s confirmation mail, spread over days by the
                  daily sending limit, and joins only after clicking it.
                </Trans>
              </span>
            </span>
          </Label>
          {!list.sender_ready && (
            <p className="ms-7 text-small text-faint">
              <Trans>This list has no e-mail channel that can send, so it cannot mail a confirmation yet.</Trans>
            </p>
          )}
        </div>
        <div className="flex flex-col gap-1.5 border-t pt-4">
          <Label className="items-start gap-3 font-normal">
            <Checkbox
              checked={attested}
              onCheckedChange={setAttested}
              className="mt-0.5"
              aria-invalid={(touched && !attested) || undefined}
              data-testid="import-attest"
            />
            <span className="text-body">{statement}</span>
          </Label>
          {touched && !attested && (
            <p role="alert" className="text-small text-destructive">
              <Trans>Confirm the consent to import the file.</Trans>
            </p>
          )}
        </div>
        {upload.error && (
          <Notice tone="danger" icon={TriangleAlertIcon} role="alert" data-testid="import-upload-error">
            <ErrorLine error={upload.error} className="text-foreground" />
            {code === "import_running" && (
              <p className="text-small text-muted-foreground">
                <Trans>Imports run one at a time in the workspace. Try again when the other one ends.</Trans>
              </p>
            )}
          </Notice>
        )}
      </Card>
      <div className="flex flex-wrap justify-end gap-2">
        {onBack && (
          <Button type="button" variant="ghost" onClick={onBack}>
            <Trans>Cancel</Trans>
          </Button>
        )}
        <Button type="submit" disabled={upload.isPending} data-testid="import-upload-submit">
          {upload.isPending && <LoaderIcon className="animate-spin" />}
          {upload.isPending ? <Trans>Uploading…</Trans> : <Trans>Upload and check</Trans>}
        </Button>
      </div>
    </form>
  )
}

type Mapping = { field: ListImportField; attribute: string }

function mappingOf(i: ListImport): Mapping[] {
  return i.columns.map((c) => ({ field: c.field, attribute: c.attribute ?? "" }))
}

function sameMapping(a: Mapping[], b: Mapping[]) {
  return a.every((m, i) => m.field === b[i]?.field && (m.field !== "attribute" || m.attribute === b[i]?.attribute))
}

function Columns({
  list,
  imp,
  onNext,
  onCancel,
  cancelling,
}: {
  list: MailingList
  imp: ListImport
  onNext: () => void
  onCancel: () => void
  cancelling: boolean
}) {
  const { t } = useLingui()
  const text = useImportText()
  const guessed = mappingOf(imp)
  const [map, setMap] = useState<Mapping[]>(guessed)
  const remap = useRemapImport(list.id, imp.id)
  const preview = imp.preview ?? []
  const emails = map.filter((m) => m.field === "email").length
  const badKeys = map.some((m) => m.field === "attribute" && !ATTRIBUTE_KEY.test(m.attribute))
  const changed = !sameMapping(map, guessed)
  const options = FIELDS.map((f) => [f, text.field[f]] as const)
  const set = (i: number, patch: Partial<Mapping>) =>
    setMap((old) =>
      old.map((m, j) => {
        if (j === i) return { ...m, ...patch }
        if (patch.field === "email" && m.field === "email") return { ...m, field: "skip" as const }
        return m
      })
    )
  const submit = () => {
    if (emails !== 1 || badKeys) return
    if (!changed) return onNext()
    const columns: ListImportColumnMapping[] = map
      .map((m, index) => ({
        index: imp.columns[index].index,
        field: m.field,
        ...(m.field === "attribute" ? { attribute: m.attribute } : {}),
      }))
      .filter((c) => c.field !== "skip")
    remap.mutate(columns, { onSuccess: onNext })
  }
  return (
    <div className="flex flex-col gap-5" data-testid="import-columns">
      <Steps step={2} />
      <Section
        title={<Trans>What each column holds</Trans>}
        description={
          <Trans>
            The columns are guessed from the header row. One must be the e-mail address; name the others as contact
            fields or attributes, or skip them. Attributes only fill keys a contact does not have yet.
          </Trans>
        }
      >
        <Card flush>
          <Rows>
            {imp.columns.map((c, i) => {
              const m = map[i]
              const samples = preview
                .map((row) => row[c.index] ?? "")
                .filter(Boolean)
                .slice(0, 3)
              const n = i + 1
              const header = c.header || t`Column ${n}`
              return (
                <li
                  key={c.index}
                  className="grid gap-x-4 gap-y-2 px-4 py-3 sm:grid-cols-[minmax(0,1fr)_14rem]"
                  data-testid="import-column"
                >
                  <div className="min-w-0">
                    <p className="truncate text-body font-medium" dir="auto">
                      {header}
                    </p>
                    <p className="truncate text-small text-faint" dir="auto">
                      {samples.length ? samples.join(" · ") : <Trans>Empty in the first rows</Trans>}
                    </p>
                  </div>
                  <div className="flex min-w-0 flex-col gap-2">
                    <ChoiceSelect
                      value={m.field}
                      onChange={(field) => set(i, { field })}
                      options={options}
                      label={t`What ${header} holds`}
                      className="w-full"
                    />
                    {m.field === "attribute" && (
                      <Input
                        value={m.attribute}
                        maxLength={64}
                        placeholder={t`Attribute key, such as plan`}
                        aria-label={t`Attribute key for ${header}`}
                        aria-invalid={!ATTRIBUTE_KEY.test(m.attribute) || undefined}
                        onChange={(e) => set(i, { attribute: e.target.value })}
                        dir="ltr"
                        className="h-9 bg-background"
                      />
                    )}
                  </div>
                </li>
              )
            })}
          </Rows>
        </Card>
        {emails !== 1 && (
          <p role="alert" className="mx-1 text-small text-destructive" data-testid="import-email-required">
            <Trans>Pick the column that holds the e-mail addresses.</Trans>
          </p>
        )}
        {badKeys && (
          <p role="alert" className="mx-1 text-small text-destructive">
            <Trans>Attribute keys use letters, digits, dots, dashes and underscores, at most 64.</Trans>
          </p>
        )}
      </Section>
      {preview.length > 0 && (
        <Section title={<Trans>First rows of the file</Trans>}>
          <div className="max-w-full overflow-x-auto rounded-2xl border bg-card" data-testid="import-preview">
            <table className="w-full text-start text-small">
              <thead>
                <tr className="border-b">
                  {imp.columns.map((c, i) => (
                    <th
                      key={c.index}
                      scope="col"
                      className="px-3 py-2 text-start font-medium whitespace-nowrap text-muted-foreground"
                    >
                      <span dir="auto">{c.header}</span>
                      <span className="block text-caption font-normal text-faint">{text.field[map[i].field]}</span>
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y">
                {preview.slice(0, 8).map((row, r) => (
                  <tr key={r}>
                    {imp.columns.map((c) => (
                      <td key={c.index} className="max-w-56 truncate px-3 py-1.5 whitespace-nowrap" dir="auto">
                        {row[c.index] ?? ""}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Section>
      )}
      <ErrorLine error={remap.error} />
      <div className="flex flex-wrap justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onCancel} disabled={cancelling} data-testid="import-cancel">
          <Trans>Cancel import</Trans>
        </Button>
        <Button
          type="button"
          onClick={submit}
          disabled={remap.isPending || emails !== 1 || badKeys}
          data-testid="import-columns-next"
        >
          {remap.isPending && <LoaderIcon className="animate-spin" />}
          {changed ? <Trans>Check again with these columns</Trans> : <Trans>Continue</Trans>}
        </Button>
      </div>
    </div>
  )
}

function CountRow({
  label,
  value,
  tone,
  testId,
}: {
  label: React.ReactNode
  value: number
  tone?: string
  testId?: string
}) {
  const num = useNumber()
  return (
    <li className="flex items-baseline justify-between gap-4 px-4 py-2.5" data-testid={testId}>
      <span className="min-w-0 text-body">{label}</span>
      <b className={cn("shrink-0 text-body tabular-nums", tone)}>{num(value)}</b>
    </li>
  )
}

function Refusals({ counts }: { counts: ListImportCounts }) {
  const text = useImportText()
  const rows = REFUSALS.filter((k) => counts[k] > 0)
  if (rows.length === 0) return null
  return (
    <>
      {rows.map((k) => (
        <CountRow
          key={k}
          label={text.refusal[k]}
          value={counts[k]}
          tone="text-muted-foreground"
          testId={`import-refused-${k}`}
        />
      ))}
    </>
  )
}

function FileLine({ imp }: { imp: ListImport }) {
  const text = useImportText()
  const num = useNumber()
  const rows = num(imp.rows_total)
  const encoding = imp.encoding ? text.encoding[imp.encoding] : ""
  const delimiter = imp.delimiter ? text.delimiter[imp.delimiter] : ""
  return (
    <p className="text-small text-faint" data-testid="import-file-line">
      {encoding && delimiter ? (
        <Trans>
          {rows} rows, read as {encoding}, separated by {delimiter}.
        </Trans>
      ) : (
        <Trans>{rows} rows.</Trans>
      )}
    </p>
  )
}

function Review({
  list,
  imp,
  onBack,
  onCancel,
  cancelling,
}: {
  list: MailingList
  imp: ListImport
  onBack?: () => void
  onCancel?: () => void
  cancelling: boolean
}) {
  const start = useStartImport(list.id, imp.id)
  const c = imp.counts
  const valid = c.valid
  const out = refused(c)
  return (
    <div className="flex flex-col gap-5" data-testid="import-review">
      <Steps step={3} />
      <Section title={<Trans>What the import will do</Trans>}>
        <Card flush>
          <Rows>
            <CountRow
              label={
                imp.reconfirm ? (
                  <Trans>New addresses that get a confirmation mail</Trans>
                ) : (
                  <Trans>New subscribers added</Trans>
                )
              }
              value={valid}
              tone="text-success"
              testId="import-valid"
            />
            <CountRow
              label={<Trans>Already on the list, left as they are</Trans>}
              value={c.existing}
              testId="import-existing"
            />
            {out > 0 && (
              <li className="bg-surface px-4 py-2 text-small font-medium text-faint">
                <Plural value={out} one="# row is left out" other="# rows are left out" />
              </li>
            )}
            <Refusals counts={c} />
          </Rows>
        </Card>
        <FileLine imp={imp} />
      </Section>
      <Notice className="text-small">
        <p>
          {imp.reconfirm ? (
            <Trans>
              No welcome mail is sent. Each new address waits for confirmation and joins after clicking the link in the
              mail.
            </Trans>
          ) : (
            <Trans>
              No welcome or confirmation mail is sent. Every subscription keeps the consent you declared and the import
              it came from.
            </Trans>
          )}
        </p>
      </Notice>
      <ErrorLine error={start.error} />
      {onBack && onCancel ? (
        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onCancel} disabled={cancelling} data-testid="import-cancel">
            <Trans>Cancel import</Trans>
          </Button>
          <Button type="button" variant="outline" onClick={onBack} data-testid="import-review-back">
            <Trans>Back to columns</Trans>
          </Button>
          <Button
            type="button"
            onClick={() => start.mutate()}
            disabled={start.isPending || valid === 0}
            data-testid="import-start"
          >
            {start.isPending && <LoaderIcon className="animate-spin" />}
            <Plural value={valid} one="Import # address" other="Import # addresses" />
          </Button>
        </div>
      ) : (
        <p className="text-small text-faint">
          <Trans>Checked and waiting for an owner or admin to start it.</Trans>
        </p>
      )}
    </div>
  )
}

function Bar({ done, total }: { done: number; total: number }) {
  const share = total > 0 ? Math.min(1, done / total) : 0
  return (
    <span
      className="relative block h-2 w-full overflow-hidden rounded-full bg-muted"
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={total || undefined}
      aria-valuenow={done}
      data-testid="import-progress"
    >
      <span
        className={cn(
          "absolute inset-y-0 start-0 rounded-full bg-brand transition-[width] duration-700",
          total === 0 && "w-1/4 animate-pulse"
        )}
        style={total === 0 ? undefined : { width: `${share * 100}%` }}
      />
    </span>
  )
}

function Running({ imp, onCancel, cancelling }: { imp: ListImport; onCancel?: () => void; cancelling: boolean }) {
  const num = useNumber()
  const done = num(imp.rows_done)
  const total = num(imp.rows_total)
  const validating = imp.state === "validating"
  return (
    <div className="flex flex-col gap-5" data-testid="import-running">
      <Steps step={validating ? 2 : 4} />
      <Card>
        <div className="flex flex-col gap-3" role="status">
          <p className="flex items-center gap-2 text-body font-medium">
            <LoaderIcon className="size-4 animate-spin text-faint" />
            {validating ? <Trans>Checking every address…</Trans> : <Trans>Importing…</Trans>}
          </p>
          <Bar done={validating ? 0 : imp.rows_done} total={validating ? 0 : imp.rows_total} />
          <p className="text-small text-faint">
            {validating ? (
              <Trans>
                Nothing is added yet. The file is read, each domain is asked whether it takes mail, and duplicates and
                addresses that may not get mail are found.
              </Trans>
            ) : (
              <Trans>
                {done} of {total} rows
              </Trans>
            )}
          </p>
        </div>
        {!validating && (
          <div className="-mx-4 -mb-4 border-t">
            <Rows>
              <CountRow
                label={imp.reconfirm ? <Trans>Waiting for confirmation</Trans> : <Trans>Added</Trans>}
                value={imp.counts.added}
                tone="text-success"
              />
              <CountRow label={<Trans>Already on the list</Trans>} value={imp.counts.existing} />
              <CountRow label={<Trans>Left out</Trans>} value={refused(imp.counts)} tone="text-muted-foreground" />
            </Rows>
          </div>
        )}
      </Card>
      <p className="text-small text-faint">
        <Trans>You can leave this page; the import goes on.</Trans>
      </p>
      {onCancel && (
        <div className="flex justify-end">
          <Button type="button" variant="outline" onClick={onCancel} disabled={cancelling} data-testid="import-cancel">
            <XIcon />
            <Trans>Cancel import</Trans>
          </Button>
        </div>
      )}
    </div>
  )
}

function ErrorsDownload({ list, imp }: { list: MailingList; imp: ListImport }) {
  const { i18n } = useLingui()
  const [now] = useState(() => Date.now())
  const save = useMutation({ mutationFn: () => downloadErrors(list.id, imp) })
  if (refused(imp.counts) === 0) return null
  const live = !imp.errors_until || Date.parse(imp.errors_until) > now
  const until = imp.errors_until ? formatDate(imp.errors_until, i18n.locale) : ""
  return (
    <div className="flex flex-col gap-2">
      {live ? (
        <>
          <div className="flex flex-wrap items-center gap-3">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => save.mutate()}
              disabled={save.isPending}
              data-testid="import-errors"
            >
              <DownloadIcon />
              <Trans>Download the rows left out</Trans>
            </Button>
            {until && (
              <span className="text-small text-faint">
                <Trans>Available until {until}</Trans>
              </span>
            )}
          </div>
          <ErrorLine error={save.error} className="text-small" />
        </>
      ) : (
        <p className="text-small text-faint">
          <Trans>The rows left out are no longer kept.</Trans>
        </p>
      )}
    </div>
  )
}

function Result({ list, imp, onAnother }: { list: MailingList; imp: ListImport; onAnother?: () => void }) {
  const { i18n } = useLingui()
  const c = imp.counts
  const finished = imp.finished_at ? formatDateTime(imp.finished_at, i18n.locale) : ""
  const failed = imp.state === "failed"
  const reason = imp.error ?? ""
  return (
    <div className="flex flex-col gap-5" data-testid="import-result">
      {failed ? (
        <Notice tone="danger" icon={TriangleAlertIcon} role="alert">
          <p className="font-medium">
            <Trans>The import failed, and nothing was added from the file.</Trans>
          </p>
          {reason && <p className="text-small text-muted-foreground">{reason}</p>}
        </Notice>
      ) : (
        <Section
          title={
            imp.state === "cancelled" ? (
              <Trans>Cancelled. What was added before stays.</Trans>
            ) : (
              <Trans>Import finished</Trans>
            )
          }
        >
          <Card flush>
            <Rows>
              <CountRow
                label={imp.reconfirm ? <Trans>Waiting for confirmation</Trans> : <Trans>Subscribers added</Trans>}
                value={c.added}
                tone="text-success"
                testId="import-added"
              />
              <CountRow label={<Trans>New contacts created</Trans>} value={c.created_contacts} />
              <CountRow label={<Trans>Already on the list</Trans>} value={c.existing} />
              <Refusals counts={c} />
            </Rows>
          </Card>
          {finished && (
            <p className="mx-1 text-small text-faint">
              <Trans>Ended {finished}</Trans>
            </p>
          )}
        </Section>
      )}
      <ErrorsDownload list={list} imp={imp} />
      {onAnother && (
        <div className="flex justify-end">
          <Button type="button" onClick={onAnother} data-testid="import-another">
            <FileUpIcon />
            <Trans>Import another file</Trans>
          </Button>
        </div>
      )}
    </div>
  )
}

function ImportView({
  list,
  id,
  onDone,
  onNew,
}: {
  list: MailingList
  id: string
  onDone: () => void
  onNew: () => void
}) {
  const { canManage } = useSession()
  const imp = useListImport(list.id, id)
  const cancel = useCancelImport(list.id, id)
  const [confirm, confirmDialog] = useConfirm()
  const [step, setStep] = useState<"columns" | "review">("columns")
  const i = imp.data
  const askCancel = () =>
    confirm({
      title: <Trans>Cancel this import?</Trans>,
      description:
        i?.state === "importing" ? (
          <Trans>It stops within a moment. Subscribers it already added stay on the list.</Trans>
        ) : (
          <Trans>Nothing from the file is added, and the uploaded file is deleted.</Trans>
        ),
      confirm: <Trans>Cancel import</Trans>,
      run: () => cancel.mutate(),
    })
  let body: React.ReactNode
  if (!i) {
    body = imp.isPending ? <Skeleton className="h-40 w-full rounded-2xl" /> : <ErrorLine error={imp.error} />
  } else if (isRunning(i.state)) {
    body = <Running imp={i} onCancel={canManage ? askCancel : undefined} cancelling={cancel.isPending} />
  } else if (i.state === "ready" && !canManage) {
    body = <Review list={list} imp={i} cancelling={false} />
  } else if (i.state === "ready") {
    body =
      step === "columns" ? (
        <Columns
          key={i.updated_at}
          list={list}
          imp={i}
          onNext={() => setStep("review")}
          onCancel={askCancel}
          cancelling={cancel.isPending}
        />
      ) : (
        <Review
          list={list}
          imp={i}
          onBack={() => setStep("columns")}
          onCancel={askCancel}
          cancelling={cancel.isPending}
        />
      )
  } else {
    body = (
      <Result list={list} imp={i} onAnother={canManage && !list.archived_at && !isOpen(i.state) ? onNew : undefined} />
    )
  }
  return (
    <div className="flex flex-col gap-4" data-testid="import-view">
      <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
        <button
          type="button"
          onClick={onDone}
          className="text-body text-faint transition-colors hover:text-foreground"
          data-testid="import-back"
        >
          <Trans>All imports</Trans>
        </button>
        <ChevronRightIcon className="size-3.5 text-faint rtl:rotate-180" aria-hidden />
        <span className="min-w-0 truncate text-body font-medium" dir="auto">
          {i?.filename}
        </span>
        {i && <ImportState state={i.state} />}
      </div>
      {body}
      <ErrorLine error={cancel.error} />
      {confirmDialog}
    </div>
  )
}

function History({ list, onOpen }: { list: MailingList; onOpen: (id: string) => void }) {
  const { i18n } = useLingui()
  const num = useNumber()
  const imports = useListImports(list.id)
  const items = imports.data?.pages.flatMap((p) => p.items) ?? []
  const [shown, setShown] = useState(true)
  return (
    <Section
      title={
        <button
          type="button"
          onClick={() => setShown((v) => !v)}
          aria-expanded={shown}
          className="inline-flex items-center gap-1 outline-none hover:text-foreground focus-visible:underline"
          data-testid="import-history-toggle"
        >
          <ChevronRightIcon className={cn("size-3.5 rtl:rotate-180", shown && "rotate-90 rtl:rotate-90")} />
          <Trans>All imports</Trans>
        </button>
      }
    >
      {shown && (
        <Card flush>
          {imports.isPending ? (
            <div className="flex flex-col gap-2 p-4">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : items.length === 0 ? (
            <EmptyRow>
              <Trans>No imports yet.</Trans>
            </EmptyRow>
          ) : (
            <Rows data-testid="import-history">
              {items.map((i) => {
                const added = num(i.counts.added)
                const out = num(refused(i.counts))
                return (
                  <li key={i.id}>
                    <Item
                      render={<button type="button" />}
                      className="flex-nowrap gap-3 rounded-none px-4 py-2.5 [button]:hover:bg-muted"
                      onClick={() => onOpen(i.id)}
                      data-testid="import-row"
                    >
                      <FileSpreadsheetIcon className="size-4 shrink-0 text-faint" />
                      <span className="flex min-w-0 flex-1 flex-col text-start">
                        <span className="truncate text-body font-medium" dir="auto">
                          {i.filename}
                        </span>
                        <span className="flex flex-wrap items-center gap-x-2 text-small text-faint">
                          <time dateTime={i.created_at} title={formatDateTime(i.created_at, i18n.locale)}>
                            {formatDate(i.created_at, i18n.locale)}
                          </time>
                          {(i.state === "done" || i.state === "cancelled" || i.state === "importing") && (
                            <span>
                              <Trans>
                                {added} added · {out} left out
                              </Trans>
                            </span>
                          )}
                        </span>
                      </span>
                      <ImportState state={i.state} />
                      <ChevronRightIcon className="size-4 shrink-0 text-faint rtl:rotate-180" />
                    </Item>
                  </li>
                )
              })}
            </Rows>
          )}
          {imports.hasNextPage && (
            <div className="flex justify-center border-t p-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => void imports.fetchNextPage()}
                disabled={imports.isFetchingNextPage}
              >
                <Trans>Load more</Trans>
              </Button>
            </div>
          )}
          <ErrorLine error={imports.error} className="px-4 pb-3" />
        </Card>
      )}
    </Section>
  )
}

export function ListImports({ list }: { list: MailingList }) {
  const { canManage } = useSession()
  const [current, go] = useImportParam()
  const canImport = canManage && !list.archived_at
  if (current && current !== "new") {
    return <ImportView key={current} list={list} id={current} onDone={() => go(null)} onNew={() => go(null)} />
  }
  return (
    <>
      <Section title={<Trans>Import from a CSV file</Trans>}>
        <p className="mx-1 max-w-2xl text-small text-faint">
          <Trans>
            Bring subscribers who already agreed elsewhere. Each address is checked first and nothing is added until you
            start: invalid addresses, domains that take no mail, role addresses, duplicates and anyone who left or
            objected are left out.
          </Trans>
        </p>
        {canImport && <Upload list={list} onCreated={(i) => go(i.id)} />}
      </Section>
      <History list={list} onOpen={go} />
    </>
  )
}

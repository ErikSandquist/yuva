import { Trans, useLingui } from "@lingui/react/macro"
import {
  ArrowRightIcon,
  ArrowUpRightIcon,
  CheckIcon,
  FileArchiveIcon,
  FileUpIcon,
  HistoryIcon,
  LoaderIcon,
  PackageOpenIcon,
  TriangleAlertIcon,
} from "lucide-react"
import { useRef, useState } from "react"
import { Link } from "react-router"

import { ErrorLine, Notice, ProgressBar } from "@/components/common"
import { formatBytes } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import { useNumber } from "@/features/broadcasts/BroadcastParts"
import { useMigrationUpload } from "@/features/migrations/queries"
import { guideUrl, useFormatText, useGuides } from "@/features/migrations/text"
import {
  remembered,
  useExportUpload,
  type LocalProblem,
  type PartState,
  type Upload,
} from "@/features/migrations/upload"
import { WizardLayout } from "@/features/migrations/Wizard"
import { StepHeader } from "@/features/setup/SetupLayout"
import { ApiError, type Migration, type MigrationPreset } from "@/lib/api"
import { cn } from "@/lib/utils"

const STRIP_MAX = 256

export function UploadStep({ m, preset }: { m: Migration; preset: MigrationPreset }) {
  const { i18n } = useLingui()
  const guide = useGuides()[m.preset]
  const formatText = useFormatText()
  const takeout = m.preset === "takeout"
  const canUpload = m.state === "draft" && m.actions.includes("upload")
  const server = useMigrationUpload(m.id, canUpload)
  const upload = useExportUpload(m)
  const input = useRef<HTMLInputElement>(null)
  const [over, setOver] = useState(false)
  const [mboxo, setMboxo] = useState(false)
  const { state, busy } = upload
  const address = m.source_address
  const file = m.upload?.completed_at ? m.upload : undefined

  const pick = (f: File | undefined) => {
    if (!f || busy) return
    void upload.start(f, { mbox: mboxo ? "mboxo" : "mboxrd" })
  }

  const stored = server.data && !server.data.completed_at ? server.data : undefined
  const cutOff = !busy && state.phase !== "done" && !!stored && stored.parts.length > 0 && !state.problem
  const before = cutOff ? remembered(m.id) : null
  const refused = state.error instanceof ApiError && state.error.code === "file_refused"

  return (
    <WizardLayout step="connect" m={m} preset={preset}>
      <StepHeader title={<Trans>Upload the export of {address}</Trans>}>
        {takeout ? (
          <Trans>Upload the .mbox file from Google Takeout. Yuva needs no access to the Google account.</Trans>
        ) : (
          <Trans>
            Upload an .mbox file, or a .zip of .eml files or of a Maildir. Yuva needs no access to the mailbox.
          </Trans>
        )}
      </StepHeader>

      <section className="mb-6 rounded-2xl border bg-card p-4" aria-labelledby="migration-guide">
        <h2 id="migration-guide" className="flex items-center gap-2 text-body font-medium">
          <PackageOpenIcon className="size-4 text-faint" />
          <Trans>Get the file</Trans>
        </h2>
        <ol className="mt-3 flex list-decimal flex-col gap-2 ps-5 text-body text-muted-foreground marker:text-faint">
          {guide.steps.map((s) => (
            <li key={s}>{s}</li>
          ))}
        </ol>
        <div className="mt-4 flex flex-wrap gap-2">
          <Button
            variant="outline"
            size="sm"
            render={<a href={guide.link} target="_blank" rel="noreferrer" />}
            data-testid="migration-guide-link"
          >
            {guide.linkLabel}
            <ArrowUpRightIcon />
          </Button>
          {guide.link !== guideUrl(preset.guide) && (
            <Button
              variant="ghost"
              size="sm"
              render={<a href={guideUrl(preset.guide)} target="_blank" rel="noreferrer" />}
            >
              <Trans>Full guide</Trans>
            </Button>
          )}
        </div>
      </section>

      {file ? (
        <Notice icon={CheckIcon} data-testid="migration-file-ready">
          <p className="font-medium break-all text-foreground" dir="auto">
            {file.filename ?? <Trans>The file</Trans>}
            {" · "}
            {formatBytes(file.size_bytes, i18n.locale)}
          </p>
          {file.format && <p data-testid="migration-file-format">{formatText[file.format]}</p>}
          <p>
            <Trans>Next, choose what to bring. The scan reads the file once.</Trans>
          </p>
          <Button
            size="sm"
            className="self-start"
            render={<Link to={`/migrations/${m.id}/choose`} />}
            data-testid="migration-upload-continue"
          >
            <Trans>Continue</Trans>
            <ArrowRightIcon className="rtl:rotate-180" />
          </Button>
        </Notice>
      ) : canUpload ? (
        <div className="flex flex-col gap-5">
          {cutOff && stored && (
            <Notice icon={HistoryIcon} data-testid="migration-upload-resume">
              <p className="font-medium text-foreground">
                <ResumeLine parts={stored.parts.length} bytes={stored.received_bytes} />
              </p>
              <p>
                {before ? (
                  <RememberedLine name={before.name} size={before.size} />
                ) : (
                  <Trans>Pick the same file again to send only what is missing.</Trans>
                )}
              </p>
            </Notice>
          )}

          {state.phase === "idle" && (
            <>
              <input
                ref={input}
                type="file"
                className="hidden"
                onChange={(e) => {
                  pick(e.target.files?.[0])
                  e.target.value = ""
                }}
                data-testid="migration-file-input"
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
                  "flex min-w-0 items-center gap-3 rounded-2xl border-2 border-dashed px-4 py-6 text-start text-body text-muted-foreground transition-colors outline-none hover:border-faint hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/30",
                  over && "border-brand bg-brand-wash text-brand"
                )}
                data-testid="migration-file-drop"
              >
                <FileUpIcon className="size-5 shrink-0" />
                <span className="flex min-w-0 flex-col">
                  <span className="font-medium text-foreground">
                    {cutOff ? <Trans>Pick the same file again</Trans> : <Trans>Pick the file, or drop it here</Trans>}
                  </span>
                  <span className="text-small text-faint">
                    {takeout ? (
                      <Trans>The .mbox file from Takeout › Mail, unzipped.</Trans>
                    ) : (
                      <Trans>
                        An .mbox file, or a .zip of .eml files, of a Maildir or of uncompressed .mbox files.
                      </Trans>
                    )}
                  </span>
                </span>
              </button>
              {!takeout && (
                <details className="text-small text-muted-foreground">
                  <summary className="cursor-pointer text-faint">
                    <Trans>mbox options</Trans>
                  </summary>
                  <Label className="mt-2.5 items-start gap-3 font-normal">
                    <Checkbox
                      checked={mboxo}
                      onCheckedChange={setMboxo}
                      className="mt-0.5"
                      data-testid="migration-mboxo"
                    />
                    <span className="flex flex-col gap-0.5">
                      <span className="text-body text-foreground">
                        <Trans>The file is an older mboxo file</Trans>
                      </span>
                      <span>
                        <Trans>
                          Only for mbox files from older programs that escape “From ” lines the old way. Google Takeout
                          and most apps write mboxrd, the default.
                        </Trans>
                      </span>
                    </span>
                  </Label>
                </details>
              )}
            </>
          )}

          {state.phase !== "idle" && state.phase !== "done" && state.file && (
            <Progress state={state} onStop={upload.stop} />
          )}

          {state.phase === "idle" && state.problem && (
            <ProblemNotice
              problem={state.problem}
              onFresh={state.file ? () => void upload.start(state.file!, { fresh: true }) : undefined}
            />
          )}
          {state.phase === "idle" && state.error !== null && (
            <div className="flex flex-col gap-2" data-testid="migration-upload-error">
              <ErrorLine error={state.error} />
              {refused ? (
                <p className="text-small text-faint">
                  <Trans>The file's parts were deleted. Pick a corrected file to upload.</Trans>
                </p>
              ) : (
                state.file && (
                  <Button
                    variant="outline"
                    size="sm"
                    className="self-start"
                    onClick={() => void upload.start(state.file!)}
                    data-testid="migration-upload-retry"
                  >
                    <Trans>Try again</Trans>
                  </Button>
                )
              )}
            </div>
          )}
          <ErrorLine error={server.error} />
        </div>
      ) : null}
    </WizardLayout>
  )
}

function ResumeLine({ parts, bytes }: { parts: number; bytes: number }) {
  const { i18n } = useLingui()
  const num = useNumber()
  const count = num(parts)
  const size = formatBytes(bytes, i18n.locale)
  return (
    <Trans>
      An upload was cut off: {count} parts, {size}, are already on the server.
    </Trans>
  )
}

function RememberedLine({ name, size }: { name: string; size: number }) {
  const { i18n } = useLingui()
  const total = formatBytes(size, i18n.locale)
  return (
    <Trans>
      It was {name} ({total}). Pick it again to send only what is missing.
    </Trans>
  )
}

function ProblemNotice({ problem, onFresh }: { problem: LocalProblem; onFresh?: () => void }) {
  const { i18n } = useLingui()
  const num = useNumber()
  if (problem.kind === "other_file") {
    const count = num(problem.stored)
    return (
      <Notice tone="warning" icon={TriangleAlertIcon} data-testid="migration-upload-other-file">
        <p>
          <Trans>
            This is not the file whose {count} parts are on the server. Pick that file again to continue, or send this
            one from the start.
          </Trans>
        </p>
        {onFresh && (
          <Button size="sm" variant="outline" className="self-start" onClick={onFresh}>
            <Trans>Send this file from the start</Trans>
          </Button>
        )}
      </Notice>
    )
  }
  const size = problem.kind === "too_large" ? formatBytes(problem.size, i18n.locale) : ""
  const max = problem.kind === "too_large" ? formatBytes(problem.max, i18n.locale) : ""
  return (
    <p role="alert" className="text-body text-destructive" data-testid="migration-upload-problem">
      {problem.kind === "too_large" ? (
        <Trans>
          The file is {size}; this server takes export files of up to {max}. Split the export, or ask the server's
          operator.
        </Trans>
      ) : problem.kind === "empty" ? (
        <Trans>The file is empty.</Trans>
      ) : problem.kind === "archive" ? (
        <Trans>This is a compressed archive. Unzip the Takeout archive and upload the .mbox file.</Trans>
      ) : (
        <Trans>This migration has ended, so nothing more can be uploaded. Start a new one for another file.</Trans>
      )}
    </p>
  )
}

function Progress({ state, onStop }: { state: Upload; onStop: () => void }) {
  const { t, i18n } = useLingui()
  const num = useNumber()
  const file = state.file!
  const size = formatBytes(file.size, i18n.locale)
  const sent = formatBytes(state.doneBytes, i18n.locale)
  const percent = file.size > 0 ? Math.floor((state.doneBytes / file.size) * 100) : 0
  const doneParts = num(state.done.size)
  const totalParts = num(state.total)
  const checking = state.phase === "checking"
  const completing = state.phase === "completing"
  const name = file.name
  const announced = useAnnouncedPercent(checking ? null : percent)
  const partText: Record<PartState, string> = {
    hashing: t`Checking`,
    sending: t`Sending`,
    retrying: t`Trying again`,
  }
  const active = [...state.active.entries()].sort((a, b) => a[0] - b[0])
  return (
    <section
      className="flex flex-col gap-3 rounded-2xl border p-4"
      aria-labelledby="migration-upload-title"
      data-testid="migration-upload"
      data-phase={state.phase}
    >
      <div className="flex min-w-0 items-center gap-3">
        <FileArchiveIcon className="size-5 shrink-0 text-faint" />
        <span className="min-w-0 flex-1">
          <span id="migration-upload-title" className="block truncate font-medium" dir="auto">
            {name}
          </span>
          <span className="block text-small text-faint">{size}</span>
        </span>
        {!completing && (
          <Button variant="outline" size="sm" onClick={onStop} data-testid="migration-upload-stop">
            <Trans>Stop</Trans>
          </Button>
        )}
      </div>
      <ProgressBar
        value={checking ? undefined : percent}
        label={t`Upload of ${name}`}
        text={checking ? t`Checking what is on the server` : t`${percent}%, ${sent} of ${size}`}
        testId="migration-upload-progress"
      />
      <p className="flex flex-wrap justify-between gap-x-3 text-small text-faint tabular-nums">
        <span>
          {checking ? (
            <Trans>Checking what is already on the server…</Trans>
          ) : completing ? (
            <Trans>All parts arrived. Reading the file…</Trans>
          ) : (
            <Trans>
              {percent}% · {sent} of {size}
            </Trans>
          )}
        </span>
        {!checking && (
          <span>
            <Trans>
              {doneParts} of {totalParts} parts
            </Trans>
          </span>
        )}
      </p>
      <p className="sr-only" role="status" aria-live="polite">
        {completing ? t`All parts arrived. Reading the file…` : announced}
      </p>
      {state.total > 1 && state.total <= STRIP_MAX && (
        <div className="flex flex-wrap gap-0.5" aria-hidden="true" data-testid="migration-upload-parts">
          {Array.from({ length: state.total }, (_, n) => (
            <span
              key={n}
              className={cn(
                "h-2 w-3 rounded-[2px] bg-muted",
                state.done.has(n) && "bg-brand",
                state.active.has(n) && "animate-pulse bg-brand/40",
                state.active.get(n) === "retrying" && "bg-warning"
              )}
            />
          ))}
        </div>
      )}
      {active.length > 0 && (
        <ul className="flex flex-col divide-y rounded-xl border text-small" aria-label={t`Parts in progress`}>
          {active.map(([n, st]) => {
            const part = num(n + 1)
            return (
              <li key={n} className="flex items-center gap-2.5 px-3 py-2" data-state={st}>
                {st === "retrying" ? (
                  <TriangleAlertIcon className="size-3.5 shrink-0 text-warning" aria-hidden="true" />
                ) : (
                  <LoaderIcon className="size-3.5 shrink-0 animate-spin text-faint" aria-hidden="true" />
                )}
                <span className="min-w-0 flex-1">
                  <Trans>
                    Part {part} of {totalParts}
                  </Trans>
                </span>
                <span className={cn("shrink-0", st === "retrying" ? "text-warning" : "text-faint")}>
                  {partText[st]}
                </span>
              </li>
            )
          })}
        </ul>
      )}
      <p className="text-small text-faint">
        <Trans>
          Keep this page open. If the connection drops, the upload goes on when it is back; after a reload, pick the
          same file and only the missing parts are sent.
        </Trans>
      </p>
    </section>
  )
}

function useAnnouncedPercent(percent: number | null) {
  const { t } = useLingui()
  if (percent === null) return ""
  const step = Math.floor(percent / 10) * 10
  return t`Uploaded ${step}%`
}

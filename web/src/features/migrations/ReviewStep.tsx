import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { ArrowRightIcon, LockIcon, PaperclipIcon, ScanSearchIcon, UserRoundIcon } from "lucide-react"
import { Link, useNavigate } from "react-router"

import { ContactAvatar, ErrorLine, MemberAvatar, PersonAvatar, ProgressBar, StepHeader } from "@/components/common"
import { formatBytes, formatDate, formatDuration, formatShort, useEnumText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useNumber } from "@/features/broadcasts/BroadcastParts"
import { Figure, SkippedList, WaitingLine } from "@/features/migrations/parts"
import { useMigrationPreview, useMigrationStep } from "@/features/migrations/queries"
import { isHelpdesk } from "@/features/migrations/text"
import { WizardLayout } from "@/features/migrations/Wizard"
import { StatusTag } from "@/features/settings/ui"
import type { Migration, MigrationEstimate, MigrationPreset, MigrationPreviewThread } from "@/lib/api"
import { cn } from "@/lib/utils"
import { useMemberMap } from "@/lib/workspace"

export function useTimeRange() {
  const { t, i18n } = useLingui()
  return (min: number, max: number) => {
    const from = formatDuration(Math.max(60, min), i18n.locale)
    const to = formatDuration(Math.max(60, max), i18n.locale)
    return from === to ? t`About ${from}` : t`About ${from} to ${to}`
  }
}

export function ReviewStep({ m, preset }: { m: Migration; preset: MigrationPreset }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const start = useMigrationStep(m.id, "start")
  const est = m.estimate
  const scanning = m.state === "scanning"
  const address = m.source_address
  const desk = isHelpdesk(m.source)
  const provider = preset.name
  return (
    <WizardLayout step="review" m={m} preset={preset}>
      <StepHeader title={<Trans>Check before anything is copied</Trans>}>
        {desk ? (
          <Trans>This is the dry run for {provider}. Nothing is imported until you start.</Trans>
        ) : (
          <Trans>This is the dry run for {address}. Nothing is imported until you start.</Trans>
        )}
      </StepHeader>

      <div className="flex flex-col gap-8">
        {scanning && (
          <div className="flex flex-col gap-3 rounded-2xl border p-4" data-testid="migration-scanning">
            <p role="status" aria-live="polite" className="flex items-center gap-2 text-body font-medium">
              <ScanSearchIcon className="size-4 text-faint" />
              <Trans>Planning the import with your choices…</Trans>
            </p>
            <ProgressBar label={t`Scan`} text={t`Scanning`} />
            <WaitingLine m={m} />
          </div>
        )}

        {est ? <DryRun est={est} desk={desk} /> : !scanning && <Skeleton className="h-48 rounded-xl" />}

        {est && <Preview m={m} />}

        <ErrorLine error={start.error} />
        <div className="flex flex-wrap items-center gap-3">
          <Button
            size="lg"
            onClick={() => start.mutate(undefined, { onSuccess: () => navigate(`/migrations/${m.id}/copy`) })}
            disabled={scanning || !est || !m.actions.includes("start") || start.isPending}
            data-testid="migration-start"
          >
            <Trans>Start migration</Trans>
            <ArrowRightIcon className="rtl:rotate-180" />
          </Button>
          {m.actions.includes("update") && (
            <Button variant="ghost" size="lg" render={<Link to={`/migrations/${m.id}/choose`} />}>
              <Trans>Change the choice</Trans>
            </Button>
          )}
        </div>
        <p className="text-small text-faint">
          <Trans>
            History arrives quietly, newest first: no notifications, no auto-replies, nothing unread. Your team can work
            meanwhile.
          </Trans>
        </p>
      </div>
    </WizardLayout>
  )
}

function DryRun({ est, desk }: { est: MigrationEstimate; desk: boolean }) {
  const { i18n } = useLingui()
  const num = useNumber()
  const range = useTimeRange()
  const incoming = num(est.messages_in)
  const outgoing = num(est.messages_out)
  const existing = num(est.contacts_existing)
  const created = est.labels.filter((l) => l.action === "create").length
  const attachments = num(est.attachments)
  const attachmentSize = formatBytes(est.attachment_bytes, i18n.locale)
  const attachmentsLeft = num(est.attachments_skipped)
  const oldest = est.oldest_at ? formatDate(est.oldest_at, i18n.locale) : ""
  const newest = est.newest_at ? formatDate(est.newest_at, i18n.locale) : ""
  return (
    <>
      <section className="flex flex-col gap-3" aria-labelledby="migration-totals">
        <h2 id="migration-totals" className="text-title">
          <Trans>What comes over</Trans>
        </h2>
        <div className="grid grid-cols-2 gap-2.5" data-testid="migration-totals">
          <Figure
            value={num(est.conversations)}
            label={<Trans>Conversations</Trans>}
            detail={desk ? <Trans>Each with all its messages and notes</Trans> : undefined}
          />
          <Figure
            value={num(est.messages)}
            label={<Trans>Messages</Trans>}
            detail={
              <Trans>
                {incoming} received, {outgoing} sent
              </Trans>
            }
          />
          <Figure
            value={num(est.contacts_new)}
            label={<Trans>New contacts</Trans>}
            detail={<Trans>{existing} already in Yuva</Trans>}
          />
          <Figure
            value={num(est.labels.length)}
            label={<Trans>Labels</Trans>}
            detail={<Plural value={created} one="# new" other="# new" />}
          />
        </div>
        <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-body">
          {oldest && newest && (
            <>
              <dt className="text-faint">
                <Trans>Dates</Trans>
              </dt>
              <dd className="text-end">
                {oldest} – {newest}
              </dd>
            </>
          )}
          <dt className="text-faint">
            <Trans>Attachments</Trans>
          </dt>
          <dd className="text-end">
            <Trans>
              {attachments} files, {attachmentSize}
            </Trans>
          </dd>
          <dt className="text-faint">
            <Trans>Storage</Trans>
          </dt>
          <dd className="text-end">{formatBytes(est.storage_bytes, i18n.locale)}</dd>
          <dt className="text-faint">
            <Trans>Time</Trans>
          </dt>
          <dd className="text-end" data-testid="migration-time">
            {range(est.seconds_min, est.seconds_max)}
          </dd>
        </dl>
        {est.attachments_skipped > 0 && (
          <p className="text-small text-faint">
            <Trans>
              {attachmentsLeft} attachments are over this server's size limit or of a type it does not keep; they stay
              in the original message.
            </Trans>
          </p>
        )}
      </section>

      {est.labels.length > 0 && (
        <section className="flex flex-col gap-3" aria-labelledby="migration-labels">
          <h2 id="migration-labels" className="text-title">
            <Trans>Labels</Trans>
          </h2>
          <ul className="flex flex-col divide-y rounded-xl border">
            {est.labels.map((l) => {
              return (
                <li key={l.name} className="flex items-baseline justify-between gap-3 px-3.5 py-2 text-body">
                  <span className="min-w-0 truncate">
                    {l.name}
                    {l.action === "create" && (
                      <span className="ms-2 text-caption text-faint">
                        <Trans>new</Trans>
                      </span>
                    )}
                  </span>
                  <span className="shrink-0 text-faint tabular-nums">
                    <Plural value={l.conversations} one="# conversation" other="# conversations" />
                  </span>
                </li>
              )
            })}
          </ul>
        </section>
      )}

      <section className="flex flex-col gap-3" aria-labelledby="migration-not-imported">
        <h2 id="migration-not-imported" className="text-title">
          <Trans>Not imported</Trans>
        </h2>
        <SkippedList skipped={est.skipped} />
      </section>
    </>
  )
}

function Preview({ m }: { m: Migration }) {
  const preview = useMigrationPreview(m)
  return (
    <section className="flex flex-col gap-3" aria-labelledby="migration-preview">
      <h2 id="migration-preview" className="text-title">
        <Trans>How the newest conversations will look</Trans>
      </h2>
      {preview.isPending ? (
        <Skeleton className="h-40 rounded-xl" />
      ) : (
        <ul className="flex flex-col gap-3" data-testid="migration-preview">
          {(preview.data ?? []).map((thread, i) => (
            <PreviewThread key={i} thread={thread} />
          ))}
        </ul>
      )}
      <ErrorLine error={preview.error} />
    </section>
  )
}

const SHOWN = 3

function PreviewThread({ thread }: { thread: MigrationPreviewThread }) {
  const { i18n } = useLingui()
  const enums = useEnumText()
  const members = useMemberMap()
  const messages = thread.messages.slice(-SHOWN)
  const hidden = thread.messages_total - messages.length
  const who = thread.contact.name || thread.contact.address
  const assigneeMember = thread.assignee_member_id ? members.get(thread.assignee_member_id) : undefined
  const assignee = assigneeMember
    ? assigneeMember.name || assigneeMember.email
    : thread.assignee
      ? thread.assignee.name || thread.assignee.address
      : ""
  return (
    <li className="overflow-hidden rounded-xl border bg-card">
      <div className="flex flex-col gap-1 border-b px-3.5 py-3">
        <div className="flex items-center gap-2">
          <ContactAvatar id={thread.contact.address} name={who} className="size-6 text-caption" />
          <span className="min-w-0 flex-1 truncate text-body font-medium">{who}</span>
          <StatusTag tone={thread.status === "closed" ? "muted" : "warning"}>{enums.status[thread.status]}</StatusTag>
        </div>
        <p className="truncate text-body">{thread.subject}</p>
        {assignee && (
          <p className="flex items-center gap-1.5 text-caption text-faint" data-testid="migration-preview-assignee">
            <UserRoundIcon className="size-3 shrink-0" />
            <span className="truncate">
              {assigneeMember ? (
                <Trans>Assigned to {assignee}</Trans>
              ) : (
                <Trans>Assigned to {assignee}, kept by name</Trans>
              )}
            </span>
          </p>
        )}
        {thread.labels.length > 0 && <p className="truncate text-caption text-faint">{thread.labels.join(" · ")}</p>}
      </div>
      {hidden > 0 && (
        <p className="border-b px-3.5 py-1.5 text-caption text-faint">
          <Plural value={hidden} one="# earlier message" other="# earlier messages" />
        </p>
      )}
      <ol className="flex flex-col gap-2 px-3.5 py-3">
        {messages.map((msg, i) => {
          const member = msg.member_id ? members.get(msg.member_id) : undefined
          const author = member ? member.name || member.email : msg.from.name || msg.from.address
          const out = msg.direction === "out" || !!msg.note
          return (
            <li key={i} className={cn("flex gap-2", out && "flex-row-reverse")}>
              {out ? (
                member ? (
                  <MemberAvatar name={author} className="size-6 text-caption" />
                ) : (
                  <PersonAvatar name={author} className="size-6 text-caption" />
                )
              ) : (
                <ContactAvatar id={msg.from.address} name={author} className="size-6 text-caption" />
              )}
              <div
                className={cn(
                  "flex max-w-[85%] min-w-0 flex-col gap-0.5 rounded-xl px-3 py-2",
                  msg.note ? "border border-note-border bg-note" : out ? "bg-brand-wash" : "bg-surface"
                )}
                data-note={msg.note || undefined}
              >
                <span className="flex items-baseline gap-2 text-caption text-faint">
                  <span className="truncate font-medium text-muted-foreground">{author}</span>
                  <span className="shrink-0">{formatShort(msg.at, i18n.locale)}</span>
                </span>
                <p className="line-clamp-4 text-body break-words whitespace-pre-line">{msg.text}</p>
                {msg.note && (
                  <span className="flex items-center gap-1 text-caption text-note-ink">
                    <LockIcon className="size-3" />
                    <Trans>Note, only the team sees it</Trans>
                  </span>
                )}
                {msg.attachments > 0 && (
                  <span className="flex items-center gap-1 text-caption text-faint">
                    <PaperclipIcon className="size-3" />
                    <Plural value={msg.attachments} one="# attachment" other="# attachments" />
                  </span>
                )}
              </div>
            </li>
          )
        })}
      </ol>
    </li>
  )
}

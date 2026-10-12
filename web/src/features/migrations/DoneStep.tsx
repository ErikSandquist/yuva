import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { ArrowUpRightIcon, CircleCheckIcon, InboxIcon, ShieldCheckIcon } from "lucide-react"
import { useNavigate } from "react-router"

import { ErrorLine, ProgressBar } from "@/components/common"
import { formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { useNumber } from "@/features/broadcasts/BroadcastParts"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { Figure, SkippedList, useMigrationConfirms, useStateAnnouncement } from "@/features/migrations/parts"
import { useGuides } from "@/features/migrations/text"
import { WizardLayout } from "@/features/migrations/Wizard"
import { StepHeader } from "@/features/setup/SetupLayout"
import type { Migration, MigrationPreset } from "@/lib/api"

export function DoneStep({ m, preset }: { m: Migration; preset: MigrationPreset }) {
  const { t, i18n } = useLingui()
  const num = useNumber()
  const navigate = useNavigate()
  const [, setInbox] = useInboxFilter()
  const guide = useGuides()[m.preset]
  const ask = useMigrationConfirms(m)
  const announcement = useStateAnnouncement([m])
  const address = m.source_address
  const provider = preset.name
  const c = m.counts
  const est = m.estimate
  const removing = m.state === "removing"
  const removed = m.state === "removed"
  const completed = m.state === "completed"
  const deletedAt = m.credentials_deleted_at ? formatDateTime(m.credentials_deleted_at, i18n.locale) : ""
  const removable = m.removable_conversations ?? 0
  const openInbox = () => {
    if (m.inbox_id) setInbox(m.inbox_id)
    navigate("/")
  }
  return (
    <WizardLayout step="done" m={m} preset={preset}>
      <StepHeader
        icon={completed ? <CircleCheckIcon /> : undefined}
        title={
          completed ? (
            <Trans>{address} is in Yuva</Trans>
          ) : removing ? (
            <Trans>Removing imported history…</Trans>
          ) : removed ? (
            <Trans>Imported history removed</Trans>
          ) : (
            <Trans>Migration cancelled</Trans>
          )
        }
      >
        {completed ? (
          <Trans>The history is copied and its mail now reaches Yuva.</Trans>
        ) : removed ? (
          <Trans>Conversations nobody wrote in after the import are deleted. Nothing changed in {address}.</Trans>
        ) : removing ? (
          <Trans>Conversations your team wrote in stay. Nothing changes in {address}.</Trans>
        ) : (
          <Trans>What was imported stays. Nothing changed in {address}.</Trans>
        )}
      </StepHeader>
      {announcement}

      <div className="flex flex-col gap-8">
        {removing && <ProgressBar label={t`Removing imported history`} text={t`Removing`} />}

        {!removed && (
          <section className="flex flex-col gap-3" aria-labelledby="migration-summary">
            <h2 id="migration-summary" className="text-title">
              <Trans>What happened</Trans>
            </h2>
            <div className="grid grid-cols-2 gap-2.5" data-testid="migration-summary">
              <Figure value={num(m.imported_conversations ?? 0)} label={<Trans>Conversations</Trans>} />
              <Figure value={num(c.imported)} label={<Trans>Messages imported</Trans>} />
              <Figure value={num(c.duplicate)} label={<Trans>Already in Yuva</Trans>} />
              <Figure value={num(c.skipped + c.failed)} label={<Trans>Skipped or failed</Trans>} />
              {est && <Figure value={num(est.contacts_new)} label={<Trans>New contacts</Trans>} />}
              {est && <Figure value={num(est.labels.length)} label={<Trans>Labels</Trans>} />}
            </div>
            {est && (
              <>
                <h3 className="mt-2 text-body font-medium">
                  <Trans>Not imported</Trans>
                </h3>
                <SkippedList skipped={est.skipped} />
              </>
            )}
          </section>
        )}

        <section className="flex flex-col gap-3" aria-labelledby="migration-access">
          <h2 id="migration-access" className="text-title">
            <Trans>Access to the mailbox</Trans>
          </h2>
          <p className="flex items-start gap-2 text-body" data-testid="migration-credentials">
            <ShieldCheckIcon className="mt-0.5 size-4 shrink-0 text-success" />
            <span>
              {deletedAt ? (
                <Trans>
                  Credentials deleted {deletedAt}. Yuva can no longer read {address}.
                </Trans>
              ) : (
                <Trans>Credentials deleted. Yuva can no longer read {address}.</Trans>
              )}
            </span>
          </p>
          <p className="text-body text-muted-foreground">
            {preset.app_password ? (
              <Trans>Remove the app password you made for Yuva in {provider}; it is not needed any more.</Trans>
            ) : (
              <Trans>If you made a password just for Yuva, remove it in {provider}.</Trans>
            )}
          </p>
          <Button
            variant="outline"
            size="sm"
            className="self-start"
            render={<a href={guide.link} target="_blank" rel="noreferrer" />}
            data-testid="migration-revoke"
          >
            {guide.linkLabel}
            <ArrowUpRightIcon className="rtl:-scale-x-100" />
          </Button>
          {completed && (
            <ul className="flex list-disc flex-col gap-1 ps-5 text-body text-muted-foreground">
              <li>
                <Trans>Keep the forwarding, routing or MX as it is: that is how mail reaches Yuva now.</Trans>
              </li>
              <li>
                <Trans>The old mailbox is untouched; keep it as an archive if you like.</Trans>
              </li>
            </ul>
          )}
        </section>

        <ErrorLine error={ask.error} />
        <div className="flex flex-wrap items-center gap-2">
          {m.inbox_id && (
            <Button onClick={openInbox} data-testid="migration-open-inbox">
              <InboxIcon />
              <Trans>Open the inbox</Trans>
            </Button>
          )}
          {m.actions.includes("remove_history") && (
            <Button
              variant="ghost"
              className="text-destructive"
              onClick={ask.removeHistory}
              disabled={ask.pending}
              data-testid="migration-remove-history"
            >
              <Trans>Remove imported history</Trans>
            </Button>
          )}
        </div>
        {m.actions.includes("remove_history") && (
          <p className="-mt-5 text-small text-faint">
            <Plural
              value={removable}
              one="# imported conversation nobody wrote in since can be removed."
              other="# imported conversations nobody wrote in since can be removed."
            />
          </p>
        )}
        {ask.dialog}
      </div>
    </WizardLayout>
  )
}

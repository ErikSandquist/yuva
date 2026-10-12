import { Trans, useLingui } from "@lingui/react/macro"
import {
  ArrowUpRightIcon,
  BookOpenIcon,
  CircleCheckIcon,
  CircleXIcon,
  HourglassIcon,
  KeyRoundIcon,
  LoaderIcon,
  SendIcon,
} from "lucide-react"
import { useState } from "react"

import { CodeLine, ErrorLine, Notice, Segmented } from "@/components/common"
import { formatDate, formatDateTime, formatRelative } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { useNumber } from "@/features/broadcasts/BroadcastParts"
import { MigrationStateTag, useMigrationConfirms, useStateAnnouncement } from "@/features/migrations/parts"
import { useMigrationStep } from "@/features/migrations/queries"
import { guideUrl, isHelpdesk } from "@/features/migrations/text"
import { WizardLayout } from "@/features/migrations/Wizard"
import { StepHeader } from "@/features/setup/SetupLayout"
import { useNow } from "@/hooks/use-now"
import type { Migration, MigrationPreset } from "@/lib/api"

type Way = "forward" | "route" | "mx"

const PROBE_GRACE = 10 * 60_000

function isGoogle(m: Migration) {
  return m.preset === "gmail" || m.preset === "google_workspace"
}

function useWaySteps(m: Migration, provider: string) {
  const { t } = useLingui()
  const address = m.source_address
  const desk = isHelpdesk(m.source)
  const forward = desk
    ? [
        t`Today ${address} is forwarded to ${provider}, in the settings of the mail provider that hosts it or by a rule there.`,
        t`Change that forwarding to the address below, so the mail of ${address} reaches Yuva instead of ${provider}.`,
        t`If your mail provider mails a confirmation to that address, it arrives in this inbox as a conversation; open it there.`,
      ]
    : isGoogle(m)
      ? [
          t`In Gmail, open Settings › See all settings › Forwarding and POP/IMAP.`,
          t`Choose “Add a forwarding address” and paste the address below.`,
          t`Google mails a confirmation code to that address. It shows up on this page: enter it in Gmail, or open its link.`,
          t`Choose “Forward a copy of incoming mail to” the address, keep Gmail's copy in the inbox, and save.`,
        ]
      : [
          t`Open ${provider}'s mail settings and find forwarding, or a filter that forwards all incoming mail.`,
          t`Forward all incoming mail to the address below and keep a copy in the mailbox.`,
          t`If ${provider} mails a confirmation to that address, it arrives in this inbox as a conversation; open it there.`,
        ]
  const route = desk
    ? [
        t`Ask the admin of your mail server to deliver ${address} to the address below instead of to ${provider}.`,
        t`In Microsoft 365 that is mailbox forwarding or a mail flow rule; an admin may first have to allow external forwarding.`,
      ]
    : m.preset === "google_workspace"
      ? [
          t`In the Google Admin console, open Apps › Google Workspace › Gmail › Routing.`,
          t`Add a rule for ${address} that also delivers its mail to the address below, keeping delivery to the mailbox.`,
        ]
      : [
          t`Ask the admin of your mail server to forward ${address} to the address below on the server, keeping delivery to the mailbox.`,
          t`In Microsoft 365 that is mailbox forwarding or a mail flow rule; an admin may first have to allow external forwarding.`,
        ]
  const mx = [
    t`Only for a domain or subdomain used for support alone: its MX records decide where all of its mail goes.`,
    t`A day ahead, lower the TTL of the MX records, so the change spreads quickly.`,
    t`Point the MX records at Yuva as the e-mail guide describes. Sync catches the mail that still reaches the old server meanwhile.`,
  ]
  const caught = t`Mail that still reaches ${provider} meanwhile is read every two minutes, so nothing is lost.`
  const note = {
    forward: desk
      ? caught
      : isGoogle(m)
        ? t`Gmail forwards only new mail that is not spam. If a Workspace admin turned forwarding off, use the admin's routing.`
        : t`${provider} keeps its copy, so the old mailbox stays complete.`,
    route: desk ? caught : t`The old mailbox keeps its copy.`,
    mx: t`The old mailbox stops receiving mail for that domain.`,
  }
  return { forward, route, mx, note }
}

export function SwitchStep({ m, preset }: { m: Migration; preset: MigrationPreset }) {
  const { t, i18n } = useLingui()
  const provider = preset.name
  const address = m.source_address
  const hasForwarding = !!m.forwarding_address
  const [way, setWay] = useState<Way>(hasForwarding ? "forward" : "mx")
  const steps = useWaySteps(m, provider)
  const ask = useMigrationConfirms(m)
  const announcement = useStateAnnouncement([m])
  const can = (a: Migration["actions"][number]) => m.actions.includes(a)
  const syncUntil = m.sync_until ? formatDate(m.sync_until, i18n.locale) : ""
  const ways = [
    { value: "forward" as const, label: t`Forwarding` },
    { value: "route" as const, label: t`Admin routing` },
    { value: "mx" as const, label: t`MX change` },
  ]
  const shown = hasForwarding ? way : "mx"
  return (
    <WizardLayout step="switch" m={m} preset={preset}>
      <StepHeader title={<Trans>Switch your mail to Yuva</Trans>}>
        {isHelpdesk(m.source) ? (
          <Trans>
            History is in. Yuva keeps reading {provider} every two minutes, so nothing is lost while you send the mail
            of {address} to Yuva.
          </Trans>
        ) : (
          <Trans>
            History is in. Yuva keeps reading {address} every two minutes, so nothing is lost while you send its mail to
            Yuva.
          </Trans>
        )}
      </StepHeader>
      {announcement}

      <div className="flex flex-col gap-8">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <MigrationStateTag m={m} />
          {m.synced_at && (
            <span className="text-small text-faint">
              <SyncedAt at={m.synced_at} />
            </span>
          )}
        </div>

        <section className="flex flex-col gap-4" aria-labelledby="migration-way">
          <h2 id="migration-way" className="text-title">
            <Trans>1. Send the mail to Yuva</Trans>
          </h2>
          {hasForwarding ? (
            <Segmented
              value={way}
              onChange={setWay}
              items={ways}
              label={t`How to send the mail`}
              className="w-full overflow-x-auto"
              itemClassName="flex-1"
            />
          ) : (
            <p className="text-small text-faint">
              <Trans>This server has no forwarding address, so the mail reaches Yuva through an MX change.</Trans>
            </p>
          )}
          <ol
            className="flex list-decimal flex-col gap-2 ps-5 text-body marker:text-faint"
            data-testid="migration-way-steps"
          >
            {steps[shown].map((s) => (
              <li key={s}>{s}</li>
            ))}
          </ol>
          <p className="text-small text-faint">{steps.note[shown]}</p>
          {hasForwarding && shown !== "mx" && (
            <div className="flex flex-col gap-1.5">
              <span className="text-small font-medium">
                <Trans>Forwarding address</Trans>
              </span>
              <CodeLine value={m.forwarding_address!} testId="migration-forwarding-address" />
            </div>
          )}
          {shown === "forward" && <Confirmation m={m} />}
          <Button
            variant="ghost"
            size="sm"
            className="-ms-3 self-start"
            render={<a href={guideUrl(shown === "mx" ? "email" : preset.guide)} target="_blank" rel="noreferrer" />}
          >
            <BookOpenIcon />
            <Trans>Read the guide</Trans>
          </Button>
        </section>

        <section className="flex flex-col gap-4" aria-labelledby="migration-probe">
          <h2 id="migration-probe" className="text-title">
            <Trans>2. Check that it arrives</Trans>
          </h2>
          <Probe m={m} provider={provider} />
        </section>

        <section className="flex flex-col gap-4" aria-labelledby="migration-finish">
          <h2 id="migration-finish" className="text-title">
            <Trans>3. Finish</Trans>
          </h2>
          <Quiet m={m} provider={provider} />
          {syncUntil && (
            <p className="text-small text-faint">
              <Trans>
                If it is not finished by {syncUntil}, sync stops on its own and the credentials are deleted.
              </Trans>
            </p>
          )}
          <ErrorLine error={ask.error} />
          <div className="flex flex-wrap items-center gap-2">
            {can("finish") &&
              (m.verified_at ? (
                <Button onClick={ask.finish} disabled={ask.pending} data-testid="migration-finish">
                  <Trans>Finish now</Trans>
                </Button>
              ) : (
                <Button variant="outline" onClick={ask.finish} disabled={ask.pending} data-testid="migration-finish">
                  <Trans>Finish without checking</Trans>
                </Button>
              ))}
            {can("cancel") && (
              <Button
                variant="ghost"
                className="text-destructive"
                onClick={ask.cancel}
                disabled={ask.pending}
                data-testid="migration-cancel"
              >
                <Trans>Cancel migration</Trans>
              </Button>
            )}
          </div>
          {ask.dialog}
        </section>
      </div>
    </WizardLayout>
  )
}

function SyncedAt({ at }: { at: string }) {
  const { i18n } = useLingui()
  const now = useNow(30_000)
  const when = formatRelative(at, i18n.locale, now)
  return <Trans>Last read {when}</Trans>
}

function Confirmation({ m }: { m: Migration }) {
  const { i18n } = useLingui()
  const c = m.forwarding_confirmation
  if (!c) {
    if (!isGoogle(m) || m.verified_at) return null
    return (
      <Notice icon={HourglassIcon} role="status" aria-live="polite" data-testid="migration-confirmation-waiting">
        <p>
          <Trans>Waiting for Google's confirmation code…</Trans>
        </p>
      </Notice>
    )
  }
  const when = formatDateTime(c.received_at, i18n.locale)
  return (
    <div
      className="flex flex-col gap-3 rounded-2xl border p-4"
      role="status"
      aria-live="polite"
      data-testid="migration-confirmation"
    >
      <p className="flex items-center gap-2 text-body font-medium">
        <KeyRoundIcon className="size-4 text-brand" />
        <Trans>Google sent a confirmation code</Trans>
      </p>
      <CodeLine value={c.code} testId="migration-confirmation-code" />
      <p className="text-small text-faint">
        {c.from ? (
          <Trans>
            For {c.from}, received {when}. Enter it in Gmail's forwarding settings, or open the link.
          </Trans>
        ) : (
          <Trans>Received {when}. Enter it in Gmail's forwarding settings, or open the link.</Trans>
        )}
      </p>
      {c.link && (
        <Button
          variant="outline"
          size="sm"
          className="self-start"
          render={<a href={c.link} target="_blank" rel="noreferrer noopener" />}
          data-testid="migration-confirmation-link"
        >
          <Trans>Open Google's confirmation page</Trans>
          <ArrowUpRightIcon className="rtl:-scale-x-100" />
        </Button>
      )}
    </div>
  )
}

function Probe({ m, provider }: { m: Migration; provider: string }) {
  const { i18n } = useLingui()
  const probe = useMigrationStep(m.id, "probe")
  const now = useNow(15_000)
  const address = m.source_address
  const sent = m.probe_sent_at ? new Date(m.probe_sent_at).getTime() : 0
  const verified = !!m.verified_at
  const stuck = !verified && !!m.probe_synced_at && sent > 0 && now - sent > PROBE_GRACE
  const sentAt = m.probe_sent_at ? formatDateTime(m.probe_sent_at, i18n.locale) : ""
  const verifiedAt = m.verified_at ? formatDateTime(m.verified_at, i18n.locale) : ""
  return (
    <>
      <p className="text-body text-muted-foreground">
        <Trans>
          Yuva mails {address} a short test message. When it reaches Yuva by the new route, the switch works.
        </Trans>
      </p>
      <div role="status" aria-live="polite" data-testid="migration-probe-state">
        {verified ? (
          <p className="flex items-start gap-2 text-body">
            <CircleCheckIcon className="mt-0.5 size-4 shrink-0 text-success" />
            <span>
              {m.verified_route === "direct" ? (
                <Trans>
                  Arrived at {address} directly, {verifiedAt}.
                </Trans>
              ) : (
                <Trans>Arrived through forwarding, {verifiedAt}.</Trans>
              )}
            </span>
          </p>
        ) : stuck ? (
          <Notice tone="danger" icon={CircleXIcon}>
            <p className="font-medium">
              <Trans>Mail still reaches {provider} but is not sent to Yuva yet.</Trans>
            </p>
            <ul className="flex list-disc flex-col gap-1 ps-4 text-small text-muted-foreground">
              <li>
                <Trans>Check that the forwarding address is exactly the one above.</Trans>
              </li>
              <li>
                <Trans>Check that forwarding is turned on and, for Gmail, confirmed with the code.</Trans>
              </li>
              <li>
                <Trans>Check that no filter or admin rule blocks forwarding to other domains.</Trans>
              </li>
            </ul>
          </Notice>
        ) : sent ? (
          <p className="flex items-start gap-2 text-body">
            <LoaderIcon className="mt-0.5 size-4 shrink-0 animate-spin text-faint motion-reduce:animate-none" />
            <span>
              <Trans>Sent {sentAt}. Waiting for it to arrive…</Trans>
            </span>
          </p>
        ) : null}
      </div>
      <ErrorLine error={probe.error} />
      {m.actions.includes("probe") && (
        <Button
          variant={verified ? "outline" : "default"}
          className="self-start"
          onClick={() => probe.mutate()}
          disabled={probe.isPending}
          data-testid="migration-probe"
        >
          <SendIcon />
          {sent ? <Trans>Send another test message</Trans> : <Trans>Send a test message</Trans>}
        </Button>
      )}
    </>
  )
}

function Quiet({ m, provider }: { m: Migration; provider: string }) {
  const { i18n } = useLingui()
  const num = useNumber()
  const now = useNow(30_000)
  const hours = num(m.options.quiet_period_hours)
  const desk = isHelpdesk(m.source)
  if (!m.verified_at || !m.quiet_until)
    return (
      <p className="text-body text-muted-foreground">
        {desk ? (
          <Trans>
            Once a test message arrives, Yuva waits {hours} hours. If nothing reaches only {provider} in that time, the
            migration finishes on its own. You can also finish it yourself.
          </Trans>
        ) : (
          <Trans>
            Once a test message arrives, Yuva waits {hours} hours. If nothing reaches only the old mailbox in that time,
            the migration finishes on its own. You can also finish it yourself.
          </Trans>
        )}
      </p>
    )
  const end = new Date(m.quiet_until).getTime()
  const when = formatRelative(m.quiet_until, i18n.locale, now)
  const at = formatDateTime(m.quiet_until, i18n.locale)
  return (
    <p className="flex items-start gap-2 text-body" data-testid="migration-quiet">
      <HourglassIcon className="mt-0.5 size-4 shrink-0 text-faint" />
      <span>
        {end > now && desk ? (
          <Trans>
            If nothing reaches only {provider}, the migration finishes on its own {when} ({at}).
          </Trans>
        ) : end > now ? (
          <Trans>
            If nothing reaches only the old mailbox, the migration finishes on its own {when} ({at}).
          </Trans>
        ) : (
          <Trans>The quiet period is over; the migration finishes at the next sync.</Trans>
        )}
      </span>
    </p>
  )
}

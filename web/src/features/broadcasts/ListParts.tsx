import { Trans, useLingui } from "@lingui/react/macro"
import { MailWarningIcon } from "lucide-react"

export function ConsentExplainer() {
  return (
    <p>
      <Trans>
        A mailing list holds the people who agreed to get your broadcasts. They join on a subscribe form on your site,
        or your team adds them and records how they agreed. Every step is kept as proof of consent, and everyone can
        leave with one click.
      </Trans>
    </p>
  )
}

export function NeedsSender() {
  const { t } = useLingui()
  return (
    <span
      className="inline-flex shrink-0 items-center gap-1 rounded-full bg-warning/12 px-2 py-px text-caption font-medium text-warning"
      title={t`No e-mail channel that can send its confirmation and welcome mails`}
      data-testid="list-needs-sender-badge"
    >
      <MailWarningIcon className="size-3" />
      <Trans>Needs a sender</Trans>
    </span>
  )
}

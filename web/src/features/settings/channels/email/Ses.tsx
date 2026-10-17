import { Trans } from "@lingui/react/macro"
import { CircleAlertIcon, LoaderIcon, ShieldCheckIcon, TriangleAlertIcon } from "lucide-react"

import { Notice } from "@/components/common"
import { type EmailSes } from "@/lib/api"

function domainOf(address: string) {
  const at = address.lastIndexOf("@")
  return at < 0 ? address : address.slice(at + 1)
}

export function SesStatus({ ses, address }: { ses: EmailSes; address: string }) {
  const domain = domainOf(address)
  const tenant = ses.tenant ?? ""
  const channelTenant = ses.channel_tenant ?? ""
  const detail = ses.detail ?? ""
  let tone: "info" | "warning" | "danger" = "warning"
  let icon: React.ComponentType<{ className?: string }> = TriangleAlertIcon
  let body: React.ReactNode = null

  if (ses.source === "server") {
    switch (ses.state) {
      case "isolated":
        icon = ShieldCheckIcon
        tone = "info"
        body = tenant ? (
          <Trans>
            Mail from this inbox goes out in its own Amazon SES tenant, <span dir="ltr">{tenant}</span>, so its
            reputation is kept apart from your other inboxes and other senders.
          </Trans>
        ) : (
          <Trans>
            Mail from this inbox goes out in its own Amazon SES tenant, so its reputation is kept apart from your other
            inboxes and other senders.
          </Trans>
        )
        break
      case "pending":
        icon = LoaderIcon
        body = <Trans>Setting up the inbox’s Amazon SES tenant…</Trans>
        break
      case "identity_missing":
        body = (
          <Trans>
            <span dir="ltr">{domain}</span> is not a verified identity in the server’s Amazon SES account. Mail goes out
            without tenant isolation and broadcasts are blocked. Ask the server’s operator to verify the domain.
          </Trans>
        )
        break
      case "paused":
        tone = "danger"
        icon = CircleAlertIcon
        body = (
          <Trans>
            Amazon SES paused sending for this inbox. Its mail through SES is refused with “Sending disabled” until the
            server’s operator reinstates it. Other inboxes keep sending.
          </Trans>
        )
        break
      case "error":
        tone = "danger"
        icon = CircleAlertIcon
        body = detail ? (
          <Trans>
            Amazon SES refused to set up the tenant: <span dir="ltr">{detail}</span>
          </Trans>
        ) : (
          <Trans>Amazon SES refused to set up the tenant.</Trans>
        )
        break
      case "not_isolated":
        body = <Trans>This channel is not isolated in an Amazon SES tenant.</Trans>
        break
    }
  } else if (ses.state === "isolated" && channelTenant) {
    icon = ShieldCheckIcon
    tone = "info"
    body = (
      <Trans>
        Mail carries the tenant <span dir="ltr">{channelTenant}</span> you typed below.
      </Trans>
    )
  } else if (ses.state === "not_isolated") {
    body = (
      <Trans>
        This channel sends through Amazon SES in another account or region, so Yuva cannot isolate it in a tenant. Mail
        goes out without one.
      </Trans>
    )
  } else if (ses.state === "error") {
    tone = "danger"
    icon = CircleAlertIcon
    body = <Trans>Amazon SES refused a step of the tenant setup.</Trans>
  }

  if (!body) return null
  return (
    <Notice tone={tone} icon={icon} className="text-caption" data-testid="ses-status" data-state={ses.state}>
      <p>{body}</p>
    </Notice>
  )
}

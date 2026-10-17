import { Trans, useLingui } from "@lingui/react/macro"
import { useState } from "react"
import { useLocation } from "react-router"

import { CodeLine, softBreaks, Notice, Segmented, StepActions, StepHeader } from "@/components/common"
import { Skeleton } from "@/components/ui/skeleton"
import { emailPaths } from "@/features/setup/email/draft"
import { EmailLayout } from "@/features/setup/email/EmailLayout"
import { useChannel } from "@/features/setup/InstallStep"
import { MailMock, PreviewCaption } from "@/features/setup/previews"
import { useInbox } from "@/features/settings/inboxes/queries"
import { type SetupState } from "@/features/setup/setup"
import type { Channel } from "@/lib/api"
import { CopyButton } from "@/components/common"
import { useNavigate } from "react-router"

const DOCS = "https://github.com/productdevbook/yuva/blob/main/docs"

type Way = "forward" | "route"

function Forward({ channel }: { channel: Channel }) {
  const own = channel.email!.address
  return (
    <div className="flex flex-col gap-4" data-testid="setup-receive-forward">
      <p className="text-muted-foreground">
        <Trans>In Gmail, Outlook or your provider, forward {own} to this address.</Trans>
      </p>
      <div className="flex min-w-0 items-center gap-2">
        <code
          className="min-w-0 flex-1 rounded-xl border bg-surface px-3 py-2 font-mono text-caption [overflow-wrap:anywhere]"
          data-testid="setup-forwarding-address"
        >
          {softBreaks(channel.forwarding_address ?? "")}
        </code>
        <CopyButton value={channel.forwarding_address ?? ""} />
      </div>
      <Notice tone="warning" className="text-small">
        <Trans>Never forward to {own} itself: the mail would loop.</Trans>
      </Notice>
    </div>
  )
}

function Route() {
  const ingress = `${window.location.origin}/ingress/email`
  return (
    <div className="flex flex-col gap-4" data-testid="setup-receive-route">
      <p className="text-muted-foreground">
        <Trans>
          With Cloudflare Email Routing, deploy the Email Worker from edge/ of the Yuva repository and add a routing
          rule for the address with the action Send to a Worker. Any other mail server can post each message, signed
          with YUVA_INGRESS_SECRET, to:
        </Trans>
      </p>
      <CodeLine value={ingress} testId="setup-ingress" />
      <a
        href={`${DOCS}/email.md#inbound`}
        target="_blank"
        rel="noreferrer"
        className="w-fit text-body text-brand underline-offset-4 hover:underline"
      >
        <Trans>Step-by-step guide</Trans>
      </a>
    </div>
  )
}

export function EmailReceiveStep({ inboxId, channelId }: { inboxId: string; channelId: string }) {
  const { t } = useLingui()
  const state = (useLocation().state ?? {}) as SetupState
  const navigate = useNavigate()
  const channel = useChannel(channelId).data
  const inbox = useInbox(inboxId).data
  const forwarding = !!channel?.forwarding_address
  const [picked, setPicked] = useState<Way | null>(null)
  const way: Way = picked ?? (forwarding ? "forward" : "route")
  return (
    <EmailLayout
      stage="receive"
      isNew={state.existing === false}
      inboxId={inboxId}
      channelId={channelId}
      preview={
        <>
          <MailMock name={inbox?.name ?? ""} to={channel?.email?.address ?? ""} waiting />
          <PreviewCaption>
            <Trans>It shows up here as soon as it reaches Yuva.</Trans>
          </PreviewCaption>
        </>
      }
    >
      <StepHeader title={<Trans>Route your mail to Yuva</Trans>}>
        <Trans>Yuva does not run a mail server: your domain's mail is passed to it.</Trans>
      </StepHeader>
      {!channel?.email ? (
        <div className="flex flex-col gap-4">
          <Skeleton className="h-8 w-3/4" />
          <Skeleton className="h-24 w-full" />
        </div>
      ) : (
        <div className="flex flex-col gap-6">
          {forwarding && (
            <Segmented
              value={way}
              onChange={setPicked}
              label={t`How mail reaches Yuva`}
              items={[
                { value: "forward", label: t`Forward from your mailbox`, testId: "setup-receive-way-forward" },
                { value: "route", label: t`Route your domain`, testId: "setup-receive-way-route" },
              ]}
              className="self-start"
            />
          )}
          {way === "forward" ? <Forward channel={channel} /> : <Route />}
          <StepActions
            type="button"
            onClick={() => navigate(emailPaths(inboxId, channelId).test, { state })}
            testId="setup-install-continue"
          >
            <Trans>Continue</Trans>
          </StepActions>
        </div>
      )}
    </EmailLayout>
  )
}

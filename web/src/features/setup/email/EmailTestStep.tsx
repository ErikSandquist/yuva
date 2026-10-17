import { Trans, useLingui } from "@lingui/react/macro"
import { useQueryClient } from "@tanstack/react-query"
import { CheckCircle2Icon, ChevronDownIcon, LoaderIcon, MailIcon, RefreshCwIcon } from "lucide-react"
import { useState } from "react"
import { Navigate, useLocation, useNavigate } from "react-router"

import { Notice, StepActions, StepHeader, SummaryList } from "@/components/common"
import { formatShort } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { Skeleton } from "@/components/ui/skeleton"
import { useConversations } from "@/features/inbox/queries"
import { DkimSection, Lines, StateNotice } from "@/features/settings/channels/email/Deliverability"
import {
  isChecking,
  RateLimited,
  useDeliverability,
  useRunCheck,
} from "@/features/settings/channels/email/deliverability"
import { sesShown } from "@/features/settings/channels/email/EmailFields"
import { emailForm } from "@/features/settings/channels/email/form"
import { SesStatus } from "@/features/settings/channels/email/Ses"
import { useInbox } from "@/features/settings/inboxes/queries"
import { emailPaths } from "@/features/setup/email/draft"
import { EmailLayout } from "@/features/setup/email/EmailLayout"
import { useChannel } from "@/features/setup/InstallStep"
import { FirstConversation, MailMock, PreviewCaption } from "@/features/setup/previews"
import { type SetupState } from "@/features/setup/setup"
import { keys } from "@/lib/keys"
import { useRealtimeStatus } from "@/lib/realtime"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"

function TestCard({ title, children, testId }: { title: React.ReactNode; children: React.ReactNode; testId: string }) {
  return (
    <section className="flex min-w-0 flex-col gap-3 rounded-2xl border bg-card p-4" data-testid={testId}>
      <h2 className="text-body font-medium">{title}</h2>
      {children}
    </section>
  )
}

function ReceivingCard({ address, inboxId, channelId }: { address: string; inboxId: string; channelId: string }) {
  const { t } = useLingui()
  const live = useRealtimeStatus() === "live"
  const recent = useConversations({ inbox_id: inboxId }).data?.pages[0]?.items
  const got = recent?.find((c) => c.channel_id === channelId)
  return (
    <TestCard title={<Trans>Receiving</Trans>} testId="setup-test-receiving">
      <p className="text-body text-muted-foreground">
        <Trans>Send a mail to {address} from any mailbox.</Trans>
      </p>
      {got ? (
        <p className="flex items-center gap-2.5 text-body text-success" role="status" data-testid="setup-test-received">
          <CheckCircle2Icon className="size-4" />
          <Trans>The mail arrived.</Trans>
        </p>
      ) : (
        <>
          <p
            className="flex items-center gap-2.5 text-body text-muted-foreground"
            role="status"
            data-testid="setup-listening"
          >
            <span className="relative flex size-2.5">
              {live && (
                <span className="absolute inline-flex size-full animate-ping rounded-full bg-brand opacity-60" />
              )}
              <span className={cn("relative inline-flex size-2.5 rounded-full", live ? "bg-brand" : "bg-faint")} />
            </span>
            {live ? <Trans>Listening for the mail…</Trans> : <Trans>Connecting…</Trans>}
          </p>
          <Button
            variant="outline"
            size="sm"
            className="self-start"
            render={<a href={`mailto:${address}?subject=${encodeURIComponent(t`Hello from the setup`)}`} />}
          >
            <MailIcon />
            <Trans>Write a test e-mail</Trans>
          </Button>
        </>
      )}
    </TestCard>
  )
}

function SendingCard({ channelId, address }: { channelId: string; address: string }) {
  const { i18n } = useLingui()
  const check = useDeliverability(channelId)
  const run = useRunCheck(channelId)
  const c = check.data
  const checking = isChecking(c)
  const limited = run.error instanceof RateLimited ? run.error : null
  const retry = limited ? formatShort(new Date(limited.retryAt).toISOString(), i18n.locale) : ""
  return (
    <TestCard title={<Trans>Sending</Trans>} testId="setup-test-sending">
      <p className="text-body text-muted-foreground">
        <Trans>A check sends a test mail through your reply account and looks up the domain's DNS.</Trans>
      </p>
      <Button
        variant="outline"
        size="sm"
        className="self-start"
        onClick={() => run.mutate()}
        disabled={run.isPending || checking}
        data-testid="deliverability-run"
      >
        {run.isPending || checking ? <LoaderIcon className="animate-spin" /> : <RefreshCwIcon />}
        {c ? <Trans>Check again</Trans> : <Trans>Send a check</Trans>}
      </Button>
      {check.isPending ? (
        <Skeleton className="h-8 w-full" />
      ) : c ? (
        <>
          <StateNotice check={c} address={address} />
          {c.state !== "failed" && <Lines check={c} />}
        </>
      ) : null}
      {limited && (
        <p role="alert" className="text-body text-destructive" data-testid="deliverability-limited">
          <Trans>That was the sixth check this hour. You can check again at {retry}.</Trans>
        </p>
      )}
    </TestCard>
  )
}

export function EmailTestStep({ inboxId, channelId }: { inboxId: string; channelId: string }) {
  const { t } = useLingui()
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  const navigate = useNavigate()
  const state = (useLocation().state ?? {}) as SetupState
  const channelQuery = useChannel(channelId)
  const inboxQuery = useInbox(inboxId)
  const channel = channelQuery.data
  const inbox = inboxQuery.data
  const check = useDeliverability(channel?.email?.smtp ? channelId : undefined)
  const recent = useConversations({ inbox_id: inboxId }).data?.pages[0]?.items
  const [dkimOpen, setDkimOpen] = useState(false)
  const email = channel?.email
  const arrived = recent?.find((c) => c.channel_id === channelId)
  const passed = !!arrived || check.data?.state === "received"
  if (channel && !email) return <Navigate to={`/setup/${inboxId}/${channelId}`} replace state={state} />
  const ses = email?.ses
  const tenant = inbox?.ses_tenant
  const setting = ses?.state === "pending" || tenant?.state === "pending"
  const refreshing = channelQuery.isFetching || inboxQuery.isFetching
  const status =
    tenant &&
    {
      enabled: t({ message: "Sending", context: "SES tenant status" }),
      reinstated: t`Reinstated`,
      disabled: t`Paused`,
    }[tenant.sending_status]
  const impact = tenant && { none: t`None`, low: t`Low`, high: t`High` }[tenant.reputation_impact]
  return (
    <EmailLayout
      stage="test"
      isNew={state.existing === false}
      inboxId={inboxId}
      channelId={channelId}
      preview={
        <>
          {arrived ? (
            <>
              <FirstConversation item={arrived} name={inbox?.name ?? ""} kind="email" />
              <PreviewCaption>
                <Trans>New conversations show up at the top of your conversation list.</Trans>
              </PreviewCaption>
            </>
          ) : (
            <>
              <MailMock name={inbox?.name ?? ""} to={email?.address ?? ""} waiting />
              <PreviewCaption>
                <Trans>It shows up here as soon as it reaches Yuva.</Trans>
              </PreviewCaption>
            </>
          )}
        </>
      }
    >
      <StepHeader title={<Trans>Check that it works</Trans>}>
        <Trans>Try each part now, or skip and come back from the channel settings.</Trans>
      </StepHeader>
      {!email ? (
        <div className="flex flex-col gap-4">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-24 w-full" />
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          <ReceivingCard address={email.address} inboxId={inboxId} channelId={channelId} />
          {email.smtp && <SendingCard channelId={channelId} address={email.address} />}
          {ses && (
            <TestCard title={<Trans>Amazon SES</Trans>} testId="setup-test-ses">
              <SesStatus ses={ses} address={email.from_address || email.address} />
              {tenant && (
                <SummaryList
                  items={[
                    [<Trans key="s">Sending</Trans>, status],
                    [<Trans key="r">Reputation impact</Trans>, impact],
                  ]}
                />
              )}
              {tenant?.sending_status === "disabled" && (
                <Notice tone="danger" className="text-small" data-testid="ses-paused">
                  <Trans>
                    Amazon SES paused this inbox’s sending. Its replies and broadcasts through the server’s SES account
                    are refused until the operator reinstates it. Other inboxes keep sending.
                  </Trans>
                </Notice>
              )}
              {setting && (
                <Button
                  variant="outline"
                  size="sm"
                  className="self-start"
                  disabled={refreshing}
                  onClick={() => {
                    void qc.invalidateQueries({ queryKey: keys.channel(ws, channelId) })
                    void qc.invalidateQueries({ queryKey: [...keys.inboxes(ws), inboxId] })
                  }}
                  data-testid="setup-ses-refresh"
                >
                  {refreshing ? <LoaderIcon className="animate-spin" /> : <RefreshCwIcon />}
                  <Trans>Check again</Trans>
                </Button>
              )}
            </TestCard>
          )}
          {email.smtp && !sesShown({ f: emailForm(email), ses }) && (
            <Collapsible open={dkimOpen} onOpenChange={setDkimOpen}>
              <CollapsibleTrigger
                className="-ms-2 inline-flex items-center gap-1.5 rounded-lg px-2 py-1 text-body text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/30"
                data-testid="setup-test-dkim"
              >
                <ChevronDownIcon className={cn("size-4 transition-transform", dkimOpen && "rotate-180")} />
                <Trans>DKIM for broadcasts</Trans>
              </CollapsibleTrigger>
              <CollapsibleContent className="mt-3">
                <DkimSection channel={channel!} />
              </CollapsibleContent>
            </Collapsible>
          )}
          <StepActions
            type="button"
            onClick={() => navigate(emailPaths(inboxId, channelId).done, { state })}
            testId="setup-test-continue"
          >
            {passed ? <Trans>Continue</Trans> : <Trans>Skip for now</Trans>}
          </StepActions>
        </div>
      )}
    </EmailLayout>
  )
}

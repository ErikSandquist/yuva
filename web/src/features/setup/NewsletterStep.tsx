import { Trans, useLingui } from "@lingui/react/macro"
import { ArrowRightIcon, MailCheckIcon, MailIcon, PartyPopperIcon } from "lucide-react"
import { Link, useLocation, useNavigate } from "react-router"

import { ErrorLine, Notice } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { canSend, ListFields, useNewList } from "@/features/broadcasts/ListForm"
import { listLink, useList } from "@/features/broadcasts/queries"
import { DEFAULT_COLOR, PreviewCaption, SubscribeMock } from "@/features/setup/previews"
import type { SetupState } from "@/features/setup/setup"
import { SetupLayout, StepHeader } from "@/features/setup/SetupLayout"
import { useChannel, useInboxes } from "@/lib/workspace"

function useSteps() {
  const { t } = useLingui()
  return [t`Mailing list`, t`Done`]
}

function Preview({ name, description, inboxId }: { name: string; description?: string; inboxId: string }) {
  const inbox = useInboxes().data?.find((i) => i.id === inboxId)
  return (
    <>
      <SubscribeMock name={name} description={description} color={inbox?.branding.color ?? DEFAULT_COLOR} />
      <PreviewCaption>
        <Trans>The subscribe form shows the list’s name and description.</Trans>
      </PreviewCaption>
    </>
  )
}

function NeedsInbox() {
  const { t } = useLingui()
  const steps = useSteps()
  return (
    <SetupLayout
      step={1}
      steps={steps}
      stepsLabel={t`Newsletter steps`}
      preview={<SubscribeMock name="" color={DEFAULT_COLOR} />}
    >
      <StepHeader icon={<MailIcon />} title={<Trans>First, an e-mail address</Trans>}>
        <Trans>
          A mailing list sends in an inbox’s name, through its e-mail channel. Set up e-mail first, then come back here
          for the list.
        </Trans>
      </StepHeader>
      <Button
        size="lg"
        render={<Link to="/setup/start/email" state={{ back: "/setup" } satisfies SetupState} />}
        data-testid="newsletter-setup-email"
      >
        <Trans>Set up e-mail</Trans>
        <ArrowRightIcon className="rtl:rotate-180" />
      </Button>
    </SetupLayout>
  )
}

function NewListForm({ inboxId }: { inboxId: string }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const state = (useLocation().state ?? {}) as SetupState
  const steps = useSteps()
  const { values, onChange, channels, sender, create, submit } = useNewList(inboxId, (l) =>
    navigate(`/setup/newsletter/${l.id}`, { replace: true, state })
  )
  return (
    <SetupLayout
      step={1}
      steps={steps}
      stepsLabel={t`Newsletter steps`}
      preview={<Preview name={values.name} description={values.description} inboxId={values.inbox_id} />}
    >
      <StepHeader title={<Trans>Start your mailing list</Trans>}>
        <Trans>People join it on a subscribe form or are added with their consent. Broadcasts go to lists.</Trans>
      </StepHeader>
      <form
        className="flex flex-col gap-6"
        onSubmit={(e) => {
          e.preventDefault()
          submit()
        }}
        data-testid="setup-newsletter"
      >
        <ListFields values={values} onChange={onChange} creating error={create.error} />
        {channels && !sender && (
          <Notice icon={MailIcon} data-testid="newsletter-no-sender">
            <p>
              <Trans>
                This inbox has no e-mail channel that can send yet. You can create the list now; it sends once an e-mail
                channel with an SMTP account is picked in its settings.
              </Trans>
            </p>
          </Notice>
        )}
        <ErrorLine error={create.error} />
        <Button
          type="submit"
          size="lg"
          className="self-start"
          disabled={!values.name.trim() || create.isPending}
          data-testid="list-create"
        >
          <Trans>Create list</Trans>
          <ArrowRightIcon className="rtl:rotate-180" />
        </Button>
      </form>
    </SetupLayout>
  )
}

function Sender({ channelId }: { channelId: string }) {
  const channel = useChannel(channelId).data
  if (!channel) return <Skeleton className="h-16 rounded-2xl" />
  const address = channel.email?.address ?? channel.name
  return (
    <Notice icon={MailCheckIcon} className="bg-success/8 [&>svg]:text-success" data-testid="newsletter-sender">
      <p className="font-medium text-foreground">
        <Trans>It sends through {address}.</Trans>
      </p>
      <p>
        {canSend(channel) ? (
          <Trans>Confirmation and welcome mails, and every broadcast to the list, go out from this address.</Trans>
        ) : (
          <Trans>This address has no SMTP account yet. Add one in the inbox settings before the first broadcast.</Trans>
        )}
      </p>
    </Notice>
  )
}

function ListDone({ listId }: { listId: string }) {
  const { t } = useLingui()
  const steps = useSteps()
  const list = useList(listId)
  const l = list.data
  const listName = l?.name ?? ""
  return (
    <SetupLayout
      step={2}
      steps={steps}
      stepsLabel={t`Newsletter steps`}
      preview={l ? <Preview name={l.name} description={l.description} inboxId={l.inbox_id} /> : null}
    >
      <ErrorLine error={list.error} />
      {!l ? (
        !list.error && <Skeleton className="h-40 rounded-2xl" />
      ) : (
        <div className="flex flex-col gap-6" data-testid="newsletter-done">
          <StepHeader icon={<PartyPopperIcon />} title={<Trans>{listName} is ready</Trans>}>
            <Trans>Put its subscribe form on your site, or add people who agreed to hear from you.</Trans>
          </StepHeader>
          {l.channel_id ? (
            <Sender channelId={l.channel_id} />
          ) : (
            <Notice tone="warning" icon={MailIcon} data-testid="newsletter-no-sender">
              <p>
                <Trans>
                  The list has no e-mail channel to send through yet. Set up e-mail with an SMTP account, then pick it
                  in the list settings.
                </Trans>
              </p>
              <Link
                to={`/setup/${l.inbox_id}/add/email`}
                state={{ back: "/setup" } satisfies SetupState}
                className="w-fit text-brand underline-offset-4 hover:underline"
              >
                <Trans>Set up e-mail</Trans>
              </Link>
            </Notice>
          )}
          <div className="flex flex-wrap gap-2">
            <Button size="lg" render={<Link to={listLink(l.id, "form")} />} data-testid="newsletter-form">
              <Trans>Get the subscribe form</Trans>
              <ArrowRightIcon className="rtl:rotate-180" />
            </Button>
            <Button size="lg" variant="outline" render={<Link to="/setup" />}>
              <Trans>Back to setup</Trans>
            </Button>
          </div>
        </div>
      )}
    </SetupLayout>
  )
}

export function NewsletterStep({ listId }: { listId?: string }) {
  const { t } = useLingui()
  const steps = useSteps()
  const inboxes = useInboxes()
  if (listId) return <ListDone listId={listId} />
  if (!inboxes.data) {
    return (
      <SetupLayout step={1} steps={steps} stepsLabel={t`Newsletter steps`} preview={null}>
        <ErrorLine error={inboxes.error} />
        {!inboxes.error && <Skeleton className="h-48 rounded-2xl" />}
      </SetupLayout>
    )
  }
  if (inboxes.data.length === 0) return <NeedsInbox />
  return <NewListForm inboxId={inboxes.data[0]!.id} />
}

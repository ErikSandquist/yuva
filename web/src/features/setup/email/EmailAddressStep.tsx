import { Trans } from "@lingui/react/macro"
import { useState } from "react"
import { Navigate, useLocation, useNavigate } from "react-router"

import { StepActions, StepHeader } from "@/components/common"
import { Input } from "@/components/ui/input"
import { useEmailFieldErrors } from "@/features/settings/channels/email/form"
import { FieldError } from "@/features/settings/channels/parts"
import { useInbox } from "@/features/settings/inboxes/queries"
import { Field } from "@/features/settings/ui"
import { useInboxDraft, useLeaveSetup } from "@/features/setup/draft"
import { emailPaths, useEmailDraft } from "@/features/setup/email/draft"
import { EmailLayout } from "@/features/setup/email/EmailLayout"
import { MailMock, PreviewCaption } from "@/features/setup/previews"
import { type SetupState } from "@/features/setup/setup"

export function EmailAddressStep({ inboxId }: { inboxId: string }) {
  const navigate = useNavigate()
  const state = (useLocation().state ?? {}) as SetupState
  const isNew = inboxId === "new"
  const fieldErrors = useEmailFieldErrors()
  const leaveSetup = useLeaveSetup()
  const [inboxDraft] = useInboxDraft()
  const inbox = useInbox(inboxId).data
  const [draft, patch, clear] = useEmailDraft(inboxId)
  const [problem, setProblem] = useState(state.emailError)
  const name = isNew ? inboxDraft.name : (inbox?.name ?? "")
  const address = draft.address.trim()
  if (isNew && !name.trim()) return <Navigate to="/setup/new" replace state={state} />
  return (
    <EmailLayout
      stage="address"
      isNew={isNew}
      inboxId={inboxId}
      onLeave={() => {
        clear()
        if (isNew) leaveSetup()
      }}
      back={isNew ? "/setup/new/pick" : undefined}
      preview={
        <>
          <MailMock name={name} to={address} />
          <PreviewCaption>
            <Trans>Each new e-mail starts a conversation; replies continue it.</Trans>
          </PreviewCaption>
        </>
      }
    >
      <StepHeader title={<Trans>Which address do customers write to?</Trans>}>
        <Trans>Use the support address you already have. Mail sent to it will arrive here.</Trans>
      </StepHeader>
      <form
        className="flex flex-col gap-6"
        onSubmit={(e) => {
          e.preventDefault()
          if (address) navigate(emailPaths(inboxId).replies, { state: { ...state, emailError: undefined } })
        }}
        data-testid="setup-add-email"
      >
        <Field
          label={<Trans>Support address</Trans>}
          htmlFor="setup-address"
          hint={
            <Trans>
              *@example.com receives every address of the domain that no other channel has; replies then go out from the
              address the customer wrote to.
            </Trans>
          }
        >
          <Input
            id="setup-address"
            type="email"
            autoFocus
            required
            maxLength={320}
            className="h-10 rounded-xl"
            value={draft.address}
            placeholder="support@example.com"
            onChange={(e) => {
              setProblem(undefined)
              patch({ address: e.target.value })
            }}
            aria-invalid={!!problem || undefined}
            aria-describedby={problem ? "err-address" : undefined}
            data-testid="setup-address"
          />
          <FieldError id="err-address">
            {problem === "taken" ? (
              <Trans>Another channel already receives mail at this address.</Trans>
            ) : problem ? (
              fieldErrors.address
            ) : null}
          </FieldError>
        </Field>
        <StepActions testId="setup-add-submit">
          <Trans>Continue</Trans>
        </StepActions>
      </form>
    </EmailLayout>
  )
}

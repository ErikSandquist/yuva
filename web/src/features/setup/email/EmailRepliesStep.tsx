import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { ChevronDownIcon, CloudIcon, DownloadIcon, ServerIcon } from "lucide-react"
import { useState } from "react"
import { Navigate, useLocation, useNavigate } from "react-router"

import { ChoiceCards, ErrorLine, Notice, StepActions, StepHeader } from "@/components/common"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import {
  SenderFields,
  SesFields,
  SmtpEncryptionField,
  SmtpHostField,
  SmtpLoginFields,
  SmtpPortField,
} from "@/features/settings/channels/email/EmailFields"
import {
  emailErrorField,
  emailForm,
  emailInput,
  useEmailFieldErrors,
  type EmailField,
  type EmailForm,
} from "@/features/settings/channels/email/form"
import { useInbox } from "@/features/settings/inboxes/queries"
import { ChoiceSelect, Field } from "@/features/settings/ui"
import { useKindText } from "@/features/setup/ChannelStep"
import { idempotencyKey, useEnsureInbox, useInboxDraft, useLeaveSetup } from "@/features/setup/draft"
import {
  emailPaths,
  SES_REGIONS,
  sesHost,
  useEmailDraft,
  type EmailDraft,
  type EmailMethod,
} from "@/features/setup/email/draft"
import { EmailLayout } from "@/features/setup/email/EmailLayout"
import { MailMock, PreviewCaption } from "@/features/setup/previews"
import { type SetupState } from "@/features/setup/setup"
import { api, ApiError, unwrap } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"

type Problems = Partial<Record<EmailField, string>>

const MORE_FIELDS: EmailField[] = [
  "display_name",
  "from_address",
  "smtp.port",
  "smtp.tls",
  "ses.tenant",
  "ses.configuration_set",
]

export function EmailRepliesStep({ inboxId }: { inboxId: string }) {
  const { t } = useLingui()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const state = (useLocation().state ?? {}) as SetupState
  const { workspaceId: ws } = useSession()
  const isNew = inboxId === "new"
  const fieldErrors = useEmailFieldErrors()
  const kindText = useKindText()
  const leaveSetup = useLeaveSetup()
  const ensureInbox = useEnsureInbox()
  const [inboxDraft, , clearInbox] = useInboxDraft()
  const inbox = useInbox(inboxId).data
  const [draft, patch, clear] = useEmailDraft(inboxId)
  const [password, setPassword] = useState("")
  const [restored] = useState(() => draft.method === "ses" || draft.method === "smtp")
  const [problems, setProblems] = useState<Problems>({})
  const [more, setMore] = useState(false)
  const name = isNew ? inboxDraft.name : (inbox?.name ?? "")
  const method = draft.method
  const paths = emailPaths(inboxId)

  const f: EmailForm = {
    ...emailForm(),
    address: draft.address,
    displayName: method === "none" ? "" : draft.displayName,
    fromAddress: method === "none" ? "" : draft.fromAddress,
    host: method === "ses" ? (draft.region ? sesHost(draft.region) : "") : method === "smtp" ? draft.host : "",
    port: method === "none" ? "" : draft.port,
    tls: draft.tls,
    username: method === "none" ? "" : draft.username,
    password: method === "none" ? "" : password,
    sesTenant: method === "ses" ? draft.sesTenant : "",
    sesSet: method === "ses" ? draft.sesSet : "",
  }

  const taken = (err: unknown) => err instanceof ApiError && err.code === "email_address_taken"

  const save = useMutation({
    mutationFn: async () => {
      const target = isNew ? (await ensureInbox()).id : inboxId
      const body = { kind: "email" as const, name: name || kindText.email.title, email: emailInput(f) }
      return unwrap(
        api.POST("/v1/inboxes/{inboxId}/channels", {
          params: {
            path: { inboxId: target },
            header: { "Idempotency-Key": idempotencyKey(draft.idempotencyKey, "channel", [target, body]) },
          },
          body,
        })
      )
    },
    onSuccess: (ch) => {
      void qc.invalidateQueries({ queryKey: keys.channels(ws, ch.inbox_id) })
      void qc.invalidateQueries({ queryKey: keys.inboxes(ws) })
      qc.setQueryData(keys.channel(ws, ch.id), ch)
      clear()
      if (isNew) clearInbox()
      navigate(emailPaths(ch.inbox_id, ch.id).receive, { state: { ...state, existing: !isNew } satisfies SetupState })
    },
    onError: (err) => {
      if (emailErrorField(err) === "address") {
        navigate(paths.address, {
          state: { ...state, emailError: taken(err) ? "taken" : "invalid" } satisfies SetupState,
        })
      }
    },
  })

  const serverBad = taken(save.error) ? null : emailErrorField(save.error)
  const error = (field: EmailField) => problems[field] ?? (serverBad === field ? fieldErrors[field] : undefined)

  const set = (p: Partial<EmailForm>) => {
    setProblems({})
    save.reset()
    if (p.password !== undefined && p.password !== null) setPassword(p.password)
    const d: Partial<EmailDraft> = {}
    if (p.displayName !== undefined) d.displayName = p.displayName
    if (p.fromAddress !== undefined) d.fromAddress = p.fromAddress
    if (p.host !== undefined) d.host = p.host
    if (p.port !== undefined) d.port = p.port
    if (p.tls !== undefined) d.tls = p.tls
    if (p.username !== undefined) d.username = p.username
    if (p.sesTenant !== undefined) d.sesTenant = p.sesTenant
    if (p.sesSet !== undefined) d.sesSet = p.sesSet
    patch(d)
  }

  const pick = (m: string) => {
    setProblems({})
    save.reset()
    patch(m === "ses" ? { method: m, tls: "starttls", port: "" } : { method: m as EmailMethod })
  }

  const submit = () => {
    const out: Problems = {}
    if (method === "ses") {
      if (!draft.region) out["smtp.host"] = t`Choose the region of your Amazon SES account.`
      if (!draft.username.trim()) out["smtp.username"] = t`Enter the SMTP user name from the SES console.`
      if (!password) out["smtp.password"] = t`Enter the SMTP password from the SES console.`
    }
    if (method === "smtp" && !draft.host.trim()) out["smtp.host"] = fieldErrors["smtp.host"]
    if (method !== "none" && draft.tls === "none" && draft.username.trim() && !out["smtp.username"]) {
      out["smtp.username"] = fieldErrors["smtp.username"]
    }
    setProblems(out)
    if (Object.keys(out).length === 0) save.mutate()
  }

  if (!draft.address.trim() && !save.isSuccess) return <Navigate to={paths.address} replace state={state} />
  if (isNew && !name.trim() && !save.isSuccess) return <Navigate to="/setup/new" replace state={state} />

  const props = { f, set, error, taken: false }
  const moreBad = MORE_FIELDS.some((k) => !!error(k))
  const ses = method === "ses"
  return (
    <EmailLayout
      stage="replies"
      isNew={isNew}
      inboxId={inboxId}
      onLeave={() => {
        clear()
        if (isNew) leaveSetup()
      }}
      back={paths.address}
      preview={
        <>
          <MailMock name={name} to={draft.address.trim()} />
          <PreviewCaption>
            <Trans>Each new e-mail starts a conversation; replies continue it.</Trans>
          </PreviewCaption>
        </>
      }
    >
      <StepHeader title={<Trans>How should replies go out?</Trans>}>
        <Trans>Choose the account that sends your answers. You can change it later in the channel settings.</Trans>
      </StepHeader>
      <form
        className="flex flex-col gap-6"
        onSubmit={(e) => {
          e.preventDefault()
          if (method) submit()
        }}
        data-testid="setup-email-replies"
      >
        <ChoiceCards
          label={t`Reply account`}
          value={method}
          onValueChange={pick}
          items={[
            {
              value: "ses",
              icon: <CloudIcon />,
              title: <Trans>Amazon SES</Trans>,
              hint: <Trans>Send through your Amazon SES account with its SMTP credentials</Trans>,
              testId: "setup-replies-ses",
            },
            {
              value: "smtp",
              icon: <ServerIcon />,
              title: <Trans>Another SMTP server</Trans>,
              hint: <Trans>Your mail provider's or your own SMTP server</Trans>,
              testId: "setup-replies-smtp",
            },
            {
              value: "none",
              icon: <DownloadIcon />,
              title: <Trans>Only receive for now</Trans>,
              hint: <Trans>Mail arrives here; add a reply account when you are ready</Trans>,
              testId: "setup-replies-none",
            },
          ]}
        />
        {method === "none" && (
          <Notice tone="warning" className="text-small" data-testid="setup-replies-warning">
            <Trans>Replies cannot go out until you add a reply account.</Trans>
          </Notice>
        )}
        {restored && method !== "none" && !password && (
          <Notice className="text-small" data-testid="setup-password-hint">
            <Trans>The password is not kept when you leave this page. Enter it again.</Trans>
          </Notice>
        )}
        {ses && (
          <>
            <Notice className="text-small">
              <Trans>
                If this server manages Amazon SES, Yuva keeps this inbox's mail in its own tenant; you see the result on
                the next steps.
              </Trans>
            </Notice>
            <Field
              label={<Trans>Region</Trans>}
              htmlFor="ses-region"
              hint={<Trans>The region of your SES account.</Trans>}
              error={error("smtp.host")}
            >
              <ChoiceSelect
                id="ses-region"
                label={t`Region`}
                placeholder={t`Choose a region`}
                value={draft.region}
                onChange={(region) => {
                  setProblems({})
                  patch({ region })
                }}
                options={SES_REGIONS.map((r) => [r, r] as const)}
                className="h-10 w-full rounded-xl"
              />
            </Field>
            <SmtpLoginFields {...props} />
            <p className="-mt-3 text-caption text-muted-foreground">
              <Trans>Use the SMTP credentials from the SES console.</Trans>
            </p>
          </>
        )}
        {method === "smtp" && (
          <>
            <div className="grid gap-5 sm:grid-cols-[1fr_7rem]">
              <SmtpHostField {...props} />
              <SmtpPortField {...props} />
            </div>
            <SmtpEncryptionField {...props} />
            <SmtpLoginFields {...props} />
          </>
        )}
        {(ses || method === "smtp") && (
          <Collapsible open={more || moreBad} onOpenChange={setMore}>
            <CollapsibleTrigger
              className="group -ms-2 inline-flex items-center gap-1.5 rounded-lg px-2 py-1 text-body text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/30"
              data-testid="setup-replies-more"
            >
              <ChevronDownIcon className={cn("size-4 transition-transform", (more || moreBad) && "rotate-180")} />
              <Trans>More options</Trans>
            </CollapsibleTrigger>
            <CollapsibleContent className="mt-4 flex flex-col gap-5">
              {ses && (
                <div className="grid gap-5 sm:grid-cols-[7rem_1fr]">
                  <SmtpPortField {...props} />
                  <SmtpEncryptionField {...props} />
                </div>
              )}
              <SenderFields {...props} />
              {ses && <SesFields {...props} />}
            </CollapsibleContent>
          </Collapsible>
        )}
        {!serverBad && !taken(save.error) && <ErrorLine error={save.error} />}
        <StepActions pending={save.isPending} disabled={!method} testId="setup-add-submit">
          <Trans>Create channel</Trans>
        </StepActions>
      </form>
    </EmailLayout>
  )
}

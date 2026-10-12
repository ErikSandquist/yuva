import { Plural, Trans } from "@lingui/react/macro"
import {
  ArrowRightIcon,
  ArrowUpRightIcon,
  CheckIcon,
  KeyRoundIcon,
  LoaderIcon,
  ShieldCheckIcon,
  XIcon,
} from "lucide-react"
import { useState } from "react"
import { Link, useNavigate, useSearchParams } from "react-router"

import { CodeLine, ErrorLine, Notice } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { FollowsNotice } from "@/features/migrations/parts"
import { useMigrationFolders, useMigrationStep, useStartSignIn } from "@/features/migrations/queries"
import { useGuides, useSignInResultText } from "@/features/migrations/text"
import { WizardLayout } from "@/features/migrations/Wizard"
import { Field } from "@/features/settings/ui"
import { StepHeader } from "@/features/setup/SetupLayout"
import type { Migration, MigrationPreset, MigrationSignInResult } from "@/lib/api"

const RESULTS: MigrationSignInResult[] = [
  "connected",
  "access_denied",
  "expired",
  "session_mismatch",
  "scope_too_broad",
  "not_admin",
  "plan_has_no_api",
  "subdomain_not_found",
  "account_mismatch",
  "token_failed",
  "source_unreachable",
]

const SUBDOMAIN = /^[a-z0-9][a-z0-9-]{0,62}$/

function cleanSubdomain(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/^https?:\/\//, "")
    .replace(/\/.*$/, "")
    .replace(/\.zendesk\.com$/, "")
}

export function HelpdeskConnectStep({ m, preset }: { m: Migration; preset: MigrationPreset }) {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const guide = useGuides()[m.preset]
  const resultText = useSignInResultText()
  const provider = preset.name
  const zendesk = m.source === "zendesk"
  const storedHost = m.credentials?.server.host ?? ""
  const [subdomain, setSubdomain] = useState(storedHost.endsWith(".zendesk.com") ? storedHost.split(".")[0]! : "")
  const [clientId, setClientId] = useState("")
  const [secret, setSecret] = useState("")
  const signIn = useStartSignIn(m.id)
  const resume = useMigrationStep(m.id, "resume")
  const reconnecting = m.state === "needs_attention"
  const canSign = m.actions.includes("sign_in")
  const connected = !!m.credentials && (m.state === "connected" || m.state === "scanning" || m.state === "ready")
  const folders = useMigrationFolders(m.id, connected)
  const fromUrl = params.get("result") as MigrationSignInResult | null
  const result = fromUrl && RESULTS.includes(fromUrl) ? fromUrl : m.sign_in_result
  const failed = result && result !== "connected" ? result : undefined
  const reconnected = reconnecting && fromUrl === "connected"
  const sub = cleanSubdomain(subdomain)
  const valid = clientId.trim() !== "" && secret.trim() !== "" && (!preset.subdomain || SUBDOMAIN.test(sub))
  const scopes = (preset.scopes ?? []).join(", ")
  const redirect = preset.redirect_url ?? ""
  const count = folders.data?.length ?? 0
  const shown = failed ? resultText(failed, provider) : undefined

  const start = () => {
    if (!valid) return
    signIn.mutate(
      {
        client_id: clientId.trim(),
        client_secret: secret.trim(),
        ...(preset.subdomain ? { subdomain: sub } : {}),
      },
      { onSuccess: (r) => window.location.assign(r.authorize_url) }
    )
  }

  return (
    <WizardLayout step="connect" m={m} preset={preset}>
      <StepHeader title={reconnecting ? <Trans>Reconnect {provider}</Trans> : <Trans>Connect {provider}</Trans>}>
        {reconnecting ? (
          <Trans>
            {provider} stopped accepting Yuva's sign-in. Sign in again to the same account; the migration continues
            where it stopped.
          </Trans>
        ) : (
          <Trans>
            Yuva signs in to {provider} with an OAuth client you create in your own account, and only reads.
          </Trans>
        )}
      </StepHeader>

      {!reconnecting && <FollowsNotice m={m} className="mb-6" />}

      {reconnecting && m.state_reason && !reconnected && (
        <Notice tone="danger" icon={XIcon} className="mb-6">
          <p>{m.state_reason}</p>
        </Notice>
      )}

      {shown && failed && (
        <Notice
          tone="danger"
          icon={XIcon}
          className="mb-6"
          role="alert"
          data-testid="migration-sign-in-result"
          data-result={failed}
        >
          <p className="font-medium">{shown.title}</p>
          <p>{shown.text}</p>
        </Notice>
      )}

      {connected && (
        <Notice icon={CheckIcon} className="mb-6" role="status" data-testid="migration-connected">
          <p className="font-medium text-foreground">
            <Trans>Connected to {provider}</Trans>
            {folders.data && (
              <>
                {" · "}
                {zendesk ? (
                  <Plural value={count} one="# brand" other="# brands" />
                ) : (
                  <Plural value={count} one="# mailbox" other="# mailboxes" />
                )}
              </>
            )}
          </p>
          <p>
            <Trans>If an assistant sent you here, you can go back to it now, or choose what to bring yourself.</Trans>
          </p>
          <Button
            size="sm"
            className="self-start"
            render={<Link to={`/migrations/${m.id}/choose`} />}
            data-testid="migration-connect-continue"
          >
            <Trans>Continue</Trans>
            <ArrowRightIcon className="rtl:rotate-180" />
          </Button>
        </Notice>
      )}

      {reconnected && (
        <Notice icon={CheckIcon} className="mb-6" role="status" data-testid="migration-connected">
          <p className="font-medium text-foreground">
            <Trans>Signed in to {provider} again</Trans>
          </p>
          <ErrorLine error={resume.error} />
          {m.actions.includes("resume") && (
            <Button
              size="sm"
              className="self-start"
              onClick={() => resume.mutate(undefined, { onSuccess: (next) => navigate(`/migrations/${next.id}`) })}
              disabled={resume.isPending}
              data-testid="migration-resume"
            >
              {resume.isPending && <LoaderIcon className="animate-spin" />}
              <Trans>Continue the migration</Trans>
              <ArrowRightIcon className="rtl:rotate-180" />
            </Button>
          )}
        </Notice>
      )}

      <section className="mb-6 rounded-2xl border bg-card p-4" aria-labelledby="migration-guide">
        <h2 id="migration-guide" className="flex items-center gap-2 text-body font-medium">
          <KeyRoundIcon className="size-4 text-faint" />
          {zendesk ? <Trans>Create an OAuth client in Zendesk</Trans> : <Trans>Create an app in Help Scout</Trans>}
        </h2>
        <ol className="mt-3 flex list-decimal flex-col gap-2 ps-5 text-body text-muted-foreground marker:text-faint">
          {guide.steps.map((s) => (
            <li key={s}>{s}</li>
          ))}
        </ol>
        {redirect && (
          <div className="mt-4 flex flex-col gap-1.5">
            <span className="text-small font-medium">
              <Trans>Redirect URL</Trans>
            </span>
            <CodeLine value={redirect} testId="migration-redirect-url" />
          </div>
        )}
        <Button
          variant="outline"
          size="sm"
          className="mt-4"
          render={<a href={guide.link} target="_blank" rel="noreferrer" />}
          data-testid="migration-guide-link"
        >
          {guide.linkLabel}
          <ArrowUpRightIcon className="rtl:-scale-x-100" />
        </Button>
      </section>

      {canSign && (
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            start()
          }}
        >
          {connected && (
            <p className="text-small text-faint">
              <Trans>Sign in again only to change the client or the account's access.</Trans>
            </p>
          )}
          {preset.subdomain && (
            <Field label={<Trans>Subdomain</Trans>} htmlFor="migration-subdomain">
              <div className="flex min-w-0 items-center gap-2">
                <Input
                  id="migration-subdomain"
                  autoComplete="off"
                  spellCheck={false}
                  value={subdomain}
                  onChange={(e) => setSubdomain(e.target.value)}
                  placeholder="acme"
                  aria-invalid={subdomain !== "" && !SUBDOMAIN.test(sub)}
                  className="h-10 min-w-0 flex-1 rounded-xl"
                  data-testid="migration-subdomain"
                />
                <span className="shrink-0 text-body text-faint">.zendesk.com</span>
              </div>
            </Field>
          )}
          <Field
            label={zendesk ? <Trans>Client identifier</Trans> : <Trans>App ID</Trans>}
            htmlFor="migration-client-id"
          >
            <Input
              id="migration-client-id"
              autoComplete="off"
              spellCheck={false}
              value={clientId}
              maxLength={200}
              onChange={(e) => setClientId(e.target.value)}
              className="h-10 rounded-xl font-mono"
              data-testid="migration-client-id"
            />
          </Field>
          <Field
            label={zendesk ? <Trans>Client secret</Trans> : <Trans>App Secret</Trans>}
            htmlFor="migration-client-secret"
            hint={<Trans>Stored encrypted and never shown again, not even to you.</Trans>}
          >
            <Input
              id="migration-client-secret"
              type="password"
              autoComplete="new-password"
              value={secret}
              maxLength={500}
              onChange={(e) => setSecret(e.target.value)}
              className="h-10 rounded-xl font-mono"
              data-testid="migration-client-secret"
            />
          </Field>

          <Notice icon={ShieldCheckIcon} data-testid="migration-read-only">
            <p>
              <Trans>
                Yuva only reads. Besides signing in, its connection sends {provider} nothing but read requests, so no
                ticket, customer or setting changes.
              </Trans>
            </p>
            <p className="text-small">
              {zendesk ? (
                <Trans>Yuva asks for the scope {scopes} and refuses a sign-in that grants more.</Trans>
              ) : (
                <Trans>
                  Help Scout has no read-only permission, so the app could change data; Yuva's connection refuses every
                  request that is not a read.
                </Trans>
              )}{" "}
              <Trans>Yuva deletes its token when the migration ends.</Trans>
            </p>
          </Notice>

          <ErrorLine error={signIn.error} />
          <Button
            type="submit"
            size="lg"
            className="self-start"
            disabled={!valid || signIn.isPending || signIn.isSuccess}
            data-testid="migration-sign-in"
          >
            {(signIn.isPending || signIn.isSuccess) && <LoaderIcon className="animate-spin" />}
            {zendesk ? <Trans>Sign in with Zendesk</Trans> : <Trans>Sign in with Help Scout</Trans>}
            <ArrowUpRightIcon className="rtl:-scale-x-100" />
          </Button>
        </form>
      )}
    </WizardLayout>
  )
}

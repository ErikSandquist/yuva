import { Plural, Trans, useLingui } from "@lingui/react/macro"
import {
  ArrowRightIcon,
  ArrowUpRightIcon,
  CheckIcon,
  CopyIcon,
  InfoIcon,
  KeyRoundIcon,
  LoaderIcon,
  ShieldCheckIcon,
  XIcon,
} from "lucide-react"
import { useState } from "react"
import { Link, useNavigate, useSearchParams } from "react-router"

import { CodeLine, CopyButton, ErrorLine, Notice, StepHeader, useCopy } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { FollowsNotice } from "@/features/migrations/parts"
import { useMigrationFolders, useMigrationStep, useStartSignIn } from "@/features/migrations/queries"
import {
  guideUrl,
  isPersonalMicrosoft,
  useGuides,
  useMicrosoftAppSteps,
  useSignInResultText,
} from "@/features/migrations/text"
import { WizardLayout } from "@/features/migrations/Wizard"
import { Field } from "@/features/settings/ui"
import type { Migration, MigrationPreset, MigrationSignInResult, MigrationSignInStart } from "@/lib/api"

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
  "consent_required",
  "admin_consent_required",
  "conditional_access",
  "xoauth2_failed",
  "admin_consented",
]

const SUBDOMAIN = /^[a-z0-9][a-z0-9-]{0,62}$/
const TENANT = /^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$/
const ENTRA_APPS = "https://entra.microsoft.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade"

function cleanSubdomain(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/^https?:\/\//, "")
    .replace(/\/.*$/, "")
    .replace(/\.zendesk\.com$/, "")
}

export function SignInConnectStep({ m, preset }: { m: Migration; preset: MigrationPreset }) {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const guide = useGuides()[m.preset]
  const appSteps = useMicrosoftAppSteps()
  const resultText = useSignInResultText()
  const microsoft = m.preset === "microsoft"
  const team = !microsoft || preset.team_client === true
  const provider = microsoft ? "Microsoft" : preset.name
  const address = m.source_address
  const zendesk = m.source === "zendesk"
  const storedHost = m.credentials?.server.host ?? ""
  const [subdomain, setSubdomain] = useState(storedHost.endsWith(".zendesk.com") ? storedHost.split(".")[0]! : "")
  const [clientId, setClientId] = useState("")
  const [secret, setSecret] = useState("")
  const [tenant, setTenant] = useState("")
  const signIn = useStartSignIn(m.id)
  const resume = useMigrationStep(m.id, "resume")
  const reconnecting = m.state === "needs_attention"
  const canSign = m.actions.includes("sign_in")
  const connected = !!m.credentials && (m.state === "connected" || m.state === "scanning" || m.state === "ready")
  const folders = useMigrationFolders(m.id, connected)
  const fromUrl = params.get("result") as MigrationSignInResult | null
  const result = fromUrl && RESULTS.includes(fromUrl) ? fromUrl : m.sign_in_result
  const failed = result && result !== "connected" && result !== "admin_consented" ? result : undefined
  const reconnected = reconnecting && fromUrl === "connected"
  const sub = cleanSubdomain(subdomain)
  const tenantId = tenant.trim()
  const valid =
    !team ||
    (clientId.trim() !== "" &&
      secret.trim() !== "" &&
      (!preset.subdomain || SUBDOMAIN.test(sub)) &&
      (tenantId === "" || TENANT.test(tenantId)))
  const scopes = (preset.scopes ?? []).join(", ")
  const redirect = preset.redirect_url ?? ""
  const count = folders.data?.length ?? 0
  const app = microsoft ? (team ? "team" : "server") : undefined
  const shown = failed ? resultText(failed, provider, app) : undefined
  const consented = result === "admin_consented" && !connected ? resultText(result, provider, app) : undefined
  const adminUrl = failed === "admin_consent_required" ? m.admin_consent_url : undefined

  const start = () => {
    if (!valid) return
    const body: MigrationSignInStart = team
      ? {
          client_id: clientId.trim(),
          client_secret: secret.trim(),
          ...(preset.subdomain ? { subdomain: sub } : {}),
          ...(microsoft && tenantId ? { tenant: tenantId } : {}),
        }
      : {}
    signIn.mutate(body, { onSuccess: (r) => window.location.assign(r.authorize_url) })
  }

  return (
    <WizardLayout step="connect" m={m} preset={preset}>
      <StepHeader
        title={
          microsoft && reconnecting ? (
            <Trans>Sign in again to {address}</Trans>
          ) : microsoft ? (
            <Trans>Connect {address}</Trans>
          ) : reconnecting ? (
            <Trans>Reconnect {provider}</Trans>
          ) : (
            <Trans>Connect {provider}</Trans>
          )
        }
      >
        {microsoft && reconnecting ? (
          <Trans>
            Microsoft stopped accepting Yuva's sign-in. Sign in with Microsoft again as {address}; the migration
            continues where it stopped.
          </Trans>
        ) : microsoft && team ? (
          <Trans>
            Outlook.com and Microsoft 365 accept no passwords. Register an app for Yuva in Microsoft Entra, then sign in
            with Microsoft as {address}: Yuva gets a token only for reading the mailbox over IMAP.
          </Trans>
        ) : microsoft ? (
          <Trans>
            Outlook.com and Microsoft 365 accept no passwords. Sign in with Microsoft as {address}: Yuva gets a token
            only for reading the mailbox over IMAP.
          </Trans>
        ) : reconnecting ? (
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
          {adminUrl && <AdminConsent url={adminUrl} address={address} />}
        </Notice>
      )}

      {consented && (
        <Notice icon={InfoIcon} className="mb-6" role="status" data-testid="migration-sign-in-result">
          <p className="font-medium text-foreground">{consented.title}</p>
          <p>{consented.text}</p>
        </Notice>
      )}

      {connected && (
        <Notice icon={CheckIcon} className="mb-6" role="status" data-testid="migration-connected">
          <p className="font-medium text-foreground">
            {microsoft ? <Trans>Connected to {address}</Trans> : <Trans>Connected to {provider}</Trans>}
            {folders.data && (
              <>
                {" · "}
                {microsoft ? (
                  <Plural value={count} one="# folder" other="# folders" />
                ) : zendesk ? (
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

      {team && (
        <section className="mb-6 rounded-2xl border bg-card p-4" aria-labelledby="migration-guide">
          <h2 id="migration-guide" className="flex items-center gap-2 text-body font-medium">
            <KeyRoundIcon className="size-4 text-faint" />
            {microsoft ? (
              <Trans>Register an app in Microsoft Entra</Trans>
            ) : zendesk ? (
              <Trans>Create an OAuth client in Zendesk</Trans>
            ) : (
              <Trans>Create an app in Help Scout</Trans>
            )}
          </h2>
          <ol className="mt-3 flex list-decimal flex-col gap-2 ps-5 text-body text-muted-foreground marker:text-faint">
            {(microsoft ? appSteps(isPersonalMicrosoft(address)) : guide.steps).map((s) => (
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
          <div className="mt-4 flex flex-wrap gap-2">
            {microsoft ? (
              <>
                <Button
                  variant="outline"
                  size="sm"
                  render={<a href={ENTRA_APPS} target="_blank" rel="noreferrer" />}
                  data-testid="migration-guide-link"
                >
                  <Trans>Open App registrations</Trans>
                  <ArrowUpRightIcon className="rtl:-scale-x-100" />
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  render={<a href={guideUrl(preset.guide)} target="_blank" rel="noreferrer" />}
                >
                  <Trans>Full guide</Trans>
                </Button>
              </>
            ) : (
              <Button
                variant="outline"
                size="sm"
                render={<a href={guide.link} target="_blank" rel="noreferrer" />}
                data-testid="migration-guide-link"
              >
                {guide.linkLabel}
                <ArrowUpRightIcon className="rtl:-scale-x-100" />
              </Button>
            )}
          </div>
        </section>
      )}

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
              {microsoft ? (
                <Trans>Sign in again only to change the app or the account's access.</Trans>
              ) : (
                <Trans>Sign in again only to change the client or the account's access.</Trans>
              )}
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
          {team && (
            <>
              <Field
                label={
                  microsoft ? (
                    <Trans>Application (client) ID</Trans>
                  ) : zendesk ? (
                    <Trans>Client identifier</Trans>
                  ) : (
                    <Trans>App ID</Trans>
                  )
                }
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
                label={zendesk || microsoft ? <Trans>Client secret</Trans> : <Trans>App Secret</Trans>}
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
              {microsoft && (
                <Field
                  label={<Trans>Directory (tenant) ID, optional</Trans>}
                  htmlFor="migration-tenant"
                  hint={
                    <Trans>
                      Only for an app limited to your organization: its Directory (tenant) ID or a verified domain.
                      Leave it empty otherwise.
                    </Trans>
                  }
                >
                  <Input
                    id="migration-tenant"
                    autoComplete="off"
                    spellCheck={false}
                    value={tenant}
                    maxLength={253}
                    onChange={(e) => setTenant(e.target.value)}
                    aria-invalid={tenantId !== "" && !TENANT.test(tenantId)}
                    className="h-10 rounded-xl font-mono"
                    data-testid="migration-tenant"
                  />
                </Field>
              )}
            </>
          )}

          <Notice icon={ShieldCheckIcon} data-testid="migration-read-only">
            {microsoft ? (
              <p>
                <Trans>
                  Microsoft's consent page asks you to allow “Read and write access to mailboxes via IMAP”, because
                  Microsoft has no read-only IMAP permission. Yuva still only reads: its connection refuses every
                  command that could delete, move or mark mail, and Yuva deletes its token when the migration ends.
                </Trans>
              </p>
            ) : (
              <>
                <p>
                  <Trans>
                    Yuva only reads. Besides signing in, its connection sends {provider} nothing but read requests, so
                    no ticket, customer or setting changes.
                  </Trans>
                </p>
                <p className="text-small">
                  {zendesk ? (
                    <Trans>Yuva asks for the scope {scopes} and refuses a sign-in that grants more.</Trans>
                  ) : (
                    <Trans>
                      Help Scout has no read-only permission, so the app could change data; Yuva's connection refuses
                      every request that is not a read.
                    </Trans>
                  )}{" "}
                  <Trans>Yuva deletes its token when the migration ends.</Trans>
                </p>
              </>
            )}
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
            {microsoft && (reconnecting || failed) ? (
              <Trans>Sign in again</Trans>
            ) : microsoft ? (
              <Trans>Sign in with Microsoft</Trans>
            ) : zendesk ? (
              <Trans>Sign in with Zendesk</Trans>
            ) : (
              <Trans>Sign in with Help Scout</Trans>
            )}
            <ArrowUpRightIcon className="rtl:-scale-x-100" />
          </Button>
        </form>
      )}
    </WizardLayout>
  )
}

function AdminConsent({ url, address }: { url: string; address: string }) {
  const { t } = useLingui()
  const [copied, copy] = useCopy()
  const message = t`Hello, I am moving the mailbox ${address} to Yuva, our support inbox. Our Microsoft 365 organization lets only an admin allow apps. Please open this link as an admin and accept, so Yuva may read the mailbox over IMAP: ${url}`
  return (
    <div className="mt-1 flex flex-col gap-2" data-testid="migration-admin-consent">
      <span className="text-small font-medium text-foreground">
        <Trans>Link for your Microsoft 365 admin</Trans>
      </span>
      <div className="flex min-w-0 items-center gap-2">
        <a
          href={url}
          target="_blank"
          rel="noreferrer"
          className="min-w-0 flex-1 rounded-xl border bg-surface px-3 py-2 font-mono text-caption break-all text-foreground underline-offset-2 hover:underline"
          data-testid="migration-admin-consent-url"
        >
          {url}
        </a>
        <CopyButton value={url} />
      </div>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="self-start"
        onClick={() => void copy(message)}
        data-testid="migration-admin-consent-message"
      >
        {copied ? <CheckIcon /> : <CopyIcon />}
        {copied ? <Trans>Message copied</Trans> : <Trans>Copy a message for your admin</Trans>}
      </Button>
    </div>
  )
}

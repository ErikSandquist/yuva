import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation } from "@tanstack/react-query"
import {
  ArrowRightIcon,
  ArrowUpRightIcon,
  CheckIcon,
  ChevronDownIcon,
  CircleIcon,
  KeyRoundIcon,
  LoaderIcon,
  PackageOpenIcon,
  ShieldCheckIcon,
  XIcon,
} from "lucide-react"
import { Fragment, useState } from "react"
import { Link, useLocation, useNavigate } from "react-router"

import { ErrorLine, Notice, StepHeader } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useNumber } from "@/features/broadcasts/BroadcastParts"
import { FollowsNotice } from "@/features/migrations/parts"
import {
  useMigrationStep,
  usePresets,
  useReplaceDraft,
  useSetCredentials,
  useTestConnection,
} from "@/features/migrations/queries"
import type { ConnectState } from "@/features/migrations/SourceStep"
import { guideUrl, TEST_STEPS, useGuides, useMigrationText, useTestCodeText } from "@/features/migrations/text"
import { WizardLayout } from "@/features/migrations/Wizard"
import { ChoiceSelect, Field } from "@/features/settings/ui"
import type {
  Migration,
  MigrationCredentialsInput,
  MigrationPreset,
  MigrationTestResult,
  MigrationTls,
} from "@/lib/api"
import { cn } from "@/lib/utils"

function fingerprint(hex: string) {
  return hex.match(/.{2}/g)?.join(":").toUpperCase() ?? hex
}

export function ConnectStep({ m, preset }: { m: Migration; preset: MigrationPreset }) {
  const { t } = useLingui()
  const location = useLocation()
  const detected = ((location.state ?? {}) as ConnectState).server
  const guides = useGuides()
  const guide = guides[m.preset]
  const provider = preset.name
  const custom = preset.servers.length === 0
  const several = preset.servers.length > 1
  const stored = m.credentials?.server
  const start = stored ?? detected ?? preset.servers[0] ?? { host: "", port: 993, tls: "tls" as MigrationTls }
  const [username, setUsername] = useState(
    preset.username === "local_part" ? m.source_address.split("@")[0]! : m.source_address
  )
  const [password, setPassword] = useState("")
  const [host, setHost] = useState(start.host)
  const [port, setPort] = useState(String(start.port))
  const [tls, setTls] = useState<MigrationTls>(start.tls)
  const [serverOpen, setServerOpen] = useState(custom || several)
  const [trust, setTrust] = useState(false)
  const [result, setResult] = useState<MigrationTestResult | null>(null)
  const credentials = useSetCredentials(m.id)
  const test = useTestConnection(m.id)
  const resume = useMigrationStep(m.id, "resume")
  const reconnecting = m.state === "needs_attention"
  const canSet = m.actions.includes("set_credentials")
  const connected = !!m.credentials && (m.state === "connected" || m.state === "scanning" || m.state === "ready")
  const pinned = result?.certificate_sha256

  const run = useMutation({
    mutationFn: async () => {
      setResult(null)
      const body: MigrationCredentialsInput = { username: username.trim(), password }
      if (custom || several) body.host = host.trim()
      if (custom) {
        body.port = Number(port) || 993
        body.tls = tls
      }
      if (trust && pinned) body.certificate_sha256 = pinned
      else if (m.credentials?.certificate_sha256 && host.trim() === stored?.host)
        body.certificate_sha256 = m.credentials.certificate_sha256
      await credentials.mutateAsync(body)
      const r = await test.mutateAsync()
      setResult(r)
      if (r.ok && reconnecting && r.migration.actions.includes("resume")) await resume.mutateAsync()
      return r
    },
  })

  const ok = result?.ok === true
  const valid = username.trim() !== "" && password !== "" && (!custom || (host.trim() !== "" && Number(port) > 0))
  const address = m.source_address
  const storedHost = stored?.host ?? ""
  return (
    <WizardLayout step="connect" m={m} preset={preset}>
      <StepHeader title={reconnecting ? <Trans>Reconnect {address}</Trans> : <Trans>Connect {address}</Trans>}>
        {reconnecting ? (
          <Trans>
            {provider} stopped accepting Yuva's sign-in. Enter a new app password; the migration continues where it
            stopped.
          </Trans>
        ) : preset.app_password ? (
          <Trans>{provider} lets other apps in with an app password, a separate password just for Yuva.</Trans>
        ) : (
          <Trans>Sign in with the mailbox's user name and password.</Trans>
        )}
      </StepHeader>

      {!reconnecting && <FollowsNotice m={m} className="mb-6" />}

      {m.state === "draft" && !m.follows_migration_id && (m.preset === "gmail" || m.preset === "google_workspace") && (
        <TakeoutOffer m={m} />
      )}

      {reconnecting && m.state_reason && (
        <Notice tone="danger" icon={XIcon} className="mb-6">
          <p>{m.state_reason}</p>
        </Notice>
      )}

      {connected && !ok && (
        <Notice icon={CheckIcon} className="mb-6" data-testid="migration-already-connected">
          <p>
            <Trans>Connected to {storedHost}. Enter the password again only to change it.</Trans>
          </p>
          <Button
            size="sm"
            className="self-start"
            render={<Link to={`/migrations/${m.id}/choose`} />}
            data-testid="migration-connect-skip"
          >
            <Trans>Continue</Trans>
            <ArrowRightIcon className="rtl:rotate-180" />
          </Button>
        </Notice>
      )}

      <section className="mb-6 rounded-2xl border bg-card p-4" aria-labelledby="migration-guide">
        <h2 id="migration-guide" className="flex items-center gap-2 text-body font-medium">
          <KeyRoundIcon className="size-4 text-faint" />
          {preset.app_password ? <Trans>Use an app password</Trans> : <Trans>Before you connect</Trans>}
        </h2>
        <ol className="mt-3 flex list-decimal flex-col gap-2 ps-5 text-body text-muted-foreground marker:text-faint">
          {guide.steps.map((s) => (
            <li key={s}>{s}</li>
          ))}
        </ol>
        <div className="mt-4 flex flex-wrap gap-2">
          <Button
            variant="outline"
            size="sm"
            render={<a href={guide.link} target="_blank" rel="noreferrer" />}
            data-testid="migration-guide-link"
          >
            {guide.linkLabel}
            <ArrowUpRightIcon />
          </Button>
          {guide.link !== guideUrl(preset.guide) && (
            <Button
              variant="ghost"
              size="sm"
              render={<a href={guideUrl(preset.guide)} target="_blank" rel="noreferrer" />}
            >
              <Trans>Full guide</Trans>
            </Button>
          )}
        </div>
      </section>

      {canSet && (
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            if (valid) run.mutate()
          }}
        >
          <Field label={<Trans>User name</Trans>} htmlFor="migration-username">
            <Input
              id="migration-username"
              autoComplete="off"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="h-10 rounded-xl"
              data-testid="migration-username"
            />
          </Field>
          <Field
            label={preset.app_password ? <Trans>App password</Trans> : <Trans>Password</Trans>}
            htmlFor="migration-password"
          >
            <Input
              id="migration-password"
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="h-10 rounded-xl font-mono"
              data-testid="migration-password"
            />
          </Field>

          {(custom || several) && (
            <div className="flex flex-col gap-4">
              <button
                type="button"
                className="-ms-1 flex w-fit items-center gap-1.5 rounded-md px-1 text-body font-medium outline-none focus-visible:ring-3 focus-visible:ring-ring/30"
                aria-expanded={serverOpen}
                aria-controls="migration-server"
                onClick={() => setServerOpen((o) => !o)}
              >
                <ChevronDownIcon className={cn("size-4 transition-transform", !serverOpen && "-rotate-90")} />
                <Trans>Server settings</Trans>
              </button>
              {serverOpen && (
                <div id="migration-server" className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_6rem_8rem]">
                  {several ? (
                    <Field label={<Trans>Server</Trans>} htmlFor="migration-host" className="sm:col-span-3">
                      <ChoiceSelect
                        id="migration-host"
                        label={t`Server`}
                        value={host}
                        onChange={setHost}
                        options={preset.servers.map((s) => [s.host, s.host] as const)}
                        className="h-10 w-full rounded-xl"
                      />
                    </Field>
                  ) : (
                    <>
                      <Field label={<Trans>IMAP server</Trans>} htmlFor="migration-host">
                        <Input
                          id="migration-host"
                          value={host}
                          onChange={(e) => setHost(e.target.value)}
                          placeholder="imap.example.com"
                          className="h-10 rounded-xl"
                          data-testid="migration-host"
                        />
                      </Field>
                      <Field label={<Trans>Port</Trans>} htmlFor="migration-port">
                        <Input
                          id="migration-port"
                          inputMode="numeric"
                          value={port}
                          onChange={(e) => setPort(e.target.value.replace(/\D/g, ""))}
                          className="h-10 rounded-xl"
                        />
                      </Field>
                      <Field label={<Trans>Encryption</Trans>} htmlFor="migration-tls">
                        <ChoiceSelect
                          id="migration-tls"
                          label={t`Encryption`}
                          value={tls}
                          onChange={(v) => {
                            setTls(v)
                            if (v === "tls" && port === "143") setPort("993")
                            if (v === "starttls" && port === "993") setPort("143")
                          }}
                          options={[
                            ["tls", "TLS"],
                            ["starttls", "STARTTLS"],
                          ]}
                          className="h-10 w-full rounded-xl"
                        />
                      </Field>
                    </>
                  )}
                </div>
              )}
            </div>
          )}

          <Notice icon={ShieldCheckIcon} data-testid="migration-read-only">
            <p>
              <Trans>
                Yuva only reads. It never deletes, moves, archives or marks anything as read in {provider}, and your
                mail stays where it is.
              </Trans>
            </p>
            {preset.app_password && (
              <p className="text-small">
                <Trans>
                  An app password could be used to change mail; Yuva's connection refuses every command that changes
                  anything. Yuva deletes this password when the migration ends; you can also revoke it in {provider}{" "}
                  afterwards.
                </Trans>
              </p>
            )}
          </Notice>

          <TestSteps pending={run.isPending} result={result} provider={provider} host={host || start.host} />

          {pinned && !ok && (
            <div className="flex flex-col gap-2 rounded-xl border p-3.5" data-testid="migration-certificate">
              <p className="text-small text-faint">
                <Trans>SHA-256 fingerprint of the certificate the server sent</Trans>
              </p>
              <code className="font-mono text-caption">
                {fingerprint(pinned)
                  .split(":")
                  .map((octet, i) => (
                    <Fragment key={i}>
                      {i > 0 && (
                        <>
                          :<wbr />
                        </>
                      )}
                      {octet}
                    </Fragment>
                  ))}
              </code>
              <Label className="mt-1 gap-3 font-normal">
                <Checkbox checked={trust} onCheckedChange={setTrust} data-testid="migration-trust" />
                <span>
                  <Trans>
                    Trust this certificate for this server. Do this only if it is your own server and the fingerprint
                    matches.
                  </Trans>
                </span>
              </Label>
            </div>
          )}

          <ErrorLine error={run.error} />

          {ok ? (
            <Connected m={m} result={result!} reconnected={reconnecting} />
          ) : (
            <Button
              type="submit"
              size="lg"
              className="self-start"
              disabled={!valid || run.isPending}
              data-testid="migration-test"
            >
              {run.isPending && <LoaderIcon className="animate-spin" />}
              {result ? <Trans>Test again</Trans> : <Trans>Test connection</Trans>}
            </Button>
          )}
        </form>
      )}
    </WizardLayout>
  )
}

function TakeoutOffer({ m }: { m: Migration }) {
  const navigate = useNavigate()
  const takeout = usePresets().data?.find((p) => p.id === "takeout")
  const replace = useReplaceDraft()
  if (!takeout?.available) return null
  const start = () =>
    replace.mutate(
      {
        draft: m,
        body: {
          source: "file",
          preset: "takeout",
          source_address: m.source_address,
          ...(m.inbox_id ? { inbox_id: m.inbox_id } : {}),
          ...(m.channel_id ? { channel_id: m.channel_id } : {}),
        },
      },
      { onSuccess: (next) => navigate(`/migrations/${next.id}/connect`) }
    )
  return (
    <Notice icon={PackageOpenIcon} className="mb-6" data-testid="migration-takeout-offer">
      <p className="font-medium text-foreground">
        <Trans>Very large mailbox? Start with a Google Takeout file</Trans>
      </p>
      <p>
        <Trans>
          Gmail lets other apps read about 1.5 GB a day over IMAP, so a mailbox of many gigabytes takes days. Google
          Takeout exports it as one file that uploads as fast as your connection allows. Import that first, then come
          back here with an app password for the newer mail and the switch.
        </Trans>
      </p>
      <ErrorLine error={replace.error} />
      <Button
        variant="outline"
        size="sm"
        className="self-start"
        onClick={start}
        disabled={replace.isPending}
        data-testid="migration-takeout-start"
      >
        {replace.isPending && <LoaderIcon className="animate-spin" />}
        <Trans>Upload a Takeout file instead</Trans>
      </Button>
    </Notice>
  )
}

function TestSteps({
  pending,
  result,
  provider,
  host,
}: {
  pending: boolean
  result: MigrationTestResult | null
  provider: string
  host: string
}) {
  const { t } = useLingui()
  const text = useMigrationText()
  const codeText = useTestCodeText()
  if (!pending && !result) return null
  const ran = new Map((result?.steps ?? []).map((s) => [s.step, s]))
  const failed = result?.steps.find((s) => !s.ok)
  return (
    <div className="flex flex-col gap-3" data-testid="migration-test-steps">
      <p role="status" aria-live="polite" className="text-body font-medium">
        {pending ? (
          <Trans>Testing the connection…</Trans>
        ) : result?.ok ? (
          <Trans>Every step passed.</Trans>
        ) : (
          <Trans>The test stopped at a step.</Trans>
        )}
      </p>
      <ol className="flex flex-col divide-y rounded-xl border">
        {TEST_STEPS.map((step) => {
          const s = ran.get(step)
          const status = pending ? "running" : !s ? "skipped" : s.ok ? "ok" : "failed"
          return (
            <li key={step} className="flex items-center gap-3 px-3.5 py-2.5 text-body" data-state={status}>
              {status === "ok" ? (
                <CheckIcon className="size-4 shrink-0 text-success" aria-hidden="true" />
              ) : status === "failed" ? (
                <XIcon className="size-4 shrink-0 text-destructive" aria-hidden="true" />
              ) : status === "running" ? (
                <LoaderIcon className="size-4 shrink-0 animate-spin text-faint" aria-hidden="true" />
              ) : (
                <CircleIcon className="size-3 shrink-0 text-faint" aria-hidden="true" />
              )}
              <span className={cn("min-w-0 flex-1", status === "skipped" && "text-faint")}>{text.step[step]}</span>
              <span className="shrink-0 text-small text-faint">
                {status === "ok"
                  ? t`Passed`
                  : status === "failed"
                    ? t`Failed`
                    : status === "running"
                      ? t`Checking`
                      : t`Not run`}
              </span>
            </li>
          )
        })}
      </ol>
      {failed?.code && (
        <Notice tone="danger" icon={XIcon} data-testid="migration-test-error" data-code={failed.code}>
          <p>{codeText(failed.code, provider, host)}</p>
          {failed.detail && (
            <details className="text-small">
              <summary className="cursor-pointer text-faint">
                <Trans>Details</Trans>
              </summary>
              <code className="mt-1.5 block font-mono text-caption break-all">{failed.detail}</code>
            </details>
          )}
        </Notice>
      )}
    </div>
  )
}

function Connected({ m, result, reconnected }: { m: Migration; result: MigrationTestResult; reconnected: boolean }) {
  const num = useNumber()
  const address = m.source_address
  const folders = num(result.folders ?? 0)
  const messages = num(result.messages ?? 0)
  return (
    <Notice icon={CheckIcon} data-testid="migration-connected">
      <p className="font-medium text-foreground">
        <Trans>
          Connected to {address} · {folders} folders · {messages} messages
        </Trans>
      </p>
      {reconnected ? (
        <>
          <p>
            <Trans>The migration continues where it stopped.</Trans>
          </p>
          <Button size="sm" className="self-start" render={<Link to="/settings/migrations" />}>
            <Trans>Back to the migrations</Trans>
          </Button>
        </>
      ) : (
        <>
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
        </>
      )}
    </Notice>
  )
}

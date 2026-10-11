import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { ArrowLeftIcon, KeyRoundIcon, MailIcon } from "lucide-react"
import { useState } from "react"

import { ErrorLine } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { api, unwrap, type Locale, type Me } from "@/lib/api"
import { signInWithPasskey } from "@/lib/passkey"
import { meKey } from "@/lib/session"

const field = "h-11 rounded-xl px-3.5"
const alert = "rounded-xl border border-destructive/25 bg-destructive/5 px-3.5 py-2.5 text-body text-destructive"

export function SignInForm({
  title,
  lead,
  reader,
  note,
  onSignedIn,
}: {
  title: React.ReactNode
  lead: React.ReactNode
  reader?: boolean
  note?: React.ReactNode
  onSignedIn?: (me: Me) => void
}) {
  const { t, i18n } = useLingui()
  const qc = useQueryClient()
  const [email, setEmail] = useState("")
  const [code, setCode] = useState("")
  const [step, setStep] = useState<"email" | "code">("email")
  const [resent, setResent] = useState(false)

  const done = (data: Me) => {
    qc.setQueryData(meKey, data)
    onSignedIn?.(data)
  }

  const request = useMutation({
    mutationFn: () => {
      const locale: Locale = i18n.locale === "tr" ? "tr" : "en"
      return unwrap(api.POST("/v1/auth/code", { body: reader ? { email, reader: true, locale } : { email, locale } }))
    },
    onSuccess: () => {
      setResent(step === "code")
      setStep("code")
    },
  })
  const verify = useMutation({
    mutationFn: () => unwrap(api.POST("/v1/auth/code/verify", { body: { email, code } })),
    onSuccess: done,
  })
  const passkey = useMutation({ mutationFn: signInWithPasskey, onSuccess: done })

  return (
    <>
      <div className="mb-8">
        {step === "code" && (
          <span className="mb-5 grid size-11 place-items-center rounded-full bg-brand-wash text-brand">
            <MailIcon className="size-5" />
          </span>
        )}
        <h1 className="text-page">{step === "email" ? title : <Trans>Check your e-mail</Trans>}</h1>
        <p className="mt-3 text-pretty text-muted-foreground">
          {step === "email" ? (
            lead
          ) : (
            <Trans>
              We sent a 6-digit code to <span className="font-medium break-words text-foreground">{email}</span>. It is
              valid for 10 minutes.
            </Trans>
          )}
        </p>
        {step === "code" && (
          <p className="mt-2 text-pretty text-body text-muted-foreground" data-testid="sign-in-spam">
            {resent ? (
              <Trans>We sent a new code. Only the newest code works.</Trans>
            ) : (
              <Trans>Not there after a minute? Look in your spam or junk folder, or send a new code below.</Trans>
            )}
          </p>
        )}
      </div>

      {step === "email" ? (
        <div className="flex flex-col gap-5">
          <form
            className="flex flex-col gap-4"
            onSubmit={(e) => {
              e.preventDefault()
              request.mutate()
            }}
          >
            <div className="flex flex-col gap-2">
              <Label htmlFor="email">
                <Trans>E-mail address</Trans>
              </Label>
              <Input
                id="email"
                type="email"
                autoComplete="username webauthn"
                required
                autoFocus
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder={reader ? t`you@example.com` : t`you@company.com`}
                className={field}
                data-testid="sign-in-email"
              />
            </div>
            <ErrorLine error={request.error} className={alert} />
            <Button type="submit" disabled={request.isPending} size="lg" data-testid="sign-in-send">
              <MailIcon />
              <Trans>Send me a code</Trans>
            </Button>
          </form>
          <div className="flex items-center gap-3 text-caption text-muted-foreground">
            <Separator className="flex-1" />
            <Trans>or</Trans>
            <Separator className="flex-1" />
          </div>
          <Button variant="outline" onClick={() => passkey.mutate()} disabled={passkey.isPending} size="lg">
            <KeyRoundIcon />
            <Trans>Sign in with a passkey</Trans>
          </Button>
          {passkey.error && (
            <p role="alert" className={alert}>
              <Trans>The passkey sign-in did not complete. Try again or use an e-mailed code.</Trans>
            </p>
          )}
          {note}
        </div>
      ) : (
        <form
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault()
            verify.mutate()
          }}
        >
          <div className="flex flex-col gap-2">
            <Label htmlFor="code">
              <Trans>Code</Trans>
            </Label>
            <Input
              id="code"
              inputMode="numeric"
              autoComplete="one-time-code"
              pattern="[0-9]{6}"
              maxLength={6}
              required
              autoFocus
              value={code}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
              className="h-14 rounded-xl text-center font-mono text-page tracking-[0.5em]"
              data-testid="sign-in-code"
            />
          </div>
          {verify.error && (
            <p role="alert" className={alert}>
              <Trans>The code is wrong or has expired.</Trans>
            </p>
          )}
          <ErrorLine error={request.error} className={alert} />
          <Button type="submit" disabled={verify.isPending || code.length !== 6} size="lg">
            <Trans>Sign in</Trans>
          </Button>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="-ms-2.5 rounded-full"
              onClick={() => {
                setStep("email")
                setCode("")
                setResent(false)
                verify.reset()
                request.reset()
              }}
            >
              <ArrowLeftIcon />
              <Trans>Change address</Trans>
            </Button>
            <Button
              type="button"
              variant="link"
              size="sm"
              disabled={request.isPending}
              onClick={() => request.mutate()}
            >
              <Trans>Send a new code</Trans>
            </Button>
          </div>
        </form>
      )}
    </>
  )
}

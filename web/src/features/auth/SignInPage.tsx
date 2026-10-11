import { Trans } from "@lingui/react/macro"
import { Navigate, useNavigate, useSearchParams } from "react-router"

import { AuthLayout } from "@/features/auth/AuthLayout"
import { SignInForm } from "@/features/auth/SignInForm"
import { useSignup } from "@/lib/api"
import { safeNext } from "@/lib/next"
import { useMe } from "@/lib/session"

export function SignInPage() {
  const navigate = useNavigate()
  const me = useMe()
  const signup = useSignup()
  const [params] = useSearchParams()
  const next = safeNext(params.get("next"))
  const create = signup === "open" && params.get("signup") === "1"

  if (me.data) return <Navigate to={next} replace />

  return (
    <AuthLayout>
      <SignInForm
        title={
          create ? (
            <Trans>Create your Yuva account</Trans>
          ) : signup === "open" ? (
            <Trans>Sign in or create an account</Trans>
          ) : (
            <Trans>Sign in to Yuva</Trans>
          )
        }
        lead={
          create ? (
            <Trans>Enter your work e-mail and we send you a code. No password; the same code signs you in later.</Trans>
          ) : signup === "open" ? (
            <Trans>We e-mail you a one-time code. A new address gets a new account; no password to remember.</Trans>
          ) : (
            <Trans>We e-mail you a one-time code. No password to remember.</Trans>
          )
        }
        note={
          signup === "invite" && (
            <p className="text-pretty text-body text-muted-foreground" data-testid="sign-in-invite-only">
              <Trans>Yuva on this server is invite only. Ask your team for an invitation.</Trans>
            </p>
          )
        }
        onSignedIn={() => navigate(next, { replace: true })}
      />
    </AuthLayout>
  )
}

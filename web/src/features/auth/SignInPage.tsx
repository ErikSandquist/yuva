import { Trans } from "@lingui/react/macro"
import { Navigate, useNavigate, useSearchParams } from "react-router"

import { AuthLayout } from "@/features/auth/AuthLayout"
import { SignInForm } from "@/features/auth/SignInForm"
import { safeNext } from "@/lib/next"
import { useMe } from "@/lib/session"

export function SignInPage() {
  const navigate = useNavigate()
  const me = useMe()
  const [params] = useSearchParams()
  const next = safeNext(params.get("next"))

  if (me.data) return <Navigate to={next} replace />

  return (
    <AuthLayout>
      <SignInForm
        title={<Trans>Sign in to Yuva</Trans>}
        lead={<Trans>We e-mail you a one-time code. No password to remember.</Trans>}
        onSignedIn={() => navigate(next, { replace: true })}
      />
    </AuthLayout>
  )
}

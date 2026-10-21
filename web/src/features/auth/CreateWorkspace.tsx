import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { ArrowLeftIcon, ArrowRightIcon, LogOutIcon, UserRoundIcon } from "lucide-react"
import { useState } from "react"
import { Link, Navigate, useNavigate } from "react-router"

import { useErrorText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { AuthHeading, AuthLayout, authAlert as alert, authField as field } from "@/features/auth/AuthLayout"
import { Field } from "@/features/settings/ui"
import { api, ApiError, unwrap, useSignup, type Me } from "@/lib/api"
import { meKey, useSession, useSignOut } from "@/lib/session"

const SECOND_LEVEL = new Set(["com", "co", "org", "net", "edu", "gov", "ac", "gen", "biz", "info", "bel", "k12"])
const MAILBOX_PROVIDERS = new Set([
  "gmail",
  "googlemail",
  "outlook",
  "hotmail",
  "live",
  "msn",
  "yahoo",
  "ymail",
  "icloud",
  "me",
  "mac",
  "aol",
  "proton",
  "protonmail",
  "pm",
  "yandex",
  "gmx",
  "mail",
  "fastmail",
  "zoho",
  "hey",
  "tutanota",
  "tuta",
])

export function workspaceNameFromEmail(email: string): string {
  const labels = (email.split("@")[1] ?? "").toLowerCase().split(".").filter(Boolean)
  labels.pop()
  if (labels.length > 1 && SECOND_LEVEL.has(labels[labels.length - 1]!)) labels.pop()
  const name = labels.pop() ?? ""
  if (!name || MAILBOX_PROVIDERS.has(name)) return ""
  return name.charAt(0).toUpperCase() + name.slice(1)
}

export function CreateWorkspace({
  me,
  back,
  onCreated,
}: {
  me: Me
  back?: string
  onCreated: (workspaceId: string) => void
}) {
  const { t } = useLingui()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const askName = !me.person.name.trim()
  const [person, setPerson] = useState("")
  const [name, setName] = useState(() => workspaceNameFromEmail(me.person.email))
  const create = useMutation({
    mutationFn: async () => {
      if (askName && person.trim()) {
        qc.setQueryData(meKey, await unwrap(api.PATCH("/v1/me", { body: { name: person.trim() } })))
      }
      const ws = await unwrap(api.POST("/v1/workspaces", { body: { name: name.trim() } }))
      await qc.invalidateQueries({ queryKey: meKey })
      return ws
    },
    onSuccess: (ws) => onCreated(ws.id),
  })
  const error = create.error
  const message =
    error instanceof ApiError && error.code === "workspace_limit"
      ? t`You have created as many workspaces as this server allows. Delete one you no longer use, or ask to be invited to another.`
      : error instanceof ApiError && error.code === "rate_limited"
        ? t`Too many workspaces were created from your network today. Try again tomorrow.`
        : error instanceof ApiError && error.code === "signup_closed"
          ? t`This server no longer lets people create workspaces. Ask your team for an invitation.`
          : error && errorText(error)

  return (
    <AuthLayout below={!back && <FirstWorkspaceFooter />}>
      {back && (
        <Link
          to={back}
          className="mb-5 inline-flex w-fit items-center gap-1.5 text-small text-faint transition-colors hover:text-foreground"
          data-testid="create-workspace-back"
        >
          <ArrowLeftIcon className="size-3.5 rtl:rotate-180" />
          <Trans>Back</Trans>
        </Link>
      )}
      <AuthHeading title={<Trans>Create your workspace</Trans>}>
        <Trans>
          A workspace holds your team, inboxes and conversations. You are its owner and can invite others later.
        </Trans>
      </AuthHeading>
      <form
        className="flex flex-col gap-5"
        onSubmit={(e) => {
          e.preventDefault()
          if (name.trim()) create.mutate()
        }}
      >
        {askName && (
          <Field
            label={<Trans>Your name</Trans>}
            htmlFor="create-workspace-person"
            hint={<Trans>Shown to your team and to customers you reply to</Trans>}
          >
            <Input
              id="create-workspace-person"
              autoComplete="name"
              required
              autoFocus
              maxLength={200}
              value={person}
              onChange={(e) => setPerson(e.target.value)}
              className={field}
              data-testid="create-workspace-person"
            />
          </Field>
        )}
        <Field
          label={<Trans>Workspace name</Trans>}
          htmlFor="create-workspace-name"
          hint={<Trans>Usually your company or team; you can change it later</Trans>}
        >
          <Input
            id="create-workspace-name"
            autoComplete="organization"
            required
            autoFocus={!askName}
            maxLength={200}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t`For example Acme`}
            className={field}
            data-testid="create-workspace-name"
          />
        </Field>
        {message && (
          <p role="alert" className={alert} data-testid="create-workspace-error">
            {message}
          </p>
        )}
        <Button
          type="submit"
          size="lg"
          disabled={!name.trim() || (askName && !person.trim()) || create.isPending}
          data-testid="create-workspace-submit"
        >
          <Trans>Create workspace</Trans>
          <ArrowRightIcon className="rtl:rotate-180" />
        </Button>
      </form>
    </AuthLayout>
  )
}

function FirstWorkspaceFooter() {
  const signOut = useSignOut()
  const navigate = useNavigate()
  return (
    <>
      <p>
        <Trans>Here to follow discussions on documentation sites?</Trans>{" "}
        <Link
          to="/account"
          className="inline-flex items-center gap-1 text-muted-foreground underline underline-offset-4 hover:text-foreground"
        >
          <UserRoundIcon className="size-3.5" />
          <Trans>Open your reader account</Trans>
        </Link>
      </p>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="text-faint"
        onClick={async () => {
          await signOut()
          navigate("/sign-in", { replace: true })
        }}
      >
        <LogOutIcon />
        <Trans>Sign out</Trans>
      </Button>
    </>
  )
}

export function NewWorkspacePage() {
  const { me, switchWorkspace } = useSession()
  const signup = useSignup()
  if (signup === "invite") return <Navigate to="/" replace />
  if (!signup) return <div className="min-h-svh bg-background" />
  return <CreateWorkspace me={me} back="/" onCreated={(id) => switchWorkspace(id, "/")} />
}

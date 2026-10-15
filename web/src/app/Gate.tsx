import { Trans } from "@lingui/react/macro"
import { BuildingIcon, ChevronRightIcon, LogOutIcon, RefreshCwIcon, UserRoundIcon } from "lucide-react"
import { useCallback, useEffect } from "react"
import { Link, Navigate, useLocation, useNavigate } from "react-router"

import { AppShell } from "@/app/AppShell"
import { EmptyState } from "@/components/common"
import { Button } from "@/components/ui/button"
import { AuthLayout } from "@/features/auth/AuthLayout"
import { DeleteAccountButton } from "@/features/auth/DeleteAccountButton"
import { CreateWorkspace } from "@/features/auth/CreateWorkspace"
import { activate, i18n } from "@/i18n"
import { useVersion, type Me } from "@/lib/api"
import { signInPath } from "@/lib/next"
import { pickMembership, SessionProvider, useMe, useSignOut, useWorkspaceChoice } from "@/lib/session"
import { Item } from "@/components/ui/item"

const heading = "text-page"
const LINK_PARAMS = ["workspace", "workspace_id"]

function NoWorkspace({ email }: { email: string }) {
  const signOut = useSignOut()
  const navigate = useNavigate()
  return (
    <AuthLayout>
      <span className="mb-5 grid size-11 place-items-center rounded-full border text-faint">
        <BuildingIcon className="size-5" />
      </span>
      <h1 className={heading}>
        <Trans>You are not in a workspace yet</Trans>
      </h1>
      <p className="mt-3 text-muted-foreground">
        <Trans>
          Yuva on this server is invite only. Ask an owner or admin of your team's workspace for an invitation.
        </Trans>
      </p>
      <p className="mt-4 rounded-xl bg-surface px-3.5 py-2.5 text-body break-words">{email}</p>
      <div className="mt-8 flex flex-wrap items-center gap-2">
        <Button
          type="button"
          variant="outline"
          size="lg"
          onClick={async () => {
            await signOut()
            navigate("/sign-in", { replace: true })
          }}
        >
          <LogOutIcon />
          <Trans>Sign out</Trans>
        </Button>
        <DeleteAccountButton email={email} size="lg" />
      </div>
      <p className="mt-8 text-body text-muted-foreground">
        <Trans>Here to follow discussions on documentation sites?</Trans>{" "}
        <Link to="/account" className="inline-flex items-center gap-1 text-brand underline-offset-4 hover:underline">
          <UserRoundIcon className="size-3.5" />
          <Trans>Open your reader account</Trans>
        </Link>
      </p>
    </AuthLayout>
  )
}

function WorkspacePicker({ me, onChoose }: { me: Me; onChoose: (id: string) => void }) {
  return (
    <AuthLayout>
      <h1 className={`mb-6 ${heading}`}>
        <Trans>Choose a workspace</Trans>
      </h1>
      <ul className="flex flex-col gap-2">
        {me.memberships.map((m) => (
          <li key={m.workspace.id}>
            <Item
              render={<button type="button" />}
              variant="outline"
              className="flex-nowrap hover:bg-muted [button]:hover:bg-muted"
              onClick={() => onChoose(m.workspace.id)}
            >
              <span className="flex-1 truncate font-medium">{m.workspace.name}</span>
              <ChevronRightIcon className="size-4 text-faint" />
            </Item>
          </li>
        ))}
      </ul>
    </AuthLayout>
  )
}

export function Gate() {
  const me = useMe()
  const version = useVersion()
  const location = useLocation()
  const navigate = useNavigate()
  const [chosen, choose] = useWorkspaceChoice()
  const params = new URLSearchParams(location.search)
  const linkParam = LINK_PARAMS.find((p) => params.has(p))
  const linked = linkParam ? params.get(linkParam) : null
  const linkedMember = !!linked && !!me.data?.memberships.some((m) => m.workspace.id === linked)
  useEffect(() => {
    if (!linkedMember || !linked) return
    if (linked !== chosen) choose(linked)
    const rest = new URLSearchParams(location.search)
    LINK_PARAMS.forEach((p) => rest.delete(p))
    const search = rest.toString()
    navigate(
      { pathname: location.pathname, search: search ? `?${search}` : "", hash: location.hash },
      { replace: true, state: location.state }
    )
  }, [linkedMember, linked, chosen, choose, navigate, location])
  const personLocale = me.data?.person.locale
  useEffect(() => {
    if (personLocale && personLocale !== i18n.locale) activate(personLocale)
  }, [personLocale])
  const onSwitch = useCallback(
    (id: string, to = "/") => {
      choose(id)
      navigate(to)
    },
    [choose, navigate]
  )

  if (me.isPending) return <div className="min-h-svh bg-background" />
  if (me.error && !me.data) {
    return (
      <div className="flex min-h-svh flex-col bg-background">
        <EmptyState icon={RefreshCwIcon} title={<Trans>Could not reach the server</Trans>}>
          <Button className="mt-3" onClick={() => me.refetch()}>
            <Trans>Try again</Trans>
          </Button>
        </EmptyState>
      </div>
    )
  }
  if (!me.data) return <Navigate to={signInPath(location.pathname, location.search)} replace />
  if (me.data.memberships.length === 0) {
    if (version.isPending) return <div className="min-h-svh bg-background" />
    if (version.data?.signup === "open") return <CreateWorkspace me={me.data} onCreated={(id) => onSwitch(id, "/")} />
    return <NoWorkspace email={me.data.person.email} />
  }
  const membership = pickMembership(me.data, linkedMember ? linked : chosen)
  if (!membership) return <WorkspacePicker me={me.data} onChoose={choose} />
  return (
    <SessionProvider key={membership.workspace.id} me={me.data} membership={membership} onSwitch={onSwitch}>
      <AppShell />
    </SessionProvider>
  )
}

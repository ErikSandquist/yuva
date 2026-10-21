import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { UserPlusIcon } from "lucide-react"
import { Link } from "react-router"

import { byPresence, useMemberStatus } from "@/app/TeamMenu"
import { ErrorLine, MemberAvatar } from "@/components/common"
import { Column, ColumnHeading, ColumnLink, Pane } from "@/components/common/Column"
import { formatDate, useEnumText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { useCounts, useStats } from "@/features/inbox/queries"
import { PageHeader, StatTile } from "@/features/settings/ui"
import { api, unwrap, type Invite } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useMembers } from "@/lib/workspace"

export function useTeamLoad() {
  const counts = useCounts().data
  return { counts, load: new Map((counts?.assignees ?? []).map((a) => [a.id, a.count])) }
}

export function TeamColumn() {
  const { t } = useLingui()
  const { membership } = useSession()
  const members = useMembers()
  const { counts, load } = useTeamLoad()
  const status = useMemberStatus()
  const order = [...(members.data ?? [])].sort(
    (a, b) => byPresence(a, b) || (load.get(b.id) ?? 0) - (load.get(a.id) ?? 0)
  )
  return (
    <Column title={<Trans>Team</Trans>} testId="team-panel">
      <div className="grid grid-cols-2 gap-2 px-1 pt-1">
        <div className="rounded-xl border bg-card px-3 py-2.5">
          <b className="block text-title tabular-nums">{counts?.unassigned ?? "–"}</b>
          <span className="text-caption text-faint">
            <Trans>open, unassigned</Trans>
          </span>
        </div>
        <div className="rounded-xl border bg-card px-3 py-2.5">
          <b className="block text-title tabular-nums">
            {order.filter((m) => m.online && m.availability === "auto").length}
          </b>
          <span className="text-caption text-faint">
            <Trans>available now</Trans>
          </span>
        </div>
      </div>
      <ColumnHeading>
        <Trans>Who is here</Trans>
      </ColumnHeading>
      {members.isPending && <Skeleton className="mx-2 h-10" />}
      <ErrorLine error={members.error} className="px-2.5" />
      <ul className="flex flex-col" data-testid="team-members">
        {order.map((m) => {
          const name = m.name || m.email
          const open = load.get(m.id) ?? 0
          return (
            <li key={m.id} data-testid="team-member">
              <ColumnLink to={`/team/${m.id}`}>
                <MemberAvatar name={name} online={m.online} away={m.availability === "away"} className="size-9" />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-body">
                    {m.id === membership.member_id ? t`${name} (you)` : name}
                  </span>
                  <small className="block truncate text-caption text-faint">{status(m).text}</small>
                </span>
                <span
                  className={cn("shrink-0 text-caption tabular-nums", open ? "text-foreground" : "text-faint")}
                  title={t`Open conversations assigned`}
                  data-testid="team-load"
                >
                  <Plural value={open} one="# open" other="# open" />
                </span>
              </ColumnLink>
            </li>
          )
        })}
      </ul>
    </Column>
  )
}

function InviteCard({ invite }: { invite: Invite }) {
  const { t, i18n } = useLingui()
  const qc = useQueryClient()
  const text = useEnumText()
  const { workspaceId: ws } = useSession()
  const refresh = () => qc.invalidateQueries({ queryKey: keys.invites(ws) })
  const resend = useMutation({
    mutationFn: () =>
      unwrap(api.POST("/v1/invites", { body: { email: invite.email, role: invite.role, locale: invite.locale } })),
    onSuccess: refresh,
  })
  const withdraw = useMutation({
    mutationFn: () => unwrap(api.DELETE("/v1/invites/{inviteId}", { params: { path: { inviteId: invite.id } } })),
    onSuccess: refresh,
  })
  const sent = formatDate(invite.created_at, i18n.locale)
  return (
    <li className="flex flex-col gap-2 rounded-2xl border border-dashed p-4" data-testid="team-invite">
      <span className="text-small font-medium text-faint">
        <Trans>Invited, not joined yet</Trans>
      </span>
      <span className="truncate text-body font-medium">{invite.email}</span>
      <span className="text-small text-faint">
        {text.role[invite.role]} · <Trans>sent {sent}</Trans>
      </span>
      <ErrorLine error={resend.error ?? withdraw.error} />
      <span className="mt-auto flex gap-2 pt-1">
        <Button
          variant="outline"
          size="sm"
          onClick={() => resend.mutate()}
          disabled={resend.isPending || resend.isSuccess}
          aria-label={t`Send the invite for ${invite.email} again`}
        >
          {resend.isSuccess ? <Trans>Sent</Trans> : <Trans>Send again</Trans>}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => withdraw.mutate()}
          disabled={withdraw.isPending}
          aria-label={t`Withdraw the invite for ${invite.email}`}
        >
          <Trans>Withdraw</Trans>
        </Button>
      </span>
    </li>
  )
}

export function TeamHome() {
  const { t, i18n } = useLingui()
  const text = useEnumText()
  const { membership, canManage, workspaceId: ws } = useSession()
  const members = useMembers()
  const { counts, load } = useTeamLoad()
  const status = useMemberStatus()
  const [inboxId] = useInboxFilter()
  const stats = useStats(inboxId).data
  const invites = useQuery({
    queryKey: keys.invites(ws),
    queryFn: () => unwrap(api.GET("/v1/invites")).then((r) => r.items),
    enabled: canManage,
  })
  const fmt = new Intl.NumberFormat(i18n.locale)
  const today = new Map((stats?.members ?? []).map((m) => [m.member_id, m.replies]))
  const order = [...(members.data ?? [])].sort(
    (a, b) => byPresence(a, b) || (load.get(b.id) ?? 0) - (load.get(a.id) ?? 0)
  )
  const here = order.filter((m) => m.online && m.availability === "auto").length
  return (
    <Pane wide testId="team-home" className="@container gap-6">
      <PageHeader
        back={false}
        title={<Trans>Team</Trans>}
        description={
          members.data ? (
            <Plural
              value={here}
              _0="Nobody is online right now."
              one="# person is online right now."
              other="# people are online right now."
            />
          ) : undefined
        }
        action={
          canManage && (
            <Button size="sm" render={<Link to="/settings/members" />} data-testid="team-invite-open">
              <UserPlusIcon />
              <Trans>Invite</Trans>
            </Button>
          )
        }
      />
      <div className="grid grid-cols-2 gap-3 @4xl:grid-cols-3" data-testid="team-tiles">
        <StatTile live label={<Trans>available now</Trans>} value={members.data ? fmt.format(here) : "–"} />
        <StatTile
          label={<Trans>open, unassigned</Trans>}
          value={counts ? fmt.format(Number(counts.unassigned)) : "–"}
        />
      </div>
      {members.isPending && <Skeleton className="h-40 rounded-2xl" />}
      <ErrorLine error={members.error ?? invites.error} />
      <ul className="grid gap-3 @lg:grid-cols-2 @4xl:grid-cols-3" data-testid="team-cards">
        {order.map((m) => {
          const name = m.name || m.email
          const open = load.get(m.id) ?? 0
          const replies = today.get(m.id) ?? 0
          return (
            <li key={m.id} data-testid="team-card">
              <Link
                to={`/team/${m.id}`}
                className="flex h-full flex-col gap-3 rounded-2xl border bg-card p-4 outline-none transition-colors hover:border-faint focus-visible:ring-2 focus-visible:ring-ring"
              >
                <span className="flex min-w-0 items-center gap-3">
                  <MemberAvatar
                    name={name}
                    online={m.online}
                    away={m.availability === "away"}
                    className="size-10"
                    ring="ring-card"
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-body font-medium">
                      {m.id === membership.member_id ? t`${name} (you)` : name}
                    </span>
                    <span className="block truncate text-small text-faint">{m.email}</span>
                  </span>
                  <span className="shrink-0 rounded-md border bg-surface px-1.5 text-caption text-muted-foreground">
                    {text.role[m.role]}
                  </span>
                </span>
                <span className="truncate text-small text-muted-foreground">{status(m).text}</span>
                <span className="mt-auto flex justify-between gap-2 border-t pt-2.5 text-caption text-faint tabular-nums">
                  <span className={cn(open > 0 && "text-foreground")} data-testid="team-card-load">
                    <Plural value={open} one="# open now" other="# open now" />
                  </span>
                  {stats && <span>{t`${fmt.format(replies)} replies today`}</span>}
                </span>
              </Link>
            </li>
          )
        })}
        {(invites.data ?? []).map((inv) => (
          <InviteCard key={inv.id} invite={inv} />
        ))}
      </ul>
    </Pane>
  )
}

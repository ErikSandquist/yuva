import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { PlugIcon } from "lucide-react"
import { Link } from "react-router"

import { ErrorLine, useConfirm } from "@/components/common"
import { API_KEY_SCOPES, formatDate, formatDateTime, useEnumText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import {
  ASSISTANTS,
  grantsFor,
  useAssistantName,
  type AssistantId,
} from "@/features/settings/connected-apps/assistants"
import { useGrants } from "@/features/settings/connected-apps/queries"
import { DeveloperTabs, DeveloperTitle } from "@/features/settings/DeveloperTabs"
import { Card, EmptyRow, PageHeader, Pill, Row, Rows, Section } from "@/features/settings/ui"
import { api, unwrap, type OAuthGrant } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"
import { useMemberMap } from "@/lib/workspace"

function GrantRow({ g, showMember, onRevoke }: { g: OAuthGrant; showMember: boolean; onRevoke: () => void }) {
  const { t, i18n } = useLingui()
  const text = useEnumText()
  const members = useMemberMap()
  const { membership } = useSession()
  const created = formatDateTime(g.created_at, i18n.locale)
  const used = g.last_used_at ? formatDateTime(g.last_used_at, i18n.locale) : null
  const member = members.get(g.member_id)
  const memberName =
    g.member_id === membership.member_id ? t`You` : member ? member.name || member.email : t`Deleted member`
  const allScopes = API_KEY_SCOPES.every((s) => g.scopes.includes(s))
  return (
    <Row className="items-start" data-testid="grant-row">
      <span className="mt-0.5 grid size-8 shrink-0 place-items-center rounded-full border text-faint">
        <PlugIcon className="size-4" />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="flex min-w-0 items-start justify-between gap-2">
          <p className="min-w-0 text-body font-medium break-words" data-testid="grant-client">
            {g.client.name}
          </p>
          <Button variant="ghost" size="sm" className="-my-1" onClick={onRevoke} data-testid="grant-revoke">
            <Trans>Revoke</Trans>
          </Button>
        </div>
        <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-caption text-muted-foreground">
          {showMember && (
            <>
              <dt className="text-faint">
                <Trans>Member</Trans>
              </dt>
              <dd className="truncate" data-testid="grant-member">
                {memberName}
              </dd>
            </>
          )}
          <dt className="text-faint">
            <Trans>Access</Trans>
          </dt>
          <dd className="truncate" data-testid="grant-resource">
            {g.resource === "mcp" ? <Trans>MCP only</Trans> : <Trans>Full API</Trans>}
          </dd>
          <dt className="text-faint">
            <Trans>Scopes</Trans>
          </dt>
          <dd title={g.scopes.map((s) => text.scope[s]).join("\n")} data-testid="grant-scopes">
            {allScopes ? (
              <Trans>All scopes</Trans>
            ) : (
              <span className="flex flex-wrap gap-x-2 font-mono">
                {g.scopes.map((s) => (
                  <span key={s} className="whitespace-nowrap">
                    {s}
                  </span>
                ))}
              </span>
            )}
          </dd>
          <dt className="text-faint">
            <Trans>This month</Trans>
          </dt>
          <dd className="truncate" data-testid="grant-requests">
            <Plural value={g.requests_this_month} one="# request" other="# requests" />
          </dd>
        </dl>
        <p className="text-caption text-faint">
          <Trans>Connected {created}</Trans>
          {" · "}
          {used ? <Trans>last used {used}</Trans> : <Trans>never used</Trans>}
        </p>
      </div>
    </Row>
  )
}

function Assistants({ grants }: { grants: OAuthGrant[] | undefined }) {
  const { i18n } = useLingui()
  const { membership } = useSession()
  const nameOf = useAssistantName()
  const details: Record<AssistantId, React.ReactNode> = {
    claude: <Trans>claude.ai and Claude Desktop</Trans>,
    "claude-code": <Trans>From the terminal</Trans>,
    codex: <Trans>CLI and IDE extension</Trans>,
    chatgpt: <Trans>On the web</Trans>,
    cursor: <Trans>One-click install</Trans>,
    vscode: <Trans>One-click install</Trans>,
    other: <Trans>Any client that speaks MCP over HTTP</Trans>,
  }
  return (
    <>
      <p className="text-muted-foreground">
        <Trans>
          Assistants work in Yuva through MCP on this server’s <code className="font-mono">/mcp</code>, with the access
          you approve.
        </Trans>
      </p>
      <ul className="grid grid-cols-3 gap-4 max-xl:grid-cols-2 phone:grid-cols-1">
        {ASSISTANTS.map((a) => {
          const Icon = a.icon
          const mine = grants ? grantsFor(a, grants, membership.member_id, new Set()) : []
          const first = mine[0]
          const since = first ? formatDate(first.created_at, i18n.locale) : ""
          return (
            <li key={a.id} className="flex flex-col gap-3 rounded-xl border bg-card p-4" data-testid="assistant-row">
              <span className="flex min-w-0 items-center gap-3">
                <span className="grid size-8 shrink-0 place-items-center rounded-lg border bg-surface text-muted-foreground">
                  <Icon className="size-4" />
                </span>
                <span className="min-w-0 flex-1 truncate font-semibold">{nameOf(a)}</span>
                {first && (
                  <Pill tone="success">
                    <Trans>Connected</Trans>
                  </Pill>
                )}
              </span>
              <span className="truncate text-caption text-faint">
                {first ? <Trans>Connected on {since}</Trans> : details[a.id]}
              </span>
              <Button
                variant={first ? "outline" : "default"}
                className={first ? "h-[30px] rounded-lg border-input font-normal" : "h-[30px] rounded-lg"}
                render={<Link to={a.id} />}
              >
                {first ? <Trans>Manage</Trans> : <Trans>Connect</Trans>}
              </Button>
            </li>
          )
        })}
      </ul>
    </>
  )
}

export function ConnectedAppsPage() {
  const qc = useQueryClient()
  const { workspaceId: ws, canManage } = useSession()
  const [confirm, confirmDialog] = useConfirm()
  const list = useGrants()
  const revoke = useMutation({
    mutationFn: (id: string) =>
      unwrap(api.DELETE("/v1/oauth/grants/{oauthGrantId}", { params: { path: { oauthGrantId: id } } })),
    onSettled: () => qc.invalidateQueries({ queryKey: keys.oauthGrants(ws) }),
  })
  return (
    <>
      <PageHeader
        title={<DeveloperTitle />}
        tabs={<DeveloperTabs current="connected-apps" />}
        description={
          canManage ? (
            <Trans>Keys, webhooks and connected assistants</Trans>
          ) : (
            <Trans>
              Assistants and apps you connected with your sign-in. Each acts as you, only within the access you allowed.
            </Trans>
          )
        }
      />
      <Assistants grants={list.data} />
      <Section title={<Trans>Connections</Trans>}>
        <Card flush>
          {list.data && list.data.length === 0 ? (
            <EmptyRow>
              <Trans>No connected apps.</Trans>
            </EmptyRow>
          ) : (
            <Rows>
              {(list.data ?? []).map((g) => (
                <GrantRow
                  key={g.id}
                  g={g}
                  showMember={canManage}
                  onRevoke={() => {
                    const clientName = g.client.name
                    confirm({
                      title: <Trans>Revoke {clientName}?</Trans>,
                      description: <Trans>Its access ends at once. To use it again, connect it again.</Trans>,
                      confirm: <Trans>Revoke</Trans>,
                      run: () => revoke.mutate(g.id),
                    })
                  }}
                />
              ))}
            </Rows>
          )}
          <ErrorLine error={list.error ?? revoke.error} className="px-5 pb-4" />
        </Card>
      </Section>
      {confirmDialog}
    </>
  )
}

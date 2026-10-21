import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { EllipsisIcon, PencilIcon } from "lucide-react"
import { useState } from "react"
import { Navigate, useLocation, useNavigate } from "react-router"

import { ErrorLine, useConfirm } from "@/components/common"
import { API_KEY_SCOPES, formatDate, formatRelative, formatShort, useEnumText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Switch } from "@/components/ui/switch"
import { ApiKeyDialog } from "@/features/settings/api-keys/ApiKeyDialog"
import { DeveloperTabs, DeveloperTitle } from "@/features/settings/DeveloperTabs"
import { Card, Chip, headButton, PageHeader, tableHeadClass, ToggleRow } from "@/features/settings/ui"
import { api, unwrap, type ApiKey } from "@/lib/api"
import { keys } from "@/lib/keys"
import { meKey, useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes, useMemberMap } from "@/lib/workspace"

function BotSending() {
  const { t } = useLingui()
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  const workspace = useQuery({ queryKey: keys.workspace(ws), queryFn: () => unwrap(api.GET("/v1/workspace")) })
  const save = useMutation({
    mutationFn: (on: boolean) => unwrap(api.PATCH("/v1/workspace", { body: { bots_may_send: on } })),
    onSuccess: (w) => {
      qc.setQueryData(keys.workspace(ws), w)
      void qc.invalidateQueries({ queryKey: meKey })
    },
  })
  const on = save.isPending ? save.variables : (workspace.data?.bots_may_send ?? false)
  return (
    <Card>
      <ToggleRow
        title={<Trans>Bots may send</Trans>}
        hint={<Trans>When off, API keys can only write drafts, and a member reviews and sends them.</Trans>}
      >
        <Switch
          checked={on}
          disabled={save.isPending || !workspace.data}
          onCheckedChange={(v) => save.mutate(v)}
          aria-label={t`Bots may send`}
          data-testid="bots-may-send"
        />
      </ToggleRow>
      <ErrorLine error={workspace.error ?? save.error} />
    </Card>
  )
}

const KEY_GRID =
  "grid grid-cols-[minmax(0,1.3fr)_minmax(0,1.8fr)_minmax(0,1.3fr)_110px_90px_34px] items-center gap-4 px-4"

function KeyRow({ k, onEdit, onRevoke }: { k: ApiKey; onEdit: () => void; onRevoke: () => void }) {
  const [now] = useState(Date.now)
  const { t, i18n } = useLingui()
  const text = useEnumText()
  const members = useMemberMap()
  const inboxes = useInboxes().data ?? []
  const created = formatShort(k.created_at, i18n.locale)
  const creator = k.created_by ? members.get(k.created_by) : undefined
  const by = creator ? creator.name || creator.email : null
  const bot = k.bot_name || k.name
  const expires = k.expires_at ? formatDate(k.expires_at, i18n.locale) : null
  const expired = !!k.expires_at && new Date(k.expires_at).getTime() <= now
  const allScopes = API_KEY_SCOPES.every((s) => k.scopes.includes(s))
  const inboxNames = k.inbox_ids?.map((id) => inboxes.find((i) => i.id === id)?.name ?? t`Deleted inbox`).join(", ")
  const inactive = !!k.revoked_at || expired
  return (
    <li className={cn(KEY_GRID, "border-t py-3")} data-testid="api-key-row">
      <span className={cn("flex min-w-0 flex-col leading-[1.3]", inactive && "opacity-60")} title={bot}>
        <span className="truncate font-medium">{k.name}</span>
        <code className="truncate font-mono text-caption text-faint">{k.prefix}…</code>
        <span className="sr-only" data-testid="api-key-bot">
          {bot}
        </span>
      </span>
      <span
        className={cn("flex min-w-0 flex-wrap gap-1", inactive && "opacity-60")}
        title={k.scopes.map((s) => text.scope[s]).join("\n")}
        data-testid="api-key-scopes"
      >
        {allScopes ? (
          <Chip>
            <Trans>All scopes</Trans>
          </Chip>
        ) : (
          k.scopes.map((s) => (
            <Chip key={s} className="font-mono">
              {s}
            </Chip>
          ))
        )}
      </span>
      <span className={cn("truncate", inactive && "opacity-60")} title={inboxNames} data-testid="api-key-inboxes">
        {inboxNames ? inboxNames : <Trans>All products</Trans>}
      </span>
      <span className="truncate">
        {k.revoked_at ? (
          <span className="text-faint">
            <Trans>Revoked</Trans>
          </span>
        ) : k.last_used_at ? (
          formatRelative(k.last_used_at, i18n.locale, now)
        ) : (
          <span className="text-faint">
            <Trans>never</Trans>
          </span>
        )}
      </span>
      <span className="flex min-w-0 flex-col text-caption leading-[1.3] text-faint" title={by ?? undefined}>
        <span className="truncate">{created}</span>
        {expires && (
          <span className={cn("truncate", expired && "text-destructive")} data-testid="api-key-expiry">
            {expired ? <Trans>expired {expires}</Trans> : <Trans>until {expires}</Trans>}
          </span>
        )}
      </span>
      <span className="flex justify-end">
        {!k.revoked_at && (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={<Button variant="ghost" size="icon-sm" />}
              aria-label={t`Actions for ${k.name}`}
              data-testid="api-key-actions"
            >
              <EllipsisIcon />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-40">
              <DropdownMenuItem onClick={onEdit} data-testid="api-key-edit">
                <PencilIcon />
                <Trans>Edit</Trans>
              </DropdownMenuItem>
              <DropdownMenuItem variant="destructive" onClick={onRevoke} data-testid="api-key-revoke">
                <Trans>Revoke</Trans>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </span>
    </li>
  )
}

export function ApiKeysPage() {
  const qc = useQueryClient()
  const { workspaceId: ws, canManage } = useSession()
  const navigate = useNavigate()
  const location = useLocation()
  const [editing, setEditing] = useState<ApiKey | null>(null)
  const [confirm, confirmDialog] = useConfirm()
  const list = useQuery({
    queryKey: keys.apiKeys(ws),
    queryFn: () => unwrap(api.GET("/v1/api-keys")).then((r) => r.items),
    enabled: canManage,
  })
  const revoke = useMutation({
    mutationFn: (id: string) => unwrap(api.DELETE("/v1/api-keys/{apiKeyId}", { params: { path: { apiKeyId: id } } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.apiKeys(ws) }),
  })
  if (!canManage) return <Navigate to="/settings/profile" replace />
  return (
    <>
      <PageHeader
        title={<DeveloperTitle />}
        tabs={<DeveloperTabs current="api-keys" />}
        description={<Trans>Keys, webhooks and connected assistants</Trans>}
        action={
          <Button
            className={headButton.primary}
            onClick={() => navigate("/setup/api-key", { state: { back: location.pathname } })}
            data-testid="api-key-new"
          >
            <Trans>New API key</Trans>
          </Button>
        }
      />
      <section className="overflow-x-auto rounded-xl border bg-card">
        <div className="min-w-[860px]">
          <div className={cn(KEY_GRID, tableHeadClass, "py-2.5")}>
            <span>
              <Trans>Key</Trans>
            </span>
            <span>
              <Trans>Access</Trans>
            </span>
            <span>
              <Trans>Products</Trans>
            </span>
            <span>
              <Trans>Last used</Trans>
            </span>
            <span>
              <Trans>Created</Trans>
            </span>
            <span />
          </div>
          {list.data && list.data.length === 0 ? (
            <p className="border-t px-4 py-8 text-center text-faint">
              <Trans>No API keys yet.</Trans>
            </p>
          ) : (
            <ul>
              {(list.data ?? []).map((k) => (
                <KeyRow
                  key={k.id}
                  k={k}
                  onEdit={() => setEditing(k)}
                  onRevoke={() => {
                    const keyName = k.name
                    confirm({
                      title: <Trans>Revoke {keyName}?</Trans>,
                      description: <Trans>Anything using this key stops working at once.</Trans>,
                      confirm: <Trans>Revoke</Trans>,
                      run: () => revoke.mutate(k.id),
                    })
                  }}
                />
              ))}
            </ul>
          )}
          <ErrorLine error={list.error ?? revoke.error} className="px-4 pb-4" />
        </div>
      </section>
      <BotSending />
      {editing && <ApiKeyDialog apiKey={editing} onClose={() => setEditing(null)} />}
      {confirmDialog}
    </>
  )
}

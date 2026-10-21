import { Trans, useLingui } from "@lingui/react/macro"
import { useQuery } from "@tanstack/react-query"

import { PageTabs } from "@/features/settings/ui"
import { api, unwrap } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

type Tab = "api-keys" | "webhooks" | "connected-apps"

export function DeveloperTitle() {
  const { canManage } = useSession()
  return canManage ? <Trans>API & MCP</Trans> : <Trans>Connected apps</Trans>
}

export function DeveloperTabs({ current }: { current: Tab }) {
  const { t } = useLingui()
  const { workspaceId: ws, canManage } = useSession()
  const apiKeys = useQuery({
    queryKey: keys.apiKeys(ws),
    queryFn: () => unwrap(api.GET("/v1/api-keys")).then((r) => r.items),
    enabled: canManage,
  })
  const hooks = useQuery({
    queryKey: keys.webhookList(ws, ""),
    queryFn: () => unwrap(api.GET("/v1/webhooks", { params: { query: {} } })).then((r) => r.items),
    enabled: canManage,
  })
  if (!canManage) return null
  return (
    <PageTabs
      label={t`API & MCP`}
      current={current}
      items={[
        {
          key: "api-keys",
          to: "/settings/api-keys",
          label: t`API keys`,
          count: apiKeys.data?.length,
          testId: "developer-tab-api-keys",
        },
        {
          key: "webhooks",
          to: "/settings/webhooks",
          label: t`Webhooks`,
          count: hooks.data?.length,
          testId: "developer-tab-webhooks",
        },
        {
          key: "connected-apps",
          to: "/settings/connected-apps",
          label: t`Connected apps`,
          testId: "developer-tab-connected-apps",
        },
      ]}
    />
  )
}

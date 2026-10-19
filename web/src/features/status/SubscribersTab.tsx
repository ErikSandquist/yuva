import { Trans } from "@lingui/react/macro"
import { UsersIcon } from "lucide-react"
import { Link } from "react-router"

import { EmptyState, ErrorLine } from "@/components/common"
import { ExportButtons } from "@/components/common/ExportButtons"
import { Skeleton } from "@/components/ui/skeleton"
import { Tiles } from "@/features/broadcasts/ListPage"
import { useList } from "@/features/broadcasts/queries"
import { Subscribers } from "@/features/broadcasts/Subscribers"
import { Card, Section } from "@/features/settings/ui"
import { statusLink } from "@/features/status/queries"
import { api, type StatusPage } from "@/lib/api"
import { saveDownload } from "@/lib/download"
import { useSession } from "@/lib/session"

export function SubscribersTab({ page }: { page: StatusPage }) {
  const { canManage } = useSession()
  const listId = page.email_subscribers.list_id ?? ""
  const list = useList(listId)
  if (!listId) {
    return (
      <EmptyState icon={UsersIcon} title={<Trans>No e-mail subscribers yet</Trans>}>
        <Trans>Turn on e-mail subscribers in the page settings to let visitors subscribe to notices.</Trans>
        {canManage && (
          <Link
            to={statusLink(page.id, "settings")}
            className="mt-2 block text-brand underline-offset-4 hover:underline"
          >
            <Trans>Open the page settings</Trans>
          </Link>
        )}
      </EmptyState>
    )
  }
  if (list.isPending) return <Skeleton className="h-40 w-full rounded-2xl" />
  const l = list.data
  if (!l) return <ErrorLine error={list.error} />
  return (
    <div className="flex flex-col gap-8" data-testid="status-subscribers">
      {!page.email_subscribers.enabled && (
        <p className="text-body text-muted-foreground">
          <Trans>E-mail subscribers are off. Nobody can subscribe and no notices go out, but the list is kept.</Trans>
        </p>
      )}
      <Tiles list={l} />
      <Subscribers list={l} />
      {canManage && (
        <Section
          title={<Trans>Consent export</Trans>}
          description={
            <Trans>
              One row per event, oldest first: who agreed or left, when, how, from which page and IP address, and the
              words they saw. Keep it as proof of consent or to answer a person who asks what you hold.
            </Trans>
          }
        >
          <Card>
            <ExportButtons
              testId="status-subscribers-export"
              run={(format) =>
                saveDownload(
                  api.GET("/v1/lists/{listId}/consent-export", {
                    params: { path: { listId: l.id }, query: { format } },
                    parseAs: "blob",
                  }),
                  `consent-${l.name}.${format}`
                )
              }
            />
          </Card>
        </Section>
      )}
    </div>
  )
}

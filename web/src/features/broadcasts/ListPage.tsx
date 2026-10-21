import { Trans, useLingui } from "@lingui/react/macro"
import { ArchiveIcon, MailWarningIcon } from "lucide-react"
import { Link, useParams, useSearchParams } from "react-router"

import { ErrorLine, InboxMark, Notice } from "@/components/common"
import { Pane, PaneBack } from "@/components/common/Column"
import { Skeleton } from "@/components/ui/skeleton"
import { FormSnippets } from "@/features/broadcasts/FormSnippets"
import { Growth } from "@/features/broadcasts/Growth"
import { ListImports } from "@/features/broadcasts/Import"
import { ListExport, ListSettings } from "@/features/broadcasts/ListSettings"
import { listLink, useList } from "@/features/broadcasts/queries"
import { Subscribers } from "@/features/broadcasts/Subscribers"
import { PageHeader, PageTabs } from "@/features/settings/ui"
import type { MailingList } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

const TABS = ["subscribers", "import", "growth", "form", "settings", "export"] as const
type Tab = (typeof TABS)[number]

export { ListsOverview as ListsHome } from "@/features/broadcasts/Overview"

export function Tiles({ list }: { list: MailingList }) {
  const { t, i18n } = useLingui()
  const fmt = new Intl.NumberFormat(i18n.locale)
  const tiles: [string, string, string?][] = [
    [fmt.format(list.counts.subscribed), t`subscribed now`, "text-success"],
    [fmt.format(list.counts.pending), t`waiting for confirmation`, list.counts.pending ? "text-warning" : undefined],
    [fmt.format(list.counts.unsubscribed), t`unsubscribed now`],
  ]
  return (
    <div className="grid grid-cols-3 gap-2" data-testid="list-tiles">
      {tiles.map(([value, label, tone]) => (
        <div key={label} className="min-w-0 rounded-2xl border bg-card px-4 py-3.5 phone:px-3">
          <b className={cn("block text-page tabular-nums", tone)}>{value}</b>
          <span className="block text-small break-words text-faint">{label}</span>
        </div>
      ))}
    </div>
  )
}

export function ListPage() {
  const { t } = useLingui()
  const { listId = "" } = useParams()
  const [params] = useSearchParams()
  const list = useList(listId)
  const { canManage } = useSession()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === list.data?.inbox_id)
  const raw = params.get("tab")
  const tab: Tab = TABS.includes(raw as Tab) ? (raw as Tab) : "subscribers"
  const names: Record<Tab, string> = {
    subscribers: t`Subscribers`,
    import: t`Import`,
    growth: t`Growth`,
    form: t`Form`,
    settings: t`Settings`,
    export: t`Export`,
  }
  const l = list.data
  if (list.isPending) {
    return (
      <Pane wide className="gap-4">
        <Skeleton className="h-8 w-60" />
        <Skeleton className="h-24 w-full rounded-2xl" />
      </Pane>
    )
  }
  if (!l) {
    return (
      <Pane wide>
        <PaneBack to="/broadcasts/lists" label={t`Lists`} always />
        <ErrorLine error={list.error} />
      </Pane>
    )
  }
  return (
    <Pane wide testId="list-page" key={l.id}>
      <PaneBack to="/broadcasts/lists" label={t`Lists`} always />
      <PageHeader
        back={false}
        title={<span data-testid="list-name-heading">{l.name}</span>}
        description={
          <span className="flex flex-col gap-1.5">
            <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
              {inbox && (
                <span className="inline-flex items-center gap-1.5">
                  <InboxMark name={inbox.name} color={inbox.branding.color} className="size-4 text-[9px]" />
                  {inbox.name}
                </span>
              )}
              <span aria-hidden>·</span>
              <span>{l.public ? <Trans>Public</Trans> : <Trans>Private</Trans>}</span>
              <span aria-hidden>·</span>
              <span>{l.double_opt_in ? <Trans>Confirmation asked</Trans> : <Trans>No confirmation</Trans>}</span>
              {l.archived_at && (
                <span className="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-0.5 text-caption">
                  <ArchiveIcon className="size-3" />
                  <Trans>Archived</Trans>
                </span>
              )}
            </span>
            {l.description && <span className="text-reading">{l.description}</span>}
          </span>
        }
      />
      {!l.sender_ready && !l.archived_at && (
        <Notice tone="warning" icon={MailWarningIcon} data-testid="list-needs-sender">
          <p>
            <Trans>
              This list has no e-mail channel that can send, so forms send no confirmation, new form subscribers keep
              waiting and nobody gets a welcome mail.
            </Trans>
          </p>
          {tab !== "settings" && (
            <Link to={listLink(l.id, "settings")} className="w-fit text-brand underline-offset-4 hover:underline">
              <Trans>Pick a sender in the settings</Trans>
            </Link>
          )}
        </Notice>
      )}
      <Tiles list={l} />
      <PageTabs
        label={t`List`}
        current={tab}
        items={TABS.filter((v) => v !== "export" || canManage).map((v) => ({
          key: v,
          to: listLink(l.id, v === "subscribers" ? undefined : v),
          label: names[v],
          testId: `list-tab-${v}`,
        }))}
      />
      {tab === "subscribers" && <Subscribers list={l} />}
      {tab === "import" && <ListImports list={l} />}
      {tab === "growth" && <Growth list={l} />}
      {tab === "form" && <FormSnippets list={l} />}
      {tab === "settings" && <ListSettings list={l} />}
      {tab === "export" && canManage && <ListExport list={l} />}
    </Pane>
  )
}

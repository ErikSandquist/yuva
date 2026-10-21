import { Trans, useLingui } from "@lingui/react/macro"
import { ActivityIcon, EyeIcon, ExternalLinkIcon, PlusIcon } from "lucide-react"
import { useState } from "react"
import { Link, useLocation, useNavigate, useParams } from "react-router"

import { ErrorLine, InboxMark } from "@/components/common"
import { Pane, PaneBack } from "@/components/common/Column"
import { keyLabel, STATUS_KEYS } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { ComponentsTab } from "@/features/status/ComponentsTab"
import { CreatePageDialog } from "@/features/status/CreatePageDialog"
import { MonitorsTab } from "@/features/status/MonitorsTab"
import { IncidentsTab } from "@/features/status/IncidentsTab"
import { PreviewDialog } from "@/features/status/PreviewDialog"
import { statusLink, usePatchStatusPage, useStatusPage, useStatusPages } from "@/features/status/queries"
import { PageHeader, PageTabs } from "@/features/settings/ui"
import { SettingsTab } from "@/features/status/SettingsTab"
import { SubscribersTab } from "@/features/status/SubscribersTab"
import { StateDot, useStatusText } from "@/features/status/text"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useRowFocus } from "@/hooks/use-row-focus"
import type { Inbox, StatusPage } from "@/lib/api"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

const TABS = ["incidents", "components", "monitors", "subscribers", "settings"] as const
type Tab = (typeof TABS)[number]

function PageCard({ p, inbox }: { p: StatusPage; inbox?: Inbox }) {
  const { i18n } = useLingui()
  const text = useStatusText()
  const fmt = new Intl.NumberFormat(i18n.locale)
  const subs = p.email_subscribers
  const subscribed = fmt.format(subs.counts.subscribed)
  const host = p.url.replace(/^https?:\/\//, "").replace(/\/$/, "")
  return (
    <li>
      <Link
        to={statusLink(p.id)}
        className="flex h-full flex-col gap-3 rounded-2xl border bg-card p-4 outline-none transition-colors hover:border-faint focus-visible:ring-2 focus-visible:ring-ring"
        data-testid="status-home-page"
      >
        <span className="flex items-center gap-2.5">
          {inbox && <InboxMark name={inbox.name} color={inbox.branding.color} className="size-6 text-caption" />}
          <span className="min-w-0 flex-1 truncate text-body font-semibold">{p.name}</span>
          <StateDot state={p.state} className="size-2.5" />
        </span>
        <span className="truncate font-mono text-caption text-faint">{host}</span>
        <span className="mt-auto flex flex-wrap items-center justify-between gap-x-3 gap-y-1 border-t pt-3 text-small">
          <span className="font-medium">{text.component[p.state]}</span>
          <span className="text-faint">
            {!p.published ? (
              <Trans>Unpublished</Trans>
            ) : subs.enabled ? (
              <Trans>{subscribed} subscribers</Trans>
            ) : (
              <Trans>Published</Trans>
            )}
          </span>
        </span>
      </Link>
    </li>
  )
}

export function StatusHome() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { canManage } = useSession()
  const [inboxId] = useInboxFilter()
  const inboxes = useInboxes().data ?? []
  const pages = useStatusPages(inboxId)
  const items = pages.data ?? []
  const shown = inboxId ? inboxes.filter((i) => i.id === inboxId) : inboxes
  const missing = canManage ? shown.find((i) => !items.some((p) => p.inbox_id === i.id)) : undefined
  const [creating, setCreating] = useState<Inbox | null>(null)
  const byInbox = new Map(inboxes.map((i) => [i.id, i]))
  const grid = useRowFocus<HTMLUListElement>(!creating)
  const newKey = keyLabel(STATUS_KEYS.incident).join("")
  useHotkeys(missing ? { [STATUS_KEYS.incident]: () => setCreating(missing) } : {}, !creating)
  return (
    <Pane wide testId="status-home" className="gap-6">
      <PageHeader
        back={false}
        title={<Trans>Status pages</Trans>}
        description={<Trans>Public pages that tell your customers whether your products work.</Trans>}
        action={
          missing && (
            <Button
              size="sm"
              onClick={() => setCreating(missing)}
              title={t`Create status page (${newKey})`}
              data-testid="status-home-create"
            >
              <PlusIcon />
              <Trans>New page</Trans>
            </Button>
          )
        }
      />
      <ErrorLine error={pages.error} />
      {pages.isPending ? (
        <div className="grid gap-3 sm:grid-cols-2">
          <Skeleton className="h-32 rounded-2xl" />
          <Skeleton className="h-32 rounded-2xl" />
        </div>
      ) : items.length === 0 ? (
        <div
          className="flex flex-col items-center gap-2 rounded-2xl border border-dashed px-6 py-12 text-center"
          data-testid="status-home-empty"
        >
          <ActivityIcon className="size-6 text-faint" />
          <p className="max-w-md text-body text-muted-foreground">
            <Trans>
              Tell your customers whether your product works. Each inbox gets one page, where you open incidents, plan
              maintenance and set the state of each component.
            </Trans>
          </p>
        </div>
      ) : (
        <ul ref={grid} className="grid gap-3 sm:grid-cols-2" aria-label={t`Status pages`}>
          {items.map((p) => (
            <PageCard key={p.id} p={p} inbox={byInbox.get(p.inbox_id)} />
          ))}
        </ul>
      )}
      {creating && (
        <CreatePageDialog
          inboxId={creating.id}
          inboxName={creating.name}
          onClose={() => setCreating(null)}
          onCreated={(p) => {
            setCreating(null)
            navigate(statusLink(p.id, "settings"))
          }}
        />
      )}
    </Pane>
  )
}

export function StatusPagePane() {
  const { t } = useLingui()
  const { pageId = "" } = useParams()
  const { pathname, search } = useLocation()
  const navigate = useNavigate()
  const page = useStatusPage(pageId)
  const patch = usePatchStatusPage(pageId)
  const { canManage } = useSession()
  const text = useStatusText()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === page.data?.inbox_id)
  const [preview, setPreview] = useState(false)
  const raw = new URLSearchParams(search).get("tab") ?? (pathname.endsWith("/monitors") ? "monitors" : null)
  const tab: Tab = TABS.includes(raw as Tab) && (raw !== "settings" || canManage) ? (raw as Tab) : "incidents"
  const setTab = (v: Tab) => navigate(statusLink(pageId, v === "incidents" ? undefined : v), { replace: true })
  const p = page.data

  useHotkeys(
    {
      [STATUS_KEYS.incidents]: () => setTab("incidents"),
      [STATUS_KEYS.components]: () => setTab("components"),
      [STATUS_KEYS.monitors]: () => setTab("monitors"),
      [STATUS_KEYS.subscribers]: () => setTab("subscribers"),
      ...(canManage ? { [STATUS_KEYS.settings]: () => setTab("settings") } : {}),
      [STATUS_KEYS.preview]: () => setPreview(true),
      ...(canManage && p ? { [STATUS_KEYS.publishPage]: () => patch.mutate({ published: !p.published }) } : {}),
    },
    !!p
  )

  if (page.isPending) {
    return (
      <Pane wide className="gap-4">
        <Skeleton className="h-8 w-60" />
        <Skeleton className="h-24 w-full rounded-2xl" />
      </Pane>
    )
  }
  if (!p) {
    return (
      <Pane wide>
        <PaneBack to="/status" label={t`Status`} always />
        <ErrorLine error={page.error} />
      </Pane>
    )
  }
  const names: Record<Tab, string> = {
    incidents: t`Incidents`,
    components: t`Components`,
    monitors: t`Monitors`,
    subscribers: t`Subscribers`,
    settings: t`Settings`,
  }
  const previewKey = keyLabel(STATUS_KEYS.preview).join("")
  const publishKey = keyLabel(STATUS_KEYS.publishPage).join("")
  const tabs = TABS.filter((v) => v !== "settings" || canManage).map((v) => ({
    key: v,
    to: statusLink(pageId, v === "incidents" ? undefined : v),
    label: names[v],
    testId: `status-tab-${v}`,
  }))
  return (
    <Pane wide testId="status-page" key={p.id} className="gap-6">
      <PaneBack to="/status" label={t`Status`} always />
      <PageHeader
        back={false}
        title={<span data-testid="status-page-name">{p.name}</span>}
        description={
          <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <span className="inline-flex items-center gap-2 font-medium text-foreground" data-testid="status-overall">
              <StateDot state={p.state} className="size-2.5" />
              {text.component[p.state]}
            </span>
            {inbox && (
              <>
                <span aria-hidden>·</span>
                <span className="inline-flex items-center gap-1.5">
                  <InboxMark name={inbox.name} color={inbox.branding.color} className="size-4 text-[9px]" />
                  {inbox.name}
                </span>
              </>
            )}
            <span aria-hidden>·</span>
            <span data-testid="status-published">
              {p.published ? <Trans>Published</Trans> : <Trans>Unpublished, only your team can preview it</Trans>}
            </span>
          </span>
        }
        action={
          <>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPreview(true)}
              title={t`Preview (${previewKey})`}
              data-testid="status-preview"
            >
              <EyeIcon />
              <Trans>Preview</Trans>
            </Button>
            {p.published && (
              <Button
                variant="outline"
                size="sm"
                render={<a href={p.url} target="_blank" rel="noreferrer noopener" />}
                data-testid="status-open-public"
              >
                <ExternalLinkIcon />
                <Trans>View page</Trans>
              </Button>
            )}
            {canManage && (
              <Button
                variant={p.published ? "outline" : "default"}
                size="sm"
                disabled={patch.isPending}
                onClick={() => patch.mutate({ published: !p.published })}
                title={p.published ? t`Unpublish (${publishKey})` : t`Publish (${publishKey})`}
                data-testid="status-publish"
              >
                {p.published ? <Trans>Unpublish</Trans> : <Trans>Publish</Trans>}
              </Button>
            )}
          </>
        }
        tabs={<PageTabs label={t`Status page`} items={tabs} current={tab} />}
      >
        <ErrorLine error={patch.error} className="mt-2" />
      </PageHeader>
      {tab === "incidents" && <IncidentsTab page={p} />}
      {tab === "components" && <ComponentsTab page={p} />}
      {tab === "monitors" && <MonitorsTab page={p} />}
      {tab === "subscribers" && <SubscribersTab page={p} />}
      {tab === "settings" && canManage && <SettingsTab page={p} />}
      {preview && <PreviewDialog page={p} onClose={() => setPreview(false)} />}
    </Pane>
  )
}

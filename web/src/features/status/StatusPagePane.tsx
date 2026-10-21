import { Trans, useLingui } from "@lingui/react/macro"
import { ArrowUpRightIcon, EyeIcon } from "lucide-react"
import { useState } from "react"
import { useLocation, useNavigate, useParams } from "react-router"

import { ErrorLine, InboxMark } from "@/components/common"
import { keyLabel, STATUS_KEYS } from "@/components/common/ShortcutSheet"
import { Skeleton } from "@/components/ui/skeleton"
import { ComponentsTab } from "@/features/status/ComponentsTab"
import { IncidentDetail } from "@/features/status/IncidentPane"
import { IncidentsPage } from "@/features/status/IncidentsPage"
import { MonitorsPage } from "@/features/status/MonitorsTab"
import { FrameButton, FrameTabs, framePad, PageFrame } from "@/features/status/PageFrame"
import { PreviewDialog } from "@/features/status/PreviewDialog"
import { statusLink, usePatchStatusPage, useStatusPage } from "@/features/status/queries"
import { SettingsTab } from "@/features/status/SettingsTab"
import { SubscribersTab } from "@/features/status/SubscribersTab"
import { StateDot, useStatusText } from "@/features/status/text"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

const TABS = ["incidents", "components", "monitors", "subscribers", "settings"] as const
type Tab = (typeof TABS)[number]

export { StatusHome } from "@/features/status/StatusPages"

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
  const p = page.data
  const setView = (v: Tab) => navigate(statusLink(pageId, v === "incidents" ? undefined : v))

  useHotkeys(
    {
      [STATUS_KEYS.incidents]: () => setView("incidents"),
      [STATUS_KEYS.components]: () => (tab === "components" ? undefined : setView("components")),
      [STATUS_KEYS.monitors]: () => setView("monitors"),
      [STATUS_KEYS.subscribers]: () => (tab === "subscribers" ? undefined : setView("subscribers")),
      ...(canManage ? { [STATUS_KEYS.settings]: () => setView("settings") } : {}),
      ...(tab !== "incidents" && tab !== "monitors"
        ? {
            [STATUS_KEYS.preview]: () => setPreview(true),
            ...(canManage && p ? { [STATUS_KEYS.publishPage]: () => patch.mutate({ published: !p.published }) } : {}),
          }
        : {}),
    },
    !!p
  )

  if (tab === "incidents")
    return <IncidentsPage detail={(i) => <IncidentDetail key={i.id} pageId={i.page_id} i={i} />} />
  if (tab === "monitors") return <MonitorsPage pageId={pageId} />

  if (page.isPending) {
    return (
      <div className="flex flex-col gap-4 p-6">
        <Skeleton className="h-8 w-60" />
        <Skeleton className="h-24 w-full rounded-xl" />
      </div>
    )
  }
  if (!p) {
    return (
      <PageFrame title={<Trans>Status pages</Trans>} back={{ to: "/status", label: t`Status` }}>
        <ErrorLine error={page.error} className="m-6" />
      </PageFrame>
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
  const tabs = (["components", "subscribers", "settings"] as const)
    .filter((v) => v !== "settings" || canManage)
    .map((v) => ({
      key: v,
      to: statusLink(pageId, v),
      label: names[v],
      testId: `status-tab-${v}`,
    }))
  return (
    <PageFrame
      testId="status-page"
      key={p.id}
      back={{ to: `/status?page=${p.id}`, label: t`Status pages` }}
      title={
        <span className="flex items-center gap-2.5">
          {inbox && <InboxMark name={inbox.name} color={inbox.branding.color} className="size-6 text-caption" />}
          <span data-testid="status-page-name">{p.name}</span>
        </span>
      }
      description={
        <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <span className="inline-flex items-center gap-2 font-medium text-foreground" data-testid="status-overall">
            <StateDot state={p.state} className="size-2.5" />
            {text.component[p.state]}
          </span>
          <span aria-hidden>·</span>
          <span data-testid="status-published">
            {p.published ? <Trans>Published</Trans> : <Trans>Unpublished, only your team can preview it</Trans>}
          </span>
        </span>
      }
      actions={
        <>
          <FrameButton onClick={() => setPreview(true)} title={t`Preview (${previewKey})`} data-testid="status-preview">
            <EyeIcon />
            <Trans>Preview</Trans>
          </FrameButton>
          {p.published && (
            <FrameButton
              render={<a href={p.url} target="_blank" rel="noreferrer noopener" />}
              data-testid="status-open-public"
            >
              <Trans>View page</Trans>
              <ArrowUpRightIcon className="rtl:-scale-x-100" />
            </FrameButton>
          )}
          {canManage && (
            <FrameButton
              ink={!p.published}
              disabled={patch.isPending}
              onClick={() => patch.mutate({ published: !p.published })}
              title={p.published ? t`Unpublish (${publishKey})` : t`Publish (${publishKey})`}
              data-testid="status-publish"
            >
              {p.published ? <Trans>Unpublish</Trans> : <Trans>Publish</Trans>}
            </FrameButton>
          )}
        </>
      }
      notice={<ErrorLine error={patch.error} />}
      tabs={<FrameTabs label={t`Status page`} items={tabs} current={tab} />}
    >
      <div className={cn(framePad, "max-w-[1040px]")}>
        {tab === "components" && <ComponentsTab page={p} />}
        {tab === "subscribers" && <SubscribersTab page={p} />}
        {tab === "settings" && canManage && <SettingsTab page={p} />}
      </div>
      {preview && <PreviewDialog page={p} onClose={() => setPreview(false)} />}
    </PageFrame>
  )
}

import { Trans, useLingui } from "@lingui/react/macro"
import { ActivityIcon, EyeIcon, ExternalLinkIcon } from "lucide-react"
import { useState } from "react"
import { useParams, useSearchParams } from "react-router"

import { Dot, ErrorLine, Segmented } from "@/components/common"
import { Pane, PaneBack, PaneEmpty } from "@/components/common/Column"
import { keyLabel, STATUS_KEYS } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { ComponentsTab } from "@/features/status/ComponentsTab"
import { IncidentsTab } from "@/features/status/IncidentsTab"
import { PreviewDialog } from "@/features/status/PreviewDialog"
import { usePatchStatusPage, useStatusPage } from "@/features/status/queries"
import { SettingsTab } from "@/features/status/SettingsTab"
import { StateDot, useStatusText } from "@/features/status/text"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

const TABS = ["incidents", "components", "settings"] as const
type Tab = (typeof TABS)[number]

export function StatusHome() {
  return (
    <PaneEmpty icon={ActivityIcon} title={<Trans>Status pages</Trans>} testId="status-home">
      <Trans>
        Tell your customers whether your product works. Pick an inbox’s page to open incidents, plan maintenance and set
        the state of each component.
      </Trans>
    </PaneEmpty>
  )
}

export function StatusPagePane() {
  const { t } = useLingui()
  const { pageId = "" } = useParams()
  const [params, setParams] = useSearchParams()
  const page = useStatusPage(pageId)
  const patch = usePatchStatusPage(pageId)
  const { canManage } = useSession()
  const text = useStatusText()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === page.data?.inbox_id)
  const [preview, setPreview] = useState(false)
  const raw = params.get("tab")
  const tab: Tab = TABS.includes(raw as Tab) && (raw !== "settings" || canManage) ? (raw as Tab) : "incidents"
  const setTab = (v: Tab) => setParams(v === "incidents" ? {} : { tab: v }, { replace: true })
  const p = page.data

  useHotkeys(
    {
      [STATUS_KEYS.incidents]: () => setTab("incidents"),
      [STATUS_KEYS.components]: () => setTab("components"),
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
        <PaneBack to="/status" label={t`Status`} />
        <ErrorLine error={page.error} />
      </Pane>
    )
  }
  const names: Record<Tab, string> = {
    incidents: t`Incidents`,
    components: t`Components`,
    settings: t`Settings`,
  }
  const previewKey = keyLabel(STATUS_KEYS.preview).join("")
  const publishKey = keyLabel(STATUS_KEYS.publishPage).join("")
  return (
    <Pane wide testId="status-page" key={p.id}>
      <header className="flex flex-col">
        <PaneBack to="/status" label={t`Status`} />
        <div className="flex flex-wrap items-start justify-between gap-3">
          <h1 className="min-w-0 text-page break-words" data-testid="status-page-name">
            {p.name}
          </h1>
          <div className="flex flex-wrap items-center gap-2">
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
          </div>
        </div>
        <p className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-body text-muted-foreground">
          {inbox && (
            <>
              <span className="inline-flex items-center gap-1.5">
                <Dot color={inbox.branding.color} className="size-[7px] rounded-[2px]" />
                {inbox.name}
              </span>
              <span aria-hidden>·</span>
            </>
          )}
          <span className="inline-flex items-center gap-1.5" data-testid="status-overall">
            <StateDot state={p.state} />
            {text.component[p.state]}
          </span>
          <span aria-hidden>·</span>
          <span data-testid="status-published">
            {p.published ? <Trans>Published</Trans> : <Trans>Unpublished, only your team can preview it</Trans>}
          </span>
        </p>
        <ErrorLine error={patch.error} className="mt-2" />
      </header>
      <div className="-mx-1 -my-3 max-w-[calc(100%+0.5rem)] overflow-x-auto px-1 py-1">
        <Segmented
          value={tab}
          onChange={setTab}
          label={t`Status page`}
          items={TABS.filter((v) => v !== "settings" || canManage).map((v) => ({
            value: v,
            label: names[v],
            testId: `status-tab-${v}`,
          }))}
        />
      </div>
      {tab === "incidents" && <IncidentsTab page={p} />}
      {tab === "components" && <ComponentsTab page={p} />}
      {tab === "settings" && canManage && <SettingsTab page={p} />}
      {preview && <PreviewDialog page={p} onClose={() => setPreview(false)} />}
    </Pane>
  )
}

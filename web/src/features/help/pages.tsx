import "@/features/help/i18n"
import { Trans, useLingui } from "@lingui/react/macro"
import { EyeIcon, ExternalLinkIcon, LifeBuoyIcon, LoaderIcon } from "lucide-react"
import { useState } from "react"
import { useNavigate, useParams, useSearchParams } from "react-router"

import { Dot } from "@/components/common"
import { Pane, PaneBack, PaneEmpty } from "@/components/common/Column"
import { HELP_KEYS, keyLabel } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { ArticlesTab } from "@/features/help/ArticlesTab"
import { CollectionsTab } from "@/features/help/CollectionsTab"
import { centerLink, useHelpCenter, useHelpPreview, useUpdateHelpCenter } from "@/features/help/queries"
import { ReportTab } from "@/features/help/ReportTab"
import { SettingsTab } from "@/features/help/SettingsTab"
import { ErrorLine } from "@/features/help/text"
import { UnderlineTabs } from "@/features/inbox/ListHead"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { HelpCenter } from "@/lib/api"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

export { ArticleEditorPage } from "@/features/help/ArticleEditor"

const TABS = ["articles", "collections", "report", "settings"] as const
type Tab = (typeof TABS)[number]

export function HelpHome() {
  return (
    <PaneEmpty icon={LifeBuoyIcon} title={<Trans>Help centers</Trans>} testId="help-home">
      <Trans>
        Answer common questions once, in writing. Pick an inbox’s help center to write articles, group them in
        collections and see what customers search for.
      </Trans>
    </PaneEmpty>
  )
}

function PreviewDialog({ center, onClose }: { center: HelpCenter; onClose: () => void }) {
  const { t } = useLingui()
  const preview = useHelpPreview(center.id, true)
  const html = preview.data ? `<base target="_blank">${preview.data}` : undefined
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        className="h-[calc(100svh-2rem)] grid-rows-[auto_1fr] sm:max-w-4xl"
        data-testid="help-preview-dialog"
      >
        <DialogHeader>
          <DialogTitle>
            <Trans>Preview</Trans>
          </DialogTitle>
          <DialogDescription>
            {center.published ? (
              <Trans>The help center as visitors see it now. Drafts are left out.</Trans>
            ) : (
              <Trans>The help center is not published. Only your team sees this.</Trans>
            )}
          </DialogDescription>
        </DialogHeader>
        <div className="relative min-h-0 overflow-hidden rounded-xl border bg-background">
          {preview.isPending && (
            <div className="grid h-full place-items-center text-faint">
              <LoaderIcon className="animate-spin" />
            </div>
          )}
          {preview.error && <ErrorLine error={preview.error} className="p-4" />}
          {html && (
            <iframe
              title={t`Preview of ${center.name}`}
              srcDoc={html}
              sandbox="allow-popups allow-popups-to-escape-sandbox"
              className="size-full bg-white"
              data-testid="help-preview-frame"
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

export function CenterPane() {
  const { t } = useLingui()
  const { centerId = "" } = useParams()
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const center = useHelpCenter(centerId)
  const patch = useUpdateHelpCenter(centerId)
  const { canManage } = useSession()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === center.data?.inbox_id)
  const [preview, setPreview] = useState(false)
  const raw = params.get("tab")
  const tab: Tab = TABS.includes(raw as Tab) && (raw !== "settings" || canManage) ? (raw as Tab) : "articles"
  const setTab = (v: Tab) => navigate(centerLink(centerId, v === "articles" ? undefined : v), { replace: true })
  const c = center.data

  useHotkeys(
    {
      [HELP_KEYS.articles]: () => setTab("articles"),
      [HELP_KEYS.collections]: () => setTab("collections"),
      [HELP_KEYS.report]: () => setTab("report"),
      ...(canManage ? { [HELP_KEYS.settings]: () => setTab("settings") } : {}),
      [HELP_KEYS.preview]: () => setPreview(true),
      ...(c?.published ? { [HELP_KEYS.openPage]: () => window.open(c.url, "_blank", "noopener,noreferrer") } : {}),
      ...(canManage && c ? { [HELP_KEYS.publishCenter]: () => patch.mutate({ published: !c.published }) } : {}),
    },
    !!c
  )

  if (center.isPending) {
    return (
      <Pane wide className="gap-4">
        <Skeleton className="h-8 w-60" />
        <Skeleton className="h-24 w-full rounded-2xl" />
      </Pane>
    )
  }
  if (!c) {
    return (
      <Pane wide>
        <PaneBack to="/knowledge" label={t`Help`} />
        <ErrorLine error={center.error} />
      </Pane>
    )
  }
  const names: Record<Tab, string> = {
    articles: t`Articles`,
    collections: t`Collections`,
    report: t`Report`,
    settings: t`Settings`,
  }
  const previewKey = keyLabel(HELP_KEYS.preview).join("")
  const publishKey = keyLabel(HELP_KEYS.publishCenter).join("")
  return (
    <Pane wide testId="help-center" key={c.id}>
      <header className="flex flex-col">
        <PaneBack to="/knowledge" label={t`Help`} />
        <div className="flex flex-wrap items-start justify-between gap-3">
          <h1 className="min-w-0 text-page break-words" data-testid="help-center-name">
            {c.name}
          </h1>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPreview(true)}
              title={t`Preview (${previewKey})`}
              data-testid="help-preview"
            >
              <EyeIcon />
              <Trans>Preview</Trans>
            </Button>
            {c.published && (
              <Button
                variant="outline"
                size="sm"
                render={<a href={c.url} target="_blank" rel="noreferrer noopener" />}
                data-testid="help-open-public"
              >
                <ExternalLinkIcon />
                <Trans>View help center</Trans>
              </Button>
            )}
            {canManage && (
              <Button
                variant={c.published ? "outline" : "default"}
                size="sm"
                disabled={patch.isPending}
                onClick={() => patch.mutate({ published: !c.published })}
                title={c.published ? t`Unpublish (${publishKey})` : t`Publish (${publishKey})`}
                data-testid="help-publish"
              >
                {c.published ? <Trans>Unpublish</Trans> : <Trans>Publish</Trans>}
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
          <span data-testid="help-published">
            {c.published ? <Trans>Published</Trans> : <Trans>Unpublished, only your team can preview it</Trans>}
          </span>
        </p>
        <ErrorLine error={patch.error} className="mt-2" />
      </header>
      <div className="-mt-5 border-b">
        <UnderlineTabs
          value={tab}
          onChange={setTab}
          label={t`Help center`}
          className="mt-0"
          items={TABS.filter((v) => v !== "settings" || canManage).map((v) => ({
            value: v,
            label: names[v],
            testId: `help-tab-${v}`,
          }))}
        />
      </div>
      {tab === "articles" && <ArticlesTab center={c} />}
      {tab === "collections" && <CollectionsTab center={c} />}
      {tab === "report" && <ReportTab center={c} />}
      {tab === "settings" && canManage && <SettingsTab center={c} />}
      {preview && <PreviewDialog center={c} onClose={() => setPreview(false)} />}
    </Pane>
  )
}

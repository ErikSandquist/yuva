import "@/features/changelog/i18n"
import { Trans, useLingui } from "@lingui/react/macro"
import { EyeIcon, ExternalLinkIcon, SparklesIcon } from "lucide-react"
import { useState } from "react"
import { useNavigate, useParams, useSearchParams } from "react-router"

import { Dot, Segmented } from "@/components/common"
import { Pane, PaneBack, PaneEmpty } from "@/components/common/Column"
import { CHANGELOG_KEYS, keyLabel } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { EntriesTab } from "@/features/changelog/EntriesTab"
import { changelogLink, useChangelog, useUpdateChangelog } from "@/features/changelog/queries"
import { PreviewDialog } from "@/features/changelog/PreviewDialog"
import { SettingsTab } from "@/features/changelog/SettingsTab"
import { ErrorLine } from "@/features/changelog/text"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

export { EntryEditorPage } from "@/features/changelog/EntryEditor"

const TABS = ["entries", "settings"] as const
type Tab = (typeof TABS)[number]

export function ChangelogHome() {
  return (
    <PaneEmpty icon={SparklesIcon} title={<Trans>What’s new</Trans>} testId="changelog-home">
      <Trans>
        Tell your customers what is new, improved and fixed, on a public page, in the chat widget and by mail. Pick an
        inbox’s changelog to write entries.
      </Trans>
    </PaneEmpty>
  )
}

export function ChangelogPane() {
  const { t } = useLingui()
  const { changelogId = "" } = useParams()
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const changelog = useChangelog(changelogId)
  const patch = useUpdateChangelog(changelogId)
  const { canManage } = useSession()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === changelog.data?.inbox_id)
  const [preview, setPreview] = useState(false)
  const raw = params.get("tab")
  const tab: Tab = raw === "settings" && canManage ? "settings" : "entries"
  const setTab = (v: Tab) => navigate(changelogLink(changelogId, v === "entries" ? undefined : v), { replace: true })
  const c = changelog.data

  useHotkeys(
    {
      [CHANGELOG_KEYS.entries]: () => setTab("entries"),
      [CHANGELOG_KEYS.preview]: () => setPreview(true),
      ...(canManage ? { [CHANGELOG_KEYS.settings]: () => setTab("settings") } : {}),
      ...(c?.published ? { [CHANGELOG_KEYS.openPage]: () => window.open(c.url, "_blank", "noopener,noreferrer") } : {}),
      ...(canManage && c ? { [CHANGELOG_KEYS.publish]: () => patch.mutate({ published: !c.published }) } : {}),
    },
    !!c
  )

  if (changelog.isPending) {
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
        <PaneBack to="/updates" label={t`What’s new`} />
        <ErrorLine error={changelog.error} />
      </Pane>
    )
  }
  const names: Record<Tab, string> = { entries: t`Entries`, settings: t`Settings` }
  const previewKey = keyLabel(CHANGELOG_KEYS.preview).join("")
  const publishKey = keyLabel(CHANGELOG_KEYS.publish).join("")
  return (
    <Pane wide testId="changelog" key={c.id}>
      <header className="flex flex-col">
        <PaneBack to="/updates" label={t`What’s new`} />
        <div className="flex flex-wrap items-start justify-between gap-3">
          <h1 className="min-w-0 text-page break-words" data-testid="changelog-name">
            {c.name}
          </h1>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPreview(true)}
              title={t`Preview (${previewKey})`}
              data-testid="changelog-preview"
            >
              <EyeIcon />
              <Trans>Preview</Trans>
            </Button>
            {c.published && (
              <Button
                variant="outline"
                size="sm"
                render={<a href={c.url} target="_blank" rel="noreferrer noopener" />}
                data-testid="changelog-open-public"
              >
                <ExternalLinkIcon />
                <Trans>View changelog</Trans>
              </Button>
            )}
            {canManage && (
              <Button
                variant={c.published ? "outline" : "default"}
                size="sm"
                disabled={patch.isPending}
                onClick={() => patch.mutate({ published: !c.published })}
                title={c.published ? t`Unpublish (${publishKey})` : t`Publish (${publishKey})`}
                data-testid="changelog-publish"
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
          <span data-testid="changelog-published">
            {c.published ? <Trans>Published</Trans> : <Trans>Unpublished, only your team can see it</Trans>}
          </span>
        </p>
        <ErrorLine error={patch.error} className="mt-2" />
      </header>
      <div className="-mx-1 -my-3 max-w-[calc(100%+0.5rem)] overflow-x-auto px-1 py-1">
        <Segmented
          value={tab}
          onChange={setTab}
          label={t`Changelog`}
          items={TABS.filter((v) => v !== "settings" || canManage).map((v) => ({
            value: v,
            label: names[v],
            testId: `changelog-tab-${v}`,
          }))}
        />
      </div>
      {tab === "entries" && <EntriesTab changelog={c} />}
      {tab === "settings" && canManage && <SettingsTab changelog={c} />}
      {preview && <PreviewDialog changelog={c} onClose={() => setPreview(false)} />}
    </Pane>
  )
}

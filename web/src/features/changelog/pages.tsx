import "@/features/changelog/i18n"
import { Trans, useLingui } from "@lingui/react/macro"
import { EyeIcon, ExternalLinkIcon, PlusIcon, SparklesIcon } from "lucide-react"
import { useState } from "react"
import { Link, useNavigate, useParams, useSearchParams } from "react-router"

import { InboxMark } from "@/components/common"
import { Pane, PaneBack } from "@/components/common/Column"
import { CHANGELOG_KEYS, keyLabel } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { CreateChangelogDialog } from "@/features/changelog/CreateChangelogDialog"
import { EntriesTab } from "@/features/changelog/EntriesTab"
import {
  changelogLink,
  changelogOfInbox,
  useChangelog,
  useChangelogs,
  useUpdateChangelog,
} from "@/features/changelog/queries"
import { PreviewDialog } from "@/features/changelog/PreviewDialog"
import { SettingsTab } from "@/features/changelog/SettingsTab"
import { ErrorLine } from "@/features/changelog/text"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { PageHeader, PageTabs } from "@/features/settings/ui"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useRowFocus } from "@/hooks/use-row-focus"
import type { Inbox } from "@/lib/api"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

export { EntryEditorPage } from "@/features/changelog/EntryEditor"

const TABS = ["entries", "settings"] as const
type Tab = (typeof TABS)[number]

export function ChangelogHome() {
  const { t, i18n } = useLingui()
  const [inboxId] = useInboxFilter()
  const inboxes = useInboxes().data ?? []
  const changelogs = useChangelogs()
  const fmt = new Intl.NumberFormat(i18n.locale)
  const byInbox = new Map(inboxes.map((i) => [i.id, i]))
  const items = (changelogs.data ?? []).filter((c) => !inboxId || c.inbox_id === inboxId)
  const navigate = useNavigate()
  const { canManage } = useSession()
  const [creating, setCreating] = useState<Inbox | null>(null)
  const shown = inboxId ? inboxes.filter((i) => i.id === inboxId) : inboxes
  const missing = canManage && changelogs.data ? shown.find((i) => !changelogOfInbox(changelogs.data, i.id)) : undefined
  const grid = useRowFocus<HTMLUListElement>(!creating)
  const createKey = keyLabel(CHANGELOG_KEYS.create).join("")
  useHotkeys(missing ? { [CHANGELOG_KEYS.create]: () => setCreating(missing) } : {}, !creating)
  return (
    <Pane wide testId="changelog-home" className="gap-6">
      <PageHeader
        back={false}
        title={<Trans>What’s new</Trans>}
        description={<Trans>Release notes shown in your products, on the web and by mail.</Trans>}
        action={
          missing && (
            <Button
              size="sm"
              onClick={() => setCreating(missing)}
              title={t`Create changelog (${createKey})`}
              data-testid="create-changelog-open"
            >
              <PlusIcon />
              <Trans>Create changelog</Trans>
            </Button>
          )
        }
      />
      <ErrorLine error={changelogs.error} />
      {changelogs.isPending ? (
        <div className="grid gap-3 sm:grid-cols-2">
          <Skeleton className="h-28 rounded-2xl" />
          <Skeleton className="h-28 rounded-2xl" />
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-2xl border border-dashed px-6 py-12 text-center">
          <SparklesIcon className="size-6 text-faint" />
          <p className="max-w-md text-body text-muted-foreground">
            <Trans>
              Tell your customers what is new, improved and fixed, on a public page, in the chat widget and by mail.
              Pick an inbox’s changelog to write entries.
            </Trans>
          </p>
        </div>
      ) : (
        <ul ref={grid} className="grid gap-3 sm:grid-cols-2" aria-label={t`Changelogs`}>
          {items.map((c) => {
            const inbox = byInbox.get(c.inbox_id)
            const published = fmt.format(c.counts.published)
            const drafts = fmt.format(Math.max(0, c.counts.entries - c.counts.published - c.counts.scheduled))
            const scheduled = fmt.format(c.counts.scheduled)
            return (
              <li key={c.id}>
                <Link
                  to={changelogLink(c.id)}
                  className="flex h-full flex-col gap-3 rounded-2xl border bg-card p-4 outline-none transition-colors hover:border-faint focus-visible:ring-2 focus-visible:ring-ring"
                  data-testid="changelog-home-row"
                >
                  <span className="flex items-center gap-2.5">
                    {inbox && (
                      <InboxMark name={inbox.name} color={inbox.branding.color} className="size-6 text-caption" />
                    )}
                    <span className="min-w-0 flex-1 truncate text-body font-semibold">{c.name}</span>
                    <span className="shrink-0 text-caption text-faint">
                      {c.published ? <Trans>Published</Trans> : <Trans>Unpublished</Trans>}
                    </span>
                  </span>
                  <span className="mt-auto flex flex-wrap gap-x-4 gap-y-1 border-t pt-3 text-small text-muted-foreground tabular-nums">
                    <span>
                      <Trans>{published} published</Trans>
                    </span>
                    <span>
                      <Trans>{scheduled} scheduled</Trans>
                    </span>
                    <span>
                      <Trans>{drafts} drafts</Trans>
                    </span>
                  </span>
                </Link>
              </li>
            )
          })}
        </ul>
      )}
      {creating && (
        <CreateChangelogDialog
          inboxId={creating.id}
          inboxName={creating.name}
          onClose={() => setCreating(null)}
          onCreated={(c) => {
            setCreating(null)
            navigate(changelogLink(c.id, "settings"))
          }}
        />
      )}
    </Pane>
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
        <PaneBack to="/updates" label={t`What’s new`} always />
        <ErrorLine error={changelog.error} />
      </Pane>
    )
  }
  const names: Record<Tab, string> = { entries: t`Entries`, settings: t`Settings` }
  const previewKey = keyLabel(CHANGELOG_KEYS.preview).join("")
  const publishKey = keyLabel(CHANGELOG_KEYS.publish).join("")
  const tabs = TABS.filter((v) => v !== "settings" || canManage).map((v) => ({
    key: v,
    to: changelogLink(changelogId, v === "entries" ? undefined : v),
    label: names[v],
    count: v === "entries" ? c.counts.entries : undefined,
    testId: `changelog-tab-${v}`,
  }))
  return (
    <Pane wide testId="changelog" key={c.id} className="gap-6">
      <PaneBack to="/updates" label={t`What’s new`} always />
      <PageHeader
        back={false}
        title={<span data-testid="changelog-name">{c.name}</span>}
        description={
          <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
            {inbox && (
              <>
                <span className="inline-flex items-center gap-1.5">
                  <InboxMark name={inbox.name} color={inbox.branding.color} className="size-4 text-[9px]" />
                  {inbox.name}
                </span>
                <span aria-hidden>·</span>
              </>
            )}
            <span data-testid="changelog-published">
              {c.published ? <Trans>Published</Trans> : <Trans>Unpublished, only your team can see it</Trans>}
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
          </>
        }
        tabs={<PageTabs label={t`Changelog`} items={tabs} current={tab} />}
      >
        <ErrorLine error={patch.error} className="mt-2" />
      </PageHeader>
      {tab === "entries" && <EntriesTab changelog={c} />}
      {tab === "settings" && canManage && <SettingsTab changelog={c} />}
      {preview && <PreviewDialog changelog={c} onClose={() => setPreview(false)} />}
    </Pane>
  )
}

import "@/features/changelog/i18n"
import { Trans, useLingui } from "@lingui/react/macro"
import { ArrowUpRightIcon, EyeIcon, PlusIcon, SettingsIcon, SparklesIcon } from "lucide-react"
import { cloneElement, useEffect, useRef, useState } from "react"
import { Link, useNavigate, useParams, useSearchParams } from "react-router"

import { InboxMark } from "@/components/common"
import { CHANGELOG_KEYS, keyLabel } from "@/components/common/ShortcutSheet"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { CreateChangelogDialog } from "@/features/changelog/CreateChangelogDialog"
import { EntriesTab, EntryPreview, EntryRow, NewEntryDialog } from "@/features/changelog/EntriesTab"
import {
  changelogLink,
  changelogOfInbox,
  entryLink,
  useAllEntries,
  useChangelog,
  useChangelogs,
  useUpdateChangelog,
} from "@/features/changelog/queries"
import { PreviewDialog } from "@/features/changelog/PreviewDialog"
import { SettingsTab } from "@/features/changelog/SettingsTab"
import { ErrorLine } from "@/features/changelog/text"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { FrameButton, FrameTabs, framePad, PageFrame } from "@/features/status/PageFrame"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { Changelog, ChangelogEntrySummary, Inbox } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

export { EntryEditorPage } from "@/features/changelog/EntryEditor"

const TABS = ["entries", "settings"] as const
type Tab = (typeof TABS)[number]

const VIEWS = ["all", "published", "draft"] as const
type View = (typeof VIEWS)[number]

function ChangelogPicker({
  items,
  inboxOf,
  onPick,
  button,
  children,
}: {
  items: Changelog[]
  inboxOf: (c: Changelog) => Inbox | undefined
  onPick: (c: Changelog) => void
  button: React.ReactElement<{ onClick?: () => void }>
  children: React.ReactNode
}) {
  if (items.length === 1) return cloneElement(button, { onClick: () => onPick(items[0]) }, children)
  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={button}>{children}</DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-52">
        {items.map((c) => {
          const inbox = inboxOf(c)
          return (
            <DropdownMenuItem key={c.id} onClick={() => onPick(c)}>
              <InboxMark name={inbox?.name ?? c.name} color={inbox?.branding.color} className="size-4 text-[9px]" />
              {c.name}
            </DropdownMenuItem>
          )
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function ChangelogHome() {
  const { t } = useLingui()
  const [inboxId] = useInboxFilter()
  const [params] = useSearchParams()
  const inboxes = useInboxes().data ?? []
  const changelogs = useChangelogs()
  const byInbox = new Map(inboxes.map((i) => [i.id, i]))
  const inboxOf = (c: Changelog) => byInbox.get(c.inbox_id)
  const items = (changelogs.data ?? []).filter((c) => !inboxId || c.inbox_id === inboxId)
  const byId = new Map(items.map((c) => [c.id, c]))
  const navigate = useNavigate()
  const { canManage } = useSession()
  const [creating, setCreating] = useState<Inbox | null>(null)
  const [writing, setWriting] = useState<Changelog | null>(null)
  const [cursor, setCursor] = useState(-1)
  const [picked, setPicked] = useState("")
  const list = useRef<HTMLUListElement>(null)
  const raw = params.get("state") as View | null
  const view: View = raw && VIEWS.includes(raw) ? raw : "all"
  const entries = useAllEntries(
    items.map((c) => c.id),
    view === "all" ? {} : { state: view }
  )
  const when = (e: ChangelogEntrySummary) => e.published_at ?? e.publish_at ?? e.created_at
  const rows = entries.data.filter((e) => byId.has(e.changelog_id)).sort((a, b) => when(b).localeCompare(when(a)))
  const shown = rows[cursor] ?? rows.find((e) => e.id === picked) ?? rows[0]
  const shownLog = shown && byId.get(shown.changelog_id)
  const shownInboxes = inboxId ? inboxes.filter((i) => i.id === inboxId) : inboxes
  const missing =
    canManage && changelogs.data ? shownInboxes.find((i) => !changelogOfInbox(changelogs.data, i.id)) : undefined
  const createKey = keyLabel(CHANGELOG_KEYS.create).join("")
  const newKey = keyLabel(CHANGELOG_KEYS.newEntry).join("")
  const open = !creating && !writing
  useHotkeys(
    {
      ...(missing ? { [CHANGELOG_KEYS.create]: () => setCreating(missing) } : {}),
      ...(items.length > 0 ? { [CHANGELOG_KEYS.newEntry]: () => setWriting(shownLog ?? items[0]) } : {}),
      j: () => setCursor((i) => Math.min(rows.length - 1, i + 1)),
      k: () => setCursor((i) => Math.max(0, i - 1)),
      ArrowDown: () => setCursor((i) => Math.min(rows.length - 1, i + 1)),
      ArrowUp: () => setCursor((i) => Math.max(0, i - 1)),
      ...(cursor >= 0 && rows[cursor] && byId.get(rows[cursor].changelog_id)
        ? {
            Enter: () =>
              navigate(
                entryLink(
                  rows[cursor].changelog_id,
                  rows[cursor].id,
                  byId.get(rows[cursor].changelog_id)?.default_locale
                )
              ),
          }
        : {}),
    },
    open
  )
  useEffect(() => {
    if (cursor >= 0) list.current?.children[cursor]?.scrollIntoView({ block: "nearest" })
  }, [cursor])
  const tabs = VIEWS.map((v) => ({
    key: v,
    to: v === "all" ? "/updates" : `/updates?state=${v}`,
    label: { all: t`All`, published: t`Published`, draft: t`Drafts` }[v],
    testId: `changelog-view-${v}`,
  }))
  return (
    <PageFrame
      testId="changelog-home"
      title={<Trans>What’s new</Trans>}
      description={<Trans>Release notes shown in your products and on the web</Trans>}
      actions={
        <>
          {(items.length > 0 || missing) && (
            <DropdownMenu>
              <DropdownMenuTrigger
                render={<FrameButton aria-label={t`Changelogs`} title={t`Changelogs`} className="px-2.5" />}
                data-testid="changelog-home-menu"
              >
                <SettingsIcon />
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="min-w-56">
                {items.map((c) => {
                  const inbox = inboxOf(c)
                  return (
                    <DropdownMenuItem
                      key={c.id}
                      render={<Link to={changelogLink(c.id)} />}
                      data-testid="changelog-home-row"
                    >
                      <InboxMark
                        name={inbox?.name ?? c.name}
                        color={inbox?.branding.color}
                        className="size-4 text-[9px]"
                      />
                      <span className="flex-1 truncate">{c.name}</span>
                      <span className="text-caption text-faint">
                        {c.published ? <Trans>Published</Trans> : <Trans>Unpublished</Trans>}
                      </span>
                    </DropdownMenuItem>
                  )
                })}
                {missing && (
                  <DropdownMenuItem onClick={() => setCreating(missing)} data-testid="create-changelog-open">
                    <PlusIcon />
                    <Trans>Create changelog</Trans>
                    <DropdownMenuShortcut>{createKey}</DropdownMenuShortcut>
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
          {items.length > 0 && (
            <ChangelogPicker
              items={items}
              inboxOf={inboxOf}
              onPick={setWriting}
              button={<FrameButton ink title={t`New post (${newKey})`} data-testid="new-entry" />}
            >
              <Trans>New post</Trans>
            </ChangelogPicker>
          )}
        </>
      }
      tabs={<FrameTabs label={t`What’s new`} items={tabs} current={view} />}
      bodyClassName="@container flex overflow-hidden in-data-frame:overflow-hidden"
    >
      <div className="flex min-w-0 flex-1 flex-col overflow-y-auto">
        <ErrorLine error={changelogs.error ?? entries.error} className="m-6" />
        {changelogs.isPending || (items.length > 0 && entries.isPending) ? (
          <div className="flex flex-col gap-2 p-6">
            <Skeleton className="h-14 w-full rounded-xl" />
            <Skeleton className="h-14 w-full rounded-xl" />
            <Skeleton className="h-14 w-full rounded-xl" />
          </div>
        ) : items.length === 0 ? (
          <div className="m-6 flex flex-col items-center gap-3 rounded-xl border border-dashed px-6 py-12 text-center">
            <SparklesIcon className="size-6 text-faint" />
            <p className="max-w-md text-body text-muted-foreground">
              <Trans>
                Tell your customers what is new, improved and fixed, on a public page, in the chat widget and by mail.
              </Trans>
            </p>
            {missing && (
              <FrameButton ink onClick={() => setCreating(missing)} title={t`Create changelog (${createKey})`}>
                <PlusIcon />
                <Trans>Create changelog</Trans>
              </FrameButton>
            )}
          </div>
        ) : rows.length === 0 ? (
          <p className="py-12 text-center text-faint" data-testid="changelog-entries-empty">
            <Trans>Nothing here.</Trans>
          </p>
        ) : (
          <ul ref={list} className="flex flex-col" aria-label={t`Entries`}>
            {rows.map((e, i) => {
              const c = byId.get(e.changelog_id)
              if (!c) return null
              return (
                <EntryRow
                  key={e.id}
                  e={e}
                  changelog={c}
                  inbox={inboxOf(c)}
                  current={i === cursor || (cursor < 0 && e.id === shown?.id)}
                  onPick={() => {
                    setPicked(e.id)
                    setCursor(-1)
                  }}
                />
              )
            })}
          </ul>
        )}
      </div>
      {shown && shownLog && (
        <div className="hidden min-h-0 @min-[1000px]:flex">
          <EntryPreview changelog={shownLog} entry={shown} />
        </div>
      )}
      {writing && <NewEntryDialog changelog={writing} onClose={() => setWriting(null)} />}
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
    </PageFrame>
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
      <div className="flex flex-col gap-4 p-6">
        <Skeleton className="h-8 w-60" />
        <Skeleton className="h-24 w-full rounded-xl" />
      </div>
    )
  }
  if (!c) {
    return (
      <PageFrame title={<Trans>What’s new</Trans>} back={{ to: "/updates", label: t`What’s new` }}>
        <ErrorLine error={changelog.error} className="m-6" />
      </PageFrame>
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
    <PageFrame
      testId="changelog"
      key={c.id}
      back={{ to: "/updates", label: t`What’s new` }}
      title={
        <span className="flex items-center gap-2.5">
          {inbox && <InboxMark name={inbox.name} color={inbox.branding.color} className="size-6 text-caption" />}
          <span data-testid="changelog-name">{c.name}</span>
        </span>
      }
      description={
        <span data-testid="changelog-published">
          {c.published ? <Trans>Published</Trans> : <Trans>Unpublished, only your team can see it</Trans>}
        </span>
      }
      actions={
        <>
          <FrameButton
            onClick={() => setPreview(true)}
            title={t`Preview (${previewKey})`}
            data-testid="changelog-preview"
          >
            <EyeIcon />
            <Trans>Preview</Trans>
          </FrameButton>
          {c.published && (
            <FrameButton
              render={<a href={c.url} target="_blank" rel="noreferrer noopener" />}
              data-testid="changelog-open-public"
            >
              <Trans>View changelog</Trans>
              <ArrowUpRightIcon className="rtl:-scale-x-100" />
            </FrameButton>
          )}
          {canManage && (
            <FrameButton
              ink={!c.published}
              disabled={patch.isPending}
              onClick={() => patch.mutate({ published: !c.published })}
              title={c.published ? t`Unpublish (${publishKey})` : t`Publish (${publishKey})`}
              data-testid="changelog-publish"
            >
              {c.published ? <Trans>Unpublish</Trans> : <Trans>Publish</Trans>}
            </FrameButton>
          )}
        </>
      }
      notice={<ErrorLine error={patch.error} />}
      tabs={<FrameTabs label={t`Changelog`} items={tabs} current={tab} />}
      bodyClassName={tab === "entries" ? "flex overflow-hidden" : undefined}
    >
      {tab === "entries" && <EntriesTab changelog={c} />}
      {tab === "settings" && canManage && (
        <div className={cn(framePad, "max-w-[1040px]")}>
          <SettingsTab changelog={c} />
        </div>
      )}
      {preview && <PreviewDialog changelog={c} onClose={() => setPreview(false)} />}
    </PageFrame>
  )
}

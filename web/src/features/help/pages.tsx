import "@/features/help/i18n"
import { Trans, useLingui } from "@lingui/react/macro"
import { ArrowUpRightIcon, ChevronDownIcon, EllipsisIcon, LifeBuoyIcon, LoaderIcon, SearchIcon } from "lucide-react"
import { useRef, useState } from "react"
import { Navigate, useNavigate, useParams, useSearchParams } from "react-router"

import { PaneEmpty } from "@/components/common/Column"
import { HELP_KEYS, keyLabel } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { ArticlesTab, NewArticleDialog } from "@/features/help/ArticlesTab"
import { CollectionsTab } from "@/features/help/CollectionsTab"
import { CreateCenterDialog } from "@/features/help/CreateCenterDialog"
import {
  centerLink,
  useHelpCenter,
  useHelpCenters,
  useHelpCollections,
  useHelpPreview,
  useUpdateHelpCenter,
} from "@/features/help/queries"
import { ReportTab } from "@/features/help/ReportTab"
import { SettingsTab } from "@/features/help/SettingsTab"
import { ErrorLine } from "@/features/help/text"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { FrameTabs, look, PageTop } from "@/features/inbox/PageFrame"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useIsPhone } from "@/hooks/use-media-query"
import type { HelpCenter, Inbox } from "@/lib/api"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

export { ArticleEditorPage } from "@/features/help/ArticleEditor"

const TABS = ["articles", "drafts", "collections", "report", "settings"] as const
type Tab = (typeof TABS)[number]

function useCenterChoice() {
  const [inboxId] = useInboxFilter()
  const inboxes = useInboxes()
  const centers = useHelpCenters()
  const shown = (inboxes.data ?? []).filter((i) => !inboxId || i.id === inboxId)
  const groups = shown.map((inbox) => ({ inbox, center: centers.data?.find((c) => c.inbox_id === inbox.id) }))
  return { groups, pending: inboxes.isPending || centers.isPending, error: centers.error }
}

function CreateList({ groups }: { groups: { inbox: Inbox; center?: HelpCenter }[] }) {
  const navigate = useNavigate()
  const { canManage } = useSession()
  const [creating, setCreating] = useState<Inbox | null>(null)
  const missing = groups.filter((g) => !g.center)
  return (
    <div className={look.card} data-testid="help-panel">
      {missing.length === 0 ? (
        <p className="px-4 py-8 text-center text-faint" data-testid="help-empty">
          <Trans>No inbox to show a help center for.</Trans>
        </p>
      ) : (
        <ul className="flex flex-col">
          {missing.map(({ inbox }) => (
            <li key={inbox.id} className="flex items-center gap-3 border-t px-4 py-3 first:border-t-0">
              <span className="min-w-0 flex-1 truncate font-medium">{inbox.name}</span>
              {canManage ? (
                <Button
                  variant="outline"
                  className={look.btnSm}
                  onClick={() => setCreating(inbox)}
                  data-testid="create-help-center-open"
                >
                  <Trans>Create help center</Trans>
                </Button>
              ) : (
                <span className="text-caption text-faint" data-testid="help-none">
                  <Trans>No help center yet. Owners and admins create it.</Trans>
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
      {creating && (
        <CreateCenterDialog
          inboxId={creating.id}
          inboxName={creating.name}
          onClose={() => setCreating(null)}
          onCreated={(c) => {
            setCreating(null)
            navigate(centerLink(c.id, "settings"))
          }}
        />
      )}
    </div>
  )
}

export function HelpHome() {
  const phone = useIsPhone()
  const { groups, pending, error } = useCenterChoice()
  if (phone) {
    return (
      <PaneEmpty icon={LifeBuoyIcon} title={<Trans>Help centers</Trans>} testId="help-home">
        <Trans>
          Answer common questions once, in writing. Pick an inbox’s help center to write articles, group them in
          collections and see what customers search for.
        </Trans>
      </PaneEmpty>
    )
  }
  const first = groups.find((g) => g.center)?.center
  if (first) return <Navigate to={centerLink(first.id)} replace />
  return (
    <main className="flex min-h-0 flex-1 flex-col" data-testid="help-home">
      <PageTop title={<Trans>Help center</Trans>} sub={<Trans>Articles people read before they write to you</Trans>} />
      <div className="min-h-0 flex-1 overflow-y-auto px-6 py-5">
        <div className="flex max-w-[640px] flex-col gap-3">
          <p className="text-muted-foreground">
            <Trans>
              Answer common questions once, in writing. Each product gets its own help center with articles, collections
              and a report of what people search for.
            </Trans>
          </p>
          {pending ? (
            <Skeleton className="h-24 w-full rounded-xl" />
          ) : error ? (
            <ErrorLine error={error} />
          ) : (
            <CreateList groups={groups} />
          )}
        </div>
      </div>
    </main>
  )
}

function CenterPicker({ center }: { center: HelpCenter }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { canManage } = useSession()
  const { groups } = useCenterChoice()
  const [creating, setCreating] = useState<Inbox | null>(null)
  const others = groups.filter((g) => g.center?.id !== center.id)
  if (others.length === 0) return null
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<button type="button" className={look.chip} />}
          aria-label={t`Help center`}
          data-testid="help-center-picker"
        >
          <span className="max-w-36 truncate">{center.name}</span>
          <ChevronDownIcon className="size-3.5 text-faint" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-52">
          <DropdownMenuRadioGroup value={center.id} onValueChange={(v) => navigate(centerLink(String(v)))}>
            {groups.flatMap(({ center: c }) =>
              c
                ? [
                    <DropdownMenuRadioItem key={c.id} value={c.id} data-testid="help-row">
                      <span className="truncate">{c.name}</span>
                    </DropdownMenuRadioItem>,
                  ]
                : []
            )}
          </DropdownMenuRadioGroup>
          {canManage && groups.some((g) => !g.center) && (
            <>
              <DropdownMenuSeparator />
              {groups
                .filter((g) => !g.center)
                .map(({ inbox }) => (
                  <DropdownMenuItem
                    key={inbox.id}
                    onClick={() => setCreating(inbox)}
                    data-testid="create-help-center-open"
                  >
                    <Trans>Create one for {inbox.name}</Trans>
                  </DropdownMenuItem>
                ))}
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      {creating && (
        <CreateCenterDialog
          inboxId={creating.id}
          inboxName={creating.name}
          onClose={() => setCreating(null)}
          onCreated={(c) => {
            setCreating(null)
            navigate(centerLink(c.id, "settings"))
          }}
        />
      )}
    </>
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
  const [inboxFilter] = useInboxFilter()
  const centers = useHelpCenters().data
  const collections = useHelpCollections(centerId).data ?? []
  const [preview, setPreview] = useState(false)
  const [q, setQ] = useState("")
  const [creating, setCreating] = useState<{ title: string; collectionId: string } | null>(null)
  const search = useRef<HTMLInputElement>(null)
  const raw = params.get("tab")
  const tab: Tab = TABS.includes(raw as Tab) && (raw !== "settings" || canManage) ? (raw as Tab) : "articles"
  const setTab = (v: Tab) => navigate(centerLink(centerId, v === "articles" ? undefined : v), { replace: true })
  const c = center.data
  const listing = tab === "articles" || tab === "drafts"

  useHotkeys(
    {
      [HELP_KEYS.articles]: () => setTab("articles"),
      [HELP_KEYS.collections]: () => setTab("collections"),
      [HELP_KEYS.report]: () => setTab("report"),
      ...(canManage ? { [HELP_KEYS.settings]: () => setTab("settings") } : {}),
      [HELP_KEYS.preview]: () => setPreview(true),
      ...(listing
        ? {
            [HELP_KEYS.newArticle]: () => setCreating({ title: "", collectionId: "" }),
            [HELP_KEYS.search]: () => search.current?.focus(),
          }
        : {}),
      ...(c?.published ? { [HELP_KEYS.openPage]: () => window.open(c.url, "_blank", "noopener,noreferrer") } : {}),
      ...(canManage && c ? { [HELP_KEYS.publishCenter]: () => patch.mutate({ published: !c.published }) } : {}),
    },
    !!c
  )

  const switched = inboxFilter && c && c.inbox_id !== inboxFilter && centers?.find((x) => x.inbox_id === inboxFilter)
  if (switched) return <Navigate to={centerLink(switched.id)} replace />

  if (center.isPending) {
    return (
      <main className="flex flex-col gap-4 px-6 py-5">
        <Skeleton className="h-8 w-60" />
        <Skeleton className="h-24 w-full rounded-xl" />
      </main>
    )
  }
  if (!c) {
    return (
      <main className="px-6 py-5">
        <ErrorLine error={center.error} />
      </main>
    )
  }
  const names: Record<Tab, string> = {
    articles: t`Articles`,
    drafts: t`Drafts`,
    collections: t`Collections`,
    report: t`Report`,
    settings: t`Settings`,
  }
  const keys: Partial<Record<Tab, string>> = {
    articles: HELP_KEYS.articles,
    collections: HELP_KEYS.collections,
    report: HELP_KEYS.report,
    settings: HELP_KEYS.settings,
  }
  const previewKey = keyLabel(HELP_KEYS.preview).join("")
  const publishKey = keyLabel(HELP_KEYS.publishCenter).join("")
  const newKey = keyLabel(HELP_KEYS.newArticle).join("")
  const findKey = keyLabel(HELP_KEYS.search).join("")
  const openKey = keyLabel(HELP_KEYS.openPage).join("")
  return (
    <main className="flex min-h-0 flex-1 flex-col" data-testid="help-center" key={c.id}>
      <PageTop
        title={<Trans>Help center</Trans>}
        sub={
          <span data-testid="help-published">
            <span data-testid="help-center-name">{c.name}</span>
            {inbox && inbox.name !== c.name ? ` · ${inbox.name}` : ""}
            {" · "}
            {c.published ? <Trans>Published</Trans> : <Trans>Unpublished, only your team can preview it</Trans>}
          </span>
        }
        actions={
          <>
            <DropdownMenu>
              <DropdownMenuTrigger
                render={<Button variant="ghost" size="icon-sm" className="text-faint" />}
                aria-label={t`More`}
                data-testid="help-more"
              >
                <EllipsisIcon />
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="min-w-52">
                <DropdownMenuItem onClick={() => setPreview(true)} data-testid="help-preview">
                  <Trans>Preview</Trans>
                  <span className="ms-auto text-caption text-faint">{previewKey}</span>
                </DropdownMenuItem>
                {canManage && c.published && (
                  <DropdownMenuItem
                    onClick={() => patch.mutate({ published: false })}
                    disabled={patch.isPending}
                    data-testid="help-publish"
                  >
                    <Trans>Unpublish</Trans>
                    <span className="ms-auto text-caption text-faint">{publishKey}</span>
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
            {c.published ? (
              <Button
                variant="outline"
                className={look.btn}
                render={<a href={c.url} target="_blank" rel="noreferrer noopener" />}
                title={t`View site (${openKey})`}
                data-testid="help-open-public"
              >
                <Trans>View site</Trans>
                <ArrowUpRightIcon className="text-faint" />
              </Button>
            ) : (
              canManage && (
                <Button
                  variant="outline"
                  className={look.btn}
                  disabled={patch.isPending}
                  onClick={() => patch.mutate({ published: true })}
                  title={t`Publish (${publishKey})`}
                  data-testid="help-publish"
                >
                  <Trans>Publish</Trans>
                </Button>
              )
            )}
            <Button
              className={look.btnInk}
              onClick={() => setCreating({ title: "", collectionId: "" })}
              title={t`New article (${newKey})`}
              data-testid="new-article"
            >
              <Trans>New article</Trans>
            </Button>
          </>
        }
        tabs={
          <FrameTabs<Tab>
            label={t`Help center`}
            value={tab}
            onChange={setTab}
            items={TABS.filter((v) => v !== "settings" || canManage).map((v) => ({
              key: v,
              label: names[v],
              title: keys[v] ? `${names[v]} (${keyLabel(keys[v]).join("")})` : undefined,
              testId: `help-tab-${v}`,
            }))}
          />
        }
        aside={
          <>
            <CenterPicker center={c} />
            {listing && (
              <label className="relative flex h-[30px] w-56 items-center">
                <SearchIcon className="pointer-events-none absolute start-2.5 size-3.5 text-faint" aria-hidden />
                <input
                  ref={search}
                  value={q}
                  onChange={(e) => setQ(e.target.value)}
                  onKeyDown={(e) => e.key === "Escape" && e.currentTarget.blur()}
                  placeholder={t`Search titles (${findKey})`}
                  aria-label={t`Search titles`}
                  className="size-full rounded-lg border border-input bg-background ps-8 pe-2 text-body outline-none placeholder:text-faint focus-visible:border-faint"
                  data-testid="help-article-search"
                />
              </label>
            )}
          </>
        }
      />
      <ErrorLine error={patch.error} className="px-6 pt-2" />
      <div className="min-h-0 flex-1 overflow-y-auto px-6 py-5 phone:px-4">
        {listing && (
          <ArticlesTab
            key={tab}
            center={c}
            drafts={tab === "drafts"}
            q={q}
            onWrite={(title, collectionId) => setCreating({ title, collectionId })}
          />
        )}
        {tab === "collections" && <CollectionsTab center={c} />}
        {tab === "report" && <ReportTab center={c} />}
        {tab === "settings" && canManage && <SettingsTab center={c} />}
      </div>
      {preview && <PreviewDialog center={c} onClose={() => setPreview(false)} />}
      {creating && (
        <NewArticleDialog
          center={c}
          collections={collections}
          collectionId={creating.collectionId}
          initialTitle={creating.title}
          onClose={() => setCreating(null)}
        />
      )}
    </main>
  )
}

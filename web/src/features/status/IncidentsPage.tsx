import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { cloneElement, useState } from "react"
import { Link, useNavigate, useParams, useSearchParams } from "react-router"

import { ErrorLine, InboxMark } from "@/components/common"
import { keyLabel, STATUS_KEYS } from "@/components/common/ShortcutSheet"
import { formatDuration, formatShort } from "@/components/common/text"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { IncidentDialog } from "@/features/status/IncidentDialog"
import { FrameButton, FrameTabs, framePad, PageFrame, Pill, ProductTag } from "@/features/status/PageFrame"
import { incidentLink, usePagesComponents, usePagesIncidents, useStatusPages } from "@/features/status/queries"
import { isClosed, useStatusText } from "@/features/status/text"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useRowFocus } from "@/hooks/use-row-focus"
import type { Inbox, StatusIncident, StatusIncidentKind, StatusPage } from "@/lib/api"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

export const VIEWS = ["active", "scheduled", "past"] as const
export type View = (typeof VIEWS)[number]

export function viewOf(i: StatusIncident): View {
  if (isClosed(i)) return "past"
  return i.state === "scheduled" ? "scheduled" : "active"
}

export function viewLink(pageId: string, view: View) {
  return `/status-pages/${pageId}${view === "active" ? "" : `?view=${view}`}`
}

export function useStatusScope() {
  const [inboxId] = useInboxFilter()
  const pages = useStatusPages(inboxId)
  const inboxes = useInboxes().data ?? []
  const byInbox = new Map(inboxes.map((i) => [i.id, i]))
  const items = pages.data ?? []
  return { pages, items, inboxOf: (p?: StatusPage) => (p ? byInbox.get(p.inbox_id) : undefined) }
}

export function useScopeIncidents(items: StatusPage[]) {
  const ids = items.map((p) => p.id)
  const open = usePagesIncidents(ids, true)
  const all = usePagesIncidents(ids, false)
  const seen = new Set<string>()
  const merged = [...open.data, ...all.data].filter((i) => !seen.has(i.id) && !!seen.add(i.id))
  return { data: merged, isPending: open.isPending || all.isPending, error: open.error ?? all.error }
}

export function PagePicker({
  pages,
  inboxOf,
  onPick,
  button,
  children,
}: {
  pages: StatusPage[]
  inboxOf: (p?: StatusPage) => Inbox | undefined
  onPick: (p: StatusPage) => void
  button: React.ReactElement<{ onClick?: () => void }>
  children: React.ReactNode
}) {
  if (pages.length === 1) return cloneElement(button, { onClick: () => onPick(pages[0]) }, children)
  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={button}>{children}</DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-52">
        {pages.map((p) => {
          const inbox = inboxOf(p)
          return (
            <DropdownMenuItem key={p.id} onClick={() => onPick(p)}>
              <InboxMark name={inbox?.name ?? p.name} color={inbox?.branding.color} className="size-4 text-[9px]" />
              {p.name}
            </DropdownMenuItem>
          )
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function IncidentsFrame({
  view,
  pageId,
  counts,
  children,
  back,
}: {
  view: View
  pageId: string
  counts: { active?: number; scheduled?: number }
  children: React.ReactNode
  back?: { to: string; label: string }
}) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { items, inboxOf } = useStatusScope()
  const [creating, setCreating] = useState<{ pageId: string; kind: StatusIncidentKind } | null>(null)
  const fallback = items.find((p) => p.id === pageId) ?? items[0]
  const incidentKey = keyLabel(STATUS_KEYS.incident).join("")
  const maintenanceKey = keyLabel(STATUS_KEYS.maintenance).join("")
  useHotkeys(
    fallback
      ? {
          [STATUS_KEYS.incident]: () => setCreating({ pageId: fallback.id, kind: "incident" }),
          [STATUS_KEYS.maintenance]: () => setCreating({ pageId: fallback.id, kind: "maintenance" }),
        }
      : {},
    !creating
  )
  const tabs = [
    { key: "active", label: t`Active`, count: counts.active },
    { key: "scheduled", label: t`Scheduled`, count: counts.scheduled },
    { key: "past", label: t`Past` },
  ].map((x) => ({ ...x, to: viewLink(pageId, x.key as View), testId: `status-incidents-${x.key}` }))
  return (
    <PageFrame
      testId="status-incidents"
      back={back}
      title={<Trans>Incidents</Trans>}
      description={<Trans>What went wrong and what you told people</Trans>}
      actions={
        items.length > 0 && (
          <>
            <PagePicker
              pages={items}
              inboxOf={inboxOf}
              onPick={(p) => setCreating({ pageId: p.id, kind: "maintenance" })}
              button={
                <FrameButton title={t`Plan maintenance (${maintenanceKey})`} data-testid="status-new-maintenance" />
              }
            >
              <Trans>Schedule maintenance</Trans>
            </PagePicker>
            <PagePicker
              pages={items}
              inboxOf={inboxOf}
              onPick={(p) => setCreating({ pageId: p.id, kind: "incident" })}
              button={<FrameButton ink title={t`New incident (${incidentKey})`} data-testid="status-new-incident" />}
            >
              <Trans>New incident</Trans>
            </PagePicker>
          </>
        )
      }
      tabs={<FrameTabs label={t`Incidents`} items={tabs} current={view} />}
    >
      {children}
      {creating && (
        <IncidentDialog
          pageId={creating.pageId}
          kind={creating.kind}
          onClose={() => setCreating(null)}
          onDone={(i) => {
            setCreating(null)
            navigate(incidentLink(creating.pageId, i.id))
          }}
        />
      )}
    </PageFrame>
  )
}

function useMeta() {
  const { i18n } = useLingui()
  const text = useStatusText()
  return (i: StatusIncident, componentName: (id: string) => string | undefined) => {
    const parts: string[] = []
    if (i.kind === "maintenance" && i.scheduled_start && i.scheduled_end) {
      const time = new Intl.DateTimeFormat(i18n.locale, { timeStyle: "short" })
      parts.push(
        `${formatShort(i.scheduled_start, i18n.locale)} · ${time.format(new Date(i.scheduled_start))}–${time.format(new Date(i.scheduled_end))}`
      )
    } else {
      parts.push(formatShort(i.started_at ?? i.published_at ?? i.created_at, i18n.locale))
      if (i.resolved_at) {
        const from = Date.parse(i.started_at ?? i.published_at ?? i.created_at)
        parts.push(formatDuration(Math.max(60, Math.round((Date.parse(i.resolved_at) - from) / 1000)), i18n.locale))
      } else parts.push(text.incident[i.state])
    }
    const names = i.components.map((c) => componentName(c.component_id)).filter((n): n is string => !!n)
    if (names.length) parts.push(names.join(", "))
    return parts.join(" · ")
  }
}

function tone(i: StatusIncident) {
  if (isClosed(i)) return "success" as const
  if (i.state === "scheduled") return "info" as const
  return "warning" as const
}

const dotTone = { success: "bg-success", info: "bg-blue-600 dark:bg-blue-400", warning: "bg-warning" }

function IncidentRow({ i, page, inbox, meta }: { i: StatusIncident; page?: StatusPage; inbox?: Inbox; meta: string }) {
  const text = useStatusText()
  const updates = i.updates.length
  return (
    <li>
      <Link
        to={incidentLink(i.page_id, i.id)}
        className="flex items-center gap-3.5 rounded-xl border bg-card px-4 py-3.5 outline-none transition-colors hover:border-faint focus-visible:ring-2 focus-visible:ring-ring"
        data-testid="status-incident-row"
      >
        <span aria-hidden className={cn("size-2.5 shrink-0 rounded-full", dotTone[tone(i)])} />
        <span className="flex min-w-0 flex-1 flex-col leading-[1.35]">
          <span className="flex min-w-0 items-center gap-2">
            <span className="truncate font-semibold">{i.title}</span>
            {page && <ProductTag name={inbox?.name ?? page.name} color={inbox?.branding.color} />}
            {!i.published_at && (
              <Pill tone="muted" data-testid="status-incident-draft-badge">
                <Trans>Draft</Trans>
              </Pill>
            )}
          </span>
          <span className="truncate text-muted-foreground">
            {meta} · <Plural value={updates} one="# update" other="# updates" />
          </span>
        </span>
        <Pill tone={tone(i)} data-testid="status-incident-state">
          {text.incident[i.state]}
        </Pill>
      </Link>
    </li>
  )
}

export function IncidentList({ items }: { items: StatusIncident[] }) {
  const { items: pages, inboxOf } = useStatusScope()
  const byId = new Map(pages.map((p) => [p.id, p]))
  const components = usePagesComponents(pages.map((p) => p.id))
  const names = new Map(components.map((c) => [c.id, c.name]))
  const meta = useMeta()
  const list = useRowFocus<HTMLUListElement>()
  return (
    <div className={cn(framePad, "max-w-[1028px]")}>
      {items.length === 0 ? (
        <p className="py-12 text-center text-faint" data-testid="status-incidents-empty">
          <Trans>Nothing here.</Trans>
        </p>
      ) : (
        <ul ref={list} className="flex flex-col gap-4">
          {items.map((i) => {
            const page = byId.get(i.page_id)
            return (
              <IncidentRow key={i.id} i={i} page={page} inbox={inboxOf(page)} meta={meta(i, (id) => names.get(id))} />
            )
          })}
        </ul>
      )}
    </div>
  )
}

export function useIncidentCounts(all: StatusIncident[]) {
  return {
    active: all.filter((i) => viewOf(i) === "active").length,
    scheduled: all.filter((i) => viewOf(i) === "scheduled").length,
  }
}

export function IncidentsPage({ detail }: { detail: (i: StatusIncident) => React.ReactNode }) {
  const { pageId = "" } = useParams()
  const [params] = useSearchParams()
  const raw = params.get("view") as View | null
  const view: View = raw && VIEWS.includes(raw) ? raw : "active"
  const { items, pages } = useStatusScope()
  const incidents = useScopeIncidents(items)
  const shown = incidents.data
    .filter((i) => viewOf(i) === view)
    .sort((a, b) =>
      view === "scheduled"
        ? (a.scheduled_start ?? a.created_at).localeCompare(b.scheduled_start ?? b.created_at)
        : (b.started_at ?? b.created_at).localeCompare(a.started_at ?? a.created_at)
    )
  const loading = pages.isPending || (items.length > 0 && incidents.isPending)
  return (
    <IncidentsFrame view={view} pageId={pageId} counts={useIncidentCounts(incidents.data)}>
      <ErrorLine error={pages.error ?? incidents.error} className="m-6" />
      {loading ? (
        <div className={framePad}>
          <Skeleton className="h-16 w-full max-w-[980px] rounded-xl" />
          <Skeleton className="h-16 w-full max-w-[980px] rounded-xl" />
        </div>
      ) : view === "active" && shown.length === 1 ? (
        detail(shown[0])
      ) : (
        <IncidentList items={shown} />
      )}
    </IncidentsFrame>
  )
}

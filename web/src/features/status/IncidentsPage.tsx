import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { CalendarClockIcon, CheckCircle2Icon, FlameIcon } from "lucide-react"
import { useState } from "react"
import { Link, useNavigate, useParams, useSearchParams } from "react-router"

import { ErrorLine } from "@/components/common"
import { PaneBack } from "@/components/common/Column"
import { INCIDENT_KEYS, keyLabel } from "@/components/common/ShortcutSheet"
import { formatDuration, formatShort } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { FrameTabs, look, PageTop } from "@/features/inbox/PageFrame"
import { IncidentDetail, incidentTone } from "@/features/status/IncidentDetail"
import { IncidentDialog } from "@/features/status/IncidentDialog"
import { Pill, ProductTag } from "@/features/status/PageFrame"
import { incidentLink } from "@/features/status/queries"
import { useScopeIncidents, useStatusScope } from "@/features/status/scope"
import { isClosed, useStatusText } from "@/features/status/text"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useIsPhone } from "@/hooks/use-media-query"
import type { Inbox, StatusIncident, StatusIncidentKind, StatusPage } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"

export const VIEWS = ["active", "scheduled", "past"] as const
export type View = (typeof VIEWS)[number]

export function viewOf(i: StatusIncident): View {
  if (isClosed(i)) return "past"
  return i.state === "scheduled" ? "scheduled" : "active"
}

const dotTone = { success: "bg-success", info: "bg-blue-600 dark:bg-blue-400", warning: "bg-warning" }

function useMeta() {
  const { i18n } = useLingui()
  const text = useStatusText()
  return (i: StatusIncident) => {
    const parts: string[] = []
    if (i.kind === "maintenance" && i.scheduled_start && i.scheduled_end) {
      const time = new Intl.DateTimeFormat(i18n.locale, { timeStyle: "short" })
      parts.push(
        `${formatShort(i.scheduled_start, i18n.locale)} · ${time.format(new Date(i.scheduled_start))}–${time.format(new Date(i.scheduled_end))}`
      )
    } else {
      const from = Date.parse(i.started_at ?? i.published_at ?? i.created_at)
      parts.push(formatShort(new Date(from).toISOString(), i18n.locale))
      if (i.resolved_at)
        parts.push(formatDuration(Math.max(60, Math.round((Date.parse(i.resolved_at) - from) / 1000)), i18n.locale))
      else parts.push(text.incident[i.state])
    }
    return parts.join(" · ")
  }
}

function Row({
  i,
  page,
  inbox,
  meta,
  current,
  view,
}: {
  i: StatusIncident
  page?: StatusPage
  inbox?: Inbox
  meta: string
  current: boolean
  view: View
}) {
  const text = useStatusText()
  const updates = i.updates.length
  return (
    <li>
      <Link
        to={incidentLink(i.page_id, i.id, view)}
        replace
        aria-current={current ? "true" : undefined}
        className={cn(
          "flex flex-col gap-1 border-b px-4 py-3 outline-none transition-colors hover:bg-muted/60 focus-visible:bg-muted",
          current && "bg-muted"
        )}
        data-testid="status-incident-row"
      >
        <span className="flex min-w-0 items-center gap-2.5">
          <span aria-hidden className={cn("size-2 shrink-0 rounded-full", dotTone[incidentTone(i)])} />
          <span className="min-w-0 flex-1 truncate font-medium">{i.title}</span>
          {!i.published_at && (
            <Pill tone="muted" data-testid="status-incident-draft-badge">
              <Trans>Draft</Trans>
            </Pill>
          )}
          <Pill tone={incidentTone(i)}>{text.incident[i.state]}</Pill>
        </span>
        <span className="flex min-w-0 items-center gap-2 ps-[18px] text-caption text-muted-foreground">
          {page && <ProductTag name={inbox?.name ?? page.name} color={inbox?.branding.color} />}
          <span className="min-w-0 truncate">
            {meta} · <Plural value={updates} one="# update" other="# updates" />
          </span>
        </span>
      </Link>
    </li>
  )
}

export function IncidentsPage() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { incidentId = "" } = useParams()
  const [params] = useSearchParams()
  const phone = useIsPhone()
  const { canManage } = useSession()
  const { items: pages, pages: pagesQuery, inboxOf } = useStatusScope()
  const incidents = useScopeIncidents(pages)
  const meta = useMeta()
  const [creating, setCreating] = useState<{ kind: StatusIncidentKind } | null>(null)
  const [kept, setKept] = useState<{ id: string; view: View } | null>(null)
  const byPage = new Map(pages.map((p) => [p.id, p]))
  const all = incidents.data.filter((i) => byPage.has(i.page_id))
  const selected = all.find((i) => i.id === incidentId)
  const raw = params.get("view") as View | null
  const view: View =
    raw && VIEWS.includes(raw)
      ? raw
      : kept && kept.id === incidentId
        ? kept.view
        : selected
          ? viewOf(selected)
          : "active"
  const counts = { active: 0, scheduled: 0 }
  for (const i of all) {
    const v = viewOf(i)
    if (v !== "past") counts[v]++
  }
  const rows = all
    .filter((i) => viewOf(i) === view || (kept?.view === view && kept.id === i.id))
    .sort((a, b) =>
      view === "scheduled"
        ? (a.scheduled_start ?? a.created_at).localeCompare(b.scheduled_start ?? b.created_at)
        : (b.started_at ?? b.published_at ?? b.created_at).localeCompare(a.started_at ?? a.published_at ?? a.created_at)
    )
  const shown = selected ?? (phone ? undefined : rows[0])
  const shownPage = shown && byPage.get(shown.page_id)
  const loading = pagesQuery.isPending || (pages.length > 0 && incidents.isPending)
  const incidentKey = keyLabel(INCIDENT_KEYS.incident).join("")
  const maintenanceKey = keyLabel(INCIDENT_KEYS.maintenance).join("")
  const canCreate = pages.length > 0
  const to = (v: View) => (v === "active" ? "/incidents" : `/incidents?view=${v}`)

  const step = (by: number) => {
    const at = shown ? rows.findIndex((i) => i.id === shown.id) : -1
    const next = rows[Math.min(rows.length - 1, Math.max(0, at + by))]
    if (next) navigate(incidentLink(next.page_id, next.id, view), { replace: true })
  }
  useHotkeys(
    {
      j: () => step(1),
      k: () => step(-1),
      ArrowDown: () => step(1),
      ArrowUp: () => step(-1),
      [INCIDENT_KEYS.active]: () => navigate(to("active")),
      [INCIDENT_KEYS.scheduled]: () => navigate(to("scheduled")),
      [INCIDENT_KEYS.past]: () => navigate(to("past")),
      ...(canCreate
        ? {
            [INCIDENT_KEYS.incident]: () => setCreating({ kind: "incident" }),
            [INCIDENT_KEYS.maintenance]: () => setCreating({ kind: "maintenance" }),
          }
        : {}),
    },
    !creating
  )

  const tabs = [
    { key: "active" as const, label: t`Active`, count: counts.active },
    { key: "scheduled" as const, label: t`Scheduled`, count: counts.scheduled },
    { key: "past" as const, label: t`Past`, count: undefined },
  ].map((x) => ({ ...x, to: to(x.key), testId: `status-incidents-${x.key}` }))

  const emptyText = {
    active: t`Nothing is wrong right now. Open an incident when something breaks.`,
    scheduled: t`No maintenance is planned.`,
    past: t`Nothing has been resolved yet.`,
  }[view]

  const detail = loading ? (
    <div className="flex flex-col gap-4 p-6">
      <Skeleton className="h-8 w-60" />
      <Skeleton className="h-24 w-full rounded-xl" />
    </div>
  ) : shown && shownPage ? (
    <IncidentDetail
      key={shown.id}
      page={shownPage}
      i={shown}
      justResolved={kept?.id === shown.id}
      onClosed={(id) => setKept({ id, view })}
      onDeleted={() => navigate("/incidents", { replace: true })}
    />
  ) : rows.length > 0 ? (
    <p className="p-10 text-center text-faint">
      <Trans>Pick an incident to read it.</Trans>
    </p>
  ) : null

  return (
    <main className="flex min-h-0 min-w-0 flex-1 flex-col bg-background" data-testid="status-incidents">
      {phone && incidentId ? (
        <div className="px-4 pt-3">
          <PaneBack to={to(view)} label={t`Incidents`} always />
        </div>
      ) : (
        <PageTop
          title={<Trans>Incidents</Trans>}
          sub={<Trans>What went wrong and what you told people</Trans>}
          actions={
            canCreate && (
              <>
                <Button
                  variant="outline"
                  className={look.btn}
                  onClick={() => setCreating({ kind: "maintenance" })}
                  title={t`Plan maintenance (${maintenanceKey})`}
                  data-testid="status-new-maintenance"
                >
                  <Trans>Plan maintenance</Trans>
                </Button>
                <Button
                  className={look.btnInk}
                  onClick={() => setCreating({ kind: "incident" })}
                  title={t`New incident (${incidentKey})`}
                  data-testid="status-new-incident"
                >
                  <Trans>New incident</Trans>
                </Button>
              </>
            )
          }
          tabs={<FrameTabs label={t`Incidents`} value={view} items={tabs} />}
        />
      )}
      <ErrorLine error={pagesQuery.error ?? incidents.error} className="m-6" />
      <div className="flex min-h-0 flex-1 overflow-hidden">
        <div
          className={cn(
            "flex w-[340px] shrink-0 flex-col overflow-y-auto border-e phone:w-full phone:border-e-0",
            phone && incidentId && "hidden"
          )}
        >
          {loading ? (
            <div className="flex flex-col gap-2 p-4">
              <Skeleton className="h-14 w-full rounded-xl" />
              <Skeleton className="h-14 w-full rounded-xl" />
            </div>
          ) : pages.length === 0 ? (
            <div className="m-4 flex flex-col items-center gap-2 rounded-xl border border-dashed px-6 py-10 text-center">
              <p className="text-body text-muted-foreground">
                <Trans>Incidents live on a status page. Create one first.</Trans>
              </p>
              {canManage && (
                <Button variant="outline" className={look.btnSm} render={<Link to="/status" />}>
                  <Trans>Status pages</Trans>
                </Button>
              )}
            </div>
          ) : rows.length === 0 ? (
            <div
              className="m-4 flex flex-col items-center gap-3 rounded-xl border border-dashed px-6 py-10 text-center"
              data-testid="status-incidents-empty"
            >
              {view === "scheduled" ? (
                <CalendarClockIcon className="size-5 text-faint" />
              ) : view === "past" ? (
                <CheckCircle2Icon className="size-5 text-faint" />
              ) : (
                <FlameIcon className="size-5 text-faint" />
              )}
              <p className="text-body text-muted-foreground">{emptyText}</p>
              {view !== "past" && (
                <Button
                  variant="outline"
                  className={look.btnSm}
                  onClick={() => setCreating({ kind: view === "scheduled" ? "maintenance" : "incident" })}
                >
                  {view === "scheduled" ? <Trans>Plan maintenance</Trans> : <Trans>New incident</Trans>}
                </Button>
              )}
            </div>
          ) : (
            <ul aria-label={t`Incidents`} data-testid="status-incident-list">
              {rows.map((i) => {
                const page = byPage.get(i.page_id)
                return (
                  <Row
                    key={i.id}
                    i={i}
                    page={page}
                    inbox={inboxOf(page)}
                    meta={meta(i)}
                    current={i.id === shown?.id}
                    view={view}
                  />
                )
              })}
            </ul>
          )}
        </div>
        <div className={cn("@container min-w-0 flex-1 overflow-y-auto", phone && !incidentId && "hidden")}>
          {detail}
        </div>
      </div>
      {creating && (
        <IncidentDialog
          pages={pages}
          pageId={shown?.page_id}
          kind={creating.kind}
          onClose={() => setCreating(null)}
          onDone={(i) => {
            setCreating(null)
            navigate(incidentLink(i.page_id, i.id, viewOf(i)))
          }}
        />
      )}
    </main>
  )
}

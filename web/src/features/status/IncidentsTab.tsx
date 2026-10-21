import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { ArrowRightIcon, CalendarClockIcon, FlameIcon, PlusIcon } from "lucide-react"
import { useState } from "react"
import { Link, useNavigate } from "react-router"

import { ErrorLine, Segmented } from "@/components/common"
import { formatDateTime, formatRelative } from "@/components/common/text"
import { keyLabel, STATUS_KEYS } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Card, EmptyRow, Rows, StatTile } from "@/features/settings/ui"
import { IncidentDialog } from "@/features/status/IncidentDialog"
import { incidentLink, useStatusComponents, useStatusIncidents, useStatusMonitors } from "@/features/status/queries"
import { isClosed, useStatusText } from "@/features/status/text"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { StatusIncident, StatusIncidentKind, StatusPage } from "@/lib/api"
import { cn } from "@/lib/utils"

export function IncidentBadges({ i }: { i: StatusIncident }) {
  const text = useStatusText()
  return (
    <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-caption text-muted-foreground">
      <span className="inline-flex items-center gap-1">
        {i.kind === "maintenance" ? <CalendarClockIcon className="size-3.5" /> : <FlameIcon className="size-3.5" />}
        {text.kind[i.kind]}
      </span>
      <span aria-hidden>·</span>
      <span
        className={isClosed(i) ? "text-success" : i.state === "scheduled" ? "text-brand" : "text-warning"}
        data-testid="status-incident-state"
      >
        {text.incident[i.state]}
      </span>
      {i.kind === "incident" && i.impact !== "none" && (
        <>
          <span aria-hidden>·</span>
          <span>{text.impact[i.impact]}</span>
        </>
      )}
      {!i.published_at && (
        <span
          className="rounded-md bg-muted px-1.5 py-0.5 font-medium text-foreground"
          data-testid="status-incident-draft-badge"
        >
          <Trans>Draft</Trans>
        </span>
      )}
    </span>
  )
}

function useWhen() {
  const { i18n } = useLingui()
  return (i: StatusIncident) =>
    i.kind === "maintenance" && i.scheduled_start && i.scheduled_end
      ? `${formatDateTime(i.scheduled_start, i18n.locale)} – ${formatDateTime(i.scheduled_end, i18n.locale)}`
      : formatRelative(i.resolved_at ?? i.created_at, i18n.locale)
}

function stateDot(i: StatusIncident) {
  if (isClosed(i)) return "bg-success"
  if (i.state === "scheduled") return "bg-brand"
  return "bg-warning"
}

function Row({ page, i }: { page: StatusPage; i: StatusIncident }) {
  const when = useWhen()
  return (
    <li>
      <Link
        to={incidentLink(page.id, i.id)}
        className="flex items-center gap-3.5 px-4 py-3 outline-none transition-colors hover:bg-muted focus-visible:bg-muted"
        data-testid="status-incident-row"
      >
        <span aria-hidden className={cn("size-2.5 shrink-0 rounded-full", stateDot(i))} />
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5">
            <span className="min-w-0 flex-1 basis-48 truncate text-body font-medium">{i.title}</span>
            <span className="shrink-0 text-caption text-faint">{when(i)}</span>
          </span>
          <IncidentBadges i={i} />
        </span>
      </Link>
    </li>
  )
}

function ActiveCard({ page, i }: { page: StatusPage; i: StatusIncident }) {
  const { i18n } = useLingui()
  const text = useStatusText()
  const started = formatRelative(i.started_at ?? i.published_at ?? i.created_at, i18n.locale)
  const updates = i.updates.length
  return (
    <li>
      <Link
        to={incidentLink(page.id, i.id)}
        className="flex items-center gap-3 rounded-2xl border border-warning/50 bg-warning/8 px-4 py-3.5 outline-none transition-colors hover:border-warning focus-visible:ring-2 focus-visible:ring-ring"
        data-testid="status-incident-row"
      >
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="truncate text-body font-semibold">{i.title}</span>
          <span className="truncate text-small text-muted-foreground">
            {text.incident[i.state]} · <Trans>since {started}</Trans> ·{" "}
            <Plural value={updates} one="# update" other="# updates" />
            {!i.published_at && (
              <>
                {" · "}
                <Trans>not public yet</Trans>
              </>
            )}
          </span>
        </span>
        <span className="inline-flex shrink-0 items-center gap-1 text-small font-medium text-warning">
          <Trans>Post update</Trans>
          <ArrowRightIcon className="size-3.5 rtl:rotate-180" />
        </span>
      </Link>
    </li>
  )
}

function Facts({ page }: { page: StatusPage }) {
  const { i18n } = useLingui()
  const fmt = new Intl.NumberFormat(i18n.locale)
  const components = useStatusComponents(page.id).data
  const monitors = useStatusMonitors(page.id).data
  const subs = page.email_subscribers
  const notOk = (components ?? []).filter((c) => c.state !== "operational").length
  const down = (monitors ?? []).filter((m) => m.state === "down" && !m.paused).length
  const up = (monitors ?? []).filter((m) => m.state === "up" && !m.paused).length
  return (
    <div className="grid grid-cols-1 gap-2 @xl:grid-cols-3" data-testid="status-facts">
      <StatTile
        label={<Trans>Subscribers</Trans>}
        value={subs.enabled ? fmt.format(subs.counts.subscribed) : "–"}
        hint={subs.enabled ? <Trans>by e-mail</Trans> : <Trans>E-mail subscriptions are off</Trans>}
      />
      <StatTile
        label={<Trans>Components</Trans>}
        value={components ? fmt.format(components.length) : "–"}
        hint={
          components &&
          (notOk > 0 ? (
            <Plural value={notOk} one="# not operational" other="# not operational" />
          ) : (
            <Trans>all operational</Trans>
          ))
        }
      />
      <StatTile
        label={<Trans>Monitors</Trans>}
        value={monitors ? fmt.format(monitors.length) : "–"}
        hint={
          monitors &&
          monitors.length > 0 &&
          (down > 0 ? (
            <Plural value={down} one="# down" other="# down" />
          ) : up === monitors.length ? (
            <Trans>all up</Trans>
          ) : (
            <Plural value={up} one="# up" other="# up" />
          ))
        }
      />
    </div>
  )
}

export function IncidentsTab({ page }: { page: StatusPage }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const [open, setOpen] = useState(true)
  const [creating, setCreating] = useState<StatusIncidentKind | null>(null)
  const list = useStatusIncidents(page.id, open)
  const all = list.data?.pages.flatMap((p) => p.items) ?? []
  const items = open ? all : all.filter(isClosed)
  const active = open ? items.filter((i) => i.kind === "incident") : []
  const rest = open ? items.filter((i) => i.kind !== "incident") : items
  const incidentKey = keyLabel(STATUS_KEYS.incident).join("")
  const maintenanceKey = keyLabel(STATUS_KEYS.maintenance).join("")

  useHotkeys({
    [STATUS_KEYS.incident]: () => setCreating("incident"),
    [STATUS_KEYS.maintenance]: () => setCreating("maintenance"),
  })

  return (
    <div className="@container flex flex-col gap-4" data-testid="status-incidents">
      <Facts page={page} />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Segmented
          value={open ? "open" : "past"}
          onChange={(v) => setOpen(v === "open")}
          label={t`Incidents`}
          items={[
            { value: "open", label: t`Open`, testId: "status-incidents-open" },
            { value: "past", label: t`Past`, testId: "status-incidents-past" },
          ]}
        />
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setCreating("maintenance")}
            title={t`Plan maintenance (${maintenanceKey})`}
            data-testid="status-new-maintenance"
          >
            <CalendarClockIcon />
            <Trans>Plan maintenance</Trans>
          </Button>
          <Button
            size="sm"
            onClick={() => setCreating("incident")}
            title={t`New incident (${incidentKey})`}
            data-testid="status-new-incident"
          >
            <PlusIcon />
            <Trans>New incident</Trans>
          </Button>
        </div>
      </div>
      <ErrorLine error={list.error} />
      {list.isPending ? (
        <Skeleton className="h-32 w-full rounded-2xl" />
      ) : items.length === 0 ? (
        <Card flush>
          <EmptyRow>
            {open ? <Trans>No open incidents or maintenance.</Trans> : <Trans>Nothing is resolved yet.</Trans>}
          </EmptyRow>
        </Card>
      ) : (
        <>
          {active.length > 0 && (
            <ul className="flex flex-col gap-2" data-testid="status-incidents-active">
              {active.map((i) => (
                <ActiveCard key={i.id} page={page} i={i} />
              ))}
            </ul>
          )}
          {rest.length > 0 && (
            <Card flush>
              <Rows>
                {rest.map((i) => (
                  <Row key={i.id} page={page} i={i} />
                ))}
              </Rows>
            </Card>
          )}
        </>
      )}
      {list.hasNextPage && (
        <div className="flex justify-center">
          <Button
            variant="ghost"
            size="sm"
            disabled={list.isFetchingNextPage}
            onClick={() => void list.fetchNextPage()}
          >
            <Trans>Load more</Trans>
          </Button>
        </div>
      )}
      {creating && (
        <IncidentDialog
          pageId={page.id}
          kind={creating}
          onClose={() => setCreating(null)}
          onDone={(i) => {
            setCreating(null)
            navigate(incidentLink(page.id, i.id))
          }}
        />
      )}
    </div>
  )
}

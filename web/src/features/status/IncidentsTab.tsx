import { Trans, useLingui } from "@lingui/react/macro"
import { CalendarClockIcon, FlameIcon, PlusIcon } from "lucide-react"
import { useState } from "react"
import { Link, useNavigate } from "react-router"

import { ErrorLine, Segmented } from "@/components/common"
import { formatDateTime, formatRelative } from "@/components/common/text"
import { keyLabel, STATUS_KEYS } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Card, EmptyRow, Rows } from "@/features/settings/ui"
import { IncidentDialog } from "@/features/status/IncidentDialog"
import { incidentLink, useStatusIncidents } from "@/features/status/queries"
import { isClosed, useStatusText } from "@/features/status/text"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { StatusIncident, StatusIncidentKind, StatusPage } from "@/lib/api"

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

function Row({ page, i }: { page: StatusPage; i: StatusIncident }) {
  const { i18n } = useLingui()
  const when =
    i.kind === "maintenance" && i.scheduled_start && i.scheduled_end
      ? `${formatDateTime(i.scheduled_start, i18n.locale)} – ${formatDateTime(i.scheduled_end, i18n.locale)}`
      : formatRelative(i.resolved_at ?? i.created_at, i18n.locale)
  return (
    <li>
      <Link
        to={incidentLink(page.id, i.id)}
        className="flex flex-col gap-1 px-4 py-3 outline-none transition-colors hover:bg-muted focus-visible:bg-muted"
        data-testid="status-incident-row"
      >
        <span className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5">
          <span className="min-w-0 flex-1 basis-48 truncate text-body font-medium">{i.title}</span>
          <span className="shrink-0 text-caption text-faint">{when}</span>
        </span>
        <IncidentBadges i={i} />
      </Link>
    </li>
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
  const incidentKey = keyLabel(STATUS_KEYS.incident).join("")
  const maintenanceKey = keyLabel(STATUS_KEYS.maintenance).join("")

  useHotkeys({
    [STATUS_KEYS.incident]: () => setCreating("incident"),
    [STATUS_KEYS.maintenance]: () => setCreating("maintenance"),
  })

  return (
    <div className="flex flex-col gap-4" data-testid="status-incidents">
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
      ) : (
        <Card flush>
          {items.length === 0 ? (
            <EmptyRow>
              {open ? <Trans>No open incidents or maintenance.</Trans> : <Trans>Nothing is resolved yet.</Trans>}
            </EmptyRow>
          ) : (
            <Rows>
              {items.map((i) => (
                <Row key={i.id} page={page} i={i} />
              ))}
            </Rows>
          )}
        </Card>
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

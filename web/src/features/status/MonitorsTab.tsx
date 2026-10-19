import { Trans, useLingui } from "@lingui/react/macro"
import {
  HistoryIcon,
  KeyRoundIcon,
  MoreHorizontalIcon,
  PauseIcon,
  PencilIcon,
  PlayIcon,
  PlusIcon,
  Trash2Icon,
} from "lucide-react"
import { useState } from "react"
import { Link } from "react-router"

import { ErrorLine, toast, useConfirm } from "@/components/common"
import { formatDuration, formatRelative } from "@/components/common/text"
import { keyLabel, STATUS_KEYS } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { Card, EmptyRow, Rows, StatusTag } from "@/features/settings/ui"
import { MonitorChecksDialog } from "@/features/status/MonitorChecks"
import { MonitorDialog } from "@/features/status/MonitorDialog"
import { MonitorTokenDialog } from "@/features/status/MonitorToken"
import { incidentLink, useMonitorMutations, useStatusComponents, useStatusMonitors } from "@/features/status/queries"
import { useStatusText } from "@/features/status/text"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { StatusComponent, StatusMonitor, StatusMonitorWithToken, StatusPage } from "@/lib/api"
import { useSession } from "@/lib/session"

function overridden(m: StatusMonitor, c?: StatusComponent) {
  return (
    m.state === "down" &&
    !m.paused &&
    !!c &&
    (c.state_source === "member" || c.state_source === "api_key") &&
    Date.parse(c.state_since) > Date.parse(m.state_since)
  )
}

function Row({
  page,
  m,
  component,
  onChecks,
  onEdit,
  onToken,
}: {
  page: StatusPage
  m: StatusMonitor
  component?: StatusComponent
  onChecks: () => void
  onEdit: () => void
  onToken: (m: StatusMonitorWithToken, rotated: boolean) => void
}) {
  const { t, i18n } = useLingui()
  const { canManage } = useSession()
  const text = useStatusText()
  const mut = useMonitorMutations(page.id)
  const [confirm, dialog] = useConfirm()
  const key = m.paused ? "paused" : m.state
  const tone = key === "up" ? "success" : key === "down" ? "danger" : key === "paused" ? "warning" : "muted"
  const held = overridden(m, component)
  const name = m.name
  const failureState = text.component[m.failure_state]
  const onFailure = text.onFailure[m.on_failure]
  const last = m.last_result_at ? formatRelative(m.last_result_at, i18n.locale) : null
  const lastLabel = last ? (
    m.kind === "heartbeat" ? (
      <Trans>Last beat {last}</Trans>
    ) : (
      <Trans>Last state {last}</Trans>
    )
  ) : (
    <Trans>No result yet</Trans>
  )
  return (
    <li className="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3" data-testid="status-monitor">
      <div className="min-w-0 flex-1 basis-56">
        <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-body font-medium">
          <button
            type="button"
            onClick={onChecks}
            className="min-w-0 truncate rounded-sm text-start outline-none hover:underline focus-visible:underline"
            data-testid="status-monitor-open"
          >
            {m.name}
          </button>
          <StatusTag tone={tone} data-testid="status-monitor-state">
            {text.monitorState[key]}
          </StatusTag>
        </p>
        <p className="mt-px text-small text-faint">
          {text.monitorKind[m.kind]}
          {component ? ` · ${component.name}` : ""}
          {m.kind === "heartbeat" && m.interval ? ` · ${formatDuration(m.interval, i18n.locale)}` : ""}
          {" · "}
          {lastLabel}
        </p>
        <p className="text-small text-faint">
          <Trans>
            Sets {failureState}. {onFailure}.
          </Trans>
        </p>
        {m.state === "down" && !m.paused && m.last_message && (
          <p className="mt-0.5 text-small break-words text-destructive">{m.last_message}</p>
        )}
        {held && (
          <p className="mt-0.5 text-small text-muted-foreground" data-testid="status-monitor-overridden">
            <Trans>A member set the component by hand later, so it stays as they left it.</Trans>
          </p>
        )}
        {m.incident_id && (
          <Link
            to={incidentLink(page.id, m.incident_id)}
            className="mt-0.5 inline-block text-small text-brand underline-offset-4 hover:underline"
          >
            <Trans>Open incident</Trans>
          </Link>
        )}
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="ghost" size="icon-sm" aria-label={t`More for ${name}`} />}
          data-testid="status-monitor-menu"
        >
          <MoreHorizontalIcon />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={onChecks}>
            <HistoryIcon />
            <Trans>Recent checks</Trans>
          </DropdownMenuItem>
          {canManage && (
            <>
              <DropdownMenuItem onClick={onEdit}>
                <PencilIcon />
                <Trans>Edit</Trans>
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => mut.update.mutate({ id: m.id, paused: !m.paused })}>
                {m.paused ? <PlayIcon /> : <PauseIcon />}
                {m.paused ? <Trans>Resume</Trans> : <Trans>Pause</Trans>}
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() =>
                  confirm({
                    title: <Trans>Make a new address for {name}?</Trans>,
                    description: <Trans>The current address stops working at once. Update whatever calls it.</Trans>,
                    confirm: <Trans>Make new address</Trans>,
                    run: () => mut.rotate.mutate(m.id, { onSuccess: (r) => onToken(r, true) }),
                  })
                }
              >
                <KeyRoundIcon />
                <Trans>New address</Trans>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                variant="destructive"
                onClick={() =>
                  confirm({
                    title: <Trans>Delete {name}?</Trans>,
                    description: <Trans>Its checks are deleted. An incident it opened is resolved.</Trans>,
                    confirm: <Trans>Delete monitor</Trans>,
                    run: () => mut.remove.mutate(m.id, { onSuccess: () => toast(t`Monitor deleted`) }),
                  })
                }
              >
                <Trash2Icon />
                <Trans>Delete</Trans>
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <ErrorLine error={mut.update.error ?? mut.remove.error ?? mut.rotate.error} className="basis-full text-caption" />
      {dialog}
    </li>
  )
}

export function MonitorsTab({ page }: { page: StatusPage }) {
  const { canManage } = useSession()
  const monitors = useStatusMonitors(page.id)
  const components = useStatusComponents(page.id)
  const [dialog, setDialog] = useState<{ edit?: StatusMonitor } | null>(null)
  const [checks, setChecks] = useState<string | null>(null)
  const [token, setToken] = useState<{ monitor: StatusMonitorWithToken; rotated: boolean } | null>(null)
  const items = monitors.data ?? []
  const byId = new Map((components.data ?? []).map((c) => [c.id, c]))
  const checking = items.find((m) => m.id === checks)
  const addKey = keyLabel(STATUS_KEYS.addMonitor).join("")
  const { t } = useLingui()

  useHotkeys(canManage ? { [STATUS_KEYS.addMonitor]: () => setDialog({}) } : {})

  return (
    <div className="flex flex-col gap-4" data-testid="status-monitors">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="min-w-60 flex-1 text-body text-muted-foreground">
          <Trans>
            Set a component’s state from outside: a job that reports in, or your own monitoring that pushes its state.
          </Trans>
        </p>
        {canManage && (
          <Button
            size="sm"
            onClick={() => setDialog({})}
            title={t`Add monitor (${addKey})`}
            data-testid="status-add-monitor"
          >
            <PlusIcon />
            <Trans>Add monitor</Trans>
          </Button>
        )}
      </div>
      <ErrorLine error={monitors.error} />
      {monitors.isPending ? (
        <Skeleton className="h-32 w-full rounded-2xl" />
      ) : (
        <Card flush>
          {items.length === 0 ? (
            <EmptyRow>
              <Trans>No monitors yet. Add one to let a job or your monitoring set a component’s state.</Trans>
            </EmptyRow>
          ) : (
            <Rows>
              {items.map((m) => (
                <Row
                  key={m.id}
                  page={page}
                  m={m}
                  component={byId.get(m.component_id)}
                  onChecks={() => setChecks(m.id)}
                  onEdit={() => setDialog({ edit: m })}
                  onToken={(monitor, rotated) => setToken({ monitor, rotated })}
                />
              ))}
            </Rows>
          )}
        </Card>
      )}
      {dialog && (
        <MonitorDialog
          pageId={page.id}
          edit={dialog.edit}
          onClose={() => setDialog(null)}
          onCreated={(monitor) => {
            setDialog(null)
            setToken({ monitor, rotated: false })
          }}
        />
      )}
      {token && <MonitorTokenDialog monitor={token.monitor} rotated={token.rotated} onClose={() => setToken(null)} />}
      {checking && <MonitorChecksDialog pageId={page.id} monitor={checking} onClose={() => setChecks(null)} />}
    </div>
  )
}

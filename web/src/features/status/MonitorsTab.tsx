import { Trans, useLingui } from "@lingui/react/macro"
import {
  HistoryIcon,
  KeyRoundIcon,
  MoreHorizontalIcon,
  PauseIcon,
  PencilIcon,
  PlayIcon,
  Trash2Icon,
} from "lucide-react"
import { useState } from "react"
import { Link } from "react-router"

import { ErrorLine, toast, useConfirm } from "@/components/common"
import { formatDate, formatDuration, formatRelative } from "@/components/common/text"
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
import { PagePicker, useStatusScope } from "@/features/status/IncidentsPage"
import { MonitorChecksDialog } from "@/features/status/MonitorChecks"
import { MonitorDialog } from "@/features/status/MonitorDialog"
import { MonitorTokenDialog } from "@/features/status/MonitorToken"
import { Chip, FrameButton, framePad, PageFrame, Pill, ProductTag } from "@/features/status/PageFrame"
import {
  incidentLink,
  useMonitorMutations,
  useMonitorSummary,
  usePagesComponents,
  usePagesMonitors,
} from "@/features/status/queries"
import { useStatusText } from "@/features/status/text"
import { usePercent } from "@/features/status/UptimeBars"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { Inbox, StatusComponent, StatusMonitor, StatusMonitorWithToken, StatusPage } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"

function overridden(m: StatusMonitor, c?: StatusComponent) {
  return (
    m.state === "down" &&
    !m.paused &&
    !!c &&
    (c.state_source === "member" || c.state_source === "api_key") &&
    Date.parse(c.state_since) > Date.parse(m.state_since)
  )
}

function target(m: StatusMonitor) {
  if (m.kind === "http") return m.http?.url
  if (m.kind === "tcp" && m.tcp) return `${m.tcp.host.includes(":") ? `[${m.tcp.host}]` : m.tcp.host}:${m.tcp.port}`
}

function certDays(m: StatusMonitor) {
  if (!m.cert_expires_at) return null
  return Math.floor((Date.parse(m.cert_expires_at) - Date.now()) / 86400000)
}

function useResponseBars(pageId: string, m: StatusMonitor) {
  const summary = useMonitorSummary(pageId, m.id, "24h").data
  const buckets = summary?.buckets ?? []
  const size = Math.max(1, Math.ceil(buckets.length / 24))
  const groups = Array.from({ length: Math.ceil(buckets.length / size) }, (_, i) =>
    buckets.slice(i * size, i * size + size)
  ).map((g) => ({
    ms: Math.max(0, ...g.map((b) => b.p50_ms ?? 0)),
    failed: g.some((b) => b.failures > 0),
    checks: g.reduce((n, b) => n + b.checks, 0),
  }))
  return { summary, groups }
}

function Response({ pageId, m }: { pageId: string; m: StatusMonitor }) {
  const { t } = useLingui()
  const { groups } = useResponseBars(pageId, m)
  const timed = m.kind === "http" || m.kind === "tcp"
  const top = Math.max(1, ...groups.map((g) => g.ms))
  const bars = groups.length > 0 ? groups : Array.from({ length: 24 }, () => ({ ms: 0, failed: false, checks: 0 }))
  return (
    <span
      className="flex h-6 items-end gap-px"
      title={timed ? t`Response time, last 24 hours` : t`Results, last 24 hours`}
      data-testid="status-monitor-metrics"
      aria-hidden
    >
      {bars.map((g, i) => (
        <span
          key={i}
          className={cn(
            "min-w-px flex-1 rounded-t-[2px]",
            g.checks === 0 ? "bg-muted" : g.failed ? "bg-warning" : timed ? "bg-success/70" : "bg-success/30",
            timed && g.checks > 0 && i !== bars.length - 1 && "opacity-70"
          )}
          style={{ height: g.checks === 0 ? 2 : timed ? Math.max(3, Math.round((24 * g.ms) / top)) : 24 }}
        />
      ))}
    </span>
  )
}

function Uptime({ pageId, m }: { pageId: string; m: StatusMonitor }) {
  const percent = usePercent()
  const { summary } = useResponseBars(pageId, m)
  return (
    <span className="text-muted-foreground tabular-nums">
      {summary?.uptime === undefined ? "–" : percent(summary.uptime)}
    </span>
  )
}

const cols =
  "grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-x-4 @4xl:grid-cols-[minmax(0,1.3fr)_90px_minmax(0,1fr)_80px_150px_80px_90px_32px]"

function MonitorRow({
  m,
  page,
  inbox,
  component,
  onChecks,
  onEdit,
  onToken,
}: {
  m: StatusMonitor
  page?: StatusPage
  inbox?: Inbox
  component?: StatusComponent
  onChecks: () => void
  onEdit: () => void
  onToken: (m: StatusMonitorWithToken, rotated: boolean) => void
}) {
  const { t, i18n } = useLingui()
  const { canManage } = useSession()
  const text = useStatusText()
  const mut = useMonitorMutations(m.page_id)
  const [confirm, dialog] = useConfirm()
  const key = m.paused ? "paused" : m.state
  const held = overridden(m, component)
  const name = m.name
  const outside = m.kind === "http" || m.kind === "tcp"
  const goal = target(m)?.replace(/^https?:\/\//, "")
  const days = certDays(m)
  const warnDays = Math.max(m.http?.cert_warn_days ?? 0, 14)
  const expires = m.cert_expires_at ? formatDate(m.cert_expires_at, i18n.locale) : null
  const every = m.interval ? formatDuration(m.interval, i18n.locale) : ""
  const last = m.last_result_at ? formatRelative(m.last_result_at, i18n.locale) : null
  const stateTone = key === "up" ? "success" : key === "down" ? "danger" : "muted"
  const stateLabel = key === "pending" ? t`Waiting` : text.monitorState[key]
  return (
    <li
      className={cn(cols, "border-t px-4 py-3 transition-colors first:border-t-0 hover:bg-muted/50")}
      data-testid="status-monitor"
    >
      <span className="flex min-w-0 flex-col leading-[1.3]">
        <button
          type="button"
          onClick={onChecks}
          className="min-w-0 truncate rounded-sm text-start font-mono text-[13px] font-medium outline-none hover:underline focus-visible:underline"
          title={m.name}
          data-testid="status-monitor-open"
        >
          {goal ?? m.name}
        </button>
        <span className="truncate text-caption text-faint" data-testid="status-monitor-outside">
          {m.kind === "heartbeat" ? (
            <Trans>expects a ping every {every}</Trans>
          ) : m.kind === "push" ? (
            <Trans>state pushed by your alerting</Trans>
          ) : (
            <Trans>every {every}</Trans>
          )}
        </span>
        {m.state === "down" && !m.paused && m.last_message && (
          <span className="truncate text-caption text-destructive">{m.last_message}</span>
        )}
        {held && (
          <span className="text-caption text-muted-foreground" data-testid="status-monitor-overridden">
            <Trans>A member set the component by hand later, so it stays as they left it.</Trans>
          </span>
        )}
        {expires && days !== null && days <= warnDays && (
          <span
            className={cn("text-caption", days < 0 ? "text-destructive" : "text-warning")}
            data-testid="status-monitor-cert"
          >
            {days < 0 ? <Trans>Certificate expired {expires}</Trans> : <Trans>Certificate expires {expires}</Trans>}
          </span>
        )}
        {m.incident_id && (
          <Link
            to={incidentLink(m.page_id, m.incident_id)}
            className="w-fit text-caption text-brand underline-offset-4 hover:underline"
          >
            <Trans>Open incident</Trans>
          </Link>
        )}
      </span>
      <span className="hidden @4xl:block">
        <Chip>{text.monitorKind[m.kind]}</Chip>
      </span>
      <span className="hidden min-w-0 items-center gap-1.5 @4xl:flex">
        {page && <ProductTag name={inbox?.name ?? page.name} color={inbox?.branding.color} />}
        <span className="truncate text-muted-foreground">{component?.name ?? "–"}</span>
      </span>
      <Pill tone={stateTone} data-testid="status-monitor-state">
        {stateLabel}
      </Pill>
      <span className="hidden @4xl:block">
        <Response pageId={m.page_id} m={m} />
      </span>
      <span className="hidden @4xl:block">
        <Uptime pageId={m.page_id} m={m} />
      </span>
      <span className="hidden truncate text-caption text-faint @4xl:block" data-testid="status-monitor-response">
        {last ?? (outside ? <Trans>No result yet</Trans> : "–")}
      </span>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="ghost" size="icon-xs" className="text-faint" aria-label={t`More for ${name}`} />}
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
              {!outside && (
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
              )}
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
      <ErrorLine
        error={mut.update.error ?? mut.remove.error ?? mut.rotate.error}
        className="col-span-full text-caption"
      />
      {dialog}
    </li>
  )
}

export function MonitorsPage({ pageId }: { pageId: string }) {
  const { t } = useLingui()
  const { canManage } = useSession()
  const { items: pages, pages: pagesQuery, inboxOf } = useStatusScope()
  const ids = pages.map((p) => p.id)
  const monitors = usePagesMonitors(ids)
  const components = usePagesComponents(ids)
  const byComponent = new Map(components.map((c) => [c.id, c]))
  const byPage = new Map(pages.map((p) => [p.id, p]))
  const [dialog, setDialog] = useState<{ pageId: string; edit?: StatusMonitor } | null>(null)
  const [checks, setChecks] = useState<string | null>(null)
  const [token, setToken] = useState<{ monitor: StatusMonitorWithToken; rotated: boolean } | null>(null)
  const items = [...monitors.data].sort(
    (a, b) => ids.indexOf(a.page_id) - ids.indexOf(b.page_id) || a.created_at.localeCompare(b.created_at)
  )
  const checking = items.find((m) => m.id === checks)
  const fallback = byPage.get(pageId) ?? pages[0]
  const addKey = keyLabel(STATUS_KEYS.addMonitor).join("")

  useHotkeys(
    canManage && fallback ? { [STATUS_KEYS.addMonitor]: () => setDialog({ pageId: fallback.id }) } : {},
    !dialog
  )

  return (
    <PageFrame
      testId="status-monitors"
      title={<Trans>Monitors</Trans>}
      description={<Trans>Checks that watch your products and set component states</Trans>}
      actions={
        canManage &&
        pages.length > 0 && (
          <PagePicker
            pages={pages}
            inboxOf={inboxOf}
            onPick={(p) => setDialog({ pageId: p.id })}
            button={<FrameButton ink title={t`Add monitor (${addKey})`} data-testid="status-add-monitor" />}
          >
            <Trans>New monitor</Trans>
          </PagePicker>
        )
      }
    >
      <div className={cn(framePad, "@container")}>
        <ErrorLine error={pagesQuery.error ?? monitors.error} />
        {pagesQuery.isPending || (pages.length > 0 && monitors.isPending) ? (
          <Skeleton className="h-48 w-full rounded-xl" />
        ) : (
          <div className="overflow-hidden rounded-xl border bg-card">
            <div
              className={cn(cols, "hidden bg-surface px-4 py-2.5 text-caption font-medium text-faint @4xl:grid")}
              aria-hidden
            >
              <span>
                <Trans>Monitor</Trans>
              </span>
              <span>
                <Trans>Type</Trans>
              </span>
              <span>
                <Trans>Component</Trans>
              </span>
              <span>
                <Trans>State</Trans>
              </span>
              <span>
                <Trans>Response · 24 h</Trans>
              </span>
              <span>
                <Trans>Uptime</Trans>
              </span>
              <span>
                <Trans>Checked</Trans>
              </span>
              <span />
            </div>
            {items.length === 0 ? (
              <p className="border-t px-4 py-10 text-center text-faint @4xl:border-t">
                <Trans>
                  No monitors yet. Add one to let a check, a job or your monitoring set a component’s state.
                </Trans>
              </p>
            ) : (
              <ul className="@4xl:border-t">
                {items.map((m) => {
                  const page = byPage.get(m.page_id)
                  return (
                    <MonitorRow
                      key={m.id}
                      m={m}
                      page={page}
                      inbox={inboxOf(page)}
                      component={byComponent.get(m.component_id)}
                      onChecks={() => setChecks(m.id)}
                      onEdit={() => setDialog({ pageId: m.page_id, edit: m })}
                      onToken={(monitor, rotated) => setToken({ monitor, rotated })}
                    />
                  )
                })}
              </ul>
            )}
          </div>
        )}
      </div>
      {dialog && (
        <MonitorDialog
          pageId={dialog.pageId}
          edit={dialog.edit}
          onClose={() => setDialog(null)}
          onCreated={(monitor) => {
            setDialog(null)
            if (monitor.url) setToken({ monitor, rotated: false })
          }}
        />
      )}
      {token && <MonitorTokenDialog monitor={token.monitor} rotated={token.rotated} onClose={() => setToken(null)} />}
      {checking && <MonitorChecksDialog pageId={checking.page_id} monitor={checking} onClose={() => setChecks(null)} />}
    </PageFrame>
  )
}

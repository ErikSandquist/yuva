import { Plural, Trans, useLingui } from "@lingui/react/macro"
import {
  ArrowRightIcon,
  ArrowUpRightIcon,
  EyeIcon,
  MoreHorizontalIcon,
  PlusIcon,
  SettingsIcon,
  SquareStackIcon,
  UsersIcon,
} from "lucide-react"
import { useState } from "react"
import { Link, useNavigate, useSearchParams } from "react-router"

import { ErrorLine, InboxMark } from "@/components/common"
import { keyLabel, STATUS_KEYS } from "@/components/common/ShortcutSheet"
import { formatDuration, formatShort } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { CreatePageDialog } from "@/features/status/CreatePageDialog"
import { IncidentDialog } from "@/features/status/IncidentDialog"
import { useLinkedConversations } from "@/features/status/linked"
import { FrameButton, framePad, PageFrame } from "@/features/status/PageFrame"
import { PreviewDialog } from "@/features/status/PreviewDialog"
import {
  incidentLink,
  usePatchStatusPage,
  usePagesIncidents,
  usePublicStatus,
  useStatusComponents,
  useStatusMonitors,
  useStatusPages,
  statusLink,
} from "@/features/status/queries"
import { STATE_TONE, StateDot, useStatusText } from "@/features/status/text"
import { UptimeBars, usePercent } from "@/features/status/UptimeBars"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { Inbox, StatusIncident, StatusPage } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

export function hostOf(url: string) {
  return url.replace(/^https?:\/\//, "").replace(/\/$/, "")
}

function PageCard({ p, inbox, current }: { p: StatusPage; inbox?: Inbox; current: boolean }) {
  const { i18n } = useLingui()
  const text = useStatusText()
  const subscribed = new Intl.NumberFormat(i18n.locale).format(p.email_subscribers.counts.subscribed)
  return (
    <li>
      <Link
        to={`/status?page=${p.id}`}
        replace
        aria-current={current ? "page" : undefined}
        className={cn(
          "flex w-full flex-col items-start gap-1 rounded-xl border p-3.5 text-start outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
          current ? "border-foreground bg-background" : "bg-surface hover:border-faint"
        )}
        data-testid="status-home-page"
      >
        <span className="flex w-full items-center gap-2">
          <InboxMark name={inbox?.name ?? p.name} color={inbox?.branding.color} className="size-[22px] rounded-[6px]" />
          <span className="min-w-0 flex-1 truncate font-semibold">{p.name}</span>
          <StateDot state={p.state} />
        </span>
        <span className="w-full truncate text-caption text-faint">{hostOf(p.url)}</span>
        <span className="flex w-full items-center justify-between gap-2 text-caption">
          <span className={cn("truncate font-medium", STATE_TONE[p.state])}>{text.component[p.state]}</span>
          <span className="shrink-0 text-faint">
            {!p.published ? (
              <Trans>Unpublished</Trans>
            ) : p.email_subscribers.enabled ? (
              <Trans>{subscribed} subscribers</Trans>
            ) : (
              <Trans>Published</Trans>
            )}
          </span>
        </span>
      </Link>
    </li>
  )
}

function ActiveBanner({ p, i }: { p: StatusPage; i: StatusIncident }) {
  const { i18n } = useLingui()
  const text = useStatusText()
  const linked = useLinkedConversations(p.id, i.id)
  const count = linked.data?.pages.flatMap((x) => x.items).length ?? 0
  const since = new Intl.DateTimeFormat(i18n.locale, { timeStyle: "short" }).format(
    new Date(i.started_at ?? i.created_at)
  )
  const state = text.incident[i.state]
  return (
    <Link
      to={incidentLink(p.id, i.id)}
      className="flex items-center gap-3 rounded-xl border border-warning bg-warning/8 px-4 py-3.5 outline-none transition-colors hover:bg-warning/12 focus-visible:ring-2 focus-visible:ring-ring"
      data-testid="status-incident-row"
    >
      <span className="flex min-w-0 flex-1 flex-col leading-[1.35]">
        <span className="truncate font-semibold">{i.title}</span>
        <span className="truncate text-muted-foreground">
          <Trans>
            {state} since {since}
          </Trans>
          {!i.published_at && (
            <>
              {" · "}
              <Trans>not posted yet</Trans>
            </>
          )}
          {count > 0 && (
            <>
              {" · "}
              <Plural value={count} one="# linked conversation" other="# linked conversations" />
            </>
          )}
        </span>
      </span>
      <span className="inline-flex shrink-0 items-center gap-1 font-medium text-warning">
        <Trans>Post update</Trans>
        <ArrowRightIcon className="size-3.5 rtl:rotate-180" />
      </span>
    </Link>
  )
}

function ComponentsCard({ p }: { p: StatusPage }) {
  const text = useStatusText()
  const percent = usePercent()
  const components = useStatusComponents(p.id)
  const pub = usePublicStatus(p)
  const uptime = new Map((pub.data?.components ?? []).map((c) => [c.id, c]))
  const items = (components.data ?? []).filter((c) => !c.hidden)
  const days = p.history_days
  return (
    <section className="overflow-hidden rounded-xl border bg-card" data-testid="status-components-card">
      <div className="flex items-center justify-between gap-2 border-b px-4 py-3">
        <Link to={statusLink(p.id, "components")} className="font-semibold hover:underline">
          <Trans>Components</Trans>
        </Link>
        <span className="text-caption text-faint">
          <Plural value={days} one="# day" other="# days" />
        </span>
      </div>
      {components.isPending ? (
        <div className="p-4">
          <Skeleton className="h-20 w-full" />
        </div>
      ) : items.length === 0 ? (
        <p className="px-4 py-6 text-center text-faint">
          <Trans>No components yet.</Trans>
        </p>
      ) : (
        <ul>
          {items.map((c) => {
            const u = uptime.get(c.id)
            return (
              <li key={c.id} className="flex flex-col gap-2 border-b px-4 py-3 last:border-b-0">
                <span className="flex items-center gap-2">
                  <span className="min-w-0 flex-1 truncate font-medium">{c.name}</span>
                  {u?.uptime !== undefined && (
                    <span className="text-caption text-faint">
                      <Trans>{percent(u.uptime)} uptime</Trans>
                    </span>
                  )}
                  <span className={cn("w-auto min-w-21 text-end text-caption font-medium", STATE_TONE[c.state])}>
                    {text.component[c.state]}
                  </span>
                </span>
                {u?.days && u.days.length > 0 ? (
                  <UptimeBars days={u.days} count={days} className="h-[22px] gap-0.5" />
                ) : (
                  <span className="flex h-[22px] gap-0.5" aria-hidden>
                    {Array.from({ length: 45 }, (_, n) => (
                      <span key={n} className="flex-1 rounded-[2px] bg-muted" />
                    ))}
                  </span>
                )}
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}

function Fact({
  label,
  value,
  hint,
  testId,
}: {
  label: React.ReactNode
  value: string
  hint?: React.ReactNode
  testId?: string
}) {
  return (
    <div className="flex min-w-0 flex-col gap-1 rounded-xl border bg-card p-4" data-testid={testId}>
      <span className="truncate text-muted-foreground">{label}</span>
      <b className="text-title tabular-nums">{value}</b>
      <span className="truncate text-caption text-faint">{hint ?? " "}</span>
    </div>
  )
}

function lasted(i: StatusIncident) {
  const from = Date.parse(i.started_at ?? i.published_at ?? i.created_at)
  const to = i.resolved_at ? Date.parse(i.resolved_at) : Date.now()
  return Math.max(60, Math.round((to - from) / 1000))
}

function Facts({ p, incidents }: { p: StatusPage; incidents: StatusIncident[] }) {
  const { t, i18n } = useLingui()
  const fmt = new Intl.NumberFormat(i18n.locale)
  const monitors = useStatusMonitors(p.id).data
  const subs = p.email_subscribers
  const down = (monitors ?? []).filter((m) => m.state === "down" && !m.paused).length
  const up = (monitors ?? []).filter((m) => m.state === "up" && !m.paused).length
  const past = incidents
    .filter((i) => i.kind === "incident")
    .sort((a, b) => (b.started_at ?? b.created_at).localeCompare(a.started_at ?? a.created_at))
  const [last, before] = past
  const date = (i: StatusIncident) => formatShort(i.started_at ?? i.created_at, i18n.locale)
  const took = (i: StatusIncident) => formatDuration(lasted(i), i18n.locale)
  const open = !!last && !last.resolved_at
  return (
    <div className="grid grid-cols-1 gap-4 @xl:grid-cols-3" data-testid="status-facts">
      <Fact
        label={<Trans>Subscribers</Trans>}
        value={subs.enabled ? fmt.format(subs.counts.subscribed) : "–"}
        hint={subs.enabled ? <Trans>by e-mail</Trans> : <Trans>E-mail subscriptions are off</Trans>}
      />
      <Fact
        label={<Trans>Monitors</Trans>}
        value={monitors ? fmt.format(monitors.length) : "–"}
        hint={
          monitors &&
          monitors.length > 0 &&
          (down > 0 ? (
            <>
              <Plural value={down} one="# down" other="# down" />
              {", "}
              <Plural value={up} one="# up" other="# up" />
            </>
          ) : up === monitors.length ? (
            <Trans>all up</Trans>
          ) : (
            <Plural value={up} one="# up" other="# up" />
          ))
        }
      />
      <Fact
        label={<Trans>Last incident</Trans>}
        value={!last ? "–" : open ? t`Now` : date(last)}
        hint={
          !last ? (
            <Trans>None yet</Trans>
          ) : open ? (
            before ? (
              <Trans>
                before that: {date(before)}, {took(before)}
              </Trans>
            ) : (
              <Trans>the first one</Trans>
            )
          ) : (
            <Trans>resolved in {took(last)}</Trans>
          )
        }
      />
    </div>
  )
}

function Detail({ p, onNewIncident }: { p: StatusPage; onNewIncident: () => void }) {
  const { t } = useLingui()
  const { canManage } = useSession()
  const text = useStatusText()
  const navigate = useNavigate()
  const patch = usePatchStatusPage(p.id)
  const [preview, setPreview] = useState(false)
  const incidents = usePagesIncidents([p.id], false).data
  const active = incidents.find((i) => i.kind === "incident" && !i.resolved_at && i.state !== "resolved")
  const headline =
    p.state === "operational"
      ? t`All systems operational`
      : p.state === "degraded" || p.state === "partial_outage"
        ? t`Partly degraded`
        : text.component[p.state]
  const previewKey = keyLabel(STATUS_KEYS.preview).join("")
  const publishKey = keyLabel(STATUS_KEYS.publishPage).join("")
  const incidentKey = keyLabel(STATUS_KEYS.update).join("")

  useHotkeys({
    [STATUS_KEYS.incidents]: () => navigate(statusLink(p.id)),
    [STATUS_KEYS.components]: () => navigate(statusLink(p.id, "components")),
    [STATUS_KEYS.monitors]: () => navigate(statusLink(p.id, "monitors")),
    [STATUS_KEYS.subscribers]: () => navigate(statusLink(p.id, "subscribers")),
    ...(canManage ? { [STATUS_KEYS.settings]: () => navigate(statusLink(p.id, "settings")) } : {}),
    [STATUS_KEYS.preview]: () => setPreview(true),
    ...(canManage ? { [STATUS_KEYS.publishPage]: () => patch.mutate({ published: !p.published }) } : {}),
    [STATUS_KEYS.update]: () => (active ? navigate(incidentLink(p.id, active.id)) : onNewIncident()),
  })

  return (
    <div className="@container flex min-w-0 flex-1 flex-col gap-4 overflow-y-auto p-6 pb-16" data-testid="status-page">
      <div className="flex flex-wrap items-center gap-3">
        <StateDot state={p.state} className="size-3.5" />
        <span className="flex min-w-0 flex-1 flex-col leading-tight">
          <span className="text-title" data-testid="status-overall">
            {headline}
          </span>
          <span className="truncate text-caption text-faint" data-testid="status-published">
            {hostOf(p.url)} · {p.published ? <Trans>Published</Trans> : <Trans>Unpublished</Trans>}
          </span>
        </span>
        {p.published ? (
          <FrameButton
            render={<a href={p.url} target="_blank" rel="noreferrer noopener" />}
            data-testid="status-open-public"
          >
            <Trans>View page</Trans>
            <ArrowUpRightIcon className="rtl:-scale-x-100" />
          </FrameButton>
        ) : (
          <FrameButton onClick={() => setPreview(true)} title={t`Preview (${previewKey})`} data-testid="status-preview">
            <EyeIcon />
            <Trans>Preview</Trans>
          </FrameButton>
        )}
        {active ? (
          <FrameButton
            ink
            render={<Link to={incidentLink(p.id, active.id)} />}
            title={t`Post an update (${incidentKey})`}
            data-testid="status-post-update"
          >
            <Trans>Post update</Trans>
          </FrameButton>
        ) : (
          <FrameButton ink onClick={onNewIncident} data-testid="status-new-incident">
            <Trans>New incident</Trans>
          </FrameButton>
        )}
        <DropdownMenu>
          <DropdownMenuTrigger
            render={<Button variant="ghost" size="icon-sm" aria-label={t`More for ${p.name}`} />}
            data-testid="status-page-menu"
          >
            <MoreHorizontalIcon />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="min-w-52">
            <DropdownMenuItem onClick={() => navigate(statusLink(p.id, "components"))}>
              <SquareStackIcon />
              <Trans>Components</Trans>
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => navigate(statusLink(p.id, "subscribers"))}>
              <UsersIcon />
              <Trans>Subscribers</Trans>
            </DropdownMenuItem>
            {canManage && (
              <DropdownMenuItem onClick={() => navigate(statusLink(p.id, "settings"))}>
                <SettingsIcon />
                <Trans>Page settings</Trans>
              </DropdownMenuItem>
            )}
            <DropdownMenuItem onClick={() => setPreview(true)}>
              <EyeIcon />
              <Trans>Preview</Trans>
            </DropdownMenuItem>
            {canManage && (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  onClick={() => patch.mutate({ published: !p.published })}
                  title={p.published ? t`Unpublish (${publishKey})` : t`Publish (${publishKey})`}
                  data-testid="status-publish"
                >
                  {p.published ? <Trans>Unpublish</Trans> : <Trans>Publish</Trans>}
                </DropdownMenuItem>
              </>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <ErrorLine error={patch.error} />
      {active && <ActiveBanner p={p} i={active} />}
      <ComponentsCard p={p} />
      <Facts p={p} incidents={incidents} />
      {preview && <PreviewDialog page={p} onClose={() => setPreview(false)} />}
    </div>
  )
}

export function StatusHome() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const { canManage } = useSession()
  const [inboxId] = useInboxFilter()
  const inboxes = useInboxes().data ?? []
  const pages = useStatusPages(inboxId)
  const items = pages.data ?? []
  const shown = inboxId ? inboxes.filter((i) => i.id === inboxId) : inboxes
  const missing = canManage ? shown.find((i) => !items.some((p) => p.inbox_id === i.id)) : undefined
  const [creating, setCreating] = useState<Inbox | null>(null)
  const [incidentFor, setIncidentFor] = useState<string | null>(null)
  const byInbox = new Map(inboxes.map((i) => [i.id, i]))
  const current = items.find((p) => p.id === params.get("page")) ?? items[0]
  const at = current ? items.indexOf(current) : -1
  const newKey = keyLabel(STATUS_KEYS.incident).join("")
  const pick = (n: number) => items[n] && navigate(`/status?page=${items[n].id}`, { replace: true })
  const busy = !!creating || !!incidentFor

  useHotkeys(
    {
      j: () => pick(at + 1),
      k: () => pick(at - 1),
      ArrowDown: () => pick(at + 1),
      ArrowUp: () => pick(at - 1),
      ...(missing ? { [STATUS_KEYS.incident]: () => setCreating(missing) } : {}),
    },
    !busy
  )

  return (
    <PageFrame
      testId="status-home"
      title={<Trans>Status pages</Trans>}
      description={<Trans>Public pages that say whether your products work</Trans>}
      actions={
        missing && (
          <FrameButton
            ink
            onClick={() => setCreating(missing)}
            title={t`Create status page (${newKey})`}
            data-testid="status-home-create"
          >
            <Trans>New page</Trans>
          </FrameButton>
        )
      }
      bodyClassName="flex overflow-hidden phone:flex-col phone:overflow-y-auto"
    >
      {pages.isPending ? (
        <div className={cn(framePad, "w-full")}>
          <Skeleton className="h-24 w-72 rounded-xl" />
        </div>
      ) : pages.error ? (
        <div className={framePad}>
          <ErrorLine error={pages.error} />
        </div>
      ) : items.length === 0 ? (
        <div className={cn(framePad, "w-full")}>
          <div
            className="flex flex-col items-center gap-3 rounded-xl border border-dashed px-6 py-12 text-center"
            data-testid="status-home-empty"
          >
            <p className="max-w-md text-muted-foreground">
              <Trans>
                Tell your customers whether your product works. Each inbox gets one page, where you open incidents, plan
                maintenance and set the state of each component.
              </Trans>
            </p>
            {missing && (
              <Button size="sm" onClick={() => setCreating(missing)}>
                <PlusIcon />
                <Trans>New page</Trans>
              </Button>
            )}
          </div>
        </div>
      ) : (
        <>
          <ul
            className="flex w-[300px] shrink-0 flex-col gap-2.5 overflow-y-auto border-e p-4 phone:w-full phone:border-e-0 phone:border-b"
            aria-label={t`Status pages`}
          >
            {items.map((p) => (
              <PageCard key={p.id} p={p} inbox={byInbox.get(p.inbox_id)} current={p.id === current?.id} />
            ))}
          </ul>
          {current && <Detail key={current.id} p={current} onNewIncident={() => setIncidentFor(current.id)} />}
        </>
      )}
      {creating && (
        <CreatePageDialog
          inboxId={creating.id}
          inboxName={creating.name}
          onClose={() => setCreating(null)}
          onCreated={(p) => {
            setCreating(null)
            navigate(statusLink(p.id, "settings"))
          }}
        />
      )}
      {incidentFor && (
        <IncidentDialog
          pageId={incidentFor}
          kind="incident"
          onClose={() => setIncidentFor(null)}
          onDone={(i) => {
            setIncidentFor(null)
            navigate(incidentLink(i.page_id, i.id))
          }}
        />
      )}
    </PageFrame>
  )
}

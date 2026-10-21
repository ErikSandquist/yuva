import { useLingui } from "@lingui/react/macro"
import { PlusIcon, SettingsIcon } from "lucide-react"
import { useEffect, useRef } from "react"
import { useLocation, useMatch, useNavigate } from "react-router"

import { moveFocus, useEntries, useGo, useSection } from "@/app/Rail"
import type { Section } from "@/app/shell"
import { UserMenu } from "@/app/UserMenu"
import { keyLabel, mod } from "@/components/common/ShortcutSheet"
import { useAnalyticsProperties } from "@/features/analytics/properties"
import { changelogLink, useChangelogs } from "@/features/changelog/queries"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { ProductSwitcher } from "@/features/inbox/ProductSwitcher"
import { statusLink, useStatusPages } from "@/features/status/queries"
import { StateDot } from "@/features/status/text"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"

const rowClass =
  "relative flex h-9 w-full shrink-0 items-center gap-2.5 rounded-lg px-2.5 text-start text-body text-muted-foreground outline-none transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring aria-[current=page]:bg-brand-wash aria-[current=page]:font-medium aria-[current=page]:text-brand"

function Count({ n, id }: { n: number; id: string }) {
  if (!n) return null
  return (
    <span
      className="ms-auto grid h-[18px] min-w-[18px] place-items-center rounded-full bg-brand px-1.5 text-[11px] leading-none font-semibold text-white tabular-nums"
      data-testid={`rail-badge-${id}`}
    >
      {n > 99 ? "99+" : n}
    </span>
  )
}

function GroupHeading({ children }: { children: React.ReactNode }) {
  return <h3 className="px-2.5 pt-4 pb-1 text-caption font-medium text-faint">{children}</h3>
}

type Child = { key: string; label: React.ReactNode; to: string; current: boolean; mark?: React.ReactNode }

function Children({ section, items }: { section: string; items: Child[] }) {
  const navigate = useNavigate()
  const { pathname, search } = useLocation()
  const ref = useRef<HTMLDivElement>(null)
  const shown = items.length > 0
  useEffect(() => {
    if (shown) ref.current?.scrollIntoView({ block: "nearest" })
  }, [shown])
  if (!shown) return null
  return (
    <div
      ref={ref}
      className="ms-4.5 mt-0.5 mb-1 flex flex-col gap-px border-s ps-2"
      data-testid={`rail-${section}-children`}
    >
      {items.map((c) => (
        <button
          key={c.key}
          type="button"
          className={cn(rowClass, "h-8 gap-2 text-small")}
          aria-current={c.current ? "page" : undefined}
          onClick={() => pathname + search !== c.to && navigate(c.to)}
          data-rail-item
          data-testid={`rail-${section}-${c.key}`}
        >
          {c.mark}
          <span className="min-w-0 truncate">{c.label}</span>
        </button>
      ))}
    </div>
  )
}

function BroadcastChildren() {
  const { t } = useLingui()
  const { pathname } = useLocation()
  const current = pathname.startsWith("/broadcasts/lists")
    ? "lists"
    : pathname.startsWith("/broadcasts/designs")
      ? "designs"
      : "broadcasts"
  const items = [
    { key: "broadcasts", label: t`Broadcasts`, to: "/broadcasts" },
    { key: "lists", label: t`Lists`, to: "/broadcasts/lists" },
    { key: "designs", label: t`Designs`, to: "/broadcasts/designs" },
  ]
  return <Children section="broadcasts" items={items.map((c) => ({ ...c, current: current === c.key }))} />
}

function SettingsChildren() {
  const { t } = useLingui()
  const { pathname, search } = useLocation()
  const { canManage } = useSession()
  const channels = pathname.includes("/channels/") || (pathname === "/settings" && search.includes("view=channels"))
  const at = (...paths: string[]) => paths.some((p) => pathname === p || pathname.startsWith(`${p}/`))
  const items: Child[] = [
    {
      key: "products",
      label: t`Products`,
      to: "/settings",
      current: !channels && (pathname === "/settings" || at("/settings/inboxes")),
    },
    { key: "channels", label: t`Channels`, to: "/settings?view=channels", current: channels },
    { key: "members", label: t`Members`, to: "/settings/members", current: at("/settings/members") },
    {
      key: "canned-replies",
      label: t`Canned replies`,
      to: "/settings/canned-replies",
      current: at("/settings/canned-replies"),
    },
    { key: "labels", label: t`Labels`, to: "/settings/labels", current: at("/settings/labels") },
    {
      key: "developer",
      label: canManage ? t`API & MCP` : t`Connected apps`,
      to: canManage ? "/settings/api-keys" : "/settings/connected-apps",
      current: at("/settings/api-keys", "/settings/webhooks", "/settings/connected-apps"),
    },
    { key: "workspace", label: t`General`, to: "/settings/workspace", current: at("/settings/workspace") },
    ...(canManage
      ? [
          {
            key: "migrations",
            label: t`Move to Yuva`,
            to: "/settings/migrations",
            current: at("/settings/migrations"),
          },
        ]
      : []),
    { key: "profile", label: t`Profile`, to: "/settings/profile", current: at("/settings/profile") },
    {
      key: "notifications",
      label: t`Notifications`,
      to: "/settings/notifications",
      current: at("/settings/notifications"),
    },
    { key: "appearance", label: t`Appearance`, to: "/settings/appearance", current: at("/settings/appearance") },
  ]
  return <Children section="settings" items={items} />
}

function StatusChildren() {
  const [inboxId] = useInboxFilter()
  const pages = useStatusPages(inboxId).data ?? []
  const current = useMatch("/status-pages/:pageId/*")?.params.pageId
  return (
    <Children
      section="status"
      items={pages.map((p) => ({
        key: p.id,
        label: p.name,
        to: statusLink(p.id),
        current: p.id === current,
        mark: <StateDot state={p.state} />,
      }))}
    />
  )
}

function ChangelogChildren() {
  const [inboxId] = useInboxFilter()
  const changelogs = (useChangelogs().data ?? []).filter((c) => !inboxId || c.inbox_id === inboxId)
  const current = useMatch("/updates/:changelogId/*")?.params.changelogId
  return (
    <Children
      section="updates"
      items={changelogs.map((c) => ({ key: c.id, label: c.name, to: changelogLink(c.id), current: c.id === current }))}
    />
  )
}

function AnalyticsChildren() {
  const { t } = useLingui()
  const { canManage } = useSession()
  const [inboxId] = useInboxFilter()
  const properties = (useAnalyticsProperties().data?.items ?? []).filter((p) => !inboxId || p.inbox_id === inboxId)
  const current = useMatch("/analytics/:propertyId/*")?.params.propertyId
  return (
    <Children
      section="analytics"
      items={properties
        .map<Child>((p) => ({
          key: p.id,
          label: p.name,
          to: `/analytics/${p.id}`,
          current: p.id === current,
        }))
        .concat(
          canManage
            ? [
                {
                  key: "add",
                  label: t`Add a website`,
                  to: "/analytics/new",
                  current: false,
                  mark: <PlusIcon className="size-3.5 shrink-0" />,
                },
              ]
            : []
        )}
    />
  )
}

const nested: Partial<Record<Section, () => React.ReactNode>> = {
  broadcasts: BroadcastChildren,
  status: StatusChildren,
  updates: ChangelogChildren,
  analytics: AnalyticsChildren,
}

export function Sidebar() {
  const { t } = useLingui()
  const section = useSection()
  const go = useGo()
  const entries = useEntries()
  const { membership } = useSession()
  const labels: Partial<Record<Section, string>> = {
    conversations: t`Inbox`,
    knowledge: t`Help center`,
    status: t`Status pages`,
  }
  const groups: { title?: string; keys: Section[] }[] = [
    { keys: ["home", "conversations", "mentions", "assistants"] },
    { title: t`Customers`, keys: ["contacts", "feedback", "knowledge"] },
    {
      title: t({ message: "Publish", context: "sidebar group" }),
      keys: ["status", "updates", "board", "broadcasts", "docs"],
    },
    { title: t`Insights`, keys: ["analytics", "reports"] },
    { title: t`Workspace`, keys: ["team"] },
  ]
  const byKey = new Map(entries.map((e) => [e.key, e]))
  const workspace = membership.workspace.name
  return (
    <aside className="flex w-62 shrink-0 flex-col border-e bg-surface" data-testid="sidebar">
      <div className="flex h-13 shrink-0 items-center gap-2.5 px-4">
        <span
          aria-hidden
          className="grid size-7 shrink-0 place-items-center rounded-lg bg-brand text-small font-semibold text-white uppercase"
        >
          {Array.from(workspace.trim())[0] ?? ""}
        </span>
        <span className="min-w-0 flex-1 truncate font-medium" data-testid="sidebar-workspace">
          {workspace}
        </span>
      </div>
      <div className="px-3 pb-1 empty:hidden">
        <ProductSwitcher />
      </div>
      <nav
        aria-label={t`Sections`}
        onKeyDown={moveFocus}
        className="flex min-h-0 flex-1 flex-col gap-px overflow-y-auto px-3 pt-2 pb-3"
        data-testid="rail"
      >
        {groups.map((g, gi) => {
          const items = g.keys.flatMap((k) => byKey.get(k) ?? [])
          const settings = gi === groups.length - 1
          if (items.length === 0 && !settings) return null
          return (
            <div key={g.title ?? "main"} className="flex flex-col gap-px" role="group" aria-label={g.title}>
              {g.title && <GroupHeading>{g.title}</GroupHeading>}
              {items.map((e) => {
                const label = labels[e.key] ?? e.label
                const Nested = section === e.key ? nested[e.key] : undefined
                return (
                  <div key={e.key} className="flex flex-col gap-px">
                    <button
                      type="button"
                      className={rowClass}
                      aria-current={section === e.key ? "page" : undefined}
                      aria-label={e.badge ? e.badgeLabel : undefined}
                      title={`${label} · ${keyLabel(e.shortcut).join(" ")}`}
                      onClick={() => go(e.key)}
                      data-rail-item
                      data-testid={`rail-${e.key}`}
                    >
                      <e.icon className="size-4.5 shrink-0" />
                      <span className="min-w-0 truncate">{label}</span>
                      <Count n={e.badge ?? 0} id={e.key} />
                    </button>
                    {Nested && <Nested />}
                  </div>
                )
              })}
              {settings && (
                <button
                  type="button"
                  className={rowClass}
                  aria-current={section === "settings" ? "page" : undefined}
                  title={`${t`Settings`} · ${mod} ,`}
                  onClick={() => go("settings")}
                  data-rail-item
                  data-testid="rail-settings"
                >
                  <SettingsIcon className="size-4.5 shrink-0" />
                  <span className="min-w-0 truncate">{t`Settings`}</span>
                </button>
              )}
              {settings && section === "settings" && <SettingsChildren />}
            </div>
          )
        })}
      </nav>
      <div className="flex shrink-0 items-center border-t px-2 py-2">
        <UserMenu align="start" side="top" row />
      </div>
    </aside>
  )
}

import { useLingui } from "@lingui/react/macro"
import { BellIcon, CircleHelpIcon } from "lucide-react"
import { useNavigate } from "react-router"

import { useShell } from "@/app/shell"
import { UserMenu } from "@/app/UserMenu"
import { Kbd } from "@/components/common"
import { YuvaMark } from "@/components/common/Logo"
import { mod } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { SidebarTrigger, useSidebar } from "@/components/ui/sidebar"
import { useMentions } from "@/features/inbox/queries"

export function TopBar() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { openPalette, openShortcuts } = useShell()
  const sidebar = useSidebar()
  const collapsed = sidebar.isMobile ? !sidebar.openMobile : !sidebar.open
  const unseen = Number(useMentions().data?.pages[0]?.unseen ?? 0)
  const sidebarLabel = collapsed ? t`Expand the sidebar` : t`Collapse the sidebar`
  const mentionsLabel = unseen ? t`Mentions, ${unseen} unseen` : t`Mentions`
  return (
    <header
      className="sticky top-0 z-20 flex h-11 shrink-0 items-center gap-1.5 bg-sidebar ps-2 pe-4 text-sidebar-foreground"
      data-testid="top-bar"
    >
      <SidebarTrigger
        className="size-8 rounded-md text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground rtl:[&_svg]:-scale-x-100"
        aria-label={sidebarLabel}
        aria-pressed={!collapsed}
        title={`${sidebarLabel} · ${mod} \\`}
        data-testid="sidebar-toggle"
      />
      <UserMenu bar align="start" />
      <div className="flex-1" />
      <button
        type="button"
        onClick={openPalette}
        className="flex h-8 w-[260px] min-w-0 shrink items-center gap-2 rounded-md bg-sidebar-accent ps-2.5 pe-1.5 text-start text-body text-sidebar-muted outline-none transition-colors hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-sidebar-ring"
        data-testid="top-search"
      >
        <span className="min-w-0 flex-1 truncate">{t`Search`}</span>
        <span className="flex shrink-0 gap-0.5" aria-hidden>
          <Kbd className="border-sidebar-border bg-sidebar text-sidebar-muted">{mod}</Kbd>
          <Kbd className="border-sidebar-border bg-sidebar text-sidebar-muted">K</Kbd>
        </span>
      </button>
      <Button
        variant="ghost"
        size="icon-sm"
        className="relative size-8 rounded-md text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
        onClick={() => navigate("/mentions")}
        aria-label={mentionsLabel}
        title={mentionsLabel}
        data-testid="top-notifications"
      >
        <BellIcon />
        {unseen > 0 && (
          <span
            aria-hidden
            className="absolute end-1.5 top-1.5 size-[7px] rounded-full bg-brand"
            data-testid="top-notifications-dot"
          />
        )}
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        className="size-8 rounded-md text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
        onClick={openShortcuts}
        aria-label={t`Keyboard shortcuts`}
        title={`${t`Keyboard shortcuts`} · ?`}
        data-testid="top-shortcuts"
      >
        <CircleHelpIcon />
      </Button>
      <span
        className="ms-3 flex items-center gap-2 text-body font-semibold text-sidebar-accent-foreground"
        data-testid="top-logo"
      >
        <YuvaMark className="size-6" />
        Yuva
      </span>
    </header>
  )
}

import { useLingui } from "@lingui/react/macro"
import { KeyboardIcon, PanelLeftCloseIcon, PanelLeftOpenIcon, SearchIcon } from "lucide-react"

import { useShell } from "@/app/shell"
import { useSidebarCollapsed } from "@/app/sidebarState"
import { Kbd } from "@/components/common"
import { mod } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"

export function TopBar() {
  const { t } = useLingui()
  const { openPalette, openShortcuts } = useShell()
  const [collapsed, toggle] = useSidebarCollapsed()
  const sidebarLabel = collapsed ? t`Expand the sidebar` : t`Collapse the sidebar`
  return (
    <header className="flex h-13 shrink-0 items-center gap-2 border-b bg-background px-3" data-testid="top-bar">
      <Button
        variant="ghost"
        size="icon-sm"
        className="text-muted-foreground"
        onClick={toggle}
        aria-label={sidebarLabel}
        aria-pressed={!collapsed}
        title={`${sidebarLabel} · ${mod} \\`}
        data-testid="sidebar-toggle"
      >
        {collapsed ? (
          <PanelLeftOpenIcon className="rtl:-scale-x-100" />
        ) : (
          <PanelLeftCloseIcon className="rtl:-scale-x-100" />
        )}
      </Button>
      <button
        type="button"
        onClick={openPalette}
        className="flex h-9 w-full max-w-md min-w-0 items-center gap-2 rounded-lg border bg-surface ps-2.5 pe-1.5 text-start text-body text-faint outline-none transition-colors hover:border-faint hover:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring"
        data-testid="top-search"
      >
        <SearchIcon className="size-4 shrink-0" />
        <span className="min-w-0 flex-1 truncate">{t`Search or jump to…`}</span>
        <span className="flex shrink-0 gap-0.5" aria-hidden>
          <Kbd>{mod}</Kbd>
          <Kbd>K</Kbd>
        </span>
      </button>
      <span className="flex-1" />
      <Button
        variant="ghost"
        size="icon-sm"
        className="text-muted-foreground"
        onClick={openShortcuts}
        aria-label={t`Keyboard shortcuts`}
        title={`${t`Keyboard shortcuts`} · ?`}
        data-testid="top-shortcuts"
      >
        <KeyboardIcon />
      </Button>
    </header>
  )
}

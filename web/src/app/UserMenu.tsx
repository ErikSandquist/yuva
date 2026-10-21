import { Trans, useLingui } from "@lingui/react/macro"
import {
  BellIcon,
  BookUserIcon,
  BuildingIcon,
  DownloadIcon,
  LanguagesIcon,
  LogOutIcon,
  MonitorIcon,
  MoonIcon,
  PlusIcon,
  SunIcon,
  UserIcon,
} from "lucide-react"
import { useNavigate } from "react-router"

import { PersonAvatar } from "@/components/common"
import { useChangeLocale } from "@/components/common/LanguageMenu"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { locales, type Locale } from "@/i18n"
import { useVersion, type Availability } from "@/lib/api"
import { useSetAvailability } from "@/lib/availability"
import { useInstallPrompt } from "@/lib/pwa"
import { useRealtimeStatus } from "@/lib/realtime"
import { useSession, useSignOut } from "@/lib/session"
import { cn } from "@/lib/utils"
import { setTheme, useTheme, type Theme } from "@/lib/theme"
import { Button } from "@/components/ui/button"

function statusClass(a: Availability, live: boolean) {
  if (a === "away") return "bg-faint"
  return live ? "bg-success" : "bg-faint"
}

export function UserMenu({
  align = "end",
  side = "bottom",
  row,
  bar,
}: {
  align?: "start" | "end"
  side?: "bottom" | "inline-end" | "top"
  row?: boolean
  bar?: boolean
}) {
  const { t, i18n } = useLingui()
  const { me, membership, switchWorkspace } = useSession()
  const theme = useTheme()
  const navigate = useNavigate()
  const signOut = useSignOut()
  const install = useInstallPrompt()
  const changeLocale = useChangeLocale()
  const { version, signup } = useVersion().data ?? {}
  const live = useRealtimeStatus() === "live"
  const availability = me.person.availability
  const setAvailability = useSetAvailability()

  const statusText = availability === "away" ? t`Away` : live ? t`Available` : t`Offline`
  const name = me.person.name || me.person.email
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          bar ? (
            <button
              type="button"
              className="flex h-11 min-w-0 items-center gap-2 px-2.5 text-start text-sidebar-foreground outline-none transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:bg-sidebar-accent data-popup-open:bg-sidebar-accent"
            />
          ) : row ? (
            <button
              type="button"
              className="flex h-11 w-full min-w-0 items-center gap-2.5 rounded-md px-1.5 text-start text-sidebar-foreground outline-none transition-colors hover:bg-sidebar-accent focus-visible:ring-2 focus-visible:ring-sidebar-ring data-popup-open:bg-sidebar-accent group-data-[collapsible=icon]:size-10 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0"
            />
          ) : (
            <Button variant="ghost" size="icon" className="relative" />
          )
        }
        aria-label={t`${name} · ${statusText}`}
        title={statusText}
        data-testid="user-menu"
      >
        <span className={row || bar ? "relative flex shrink-0" : "contents"}>
          <PersonAvatar
            name={name}
            className={cn(
              bar ? "size-7 text-caption" : "size-[30px]",
              row || bar ? "bg-sidebar-muted text-sidebar" : "bg-muted-foreground text-background"
            )}
          />
          <span
            className={cn(
              "absolute rounded-full ring-2",
              bar ? "end-[-2px] bottom-[-2px] size-[7px]" : "size-[9px]",
              row || bar ? "ring-sidebar" : "ring-background",
              !bar && (row ? "end-[-1px] bottom-[-1px]" : "end-px bottom-px"),
              statusClass(availability, live)
            )}
            data-testid="availability-dot"
            data-availability={availability}
          />
        </span>
        {bar && <span className="max-w-48 truncate text-body font-medium">{name}</span>}
        {row && (
          <span className="min-w-0 flex-1 group-data-[collapsible=icon]:hidden">
            <span className="block truncate text-body font-medium text-sidebar-accent-foreground">{name}</span>
            <span className="block truncate text-caption text-sidebar-muted">{statusText}</span>
          </span>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align={align} side={side} className="w-64">
        <DropdownMenuGroup>
          <DropdownMenuLabel className="flex items-center gap-2.5 py-2">
            <PersonAvatar name={name} className="size-8 bg-muted-foreground text-background" />
            <span className="min-w-0">
              <span className="block truncate text-body font-medium text-foreground">{name}</span>
              <span className="block truncate text-caption font-normal text-muted-foreground">{me.person.email}</span>
            </span>
          </DropdownMenuLabel>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuGroup>
          <DropdownMenuRadioGroup
            value={availability}
            onValueChange={(v) => {
              if (v !== availability) setAvailability.mutate(v as Availability)
            }}
          >
            <DropdownMenuRadioItem value="auto" data-testid="availability-auto">
              <span className={cn("size-2 shrink-0 rounded-full", statusClass("auto", true))} />
              <Trans>Available</Trans>
            </DropdownMenuRadioItem>
            <DropdownMenuRadioItem value="away" data-testid="availability-away">
              <span className={cn("size-2 shrink-0 rounded-full", statusClass("away", true))} />
              <Trans>Away</Trans>
            </DropdownMenuRadioItem>
          </DropdownMenuRadioGroup>
          {setAvailability.error && (
            <p role="alert" className="px-2.5 py-1 text-caption text-destructive">
              <Trans>The status was not changed. Try again.</Trans>
            </p>
          )}
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => navigate("/settings/profile")}>
          <UserIcon />
          <Trans>My profile</Trans>
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => navigate("/settings/notifications")} data-testid="menu-notifications">
          <BellIcon />
          <Trans>Notifications</Trans>
        </DropdownMenuItem>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            {theme === "dark" ? <MoonIcon /> : theme === "light" ? <SunIcon /> : <MonitorIcon />}
            <Trans>Appearance</Trans>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="min-w-40">
            <DropdownMenuRadioGroup value={theme} onValueChange={(v) => setTheme(v as Theme)}>
              <DropdownMenuRadioItem value="system" data-testid="theme-system">
                <Trans>System</Trans>
              </DropdownMenuRadioItem>
              <DropdownMenuRadioItem value="light" data-testid="theme-light">
                <Trans>Light</Trans>
              </DropdownMenuRadioItem>
              <DropdownMenuRadioItem value="dark" data-testid="theme-dark">
                <Trans>Dark</Trans>
              </DropdownMenuRadioItem>
            </DropdownMenuRadioGroup>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <LanguagesIcon />
            <span className="flex-1">
              <Trans>Language</Trans>
            </span>
            <span className="text-caption text-muted-foreground">{Object.keys(locales).length}</span>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="max-h-80 min-w-44 overflow-y-auto">
            <DropdownMenuRadioGroup value={i18n.locale} onValueChange={(v) => changeLocale(v as Locale)}>
              {Object.entries(locales)
                .sort(([a], [b]) => Number(b === i18n.locale) - Number(a === i18n.locale))
                .map(([k, v]) => (
                  <DropdownMenuRadioItem key={k} value={k}>
                    {v}
                  </DropdownMenuRadioItem>
                ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        {!bar && (me.memberships.length > 1 || signup === "open") && (
          <DropdownMenuSub>
            <DropdownMenuSubTrigger>
              <BuildingIcon />
              <span className="min-w-0 flex-1 truncate">{membership.workspace.name}</span>
            </DropdownMenuSubTrigger>
            <DropdownMenuSubContent className="min-w-56">
              <DropdownMenuRadioGroup
                value={membership.workspace.id}
                onValueChange={(id) => switchWorkspace(String(id))}
              >
                {me.memberships.map((m) => (
                  <DropdownMenuRadioItem key={m.workspace.id} value={m.workspace.id}>
                    <span className="truncate">{m.workspace.name}</span>
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
              {signup === "open" && (
                <>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={() => navigate("/workspaces/new")} data-testid="create-workspace">
                    <PlusIcon />
                    <Trans>Create workspace</Trans>
                  </DropdownMenuItem>
                </>
              )}
            </DropdownMenuSubContent>
          </DropdownMenuSub>
        )}
        {!bar && (
          <DropdownMenuItem
            onClick={() => navigate("/contacts")}
            className="hidden phone:flex"
            data-testid="menu-contacts"
          >
            <BookUserIcon />
            <Trans>Contacts</Trans>
          </DropdownMenuItem>
        )}
        {install && (
          <DropdownMenuItem onClick={() => void install()} data-testid="install-app">
            <DownloadIcon />
            <Trans>Install app</Trans>
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onClick={async () => {
            await signOut()
            navigate("/sign-in", { replace: true })
          }}
        >
          <LogOutIcon />
          <Trans>Sign out</Trans>
          {version && <span className="ms-auto text-caption text-faint">{version}</span>}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

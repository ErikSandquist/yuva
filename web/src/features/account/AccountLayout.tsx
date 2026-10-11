import { Trans } from "@lingui/react/macro"
import { RefreshCwIcon } from "lucide-react"
import { useEffect } from "react"

import { EmptyState, YuvaMark } from "@/components/common"
import { LanguageMenu } from "@/components/common/LanguageMenu"
import { Button } from "@/components/ui/button"
import { activate, i18n } from "@/i18n"
import { cn } from "@/lib/utils"
import { useMe } from "@/lib/session"

export function AccountLayout({ children, narrow }: { children: React.ReactNode; narrow?: boolean }) {
  return (
    <div className="flex min-h-svh flex-col bg-background" data-testid="account-layout">
      <header
        className={cn(
          "mx-auto flex h-16 w-full shrink-0 items-center justify-between gap-4 px-4 sm:px-6",
          narrow ? "max-w-md" : "max-w-3xl"
        )}
      >
        <span className="flex items-center gap-2.5">
          <YuvaMark className="size-[26px]" />
          <span className="text-title">Yuva</span>
        </span>
        <LanguageMenu />
      </header>
      <main
        className={cn(
          "mx-auto flex w-full flex-1 flex-col px-4 pb-16 sm:px-6",
          narrow ? "max-w-md pt-[4vh]" : "max-w-3xl pt-4"
        )}
      >
        {children}
      </main>
    </div>
  )
}

export function usePersonLocale() {
  const locale = useMe().data?.person.locale
  useEffect(() => {
    if (locale && locale !== i18n.locale) activate(locale)
  }, [locale])
}

export function Unreachable({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="flex min-h-svh flex-col bg-background">
      <EmptyState icon={RefreshCwIcon} title={<Trans>Could not reach the server</Trans>}>
        <Button className="mt-3" onClick={onRetry}>
          <Trans>Try again</Trans>
        </Button>
      </EmptyState>
    </div>
  )
}

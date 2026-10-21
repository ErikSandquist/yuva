import { Trans } from "@lingui/react/macro"
import { RefreshCwIcon } from "lucide-react"
import { useEffect } from "react"

import { EmptyState, YuvaMark } from "@/components/common"
import { LanguageMenu } from "@/components/common/LanguageMenu"
import { Button } from "@/components/ui/button"
import { activate, i18n } from "@/i18n"
import { useMe } from "@/lib/session"

export function AccountLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-svh flex-col bg-background" data-testid="account-layout">
      <header className="h-13 shrink-0 border-b">
        <div className="mx-auto flex h-full w-full max-w-3xl items-center justify-between gap-4 px-4 sm:px-6">
          <span className="flex items-center gap-2">
            <YuvaMark className="size-6" />
            <span className="text-body font-semibold">Yuva</span>
          </span>
          <LanguageMenu />
        </div>
      </header>
      <main className="mx-auto flex w-full max-w-3xl flex-1 flex-col px-4 pt-8 pb-16 sm:px-6">{children}</main>
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

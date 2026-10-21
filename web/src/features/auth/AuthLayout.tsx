import { Trans } from "@lingui/react/macro"

import { YuvaMark } from "@/components/common"
import { LanguageMenu } from "@/components/common/LanguageMenu"
import { useVersion } from "@/lib/api"
import { cn } from "@/lib/utils"

export const authField = "h-10 px-3.5"
export const authAlert =
  "rounded-xl border border-destructive/25 bg-destructive/5 px-3.5 py-2.5 text-body text-destructive"

export function AuthLayout({
  children,
  wide,
  below,
}: {
  children: React.ReactNode
  wide?: boolean
  below?: React.ReactNode
}) {
  const version = useVersion().data?.version
  return (
    <div className="flex min-h-svh flex-col bg-surface" data-testid="auth-layout">
      <header className="flex h-13 shrink-0 items-center justify-end px-4 sm:px-6">
        <LanguageMenu />
      </header>
      <main className="flex flex-1 flex-col items-center px-4 pt-[2vh] pb-10 sm:pt-[7vh]">
        <span className="mb-6 flex items-center gap-2.5">
          <YuvaMark className="size-8" />
          <span className="text-title">Yuva</span>
        </span>
        <div className={cn("flex w-full flex-col gap-5", wide ? "max-w-[30rem]" : "max-w-[25rem]")}>
          <div className="rounded-2xl border bg-card px-5 py-7 shadow-[0_1px_2px_rgb(15_23_42/0.04)] sm:px-8 sm:py-8">
            {children}
          </div>
          {below && (
            <div className="flex flex-col items-center gap-2 px-2 text-center text-small text-faint">{below}</div>
          )}
        </div>
      </main>
      <footer className="flex h-12 shrink-0 items-center justify-center text-caption text-faint">
        {version && <Trans>Yuva {version}</Trans>}
      </footer>
    </div>
  )
}

export function AuthHeading({
  icon,
  tone = "brand",
  title,
  children,
  titleTestId,
  className,
}: {
  icon?: React.ReactNode
  tone?: "brand" | "muted" | "success" | "danger"
  title: React.ReactNode
  children?: React.ReactNode
  titleTestId?: string
  className?: string
}) {
  return (
    <div className={cn("mb-7 flex flex-col items-center text-center", className)}>
      {icon && (
        <span
          className={cn(
            "mb-4 grid size-10 place-items-center rounded-full [&_svg]:size-[18px]",
            tone === "brand" && "bg-brand-wash text-brand",
            tone === "muted" && "border text-faint",
            tone === "success" && "bg-success/10 text-success",
            tone === "danger" && "bg-destructive/10 text-destructive"
          )}
        >
          {icon}
        </span>
      )}
      <h1 className="text-title text-balance break-words" data-testid={titleTestId}>
        {title}
      </h1>
      {children && <div className="mt-2 text-body text-pretty text-muted-foreground">{children}</div>}
    </div>
  )
}

export function AuthDivider({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3 text-caption text-faint">
      <span className="h-px flex-1 bg-border" />
      {children}
      <span className="h-px flex-1 bg-border" />
    </div>
  )
}

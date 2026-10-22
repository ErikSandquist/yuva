import { cn } from "@/lib/utils"

export function EmptyBox({
  icon: Icon,
  title,
  children,
  actions,
  className,
  testId,
}: {
  icon?: React.ComponentType<{ className?: string }>
  title: React.ReactNode
  children?: React.ReactNode
  actions?: React.ReactNode
  className?: string
  testId?: string
}) {
  return (
    <div
      className={cn(
        "flex flex-col items-center gap-2 rounded-xl border border-dashed px-6 py-12 text-center text-body text-muted-foreground",
        className
      )}
      data-testid={testId}
    >
      {Icon && <Icon className="size-6 text-faint" />}
      <p className="font-semibold text-foreground">{title}</p>
      {children && <p className="max-w-md">{children}</p>}
      {actions && <div className="mt-1 flex flex-wrap justify-center gap-2">{actions}</div>}
    </div>
  )
}

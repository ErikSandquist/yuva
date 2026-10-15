import { cn } from "@/lib/utils"

export function Sparkline({
  values,
  label,
  className,
  testId,
}: {
  values: number[]
  label: string
  className?: string
  testId?: string
}) {
  const max = Math.max(1, ...values)
  return (
    <div role="img" aria-label={label} className={cn("flex h-10 items-end gap-1", className)} data-testid={testId}>
      {values.map((v, i) => (
        <span
          key={i}
          className={cn(
            "min-h-[3px] flex-1 rounded-[3px]",
            i === values.length - 1 ? "bg-brand" : v > 0 ? "bg-brand/35" : "bg-border"
          )}
          style={{ height: `${Math.max(8, (v / max) * 100)}%` }}
        />
      ))}
    </div>
  )
}

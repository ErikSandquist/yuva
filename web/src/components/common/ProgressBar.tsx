import { cn } from "@/lib/utils"

export function ProgressBar({
  value,
  label,
  text,
  thin,
  className,
  testId,
}: {
  value?: number
  label: string
  text?: string
  thin?: boolean
  className?: string
  testId?: string
}) {
  const known = value !== undefined
  return (
    <span
      className={cn("relative block w-full overflow-hidden rounded-full bg-muted", thin ? "h-1" : "h-2", className)}
      role="progressbar"
      aria-label={label}
      aria-valuemin={known ? 0 : undefined}
      aria-valuemax={known ? 100 : undefined}
      aria-valuenow={known ? value : undefined}
      aria-valuetext={text}
      data-testid={testId}
    >
      <span
        className={cn(
          "absolute inset-y-0 start-0 rounded-full bg-brand transition-[width] duration-700",
          !known && "w-1/4 animate-pulse"
        )}
        style={known ? { width: `${Math.max(0, Math.min(100, value))}%` } : undefined}
      />
    </span>
  )
}

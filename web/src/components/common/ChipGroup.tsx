import { cn } from "@/lib/utils"

export function ChipGroup<T extends string>({
  value,
  onChange,
  options,
  label,
  disabled,
  className,
}: {
  value: T
  onChange: (value: T) => void
  options: readonly (readonly [T, React.ReactNode])[]
  label: string
  disabled?: boolean
  className?: string
}) {
  return (
    <div className={cn("flex flex-wrap gap-1.5", className)} role="group" aria-label={label}>
      {options.map(([v, text]) => (
        <button
          key={v}
          type="button"
          disabled={disabled}
          aria-pressed={value === v}
          onClick={() => onChange(v)}
          className={cn(
            "inline-flex h-[30px] shrink-0 items-center rounded-lg border px-2.5 text-body outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-60",
            value === v
              ? "border-foreground bg-surface font-medium text-foreground"
              : "border-input bg-background text-muted-foreground hover:border-faint hover:text-foreground"
          )}
        >
          {text}
        </button>
      ))}
    </div>
  )
}

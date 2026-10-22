import { look } from "@/features/inbox/PageFrame"
import { cn } from "@/lib/utils"

export function ChoiceChips<T extends string>({
  value,
  onChange,
  options,
  label,
  className,
  disabled,
}: {
  value: T
  onChange: (value: T) => void
  options: readonly (readonly [T, React.ReactNode])[]
  label: string
  className?: string
  disabled?: boolean
}) {
  return (
    <div role="group" aria-label={label} className={cn("flex flex-wrap gap-1.5", className)}>
      {options.map(([v, text]) => (
        <button
          key={v}
          type="button"
          aria-pressed={v === value}
          disabled={disabled}
          onClick={() => onChange(v)}
          className={cn(v === value ? look.chipOn : look.chip, "disabled:opacity-50")}
        >
          {text}
        </button>
      ))}
    </div>
  )
}

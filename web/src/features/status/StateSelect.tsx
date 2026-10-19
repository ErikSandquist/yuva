import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { COMPONENT_STATES, StateDot, useStatusText } from "@/features/status/text"
import type { StatusComponentStateValue } from "@/lib/api"
import { cn } from "@/lib/utils"

export function StateSelect({
  value,
  onChange,
  label,
  disabled,
  className,
  testId,
}: {
  value: StatusComponentStateValue
  onChange: (v: StatusComponentStateValue) => void
  label: string
  disabled?: boolean
  className?: string
  testId?: string
}) {
  const text = useStatusText()
  return (
    <Select
      value={value}
      items={text.component}
      onValueChange={(v) => v && onChange(v as StatusComponentStateValue)}
      disabled={disabled}
    >
      <SelectTrigger
        aria-label={label}
        size="sm"
        className={cn("min-w-0 bg-background", className)}
        data-testid={testId}
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent className="min-w-52">
        {COMPONENT_STATES.map((s) => (
          <SelectItem key={s} value={s}>
            <StateDot state={s} className="mt-1.5" />
            {text.component[s]}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

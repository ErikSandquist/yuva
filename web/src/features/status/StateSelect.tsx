import { ChevronDownIcon } from "lucide-react"

import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
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

const pillTone: Record<StatusComponentStateValue, string> = {
  operational: "border-success/35 text-success",
  degraded: "border-warning/40 text-warning",
  partial_outage: "border-warning/40 text-warning",
  major_outage: "border-destructive/35 text-destructive",
  maintenance: "border-blue-600/35 text-blue-600 dark:border-blue-400/35 dark:text-blue-400",
}

export function StatePill({
  value,
  onChange,
  label,
  disabled,
  testId,
}: {
  value: StatusComponentStateValue
  onChange: (v: StatusComponentStateValue) => void
  label: string
  disabled?: boolean
  testId?: string
}) {
  const text = useStatusText()
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        disabled={disabled}
        aria-label={label}
        className={cn(
          "inline-flex h-7 items-center gap-1.5 rounded-full border bg-background ps-2.5 pe-2 text-caption font-medium whitespace-nowrap outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50",
          pillTone[value]
        )}
        data-testid={testId}
      >
        {text.component[value]}
        <ChevronDownIcon className="size-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-52">
        {COMPONENT_STATES.map((s) => (
          <DropdownMenuItem key={s} onClick={() => s !== value && onChange(s)}>
            <StateDot state={s} />
            {text.component[s]}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

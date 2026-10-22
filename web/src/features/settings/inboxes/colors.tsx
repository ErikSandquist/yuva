import { Trans, useLingui } from "@lingui/react/macro"

import { cn } from "@/lib/utils"

export const PRODUCT_COLORS = ["#c2410c", "#2563eb", "#7c3aed", "#0891b2", "#15803d", "#be185d", "#52525b", "#a16207"]

export function ColorPicker({
  value,
  onChange,
  disabled,
}: {
  value: string
  onChange: (color: string) => void
  disabled?: boolean
}) {
  const { t } = useLingui()
  const custom = !PRODUCT_COLORS.includes(value)
  return (
    <div className="flex flex-wrap items-center gap-2" role="group" aria-label={t`Colour`}>
      {PRODUCT_COLORS.map((c) => (
        <button
          key={c}
          type="button"
          disabled={disabled}
          aria-label={c}
          aria-pressed={value === c}
          onClick={() => onChange(c)}
          className={cn(
            "size-7 rounded-full border-2 border-transparent outline-none transition-shadow focus-visible:ring-2 focus-visible:ring-ring",
            value === c && "ring-2 ring-foreground ring-offset-2 ring-offset-background"
          )}
          style={{ backgroundColor: c }}
        />
      ))}
      <label
        className={cn(
          "inline-flex h-7 cursor-pointer items-center gap-1.5 rounded-full border px-2 text-caption text-muted-foreground",
          custom && "border-foreground text-foreground"
        )}
      >
        <input
          type="color"
          value={value}
          disabled={disabled}
          onChange={(e) => onChange(e.target.value)}
          className="size-4 cursor-pointer rounded border-0 bg-transparent p-0"
        />
        <Trans>Custom</Trans>
      </label>
    </div>
  )
}

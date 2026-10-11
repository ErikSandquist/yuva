import { Trans, useLingui } from "@lingui/react/macro"
import { XIcon } from "lucide-react"
import { useId, useState } from "react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Slider } from "@/components/ui/slider"
import { COLOR_RE, ratio } from "@/features/designs/colors"
import { validUrl } from "@/features/designs/doc"
import { cn } from "@/lib/utils"

export function Group({
  title,
  children,
  action,
}: {
  title: React.ReactNode
  children: React.ReactNode
  action?: React.ReactNode
}) {
  return (
    <section className="flex flex-col gap-3 border-b px-4 py-4 last:border-b-0">
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-small font-medium text-faint">{title}</h3>
        {action}
      </div>
      {children}
    </section>
  )
}

export function Row({
  label,
  htmlFor,
  hint,
  error,
  children,
  className,
}: {
  label: React.ReactNode
  htmlFor?: string
  hint?: React.ReactNode
  error?: React.ReactNode
  children: React.ReactNode
  className?: string
}) {
  return (
    <div className={cn("flex min-w-0 flex-col gap-1.5", className)}>
      <Label htmlFor={htmlFor} className="text-small">
        {label}
      </Label>
      {children}
      {error ? (
        <p role="alert" className="text-caption text-destructive">
          {error}
        </p>
      ) : (
        hint && <p className="text-caption text-faint">{hint}</p>
      )}
    </div>
  )
}

export function ContrastBadge({ fg, bg }: { fg: string; bg: string }) {
  const { t } = useLingui()
  const r = ratio(fg, bg)
  const ok = r >= 4.5
  const value = r.toFixed(1)
  return (
    <span
      className={cn(
        "shrink-0 rounded-full px-1.5 py-px text-[11px] font-medium tabular-nums",
        ok ? "bg-success/12 text-success" : "bg-warning/15 text-warning"
      )}
      title={ok ? t`Easy to read: ${value}:1` : t`Hard to read: ${value}:1, aim for 4.5:1`}
    >
      {value}
    </span>
  )
}

export function ColorInput({
  label,
  value,
  onChange,
  placeholder,
  clearable,
  contrastWith,
  testId,
}: {
  label: React.ReactNode
  value: string | undefined
  onChange: (v: string | undefined) => void
  placeholder?: string
  clearable?: boolean
  contrastWith?: string
  testId?: string
}) {
  const { t } = useLingui()
  const id = useId()
  const [draft, setDraft] = useState<string | null>(null)
  const shown = draft ?? value ?? ""
  const invalid = shown !== "" && !COLOR_RE.test(shown)
  const swatch = value ?? (placeholder || undefined)
  return (
    <div className="flex items-center gap-2">
      <label
        className="relative size-8 shrink-0 cursor-pointer overflow-hidden rounded-lg border shadow-[inset_0_0_0_1px_rgb(0_0_0/0.06)]"
        style={{ background: swatch ?? "transparent" }}
        title={t`Pick a colour`}
      >
        <input
          type="color"
          className="absolute inset-0 cursor-pointer opacity-0"
          value={(swatch ?? "#ffffff").toLowerCase()}
          onChange={(e) => {
            setDraft(null)
            onChange(e.target.value)
          }}
          aria-label={t`Pick a colour`}
        />
        {!value && placeholder && (
          <span className="absolute inset-0 bg-[repeating-linear-gradient(45deg,rgb(255_255_255/0.5)_0_3px,transparent_3px_6px)]" />
        )}
        {!swatch && (
          <span className="absolute inset-0 bg-[linear-gradient(135deg,transparent_46%,var(--destructive)_46%,var(--destructive)_54%,transparent_54%)] opacity-60" />
        )}
      </label>
      <Label htmlFor={id} className="min-w-0 flex-1 truncate text-small font-normal">
        {label}
      </Label>
      {contrastWith && (value ?? placeholder) && <ContrastBadge fg={(value ?? placeholder)!} bg={contrastWith} />}
      <Input
        id={id}
        value={shown}
        placeholder={placeholder}
        maxLength={7}
        spellCheck={false}
        className="h-8 w-[5.75rem] shrink-0 px-2 font-mono text-small uppercase"
        aria-invalid={invalid || undefined}
        onChange={(e) => {
          let v = e.target.value.trim()
          if (v && !v.startsWith("#")) v = `#${v}`
          setDraft(v)
          if (COLOR_RE.test(v)) onChange(v.toLowerCase())
          else if (v === "" && clearable) onChange(undefined)
        }}
        onBlur={() => setDraft(null)}
        data-testid={testId}
      />
      {clearable && (
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label={t`Use the default`}
          className={cn(!value && "invisible")}
          onClick={() => onChange(undefined)}
        >
          <XIcon />
        </Button>
      )}
    </div>
  )
}

export function SliderRow({
  label,
  value,
  min,
  max,
  step = 1,
  unit = "px",
  onChange,
  testId,
}: {
  label: React.ReactNode
  value: number
  min: number
  max: number
  step?: number
  unit?: string
  onChange: (v: number) => void
  testId?: string
}) {
  const id = useId()
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between gap-2">
        <Label htmlFor={id} className="text-small">
          {label}
        </Label>
        <span className="text-small text-muted-foreground tabular-nums">
          {value}
          {unit}
        </span>
      </div>
      <Slider
        id={id}
        value={[value]}
        min={min}
        max={max}
        step={step}
        onValueChange={(v) => onChange(Array.isArray(v) ? v[0] : v)}
        data-testid={testId}
      />
    </div>
  )
}

export function NumberBox({
  label,
  value,
  min,
  max,
  onChange,
  placeholder,
}: {
  label: string
  value: number | undefined
  min: number
  max: number
  onChange: (v: number) => void
  placeholder?: number
}) {
  return (
    <label className="flex min-w-0 flex-col gap-1">
      <span className="text-caption text-faint">{label}</span>
      <Input
        type="number"
        inputMode="numeric"
        min={min}
        max={max}
        value={value ?? ""}
        placeholder={placeholder === undefined ? undefined : String(placeholder)}
        className="h-8 px-2 text-small tabular-nums"
        onChange={(e) => {
          const n = Math.round(Number(e.target.value))
          if (e.target.value !== "" && Number.isFinite(n)) onChange(Math.min(max, Math.max(min, n)))
        }}
      />
    </label>
  )
}

export function UrlInput({
  label,
  value,
  onChange,
  required,
  testId,
}: {
  label: React.ReactNode
  value: string | undefined
  onChange: (v: string | undefined) => void
  required?: boolean
  testId?: string
}) {
  const id = useId()
  const [draft, setDraft] = useState<string | null>(null)
  const shown = draft ?? value ?? ""
  const invalid = shown !== "" && !validUrl(shown)
  const missing = required && shown === ""
  return (
    <Row
      label={label}
      htmlFor={id}
      error={
        invalid ? (
          <Trans>Links start with https://, http://, mailto: or tel:.</Trans>
        ) : missing ? (
          <Trans>A button needs a link.</Trans>
        ) : undefined
      }
    >
      <Input
        id={id}
        value={shown}
        placeholder="https://"
        inputMode="url"
        spellCheck={false}
        aria-invalid={invalid || missing || undefined}
        onChange={(e) => {
          const v = e.target.value
          setDraft(v)
          if (validUrl(v)) onChange(v.trim())
          else if (v === "" && !required) onChange(undefined)
        }}
        onBlur={() => {
          if (draft !== null && (validUrl(draft) || (draft === "" && !required))) setDraft(null)
        }}
        data-testid={testId}
      />
    </Row>
  )
}

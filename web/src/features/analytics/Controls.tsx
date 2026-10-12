import { Trans, useLingui } from "@lingui/react/macro"
import { CalendarRangeIcon, ChevronDownIcon, GitCompareArrowsIcon, XIcon } from "lucide-react"
import { useState } from "react"

import { formatDate } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { OPS, PERIODS, todayIso, useView, type FilterOp, type Period } from "@/features/analytics/state"
import { flag, useAnalyticsText, valueText } from "@/features/analytics/text"
import type { AnalyticsFilter } from "@/lib/api"

const CUSTOM = "custom"

function RangeDialog({
  open,
  onOpenChange,
  title,
  initial,
  onPick,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: React.ReactNode
  initial: { from: string; to: string }
  onPick: (r: { from: string; to: string }) => void
}) {
  const [from, setFrom] = useState(initial.from)
  const [to, setTo] = useState(initial.to)
  const valid = !!from && !!to && from <= to
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent data-testid="analytics-range-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            if (!valid) return
            onPick({ from, to })
            onOpenChange(false)
          }}
        >
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-2">
              <Label htmlFor="range-from">
                <Trans>From</Trans>
              </Label>
              <Input id="range-from" type="date" value={from} max={to} onChange={(e) => setFrom(e.target.value)} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="range-to">
                <Trans>To</Trans>
              </Label>
              <Input id="range-to" type="date" value={to} min={from} onChange={(e) => setTo(e.target.value)} />
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              <Trans>Cancel</Trans>
            </Button>
            <Button type="submit" disabled={!valid}>
              <Trans>Show</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function daysAgo(n: number) {
  const d = new Date(Date.parse(`${todayIso()}T00:00:00Z`) - n * 86_400_000)
  return d.toISOString().slice(0, 10)
}

export function RangePicker() {
  const { t, i18n } = useLingui()
  const text = useAnalyticsText()
  const { view, setRange } = useView()
  const [custom, setCustom] = useState(false)
  const r = view.range
  const customLabel = r.period
    ? t`Custom range`
    : `${formatDate(`${r.from}T12:00:00Z`, i18n.locale)} – ${formatDate(`${r.to}T12:00:00Z`, i18n.locale)}`
  const items: Record<string, string> = { ...text.period, [CUSTOM]: customLabel }
  return (
    <>
      <Select
        value={r.period ?? CUSTOM}
        items={items}
        onValueChange={(v) => {
          if (v === CUSTOM) setCustom(true)
          else if (v) setRange({ period: v as Period })
        }}
      >
        <SelectTrigger aria-label={t`Range`} className="max-w-full min-w-0" data-testid="analytics-range">
          <CalendarRangeIcon className="text-faint" />
          <SelectValue />
        </SelectTrigger>
        <SelectContent className="min-w-52">
          {PERIODS.map((p) => (
            <SelectItem key={p} value={p}>
              {text.period[p]}
            </SelectItem>
          ))}
          <SelectItem value={CUSTOM} onClick={() => setCustom(true)}>
            <Trans>Custom range…</Trans>
          </SelectItem>
        </SelectContent>
      </Select>
      {custom && (
        <RangeDialog
          open
          onOpenChange={setCustom}
          title={<Trans>Show the days</Trans>}
          initial={r.period ? { from: daysAgo(29), to: todayIso() } : { from: r.from!, to: r.to! }}
          onPick={setRange}
        />
      )}
    </>
  )
}

export function ComparePicker() {
  const { t, i18n } = useLingui()
  const { view, setCompare } = useView()
  const [custom, setCustom] = useState(false)
  const c = view.compare
  const value = c?.with ?? "none"
  const items: Record<string, string> = {
    none: t`No comparison`,
    previous: t`Compare with the period before`,
    year: t`Compare with a year earlier`,
    range: c?.from
      ? t`Compare with ${formatDate(`${c.from}T12:00:00Z`, i18n.locale)} – ${formatDate(`${c.to}T12:00:00Z`, i18n.locale)}`
      : t`Compare with other days`,
  }
  return (
    <>
      <Select
        value={value}
        items={items}
        onValueChange={(v) => {
          if (v === "range") setCustom(true)
          else setCompare(v === "previous" || v === "year" ? { with: v } : undefined)
        }}
      >
        <SelectTrigger aria-label={t`Comparison`} className="max-w-full min-w-0" data-testid="analytics-compare">
          <GitCompareArrowsIcon className="text-faint" />
          <SelectValue />
        </SelectTrigger>
        <SelectContent className="min-w-60">
          <SelectItem value="none">{items.none}</SelectItem>
          <SelectItem value="previous">{items.previous}</SelectItem>
          <SelectItem value="year">{items.year}</SelectItem>
          <SelectItem value="range" onClick={() => setCustom(true)}>
            <Trans>Compare with other days…</Trans>
          </SelectItem>
        </SelectContent>
      </Select>
      {custom && (
        <RangeDialog
          open
          onOpenChange={setCustom}
          title={<Trans>Compare with the days</Trans>}
          initial={c?.from && c.to ? { from: c.from, to: c.to } : { from: daysAgo(59), to: daysAgo(30) }}
          onPick={(r) => setCompare({ with: "range", ...r })}
        />
      )}
    </>
  )
}

function FilterChip({ f, i }: { f: AnalyticsFilter; i: number }) {
  const { t, i18n } = useLingui()
  const text = useAnalyticsText()
  const { setFilter, removeFilter } = useView()
  const value = f.values[0] ?? ""
  const shown = valueText(text, f.dimension, value, i18n.locale)
  const label = `${text.dimension[f.dimension]} ${text.op[f.op]} ${shown}`
  return (
    <li
      className="flex h-7 shrink-0 items-center rounded-full border bg-card ps-1 text-small"
      data-testid="analytics-filter"
    >
      <DropdownMenu>
        <DropdownMenuTrigger
          className="flex h-full max-w-72 items-center gap-1 rounded-full ps-2 pe-1 outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={t`Change filter: ${label}`}
        >
          <span className="text-faint">{text.dimension[f.dimension]}</span>
          <span className="text-faint">{text.op[f.op]}</span>
          <span className="truncate font-medium" dir="auto">
            {f.dimension === "country" && flag(value)} {shown}
          </span>
          <ChevronDownIcon className="size-3.5 shrink-0 text-faint" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuRadioGroup value={f.op} onValueChange={(op) => setFilter(i, { ...f, op: op as FilterOp })}>
            {OPS.map((op) => (
              <DropdownMenuRadioItem key={op} value={op}>
                {text.op[op]}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={() => removeFilter(i)}>
            <XIcon />
            <Trans>Remove filter</Trans>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <button
        type="button"
        onClick={() => removeFilter(i)}
        className="grid size-6 place-items-center rounded-full text-faint outline-none hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
        aria-label={t`Remove filter: ${label}`}
      >
        <XIcon className="size-3.5" />
      </button>
    </li>
  )
}

export function FilterBar() {
  const { t } = useLingui()
  const { view, clearFilters } = useView()
  if (view.filters.length === 0) return null
  return (
    <div className="-mx-1 flex items-center gap-2 overflow-x-auto px-1 py-0.5" aria-label={t`Filters`} role="group">
      <ul className="flex items-center gap-1.5">
        {view.filters.map((f, i) => (
          <FilterChip key={`${f.dimension}${i}`} f={f} i={i} />
        ))}
      </ul>
      {view.filters.length > 1 && (
        <Button
          variant="ghost"
          size="sm"
          className="shrink-0"
          onClick={clearFilters}
          data-testid="analytics-clear-filters"
        >
          <Trans>Clear</Trans>
        </Button>
      )}
    </div>
  )
}

export function Controls() {
  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex flex-wrap items-center gap-2">
        <RangePicker />
        <ComparePicker />
      </div>
      <FilterBar />
    </div>
  )
}

import { Trans, useLingui } from "@lingui/react/macro"
import { createContext, Fragment, useCallback, useContext, useEffect, useId, useMemo, useRef, useState } from "react"

import { ErrorLine } from "@/components/common/ErrorLine"
import { toast } from "@/components/common/Toast"
import { Button } from "@/components/ui/button"

export function SaveBar({
  what,
  pending,
  blocked,
  error,
  onDiscard,
  onSave,
}: {
  what: string
  pending?: boolean
  blocked?: boolean
  error?: unknown
  onDiscard: () => void
  onSave: () => void
}) {
  const { t } = useLingui()
  return (
    <div
      role="region"
      aria-label={t`Unsaved changes`}
      className="sticky bottom-4 z-20 flex flex-wrap items-center gap-x-3 gap-y-2 rounded-xl border bg-card px-4 py-3 shadow-[0_8px_30px_-12px_rgb(15_23_42/0.35)]"
      data-testid="save-bar"
    >
      <span className="min-w-0 flex-1 basis-56 text-body">
        <Trans>You have unsaved changes to {what}.</Trans>
        <ErrorLine error={error} className="text-caption" />
      </span>
      <Button
        variant="outline"
        className="h-[34px] rounded-lg border-input px-3.5 font-normal"
        onClick={onDiscard}
        disabled={pending}
        data-testid="save-bar-discard"
      >
        <Trans>Discard changes</Trans>
      </Button>
      <Button
        className="h-[34px] rounded-lg px-3.5"
        onClick={onSave}
        disabled={pending || blocked}
        data-testid="save-bar-save"
      >
        <Trans>Save changes</Trans>
      </Button>
    </div>
  )
}

type Entry = { enabled: boolean; pending: boolean }
type Registry = { register: (id: string, entry: (Entry & { submit: () => void }) | null) => void }

const BarContext = createContext<Registry | null>(null)

const wait = (ms: number) => new Promise((r) => setTimeout(r, ms))

export function SaveBarProvider({ what, children }: { what: string; children: React.ReactNode }) {
  const [entries, setEntries] = useState<Record<string, Entry>>({})
  const submits = useRef(new Map<string, () => void>())
  const latest = useRef(entries)
  const [epoch, setEpoch] = useState(0)
  const [running, setRunning] = useState(false)
  useEffect(() => {
    latest.current = entries
  })
  const register = useCallback<Registry["register"]>((id, entry) => {
    if (!entry) {
      submits.current.delete(id)
      setEntries((prev) => {
        if (!(id in prev)) return prev
        const rest = { ...prev }
        delete rest[id]
        return rest
      })
      return
    }
    submits.current.set(id, entry.submit)
    setEntries((prev) =>
      prev[id]?.enabled === entry.enabled && prev[id]?.pending === entry.pending
        ? prev
        : { ...prev, [id]: { enabled: entry.enabled, pending: entry.pending } }
    )
  }, [])
  const value = useMemo(() => ({ register }), [register])
  const all = Object.values(entries)
  const dirty = all.some((e) => e.enabled)
  const pending = running || all.some((e) => e.pending)
  const save = async () => {
    setRunning(true)
    for (const id of Object.keys(latest.current).filter((x) => latest.current[x]?.enabled)) {
      submits.current.get(id)?.()
      await wait(30)
      for (let i = 0; i < 300 && latest.current[id]?.pending; i++) await wait(50)
    }
    setRunning(false)
    if (!Object.values(latest.current).some((e) => e.enabled)) toast(<Trans>Saved</Trans>)
  }
  return (
    <BarContext.Provider value={value}>
      <Fragment key={epoch}>{children}</Fragment>
      {dirty && (
        <SaveBar what={what} pending={pending} onDiscard={() => setEpoch((n) => n + 1)} onSave={() => void save()} />
      )}
    </BarContext.Provider>
  )
}

export function useSaveBarEntry(entry: { enabled: boolean; pending: boolean }) {
  const bar = useContext(BarContext)
  const id = useId()
  const button = useRef<HTMLButtonElement>(null)
  const { enabled, pending } = entry
  const register = bar?.register
  useEffect(() => {
    register?.(id, { enabled, pending, submit: () => button.current?.form?.requestSubmit() })
  }, [register, id, enabled, pending])
  useEffect(() => () => register?.(id, null), [register, id])
  return bar ? button : null
}

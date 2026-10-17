import { useCallback, useState } from "react"

import { useSession } from "@/lib/session"

type Draft<T> = T & { idempotencyKey: string }

function fresh<T extends object>(initial: T): Draft<T> {
  return { ...initial, idempotencyKey: crypto.randomUUID() }
}

function load<T extends object>(key: string, initial: T): Draft<T> {
  try {
    const raw = sessionStorage.getItem(key)
    if (raw) {
      const saved = JSON.parse(raw) as Partial<Draft<T>>
      if (typeof saved.idempotencyKey === "string") return { ...initial, ...saved } as Draft<T>
    }
  } catch {
    // storage unavailable or corrupt: start fresh
  }
  return fresh(initial)
}

function save(key: string, value: unknown) {
  try {
    sessionStorage.setItem(key, JSON.stringify(value))
  } catch {
    // storage unavailable: the wizard forgets on reload
  }
}

export function useWizardDraft<T extends object>(
  name: string,
  initial: T
): [Draft<T>, (patch: Partial<T>) => void, () => void] {
  const { membership } = useSession()
  const key = `yuva:wizard:${membership.workspace.id}:${name}`
  const [state, setState] = useState(() => ({ key, value: load(key, initial) }))
  const current = state.key === key ? state : { key, value: load(key, initial) }
  if (current !== state) setState(current)

  const patch = useCallback(
    (p: Partial<T>) =>
      setState((s) => {
        const value = { ...s.value, ...p }
        save(s.key, value)
        return { key: s.key, value }
      }),
    []
  )
  const clear = useCallback(() => {
    try {
      sessionStorage.removeItem(key)
    } catch {
      // nothing to clear
    }
    setState({ key, value: fresh(initial) })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key])

  return [current.value, patch, clear]
}

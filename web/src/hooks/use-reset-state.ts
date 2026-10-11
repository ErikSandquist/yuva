import { useState } from "react"

export function useResetState<T>(initial: T, key: unknown) {
  const [state, setState] = useState(initial)
  const [seen, setSeen] = useState(key)
  if (!Object.is(seen, key)) {
    setSeen(key)
    setState(initial)
  }
  return [Object.is(seen, key) ? state : initial, setState] as const
}

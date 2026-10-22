import { useSearchParams, type To } from "react-router"

export function useTab<T extends string>(tabs: readonly T[], fallback: T) {
  const [params] = useSearchParams()
  const raw = params.get("tab") ?? ""
  const tab = (tabs as readonly string[]).includes(raw) ? (raw as T) : fallback
  const to = (next: T): To => {
    const p = new URLSearchParams(params)
    if (next === fallback) p.delete("tab")
    else p.set("tab", next)
    const search = p.toString()
    return { search: search ? `?${search}` : "" }
  }
  return { tab, to }
}

export function signInPath(path: string, search: string): string {
  const next = path + search
  if (next === "/") return "/sign-in"
  if (path === "/" && new URLSearchParams(search).get("signup") === "1") return "/sign-in?signup=1"
  return `/sign-in?next=${encodeURIComponent(next)}`
}

export function safeNext(raw: string | null): string {
  if (!raw || !raw.startsWith("/") || raw.startsWith("//") || raw.startsWith("/\\")) return "/"
  try {
    const url = new URL(raw, window.location.origin)
    if (url.origin !== window.location.origin || url.pathname === "/sign-in") return "/"
    return url.pathname + url.search
  } catch {
    return "/"
  }
}

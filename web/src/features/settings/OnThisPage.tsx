import { useLingui } from "@lingui/react/macro"
import { useEffect } from "react"
import { Link, useLocation, useNavigate } from "react-router"

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

export function useHashScroll(ready: boolean) {
  const { hash } = useLocation()
  useEffect(() => {
    if (!ready || !hash) return
    document.getElementById(hash.slice(1))?.scrollIntoView({ block: "start" })
  }, [ready, hash])
}

export function OnThisPage({ items, testId }: { items: [string, string][]; testId: string }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { hash } = useLocation()
  const labels = Object.fromEntries(items)
  const current = items.some(([id]) => `#${id}` === hash) ? hash.slice(1) : items[0][0]
  return (
    <nav aria-label={t`On this page`} data-testid={testId}>
      <ul className="hidden flex-wrap gap-x-4 gap-y-1 sm:flex">
        {items.map(([id, label]) => (
          <li key={id}>
            <Link to={{ hash: id }} replace className="text-body text-muted-foreground hover:text-foreground">
              {label}
            </Link>
          </li>
        ))}
      </ul>
      <div className="sm:hidden">
        <Select value={current} onValueChange={(id) => id && navigate({ hash: id }, { replace: true })} items={labels}>
          <SelectTrigger className="w-full" aria-label={t`On this page`}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {items.map(([id, label]) => (
              <SelectItem key={id} value={id}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </nav>
  )
}

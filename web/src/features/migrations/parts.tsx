import { Trans, useLingui } from "@lingui/react/macro"
import { useEffect, useRef, useState } from "react"

import { ProgressBar } from "@/components/common"
import { formatDateTime } from "@/components/common/text"
import { useNumber } from "@/features/broadcasts/BroadcastParts"
import { percentOf } from "@/features/migrations/queries"
import { stateTones, useMigrationText } from "@/features/migrations/text"
import { StatusTag } from "@/features/settings/ui"
import type { Migration } from "@/lib/api"

export function isWaiting(m: Migration) {
  return !!m.waiting_until && (m.state === "copying" || m.state === "syncing" || m.state === "scanning")
}

export function MigrationStateTag({ m }: { m: Migration }) {
  const { t } = useLingui()
  const text = useMigrationText()
  const percent = percentOf(m)
  const label = isWaiting(m)
    ? t`Waiting`
    : m.state === "copying" && m.counts.total > 0
      ? t`Copying ${percent}%`
      : text.state[m.state]
  return (
    <StatusTag tone={stateTones[m.state]} data-testid="migration-state" data-state={m.state}>
      {label}
    </StatusTag>
  )
}

export function MigrationBar({ m, thin }: { m: Migration; thin?: boolean }) {
  const { t } = useLingui()
  const num = useNumber()
  const c = m.counts
  const done = num(c.imported + c.duplicate + c.skipped + c.failed)
  const total = num(c.total)
  const percent = percentOf(m)
  return (
    <ProgressBar
      thin={thin}
      value={c.total > 0 ? percent : undefined}
      label={t`Messages copied`}
      text={c.total > 0 ? t`${percent}%, ${done} of ${total} messages` : t`Starting`}
      testId="migration-progress"
    />
  )
}

export function WaitingLine({ m }: { m: Migration }) {
  const { i18n } = useLingui()
  if (!isWaiting(m) || !m.waiting_until) return null
  const time = formatDateTime(m.waiting_until, i18n.locale)
  return (
    <p className="text-small text-warning" data-testid="migration-waiting">
      <Trans>Waiting for the provider's limit; continues at {time}.</Trans>
    </p>
  )
}

export function useStateAnnouncement(items: Migration[]) {
  const text = useMigrationText()
  const seen = useRef<Map<string, string> | null>(null)
  const [message, setMessage] = useState("")
  useEffect(() => {
    const now = new Map(items.map((m) => [m.id, m.state]))
    const before = seen.current
    seen.current = now
    if (!before) return
    const changed = items.find((m) => before.has(m.id) && before.get(m.id) !== m.state)
    if (changed) setMessage(`${changed.source_address}: ${text.state[changed.state]}`)
  }, [items, text.state])
  return (
    <p className="sr-only" role="status" aria-live="polite">
      {message}
    </p>
  )
}

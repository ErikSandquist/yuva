import { Trans, useLingui } from "@lingui/react/macro"
import { HistoryIcon } from "lucide-react"
import { useEffect } from "react"
import { Link } from "react-router"

import { ProgressBar } from "@/components/common"
import { Alert } from "@/components/ui/alert"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

export function ImportBanner() {
  const { t } = useLingui()
  const { canManage } = useSession()
  const [inboxId] = useInboxFilter()
  const inboxes = useInboxes()
  const inbox = inboxes.data?.find((x) => x.import && x.import.state !== "syncing" && (!inboxId || x.id === inboxId))
  const copying = inbox?.import?.state === "copying"
  const { refetch } = inboxes
  useEffect(() => {
    if (!copying) return
    const id = setInterval(() => void refetch(), 30_000)
    return () => clearInterval(id)
  }, [copying, refetch])
  const imp = inbox?.import
  if (!imp) return null
  const address = imp.address
  const percent = imp.percent
  return (
    <Alert
      role="status"
      className="mx-3 mb-2 flex flex-col gap-1.5 rounded-xl border-border bg-card py-2 ps-3 pe-2 text-small text-muted-foreground"
      data-testid="import-banner"
    >
      <span className="flex items-start gap-2">
        <HistoryIcon className="mt-0.5 size-3.5 shrink-0" />
        <span className="min-w-0 flex-1">
          <Trans>
            Importing history from {address} · {percent}% · new mail keeps arriving as usual
          </Trans>
        </span>
        {canManage && (
          <Link
            to="/migrations"
            className="shrink-0 font-medium text-foreground underline-offset-2 hover:underline"
            data-testid="import-banner-details"
          >
            <Trans>Details</Trans>
          </Link>
        )}
      </span>
      <ProgressBar thin value={percent} label={t`History imported from ${address}`} text={t`${percent}%`} />
    </Alert>
  )
}

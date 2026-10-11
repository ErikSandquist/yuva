import { Trans } from "@lingui/react/macro"
import { MegaphoneIcon } from "lucide-react"
import { Link } from "react-router"

import { broadcastLink, useBroadcast } from "@/features/broadcasts/broadcastQueries"

export function BroadcastLine({ id }: { id: string }) {
  const broadcast = useBroadcast(id)
  const name = broadcast.data?.name
  return (
    <span
      className="inline-flex min-w-0 items-center gap-1 rounded-full bg-brand-wash px-2 py-px text-caption text-brand"
      data-testid="broadcast-reply-badge"
    >
      <MegaphoneIcon className="size-3 shrink-0" />
      {name ? (
        <Link to={broadcastLink(id)} className="min-w-0 truncate font-medium underline-offset-2 hover:underline">
          <Trans>Reply to {name}</Trans>
        </Link>
      ) : (
        <span className="truncate">
          <Trans>Reply to a broadcast</Trans>
        </span>
      )}
    </span>
  )
}

import { Trans } from "@lingui/react/macro"
import { Link } from "react-router"

import { FINISHED, isActive } from "@/features/broadcasts/broadcastQueries"
import type { Broadcast } from "@/lib/api"

export function BroadcastSteps() {
  return (
    <ol className="flex list-decimal flex-col gap-1 ps-5 text-start">
      <li>
        <Trans>
          A{" "}
          <Link to="/broadcasts/lists" className="text-brand underline-offset-4 hover:underline">
            mailing list
          </Link>{" "}
          holds the people who agreed to get your mail.
        </Trans>
      </li>
      <li>
        <Trans>
          A{" "}
          <Link to="/broadcasts/designs" className="text-brand underline-offset-4 hover:underline">
            design
          </Link>{" "}
          is how the mail looks.
        </Trans>
      </li>
      <li>
        <Trans>A broadcast sends a design to one or more lists, now or later, and reports what happened.</Trans>
      </li>
    </ol>
  )
}

export function broadcastWhen(b: Broadcast) {
  if (b.state === "scheduled") return b.scheduled_at
  if (FINISHED.includes(b.state)) return b.finished_at ?? b.started_at ?? b.updated_at
  if (isActive(b)) return b.started_at ?? b.scheduled_at ?? b.updated_at
  return b.updated_at
}

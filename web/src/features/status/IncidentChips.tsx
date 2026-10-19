import { useLingui } from "@lingui/react/macro"
import { CalendarClockIcon, FlameIcon, XIcon } from "lucide-react"
import { Link } from "react-router"

import { toast } from "@/components/common"
import { useErrorText } from "@/components/common/text"
import { useLinkMutations } from "@/features/status/linked"
import { incidentLink } from "@/features/status/queries"
import { useStatusText } from "@/features/status/text"
import type { ConversationIncident } from "@/lib/api"

export function IncidentChips({ conversationId, items }: { conversationId: string; items: ConversationIncident[] }) {
  const { t } = useLingui()
  const text = useStatusText()
  const errorText = useErrorText()
  const { unlink } = useLinkMutations()
  return (
    <>
      {items.map((i) => {
        const Icon = i.kind === "maintenance" ? CalendarClockIcon : FlameIcon
        const title = i.title
        const page = i.page.name
        return (
          <span
            key={i.id}
            className="inline-flex max-w-full items-center rounded-full border bg-card text-caption text-muted-foreground"
            data-testid="incident-chip"
          >
            <Link
              to={incidentLink(i.page.id, i.id)}
              className="inline-flex min-w-0 items-center gap-1 rounded-s-full py-px ps-2 pe-1 outline-none hover:text-foreground focus-visible:text-foreground"
              title={`${page} · ${text.incident[i.state]}`}
            >
              <Icon className="size-3 shrink-0" />
              <span className="truncate">{i.title}</span>
            </Link>
            <button
              type="button"
              onClick={() =>
                unlink.mutate(
                  { pageId: i.page.id, incidentId: i.id, conversationId },
                  { onError: (e) => toast(errorText(e)) }
                )
              }
              disabled={unlink.isPending}
              className="grid size-5 shrink-0 place-items-center rounded-e-full outline-none hover:text-foreground focus-visible:text-foreground"
              aria-label={t`Unlink from ${title}`}
              title={t`Unlink from ${title}`}
              data-testid="incident-chip-unlink"
            >
              <XIcon className="size-3" />
            </button>
          </span>
        )
      })}
    </>
  )
}

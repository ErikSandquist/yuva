import { Dot } from "@/components/common/Dot"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

export function InboxBadges({
  inboxes,
  max,
  className,
}: {
  inboxes: { id: string; name: string }[]
  max?: number
  className?: string
}) {
  const all = useInboxes().data ?? []
  if (inboxes.length === 0) return null
  const shown = max ? inboxes.slice(0, max) : inboxes
  const rest = inboxes.slice(shown.length)
  return (
    <span className={cn("flex min-w-0 flex-wrap gap-1", className)} data-testid="inbox-badges">
      {shown.map((i) => (
        <span
          key={i.id}
          className="inline-flex max-w-36 items-center gap-1.5 rounded-md bg-muted px-1.5 py-0.5 text-caption text-muted-foreground"
        >
          <Dot color={all.find((a) => a.id === i.id)?.branding.color} className="size-1.5 rounded-[2px]" />
          <span className="truncate">{i.name}</span>
        </span>
      ))}
      {rest.length > 0 && (
        <span
          className="inline-flex items-center rounded-md bg-muted px-1.5 py-0.5 text-caption text-muted-foreground"
          title={rest.map((i) => i.name).join(", ")}
        >
          +{rest.length}
        </span>
      )}
    </span>
  )
}

import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

export function useProductTag() {
  const inboxes = useInboxes().data ?? []
  const [filter] = useInboxFilter()
  const byId = new Map(inboxes.map((i) => [i.id, i]))
  return inboxes.length > 1 && !filter ? (id: string) => byId.get(id) : () => undefined
}

export function ProductTag({ id, className }: { id: string; className?: string }) {
  const inbox = useProductTag()(id)
  if (!inbox) return null
  const color = inbox.branding.color
  return (
    <span
      className={cn(
        "inline-flex max-w-32 min-w-0 shrink-0 items-center rounded-md bg-muted px-1.5 py-px text-caption font-medium text-muted-foreground",
        className
      )}
      style={
        color
          ? {
              color: `color-mix(in oklab, ${color} 80%, var(--foreground))`,
              backgroundColor: `color-mix(in srgb, ${color} 10%, transparent)`,
            }
          : undefined
      }
      data-testid="product-tag"
    >
      <span className="truncate">{inbox.name}</span>
    </span>
  )
}

import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

export function useProductTag() {
  const inboxes = useInboxes().data ?? []
  const [filter] = useInboxFilter()
  const byId = new Map(inboxes.map((i) => [i.id, i]))
  return inboxes.length > 1 && !filter ? (id: string) => byId.get(id) : () => undefined
}

export function ProductTag({ id, className, always }: { id: string; className?: string; always?: boolean }) {
  const shown = useProductTag()(id)
  const any = useInboxes().data?.find((i) => i.id === id)
  const inbox = always ? any : shown
  if (!inbox) return null
  const color = inbox.branding.color
  return (
    <span
      className={cn(
        "inline-flex h-[18px] max-w-32 min-w-0 shrink-0 items-center rounded-[4px] border border-input px-1.5 text-[11px] leading-4 font-medium text-muted-foreground",
        className
      )}
      style={
        color
          ? {
              color: `color-mix(in oklab, ${color} 85%, var(--foreground))`,
              borderColor: `color-mix(in srgb, ${color} 45%, transparent)`,
            }
          : undefined
      }
      data-testid="product-tag"
    >
      <span className="truncate">{inbox.name}</span>
    </span>
  )
}

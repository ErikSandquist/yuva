import { cn } from "@/lib/utils"

export function InboxMark({ name, color, className }: { name: string; color?: string; className?: string }) {
  return (
    <span
      aria-hidden
      className={cn(
        "grid size-5 shrink-0 place-items-center rounded-md bg-faint text-[11px] leading-none font-semibold text-white uppercase",
        className
      )}
      style={color ? { backgroundColor: color } : undefined}
    >
      {Array.from(name.trim())[0] ?? ""}
    </span>
  )
}

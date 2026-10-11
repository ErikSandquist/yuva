import { useLingui } from "@lingui/react/macro"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import type { DiscussionAuthor } from "@/lib/api"
import { cn } from "@/lib/utils"

export function useAuthorName() {
  const { t } = useLingui()
  return (a: DiscussionAuthor) => {
    if (a.deleted) return a.kind === "member" ? t`Deleted member` : t`Deleted reader`
    if (a.kind === "anonymous") return t`Anonymous`
    return a.name || (a.kind === "member" ? t`Team` : t`Reader`)
  }
}

export function InitialsAvatar({
  initials,
  team,
  className,
}: {
  initials: string
  team?: boolean
  className?: string
}) {
  return (
    <Avatar
      aria-hidden
      className={cn(
        "@container size-8 shrink-0 items-center justify-center font-medium after:hidden",
        team ? "bg-mate text-white" : "bg-muted text-muted-foreground",
        className
      )}
    >
      <AvatarFallback className="bg-transparent text-[length:38cqw] font-[inherit] text-inherit">
        {initials || "·"}
      </AvatarFallback>
    </Avatar>
  )
}

export function AuthorAvatar({ author, className }: { author: DiscussionAuthor; className?: string }) {
  return (
    <InitialsAvatar
      initials={author.initials}
      team={author.kind === "member" && !author.deleted}
      className={className}
    />
  )
}

"use client"

import * as React from "react"
import type { Discussion, DiscussionSort } from "useyuva"
import { CheckCircle2, ChevronUp, Lock, MessageSquare } from "lucide-react"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useDiscussions, useYuvaDiscussionsContext, useYuvaLabels } from "@/hooks/use-yuva-discussions"
import { cn } from "@/lib/utils"
import { relativeTime, yuvaErrorMessage, type YuvaDiscussionsLabels } from "@/lib/yuva-utils"

export interface YuvaDiscussionListProps {
  page: string | null | undefined
  sort?: DiscussionSort
  onSortChange?(sort: DiscussionSort): void
  onSelect(discussion: Discussion): void
  labels?: Partial<YuvaDiscussionsLabels>
  className?: string
}

export function YuvaDiscussionList({
  page,
  sort = "new",
  onSortChange,
  onSelect,
  labels: overrides,
  className,
}: YuvaDiscussionListProps) {
  const labels = useYuvaLabels(overrides)
  const { locale } = useYuvaDiscussionsContext()
  const list = useDiscussions(page, sort)
  const sorts: [DiscussionSort, string][] = [
    ["new", labels.sortNew],
    ["top", labels.sortTop],
    ["unanswered", labels.sortUnanswered],
  ]

  return (
    <div className={cn("flex flex-col gap-3", className)}>
      {onSortChange && (
        <Tabs value={sort} onValueChange={(value) => onSortChange(value as DiscussionSort)}>
          <TabsList aria-label={labels.heading}>
            {sorts.map(([value, label]) => (
              <TabsTrigger key={value} value={value}>
                {label}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      )}

      {list.error && list.discussions.length === 0 ? (
        <div role="alert" className="flex flex-col items-start gap-2 rounded-lg border p-4 text-sm">
          <p className="text-destructive">{yuvaErrorMessage(list.error, labels) ?? labels.loadError}</p>
          <Button variant="outline" size="sm" onClick={() => void list.reload()}>
            {labels.retry}
          </Button>
        </div>
      ) : list.loading && list.discussions.length === 0 ? (
        <div className="flex flex-col gap-2" aria-busy>
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      ) : list.discussions.length === 0 ? (
        <p className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
          {sort === "unanswered" ? labels.emptyUnanswered : labels.empty}
        </p>
      ) : (
        <ul className="divide-y rounded-lg border" aria-busy={list.loading}>
          {list.discussions.map((discussion) => (
            <li key={discussion.id}>
              <button
                type="button"
                onClick={() => onSelect(discussion)}
                className="flex w-full items-start gap-3 p-3 text-start transition-colors outline-none hover:bg-muted/50 focus-visible:bg-muted/50 focus-visible:ring-[3px] focus-visible:ring-ring/50 sm:p-4"
              >
                <Avatar size="sm" aria-hidden className="mt-0.5">
                  <AvatarFallback>{discussion.author.deleted ? "?" : discussion.author.initials}</AvatarFallback>
                </Avatar>
                <span className="flex min-w-0 flex-1 flex-col gap-1">
                  <span className="font-medium break-words">{discussion.title}</span>
                  <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
                    <span>
                      {discussion.author.deleted
                        ? discussion.author.kind === "member"
                          ? labels.deletedMember
                          : labels.deletedAuthor
                        : discussion.author.name || labels.anonymous}
                    </span>
                    <time dateTime={discussion.last_activity_at}>
                      {relativeTime(discussion.last_activity_at, locale)}
                    </time>
                    {discussion.answer_post_id && (
                      <Badge variant="secondary" className="gap-1">
                        <CheckCircle2 aria-hidden />
                        {labels.answered}
                      </Badge>
                    )}
                    {discussion.locked && (
                      <Badge variant="outline" className="gap-1">
                        <Lock aria-hidden />
                        {labels.locked}
                      </Badge>
                    )}
                    {discussion.state === "pending" && <Badge variant="outline">{labels.pending}</Badge>}
                  </span>
                </span>
                <span className="flex shrink-0 flex-col items-end gap-1 text-xs text-muted-foreground tabular-nums">
                  <span className="flex items-center gap-1">
                    <ChevronUp className="size-3.5" aria-hidden />
                    <span aria-hidden>{discussion.votes}</span>
                    <span className="sr-only">{labels.votes(discussion.votes)}</span>
                  </span>
                  <span className="flex items-center gap-1">
                    <MessageSquare className="size-3.5" aria-hidden />
                    <span aria-hidden>{discussion.reply_count}</span>
                    <span className="sr-only">{labels.replies(discussion.reply_count)}</span>
                  </span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}

      {list.hasMore && (
        <Button variant="outline" onClick={() => void list.loadMore()} disabled={list.loading} className="self-center">
          {labels.loadMore}
        </Button>
      )}
    </div>
  )
}

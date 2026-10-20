"use client"

import * as React from "react"
import type { Discussion } from "useyuva"
import { Button } from "@/components/ui/button"
import { YuvaDiscussionComposer, YuvaSignIn } from "@/components/yuva-discussion-composer"
import { useIdeas, useYuvaRoadmapLabels } from "@/hooks/use-yuva-roadmap"
import { useYuvaDiscussionsContext, useYuvaReader } from "@/hooks/use-yuva-discussions"
import { cn } from "@/lib/utils"
import type { YuvaRoadmapLabels } from "@/lib/yuva-roadmap-utils"
import type { YuvaDiscussionsLabels } from "@/lib/yuva-utils"

export interface YuvaIdeaSuggestProps {
  labels?: Partial<YuvaRoadmapLabels>
  discussionLabels?: Partial<YuvaDiscussionsLabels>
  className?: string
  onSuggested?(idea: Discussion): void
  onSelect?(idea: Discussion): void
  onCancel?(): void
}

function SimilarIdeas({
  title,
  labels,
  onSelect,
}: {
  title: string
  labels: YuvaRoadmapLabels
  onSelect?(idea: Discussion): void
}) {
  const [query, setQuery] = React.useState("")
  React.useEffect(() => {
    const q = title.trim()
    const timer = setTimeout(() => setQuery(q.length >= 3 ? q : ""), 300)
    return () => clearTimeout(timer)
  }, [title])
  const similar = useIdeas(undefined, "top", query || undefined)
  if (!query || similar.ideas.length === 0) return null
  return (
    <div className="flex flex-col gap-2 rounded-lg border p-3">
      <p className="text-sm font-medium">{labels.similarIdeas}</p>
      <ul className="flex flex-col gap-1">
        {similar.ideas.slice(0, 3).map((idea) => (
          <li key={idea.id}>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-auto w-full justify-between gap-3 py-1.5 text-start whitespace-normal"
              onClick={() => onSelect?.(idea)}
            >
              <span className="min-w-0 break-words">{idea.title}</span>
              <span className="shrink-0 text-xs text-muted-foreground tabular-nums">{labels.votes(idea.votes)}</span>
            </Button>
          </li>
        ))}
      </ul>
    </div>
  )
}

export function YuvaIdeaSuggest({
  labels: overrides,
  discussionLabels,
  className,
  onSuggested,
  onSelect,
  onCancel,
}: YuvaIdeaSuggestProps) {
  const labels = useYuvaRoadmapLabels(overrides)
  const { client } = useYuvaDiscussionsContext()
  const { writer } = useYuvaReader()
  const [title, setTitle] = React.useState("")
  const [sent, setSent] = React.useState<Discussion | null>(null)

  if (sent) {
    return (
      <div className={cn("flex flex-col items-start gap-3", className)}>
        <p role="status" className="text-sm">
          {sent.state === "pending" ? labels.ideaPending : labels.ideaSent}
        </p>
        {onCancel && (
          <Button type="button" variant="outline" size="sm" onClick={onCancel}>
            {labels.back}
          </Button>
        )}
      </div>
    )
  }

  return (
    <div className={cn("flex flex-col gap-4", className)}>
      <h3 className="text-base font-semibold">{labels.suggestHeading}</h3>
      {writer ? (
        <>
          <SimilarIdeas title={title} labels={labels} onSelect={onSelect} />
          <YuvaDiscussionComposer
            withTitle
            autoFocus
            submitLabel={labels.sendIdea}
            labels={{
              titlePlaceholder: labels.suggestTitlePlaceholder,
              bodyPlaceholder: labels.suggestBodyPlaceholder,
              ...discussionLabels,
            }}
            onTitleChange={setTitle}
            onCancel={onCancel}
            onSubmit={async ({ title: heading, body }) => {
              if (!client) return
              const idea = await client.suggestIdea({ title: heading, body })
              setSent(idea)
              onSuggested?.(idea)
            }}
          />
        </>
      ) : (
        <YuvaSignIn labels={discussionLabels} />
      )}
    </div>
  )
}

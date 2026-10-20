"use client"

import * as React from "react"
import type { Discussion, IdentityTokenSource, YuvaClient } from "useyuva"
import { ChevronUp, Lightbulb, MessageSquare, Plus } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { YuvaIdea } from "@/components/yuva-idea"
import { YuvaIdeaSuggest } from "@/components/yuva-idea-suggest"
import { YuvaDiscussionsProvider, useYuvaDiscussionsContext, useYuvaLabels } from "@/hooks/use-yuva-discussions"
import { useIdeas, useRoadmap, useYuvaRoadmapLabels } from "@/hooks/use-yuva-roadmap"
import { cn } from "@/lib/utils"
import {
  ideaStageLabel,
  ideaVotingClosed,
  yuvaRoadmapStages,
  type YuvaRoadmapLabels,
  type YuvaRoadmapStage,
} from "@/lib/yuva-roadmap-utils"
import { yuvaErrorMessage, type YuvaDiscussionsLabels } from "@/lib/yuva-utils"

export interface YuvaRoadmapListProps {
  stage?: YuvaRoadmapStage
  onStageChange?(stage: YuvaRoadmapStage): void
  onSelect(idea: Discussion): void
  onSuggest?(): void
  labels?: Partial<YuvaRoadmapLabels>
  discussionLabels?: Partial<YuvaDiscussionsLabels>
  className?: string
}

export function YuvaRoadmapList({
  stage = "ideas",
  onStageChange,
  onSelect,
  onSuggest,
  labels: overrides,
  discussionLabels,
  className,
}: YuvaRoadmapListProps) {
  const labels = useYuvaRoadmapLabels(overrides)
  const discussions = useYuvaLabels(discussionLabels)
  const { board } = useRoadmap()
  const list = useIdeas(stage, "top")
  const [error, setError] = React.useState<string | null>(null)
  const suggestions = board ? board.suggestions !== "off" : false

  return (
    <div className={cn("flex flex-col gap-3", className)}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Tabs value={stage} onValueChange={(value) => onStageChange?.(value as YuvaRoadmapStage)}>
          <TabsList aria-label={labels.heading}>
            {yuvaRoadmapStages.map((value) => (
              <TabsTrigger key={value} value={value}>
                {ideaStageLabel(value, labels)}
                {board && (
                  <span className="ms-1.5 text-xs text-muted-foreground tabular-nums">{board.stages[value]}</span>
                )}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        {suggestions && onSuggest && (
          <Button size="sm" onClick={onSuggest}>
            <Plus data-icon="inline-start" />
            {labels.suggest}
          </Button>
        )}
      </div>

      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}

      {list.error && list.ideas.length === 0 ? (
        <div role="alert" className="flex flex-col items-start gap-2 rounded-lg border p-4 text-sm">
          <p className="text-destructive">{yuvaErrorMessage(list.error, discussions) ?? discussions.loadError}</p>
          <Button variant="outline" size="sm" onClick={() => void list.reload()}>
            {discussions.retry}
          </Button>
        </div>
      ) : list.loading && list.ideas.length === 0 ? (
        <div className="flex flex-col gap-2" aria-busy>
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      ) : list.ideas.length === 0 ? (
        <p className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">{labels.empty}</p>
      ) : (
        <ul className="divide-y rounded-lg border" aria-busy={list.loading}>
          {list.ideas.map((idea) => {
            const closed = ideaVotingClosed(idea.stage)
            return (
              <li key={idea.id} className="flex items-start gap-3 p-3 sm:p-4">
                <Button
                  type="button"
                  variant={idea.voted ? "secondary" : "outline"}
                  aria-pressed={!!idea.voted}
                  aria-label={idea.voted ? labels.removeVote(idea.title) : labels.vote(idea.title)}
                  disabled={closed}
                  onClick={() =>
                    void list.vote(idea).then(
                      () => setError(null),
                      (failure) => setError(yuvaErrorMessage(failure, discussions))
                    )
                  }
                  className={cn("h-auto min-w-12 flex-col gap-0.5 py-2 tabular-nums", idea.voted && "text-primary")}
                >
                  <ChevronUp aria-hidden />
                  {idea.votes}
                </Button>
                <button
                  type="button"
                  onClick={() => onSelect(idea)}
                  className="flex min-w-0 flex-1 flex-col gap-1 text-start outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
                >
                  <span className="font-medium break-words">{idea.title}</span>
                  <span className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                    {idea.state === "pending" && <Badge variant="outline">{discussions.pending}</Badge>}
                    <span className="flex items-center gap-1">
                      <MessageSquare className="size-3.5" aria-hidden />
                      <span aria-hidden>{idea.reply_count}</span>
                      <span className="sr-only">{discussions.replies(idea.reply_count)}</span>
                    </span>
                  </span>
                </button>
              </li>
            )
          })}
        </ul>
      )}

      {list.hasMore && (
        <Button variant="outline" onClick={() => void list.loadMore()} disabled={list.loading} className="self-center">
          {discussions.loadMore}
        </Button>
      )}
    </div>
  )
}

export interface YuvaRoadmapProps {
  server?: string
  channel?: string
  identityToken?: IdentityTokenSource | null
  client?: YuvaClient
  defaultStage?: YuvaRoadmapStage
  labels?: Partial<YuvaRoadmapLabels>
  discussionLabels?: Partial<YuvaDiscussionsLabels>
  locale?: string
  className?: string
}

export function YuvaRoadmap({
  server,
  channel,
  identityToken,
  client,
  discussionLabels,
  locale,
  ...props
}: YuvaRoadmapProps) {
  if (server || channel || client) {
    return (
      <YuvaDiscussionsProvider
        server={server}
        channel={channel}
        identityToken={identityToken}
        client={client}
        labels={discussionLabels}
        locale={locale}
      >
        <Roadmap discussionLabels={discussionLabels} {...props} />
      </YuvaDiscussionsProvider>
    )
  }
  return <Roadmap discussionLabels={discussionLabels} {...props} />
}

type View = { kind: "list" } | { kind: "suggest" } | { kind: "idea"; id: string }

function Roadmap({
  defaultStage = "ideas",
  labels: overrides,
  discussionLabels,
  className,
}: Pick<YuvaRoadmapProps, "defaultStage" | "labels" | "discussionLabels" | "className">) {
  const labels = useYuvaRoadmapLabels(overrides)
  const { client } = useYuvaDiscussionsContext()
  const { board, error } = useRoadmap()
  const [stage, setStage] = React.useState<YuvaRoadmapStage>(defaultStage)
  const [view, setView] = React.useState<View>({ kind: "list" })

  if (!client || (error && !board)) return null

  return (
    <Card className={cn("w-full", className)}>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Lightbulb className="size-5" aria-hidden />
          <h2>{board?.name || labels.heading}</h2>
        </CardTitle>
        {board?.description && <CardDescription>{board.description}</CardDescription>}
      </CardHeader>
      <CardContent>
        {view.kind === "idea" ? (
          <YuvaIdea
            id={view.id}
            labels={overrides}
            discussionLabels={discussionLabels}
            onBack={() => setView({ kind: "list" })}
            onGone={() => setView({ kind: "list" })}
          />
        ) : view.kind === "suggest" ? (
          <YuvaIdeaSuggest
            labels={overrides}
            discussionLabels={discussionLabels}
            onSelect={(idea) => setView({ kind: "idea", id: idea.id })}
            onCancel={() => setView({ kind: "list" })}
          />
        ) : (
          <YuvaRoadmapList
            stage={stage}
            onStageChange={setStage}
            labels={overrides}
            discussionLabels={discussionLabels}
            onSelect={(idea) => setView({ kind: "idea", id: idea.id })}
            onSuggest={() => setView({ kind: "suggest" })}
          />
        )}
      </CardContent>
    </Card>
  )
}

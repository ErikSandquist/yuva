"use client"

import * as React from "react"
import type { Discussion } from "useyuva"
import { Badge } from "@/components/ui/badge"
import { YuvaDiscussionThread } from "@/components/yuva-discussion-thread"
import { useYuvaRoadmapLabels } from "@/hooks/use-yuva-roadmap"
import { ideaStageLabel, ideaVotingClosed, type YuvaRoadmapLabels } from "@/lib/yuva-roadmap-utils"
import type { YuvaDiscussionsLabels } from "@/lib/yuva-utils"

export interface YuvaIdeaProps {
  id: string
  labels?: Partial<YuvaRoadmapLabels>
  discussionLabels?: Partial<YuvaDiscussionsLabels>
  className?: string
  onBack?(): void
  onGone?(): void
}

export function YuvaIdea({ id, labels: overrides, discussionLabels, className, onBack, onGone }: YuvaIdeaProps) {
  const labels = useYuvaRoadmapLabels(overrides)
  return (
    <YuvaDiscussionThread
      id={id}
      labels={{ back: labels.back, ...discussionLabels }}
      className={className}
      onBack={onBack}
      onGone={onGone}
      votingClosed={(idea: Discussion) => ideaVotingClosed(idea.stage)}
      badges={(idea: Discussion) => (
        <>
          <Badge variant={idea.stage === "shipped" ? "default" : "secondary"}>
            {ideaStageLabel(idea.stage, labels)}
          </Badge>
          {ideaVotingClosed(idea.stage) && <span>{labels.votingClosed}</span>}
        </>
      )}
    />
  )
}

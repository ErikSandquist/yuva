import type { IdeaStage } from "useyuva"

export const yuvaRoadmapStages = ["ideas", "planned", "in_progress", "shipped"] as const satisfies readonly IdeaStage[]

export type YuvaRoadmapStage = (typeof yuvaRoadmapStages)[number]

export interface YuvaRoadmapLabels {
  heading: string
  stageIdeas: string
  stagePlanned: string
  stageInProgress: string
  stageShipped: string
  stageClosed: string
  suggest: string
  suggestHeading: string
  suggestTitlePlaceholder: string
  suggestBodyPlaceholder: string
  sendIdea: string
  similarIdeas: string
  ideaSent: string
  ideaPending: string
  votingClosed: string
  empty: string
  vote: (title: string) => string
  removeVote: (title: string) => string
  votes: (count: number) => string
  back: string
}

export const defaultYuvaRoadmapLabels: YuvaRoadmapLabels = {
  heading: "Roadmap",
  stageIdeas: "Ideas",
  stagePlanned: "Planned",
  stageInProgress: "In progress",
  stageShipped: "Shipped",
  stageClosed: "Closed",
  suggest: "Suggest an idea",
  suggestHeading: "Suggest an idea",
  suggestTitlePlaceholder: "Your idea in a few words",
  suggestBodyPlaceholder: "Describe it. What do you need, and why? Markdown works.",
  sendIdea: "Send idea",
  similarIdeas: "Similar ideas",
  ideaSent: "Thanks! Your idea is on the board.",
  ideaPending: "Thanks! The team will review your idea before it is shown.",
  votingClosed: "Voting is closed for this idea.",
  empty: "Nothing here yet.",
  vote: (title) => `Vote for ${title}`,
  removeVote: (title) => `Remove your vote for ${title}`,
  votes: (count) => (count === 1 ? "1 vote" : `${count} votes`),
  back: "Roadmap",
}

export function ideaStageLabel(stage: IdeaStage | undefined, labels: YuvaRoadmapLabels): string {
  switch (stage) {
    case "planned":
      return labels.stagePlanned
    case "in_progress":
      return labels.stageInProgress
    case "shipped":
      return labels.stageShipped
    case "closed":
      return labels.stageClosed
    default:
      return labels.stageIdeas
  }
}

export function ideaVotingClosed(stage: IdeaStage | undefined): boolean {
  return stage === "shipped" || stage === "closed"
}

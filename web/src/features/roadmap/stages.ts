import type { IdeaStage } from "@/lib/api"

export const STAGE_DOT: Record<IdeaStage, string> = {
  ideas: "bg-faint",
  planned: "bg-blue-600 dark:bg-blue-400",
  in_progress: "bg-warning",
  shipped: "bg-success",
  closed: "bg-muted-foreground/40",
}

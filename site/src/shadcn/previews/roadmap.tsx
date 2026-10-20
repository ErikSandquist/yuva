import * as React from "react"
import { YuvaIdea } from "@/components/yuva-idea"
import { YuvaIdeaSuggest } from "@/components/yuva-idea-suggest"
import { YuvaRoadmap } from "@/components/yuva-roadmap"
import { YuvaDiscussionsProvider } from "@/hooks/use-yuva-discussions"
import { useIdeas, useRoadmap } from "@/hooks/use-yuva-roadmap"
import { Badge } from "@/components/ui/badge"
import { createRoadmapFixture } from "../fixtures"

function Provider({ children }: { children: React.ReactNode }) {
  const [client] = React.useState(createRoadmapFixture)
  return <YuvaDiscussionsProvider client={client}>{children}</YuvaDiscussionsProvider>
}

export function YuvaRoadmapPreview() {
  const [client] = React.useState(createRoadmapFixture)
  return <YuvaRoadmap client={client} className="w-full max-w-2xl" />
}

export function YuvaIdeaPreview() {
  return (
    <Provider>
      <YuvaIdea id="i_dark" className="w-full max-w-2xl" />
    </Provider>
  )
}

export function YuvaIdeaSuggestPreview() {
  const [sent, setSent] = React.useState("")
  return (
    <Provider>
      <div className="flex w-full max-w-md flex-col gap-3">
        <YuvaIdeaSuggest onSuggested={(idea) => setSent(idea.title)} />
        {sent && <p className="text-sm text-muted-foreground">onSuggested: {sent}</p>}
      </div>
    </Provider>
  )
}

function Planned() {
  const { board } = useRoadmap()
  const { ideas, loading } = useIdeas("planned", "top")
  return (
    <div className="flex w-full max-w-md flex-col gap-3 rounded-xl border bg-card p-4 text-sm">
      <p className="text-muted-foreground">{board ? `${board.name}: ${board.stages.planned} planned` : "Loading"}</p>
      <ul className="flex flex-col gap-2">
        {ideas.map((idea) => (
          <li key={idea.id} className="flex items-center gap-3">
            <span className="flex-1">{idea.title}</span>
            <Badge variant="outline">{idea.votes}</Badge>
          </li>
        ))}
        {loading && <li className="text-muted-foreground">Loading</li>}
      </ul>
    </div>
  )
}

export function UseYuvaRoadmapPreview() {
  return (
    <Provider>
      <Planned />
    </Provider>
  )
}

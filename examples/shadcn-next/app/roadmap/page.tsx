import type { Metadata } from "next"
import { RoadmapBoard } from "./roadmap-board"

export const metadata: Metadata = { title: "Roadmap" }

export default function RoadmapPage() {
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-3xl font-semibold tracking-tight">Roadmap</h1>
      <RoadmapBoard server={process.env.NEXT_PUBLIC_YUVA_SERVER} channel={process.env.NEXT_PUBLIC_YUVA_CHANNEL} />
    </div>
  )
}

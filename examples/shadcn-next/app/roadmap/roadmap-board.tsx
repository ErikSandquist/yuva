"use client"

import { YuvaRoadmap } from "@/components/yuva-roadmap"

export function RoadmapBoard({ server, channel }: { server?: string; channel?: string }) {
  return <YuvaRoadmap server={server} channel={channel} />
}

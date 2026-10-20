import * as React from "react"
import type { Discussion } from "useyuva"
import { Badge } from "@/components/ui/badge"
import { YuvaDiscussionComposer } from "@/components/yuva-discussion-composer"
import { YuvaDiscussionList } from "@/components/yuva-discussion-list"
import { YuvaDiscussionThread } from "@/components/yuva-discussion-thread"
import { YuvaDiscussions } from "@/components/yuva-discussions"
import { YuvaPageFeedback } from "@/components/yuva-page-feedback"
import { useDiscussions, useYuvaReader, YuvaDiscussionsProvider } from "@/hooks/use-yuva-discussions"
import { createDiscussionsFixture } from "../fixtures"

const page = "/docs/getting-started"

function Provider({ children }: { children: React.ReactNode }) {
  const [client] = React.useState(createDiscussionsFixture)
  return <YuvaDiscussionsProvider client={client}>{children}</YuvaDiscussionsProvider>
}

export function YuvaDiscussionsPreview() {
  const [client] = React.useState(createDiscussionsFixture)
  return <YuvaDiscussions client={client} page={page} pageTitle="Getting started" className="w-full max-w-2xl" />
}

export function YuvaDiscussionListPreview() {
  const [picked, setPicked] = React.useState<Discussion | null>(null)
  return (
    <Provider>
      <div className="flex w-full max-w-2xl flex-col gap-3">
        <YuvaDiscussionList page={page} onSelect={setPicked} />
        {picked && <p className="text-sm text-muted-foreground">onSelect: {picked.title}</p>}
      </div>
    </Provider>
  )
}

export function YuvaDiscussionThreadPreview() {
  return (
    <Provider>
      <YuvaDiscussionThread id="d_key" className="w-full max-w-2xl" />
    </Provider>
  )
}

export function YuvaDiscussionComposerPreview() {
  const [sent, setSent] = React.useState<{ title: string; body: string } | null>(null)
  return (
    <Provider>
      <div className="flex w-full max-w-2xl flex-col gap-3">
        <YuvaDiscussionComposer
          withTitle
          onSubmit={async (input) => {
            setSent(input)
          }}
        />
        {sent && <p className="text-sm text-muted-foreground">onSubmit: {sent.title}</p>}
      </div>
    </Provider>
  )
}

export function YuvaPageFeedbackPreview() {
  return (
    <Provider>
      <YuvaPageFeedback page={page} pageTitle="Getting started" />
    </Provider>
  )
}

function DiscussionsHookDemo() {
  const { discussions, loading } = useDiscussions(page, "top")
  const { reader, writer } = useYuvaReader()
  return (
    <div className="flex w-full max-w-md flex-col gap-3 rounded-xl border bg-card p-4 text-sm">
      <p className="flex items-center gap-2">
        <Badge variant="outline">{writer ?? "reader"}</Badge>
        <span className="text-muted-foreground">{reader?.reader.name ?? "signed out"}</span>
      </p>
      {loading ? (
        <p className="text-muted-foreground">Loading</p>
      ) : (
        <ul className="flex flex-col gap-1">
          {discussions.map((d) => (
            <li key={d.id} className="flex justify-between gap-3">
              <span>{d.title}</span>
              <span className="shrink-0 text-muted-foreground">{d.votes} votes</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

export function UseYuvaDiscussionsPreview() {
  return (
    <Provider>
      <DiscussionsHookDemo />
    </Provider>
  )
}

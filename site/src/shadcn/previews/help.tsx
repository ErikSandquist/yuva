import * as React from "react"
import { Badge } from "@/components/ui/badge"
import { YuvaHelp } from "@/components/yuva-help"
import { YuvaHelpArticle } from "@/components/yuva-help-article"
import { YuvaHelpSearch } from "@/components/yuva-help-search"
import { useYuvaHelp, useYuvaHelpSearch } from "@/hooks/use-yuva-help"
import { createHelpFixture } from "../fixtures"

export function YuvaHelpPreview() {
  const [client] = React.useState(createHelpFixture)
  return <YuvaHelp client={client} page="/billing/invoices" onAsk={() => undefined} className="w-full max-w-2xl" />
}

export function YuvaHelpSearchPreview() {
  const [client] = React.useState(createHelpFixture)
  const [picked, setPicked] = React.useState("")
  return (
    <div className="flex w-full max-w-md flex-col gap-3">
      <YuvaHelpSearch client={client} onSelect={(article) => setPicked(article.title)} />
      {picked && <p className="text-sm text-muted-foreground">onSelect: {picked}</p>}
    </div>
  )
}

export function YuvaHelpArticlePreview() {
  const [client] = React.useState(createHelpFixture)
  return <YuvaHelpArticle client={client} id="ha_invite" onAsk={() => undefined} className="w-full max-w-2xl" />
}

export function UseYuvaHelpPreview() {
  const [client] = React.useState(createHelpFixture)
  const [query, setQuery] = React.useState("invoice")
  const help = useYuvaHelp({ client })
  const { results, searching } = useYuvaHelpSearch(help, query)
  return (
    <div className="flex w-full max-w-md flex-col gap-3 rounded-xl border bg-card p-4 text-sm">
      <p className="text-muted-foreground">
        {help.loading ? "Loading" : `${help.center?.center.name}: ${help.center?.collections.length} collections`}
      </p>
      <input
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        aria-label="Query"
        className="rounded-md border bg-background px-3 py-1.5"
      />
      <ul className="flex flex-col gap-2">
        {results?.items.map((item) => (
          <li key={item.id} className="flex items-center gap-3">
            <span className="flex-1">{item.title}</span>
            {item.collection && <Badge variant="outline">{item.collection.name}</Badge>}
          </li>
        ))}
        {searching && <li className="text-muted-foreground">Searching</li>}
      </ul>
    </div>
  )
}

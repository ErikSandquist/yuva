import * as React from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { YuvaSubscribe } from "@/components/yuva-subscribe"
import { useYuvaSubscribe } from "@/hooks/use-yuva-subscribe"
import { createSubscribeFixture } from "../fixtures"

export function YuvaSubscribePreview() {
  const [client] = React.useState(createSubscribeFixture)
  return <YuvaSubscribe client={client} name="optional" className="w-full max-w-md" />
}

export function UseYuvaSubscribePreview() {
  const [client] = React.useState(createSubscribeFixture)
  const { lists, states, loading, inboxName, setSubscribed } = useYuvaSubscribe({ client })
  return (
    <div className="flex w-full max-w-md flex-col gap-3 rounded-xl border bg-card p-4 text-sm">
      <p className="text-muted-foreground">{loading ? "Loading" : inboxName}</p>
      <ul className="flex flex-col gap-2">
        {lists.map((list) => (
          <li key={list.id} className="flex items-center gap-3">
            <span className="flex-1">{list.name}</span>
            {states[list.id] && <Badge variant="outline">{states[list.id]}</Badge>}
            <Button
              size="sm"
              variant="outline"
              onClick={() => void setSubscribed(list.id, states[list.id] !== "subscribed", "I agree")}
            >
              {states[list.id] === "subscribed" ? "Unsubscribe" : "Subscribe"}
            </Button>
          </li>
        ))}
      </ul>
    </div>
  )
}

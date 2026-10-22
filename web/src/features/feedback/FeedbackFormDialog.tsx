import { Trans } from "@lingui/react/macro"
import { useQueries } from "@tanstack/react-query"
import { useNavigate } from "react-router"

import { CodeBlock } from "@/components/common"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { api, unwrap } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

const WEB = 'Yuva.openApp("feedback")'
const APP = "YuvaFeedbackView(screen: screenName, onDone: { })"

export default function FeedbackFormDialog({ onClose }: { onClose: () => void }) {
  const navigate = useNavigate()
  const { workspaceId: ws } = useSession()
  const inboxes = useInboxes().data ?? []
  const lists = useQueries({
    queries: inboxes.map((i) => ({
      queryKey: keys.channels(ws, i.id),
      queryFn: () =>
        unwrap(api.GET("/v1/inboxes/{inboxId}/channels", { params: { path: { inboxId: i.id } } })).then((r) => r.items),
      staleTime: 5 * 60_000,
    })),
  })
  const rows = inboxes.flatMap((inbox, n) =>
    (lists[n]?.data ?? [])
      .filter((ch) => ch.kind === "chat" || ch.kind === "app")
      .map((ch) => ({ inbox: inbox.name, channel: ch.name, app: ch.kind === "app" }))
  )
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-xl" data-testid="feedback-form-dialog">
        <DialogHeader>
          <DialogTitle>
            <Trans>Feedback form</Trans>
          </DialogTitle>
          <DialogDescription>
            <Trans>
              People send a bug, an idea or praise, with a screenshot and the device details. It lands here.
            </Trans>
          </DialogDescription>
        </DialogHeader>
        <ul className="divide-y rounded-[10px] border">
          {rows.length === 0 && (
            <li className="px-3 py-4 text-faint">
              <Trans>No website chat or app is set up yet.</Trans>
            </li>
          )}
          {rows.map((r) => (
            <li key={`${r.inbox}/${r.channel}`} className="flex items-center gap-3 px-3 py-2.5">
              <span className="flex min-w-0 flex-1 flex-col leading-[1.3]">
                <span className="truncate font-medium">{r.inbox}</span>
                <span className="truncate text-caption text-faint">{r.channel}</span>
              </span>
              <span className="shrink-0 text-caption text-muted-foreground">
                {r.app ? <Trans>App</Trans> : <Trans>Website</Trans>}
              </span>
            </li>
          ))}
        </ul>
        <p className="text-caption text-faint">
          <Trans>
            Web pages use the chat widget with the feedback form turned on. Apps call the feedback form from the SDK.
          </Trans>
        </p>
        <div className="grid gap-2">
          <span className="text-caption text-faint">
            <Trans>Website</Trans>
          </span>
          <CodeBlock value={WEB} />
          <span className="text-caption text-faint">
            <Trans>App</Trans>
          </span>
          <CodeBlock value={APP} />
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="ghost"
            onClick={() => {
              onClose()
              navigate("/settings?view=channels")
            }}
          >
            <Trans>Channel settings</Trans>
          </Button>
          <Button type="button" onClick={onClose}>
            <Trans>Done</Trans>
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

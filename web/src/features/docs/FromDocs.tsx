import { Trans, useLingui } from "@lingui/react/macro"
import { useState } from "react"

import { ContactAvatar, ErrorLine } from "@/components/common"
import { formatShort } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { useConversation, useMessages } from "@/features/conversation/queries"
import { FromConversationDialog } from "@/features/docs/discussions/FromConversation"
import { conversationPage, pageLabel } from "@/features/docs/PageContext"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { useConversations } from "@/features/inbox/queries"
import { cn } from "@/lib/utils"

function Picked({ id, onClose }: { id: string; onClose: () => void }) {
  const conversation = useConversation(id)
  const messages = useMessages(id)
  if (conversation.data && messages.data) {
    const items = messages.data.pages.toReversed().flatMap((p) => p.items.toReversed())
    return <FromConversationDialog c={conversation.data} messages={items} onClose={onClose} />
  }
  const error = conversation.error ?? messages.error
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>
            <Trans>Turn into a discussion</Trans>
          </DialogTitle>
        </DialogHeader>
        {error ? <ErrorLine error={error} /> : <Skeleton className="h-24 w-full" />}
      </DialogContent>
    </Dialog>
  )
}

export function FromDocsDialog({ onClose }: { onClose: () => void }) {
  const { t, i18n } = useLingui()
  const [inboxId] = useInboxFilter()
  const [picked, setPicked] = useState("")
  const list = useConversations({
    inbox_id: inboxId || undefined,
    kind: ["conversation"],
    channel_kind: ["chat"],
  })
  const items = list.data?.pages.flatMap((p) => p.items) ?? []
  if (picked) return <Picked id={picked} onClose={onClose} />
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        className="max-h-[90svh] grid-cols-[minmax(0,1fr)] overflow-y-auto sm:max-w-xl"
        data-testid="docs-from-dialog"
      >
        <DialogHeader>
          <DialogTitle>
            <Trans>Turn into a discussion</Trans>
          </DialogTitle>
          <DialogDescription>
            <Trans>
              Pick the conversation to answer in public. The next reader finds it under your documentation page.
            </Trans>
          </DialogDescription>
        </DialogHeader>
        {list.isPending ? (
          <Skeleton className="h-24 w-full" />
        ) : list.error ? (
          <ErrorLine error={list.error} />
        ) : items.length === 0 ? (
          <p className="text-body text-faint">
            <Trans>No conversations to choose from.</Trans>
          </p>
        ) : (
          <ul className="flex flex-col gap-1" aria-label={t`Conversations`}>
            {items.map((c) => {
              const name = c.contact.name || c.contact.email || t`Visitor`
              const page = conversationPage(c)
              return (
                <li key={c.id}>
                  <button
                    type="button"
                    onClick={() => setPicked(c.id)}
                    className={cn(
                      "flex w-full items-center gap-3 rounded-lg border border-transparent px-2.5 py-2 text-start outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
                    )}
                    data-testid="docs-from-row"
                  >
                    <ContactAvatar id={c.contact_id} name={name} className="size-8 shrink-0" />
                    <span className="flex min-w-0 flex-1 flex-col leading-[1.35]">
                      <span className="flex items-center gap-2">
                        <span className="min-w-0 flex-1 truncate text-body font-medium">{name}</span>
                        <time className="shrink-0 text-caption text-faint" dateTime={c.last_activity_at}>
                          {formatShort(c.last_activity_at, i18n.locale)}
                        </time>
                      </span>
                      <span className="truncate text-small text-muted-foreground">
                        {c.subject || c.last_message?.text}
                      </span>
                      {page && (
                        <span className="truncate text-caption text-faint" dir="ltr">
                          {pageLabel(page.page_url, page.page_title)}
                        </span>
                      )}
                    </span>
                  </button>
                </li>
              )
            })}
          </ul>
        )}
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            <Trans>Cancel</Trans>
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

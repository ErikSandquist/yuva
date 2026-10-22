import { Trans, useLingui } from "@lingui/react/macro"
import { useState } from "react"

import { ContactAvatar, ErrorLine, isVisitor, toast, VisitorAvatar } from "@/components/common"
import { useContactName } from "@/components/common/Source"
import { useErrorText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { useConversations } from "@/features/inbox/queries"
import { useLinkedConversations, useLinkMutations } from "@/features/status/linked"
import type { ConversationListItem, StatusIncident, StatusPage } from "@/lib/api"
import { useChannelMap } from "@/lib/workspace"

function Row({
  c,
  linked,
  busy,
  onLink,
}: {
  c: ConversationListItem
  linked: boolean
  busy: boolean
  onLink: () => void
}) {
  const channel = useChannelMap().get(c.channel_id ?? "")
  const contactName = useContactName()
  const visitor = isVisitor(c.contact.name, c.contact.email)
  const name = contactName(c, channel, c.contact.name, c.contact.email)
  return (
    <li className="flex items-center gap-3 px-4 py-2.5" data-testid="link-conversation-row">
      {visitor ? (
        <VisitorAvatar className="size-8" />
      ) : (
        <ContactAvatar id={c.contact.id} name={name} className="size-8" />
      )}
      <span className="flex min-w-0 flex-1 flex-col leading-[1.35]">
        <span className="truncate font-medium">{name}</span>
        <span className="truncate text-caption text-faint">{c.last_message?.text || c.subject}</span>
      </span>
      {linked ? (
        <span className="text-caption text-faint">
          <Trans>Already linked</Trans>
        </span>
      ) : (
        <Button variant="outline" size="sm" disabled={busy} onClick={onLink}>
          <Trans>Link</Trans>
        </Button>
      )}
    </li>
  )
}

export function LinkConversationsDialog({
  page,
  incident,
  onClose,
}: {
  page: StatusPage
  incident: StatusIncident
  onClose: () => void
}) {
  const { t } = useLingui()
  const errorText = useErrorText()
  const [q, setQ] = useState("")
  const title = incident.title
  const list = useConversations({ inbox_id: page.inbox_id, status: ["open", "pending"], q: q.trim() || undefined })
  const linkedList = useLinkedConversations(page.id, incident.id)
  const linked = new Set(linkedList.data?.pages.flatMap((p) => p.items.map((c) => c.id)) ?? [])
  const { link } = useLinkMutations()
  const items = list.data?.pages.flatMap((p) => p.items) ?? []
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg" data-testid="link-conversations">
        <DialogHeader>
          <DialogTitle>
            <Trans>Link conversations</Trans>
          </DialogTitle>
          <DialogDescription>
            <Trans>Open conversations of this product that may be about {title}.</Trans>
          </DialogDescription>
        </DialogHeader>
        <Input
          type="search"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder={t`Search conversations`}
          aria-label={t`Search conversations`}
          autoFocus
        />
        <div className="max-h-[50vh] overflow-y-auto rounded-xl border">
          {list.isPending ? (
            <Skeleton className="m-3 h-12" />
          ) : items.length === 0 ? (
            <p className="px-4 py-6 text-center text-faint">
              <Trans>No conversation matches.</Trans>
            </p>
          ) : (
            <ul className="divide-y">
              {items.map((c) => (
                <Row
                  key={c.id}
                  c={c}
                  linked={linked.has(c.id)}
                  busy={link.isPending}
                  onLink={() =>
                    link.mutate(
                      { pageId: page.id, incidentId: incident.id, conversationId: c.id },
                      { onError: (e) => toast(errorText(e)) }
                    )
                  }
                />
              ))}
            </ul>
          )}
        </div>
        <ErrorLine error={list.error} />
        {list.hasNextPage && (
          <Button
            variant="ghost"
            size="sm"
            disabled={list.isFetchingNextPage}
            onClick={() => void list.fetchNextPage()}
          >
            <Trans>Load more</Trans>
          </Button>
        )}
        <DialogFooter>
          <Button onClick={onClose}>
            <Trans>Done</Trans>
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

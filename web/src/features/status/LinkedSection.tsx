import { Trans, useLingui } from "@lingui/react/macro"

import { toast } from "@/components/common"
import { LinkedConversations as Linked } from "@/components/common/Linked"
import { STATUS_KEYS } from "@/components/common/ShortcutSheet"
import { useErrorText } from "@/components/common/text"
import {
  useLinkedConversations,
  useLinkMutations,
  useReplyCounts,
  useReplyOutcomes,
  useReplyToLinked,
} from "@/features/status/linked"
import type { StatusIncident } from "@/lib/api"

export function LinkedConversations({
  pageId,
  incident,
  replyId,
  onReply,
  onDismissReply,
}: {
  pageId: string
  incident: StatusIncident
  replyId: string
  onReply: (id: string) => void
  onDismissReply: () => void
}) {
  const { t } = useLingui()
  const errorText = useErrorText()
  const list = useLinkedConversations(pageId, incident.id)
  const { unlink } = useLinkMutations()
  const title = incident.title
  return (
    <Linked
      list={list}
      title={<Trans>Conversations</Trans>}
      empty={
        <Trans>
          No conversation is linked yet. Open one and use Link to an incident, or select several in the list.
        </Trans>
      }
      description={
        <Trans>
          One reply, written once, goes to every open conversation linked to {title}, as you, on each conversation’s own
          channel.
        </Trans>
      }
      unlinked={t`It was unlinked from the incident.`}
      replyKey={STATUS_KEYS.replyLinked}
      replyId={replyId}
      hooks={{
        useSend: () => useReplyToLinked(pageId, incident.id),
        useCounts: (id) => useReplyCounts(pageId, incident.id, id),
        useOutcomes: (id, enabled) => useReplyOutcomes(pageId, incident.id, id, enabled),
      }}
      busy={unlink.isPending}
      onUnlink={(c) =>
        unlink.mutate(
          { pageId, incidentId: incident.id, conversationId: c.id },
          { onError: (e) => toast(errorText(e)) }
        )
      }
      onReply={onReply}
      onDismissReply={onDismissReply}
    />
  )
}

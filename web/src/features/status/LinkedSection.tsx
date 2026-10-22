import { Trans, useLingui } from "@lingui/react/macro"
import { useState } from "react"

import { toast } from "@/components/common"
import { LinkedConversations as Linked } from "@/components/common/Linked"
import { INCIDENT_KEYS } from "@/components/common/ShortcutSheet"
import { useErrorText } from "@/components/common/text"
import {
  useLinkedConversations,
  useLinkMutations,
  useReplyCounts,
  useReplyOutcomes,
  useReplyToLinked,
} from "@/features/status/linked"
import { LinkConversationsDialog } from "@/features/status/LinkConversationsDialog"
import { useStatusPage } from "@/features/status/queries"
import { useHotkeys } from "@/hooks/use-hotkeys"
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
  const page = useStatusPage(pageId).data
  const [linking, setLinking] = useState(false)
  const canLink = !!page
  useHotkeys(canLink ? { [INCIDENT_KEYS.link]: () => setLinking(true) } : {}, !linking)
  return (
    <>
      <Linked
        list={list}
        title={<Trans>Linked conversations</Trans>}
        empty={
          <Trans>
            No conversation is linked yet. Open one and use Link to an incident, or select several in the list.
          </Trans>
        }
        description={
          <Trans>
            One reply, written once, goes to every open conversation linked to {title}, as you, on each conversation’s
            own channel.
          </Trans>
        }
        unlinked={t`It was unlinked from the incident.`}
        replyKey={INCIDENT_KEYS.replyLinked}
        replyId={replyId}
        hooks={{
          useSend: () => useReplyToLinked(pageId, incident.id),
          useCounts: (id) => useReplyCounts(pageId, incident.id, id),
          useOutcomes: (id, enabled) => useReplyOutcomes(pageId, incident.id, id, enabled),
        }}
        busy={unlink.isPending}
        onLink={canLink ? () => setLinking(true) : undefined}
        onUnlink={(c) =>
          unlink.mutate(
            { pageId, incidentId: incident.id, conversationId: c.id },
            { onError: (e) => toast(errorText(e)) }
          )
        }
        onReply={onReply}
        onDismissReply={onDismissReply}
      />
      {linking && page && <LinkConversationsDialog page={page} incident={incident} onClose={() => setLinking(false)} />}
    </>
  )
}

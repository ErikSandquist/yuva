import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { CornerUpLeftIcon, Link2OffIcon, ReplyAllIcon, XIcon } from "lucide-react"
import { useEffect, useState } from "react"
import { Link } from "react-router"

import {
  ContactAvatar,
  ErrorLine,
  isVisitor,
  Kbd,
  ProgressBar,
  toast,
  useContactName,
  VisitorAvatar,
} from "@/components/common"
import { keyLabel, STATUS_KEYS } from "@/components/common/ShortcutSheet"
import { formatShort, useEnumText, useErrorText } from "@/components/common/text"
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
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { useContact } from "@/features/contact/queries"
import { useConversation } from "@/features/conversation/queries"
import { Card, EmptyRow, Section, ToggleRow } from "@/features/settings/ui"
import {
  useLinkedConversations,
  useLinkMutations,
  useReplyCounts,
  useReplyOutcomes,
  useReplyToLinked,
} from "@/features/status/linked"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { StatusIncident, StatusIncidentConversation } from "@/lib/api"
import { cn } from "@/lib/utils"
import { useChannelMap } from "@/lib/workspace"

const MAX = 5000

function LinkedRow({ c, onUnlink, busy }: { c: StatusIncidentConversation; onUnlink: () => void; busy: boolean }) {
  const { t, i18n } = useLingui()
  const text = useEnumText()
  const channel = useChannelMap().get(c.channel_id ?? "")
  const contactName = useContactName()
  const visitor = isVisitor(c.contact.name, c.contact.email)
  const name = contactName(c, channel, c.contact.name, c.contact.email)
  const preview = c.last_message?.text || c.subject
  return (
    <li className="flex items-center gap-1 pe-2 transition-colors hover:bg-muted/60" data-testid="linked-conversation">
      <Link
        to={`/conversations/${c.id}`}
        className="flex min-w-0 flex-1 items-center gap-3 py-2.5 ps-4 outline-none focus-visible:bg-muted"
      >
        {visitor ? (
          <VisitorAvatar className="size-9" />
        ) : (
          <ContactAvatar id={c.contact.id} name={name} className="size-9" />
        )}
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="flex min-w-0 items-baseline gap-2">
            <span className="min-w-0 flex-1 truncate text-body font-medium">{name}</span>
            <span
              className={cn(
                "shrink-0 text-caption font-medium",
                c.status === "closed" ? "text-success" : "text-warning"
              )}
              data-testid="linked-state"
            >
              {text.status[c.status]}
            </span>
          </span>
          <span className="flex min-w-0 items-baseline gap-2 text-small text-faint">
            <span className="min-w-0 flex-1 truncate">{preview}</span>
            <time className="shrink-0 text-caption" dateTime={c.linked_at} title={t`Linked`}>
              {formatShort(c.linked_at, i18n.locale)}
            </time>
          </span>
        </span>
      </Link>
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={onUnlink}
        disabled={busy}
        aria-label={t`Unlink ${name}`}
        title={t`Unlink ${name}`}
        data-testid="linked-unlink"
      >
        <Link2OffIcon />
      </Button>
    </li>
  )
}

function ReplyDialog({
  pageId,
  incident,
  open,
  onClose,
  onSent,
}: {
  pageId: string
  incident: StatusIncident
  open: number
  onClose: () => void
  onSent: (id: string) => void
}) {
  const { t } = useLingui()
  const send = useReplyToLinked(pageId, incident.id)
  const [body, setBody] = useState("")
  const [close, setClose] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const title = incident.title
  return (
    <Dialog open onOpenChange={(o) => !o && !send.isPending && onClose()}>
      <DialogContent className="sm:max-w-xl" data-testid="linked-reply-dialog">
        <DialogHeader>
          <DialogTitle>
            <Trans>Reply to linked conversations</Trans>
          </DialogTitle>
          <DialogDescription>
            <Trans>
              One reply, written once, goes to every open conversation linked to {title}, as you, on each conversation’s
              own channel.
            </Trans>
          </DialogDescription>
        </DialogHeader>
        {confirming ? (
          <div className="flex flex-col gap-3" data-testid="linked-reply-confirm">
            <p className="text-body font-medium">
              <Trans>Open conversations that get this reply: {open}</Trans>
            </p>
            <p className="rounded-xl border bg-muted/50 p-3 text-reading break-words whitespace-pre-wrap">{body}</p>
            <p className="text-small text-muted-foreground">
              {close ? (
                <Trans>
                  Each conversation is closed after the reply. Customers get it right away and it cannot be recalled.
                </Trans>
              ) : (
                <Trans>The conversations stay open. Customers get it right away and it cannot be recalled.</Trans>
              )}
            </p>
            <ErrorLine error={send.error} />
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            <Textarea
              value={body}
              rows={6}
              autoFocus
              onChange={(e) => setBody(e.target.value)}
              placeholder={t`What do these customers need to know?`}
              aria-label={t`Reply text`}
              data-testid="linked-reply-body"
            />
            <ToggleRow title={<Trans>Close each conversation after replying</Trans>}>
              <Switch checked={close} onCheckedChange={setClose} data-testid="linked-reply-close" />
            </ToggleRow>
          </div>
        )}
        <DialogFooter>
          {confirming ? (
            <>
              <Button variant="ghost" onClick={() => setConfirming(false)} disabled={send.isPending}>
                <Trans>Back</Trans>
              </Button>
              <Button
                disabled={send.isPending}
                onClick={() => send.mutate({ body: body.trim(), close }, { onSuccess: (r) => onSent(r.id) })}
                data-testid="linked-reply-send"
              >
                <ReplyAllIcon />
                <Trans>Send reply</Trans>
              </Button>
            </>
          ) : (
            <>
              <Button variant="ghost" onClick={onClose}>
                <Trans>Cancel</Trans>
              </Button>
              <Button
                disabled={!body.trim() || open === 0}
                onClick={() => setConfirming(true)}
                data-testid="linked-reply-review"
              >
                <Trans>Review</Trans>
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function OutcomeRow({ id, outcome, reason }: { id: string; outcome: "skipped" | "failed"; reason?: string }) {
  const { t } = useLingui()
  const conversation = useConversation(id)
  const contact = useContact(conversation.data?.contact_id).data
  const name = contact?.name || contact?.emails[0] || conversation.data?.subject || t`A conversation`
  const skipped: Record<string, string> = {
    closed: t`It was closed before its turn.`,
    unlinked: t`It was unlinked from the incident.`,
    conversation_deleted: t`It was deleted.`,
    revoked: t`Your access ended before it was sent.`,
  }
  const why =
    outcome === "skipped"
      ? (skipped[reason ?? ""] ?? t`It was skipped.`)
      : reason === "gave_up"
        ? t`Sending kept failing, so Yuva gave up.`
        : reason
          ? t`Sending failed: ${reason}`
          : t`Sending failed.`
  return (
    <li className="flex items-start gap-3 px-4 py-2.5" data-testid="reply-outcome-row" data-outcome={outcome}>
      <span
        className={cn("mt-1.5 size-2 shrink-0 rounded-full", outcome === "failed" ? "bg-destructive" : "bg-warning")}
        aria-hidden
      />
      <span className="flex min-w-0 flex-1 flex-col">
        <Link to={`/conversations/${id}`} className="truncate text-body font-medium hover:underline">
          {name}
        </Link>
        <span className="text-small text-muted-foreground">{why}</span>
      </span>
      <span className="shrink-0 text-caption font-medium text-muted-foreground">
        {outcome === "failed" ? <Trans>Failed</Trans> : <Trans>Skipped</Trans>}
      </span>
    </li>
  )
}

function ReplyOutcome({
  pageId,
  incidentId,
  replyId,
  onDismiss,
}: {
  pageId: string
  incidentId: string
  replyId: string
  onDismiss: () => void
}) {
  const { t, i18n } = useLingui()
  const report = useReplyCounts(pageId, incidentId, replyId)
  const r = report.data
  const counts = r?.counts
  const done = !!counts && counts.pending === 0
  const problems = !!counts && counts.skipped + counts.failed > 0
  const outcomes = useReplyOutcomes(pageId, incidentId, replyId, done && problems)
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = outcomes
  useEffect(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage])
  const rows = (outcomes.data?.pages.flatMap((p) => p.items) ?? []).filter(
    (x): x is typeof x & { outcome: "skipped" | "failed" } => x.outcome === "skipped" || x.outcome === "failed"
  )
  const total = r ? counts!.pending + counts!.sent + counts!.skipped + counts!.failed : 0
  const finished = total - (counts?.pending ?? 0)
  return (
    <Section
      title={<Trans>Latest reply</Trans>}
      action={
        done && (
          <Button variant="ghost" size="icon-xs" onClick={onDismiss} aria-label={t`Dismiss`} title={t`Dismiss`}>
            <XIcon />
          </Button>
        )
      }
    >
      <Card flush data-testid="reply-outcome">
        {report.isPending ? (
          <Skeleton className="m-4 h-12" />
        ) : !r || !counts ? (
          <div className="p-4">
            <ErrorLine error={report.error} />
          </div>
        ) : (
          <>
            <div className="flex flex-col gap-3 p-4">
              <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <b className="text-body font-semibold" role="status" data-testid="reply-outcome-state">
                  {done ? <Trans>Finished</Trans> : <Trans>Sending…</Trans>}
                </b>
                <span className="text-small text-faint">{formatShort(r.created_at, i18n.locale)}</span>
                {r.close && (
                  <span className="text-small text-faint">
                    <Trans>Closing each conversation</Trans>
                  </span>
                )}
              </div>
              <ProgressBar
                value={total ? (finished / total) * 100 : 100}
                label={t`Reply progress`}
                text={`${finished} / ${total}`}
                testId="reply-progress"
              />
              <dl className="grid grid-cols-2 gap-3 text-small sm:grid-cols-4">
                {(
                  [
                    [t`Sent`, counts.sent, "text-success"],
                    [t`Waiting`, counts.pending, "text-muted-foreground"],
                    [t`Skipped`, counts.skipped, "text-warning"],
                    [t`Failed`, counts.failed, "text-destructive"],
                  ] as const
                ).map(([label, n, tone]) => (
                  <div key={label} className="flex flex-col">
                    <dt className="text-caption text-faint">{label}</dt>
                    <dd className={cn("text-title font-semibold tabular-nums", n > 0 && tone)}>{n}</dd>
                  </div>
                ))}
              </dl>
            </div>
            {done && problems && (
              <ul className="divide-y border-t" data-testid="reply-outcome-list">
                {rows.map((x) => (
                  <OutcomeRow key={x.conversation_id} id={x.conversation_id} outcome={x.outcome} reason={x.reason} />
                ))}
                {outcomes.isPending && (
                  <li className="px-4 py-3">
                    <Skeleton className="h-9" />
                  </li>
                )}
              </ul>
            )}
            <ErrorLine error={outcomes.error} className="border-t px-4 py-3" />
          </>
        )}
      </Card>
    </Section>
  )
}

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
  const [replying, setReplying] = useState(false)
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = list
  useEffect(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage])
  const items = list.data?.pages.flatMap((p) => p.items) ?? []
  const counting = list.isPending || !!hasNextPage
  const open = items.filter((c) => c.status !== "closed").length
  const closed = items.length - open
  const key = keyLabel(STATUS_KEYS.replyLinked).join("")
  const canReply = !counting && open > 0

  useHotkeys(canReply ? { [STATUS_KEYS.replyLinked]: () => setReplying(true) } : {})

  return (
    <>
      <Section
        title={<Trans>Conversations</Trans>}
        action={
          <Button
            variant="outline"
            size="sm"
            disabled={!canReply}
            onClick={() => setReplying(true)}
            title={t`Reply to linked (${key})`}
            data-testid="linked-reply"
          >
            <CornerUpLeftIcon />
            <Trans>Reply to linked</Trans>
            <Kbd className="phone:hidden">{key}</Kbd>
          </Button>
        }
      >
        <Card flush data-testid="linked-conversations">
          {list.isPending ? (
            <Skeleton className="m-4 h-12" />
          ) : items.length === 0 ? (
            <EmptyRow>
              <Trans>
                No conversation is linked yet. Open one and use Link to an incident, or select several in the list.
              </Trans>
            </EmptyRow>
          ) : (
            <>
              <p className="border-b px-4 py-2.5 text-small text-muted-foreground" data-testid="linked-counts">
                {counting ? (
                  <Trans>Counting…</Trans>
                ) : (
                  <>
                    <Plural value={open} one="# open" other="# open" /> ·{" "}
                    <Plural value={closed} one="# closed" other="# closed" />
                  </>
                )}
                {items.length >= MAX && (
                  <>
                    {" · "}
                    <Trans>The limit of 5,000 is reached</Trans>
                  </>
                )}
              </p>
              <ul className="divide-y">
                {items.map((c) => (
                  <LinkedRow
                    key={c.id}
                    c={c}
                    busy={unlink.isPending}
                    onUnlink={() =>
                      unlink.mutate(
                        { pageId, incidentId: incident.id, conversationId: c.id },
                        { onError: (e) => toast(errorText(e)) }
                      )
                    }
                  />
                ))}
              </ul>
            </>
          )}
          <ErrorLine error={list.error} className="border-t px-4 py-3" />
        </Card>
      </Section>
      {replyId && (
        <ReplyOutcome pageId={pageId} incidentId={incident.id} replyId={replyId} onDismiss={onDismissReply} />
      )}
      {replying && (
        <ReplyDialog
          pageId={pageId}
          incident={incident}
          open={open}
          onClose={() => setReplying(false)}
          onSent={(id) => {
            setReplying(false)
            onReply(id)
          }}
        />
      )}
    </>
  )
}

"use client"

import * as React from "react"
import type { Discussion, DiscussionPost } from "useyuva"
import { ArrowLeft, Bell, BellOff, CheckCircle2, ChevronUp, Lock, MessageSquare, Pencil, Trash2 } from "lucide-react"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { YuvaDiscussionComposer, YuvaSignIn } from "@/components/yuva-discussion-composer"
import {
  useDiscussion,
  useYuvaDiscussionsContext,
  useYuvaLabels,
  useYuvaReader,
  type DiscussionResult,
} from "@/hooks/use-yuva-discussions"
import { cn } from "@/lib/utils"
import {
  canEdit,
  relativeTime,
  yuvaErrorMessage,
  yuvaProseClassName,
  type YuvaDiscussionsLabels,
} from "@/lib/yuva-utils"

function authorName(author: DiscussionPost["author"], labels: YuvaDiscussionsLabels): string {
  if (author.deleted) return author.kind === "member" ? labels.deletedMember : labels.deletedAuthor
  return author.name || labels.anonymous
}

export interface YuvaVoteButtonProps {
  votes: number
  voted?: boolean
  disabled?: boolean
  labels: YuvaDiscussionsLabels
  onVote(): void
}

export function YuvaVoteButton({ votes, voted, disabled, labels, onVote }: YuvaVoteButtonProps) {
  return (
    <Button
      type="button"
      variant={voted ? "secondary" : "outline"}
      size="sm"
      aria-pressed={!!voted}
      aria-label={`${voted ? labels.removeUpvote : labels.upvote} (${labels.votes(votes)})`}
      title={voted ? labels.removeUpvote : labels.upvote}
      disabled={disabled}
      onClick={onVote}
      className={cn("tabular-nums", voted && "text-primary")}
    >
      <ChevronUp data-icon="inline-start" />
      {votes}
    </Button>
  )
}

interface PostProps {
  post: DiscussionPost
  thread: DiscussionResult
  discussion: Discussion
  labels: YuvaDiscussionsLabels
  locale?: string
  canWrite: boolean
  accepted?: boolean
  onReply?(): void
  onNeedSignIn(): void
  onError(message: string | null): void
  children?: React.ReactNode
}

function Post({
  post,
  thread,
  discussion,
  labels,
  locale,
  canWrite,
  accepted,
  onReply,
  onNeedSignIn,
  onError,
  children,
}: PostProps) {
  const [editing, setEditing] = React.useState(false)
  const locked = discussion.locked
  const run = (action: () => Promise<unknown>) =>
    action().then(
      () => onError(null),
      (failure) => onError(yuvaErrorMessage(failure, labels))
    )
  const canAccept =
    discussion.mine && !locked && !post.opening && !post.parent_post_id && !post.deleted && post.state === "published"
  const name = authorName(post.author, labels)

  return (
    <article
      aria-label={name}
      className={cn(
        "flex flex-col gap-3 rounded-lg border bg-card p-4 text-card-foreground",
        accepted && "border-primary/60 ring-1 ring-primary/30",
        post.state === "pending" && "border-dashed"
      )}
    >
      {accepted && (
        <p className="flex items-center gap-1.5 text-sm font-medium text-primary">
          <CheckCircle2 className="size-4" aria-hidden />
          {labels.acceptedAnswer}
        </p>
      )}
      <header className="flex flex-wrap items-center gap-2 text-sm">
        <Avatar size="sm" aria-hidden>
          <AvatarFallback>{post.deleted || post.author.deleted ? "?" : post.author.initials}</AvatarFallback>
        </Avatar>
        <span className="font-medium">{post.deleted ? labels.deletedAuthor : name}</span>
        {post.author.kind === "member" && !post.deleted && <Badge variant="secondary">{labels.team}</Badge>}
        {post.state === "pending" && <Badge variant="outline">{labels.pending}</Badge>}
        <time
          dateTime={post.created_at}
          className="text-muted-foreground"
          title={new Date(post.created_at).toLocaleString(locale)}
        >
          {relativeTime(post.created_at, locale)}
        </time>
        {post.edited_at && !post.deleted && <span className="text-muted-foreground">· {labels.edited}</span>}
      </header>

      {editing ? (
        <YuvaDiscussionComposer
          withTitle={post.opening}
          initialTitle={post.opening ? discussion.title : ""}
          initialBody={post.body}
          submitLabel={labels.save}
          autoFocus
          onCancel={() => setEditing(false)}
          onSubmit={async ({ title, body }) => {
            await thread.edit(post.id, post.opening ? { body, title } : { body })
            setEditing(false)
          }}
        />
      ) : post.deleted ? (
        <p className="text-sm text-muted-foreground italic">{labels.deletedPost}</p>
      ) : (
        <div className={yuvaProseClassName} dangerouslySetInnerHTML={{ __html: post.body_html }} />
      )}

      {post.state === "pending" && <p className="text-xs text-muted-foreground">{labels.pendingNote}</p>}

      {!editing && !post.deleted && (
        <footer className="flex flex-wrap items-center gap-1">
          <YuvaVoteButton
            votes={post.votes}
            voted={post.voted}
            labels={labels}
            disabled={locked || post.state !== "published"}
            onVote={() => (canWrite ? void run(() => thread.vote(post)) : onNeedSignIn())}
          />
          {onReply && !locked && (
            <Button type="button" variant="ghost" size="sm" onClick={() => (canWrite ? onReply() : onNeedSignIn())}>
              <MessageSquare data-icon="inline-start" />
              {labels.reply}
            </Button>
          )}
          {canAccept && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              aria-pressed={!!accepted}
              onClick={() => void run(() => thread.accept(accepted ? null : post.id))}
            >
              <CheckCircle2 data-icon="inline-start" />
              {accepted ? labels.unmarkAnswer : labels.markAnswer}
            </Button>
          )}
          {canEdit(post) && !locked && (
            <Button type="button" variant="ghost" size="sm" onClick={() => setEditing(true)}>
              <Pencil data-icon="inline-start" />
              {labels.edit}
            </Button>
          )}
          {post.mine && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="text-destructive"
              onClick={() => {
                if (window.confirm(labels.deleteConfirm)) void run(() => thread.remove(post.id))
              }}
            >
              <Trash2 data-icon="inline-start" />
              {labels.delete}
            </Button>
          )}
        </footer>
      )}
      {children}
    </article>
  )
}

export interface YuvaDiscussionThreadProps {
  id: string
  labels?: Partial<YuvaDiscussionsLabels>
  className?: string
  onBack?(): void
  onGone?(): void
}

export function YuvaDiscussionThread({ id, labels: overrides, className, onBack, onGone }: YuvaDiscussionThreadProps) {
  const labels = useYuvaLabels(overrides)
  const { locale } = useYuvaDiscussionsContext()
  const thread = useDiscussion(id)
  const { writer, signIn } = useYuvaReader()
  const [replyTo, setReplyTo] = React.useState<string | null>(null)
  const [error, setError] = React.useState<string | null>(null)
  const { discussion, opening, posts, loading } = thread
  const canWrite = writer !== null
  const needSignIn = () => {
    setError(null)
    signIn().catch((failure) => setError(yuvaErrorMessage(failure, labels)))
  }

  React.useEffect(() => {
    if (!loading && !discussion && !thread.error) onGone?.()
  }, [loading, discussion, thread.error])

  const back = onBack && (
    <Button type="button" variant="ghost" size="sm" onClick={onBack} className="self-start">
      <ArrowLeft data-icon="inline-start" className="rtl:rotate-180" />
      {labels.back}
    </Button>
  )

  if (!discussion) {
    return (
      <div className={cn("flex flex-col gap-4", className)} aria-busy={loading}>
        {back}
        {thread.error ? (
          <div role="alert" className="flex flex-col items-start gap-2 text-sm">
            <p className="text-destructive">{yuvaErrorMessage(thread.error, labels) ?? labels.loadError}</p>
            <Button variant="outline" size="sm" onClick={() => void thread.reload()}>
              {labels.retry}
            </Button>
          </div>
        ) : (
          <>
            <Skeleton className="h-7 w-2/3" />
            <Skeleton className="h-32 w-full" />
            <Skeleton className="h-20 w-full" />
          </>
        )}
      </div>
    )
  }

  const top = posts.filter((p) => !p.parent_post_id)
  const childrenOf = (postId: string) => posts.filter((p) => p.parent_post_id === postId)
  const post = (p: DiscussionPost, extra?: Partial<PostProps>) => (
    <Post
      key={p.id}
      post={p}
      thread={thread}
      discussion={discussion}
      labels={labels}
      locale={locale}
      canWrite={canWrite}
      onNeedSignIn={needSignIn}
      onError={setError}
      {...extra}
    />
  )

  return (
    <section className={cn("flex flex-col gap-4", className)} aria-labelledby={`yuva-thread-${discussion.id}`}>
      {back}
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-2">
          <h3 id={`yuva-thread-${discussion.id}`} className="text-xl font-semibold tracking-tight break-words">
            {discussion.title}
          </h3>
          <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            {discussion.answer_post_id && (
              <Badge className="gap-1">
                <CheckCircle2 aria-hidden />
                {labels.answered}
              </Badge>
            )}
            {discussion.locked && (
              <Badge variant="outline" className="gap-1">
                <Lock aria-hidden />
                {labels.locked}
              </Badge>
            )}
            {discussion.state === "pending" && <Badge variant="outline">{labels.pending}</Badge>}
            <span>{labels.replies(discussion.reply_count)}</span>
          </div>
        </div>
        {canWrite && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            aria-pressed={!!discussion.following}
            onClick={() =>
              void thread.follow(!discussion.following).then(
                () => setError(null),
                (failure) => setError(yuvaErrorMessage(failure, labels))
              )
            }
          >
            {discussion.following ? <BellOff data-icon="inline-start" /> : <Bell data-icon="inline-start" />}
            {discussion.following ? labels.unfollow : labels.follow}
          </Button>
        )}
      </div>

      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}

      {opening && post(opening)}

      {top.length > 0 && <Separator />}

      <ol className="flex flex-col gap-3" aria-label={labels.replies(discussion.reply_count)}>
        {top.map((reply) => (
          <li key={reply.id}>
            {post(reply, {
              accepted: reply.id === discussion.answer_post_id,
              onReply: () => setReplyTo(reply.id),
              children: (childrenOf(reply.id).length > 0 || replyTo === reply.id) && (
                <div className="flex flex-col gap-3 border-s-2 ps-3 sm:ps-4">
                  {childrenOf(reply.id).map((child) => post(child, { onReply: () => setReplyTo(reply.id) }))}
                  {replyTo === reply.id && (
                    <YuvaDiscussionComposer
                      autoFocus
                      labels={overrides}
                      onCancel={() => setReplyTo(null)}
                      onSubmit={async ({ body }) => {
                        await thread.reply(body, reply.id)
                        setReplyTo(null)
                      }}
                    />
                  )}
                </div>
              ),
            })}
          </li>
        ))}
      </ol>

      <Separator />
      {discussion.locked ? (
        <p className="flex items-center gap-2 text-sm text-muted-foreground">
          <Lock className="size-4" aria-hidden />
          {labels.lockedNote}
        </p>
      ) : canWrite ? (
        <YuvaDiscussionComposer labels={overrides} onSubmit={({ body }) => thread.reply(body)} />
      ) : (
        <YuvaSignIn labels={overrides} />
      )}
    </section>
  )
}

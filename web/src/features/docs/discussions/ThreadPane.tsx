import { Plural, Trans, useLingui } from "@lingui/react/macro"
import {
  ArrowBigUpIcon,
  CheckIcon,
  CircleCheckIcon,
  CircleXIcon,
  EyeIcon,
  EyeOffIcon,
  ExternalLinkIcon,
  LockIcon,
  LockOpenIcon,
  MessagesSquareIcon,
  PencilIcon,
  ReplyIcon,
  ShieldAlertIcon,
  XIcon,
} from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"
import { Link, useLocation, useParams } from "react-router"

import { Dot, ErrorLine, Notice, toast } from "@/components/common"
import { Pane, PaneBack } from "@/components/common/Column"
import { AuthorAvatar, useAuthorName } from "@/components/common/DiscussionAuthor"
import { DISCUSSION_KEYS, keyLabel, mod } from "@/components/common/ShortcutSheet"
import { formatDateTime, useErrorText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { pageLabel } from "@/features/docs/PageContext"
import { AuthorName, PostBody, StateTag, TeamTag, ThreadTags, useModeration } from "@/features/docs/discussions/parts"
import { useAnswer, useDiscussion, useReply, useUpdateDiscussion } from "@/features/docs/discussions/queries"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { isGone, type Discussion, type DiscussionPost } from "@/lib/api"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

const hint = (label: string, key: string) => `${label} (${keyLabel(key).join("")})`

function openingPost(d: Discussion): DiscussionPost {
  return {
    id: d.opening_post_id,
    discussion_id: d.id,
    opening: true,
    body: d.body,
    body_html: d.body_html,
    author: d.author,
    state: d.state,
    votes: d.votes,
    accepted: false,
    deleted: !!d.deleted,
    created_at: d.created_at,
    edited_at: d.edited_at,
  }
}

function orderPosts(posts: DiscussionPost[]) {
  const top = posts.filter((p) => !p.parent_post_id)
  const children = new Map<string, DiscussionPost[]>()
  for (const p of posts) {
    if (!p.parent_post_id) continue
    const list = children.get(p.parent_post_id) ?? []
    list.push(p)
    children.set(p.parent_post_id, list)
  }
  return top.map((p) => ({ post: p, replies: children.get(p.id) ?? [] }))
}

type PostActions = {
  reply: (p: DiscussionPost) => void
  answer: (p: DiscussionPost) => void
  moderate: (p: DiscussionPost, to: "published" | "hidden" | "spam") => void
}

function PostCard({
  post,
  selected,
  actions,
  nested,
}: {
  post: DiscussionPost
  selected: boolean
  actions: PostActions
  nested?: boolean
}) {
  const { t, i18n } = useLingui()
  const fmt = new Intl.NumberFormat(i18n.locale)
  const votes = fmt.format(post.votes)
  const when = formatDateTime(post.created_at, i18n.locale)
  const answerable = !post.opening && !nested && post.state === "published" && !post.deleted
  return (
    <article
      className={cn(
        "flex flex-col gap-2.5 rounded-2xl border bg-card px-4 py-3.5",
        post.accepted && "border-success/60",
        post.state === "hidden" && "bg-surface",
        selected && "ring-2 ring-ring"
      )}
      data-testid={post.opening ? "opening-post" : "discussion-post"}
      data-post-id={post.id}
      data-state={post.state}
      data-accepted={post.accepted || undefined}
    >
      {post.accepted && (
        <p className="-mt-0.5 inline-flex items-center gap-1.5 text-small font-medium text-success">
          <CircleCheckIcon className="size-4" />
          <Trans>Accepted answer</Trans>
        </p>
      )}
      <header className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-body">
        <AuthorAvatar author={post.author} className="size-7" />
        <AuthorName author={post.author} />
        {post.author.kind === "member" && !post.author.deleted && <TeamTag />}
        <time className="text-caption text-faint" dateTime={post.created_at}>
          {when}
        </time>
        {post.edited_at && (
          <span className="text-caption text-faint">
            <Trans>edited</Trans>
          </span>
        )}
        <StateTag state={post.state} />
        <span
          className={cn(
            "ms-auto inline-flex items-center gap-1 text-small text-muted-foreground tabular-nums",
            post.votes === 0 && "opacity-45"
          )}
          title={t`${votes} votes`}
        >
          <ArrowBigUpIcon className="size-4" />
          <span className="sr-only">{t`${votes} votes`}</span>
          <span aria-hidden>{votes}</span>
        </span>
      </header>
      <PostBody post={post} className={cn(post.state === "hidden" && "opacity-60")} />
      <footer className="-mx-2 -mb-1 flex flex-wrap items-center gap-0.5">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => actions.reply(post)}
          title={post.opening ? hint(t`Reply`, DISCUSSION_KEYS.reply) : hint(t`Reply`, DISCUSSION_KEYS.replyTo)}
          data-testid="post-reply"
        >
          <ReplyIcon />
          <Trans>Reply</Trans>
        </Button>
        {answerable && (
          <Button
            variant="ghost"
            size="sm"
            className={cn(!post.accepted && "text-success hover:text-success")}
            onClick={() => actions.answer(post)}
            title={
              post.accepted
                ? hint(t`Clear the answer`, DISCUSSION_KEYS.answer)
                : hint(t`Mark as answer`, DISCUSSION_KEYS.answer)
            }
            data-testid="post-answer"
          >
            {post.accepted ? <CircleXIcon /> : <CircleCheckIcon />}
            {post.accepted ? <Trans>Clear the answer</Trans> : <Trans>Mark as answer</Trans>}
          </Button>
        )}
        <span className="ms-auto flex flex-wrap items-center gap-0.5">
          {post.state !== "published" && (
            <Button
              variant="ghost"
              size="sm"
              className="text-success hover:text-success"
              onClick={() => actions.moderate(post, "published")}
              title={hint(post.state === "pending" ? t`Approve` : t`Show`, DISCUSSION_KEYS.approve)}
              data-testid="post-approve"
            >
              {post.state === "pending" ? <CheckIcon /> : <EyeIcon />}
              {post.state === "pending" ? <Trans>Approve</Trans> : <Trans>Show</Trans>}
            </Button>
          )}
          {post.state !== "hidden" && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => actions.moderate(post, "hidden")}
              title={hint(t`Hide`, DISCUSSION_KEYS.hide)}
              data-testid="post-hide"
            >
              <EyeOffIcon />
              <Trans>Hide</Trans>
            </Button>
          )}
          {post.author.kind !== "member" && !post.deleted && (
            <Button
              variant="ghost"
              size="sm"
              className="text-destructive hover:text-destructive"
              onClick={() => actions.moderate(post, "spam")}
              title={hint(t`Spam`, DISCUSSION_KEYS.spam)}
              data-testid="post-spam"
            >
              <ShieldAlertIcon />
              <Trans>Spam</Trans>
            </Button>
          )}
        </span>
      </footer>
    </article>
  )
}

function TitleEditor({ d, onDone }: { d: Discussion; onDone: () => void }) {
  const { t } = useLingui()
  const update = useUpdateDiscussion(d.id)
  const [title, setTitle] = useState(d.title)
  const value = title.trim()
  const valid = value.length >= 3 && value.length <= 200
  return (
    <form
      className="flex flex-col gap-2"
      onSubmit={(e) => {
        e.preventDefault()
        if (!valid) return
        if (value === d.title) return onDone()
        update.mutate({ title: value }, { onSuccess: onDone })
      }}
    >
      <div className="flex flex-wrap items-center gap-2">
        <Input
          autoFocus
          value={title}
          maxLength={200}
          onChange={(e) => setTitle(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              e.preventDefault()
              onDone()
            }
          }}
          aria-label={t`Title`}
          className="h-10 min-w-0 flex-1 text-title"
          data-testid="title-input"
        />
        <Button type="submit" disabled={!valid || update.isPending} data-testid="title-save">
          <Trans>Save</Trans>
        </Button>
        <Button type="button" variant="ghost" onClick={onDone}>
          <Trans>Cancel</Trans>
        </Button>
      </div>
      <ErrorLine error={update.error} />
    </form>
  )
}

function Composer({
  d,
  replyTo,
  onClearReplyTo,
  inputRef,
}: {
  d: Discussion
  replyTo: DiscussionPost | null
  onClearReplyTo: () => void
  inputRef: React.RefObject<HTMLTextAreaElement | null>
}) {
  const { t } = useLingui()
  const reply = useReply(d.id)
  const authorName = useAuthorName()
  const [body, setBody] = useState("")
  const send = () => {
    const text = body.trim()
    if (!text || reply.isPending) return
    reply.mutate(
      { body: text, parent_post_id: replyTo?.id },
      {
        onSuccess: () => {
          setBody("")
          onClearReplyTo()
          toast(t`Reply posted`)
        },
      }
    )
  }
  const to = replyTo ? authorName(replyTo.author) : ""
  return (
    <section className="flex flex-col gap-2" aria-label={t`Reply`} data-testid="discussion-composer">
      {d.locked && (
        <Notice icon={LockIcon}>
          <p>
            <Trans>Locked: readers can no longer write here. You still can.</Trans>
          </p>
        </Notice>
      )}
      <div className="flex flex-col gap-2 rounded-2xl border bg-card p-3 focus-within:border-faint">
        {replyTo && (
          <p className="flex items-center gap-1.5 text-small text-muted-foreground" data-testid="reply-to">
            <ReplyIcon className="size-3.5" />
            <span className="min-w-0 truncate">
              <Trans>Replying to {to}</Trans>
            </span>
            <Button
              variant="ghost"
              size="icon-xs"
              onClick={onClearReplyTo}
              aria-label={t`Reply to the discussion instead`}
            >
              <XIcon />
            </Button>
          </p>
        )}
        <Textarea
          ref={inputRef}
          value={body}
          onChange={(e) => setBody(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
              e.preventDefault()
              send()
            } else if (e.key === "Escape") {
              e.preventDefault()
              e.currentTarget.blur()
            }
          }}
          rows={4}
          maxLength={20000}
          placeholder={t`Write a public reply. Markdown works.`}
          aria-label={t`Reply`}
          className="min-h-24 border-0 bg-transparent px-1 shadow-none focus-visible:ring-0 dark:bg-transparent"
          data-testid="discussion-reply-input"
        />
        <ErrorLine error={reply.error} />
        <div className="flex flex-wrap items-center justify-between gap-2">
          <span className="text-caption text-faint">
            <Trans>Published at once and sent to the followers.</Trans>
          </span>
          <Button
            onClick={send}
            disabled={!body.trim() || reply.isPending}
            title={t`Reply (${mod}↵)`}
            data-testid="discussion-reply-send"
          >
            <Trans>Reply</Trans>
          </Button>
        </div>
      </div>
    </section>
  )
}

export function ThreadPane() {
  const { discussionId = "" } = useParams()
  const location = useLocation()
  const { t, i18n } = useLingui()
  const errorText = useErrorText()
  const detail = useDiscussion(discussionId)
  const from = (location.state as { from?: string } | null)?.from
  const back = from?.startsWith("/docs/page")
    ? { to: from, label: t`Page` }
    : from === "/docs/discussions/queue"
      ? { to: from, label: t`Waiting for approval` }
      : { to: "/docs/discussions", label: t`Discussions` }
  const inboxes = useInboxes().data ?? []
  const d = detail.data?.discussion
  const inbox = d ? inboxes.find((i) => i.id === d.inbox_id) : undefined
  const update = useUpdateDiscussion(discussionId)
  const answer = useAnswer(discussionId)
  const moderation = useModeration()
  const authorName = useAuthorName()
  const [editing, setEditing] = useState(false)
  const [replyTo, setReplyTo] = useState<DiscussionPost | null>(null)
  const [cursor, setCursor] = useState(-1)
  const input = useRef<HTMLTextAreaElement>(null)
  const list = useRef<HTMLDivElement>(null)

  const groups = useMemo(() => orderPosts(detail.data?.posts ?? []), [detail.data?.posts])
  const flat = useMemo(() => (d ? [openingPost(d), ...groups.flatMap((g) => [g.post, ...g.replies])] : []), [d, groups])
  useEffect(() => setCursor(-1), [discussionId])
  useEffect(() => {
    if (cursor >= 0)
      list.current?.querySelectorAll("[data-post-id]")[cursor]?.scrollIntoView({ block: "nearest", behavior: "smooth" })
  }, [cursor])

  const fail = (e: unknown) => toast(errorText(e))
  const focusComposer = () => requestAnimationFrame(() => input.current?.focus())
  const parentOf = (p: DiscussionPost) =>
    p.opening ? null : p.parent_post_id ? (flat.find((x) => x.id === p.parent_post_id) ?? null) : p
  const actions: PostActions = {
    reply: (p) => {
      setReplyTo(parentOf(p))
      focusComposer()
    },
    answer: (p) => {
      if (p.opening || p.parent_post_id || p.state !== "published") return
      answer.mutate(p.accepted ? null : p.id, {
        onSuccess: () => toast(p.accepted ? t`Answer cleared` : t`Marked as the answer`),
        onError: fail,
      })
    },
    moderate: moderation.act,
  }
  const toggleLock = () => {
    if (!d) return
    update.mutate({ locked: !d.locked }, { onSuccess: (x) => toast(x.locked ? t`Locked` : t`Unlocked`), onError: fail })
  }
  const selected = cursor >= 0 ? flat[cursor] : flat[0]
  const down = () => setCursor((i) => Math.min(flat.length - 1, i + 1))
  const up = () => setCursor((i) => Math.max(0, i - 1))
  useHotkeys(
    {
      [DISCUSSION_KEYS.next]: down,
      [DISCUSSION_KEYS.previous]: up,
      ArrowDown: down,
      ArrowUp: up,
      [DISCUSSION_KEYS.reply]: () => {
        setReplyTo(null)
        focusComposer()
      },
      [DISCUSSION_KEYS.replyTo]: () => selected && actions.reply(selected),
      [DISCUSSION_KEYS.answer]: () => selected && actions.answer(selected),
      [DISCUSSION_KEYS.approve]: () => selected && actions.moderate(selected, "published"),
      [DISCUSSION_KEYS.hide]: () => selected && actions.moderate(selected, "hidden"),
      [DISCUSSION_KEYS.spam]: () => selected && actions.moderate(selected, "spam"),
      [DISCUSSION_KEYS.title]: () => setEditing(true),
      [DISCUSSION_KEYS.lock]: toggleLock,
      [DISCUSSION_KEYS.page]: () => d && window.open(d.page, "_blank", "noopener,noreferrer"),
    },
    !!d && !editing
  )

  if (detail.isPending)
    return (
      <Pane testId="discussion">
        <Skeleton className="h-8 w-2/3" />
        <Skeleton className="h-40 rounded-2xl" />
      </Pane>
    )
  if (!d)
    return (
      <Pane testId="discussion">
        <PaneBack to={back.to} label={back.label} always />
        {isGone(detail.error) ? (
          <p className="text-body text-muted-foreground">
            <Trans>This discussion no longer exists or you cannot see it.</Trans>
          </p>
        ) : (
          <ErrorLine error={detail.error} />
        )}
      </Pane>
    )

  const started = formatDateTime(d.created_at, i18n.locale)
  const starter = authorName(d.author)
  return (
    <Pane testId="discussion" key={d.id}>
      <header className="flex flex-col gap-2.5">
        <PaneBack to={back.to} label={back.label} always />
        {editing ? (
          <TitleEditor d={d} onDone={() => setEditing(false)} />
        ) : (
          <div className="flex items-start gap-2">
            <h1 className="min-w-0 flex-1 text-page break-words" data-testid="discussion-title">
              {d.title}
            </h1>
            <Button
              variant="ghost"
              size="icon-sm"
              className="mt-0.5 text-faint"
              onClick={() => setEditing(true)}
              aria-label={t`Edit the title`}
              title={hint(t`Edit the title`, DISCUSSION_KEYS.title)}
              data-testid="title-edit"
            >
              <PencilIcon />
            </Button>
          </div>
        )}
        <ThreadTags d={d} />
        <p className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-body text-muted-foreground">
          <span>
            <Trans>
              Started by {starter} · {started}
            </Trans>
          </span>
          {inbox && (
            <span className="inline-flex items-center gap-1.5">
              <Dot color={inbox.branding.color} className="size-[7px] rounded-[2px]" />
              {inbox.name}
            </span>
          )}
          {d.conversation_id && (
            <Link
              to={`/conversations/${d.conversation_id}`}
              className="text-brand underline-offset-4 hover:underline"
              data-testid="discussion-conversation"
            >
              <Trans>From a conversation</Trans>
            </Link>
          )}
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            render={<a href={d.page} target="_blank" rel="noreferrer" />}
            title={hint(t`Open on its page`, DISCUSSION_KEYS.page)}
            className="min-w-0"
            data-testid="discussion-page"
          >
            <ExternalLinkIcon />
            <span className="truncate" dir="ltr">
              {pageLabel(d.page, d.page_title)}
            </span>
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={toggleLock}
            disabled={update.isPending}
            title={hint(d.locked ? t`Unlock` : t`Lock`, DISCUSSION_KEYS.lock)}
            data-testid="discussion-lock"
          >
            {d.locked ? <LockOpenIcon /> : <LockIcon />}
            {d.locked ? <Trans>Unlock</Trans> : <Trans>Lock</Trans>}
          </Button>
        </div>
      </header>
      <div ref={list} className="flex flex-col gap-3">
        <PostCard post={flat[0]} selected={cursor === 0} actions={actions} />
        <h2 className="mx-1 mt-3 inline-flex items-center gap-1.5 text-small font-medium text-faint">
          <MessagesSquareIcon className="size-4" />
          <Plural value={d.reply_count} one="# reply" other="# replies" />
        </h2>
        {groups.length === 0 && (
          <p className="mx-1 text-body text-faint">
            <Trans>No replies yet.</Trans>
          </p>
        )}
        {groups.map((g) => (
          <div key={g.post.id} className="flex flex-col gap-2">
            <PostCard post={g.post} selected={flat[cursor]?.id === g.post.id} actions={actions} />
            {g.replies.length > 0 && (
              <div className="ms-5 flex flex-col gap-2 border-s ps-4 phone:ms-2 phone:ps-3">
                {g.replies.map((r) => (
                  <PostCard key={r.id} post={r} nested selected={flat[cursor]?.id === r.id} actions={actions} />
                ))}
              </div>
            )}
          </div>
        ))}
      </div>
      <Composer d={d} replyTo={replyTo} onClearReplyTo={() => setReplyTo(null)} inputRef={input} />
      {moderation.dialog}
    </Pane>
  )
}

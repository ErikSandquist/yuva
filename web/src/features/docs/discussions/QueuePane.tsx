import { Trans, useLingui } from "@lingui/react/macro"
import { CheckIcon, EyeOffIcon, ShieldAlertIcon } from "lucide-react"
import { useEffect, useRef } from "react"
import { Link, useNavigate } from "react-router"

import { ErrorLine } from "@/components/common"
import { Pane, PaneBack } from "@/components/common/Column"
import { AuthorAvatar } from "@/components/common/DiscussionAuthor"
import { DISCUSSION_KEYS, keyLabel } from "@/components/common/ShortcutSheet"
import { formatShort } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { DocsSitePicker, usePickedSite } from "@/features/docs/DocsColumn"
import { pageLabel } from "@/features/docs/PageContext"
import { AuthorName, ListSkeleton, LoadMore, PostBody, TeamTag, useModeration } from "@/features/docs/discussions/parts"
import { threadLink, useQueue } from "@/features/docs/discussions/queries"
import { ideaLink } from "@/features/roadmap/queries"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { Card, EmptyRow } from "@/features/settings/ui"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { useResetState } from "@/hooks/use-reset-state"
import type { Discussion, DiscussionPostItem } from "@/lib/api"
import { cn } from "@/lib/utils"

const FROM = "/docs/discussions/queue"

const hrefOf = (d: Discussion) => (d.kind === "idea" && d.board_id ? ideaLink(d.board_id, d.id) : threadLink(d.id))

function QueueItem({
  item,
  cursor,
  onAct,
}: {
  item: DiscussionPostItem
  cursor: boolean
  onAct: (to: "published" | "hidden" | "spam") => void
}) {
  const { t, i18n } = useLingui()
  const { post, discussion: d } = item
  const hint = (label: string, key: string) => `${label} (${keyLabel(key).join("")})`
  return (
    <li
      className={cn("flex flex-col gap-2.5 px-4 py-3.5", cursor && "bg-muted/60 ring-2 ring-ring ring-inset")}
      data-testid="queue-item"
      data-post-id={post.id}
    >
      <div className="flex min-w-0 flex-col gap-0.5">
        <span className="text-caption font-medium text-faint">
          {post.opening ? (
            d.kind === "idea" ? (
              <Trans>New idea</Trans>
            ) : (
              <Trans>New discussion</Trans>
            )
          ) : (
            <Trans>Reply</Trans>
          )}
        </span>
        <Link
          to={hrefOf(d)}
          state={{ from: FROM }}
          className="min-w-0 truncate text-body font-medium hover:underline"
          data-testid="queue-thread"
        >
          {d.title}
        </Link>
        {d.page && (
          <span className="truncate text-caption text-faint" dir="ltr">
            {pageLabel(d.page, d.page_title)}
          </span>
        )}
      </div>
      <div className="flex min-w-0 items-center gap-2 text-body">
        <AuthorAvatar author={post.author} className="size-6" />
        <AuthorName author={post.author} />
        {post.author.kind === "member" && <TeamTag />}
        <time className="ms-auto shrink-0 text-caption text-faint" dateTime={post.created_at}>
          {formatShort(post.created_at, i18n.locale)}
        </time>
      </div>
      <PostBody post={post} className="max-h-60 overflow-y-auto" />
      <div className="-mx-2 flex flex-wrap items-center gap-0.5">
        <Button
          variant="ghost"
          size="sm"
          className="text-success hover:text-success"
          onClick={() => onAct("published")}
          title={hint(t`Approve`, DISCUSSION_KEYS.approve)}
          data-testid="queue-approve"
        >
          <CheckIcon />
          <Trans>Approve</Trans>
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onAct("hidden")}
          title={hint(t`Hide`, DISCUSSION_KEYS.hide)}
          data-testid="queue-hide"
        >
          <EyeOffIcon />
          <Trans>Hide</Trans>
        </Button>
        {post.author.kind !== "member" && (
          <Button
            variant="ghost"
            size="sm"
            className="text-destructive hover:text-destructive"
            onClick={() => onAct("spam")}
            title={hint(t`Spam`, DISCUSSION_KEYS.spam)}
            data-testid="queue-spam"
          >
            <ShieldAlertIcon />
            <Trans>Spam</Trans>
          </Button>
        )}
      </div>
    </li>
  )
}

export function QueuePane() {
  const { t } = useLingui()
  const navigate = useNavigate()
  const [inboxId] = useInboxFilter()
  const site = usePickedSite(inboxId)
  const queue = useQueue({ inbox_id: inboxId, site })
  const items = queue.data?.pages.flatMap((p) => p.items) ?? []
  const moderation = useModeration()
  const [picked, setCursor] = useResetState(-1, JSON.stringify([inboxId, site]))
  const cursor = Math.min(picked, items.length - 1)
  const list = useRef<HTMLUListElement>(null)
  useEffect(() => {
    if (cursor >= 0)
      list.current?.querySelectorAll("[data-testid=queue-item]")[cursor]?.scrollIntoView({ block: "nearest" })
  }, [cursor])
  const target = items[Math.max(0, cursor)]
  const act = (to: "published" | "hidden" | "spam") => target && moderation.act(target.post, to)
  const down = () => setCursor(Math.min(items.length - 1, cursor + 1))
  const up = () => setCursor(Math.max(0, cursor - 1))
  useHotkeys({
    [DISCUSSION_KEYS.next]: down,
    [DISCUSSION_KEYS.previous]: up,
    ArrowDown: down,
    ArrowUp: up,
    Enter: () => target && navigate(hrefOf(target.discussion), { state: { from: FROM } }),
    [DISCUSSION_KEYS.approve]: () => act("published"),
    [DISCUSSION_KEYS.hide]: () => act("hidden"),
    [DISCUSSION_KEYS.spam]: () => act("spam"),
  })
  return (
    <Pane testId="discussion-queue">
      <header className="flex flex-col">
        <PaneBack to="/docs/discussions" label={t`Discussions`} always />
        <h1 className="min-w-0 text-page break-words">
          <Trans>Waiting for approval</Trans>
        </h1>
        <p className="mt-1.5 text-reading text-muted-foreground">
          <Trans>
            Readers' posts wait here until a member approves one of theirs; after that, they publish at once. Oldest
            first.
          </Trans>
        </p>
      </header>
      <DocsSitePicker className="hidden phone:flex" />
      <Card flush data-testid="queue">
        {queue.isPending ? (
          <ListSkeleton />
        ) : queue.error ? (
          <ErrorLine error={queue.error} className="p-4" />
        ) : items.length === 0 ? (
          <EmptyRow>
            <Trans>Nothing is waiting for approval.</Trans>
          </EmptyRow>
        ) : (
          <ul ref={list} className="divide-y">
            {items.map((it, i) => (
              <QueueItem
                key={it.post.id}
                item={it}
                cursor={i === cursor}
                onAct={(to) => {
                  setCursor(i)
                  moderation.act(it.post, to)
                }}
              />
            ))}
          </ul>
        )}
        <LoadMore list={queue} />
      </Card>
      {moderation.dialog}
    </Pane>
  )
}

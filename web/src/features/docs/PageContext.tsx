import { Trans, useLingui } from "@lingui/react/macro"
import {
  BadgeCheckIcon,
  ExternalLinkIcon,
  EyeOffIcon,
  GlobeIcon,
  MessageCircleQuestionMarkIcon,
  PencilIcon,
  ThumbsDownIcon,
  ThumbsUpIcon,
  UploadIcon,
} from "lucide-react"
import { useState } from "react"

import { keyLabel, SHORTCUTS } from "@/components/common/ShortcutSheet"
import { Button } from "@/components/ui/button"
import { pageLabel, PublishDialog, useAnswerActions } from "@/features/docs/AnswerDialog"
import { usePageAnswers } from "@/features/docs/queries"
import type { Conversation, Message, PageRating } from "@/lib/api"
import { cn } from "@/lib/utils"

const small = "h-6 gap-1 px-2 text-caption [&_svg]:size-3.5"

export function PageLink({ url, title, className }: { url: string; title?: string; className?: string }) {
  return (
    <a
      href={url}
      target="_blank"
      rel="noreferrer"
      title={url}
      className={cn(
        "min-w-0 truncate text-muted-foreground underline-offset-4 hover:text-foreground hover:underline",
        className
      )}
      data-testid="page-link"
    >
      {pageLabel(url, title)}
    </a>
  )
}

export function PageRatingMark({ rating }: { rating: PageRating }) {
  return rating === "up" ? (
    <span className="inline-flex shrink-0 items-center gap-1 text-success" data-testid="page-rating" data-rating="up">
      <ThumbsUpIcon className="size-3.5" />
      <Trans>Helpful</Trans>
    </span>
  ) : (
    <span
      className="inline-flex shrink-0 items-center gap-1 text-destructive"
      data-testid="page-rating"
      data-rating="down"
    >
      <ThumbsDownIcon className="size-3.5" />
      <Trans>Not helpful</Trans>
    </span>
  )
}

const isMemberReply = (m: Message) =>
  m.kind === "message" && m.direction === "out" && m.author.type === "member" && !m.draft

export function usePagePublishing(c: Conversation, items: Message[], allLoaded: boolean) {
  const { t } = useLingui()
  const question = c.kind === "question" ? c.question : undefined
  const answers = usePageAnswers({ conversation_id: c.id }, !!question)
  const published = answers.data?.pages[0]?.items[0]
  const reply = items.findLast(isMemberReply)
  const asked = allLoaded ? items.find((m) => m.kind === "message" && m.direction !== "out") : undefined
  const [publishing, setPublishing] = useState<string | null>(null)
  const actions = useAnswerActions()
  const ready = !!question && !answers.isPending
  const run = () => {
    if (!ready) return
    if (published) actions.edit(published)
    else if (reply) setPublishing(reply.body.trim())
  }
  const key = keyLabel(SHORTCUTS.publish).join("")

  const line = question ? (
    <span className="inline-flex min-w-0 items-center gap-1.5" data-testid="question-page">
      <MessageCircleQuestionMarkIcon className="size-3.5 shrink-0" />
      <span className="min-w-0 truncate">
        <Trans>
          Asked on <PageLink url={question.page_url} title={question.page_title} />
        </Trans>
      </span>
      {ready && !published && !reply && (
        <span className="shrink-0">
          · <Trans>reply first to publish it</Trans>
        </span>
      )}
    </span>
  ) : c.kind === "feedback" && c.feedback?.page_url ? (
    <span className="inline-flex min-w-0 items-center gap-1.5" data-testid="feedback-page">
      <GlobeIcon className="size-3.5 shrink-0" />
      <PageLink url={c.feedback.page_url} title={c.feedback.page_title} />
      {c.feedback.rating && (
        <>
          <span aria-hidden>·</span>
          <PageRatingMark rating={c.feedback.rating} />
        </>
      )}
    </span>
  ) : null

  const banner =
    ready && published ? (
      <div
        role="status"
        className="flex shrink-0 flex-wrap items-center gap-x-2 gap-y-1 border-b bg-success/8 py-1.5 ps-4 pe-2 text-small"
        data-testid="publish-banner"
        data-state="published"
      >
        <BadgeCheckIcon className="size-4 shrink-0 text-success" />
        <span className="font-medium" data-testid="question-published">
          <Trans>On the page</Trans>
        </span>
        <a
          href={published.page}
          target="_blank"
          rel="noreferrer"
          className="inline-flex min-w-0 items-center gap-1 text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
          data-testid="publish-banner-link"
        >
          <span className="truncate">{pageLabel(published.page, published.title || question?.page_title)}</span>
          <ExternalLinkIcon className="size-3.5 shrink-0" />
        </a>
        <span className="ms-auto flex shrink-0 items-center gap-0.5">
          <Button
            variant="ghost"
            className={small}
            onClick={() => actions.edit(published)}
            title={t`Edit the published answer (${key})`}
            data-testid="question-edit"
          >
            <PencilIcon />
            <Trans>Edit</Trans>
          </Button>
          <Button
            variant="ghost"
            className={cn(small, "text-destructive hover:text-destructive")}
            onClick={() => actions.unpublish(published)}
            data-testid="question-unpublish"
          >
            <Trans>Unpublish</Trans>
          </Button>
        </span>
      </div>
    ) : ready && reply ? (
      <div
        role="status"
        className="flex shrink-0 flex-wrap items-center gap-x-2 gap-y-1 border-b bg-warning/8 py-1.5 ps-4 pe-2 text-small"
        data-testid="publish-banner"
        data-state="private"
      >
        <EyeOffIcon className="size-4 shrink-0 text-warning" />
        <span className="min-w-0 flex-1 font-medium">
          <Trans>Answered privately — not on the page yet</Trans>
        </span>
        <Button
          variant="outline"
          className={cn(small, "shrink-0 bg-background")}
          onClick={run}
          title={t`Publish to the page (${key})`}
          data-testid="question-publish"
        >
          <UploadIcon />
          <Trans>Publish</Trans>
        </Button>
      </div>
    ) : null

  const sendAndPublish = ready && !published ? (answer: string) => setPublishing(answer) : undefined

  const dialogs = (
    <>
      {actions.dialogs}
      {publishing !== null && question && (
        <PublishDialog
          conversationId={c.id}
          page={question.page_url}
          initial={{ question: asked?.body.trim() || c.subject, answer: publishing }}
          onClose={() => setPublishing(null)}
        />
      )}
    </>
  )
  return { line, banner, dialogs, run, sendAndPublish }
}

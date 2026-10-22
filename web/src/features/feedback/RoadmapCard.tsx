import { Trans, useLingui } from "@lingui/react/macro"
import { ArrowBigUpIcon, LightbulbIcon } from "lucide-react"
import { Link } from "react-router"

import { toast } from "@/components/common"
import { useEnumText, useErrorText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { useDiscussion } from "@/features/docs/discussions/queries"
import { Overline, look } from "@/features/inbox/PageFrame"
import { boardOfInbox, ideaLink, useBoards, useLinkIdea } from "@/features/roadmap/queries"
import type { Conversation } from "@/lib/api"

export function RoadmapCard({ c, onLink, onNew }: { c: Conversation; onLink: () => void; onNew: () => void }) {
  const { t } = useLingui()
  const text = useEnumText()
  const errorText = useErrorText()
  const { unlink } = useLinkIdea()
  const boards = useBoards()
  const idea = c.idea
  const detail = useDiscussion(idea?.id ?? "").data?.discussion
  const board = boards.data?.find((b) => b.id === detail?.board_id) ?? boardOfInbox(boards.data, c.inbox_id)
  const others = Math.max(0, (detail?.linked_count ?? 1) - 1)
  const votes = detail?.votes ?? 0
  return (
    <section className="flex flex-col gap-2.5 border-b px-5 py-4" data-testid="feedback-roadmap">
      <Overline>
        <Trans>Roadmap</Trans>
      </Overline>
      {idea ? (
        <>
          <div className="flex flex-col gap-2.5 rounded-[10px] border bg-surface p-3" data-testid="idea-chip">
            <div className="flex items-start gap-2.5">
              <span className="grid size-7 shrink-0 place-items-center rounded-lg bg-brand-wash text-brand">
                <LightbulbIcon className="size-[15px]" />
              </span>
              <span className="flex min-w-0 flex-1 flex-col leading-[1.35]">
                <span className="font-semibold break-words" dir="auto">
                  {idea.title}
                </span>
                <span className={look.faint}>
                  {board ? `${board.name} · ` : ""}
                  {t`linked with ${others} other feedback`}
                </span>
              </span>
              <span className={look.pill} data-testid="idea-stage">
                {text.stage[idea.stage]}
              </span>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <span className="inline-flex items-center gap-1 text-muted-foreground">
                <ArrowBigUpIcon className="size-3.5" aria-hidden />
                <span className="tabular-nums">{t`${votes} votes`}</span>
              </span>
              <span className="flex-1" />
              <Button variant="outline" className={look.btnSm} onClick={onLink} data-testid="idea-change">
                <Trans>Change</Trans>
              </Button>
              <Button
                variant="outline"
                className={look.btnSm}
                disabled={unlink.isPending}
                onClick={() =>
                  unlink.mutate({ ideaId: idea.id, conversationId: c.id }, { onError: (e) => toast(errorText(e)) })
                }
                data-testid="idea-chip-unlink"
              >
                <Trans>Unlink</Trans>
              </Button>
              <Button
                className={look.btnInkSm}
                render={<Link to={detail?.board_id ? ideaLink(detail.board_id, idea.id) : `/board/ideas/${idea.id}`} />}
                data-testid="idea-open"
              >
                <Trans>Open idea</Trans>
              </Button>
            </div>
          </div>
          <p className={look.faint}>
            <Trans>When this idea ships, you can answer everyone linked to it in one reply.</Trans>
          </p>
        </>
      ) : (
        <div className="flex flex-col gap-2.5 rounded-[10px] border border-dashed p-3" data-testid="roadmap-empty">
          <span className="text-muted-foreground">
            <Trans>Not on the roadmap yet. Link it to an idea, so you can tell everyone who asked when it ships.</Trans>
          </span>
          <div className="flex flex-wrap gap-2">
            <Button className={look.btnPSm} onClick={onLink} title={t`Link to an idea (B)`} data-testid="header-idea">
              <Trans>Link to an idea</Trans>
            </Button>
            <Button variant="outline" className={look.btnSm} onClick={onNew} data-testid="idea-new">
              <Trans>New idea from this</Trans>
            </Button>
          </div>
        </div>
      )}
    </section>
  )
}

import { useLingui } from "@lingui/react/macro"
import { LightbulbIcon, XIcon } from "lucide-react"
import { Link } from "react-router"

import { toast } from "@/components/common"
import { useEnumText, useErrorText } from "@/components/common/text"
import { useLinkIdea } from "@/features/roadmap/queries"
import type { IdeaRef } from "@/lib/api"
import { cn } from "@/lib/utils"

export function IdeaMark({ idea, className, pill }: { idea: IdeaRef; className?: string; pill?: boolean }) {
  const text = useEnumText()
  return (
    <span
      className={cn(
        "inline-flex min-w-0 shrink items-center gap-1 text-caption",
        pill ? "min-w-16 rounded-full bg-brand-wash px-2 py-px font-medium text-brand" : "text-muted-foreground",
        className
      )}
      title={`${idea.title} · ${text.stage[idea.stage]}`}
      data-testid="idea-mark"
    >
      <LightbulbIcon className={cn("size-3.5 shrink-0", !pill && "text-warning")} />
      <span className="truncate">{idea.title}</span>
    </span>
  )
}

export function IdeaChip({ conversationId, idea }: { conversationId: string; idea: IdeaRef }) {
  const { t } = useLingui()
  const text = useEnumText()
  const errorText = useErrorText()
  const { unlink } = useLinkIdea()
  const title = idea.title
  return (
    <span
      className="inline-flex max-w-full items-center rounded-full border bg-card text-caption text-muted-foreground"
      data-testid="idea-chip"
    >
      <Link
        to={`/board/ideas/${idea.id}`}
        className="inline-flex min-w-0 items-center gap-1 rounded-s-full py-px ps-2 pe-1 outline-none hover:text-foreground focus-visible:text-foreground"
        title={text.stage[idea.stage]}
      >
        <LightbulbIcon className="size-3 shrink-0" />
        <span className="truncate">{idea.title}</span>
      </Link>
      <button
        type="button"
        onClick={() => unlink.mutate({ ideaId: idea.id, conversationId }, { onError: (e) => toast(errorText(e)) })}
        disabled={unlink.isPending}
        className="grid size-5 shrink-0 place-items-center rounded-e-full outline-none hover:text-foreground focus-visible:text-foreground"
        aria-label={t`Unlink from ${title}`}
        title={t`Unlink from ${title}`}
        data-testid="idea-chip-unlink"
      >
        <XIcon className="size-3" />
      </button>
    </span>
  )
}

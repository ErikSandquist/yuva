import { Trans, useLingui } from "@lingui/react/macro"
import { ExternalLinkIcon } from "lucide-react"
import { useState } from "react"
import { Link } from "react-router"

import { ErrorLine, InboxMark, Segmented } from "@/components/common"
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
import { useInboxChannels } from "@/features/broadcasts/queries"
import { GUIDE_URL } from "@/features/docs/PagesTab"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { look } from "@/features/inbox/PageFrame"
import { CodeBlock, docsLines, type DocsStack } from "@/features/setup/InstallStep"
import { useInboxes } from "@/lib/workspace"

type Stack = DocsStack | "shadcn"

const SHADCN =
  "npx shadcn@latest add https://useyuva.com/r/yuva-discussions.json https://useyuva.com/r/yuva-page-feedback.json"

export function InstallDialog({ onClose }: { onClose: () => void }) {
  const { t } = useLingui()
  const inboxes = useInboxes().data ?? []
  const [filter] = useInboxFilter()
  const [picked, setPicked] = useState(filter)
  const inboxId = picked || inboxes[0]?.id || ""
  const channels = useInboxChannels(inboxId)
  const chats = (channels.data ?? []).filter((c) => c.chat)
  const [channelId, setChannelId] = useState("")
  const channel = chats.find((c) => c.id === channelId) ?? chats[0]
  const [stack, setStack] = useState<Stack>("html")
  const names: Record<Stack, string> = { html: "HTML", nuxt: "Nuxt", react: t`Fumadocs / React`, shadcn: "shadcn" }
  const code =
    channel?.chat && (stack === "shadcn" ? SHADCN : docsLines(stack, window.location.origin, channel.chat.public_key))
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        className="max-h-[90svh] grid-cols-[minmax(0,1fr)] overflow-y-auto sm:max-w-2xl"
        data-testid="docs-install-dialog"
      >
        <DialogHeader>
          <DialogTitle>
            <Trans>Add ratings and discussions to your docs</Trans>
          </DialogTitle>
          <DialogDescription>
            <Trans>
              Put these elements under every page of your documentation. They use a chat channel of the product, so the
              results land in its inbox.
            </Trans>
          </DialogDescription>
        </DialogHeader>
        {inboxes.length > 1 && (
          <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label={t`Inbox`}>
            {inboxes.map((i) => (
              <button
                key={i.id}
                type="button"
                aria-pressed={i.id === inboxId}
                className={i.id === inboxId ? look.chipOn : look.chip}
                onClick={() => {
                  setPicked(i.id)
                  setChannelId("")
                }}
              >
                <InboxMark name={i.name} color={i.branding.color} className="size-4 text-[9px]" />
                <span className="max-w-40 truncate">{i.name}</span>
              </button>
            ))}
          </div>
        )}
        {channels.isPending ? (
          <Skeleton className="h-32 w-full" />
        ) : channels.error ? (
          <ErrorLine error={channels.error} />
        ) : !channel ? (
          <p className="text-body text-muted-foreground" data-testid="docs-install-none">
            <Trans>
              This inbox has no chat channel yet. Add one under{" "}
              <Link to={`/setup/${inboxId}/add/chat`} className="text-brand hover:underline" onClick={onClose}>
                Add a channel
              </Link>
              .
            </Trans>
          </p>
        ) : (
          <>
            {chats.length > 1 && (
              <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label={t`Chat channel`}>
                {chats.map((c) => (
                  <button
                    key={c.id}
                    type="button"
                    aria-pressed={c.id === channel.id}
                    className={c.id === channel.id ? look.chipOn : look.chip}
                    onClick={() => setChannelId(c.id)}
                  >
                    <span className="max-w-48 truncate">{c.name}</span>
                  </button>
                ))}
              </div>
            )}
            <div className="max-w-full self-start overflow-x-auto [scrollbar-width:none]">
              <Segmented
                value={stack}
                onChange={setStack}
                label={t`Site generator`}
                items={(["html", "nuxt", "react", "shadcn"] as const).map((s) => ({
                  value: s,
                  label: names[s],
                  testId: `docs-install-${s}`,
                }))}
              />
            </div>
            <CodeBlock value={code || ""} label={names[stack]} testId="docs-install-snippet" />
          </>
        )}
        <DialogFooter>
          <Button
            variant="outline"
            className={look.btn}
            render={<a href={GUIDE_URL} target="_blank" rel="noreferrer" />}
            data-testid="docs-install-guide"
          >
            <Trans>Read the guide</Trans>
            <ExternalLinkIcon className="text-faint" />
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

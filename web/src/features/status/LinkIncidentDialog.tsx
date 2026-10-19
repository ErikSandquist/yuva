import { Trans, useLingui } from "@lingui/react/macro"
import { Command as CommandPrimitive } from "cmdk"
import { CalendarClockIcon, CheckIcon, FlameIcon } from "lucide-react"
import { useState } from "react"

import { ErrorLine, toast } from "@/components/common"
import { Command, CommandDialog, CommandEmpty, CommandGroup, CommandItem, CommandList } from "@/components/ui/command"
import { Skeleton } from "@/components/ui/skeleton"
import { useLinkMutations, useOpenIncidents, type OpenIncident } from "@/features/status/linked"
import { useStatusText } from "@/features/status/text"

const PARALLEL = 4

export function LinkIncidentDialog({
  conversationIds,
  linked,
  onClose,
  onDone,
}: {
  conversationIds: string[]
  linked?: Set<string>
  onClose: () => void
  onDone?: () => void
}) {
  const { t } = useLingui()
  const text = useStatusText()
  const open = useOpenIncidents(true)
  const { link } = useLinkMutations()
  const [q, setQ] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const many = conversationIds.length > 1

  const pick = async ({ page, incident }: OpenIncident) => {
    setBusy(true)
    setError(null)
    const queue = [...conversationIds]
    let done = 0
    let failure: unknown = null
    const worker = async () => {
      for (let id = queue.shift(); id && !failure; id = queue.shift()) {
        try {
          await link.mutateAsync({ pageId: page.id, incidentId: incident.id, conversationId: id })
          done++
        } catch (e) {
          failure = e
        }
      }
    }
    await Promise.all(Array.from({ length: Math.min(PARALLEL, queue.length) }, worker))
    setBusy(false)
    if (failure) {
      setError(failure)
      if (done > 0) toast(t`Linked so far: ${done}`)
      return
    }
    const title = incident.title
    toast(many ? t`Linked the selected conversations to ${title}` : t`Linked to ${title}`)
    onDone?.()
    onClose()
  }

  const pages = [...new Map(open.items.map((x) => [x.page.id, x.page])).values()]
  return (
    <CommandDialog
      open
      onOpenChange={(o) => !o && !busy && onClose()}
      title={t`Link to an incident`}
      description={t`Pick an open incident or maintenance`}
      className="top-[14vh] w-[560px] max-w-[calc(100%-32px)] gap-0 rounded-[18px]! border bg-card p-0 shadow-[0_30px_80px_-20px_rgb(0_0_0/0.35)] sm:max-w-[560px]"
    >
      <Command className="rounded-[18px]! bg-card p-0" data-testid="link-incident" loop>
        <CommandPrimitive.Input
          value={q}
          onValueChange={setQ}
          placeholder={many ? t`Link the selected conversations to…` : t`Link this conversation to an incident…`}
          aria-label={t`Search incidents`}
          className="w-full border-b bg-transparent px-[18px] py-4 text-base outline-none md:text-body placeholder:text-faint"
          data-testid="link-incident-search"
        />
        <CommandList className="max-h-[50vh] p-1.5">
          {open.pending ? (
            <div className="grid gap-2 p-2">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
            </div>
          ) : (
            <CommandEmpty className="px-6 py-10 text-faint">
              {open.pageCount === 0 ? (
                <Trans>No status page yet. Create one under Status first.</Trans>
              ) : open.items.length === 0 ? (
                <Trans>No incident or maintenance is open on any status page.</Trans>
              ) : (
                <Trans>No incident matches.</Trans>
              )}
            </CommandEmpty>
          )}
          {pages.map((page) => (
            <CommandGroup key={page.id} heading={page.name}>
              {open.items
                .filter((x) => x.page.id === page.id)
                .map(({ incident }) => {
                  const Icon = incident.kind === "maintenance" ? CalendarClockIcon : FlameIcon
                  const already = linked?.has(incident.id)
                  return (
                    <CommandItem
                      key={incident.id}
                      value={`${incident.title} ${page.name} ${incident.id}`}
                      disabled={busy || already}
                      onSelect={() => void pick({ page, incident })}
                      data-testid="link-incident-option"
                    >
                      <Icon className="text-muted-foreground" />
                      <span className="min-w-0 flex-1 truncate">{incident.title}</span>
                      <span className="shrink-0 text-caption text-faint">
                        {!incident.published_at ? t`Draft` : text.incident[incident.state]}
                      </span>
                      {already && <CheckIcon className="text-success" aria-label={t`Already linked`} />}
                    </CommandItem>
                  )
                })}
            </CommandGroup>
          ))}
        </CommandList>
        {(error !== null || busy) && (
          <div className="border-t px-4 py-2.5 text-small text-muted-foreground" role="status">
            {busy ? <Trans>Linking…</Trans> : <ErrorLine error={error} className="text-small" />}
          </div>
        )}
      </Command>
    </CommandDialog>
  )
}

import { Trans, useLingui } from "@lingui/react/macro"
import { LoaderIcon } from "lucide-react"
import { useState } from "react"
import { useNavigate } from "react-router"

import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { broadcastLink } from "@/features/broadcasts/broadcastQueries"
import { senderState } from "@/features/broadcasts/BroadcastParts"
import { useInboxChannels, useLists } from "@/features/broadcasts/queries"
import { useEntryMutations } from "@/features/changelog/queries"
import { ErrorLine } from "@/features/changelog/text"
import { useLocaleName } from "@/features/help/text"
import { ChoiceSelect, Field } from "@/features/settings/ui"
import type { Changelog, ChangelogEntry } from "@/lib/api"

const MAX_LISTS = 10

export function MailDialog({
  changelog,
  entry,
  locale,
  onClose,
}: {
  changelog: Changelog
  entry: ChangelogEntry
  locale: string
  onClose: () => void
}) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const localeName = useLocaleName()
  const m = useEntryMutations(changelog.id, entry.id)
  const lists = useLists(changelog.inbox_id, false)
  const channels = useInboxChannels(changelog.inbox_id).data ?? []
  const senders = channels.filter((c) => c.kind === "email" && senderState(c) === "ready")
  const mailable = (lists.data?.pages.flatMap((p) => p.items) ?? []).filter((l) => l.kind === "list")
  const published = entry.translations.filter((x) => x.published).map((x) => x.locale)
  const [language, setLanguage] = useState(published.includes(locale) ? locale : (published[0] ?? locale))
  const [chosen, setChosen] = useState<string[]>([])
  const [channel, setChannel] = useState("")
  const sender = channel || senders[0]?.id || ""
  const ok = chosen.length > 0 && chosen.length <= MAX_LISTS && published.length > 0
  const toggle = (id: string) => setChosen((cur) => (cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id]))
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent data-testid="mail-entry-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            if (!ok) return
            m.broadcast.mutate(
              { locale: language, list_ids: chosen, channel_id: sender || undefined },
              { onSuccess: (b) => navigate(broadcastLink(b.id)) }
            )
          }}
        >
          <DialogHeader>
            <DialogTitle>
              <Trans>Mail this to subscribers</Trans>
            </DialogTitle>
            <DialogDescription>
              <Trans>
                This makes a draft broadcast from the entry and opens it. Nothing is sent until you review it and send
                or schedule it.
              </Trans>
            </DialogDescription>
          </DialogHeader>
          <Field label={<Trans>Lists</Trans>} hint={<Trans>Pick up to 10 lists of this inbox.</Trans>}>
            {lists.isPending ? (
              <Skeleton className="h-16 w-full" />
            ) : mailable.length === 0 ? (
              <p className="rounded-xl border border-dashed px-4 py-4 text-body text-faint" data-testid="mail-no-lists">
                <Trans>This inbox has no mailing list yet. Create one under Broadcasts.</Trans>
              </p>
            ) : (
              <ul
                className="flex max-h-56 flex-col divide-y overflow-y-auto rounded-xl border"
                data-testid="mail-lists"
              >
                {mailable.map((l) => {
                  const subscribed = l.counts.subscribed
                  return (
                    <li key={l.id}>
                      <Label className="items-center gap-3 px-3 py-2.5 font-normal">
                        <Checkbox
                          checked={chosen.includes(l.id)}
                          disabled={!chosen.includes(l.id) && chosen.length >= MAX_LISTS}
                          onCheckedChange={() => toggle(l.id)}
                          data-testid="mail-list"
                        />
                        <span className="min-w-0 flex-1 truncate">{l.name}</span>
                        <span className="text-caption text-faint tabular-nums">
                          <Trans>Subscribed: {subscribed}</Trans>
                        </span>
                      </Label>
                    </li>
                  )
                })}
              </ul>
            )}
          </Field>
          <div className="grid gap-5 sm:grid-cols-2">
            <Field label={<Trans>Language</Trans>} htmlFor="mail-entry-locale">
              <ChoiceSelect
                id="mail-entry-locale"
                value={language}
                onChange={setLanguage}
                options={published.map((l) => [l, localeName(l)] as const)}
                label={t`Language`}
              />
            </Field>
            {senders.length > 1 && (
              <Field label={<Trans>Send through</Trans>} htmlFor="mail-entry-channel">
                <ChoiceSelect
                  id="mail-entry-channel"
                  value={sender}
                  onChange={setChannel}
                  options={senders.map((c) => [c.id, c.name] as const)}
                  label={t`Send through`}
                />
              </Field>
            )}
          </div>
          <ErrorLine error={m.broadcast.error} />
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              <Trans>Cancel</Trans>
            </Button>
            <Button type="submit" disabled={!ok || m.broadcast.isPending} data-testid="mail-entry-create">
              {m.broadcast.isPending && <LoaderIcon className="animate-spin" />}
              <Trans>Create draft broadcast</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

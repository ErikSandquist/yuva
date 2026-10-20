import { Trans, useLingui } from "@lingui/react/macro"
import { LoaderIcon, PlusIcon, SparklesIcon, TriangleAlertIcon } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { Link, useNavigate } from "react-router"

import { CHANGELOG_KEYS } from "@/components/common/ShortcutSheet"
import { formatDate, formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { entryLink, useCreateEntry, useEntries } from "@/features/changelog/queries"
import {
  entryTitle,
  EntryLabel,
  ErrorLine,
  LABELS,
  LABEL_TONES,
  useEntryStateText,
  useFailureText,
  useLabelText,
  useTranslationStateText,
} from "@/features/changelog/text"
import { useLocaleName } from "@/features/help/text"
import { ChoiceSelect, Field } from "@/features/settings/ui"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { Changelog, ChangelogEntrySummary, ChangelogLabel } from "@/lib/api"
import type { ChangelogEntryFilters } from "@/lib/keys"
import { cn } from "@/lib/utils"

type StateFilter = NonNullable<ChangelogEntryFilters["state"]> | "all"

function chipTone(state: string | undefined) {
  if (state === "published") return "border-success/40 bg-success/10 text-success"
  if (state === "changed") return "border-warning/50 bg-warning/10 text-warning"
  if (state === "draft") return "border-border bg-muted text-muted-foreground"
  return "border-dashed border-border text-faint"
}

function stateTone(state: ChangelogEntrySummary["state"]) {
  if (state === "published") return "bg-success/12 text-success"
  if (state === "scheduled") return "bg-brand-wash text-brand"
  return "bg-muted text-muted-foreground"
}

function LocaleChips({ e, changelog }: { e: ChangelogEntrySummary; changelog: Changelog }) {
  const { t } = useLingui()
  const name = useLocaleName()
  const stateText = useTranslationStateText()
  return (
    <span className="flex flex-wrap gap-1" data-testid="entry-locales">
      {changelog.locales.map((l) => {
        const tr = e.translations.find((x) => x.locale === l)
        const label = tr
          ? `${name(l)}: ${stateText[tr.state]}${tr.outdated ? `, ${t`out of date`}` : ""}`
          : `${name(l)}: ${t`not written`}`
        return (
          <Link
            key={l}
            to={entryLink(changelog.id, e.id, l)}
            title={label}
            aria-label={label}
            className={cn(
              "rounded-md border px-1.5 py-0.5 text-caption leading-none font-medium uppercase outline-none focus-visible:ring-2 focus-visible:ring-ring",
              chipTone(tr?.state),
              tr?.outdated && "underline decoration-warning decoration-2 underline-offset-2"
            )}
            data-testid={`entry-locale-${l}`}
          >
            {l}
          </Link>
        )
      })}
    </span>
  )
}

function NewEntryDialog({ changelog, onClose }: { changelog: Changelog; onClose: () => void }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const localeName = useLocaleName()
  const labelText = useLabelText()
  const create = useCreateEntry(changelog.id)
  const [title, setTitle] = useState("")
  const [locale, setLocale] = useState(changelog.default_locale)
  const [labels, setLabels] = useState<ChangelogLabel[]>(["new"])
  const ok = title.trim().length >= 3 && labels.length > 0
  const toggle = (l: ChangelogLabel) =>
    setLabels((cur) =>
      cur.includes(l) ? cur.filter((x) => x !== l) : LABELS.filter((x) => x === l || cur.includes(x))
    )
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent data-testid="new-entry-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            if (!ok) return
            create.mutate(
              { locale, title: title.trim(), labels },
              { onSuccess: (entry) => navigate(entryLink(changelog.id, entry.id, locale)) }
            )
          }}
        >
          <DialogHeader>
            <DialogTitle>
              <Trans>New entry</Trans>
            </DialogTitle>
            <DialogDescription>
              <Trans>Start with a title. You write the text next, and it stays a draft until it is published.</Trans>
            </DialogDescription>
          </DialogHeader>
          <Field label={<Trans>Title</Trans>} htmlFor="new-entry-title" hint={<Trans>At least 3 characters.</Trans>}>
            <Input
              id="new-entry-title"
              value={title}
              maxLength={200}
              autoFocus
              required
              onChange={(e) => setTitle(e.target.value)}
              data-testid="new-entry-title"
            />
          </Field>
          <Field label={<Trans>Labels</Trans>}>
            <div className="flex flex-wrap gap-2" role="group" aria-label={t`Labels`}>
              {LABELS.map((l) => (
                <button
                  key={l}
                  type="button"
                  aria-pressed={labels.includes(l)}
                  onClick={() => toggle(l)}
                  className={cn(
                    "rounded-lg border px-3 py-1.5 text-body font-medium outline-none focus-visible:ring-2 focus-visible:ring-ring",
                    labels.includes(l) ? LABEL_TONES[l] : "border-border text-muted-foreground"
                  )}
                  data-testid={`new-entry-label-${l}`}
                >
                  {labelText[l]}
                </button>
              ))}
            </div>
          </Field>
          <Field label={<Trans>Language</Trans>} htmlFor="new-entry-locale">
            <ChoiceSelect
              id="new-entry-locale"
              value={locale}
              onChange={setLocale}
              options={changelog.locales.map((l) => [l, localeName(l)] as const)}
              label={t`Language`}
              className="w-64"
            />
          </Field>
          <ErrorLine error={create.error} />
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              <Trans>Cancel</Trans>
            </Button>
            <Button type="submit" disabled={!ok || create.isPending} data-testid="create-entry">
              {create.isPending && <LoaderIcon className="animate-spin" />}
              <Trans>Create entry</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export function EntriesTab({ changelog }: { changelog: Changelog }) {
  const { t, i18n } = useLingui()
  const navigate = useNavigate()
  const stateText = useEntryStateText()
  const translationText = useTranslationStateText()
  const labelText = useLabelText()
  const failureText = useFailureText()
  const [state, setState] = useState<StateFilter>("all")
  const [label, setLabel] = useState<ChangelogLabel | "">("")
  const [creating, setCreating] = useState(false)
  const [cursor, setCursor] = useState(-1)
  const list = useRef<HTMLUListElement>(null)
  const filters: ChangelogEntryFilters = {
    state: state === "all" ? undefined : state,
    label: label || undefined,
  }
  const entries = useEntries(changelog.id, filters)
  const items = entries.data?.pages.flatMap((p) => p.items) ?? []
  const prefer = [i18n.locale, changelog.default_locale]

  useHotkeys({
    [CHANGELOG_KEYS.newEntry]: () => setCreating(true),
    j: () => setCursor((i) => Math.min(items.length - 1, i + 1)),
    k: () => setCursor((i) => Math.max(0, i - 1)),
    ArrowDown: () => setCursor((i) => Math.min(items.length - 1, i + 1)),
    ArrowUp: () => setCursor((i) => Math.max(0, i - 1)),
    ...(cursor >= 0 && items[cursor]
      ? { Enter: () => navigate(entryLink(changelog.id, items[cursor].id, changelog.default_locale)) }
      : {}),
  })
  useEffect(() => {
    if (cursor >= 0) list.current?.children[cursor]?.scrollIntoView({ block: "nearest" })
  }, [cursor])

  const states: [StateFilter, string][] = [
    ["all", t`Any state`],
    ["draft", stateText.draft],
    ["scheduled", stateText.scheduled],
    ["published", stateText.published],
    ["changed", translationText.changed],
  ]

  return (
    <div className="flex flex-col gap-4" data-testid="changelog-entries">
      <div className="flex flex-wrap items-center gap-2">
        <ChoiceSelect
          value={state}
          onChange={(v) => {
            setState(v)
            setCursor(-1)
          }}
          options={states}
          label={t`State`}
          className="w-48"
        />
        <ChoiceSelect
          value={label}
          onChange={(v) => {
            setLabel(v)
            setCursor(-1)
          }}
          options={[["", t`All labels`] as const, ...LABELS.map((l) => [l, labelText[l]] as const)]}
          label={t`Label`}
          className="w-40"
        />
        <span className="flex-1" />
        <Button size="sm" onClick={() => setCreating(true)} data-testid="new-entry">
          <PlusIcon />
          <Trans>New entry</Trans>
        </Button>
      </div>
      {entries.isPending ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-16 w-full rounded-xl" />
          <Skeleton className="h-16 w-full rounded-xl" />
          <Skeleton className="h-16 w-full rounded-xl" />
        </div>
      ) : entries.error ? (
        <ErrorLine error={entries.error} />
      ) : items.length === 0 ? (
        <div
          className="flex flex-col items-center gap-2 rounded-2xl border border-dashed px-6 py-12 text-center"
          data-testid="changelog-entries-empty"
        >
          <SparklesIcon className="size-6 text-faint" />
          <p className="text-body text-muted-foreground">
            {state !== "all" || label ? (
              <Trans>No entry matches.</Trans>
            ) : (
              <Trans>No entries yet. Tell your customers what changed in your latest release.</Trans>
            )}
          </p>
        </div>
      ) : (
        <ul ref={list} className="flex flex-col divide-y overflow-hidden rounded-2xl border bg-card">
          {items.map((e, i) => (
            <li
              key={e.id}
              className={cn("flex flex-wrap items-center gap-x-3 gap-y-1.5 px-4 py-3", i === cursor && "bg-muted")}
              data-testid="entry-row"
            >
              <div className="flex min-w-0 flex-1 basis-60 flex-col gap-1">
                <Link
                  to={entryLink(changelog.id, e.id, changelog.default_locale)}
                  className="block truncate text-body font-medium outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
                >
                  {entryTitle(e.translations, ...prefer)}
                </Link>
                <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <span
                    className={cn("inline-flex rounded-full px-2 py-px text-caption font-medium", stateTone(e.state))}
                    data-testid="entry-state"
                    data-state={e.state}
                  >
                    {stateText[e.state]}
                  </span>
                  {e.state === "scheduled" && e.publish_at && (
                    <span className="text-small text-muted-foreground">
                      {formatDateTime(e.publish_at, i18n.locale)}
                    </span>
                  )}
                  {e.state === "published" && e.published_at && (
                    <span className="text-small text-muted-foreground">{formatDate(e.published_at, i18n.locale)}</span>
                  )}
                  {e.labels.map((l) => (
                    <EntryLabel key={l} label={l} />
                  ))}
                </span>
                {e.schedule_failed && (
                  <span
                    className="flex items-center gap-1.5 text-small text-warning"
                    data-testid="entry-schedule-failed"
                  >
                    <TriangleAlertIcon className="size-3.5 shrink-0" />
                    <span>
                      <Trans>Publishing on schedule failed.</Trans> {failureText[e.schedule_failed.reason]}
                    </span>
                  </span>
                )}
              </div>
              <LocaleChips e={e} changelog={changelog} />
            </li>
          ))}
        </ul>
      )}
      {entries.hasNextPage && (
        <Button
          variant="outline"
          size="sm"
          className="self-center"
          disabled={entries.isFetchingNextPage}
          onClick={() => entries.fetchNextPage()}
        >
          <Trans>Load more</Trans>
        </Button>
      )}
      {creating && <NewEntryDialog changelog={changelog} onClose={() => setCreating(false)} />}
    </div>
  )
}

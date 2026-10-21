import { Trans, useLingui } from "@lingui/react/macro"
import { LoaderIcon, PlusIcon, SparklesIcon, TriangleAlertIcon } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { Link, useNavigate } from "react-router"

import { InboxMark } from "@/components/common"
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
import { entryLink, useCreateEntry, useEntries, useEntry } from "@/features/changelog/queries"
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
import { useInboxes } from "@/lib/workspace"

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

function plain(md: string) {
  return md
    .replace(/!\[[^\]]*\]\([^)]*\)/g, "")
    .replace(/\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/[#>*_`~-]+/g, " ")
    .replace(/\s+/g, " ")
    .trim()
}

function DateBlock({ iso }: { iso: string }) {
  const { i18n } = useLingui()
  const d = new Date(iso)
  return (
    <time
      dateTime={iso}
      className="flex w-11 shrink-0 flex-col items-center rounded-lg border bg-card py-1 leading-tight"
      title={formatDate(iso, i18n.locale)}
    >
      <span className="sr-only">{formatDate(iso, i18n.locale)}</span>
      <span className="text-[10px] font-medium text-faint uppercase" aria-hidden>
        {new Intl.DateTimeFormat(i18n.locale, { month: "short" }).format(d)}
      </span>
      <span className="text-body font-semibold tabular-nums" aria-hidden>
        {new Intl.DateTimeFormat(i18n.locale, { day: "numeric" }).format(d)}
      </span>
    </time>
  )
}

function Preview({ changelog, entry }: { changelog: Changelog; entry: ChangelogEntrySummary }) {
  const { i18n } = useLingui()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === changelog.inbox_id)
  const full = useEntry(changelog.id, entry.id).data
  const tr =
    full?.translations.find((x) => x.locale === i18n.locale) ??
    full?.translations.find((x) => x.locale === changelog.default_locale) ??
    full?.translations[0]
  const title =
    tr?.published?.title ?? tr?.title ?? entryTitle(entry.translations, i18n.locale, changelog.default_locale)
  const body = tr?.published?.excerpt || plain(tr?.body ?? "")
  const color = inbox?.branding.color ?? "var(--brand)"
  const host = changelog.url.replace(/^https?:\/\//, "").replace(/\/$/, "")
  return (
    <aside className="flex flex-col gap-3" aria-label={title} data-testid="entry-preview">
      <h3 className="text-caption font-medium tracking-wide text-faint uppercase">
        {inbox ? <Trans>How it looks in {inbox.name}</Trans> : <Trans>How it looks</Trans>}
      </h3>
      <div className="overflow-hidden rounded-2xl border bg-card shadow-[0_10px_30px_-12px_rgb(0_0_0/0.18)]">
        <div className="flex items-center gap-2 border-b px-4 py-3">
          {inbox && <InboxMark name={inbox.name} color={inbox.branding.color} className="size-5" />}
          <span className="flex-1 text-body font-semibold">
            <Trans>What’s new</Trans>
          </span>
        </div>
        {entry.image ? (
          <img src={entry.image.url} alt="" className="aspect-[2/1] w-full object-cover" />
        ) : (
          <div
            className="aspect-[2/1] w-full"
            style={{ background: `linear-gradient(135deg, ${color}, color-mix(in oklab, ${color} 35%, transparent))` }}
          />
        )}
        <div className="flex flex-col gap-2 p-4">
          {entry.labels.length > 0 && (
            <span className="flex flex-wrap gap-1">
              {entry.labels.map((l) => (
                <EntryLabel key={l} label={l} />
              ))}
            </span>
          )}
          <p className="text-title break-words">{title}</p>
          {body ? (
            <p className="line-clamp-6 text-body break-words text-muted-foreground">{body}</p>
          ) : (
            <p className="text-body text-faint">
              <Trans>No text yet.</Trans>
            </p>
          )}
        </div>
      </div>
      <p className="text-small text-faint">
        <Trans>Shown in the chat widget and on {host}.</Trans>
      </p>
    </aside>
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
  const [picked, setPicked] = useState("")
  const list = useRef<HTMLUListElement>(null)
  const filters: ChangelogEntryFilters = {
    state: state === "all" ? undefined : state,
    label: label || undefined,
  }
  const entries = useEntries(changelog.id, filters)
  const items = entries.data?.pages.flatMap((p) => p.items) ?? []
  const prefer = [i18n.locale, changelog.default_locale]
  const shown = items[cursor] ?? items.find((e) => e.id === picked) ?? items[0]

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
    <div className="@container flex flex-col gap-4" data-testid="changelog-entries">
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
        <div className="grid items-start gap-6 @min-[720px]:grid-cols-[minmax(0,1fr)_300px]">
          <ul ref={list} className="flex flex-col divide-y overflow-hidden rounded-2xl border bg-card">
            {items.map((e, i) => (
              <li
                key={e.id}
                className={cn(
                  "flex flex-wrap items-center gap-x-3.5 gap-y-1.5 px-4 py-3 transition-colors",
                  (i === cursor || (cursor < 0 && e.id === shown?.id)) &&
                    "bg-muted/60 shadow-[inset_2px_0_0_var(--brand)]"
                )}
                onClick={() => {
                  setPicked(e.id)
                  setCursor(-1)
                }}
                data-testid="entry-row"
              >
                <DateBlock iso={e.published_at ?? e.publish_at ?? e.created_at} />
                <div className="flex min-w-0 flex-1 basis-48 flex-col gap-1">
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
          {shown && (
            <div className="hidden @min-[720px]:block">
              <Preview changelog={changelog} entry={shown} />
            </div>
          )}
        </div>
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

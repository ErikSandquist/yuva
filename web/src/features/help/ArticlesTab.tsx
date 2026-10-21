import { Trans, useLingui } from "@lingui/react/macro"
import { FileTextIcon, LoaderIcon, PlusIcon, SearchIcon } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { Link, useNavigate } from "react-router"

import { HELP_KEYS, keyLabel } from "@/components/common/ShortcutSheet"
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
import {
  articleLink,
  useCreateArticle,
  useHelpArticles,
  useHelpCollections,
  useHelpReport,
} from "@/features/help/queries"
import { articleTitle, collectionName, ErrorLine, useLocaleName, useStateText } from "@/features/help/text"
import { ChoiceSelect, Field } from "@/features/settings/ui"
import { useDebounced } from "@/hooks/use-debounced"
import { useHotkeys } from "@/hooks/use-hotkeys"
import { formatDateTime, formatShort } from "@/components/common/text"
import { FilterChip } from "@/features/inbox/ListHead"
import type { HelpArticleSummary, HelpCenter, HelpCollection, HelpReport, HelpReportArticle } from "@/lib/api"
import type { HelpArticleFilters } from "@/lib/keys"
import { cn } from "@/lib/utils"

const grid =
  "grid grid-cols-[minmax(0,1fr)_5rem_3.5rem] gap-3 @xl:gap-4 @xl:grid-cols-[minmax(0,1fr)_6.5rem_4rem_4.5rem] @2xl:grid-cols-[minmax(0,1fr)_7rem_6.5rem_4rem_4.5rem]"

type StateFilter = NonNullable<HelpArticleFilters["state"]> | "all"

function chipTone(state: string | undefined) {
  if (state === "published") return "border-success/40 bg-success/10 text-success"
  if (state === "changed") return "border-warning/50 bg-warning/10 text-warning"
  if (state === "draft") return "border-border bg-muted text-muted-foreground"
  return "border-dashed border-border text-faint"
}

function LocaleChips({ a, center }: { a: HelpArticleSummary; center: HelpCenter }) {
  const { t } = useLingui()
  const name = useLocaleName()
  const stateText = useStateText()
  return (
    <span className="flex flex-wrap gap-1" data-testid="help-article-locales">
      {center.locales.map((l) => {
        const tr = a.translations.find((x) => x.locale === l)
        const label = tr
          ? `${name(l)}: ${stateText[tr.state]}${tr.outdated ? `, ${t`out of date`}` : ""}`
          : `${name(l)}: ${t`not written`}`
        return (
          <Link
            key={l}
            to={articleLink(center.id, a.id, l)}
            title={label}
            aria-label={label}
            className={cn(
              "rounded-md border px-1.5 py-0.5 text-caption leading-none font-medium uppercase outline-none focus-visible:ring-2 focus-visible:ring-ring",
              chipTone(tr?.state),
              tr?.outdated && "underline decoration-warning decoration-2 underline-offset-2"
            )}
            data-testid={`help-locale-${l}`}
          >
            {l}
          </Link>
        )
      })}
    </span>
  )
}

function NewArticleDialog({
  center,
  collections,
  collectionId,
  initialTitle = "",
  onClose,
}: {
  center: HelpCenter
  collections: HelpCollection[]
  collectionId: string
  initialTitle?: string
  onClose: () => void
}) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const localeName = useLocaleName()
  const create = useCreateArticle(center.id)
  const [title, setTitle] = useState(initialTitle)
  const [locale, setLocale] = useState(center.default_locale)
  const [collection, setCollection] = useState(collectionId)
  const ok = title.trim().length >= 3
  const prefer = [center.default_locale]
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent data-testid="new-article-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            if (!ok) return
            create.mutate(
              { locale, title: title.trim(), collection_id: collection || undefined },
              { onSuccess: (a) => navigate(articleLink(center.id, a.id, locale)) }
            )
          }}
        >
          <DialogHeader>
            <DialogTitle>
              <Trans>New article</Trans>
            </DialogTitle>
            <DialogDescription>
              <Trans>Start with a title. You write the text next, and it stays a draft until it is published.</Trans>
            </DialogDescription>
          </DialogHeader>
          <Field label={<Trans>Title</Trans>} htmlFor="new-article-title" hint={<Trans>At least 3 characters.</Trans>}>
            <Input
              id="new-article-title"
              value={title}
              maxLength={200}
              autoFocus
              required
              placeholder={t`How do I reset my password?`}
              onChange={(e) => setTitle(e.target.value)}
              data-testid="new-article-title"
            />
          </Field>
          <div className="grid gap-5 sm:grid-cols-2">
            <Field label={<Trans>Language</Trans>} htmlFor="new-article-locale">
              <ChoiceSelect
                id="new-article-locale"
                value={locale}
                onChange={setLocale}
                options={center.locales.map((l) => [l, localeName(l)] as const)}
                label={t`Language`}
              />
            </Field>
            <Field label={<Trans>Collection</Trans>} htmlFor="new-article-collection">
              <ChoiceSelect
                id="new-article-collection"
                value={collection}
                onChange={setCollection}
                options={[
                  ["", t`No collection`] as const,
                  ...collections.map((c) => [c.id, collectionName(c, ...prefer)] as const),
                ]}
                label={t`Collection`}
              />
            </Field>
          </div>
          <ErrorLine error={create.error} />
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              <Trans>Cancel</Trans>
            </Button>
            <Button type="submit" disabled={!ok || create.isPending} data-testid="create-article">
              {create.isPending && <LoaderIcon className="animate-spin" />}
              <Trans>Create article</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function Stat({
  label,
  value,
  detail,
  tone,
  testId,
}: {
  label: React.ReactNode
  value: string
  detail?: React.ReactNode
  tone?: string
  testId: string
}) {
  return (
    <div className="flex min-w-0 flex-col gap-0.5 rounded-xl border bg-card px-4 py-3" data-testid={testId}>
      <span className="truncate text-small text-muted-foreground">{label}</span>
      <span className={cn("text-title tabular-nums", tone)}>{value}</span>
      {detail && <span className="truncate text-caption text-faint">{detail}</span>}
    </div>
  )
}

function Stats({ center, report }: { center: HelpCenter; report?: HelpReport }) {
  const { i18n } = useLingui()
  const n = new Intl.NumberFormat(i18n.locale)
  const published = n.format(center.counts.published_articles)
  const missed = report?.no_result_searches ?? 0
  return (
    <div className="grid grid-cols-2 gap-3 @2xl:grid-cols-4" data-testid="help-stats">
      <Stat
        label={<Trans>Articles</Trans>}
        value={n.format(center.counts.articles)}
        detail={<Trans>{published} published</Trans>}
        testId="help-stat-articles"
      />
      <Stat
        label={<Trans>Views · 7 days</Trans>}
        value={report ? n.format(report.views) : "–"}
        testId="help-stat-views"
      />
      <Stat
        label={<Trans>Searches · 7 days</Trans>}
        value={report ? n.format(report.searches) : "–"}
        testId="help-stat-searches"
      />
      <Stat
        label={<Trans>Found nothing · 7 days</Trans>}
        value={report ? n.format(missed) : "–"}
        tone={missed > 0 ? "text-warning" : undefined}
        testId="help-stat-missed"
      />
    </div>
  )
}

function Misses({ report, onWrite }: { report?: HelpReport; onWrite: (query: string) => void }) {
  const { i18n } = useLingui()
  const n = new Intl.NumberFormat(i18n.locale)
  const items = (report?.no_result_queries ?? []).filter((q) => q.query !== "(scrubbed)").slice(0, 8)
  if (items.length === 0) return null
  return (
    <section className="flex flex-col gap-2.5 rounded-xl border bg-card px-4 py-3.5" data-testid="help-misses">
      <h2 className="flex items-baseline justify-between gap-2 text-body font-semibold">
        <Trans>Searched for, found nothing</Trans>
        <span className="text-caption font-normal text-faint">
          <Trans>last 7 days</Trans>
        </span>
      </h2>
      <ul className="flex flex-wrap gap-2">
        {items.map((q) => (
          <li
            key={`${q.locale}:${q.query}`}
            className="inline-flex items-center gap-2 rounded-full border py-0.5 ps-3 pe-0.5 text-small"
          >
            <span dir="auto" className="max-w-48 truncate">
              {q.query}
            </span>
            <span className="text-faint tabular-nums">{n.format(q.searches)}×</span>
            <Button
              size="sm"
              className="h-6 rounded-full px-2.5"
              onClick={() => onWrite(q.query)}
              data-testid="help-miss-write"
            >
              <Trans>Write</Trans>
            </Button>
          </li>
        ))}
      </ul>
    </section>
  )
}

export function ArticlesTab({ center }: { center: HelpCenter }) {
  const { t, i18n } = useLingui()
  const navigate = useNavigate()
  const localeName = useLocaleName()
  const [q, setQ] = useState("")
  const [state, setState] = useState<StateFilter>("all")
  const [locale, setLocale] = useState(center.default_locale)
  const [collection, setCollection] = useState("")
  const [creating, setCreating] = useState<string | null>(null)
  const [cursor, setCursor] = useState(-1)
  const report = useHelpReport(center.id, 7).data
  const search = useRef<HTMLInputElement>(null)
  const list = useRef<HTMLUListElement>(null)
  const debounced = useDebounced(q.trim(), 300)
  const collections = useHelpCollections(center.id)
  const cols = collections.data ?? []
  const filters: HelpArticleFilters = {
    q: debounced || undefined,
    collection_id: collection || undefined,
    locale: state === "all" ? undefined : locale,
    state: state === "all" ? undefined : state,
  }
  const articles = useHelpArticles(center.id, filters)
  const items = articles.data?.pages.flatMap((p) => p.items) ?? []
  const prefer = [i18n.locale, center.default_locale]
  const stateText = useStateText()
  const newKey = keyLabel(HELP_KEYS.newArticle).join("")
  const findKey = keyLabel(HELP_KEYS.search).join("")

  useHotkeys({
    [HELP_KEYS.newArticle]: () => setCreating(""),
    [HELP_KEYS.search]: () => search.current?.focus(),
    j: () => setCursor((i) => Math.min(items.length - 1, i + 1)),
    k: () => setCursor((i) => Math.max(0, i - 1)),
    ArrowDown: () => setCursor((i) => Math.min(items.length - 1, i + 1)),
    ArrowUp: () => setCursor((i) => Math.max(0, i - 1)),
    ...(cursor >= 0 && items[cursor]
      ? { Enter: () => navigate(articleLink(center.id, items[cursor].id, center.default_locale)) }
      : {}),
  })
  useEffect(() => {
    if (cursor >= 0) list.current?.children[cursor]?.scrollIntoView({ block: "nearest" })
  }, [cursor])

  const fmt = new Intl.NumberFormat(i18n.locale)
  const percent = new Intl.NumberFormat(i18n.locale, { style: "percent", maximumFractionDigits: 0 })
  const stats = new Map<string, HelpReportArticle>()
  for (const r of [...(report?.most_down ?? []), ...(report?.top_articles ?? [])]) stats.set(r.article_id, r)
  const complete = !!report && report.top_articles.length < 20
  const maxViews = Math.max(0, ...(report?.top_articles ?? []).map((r) => r.views))
  const collectionOf = (a: HelpArticleSummary) => {
    const c = cols.find((x) => x.id === a.collection_id)
    return c ? collectionName(c, ...prefer) : undefined
  }
  const states: [StateFilter, string][] = [
    ["all", t`Any state`],
    ["draft", stateText.draft],
    ["published", stateText.published],
    ["changed", stateText.changed],
    ["missing", t`Not written`],
  ]

  return (
    <div className="@container flex flex-col gap-4" data-testid="help-articles">
      <Stats center={center} report={report} />
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-48 flex-1">
          <SearchIcon className="pointer-events-none absolute start-2.5 top-1/2 size-4 -translate-y-1/2 text-faint" />
          <Input
            ref={search}
            value={q}
            onChange={(e) => {
              setQ(e.target.value)
              setCursor(-1)
            }}
            onKeyDown={(e) => e.key === "Escape" && e.currentTarget.blur()}
            placeholder={t`Search titles (${findKey})`}
            aria-label={t`Search titles`}
            className="ps-8"
            data-testid="help-article-search"
          />
        </div>
        <ChoiceSelect
          value={state}
          onChange={(v) => {
            setState(v)
            setCursor(-1)
          }}
          options={states}
          label={t`State`}
          className="w-40"
        />
        {state !== "all" && (
          <ChoiceSelect
            value={locale}
            onChange={setLocale}
            options={center.locales.map((l) => [l, localeName(l)] as const)}
            label={t`Language`}
            className="w-36"
          />
        )}
        <Button size="sm" onClick={() => setCreating("")} title={t`New article (${newKey})`} data-testid="new-article">
          <PlusIcon />
          <Trans>New article</Trans>
        </Button>
      </div>
      {cols.length > 0 && (
        <div
          className="flex flex-wrap gap-1.5"
          role="toolbar"
          aria-label={t`Collection`}
          data-testid="help-collections"
        >
          <FilterChip
            active={!collection}
            onClick={() => {
              setCollection("")
              setCursor(-1)
            }}
            testId="help-collection-all"
          >
            <Trans>All collections</Trans>
          </FilterChip>
          {cols.map((c) => (
            <FilterChip
              key={c.id}
              active={collection === c.id}
              onClick={() => {
                setCollection(c.id)
                setCursor(-1)
              }}
              testId="help-collection"
            >
              {collectionName(c, ...prefer)}
              <span className="tabular-nums opacity-60">{fmt.format(c.article_count)}</span>
            </FilterChip>
          ))}
        </div>
      )}
      {articles.isPending ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-14 w-full rounded-xl" />
          <Skeleton className="h-14 w-full rounded-xl" />
          <Skeleton className="h-14 w-full rounded-xl" />
        </div>
      ) : articles.error ? (
        <ErrorLine error={articles.error} />
      ) : items.length === 0 ? (
        <div
          className="flex flex-col items-center gap-2 rounded-2xl border border-dashed px-6 py-12 text-center"
          data-testid="help-articles-empty"
        >
          <FileTextIcon className="size-6 text-faint" />
          <p className="text-body text-muted-foreground">
            {debounced || state !== "all" || collection ? (
              <Trans>No article matches.</Trans>
            ) : (
              <Trans>No articles yet. Write the first answer your customers keep asking for.</Trans>
            )}
          </p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-xl border bg-card">
          <div className={cn(grid, "border-b bg-muted/50 px-4 py-2 text-caption font-medium text-faint")} aria-hidden>
            <span>
              <Trans>Article</Trans>
            </span>
            <span className="hidden @2xl:block">
              <Trans>Collection</Trans>
            </span>
            <span className="text-end">
              <Trans>Views · 7 days</Trans>
            </span>
            <span className="text-end">
              <Trans>Helpful</Trans>
            </span>
            <span className="hidden text-end @xl:block">
              <Trans>Updated</Trans>
            </span>
          </div>
          <ul ref={list} className="flex flex-col divide-y">
            {items.map((a, i) => {
              const r = stats.get(a.id)
              const votes = r ? r.helpful_up + r.helpful_down : 0
              const views = r ? r.views : complete ? 0 : undefined
              return (
                <li
                  key={a.id}
                  className={cn(grid, "items-center px-4 py-2.5", i === cursor && "bg-muted")}
                  data-testid="help-article-row"
                >
                  <div className="flex min-w-0 flex-col gap-1">
                    <Link
                      to={articleLink(center.id, a.id, center.default_locale)}
                      className="block truncate text-body font-medium outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
                    >
                      {articleTitle(a, ...prefer)}
                    </Link>
                    <LocaleChips a={a} center={center} />
                  </div>
                  <span className="hidden truncate text-small text-muted-foreground @2xl:block">
                    {collectionOf(a) ?? <span className="text-faint">–</span>}
                  </span>
                  <span className="flex items-center justify-end gap-2 text-small tabular-nums">
                    {views !== undefined && maxViews > 0 && (
                      <span className="h-1.5 w-12 overflow-hidden rounded-full bg-muted phone:hidden" aria-hidden>
                        <span
                          className="block h-full rounded-full bg-brand/70"
                          style={{ width: `${(views / maxViews) * 100}%` }}
                        />
                      </span>
                    )}
                    {views !== undefined ? fmt.format(views) : <span className="text-faint">–</span>}
                  </span>
                  <span className="text-end text-small tabular-nums">
                    {votes > 0 && r ? (
                      <span
                        className={cn(r.helpful_up / votes < 0.7 && "font-medium text-warning")}
                        title={`${fmt.format(r.helpful_up)} / ${fmt.format(votes)}`}
                      >
                        {percent.format(r.helpful_up / votes)}
                      </span>
                    ) : (
                      <span className="text-faint">–</span>
                    )}
                  </span>
                  <time
                    className="hidden text-end text-small text-faint @xl:block"
                    dateTime={a.updated_at}
                    title={formatDateTime(a.updated_at, i18n.locale)}
                  >
                    {formatShort(a.updated_at, i18n.locale)}
                  </time>
                </li>
              )
            })}
          </ul>
        </div>
      )}
      <Misses report={report} onWrite={setCreating} />
      {articles.hasNextPage && (
        <Button
          variant="outline"
          size="sm"
          className="self-center"
          disabled={articles.isFetchingNextPage}
          onClick={() => articles.fetchNextPage()}
        >
          <Trans>Load more</Trans>
        </Button>
      )}
      {creating !== null && (
        <NewArticleDialog
          center={center}
          collections={cols}
          collectionId={collection}
          initialTitle={creating}
          onClose={() => setCreating(null)}
        />
      )}
    </div>
  )
}

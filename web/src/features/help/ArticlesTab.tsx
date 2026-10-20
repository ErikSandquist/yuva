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
import { articleLink, useCreateArticle, useHelpArticles, useHelpCollections } from "@/features/help/queries"
import { articleTitle, collectionName, ErrorLine, useLocaleName, useStateText } from "@/features/help/text"
import { ChoiceSelect, Field } from "@/features/settings/ui"
import { useDebounced } from "@/hooks/use-debounced"
import { useHotkeys } from "@/hooks/use-hotkeys"
import type { HelpArticleSummary, HelpCenter, HelpCollection } from "@/lib/api"
import type { HelpArticleFilters } from "@/lib/keys"
import { cn } from "@/lib/utils"

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
  onClose,
}: {
  center: HelpCenter
  collections: HelpCollection[]
  collectionId: string
  onClose: () => void
}) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const localeName = useLocaleName()
  const create = useCreateArticle(center.id)
  const [title, setTitle] = useState("")
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

export function ArticlesTab({ center }: { center: HelpCenter }) {
  const { t, i18n } = useLingui()
  const navigate = useNavigate()
  const localeName = useLocaleName()
  const [q, setQ] = useState("")
  const [state, setState] = useState<StateFilter>("all")
  const [locale, setLocale] = useState(center.default_locale)
  const [collection, setCollection] = useState("")
  const [creating, setCreating] = useState(false)
  const [cursor, setCursor] = useState(-1)
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
    [HELP_KEYS.newArticle]: () => setCreating(true),
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
    <div className="flex flex-col gap-4" data-testid="help-articles">
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
          value={collection}
          onChange={(v) => {
            setCollection(v)
            setCursor(-1)
          }}
          options={[
            ["", t`All collections`] as const,
            ...cols.map((c) => [c.id, collectionName(c, ...prefer)] as const),
          ]}
          label={t`Collection`}
          className="w-44"
        />
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
        <Button
          size="sm"
          onClick={() => setCreating(true)}
          title={t`New article (${newKey})`}
          data-testid="new-article"
        >
          <PlusIcon />
          <Trans>New article</Trans>
        </Button>
      </div>
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
        <ul ref={list} className="flex flex-col divide-y overflow-hidden rounded-2xl border bg-card">
          {items.map((a, i) => (
            <li
              key={a.id}
              className={cn("flex flex-wrap items-center gap-x-3 gap-y-1.5 px-4 py-3", i === cursor && "bg-muted")}
              data-testid="help-article-row"
            >
              <div className="min-w-0 flex-1 basis-60">
                <Link
                  to={articleLink(center.id, a.id, center.default_locale)}
                  className="block truncate text-body font-medium outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
                >
                  {articleTitle(a, ...prefer)}
                </Link>
                <span className="block truncate text-small text-faint">
                  {collectionOf(a) ?? <Trans>No collection</Trans>}
                </span>
              </div>
              <LocaleChips a={a} center={center} />
            </li>
          ))}
        </ul>
      )}
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
      {creating && (
        <NewArticleDialog
          center={center}
          collections={cols}
          collectionId={collection}
          onClose={() => setCreating(false)}
        />
      )}
    </div>
  )
}

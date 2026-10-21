import "@/features/designs/i18n"

import { Trans, useLingui } from "@lingui/react/macro"
import {
  ArchiveIcon,
  FilePlusIcon,
  LanguagesIcon,
  LayoutTemplateIcon,
  LoaderIcon,
  PlusIcon,
  TriangleAlertIcon,
} from "lucide-react"
import { useState } from "react"
import { Link, useLocation, useNavigate, useSearchParams } from "react-router"

import { Dot, ErrorLine, Notice, Segmented } from "@/components/common"
import { Pane, PaneBack, PaneEmpty } from "@/components/common/Column"
import { MailFrame } from "@/components/common/MailFrame"
import { formatRelative } from "@/components/common/text"
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
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import {
  designLink,
  useCreateDesign,
  useDesign,
  useDesigns,
  useRender,
  useShowArchivedDesigns,
  useTemplates,
} from "@/features/designs/queries"
import { useTemplateLocales } from "@/features/designs/text"
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { PageHeader, PageTabs } from "@/features/settings/ui"
import type { EmailDesignSummary, EmailTemplate, EmailTemplateLocale } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

const WORKSPACE = "workspace"
const LOCALES: EmailTemplateLocale[] = ["en", "tr", "de"]

type Choice = { template: EmailTemplate | null }

function CreateDialog({
  choice,
  locale,
  onClose,
}: {
  choice: Choice
  locale: EmailTemplateLocale
  onClose: () => void
}) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const inboxes = useInboxes().data ?? []
  const create = useCreateDesign()
  const tpl = choice.template
  const title = tpl?.name ?? ""
  const [name, setName] = useState(tpl ? tpl.name : t`Untitled design`)
  const [inbox, setInbox] = useState(inboxes.length === 1 ? inboxes[0].id : WORKSPACE)
  const items: Record<string, string> = {
    [WORKSPACE]: t`Whole workspace`,
    ...Object.fromEntries(inboxes.map((i) => [i.id, i.name])),
  }
  const submit = () => {
    if (!name.trim()) return
    create.mutate(
      {
        name: name.trim(),
        inbox_id: inbox === WORKSPACE ? undefined : inbox,
        template: tpl ? { slug: tpl.slug, locale } : undefined,
      },
      { onSuccess: (d) => navigate(designLink(d.id)) }
    )
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent data-testid="create-design-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            submit()
          }}
        >
          <DialogHeader>
            <DialogTitle>{tpl ? <Trans>Use “{title}”</Trans> : <Trans>New blank design</Trans>}</DialogTitle>
            <DialogDescription>
              {tpl ? (
                <Trans>Its images are copied into your workspace’s images. You can change everything after.</Trans>
              ) : (
                <Trans>Start from an empty mail with simple colours.</Trans>
              )}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2">
            <Label htmlFor="design-name-new">
              <Trans>Name</Trans>
            </Label>
            <Input
              id="design-name-new"
              value={name}
              maxLength={200}
              autoFocus
              onChange={(e) => setName(e.target.value)}
              data-testid="new-design-name"
            />
          </div>
          {inboxes.length > 0 && (
            <div className="flex flex-col gap-2">
              <Label>
                <Trans>Inbox</Trans>
              </Label>
              <Select value={inbox} items={items} onValueChange={(v) => setInbox(String(v ?? WORKSPACE))}>
                <SelectTrigger className="w-full min-w-0" aria-label={t`Inbox`} data-testid="new-design-inbox">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={WORKSPACE}>{t`Whole workspace`}</SelectItem>
                  {inboxes.map((i) => (
                    <SelectItem key={i.id} value={i.id}>
                      <Dot color={i.branding.color} className="mt-1.5 size-2 rounded-[3px]" />
                      {i.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-small text-faint">
                <Trans>The inbox’s sender details fill the footer every mail gets.</Trans>
              </p>
            </div>
          )}
          <ErrorLine error={create.error} />
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              <Trans>Cancel</Trans>
            </Button>
            <Button type="submit" disabled={!name.trim() || create.isPending} data-testid="create-design">
              {create.isPending && <LoaderIcon className="animate-spin" />}
              <Trans>Create and edit</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function useCategory() {
  const { t } = useLingui()
  const names: Record<string, string> = {
    newsletter: t`Newsletter`,
    product: t`Product`,
    announcement: t`Announcement`,
    onboarding: t`Onboarding`,
    event: t`Event`,
    digest: t`Digest`,
  }
  return (c: string) => names[c] ?? c
}

function TemplateCard({ tpl, locale, onUse }: { tpl: EmailTemplate; locale: EmailTemplateLocale; onUse: () => void }) {
  const { t } = useLingui()
  const category = useCategory()
  const own = tpl.locales.includes(locale)
  const title = tpl.name
  return (
    <li>
      <button
        type="button"
        onClick={onUse}
        className="group flex w-full flex-col overflow-hidden rounded-2xl border bg-card text-start outline-none transition-[border-color,box-shadow] hover:border-faint hover:shadow-[0_12px_32px_-18px_rgb(0_0_0/0.35)] focus-visible:ring-2 focus-visible:ring-ring"
        data-testid="template-card"
        data-slug={tpl.slug}
      >
        <span className="relative block aspect-[4/5] overflow-hidden border-b bg-surface">
          <img
            src={tpl.preview_url}
            alt={t`Preview of ${title}`}
            loading="lazy"
            className="absolute inset-x-0 top-0 w-full transition-transform duration-500 group-hover:-translate-y-[6%]"
          />
          <span className="absolute inset-x-0 bottom-0 h-16 bg-gradient-to-t from-black/10 to-transparent" />
        </span>
        <span className="flex flex-col gap-1 px-4 py-3.5">
          <span className="flex items-center gap-2">
            <span className="min-w-0 flex-1 truncate text-body font-semibold">{tpl.name}</span>
            <span className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-caption text-muted-foreground">
              {category(tpl.category)}
            </span>
          </span>
          <span className="line-clamp-2 text-small text-muted-foreground">{tpl.description}</span>
          {!own && (
            <span className="mt-1 inline-flex items-center gap-1 text-caption text-faint">
              <LanguagesIcon className="size-3" />
              <Trans>Text in English</Trans>
            </span>
          )}
        </span>
      </button>
    </li>
  )
}

const THUMB_WIDTH = 600
const THUMB_SCALE = 0.3

function DesignCard({ d }: { d: EmailDesignSummary }) {
  const { t, i18n } = useLingui()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === d.inbox_id)
  const design = useDesign(d.id)
  const render = useRender(
    design.data ? { document: design.data.document, ...(d.inbox_id ? { inbox_id: d.inbox_id } : {}) } : null
  )
  const name = d.name
  return (
    <li>
      <Link
        to={designLink(d.id)}
        className="group flex flex-col gap-2.5 rounded-2xl outline-none focus-visible:ring-2 focus-visible:ring-ring"
        data-testid="design-card"
      >
        <span className="relative flex h-64 justify-center overflow-hidden rounded-2xl border bg-surface pt-4.5 transition-colors group-hover:border-faint">
          <span className="relative h-full w-[180px] overflow-hidden rounded-t-md bg-white shadow-[0_4px_14px_rgb(0_0_0/0.08)]">
            {render.data ? (
              <span
                className="pointer-events-none absolute top-0 start-0 origin-top-left rtl:origin-top-right"
                style={{ width: THUMB_WIDTH, transform: `scale(${THUMB_SCALE})` }}
                aria-hidden
              >
                <MailFrame html={render.data.html} width={THUMB_WIDTH} title={t`Preview of ${name}`} />
              </span>
            ) : render.isError || design.isError ? (
              <span className="grid h-full place-items-center text-faint">
                <LayoutTemplateIcon className="size-6" />
              </span>
            ) : (
              <Skeleton className="h-full w-full rounded-none" />
            )}
          </span>
          {d.archived_at && (
            <span className="absolute top-2.5 end-2.5 inline-flex items-center gap-1 rounded-md bg-background/90 px-1.5 py-0.5 text-caption text-muted-foreground">
              <ArchiveIcon className="size-3" />
              <Trans>Archived</Trans>
            </span>
          )}
        </span>
        <span className="flex min-w-0 flex-col px-1 leading-snug">
          <span className="truncate text-body font-medium">{d.name}</span>
          <span className="flex min-w-0 items-center gap-1.5 text-caption text-faint">
            {inbox && (
              <>
                <Dot color={inbox.branding.color} className="size-[6px] shrink-0 rounded-[2px]" />
                <span className="truncate">{inbox.name}</span>
                <span aria-hidden>·</span>
              </>
            )}
            <span className="shrink-0">{formatRelative(d.updated_at, i18n.locale)}</span>
          </span>
        </span>
      </Link>
    </li>
  )
}

function MyDesigns() {
  const [inboxId] = useInboxFilter()
  const [archived] = useShowArchivedDesigns()
  const designs = useDesigns(inboxId, "", archived)
  const items = designs.data?.pages.flatMap((p) => p.items) ?? []
  if (designs.isPending)
    return (
      <div className="grid grid-cols-[repeat(auto-fill,minmax(13rem,1fr))] gap-4">
        {[0, 1, 2].map((i) => (
          <Skeleton key={i} className="h-72 rounded-2xl" />
        ))}
      </div>
    )
  return (
    <>
      <ErrorLine error={designs.error} />
      {items.length === 0 ? (
        <div
          className="flex flex-col items-center gap-3 rounded-2xl border border-dashed px-6 py-12 text-center"
          data-testid="designs-home-empty"
        >
          <LayoutTemplateIcon className="size-6 text-faint" />
          <p className="max-w-md text-body text-muted-foreground">
            <Trans>A design is the look of a mail: start from a template, then use it for your broadcasts.</Trans>
          </p>
          <Button size="sm" variant="outline" render={<Link to="/broadcasts/designs/new" />}>
            <Trans>Browse the gallery</Trans>
          </Button>
        </div>
      ) : (
        <ul className="grid grid-cols-[repeat(auto-fill,minmax(13rem,1fr))] gap-x-4 gap-y-6" data-testid="design-grid">
          {items.map((d) => (
            <DesignCard key={d.id} d={d} />
          ))}
        </ul>
      )}
      {designs.hasNextPage && (
        <Button
          variant="ghost"
          size="sm"
          className="self-center"
          onClick={() => void designs.fetchNextPage()}
          disabled={designs.isFetchingNextPage}
        >
          <Trans>Load more</Trans>
        </Button>
      )}
    </>
  )
}

export default function GalleryPage() {
  const { t, i18n } = useLingui()
  const { canManage } = useSession()
  const [params, setParams] = useSearchParams()
  const localeNames = useTemplateLocales()
  const wanted = params.get("locale") as EmailTemplateLocale | null
  const fallback: EmailTemplateLocale = LOCALES.includes(i18n.locale as EmailTemplateLocale)
    ? (i18n.locale as EmailTemplateLocale)
    : "en"
  const [locale, setLocale] = useState<EmailTemplateLocale>(wanted && LOCALES.includes(wanted) ? wanted : fallback)
  const templates = useTemplates(locale)
  const [choice, setChoice] = useState<Choice | null>(null)
  const slug = params.get("template")
  const asked = slug ? templates.data?.find((x) => x.slug === slug) : undefined
  const unknown = !!slug && !!templates.data && !asked
  const gallery = useLocation().pathname.endsWith("/new")
  if (!canManage)
    return (
      <PaneEmpty icon={LayoutTemplateIcon} title={<Trans>E-mail designs</Trans>} testId="designs-read-only">
        <Trans>Owners and admins make e-mail designs and send broadcasts.</Trans>
      </PaneEmpty>
    )
  const open = choice ?? (asked ? { template: asked } : null)
  const close = () => {
    setChoice(null)
    if (slug) setParams({}, { replace: true })
  }
  const tabs = [
    { key: "mine", to: "/broadcasts/designs", label: t`Mine`, testId: "designs-tab-mine" },
    { key: "gallery", to: "/broadcasts/designs/new", label: t`Gallery`, testId: "designs-tab-gallery" },
  ]
  return (
    <Pane wide testId="design-gallery" className="gap-6">
      <PaneBack to="/broadcasts/designs" label={t`Designs`} />
      <PageHeader
        back={false}
        title={<Trans>Designs</Trans>}
        description={
          gallery ? (
            <Trans>
              Start from a template or an empty page. Every design gets the footer the law asks for, and you can change
              everything else.
            </Trans>
          ) : (
            <Trans>E-mail layouts you reuse across broadcasts.</Trans>
          )
        }
        action={
          gallery ? (
            <Segmented
              value={locale}
              onChange={setLocale}
              label={t`Template language`}
              items={localeNames.map((l) => ({ ...l, testId: `template-locale-${l.value}` }))}
            />
          ) : (
            <Button size="sm" render={<Link to="/broadcasts/designs/new" />} data-testid="designs-home-new">
              <PlusIcon />
              <Trans>New design</Trans>
            </Button>
          )
        }
        tabs={<PageTabs label={t`Designs`} items={tabs} current={gallery ? "gallery" : "mine"} />}
      />
      {unknown && (
        <Notice tone="warning" icon={TriangleAlertIcon} data-testid="template-unknown">
          <p>
            <Trans>This server does not have the template “{slug}”. It may need a newer Yuva.</Trans>
          </p>
        </Notice>
      )}
      {!gallery && <MyDesigns />}
      {gallery && (
        <ul className="grid grid-cols-[repeat(auto-fill,minmax(14rem,1fr))] gap-4" data-testid="template-grid">
          <li>
            <button
              type="button"
              onClick={() => setChoice({ template: null })}
              className="flex aspect-[4/5] w-full flex-col items-center justify-center gap-3 rounded-2xl border-2 border-dashed text-center text-muted-foreground outline-none transition-colors hover:border-brand hover:bg-brand-wash/50 hover:text-brand focus-visible:ring-2 focus-visible:ring-ring"
              data-testid="template-blank"
            >
              <FilePlusIcon className="size-7" />
              <span className="text-body font-semibold">
                <Trans>Empty page</Trans>
              </span>
              <span className="max-w-44 text-small">
                <Trans>Build it yourself from blocks</Trans>
              </span>
            </button>
          </li>
          {templates.isPending
            ? Array.from({ length: 6 }, (_, i) => (
                <li key={i}>
                  <Skeleton className={cn("aspect-[4/5] w-full rounded-2xl")} />
                </li>
              ))
            : (templates.data ?? []).map((tpl) => (
                <TemplateCard key={tpl.slug} tpl={tpl} locale={locale} onUse={() => setChoice({ template: tpl })} />
              ))}
        </ul>
      )}
      {gallery && <ErrorLine error={templates.error} />}
      {open && <CreateDialog key={open.template?.slug ?? "blank"} choice={open} locale={locale} onClose={close} />}
    </Pane>
  )
}

import "@/features/designs/i18n"

import { Trans, useLingui } from "@lingui/react/macro"
import {
  ArchiveIcon,
  FilePlusIcon,
  LanguagesIcon,
  LayoutTemplateIcon,
  LoaderIcon,
  TriangleAlertIcon,
} from "lucide-react"
import { useState } from "react"
import { Link, useLocation, useNavigate, useSearchParams } from "react-router"

import { Dot, ErrorLine, Notice, Segmented } from "@/components/common"
import { PaneEmpty } from "@/components/common/Column"
import { MailFrame } from "@/components/common/MailFrame"
import { formatShort } from "@/components/common/text"
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
import { FrameButton, FrameTabs, framePad, PageFrame } from "@/features/status/PageFrame"
import type { EmailDesignSummary, EmailTemplate, EmailTemplateLocale } from "@/lib/api"
import { useSession } from "@/lib/session"
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

const sheet = "relative h-[260px] w-[184px] overflow-hidden rounded-t-md bg-white shadow-[0_4px_14px_rgb(0_0_0/0.08)]"
const box =
  "relative flex h-[297px] justify-center overflow-hidden rounded-xl border bg-surface pt-[18px] transition-colors group-hover:border-faint"

function TemplateCard({ tpl, locale, onUse }: { tpl: EmailTemplate; locale: EmailTemplateLocale; onUse: () => void }) {
  const { t } = useLingui()
  const category = useCategory()
  const own = tpl.locales.includes(locale)
  const title = tpl.name
  const kind = category(tpl.category)
  return (
    <li>
      <button
        type="button"
        onClick={onUse}
        title={tpl.description}
        className="group flex w-full flex-col gap-2.5 rounded-xl text-start outline-none focus-visible:ring-2 focus-visible:ring-ring"
        data-testid="template-card"
        data-slug={tpl.slug}
      >
        <span className={box}>
          <span className={sheet}>
            <img
              src={tpl.preview_url}
              alt={t`Preview of ${title}`}
              loading="lazy"
              className="absolute inset-x-0 top-0 w-full transition-transform duration-500 group-hover:-translate-y-[6%]"
            />
          </span>
        </span>
        <span className="flex min-w-0 flex-col leading-snug">
          <span className="truncate text-body font-semibold">{tpl.name}</span>
          <span className="flex min-w-0 items-center gap-1.5 text-caption text-faint">
            <span className="truncate">
              <Trans>Gallery · {kind}</Trans>
            </span>
            {!own && (
              <span className="inline-flex shrink-0 items-center gap-1">
                <LanguagesIcon className="size-3" />
                <Trans>Text in English</Trans>
              </span>
            )}
          </span>
        </span>
      </button>
    </li>
  )
}

const THUMB_WIDTH = 600
const THUMB_SCALE = 184 / 600

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
        className="group flex flex-col gap-2.5 rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring"
        data-testid="design-card"
      >
        <span className={box}>
          <span className={sheet}>
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
        <span className="flex min-w-0 flex-col leading-snug">
          <span className="truncate text-body font-semibold">{d.name}</span>
          <span className="flex min-w-0 items-center gap-1.5 text-caption text-faint">
            {inbox && (
              <>
                <span className="truncate">{inbox.name}</span>
                <span aria-hidden>·</span>
              </>
            )}
            <span className="shrink-0">{formatShort(d.updated_at, i18n.locale)}</span>
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
      <div className="grid grid-cols-[repeat(auto-fill,minmax(15rem,1fr))] gap-4">
        {[0, 1, 2].map((i) => (
          <Skeleton key={i} className="h-[297px] rounded-xl" />
        ))}
      </div>
    )
  return (
    <>
      <ErrorLine error={designs.error} />
      {items.length === 0 ? (
        <div
          className="flex flex-col items-center gap-3 rounded-xl border border-dashed px-6 py-12 text-center"
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
        <ul className="grid grid-cols-[repeat(auto-fill,minmax(15rem,1fr))] gap-x-4 gap-y-5" data-testid="design-grid">
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
    <PageFrame
      testId="design-gallery"
      back={gallery ? { to: "/broadcasts/designs", label: t`Designs` } : undefined}
      title={<Trans>Designs</Trans>}
      description={<Trans>E-mail layouts you reuse across broadcasts</Trans>}
      actions={
        gallery ? (
          <>
            <Segmented
              value={locale}
              onChange={setLocale}
              label={t`Template language`}
              items={localeNames.map((l) => ({ ...l, testId: `template-locale-${l.value}` }))}
            />
            <FrameButton ink onClick={() => setChoice({ template: null })} data-testid="designs-home-new">
              <Trans>New design</Trans>
            </FrameButton>
          </>
        ) : (
          <FrameButton ink render={<Link to="/broadcasts/designs/new" />} data-testid="designs-home-new">
            <Trans>New design</Trans>
          </FrameButton>
        )
      }
      tabs={<FrameTabs label={t`Designs`} items={tabs} current={gallery ? "gallery" : "mine"} />}
    >
      <div className={framePad}>
        {unknown && (
          <Notice tone="warning" icon={TriangleAlertIcon} data-testid="template-unknown">
            <p>
              <Trans>This server does not have the template “{slug}”. It may need a newer Yuva.</Trans>
            </p>
          </Notice>
        )}
        {!gallery && <MyDesigns />}
        {gallery && (
          <ul
            className="grid grid-cols-[repeat(auto-fill,minmax(15rem,1fr))] gap-x-4 gap-y-5"
            data-testid="template-grid"
          >
            {templates.isPending
              ? Array.from({ length: 6 }, (_, i) => (
                  <li key={i}>
                    <Skeleton className="h-[297px] w-full rounded-xl" />
                  </li>
                ))
              : (templates.data ?? []).map((tpl) => (
                  <TemplateCard key={tpl.slug} tpl={tpl} locale={locale} onUse={() => setChoice({ template: tpl })} />
                ))}
            <li>
              <button
                type="button"
                onClick={() => setChoice({ template: null })}
                className="group flex w-full flex-col gap-2.5 rounded-xl text-start outline-none focus-visible:ring-2 focus-visible:ring-ring"
                data-testid="template-blank"
              >
                <span className="flex h-[297px] flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed text-muted-foreground transition-colors group-hover:border-brand group-hover:text-brand">
                  <FilePlusIcon className="size-7" />
                </span>
                <span className="flex min-w-0 flex-col leading-snug">
                  <span className="truncate text-body font-semibold">
                    <Trans>Empty page</Trans>
                  </span>
                  <span className="truncate text-caption text-faint">
                    <Trans>Build it yourself from blocks</Trans>
                  </span>
                </span>
              </button>
            </li>
          </ul>
        )}
        {gallery && <ErrorLine error={templates.error} />}
      </div>
      {open && <CreateDialog key={open.template?.slug ?? "blank"} choice={open} locale={locale} onClose={close} />}
    </PageFrame>
  )
}

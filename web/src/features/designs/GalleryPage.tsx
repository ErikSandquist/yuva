import "@/features/designs/i18n"

import { Trans, useLingui } from "@lingui/react/macro"
import { FilePlusIcon, LanguagesIcon, LayoutTemplateIcon, LoaderIcon, TriangleAlertIcon } from "lucide-react"
import { useState } from "react"
import { useNavigate, useSearchParams } from "react-router"

import { Dot, ErrorLine, Notice, Segmented } from "@/components/common"
import { Pane, PaneBack, PaneEmpty } from "@/components/common/Column"
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
import { designLink, useCreateDesign, useTemplates } from "@/features/designs/queries"
import { useTemplateLocales } from "@/features/designs/text"
import type { EmailTemplate, EmailTemplateLocale } from "@/lib/api"
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
  return (
    <Pane wide testId="design-gallery">
      <header className="flex flex-col">
        <PaneBack to="/broadcasts/designs" label={t`Designs`} />
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div className="min-w-0">
            <h1 className="text-page">
              <Trans>New design</Trans>
            </h1>
            <p className="mt-1.5 max-w-2xl text-reading text-muted-foreground">
              <Trans>
                Start from a template or an empty page. Every design gets the footer the law asks for, and you can
                change everything else.
              </Trans>
            </p>
          </div>
          <Segmented
            value={locale}
            onChange={setLocale}
            label={t`Template language`}
            items={localeNames.map((l) => ({ ...l, testId: `template-locale-${l.value}` }))}
          />
        </div>
      </header>
      {unknown && (
        <Notice tone="warning" icon={TriangleAlertIcon} data-testid="template-unknown">
          <p>
            <Trans>This server does not have the template “{slug}”. It may need a newer Yuva.</Trans>
          </p>
        </Notice>
      )}
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
      <ErrorLine error={templates.error} />
      {open && <CreateDialog key={open.template?.slug ?? "blank"} choice={open} locale={locale} onClose={close} />}
    </Pane>
  )
}

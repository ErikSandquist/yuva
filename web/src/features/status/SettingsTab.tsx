import { Trans, useLingui } from "@lingui/react/macro"
import { MailWarningIcon } from "lucide-react"
import { useState } from "react"
import { Link, useNavigate } from "react-router"

import { CopyButton, ErrorLine, Notice, toast, useConfirm } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { NativeSelect } from "@/components/ui/native-select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { canSend } from "@/features/broadcasts/ListForm"
import { useInboxChannels } from "@/features/broadcasts/queries"
import { DomainCard } from "@/features/status/DomainCard"
import { validSlug } from "@/features/status/CreatePageDialog"
import { useDeleteStatusPage, usePatchStatusPage } from "@/features/status/queries"
import { Card, ChoiceSelect, Field, FormActions, FormCard, Section, ToggleRow } from "@/features/settings/ui"
import type { StatusPage, StatusPageUpdate } from "@/lib/api"

const COLOR_RE = /^#[0-9a-fA-F]{6}$/

function zones(current: string) {
  const all =
    typeof Intl.supportedValuesOf === "function" ? Intl.supportedValuesOf("timeZone") : ["UTC", "Europe/Istanbul"]
  return all.includes(current) ? all : [current, ...all]
}

function General({ page }: { page: StatusPage }) {
  const { t } = useLingui()
  const patch = usePatchStatusPage(page.id)
  const [name, setName] = useState(page.name)
  const [description, setDescription] = useState(page.description)
  const [slug, setSlug] = useState(page.slug)
  const [logo, setLogo] = useState(page.logo_url ?? "")
  const [color, setColor] = useState(page.accent_color ?? "")
  const [site, setSite] = useState(page.site_url ?? "")
  const [support, setSupport] = useState(page.support_url ?? "")
  const [locale, setLocale] = useState(page.locale)
  const [timezone, setTimezone] = useState(page.timezone)
  const [days, setDays] = useState(String(page.history_days))
  const [uptime, setUptime] = useState(page.show_uptime)

  const link = (v: string) => (v.trim() ? v.trim() : null)
  const body: StatusPageUpdate = {}
  if (name.trim() !== page.name) body.name = name.trim()
  if (description.trim() !== page.description) body.description = description.trim()
  if (slug !== page.slug) body.slug = slug
  if (link(logo) !== (page.logo_url ?? null)) body.logo_url = link(logo)
  if (link(color) !== (page.accent_color ?? null)) body.accent_color = link(color)
  if (link(site) !== (page.site_url ?? null)) body.site_url = link(site)
  if (link(support) !== (page.support_url ?? null)) body.support_url = link(support)
  if (locale !== page.locale) body.locale = locale
  if (timezone !== page.timezone) body.timezone = timezone
  if (Number(days) !== page.history_days) body.history_days = Number(days)
  if (uptime !== page.show_uptime) body.show_uptime = uptime
  const dirty = Object.keys(body).length > 0
  const daysOk = Number.isInteger(Number(days)) && Number(days) >= 7 && Number(days) <= 90
  const colorOk = !color || COLOR_RE.test(color)
  const ok = !!name.trim() && validSlug(slug) && daysOk && colorOk
  const locales: [string, string][] = [
    ["en", t`English`],
    ["tr", t`Türkçe`],
    ["de", t`Deutsch`],
  ]
  if (!locales.some(([v]) => v === page.locale)) locales.push([page.locale, page.locale])
  const address = page.url.slice(0, page.url.length - page.slug.length) + slug

  return (
    <Section title={<Trans>Page</Trans>}>
      <FormCard
        onSubmit={() =>
          ok &&
          patch.mutate(body, {
            onSuccess: (saved) => {
              setName(saved.name)
              setSlug(saved.slug)
              toast(t`Saved`)
            },
          })
        }
        footer={
          <FormActions
            pending={patch.isPending}
            saved={patch.isSuccess && !dirty}
            error={patch.error}
            disabled={!dirty || !ok}
          />
        }
      >
        <Field label={<Trans>Name</Trans>} htmlFor="status-name">
          <Input
            id="status-name"
            value={name}
            maxLength={100}
            onChange={(e) => setName(e.target.value)}
            data-testid="status-setting-name"
          />
        </Field>
        <Field
          label={<Trans>Description</Trans>}
          htmlFor="status-description"
          hint={<Trans>Shown under the name on the public page.</Trans>}
        >
          <Textarea
            id="status-description"
            value={description}
            maxLength={1000}
            onChange={(e) => setDescription(e.target.value)}
            data-testid="status-setting-description"
          />
        </Field>
        <Field
          label={<Trans>Address</Trans>}
          htmlFor="status-slug"
          hint={<Trans>The page is at {address}. The old address keeps redirecting if you change it.</Trans>}
          error={slug && !validSlug(slug) ? <Trans>Use 3 to 48 letters, numbers and inner hyphens.</Trans> : undefined}
        >
          <Input
            id="status-slug"
            value={slug}
            maxLength={48}
            spellCheck={false}
            autoCapitalize="none"
            onChange={(e) => setSlug(e.target.value.toLowerCase())}
            data-testid="status-setting-slug"
          />
        </Field>
        <div className="grid gap-5 sm:grid-cols-2">
          <Field label={<Trans>Logo URL</Trans>} htmlFor="status-logo" hint={<Trans>An https image.</Trans>}>
            <Input
              id="status-logo"
              type="url"
              value={logo}
              placeholder="https://"
              onChange={(e) => setLogo(e.target.value)}
              data-testid="status-setting-logo"
            />
          </Field>
          <Field
            label={<Trans>Accent colour</Trans>}
            htmlFor="status-color"
            error={!colorOk ? <Trans>Use a colour like #2563eb.</Trans> : undefined}
          >
            <div className="flex items-center gap-2">
              <input
                type="color"
                aria-label={t`Pick a colour`}
                value={COLOR_RE.test(color) ? color : "#2563eb"}
                onChange={(e) => setColor(e.target.value)}
                className="size-9 shrink-0 cursor-pointer rounded-xl border border-input bg-background p-1"
              />
              <Input
                id="status-color"
                value={color}
                maxLength={7}
                placeholder="#2563eb"
                spellCheck={false}
                onChange={(e) => setColor(e.target.value)}
                data-testid="status-setting-color"
              />
            </div>
          </Field>
          <Field label={<Trans>Site link</Trans>} htmlFor="status-site" hint={<Trans>Your product.</Trans>}>
            <Input
              id="status-site"
              type="url"
              value={site}
              placeholder="https://"
              onChange={(e) => setSite(e.target.value)}
              data-testid="status-setting-site"
            />
          </Field>
          <Field
            label={<Trans>Support link</Trans>}
            htmlFor="status-support"
            hint={<Trans>A page or a mailto: link where customers get help.</Trans>}
          >
            <Input
              id="status-support"
              value={support}
              placeholder="https://"
              onChange={(e) => setSupport(e.target.value)}
              data-testid="status-setting-support"
            />
          </Field>
          <Field
            label={<Trans>Language</Trans>}
            htmlFor="status-locale"
            hint={<Trans>The language of the page’s words.</Trans>}
          >
            <ChoiceSelect
              id="status-locale"
              value={locale}
              onChange={setLocale}
              options={locales}
              label={t`Language`}
            />
          </Field>
          <Field
            label={<Trans>Time zone</Trans>}
            htmlFor="status-timezone"
            hint={<Trans>The zone of the day bars.</Trans>}
          >
            <NativeSelect
              id="status-timezone"
              className="w-full"
              value={timezone}
              onChange={(e) => setTimezone(e.target.value)}
              data-testid="status-setting-timezone"
            >
              {zones(page.timezone).map((z) => (
                <option key={z} value={z}>
                  {z}
                </option>
              ))}
            </NativeSelect>
          </Field>
          <Field
            label={<Trans>Days of history</Trans>}
            htmlFor="status-days"
            hint={<Trans>7 to 90.</Trans>}
            error={!daysOk ? <Trans>Use a whole number from 7 to 90.</Trans> : undefined}
          >
            <Input
              id="status-days"
              type="number"
              min={7}
              max={90}
              value={days}
              onChange={(e) => setDays(e.target.value)}
              data-testid="status-setting-days"
            />
          </Field>
        </div>
        <ToggleRow title={<Trans>Show uptime</Trans>} hint={<Trans>A percentage next to each component.</Trans>}>
          <Switch checked={uptime} onCheckedChange={setUptime} data-testid="status-setting-uptime" />
        </ToggleRow>
      </FormCard>
    </Section>
  )
}

function EmailSubscribers({ page }: { page: StatusPage }) {
  const { t } = useLingui()
  const patch = usePatchStatusPage(page.id)
  const channels = (useInboxChannels(page.inbox_id).data ?? []).filter(canSend)
  const sub = page.email_subscribers
  const options: [string, string][] = channels.map((c) => [c.id, c.email ? `${c.name} · ${c.email.address}` : c.name])
  const channelId = sub.channel_id ?? ""
  if (channelId && !options.some(([id]) => id === channelId)) options.unshift([channelId, channelId])
  const warnings: Record<(typeof sub.warnings)[number], string> = {
    no_deliverability_check: t`The sending domain has not passed a check of SPF, DKIM and DMARC yet. Notices may land in spam.`,
    no_postal_address: t`The inbox names no postal address for the sender. Add one in the inbox settings.`,
    dkim_unaligned: t`Mail is not signed with a key of the sender’s own domain, which Gmail and Yahoo expect.`,
  }
  return (
    <Section
      title={<Trans>E-mail subscribers</Trans>}
      description={
        <Trans>Visitors subscribe on the page and get a mail when an incident or maintenance changes.</Trans>
      }
    >
      <Card>
        <ToggleRow
          title={<Trans>Send notices by e-mail</Trans>}
          hint={<Trans>Subscribers confirm their address by mail before they get notices.</Trans>}
        >
          <Switch
            checked={sub.enabled}
            disabled={patch.isPending}
            onCheckedChange={(enabled) =>
              patch.mutate({
                email_subscribers: {
                  enabled,
                  ...(enabled && !sub.channel_id && channels.length === 1 ? { channel_id: channels[0].id } : {}),
                },
              })
            }
            data-testid="status-setting-subscribers"
          />
        </ToggleRow>
        <Field
          label={<Trans>Sends its mail from</Trans>}
          htmlFor="status-channel"
          hint={
            channels.length === 0 ? (
              <Trans>This inbox has no e-mail channel that can send. Add an SMTP account in the inbox settings.</Trans>
            ) : (
              <Trans>The inbox’s e-mail channel that sends confirmations and notices.</Trans>
            )
          }
        >
          <ChoiceSelect
            id="status-channel"
            value={channelId}
            onChange={(id) => patch.mutate({ email_subscribers: { channel_id: id } })}
            options={options}
            placeholder={t`Choose a channel`}
            label={t`Sends its mail from`}
            disabled={patch.isPending || options.length === 0}
          />
        </Field>
        {sub.enabled && !sub.sender_ready && (
          <Notice tone="warning" icon={MailWarningIcon} data-testid="status-subscribers-no-sender">
            <p>
              <Trans>
                No channel that can send is chosen. Until there is one, nobody gets a confirmation or a notice.
              </Trans>
            </p>
            <Link
              to={`/settings/inboxes/${page.inbox_id}`}
              className="w-fit text-brand underline-offset-4 hover:underline"
            >
              <Trans>Open the inbox settings</Trans>
            </Link>
          </Notice>
        )}
        {sub.warnings.map((w) => (
          <Notice key={w} tone="warning" icon={MailWarningIcon} data-testid="status-subscribers-warning">
            <p>{warnings[w]}</p>
          </Notice>
        ))}
        <ErrorLine error={patch.error} />
      </Card>
    </Section>
  )
}

function Address({ page }: { page: StatusPage }) {
  return (
    <Section title={<Trans>Public address</Trans>}>
      <Card>
        <div className="flex flex-wrap items-center gap-3">
          {page.published ? (
            <a
              href={page.url}
              target="_blank"
              rel="noreferrer noopener"
              className="min-w-0 flex-1 break-all text-body text-brand underline-offset-4 hover:underline"
              data-testid="status-public-url"
            >
              {page.url}
            </a>
          ) : (
            <span className="min-w-0 flex-1 break-all text-body text-muted-foreground" data-testid="status-public-url">
              {page.url}
            </span>
          )}
          <CopyButton value={page.url} />
        </div>
        {!page.published && (
          <p className="text-small text-faint">
            <Trans>The address answers “not found” until you publish the page.</Trans>
          </p>
        )}
      </Card>
    </Section>
  )
}

function Danger({ page }: { page: StatusPage }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const [confirm, dialog] = useConfirm()
  const remove = useDeleteStatusPage(page.id)
  return (
    <>
      <Section title={<Trans>Delete</Trans>}>
        <Card tone="danger">
          <div className="flex flex-wrap items-center gap-3">
            <p className="min-w-0 flex-1 text-body">
              <Trans>
                Deletes the page with its components, their history, incidents and updates. The public address stops
                working.
              </Trans>
            </p>
            <Button
              variant="destructive"
              size="sm"
              disabled={remove.isPending}
              onClick={() =>
                confirm({
                  title: <Trans>Delete {page.name}?</Trans>,
                  description: <Trans>Components, history, incidents and updates are deleted with it.</Trans>,
                  confirm: <Trans>Delete page</Trans>,
                  run: () =>
                    remove.mutate(undefined, {
                      onSuccess: () => {
                        toast(t`Status page deleted`)
                        navigate("/status")
                      },
                    }),
                })
              }
              data-testid="status-delete"
            >
              <Trans>Delete page</Trans>
            </Button>
          </div>
          <ErrorLine error={remove.error} />
        </Card>
      </Section>
      {dialog}
    </>
  )
}

export function SettingsTab({ page }: { page: StatusPage }) {
  return (
    <div className="flex flex-col gap-8" data-testid="status-settings">
      <Address page={page} />
      <General page={page} />
      <DomainCard page={page} />
      <EmailSubscribers page={page} />
      <Danger page={page} />
    </div>
  )
}

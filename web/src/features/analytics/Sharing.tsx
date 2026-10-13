import { Trans, useLingui } from "@lingui/react/macro"
import { LockIcon, PencilIcon, PlusIcon, Share2Icon, Trash2Icon } from "lucide-react"
import { useState } from "react"

import { CodeBlock, CodeLine, ErrorLine, useConfirm } from "@/components/common"
import { formatDate, formatDateTime } from "@/components/common/text"
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
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { splitLines } from "@/features/analytics/NewProperty"
import { useCreateShare, useShareActions, useShares } from "@/features/analytics/queries"
import { todayIso } from "@/features/analytics/state"
import { useAnalyticsText } from "@/features/analytics/text"
import { Card, EmptyRow, Field, Row, Rows, Section, StatusTag } from "@/features/settings/ui"
import { useNow } from "@/hooks/use-now"
import {
  ApiError,
  type AnalyticsProperty,
  type AnalyticsShare,
  type AnalyticsShareCreated,
  type AnalyticsShareView,
} from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useMemberMap } from "@/lib/workspace"

export const SHARE_VIEWS: AnalyticsShareView[] = [
  "overview",
  "realtime",
  "pages",
  "sources",
  "locations",
  "devices",
  "events",
  "goals",
]

export function useShareViewNames(): Record<AnalyticsShareView, string> {
  const { t } = useLingui()
  const text = useAnalyticsText()
  return {
    overview: t`Overview`,
    realtime: t`Realtime`,
    pages: text.title.pages,
    sources: t`Sources`,
    locations: t`Locations`,
    devices: t`Devices`,
    events: t`Events`,
    goals: t`Goals`,
  }
}

function localDay(iso: string) {
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function endOfDay(day: string) {
  return new Date(`${day}T23:59:59`).toISOString()
}

function ShareDialog({
  propertyId,
  share,
  onClose,
  onCreated,
}: {
  propertyId: string
  share?: AnalyticsShare
  onClose: () => void
  onCreated: (c: AnalyticsShareCreated) => void
}) {
  const { t } = useLingui()
  const names = useShareViewNames()
  const create = useCreateShare(propertyId)
  const { update } = useShareActions(propertyId)
  const [name, setName] = useState(share?.name ?? "")
  const [views, setViews] = useState<AnalyticsShareView[]>(share?.views ?? ["overview"])
  const [locked, setLocked] = useState(share?.has_password ?? false)
  const [password, setPassword] = useState("")
  const [origins, setOrigins] = useState((share?.embed_origins ?? []).join("\n"))
  const [expires, setExpires] = useState(share?.expires_at ? localDay(share.expires_at) : "")
  const today = todayIso()
  const keepsPassword = !!share?.has_password && locked && !password
  const passwordOk = !locked || keepsPassword || password.length >= 8
  const list = splitLines(origins)
  const valid = !!name.trim() && views.length > 0 && passwordOk && list.length <= 20 && (!expires || expires >= today)
  const save = share ? update : create
  const submit = () => {
    const ordered = SHARE_VIEWS.filter((v) => views.includes(v))
    if (!share) {
      create.mutate(
        {
          name: name.trim(),
          views: ordered,
          ...(locked ? { password } : {}),
          ...(list.length ? { embed_origins: list } : {}),
          ...(expires ? { expires_at: endOfDay(expires) } : {}),
        },
        { onSuccess: onCreated }
      )
      return
    }
    const before = share.expires_at ? localDay(share.expires_at) : ""
    update.mutate(
      {
        shareId: share.id,
        body: {
          name: name.trim(),
          views: ordered,
          embed_origins: list,
          ...(!locked && share.has_password ? { password: null } : locked && password ? { password } : {}),
          ...(expires !== before ? { expires_at: expires ? endOfDay(expires) : null } : {}),
        },
      },
      { onSuccess: onClose }
    )
  }
  const limit = save.error instanceof ApiError && save.error.code === "share_limit"
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg" data-testid="share-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            if (valid && !save.isPending) submit()
          }}
        >
          <DialogHeader>
            <DialogTitle>{share ? <Trans>Edit share link</Trans> : <Trans>New share link</Trans>}</DialogTitle>
            <DialogDescription>
              <Trans>Anyone with the link sees the views you pick, read-only, without signing in.</Trans>
            </DialogDescription>
          </DialogHeader>
          <Field
            label={<Trans>Name</Trans>}
            htmlFor="share-name"
            hint={<Trans>Only members see it, to tell links apart.</Trans>}
          >
            <Input
              id="share-name"
              required
              maxLength={100}
              value={name}
              placeholder={t`Investors`}
              onChange={(e) => setName(e.target.value)}
              data-testid="share-name"
            />
          </Field>
          <Field label={<Trans>Views</Trans>}>
            <div className="grid grid-cols-2 gap-x-4 gap-y-2.5">
              {SHARE_VIEWS.map((v) => (
                <Label key={v} className="font-normal" data-testid={`share-view-${v}`}>
                  <Checkbox
                    checked={views.includes(v)}
                    onCheckedChange={(on) => setViews(on ? [...views, v] : views.filter((x) => x !== v))}
                    aria-invalid={views.length === 0 || undefined}
                  />
                  {names[v]}
                </Label>
              ))}
            </div>
            {views.length === 0 && (
              <p role="alert" className="text-small text-destructive">
                <Trans>Pick at least one view.</Trans>
              </p>
            )}
          </Field>
          <div className="flex flex-col gap-2.5">
            <Label className="font-normal" data-testid="share-locked">
              <Checkbox checked={locked} onCheckedChange={(on) => setLocked(!!on)} />
              <Trans>Ask for a password</Trans>
            </Label>
            {locked && (
              <Input
                type="password"
                autoComplete="new-password"
                minLength={share?.has_password ? undefined : 8}
                maxLength={200}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={share?.has_password ? t`Leave empty to keep the current one` : t`At least 8 characters`}
                aria-label={t`Password`}
                data-testid="share-password"
              />
            )}
            {share?.has_password && locked && password && (
              <p className="text-small text-faint">
                <Trans>A new password signs out everyone who unlocked the link.</Trans>
              </p>
            )}
          </div>
          <Field
            label={<Trans>Sites that may embed it</Trans>}
            htmlFor="share-origins"
            hint={
              <Trans>
                One per line, like https://example.com or https://*.example.com. Without any, no other site can show it
                in a frame.
              </Trans>
            }
          >
            <Textarea
              id="share-origins"
              dir="ltr"
              rows={2}
              value={origins}
              onChange={(e) => setOrigins(e.target.value)}
              placeholder="https://example.com"
              data-testid="share-origins"
            />
          </Field>
          <Field
            label={<Trans>Expires</Trans>}
            htmlFor="share-expires"
            hint={<Trans>The link stops working at the end of this day. Leave empty to keep it until revoked.</Trans>}
          >
            <div className="flex items-center gap-2">
              <Input
                id="share-expires"
                type="date"
                className="w-44"
                min={today}
                value={expires}
                onChange={(e) => setExpires(e.target.value)}
                data-testid="share-expires"
              />
              {expires && (
                <Button type="button" variant="ghost" size="sm" onClick={() => setExpires("")}>
                  <Trans>Clear</Trans>
                </Button>
              )}
            </div>
          </Field>
          {limit ? (
            <p role="alert" className="text-body text-destructive">
              <Trans>A property can have at most 50 share links. Delete one first.</Trans>
            </p>
          ) : (
            <ErrorLine error={save.error} />
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              <Trans>Cancel</Trans>
            </Button>
            <Button type="submit" disabled={!valid || save.isPending} data-testid="share-save">
              {share ? <Trans>Save</Trans> : <Trans>Make link</Trans>}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function CreatedDialog({ created, onClose }: { created: AnalyticsShareCreated; onClose: () => void }) {
  const embed = created.share.embed_origins.length > 0
  const snippet = `<iframe src="${created.url}" title="${created.share.name.replace(/"/g, "&quot;")}" width="100%" height="900" style="border:0" loading="lazy"></iframe>`
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg" data-testid="share-created">
        <DialogHeader>
          <DialogTitle>
            <Trans>Your new share link</Trans>
          </DialogTitle>
          <DialogDescription>
            <Trans>Copy it now; it is not shown again. Lost links can only be revoked and made anew.</Trans>
          </DialogDescription>
        </DialogHeader>
        <div className="flex min-w-0 flex-col gap-2">
          <Label>
            <Trans>Link</Trans>
          </Label>
          <CodeLine value={created.url} testId="share-url" />
        </div>
        <div className="flex min-w-0 flex-col gap-2">
          <Label>
            <Trans>Token</Trans>
          </Label>
          <CodeLine value={created.token} testId="share-token" />
        </div>
        {embed && (
          <div className="flex min-w-0 flex-col gap-2">
            <Label>
              <Trans>Embed</Trans>
            </Label>
            <CodeBlock value={snippet} testId="share-embed" />
          </div>
        )}
        <DialogFooter>
          <Button onClick={onClose}>
            <Trans>I saved it</Trans>
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function ShareRow({
  s,
  now,
  onEdit,
  onRevoke,
  onDelete,
}: {
  s: AnalyticsShare
  now: number
  onEdit?: () => void
  onRevoke?: () => void
  onDelete?: () => void
}) {
  const { t, i18n } = useLingui()
  const names = useShareViewNames()
  const members = useMemberMap()
  const creator = s.created_by ? members.get(s.created_by) : undefined
  const by = creator ? creator.name || creator.email : null
  const created = formatDate(s.created_at, i18n.locale)
  const viewed = s.last_viewed_at ? formatDateTime(s.last_viewed_at, i18n.locale) : null
  const expires = s.expires_at ? formatDateTime(s.expires_at, i18n.locale) : null
  const expired = !s.revoked_at && !!s.expires_at && Date.parse(s.expires_at) <= now
  const status = s.revoked_at ? (
    <StatusTag data-testid="share-status">
      <Trans>Revoked</Trans>
    </StatusTag>
  ) : s.active && !expired ? (
    <StatusTag tone="success" data-testid="share-status">
      <Trans>Active</Trans>
    </StatusTag>
  ) : (
    <StatusTag tone="warning" data-testid="share-status">
      <Trans>Expired</Trans>
    </StatusTag>
  )
  return (
    <Row className={cn("items-start", !s.active && "opacity-70")} data-testid="share-row">
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
          <p className="min-w-0 text-body font-medium break-words">
            {s.name} <code className="font-mono text-caption font-normal text-faint">{s.token_prefix}…</code>
          </p>
          {status}
          {s.has_password && (
            <LockIcon className="size-3.5 text-faint" aria-label={t`Password`} data-testid="share-has-password" />
          )}
        </div>
        <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-caption text-muted-foreground">
          <dt className="text-faint">
            <Trans>Views</Trans>
          </dt>
          <dd className="break-words">{s.views.map((v) => names[v]).join(", ")}</dd>
          {s.embed_origins.length > 0 && (
            <>
              <dt className="text-faint">
                <Trans>Embeds</Trans>
              </dt>
              <dd className="break-all" dir="ltr">
                {s.embed_origins.join(", ")}
              </dd>
            </>
          )}
          <dt className="text-faint">
            <Trans>Expires</Trans>
          </dt>
          <dd className={cn("truncate", expired && "text-destructive")}>{expires ?? <Trans>Never</Trans>}</dd>
        </dl>
        <p className="text-caption text-faint">
          {by ? (
            <Trans>
              Made {created} by {by}
            </Trans>
          ) : (
            <Trans>Made {created}</Trans>
          )}
          {" · "}
          {viewed ? <Trans>last opened {viewed}</Trans> : <Trans>never opened</Trans>}
        </p>
      </div>
      {(onEdit || onRevoke || onDelete) && (
        <div className="-my-1 flex shrink-0 items-center gap-1">
          {onEdit && !s.revoked_at && (
            <Button variant="ghost" size="icon-sm" aria-label={t`Edit`} title={t`Edit`} onClick={onEdit}>
              <PencilIcon />
            </Button>
          )}
          {onRevoke && !s.revoked_at && (
            <Button variant="ghost" size="sm" onClick={onRevoke} data-testid="share-revoke">
              <Trans>Revoke</Trans>
            </Button>
          )}
          {onDelete && (
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={t`Delete`}
              title={t`Delete`}
              onClick={onDelete}
              data-testid="share-delete"
            >
              <Trash2Icon />
            </Button>
          )}
        </div>
      )}
    </Row>
  )
}

export function Sharing({ p }: { p: AnalyticsProperty }) {
  const { canManage } = useSession()
  const list = useShares(p.id)
  const { revoke, remove } = useShareActions(p.id)
  const [editing, setEditing] = useState<AnalyticsShare | "new" | null>(null)
  const [created, setCreated] = useState<AnalyticsShareCreated | null>(null)
  const [ask, dialog] = useConfirm()
  const now = useNow(60_000)
  const items = list.data ?? []
  return (
    <div className="flex flex-col gap-6" data-testid="analytics-sharing">
      <Section
        title={<Trans>Share links</Trans>}
        action={
          canManage && (
            <Button size="sm" onClick={() => setEditing("new")} data-testid="share-new">
              <PlusIcon />
              <Trans>New link</Trans>
            </Button>
          )
        }
        description={
          <Trans>
            A link shows only its views of this property, read-only, without filters or segments. Revoking or deleting
            it stops it at once.
          </Trans>
        }
      >
        <Card flush>
          {list.isPending ? null : items.length === 0 ? (
            <EmptyRow>
              <span className="flex items-center gap-2">
                <Share2Icon className="size-4 shrink-0" />
                {canManage ? (
                  <Trans>No share links yet. Make one to show these numbers to people outside the workspace.</Trans>
                ) : (
                  <Trans>No share links yet. Owners and admins make them.</Trans>
                )}
              </span>
            </EmptyRow>
          ) : (
            <Rows>
              {items.map((s) => (
                <ShareRow
                  key={s.id}
                  s={s}
                  now={now}
                  onEdit={canManage ? () => setEditing(s) : undefined}
                  onRevoke={
                    canManage
                      ? () => {
                          const name = s.name
                          ask({
                            title: <Trans>Revoke {name}?</Trans>,
                            description: <Trans>The link stops working at once and stays in the list.</Trans>,
                            confirm: <Trans>Revoke</Trans>,
                            run: () => revoke.mutate(s),
                          })
                        }
                      : undefined
                  }
                  onDelete={
                    canManage
                      ? () => {
                          const name = s.name
                          ask({
                            title: <Trans>Delete {name}?</Trans>,
                            description: <Trans>The link stops working at once and leaves the list.</Trans>,
                            confirm: <Trans>Delete</Trans>,
                            run: () => remove.mutate(s),
                          })
                        }
                      : undefined
                  }
                />
              ))}
            </Rows>
          )}
          <ErrorLine error={list.error ?? revoke.error ?? remove.error} className="px-4 py-3" />
        </Card>
      </Section>
      {editing && (
        <ShareDialog
          propertyId={p.id}
          share={editing === "new" ? undefined : editing}
          onClose={() => setEditing(null)}
          onCreated={(c) => {
            setEditing(null)
            setCreated(c)
          }}
        />
      )}
      {created && <CreatedDialog created={created} onClose={() => setCreated(null)} />}
      {dialog}
    </div>
  )
}

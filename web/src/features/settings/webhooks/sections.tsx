import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import { useNavigate } from "react-router"

import { ErrorLine, Notice, SecretDialog, toast, useConfirm } from "@/components/common"
import { formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { PageSection } from "@/features/settings/channels/sections"
import { Card, Field, FormActions, FormCard, ToggleRow } from "@/features/settings/ui"
import { EventPicker, sortedEvents, UrlProblem, urlProblem, validUrl } from "@/features/settings/webhooks/events"
import { useSetEndpoint } from "@/features/settings/webhooks/queries"
import { api, unwrap, type WebhookEndpoint, type WebhookEventType } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

function usePatchEndpoint(e: WebhookEndpoint) {
  const setEndpoint = useSetEndpoint()
  return useMutation({
    mutationFn: (body: {
      url?: string
      description?: string
      events?: WebhookEventType[]
      include_notes?: boolean
      enabled?: boolean
    }) => unwrap(api.PATCH("/v1/webhooks/{webhookId}", { params: { path: { webhookId: e.id } }, body })),
    onSuccess: setEndpoint,
  })
}

export function EndpointSection({ e }: { e: WebhookEndpoint }) {
  const inboxes = useInboxes().data ?? []
  const patch = usePatchEndpoint(e)
  const [url, setUrl] = useState(e.url)
  const [description, setDescription] = useState(e.description)
  const inbox = e.inbox_id ? inboxes.find((i) => i.id === e.inbox_id) : undefined
  const dirty = url.trim() !== e.url || description.trim() !== e.description
  const bad = urlProblem(patch.error)
  const invalid = url.trim() !== e.url && !validUrl(url)
  return (
    <PageSection id="endpoint" title={<Trans>Endpoint</Trans>}>
      <FormCard
        onSubmit={() =>
          !invalid &&
          patch.mutate(
            { url: url.trim(), description: description.trim() },
            {
              onSuccess: (saved) => {
                setUrl(saved.url)
                setDescription(saved.description)
              },
            }
          )
        }
        footer={
          <FormActions
            pending={patch.isPending}
            saved={patch.isSuccess && !dirty}
            error={bad ? undefined : patch.error}
            disabled={!dirty || !url.trim() || invalid}
          />
        }
      >
        <Field
          label={<Trans>URL</Trans>}
          htmlFor="webhook-url"
          error={bad ? <UrlProblem problem={bad} /> : invalid ? <UrlProblem problem="invalid" /> : undefined}
        >
          <Input
            id="webhook-url"
            type="url"
            required
            maxLength={2000}
            value={url}
            placeholder="https://api.example.com/yuva/webhook"
            onChange={(ev) => {
              patch.reset()
              setUrl(ev.target.value)
            }}
            aria-invalid={!!bad || invalid || undefined}
            className="font-mono text-caption"
            data-testid="webhook-url-input"
          />
        </Field>
        <Field label={<Trans>Description (optional)</Trans>} htmlFor="webhook-description">
          <Input
            id="webhook-description"
            maxLength={200}
            value={description}
            onChange={(ev) => {
              patch.reset()
              setDescription(ev.target.value)
            }}
          />
        </Field>
        <Field label={<Trans>Inbox</Trans>} hint={<Trans>The inbox cannot be changed later.</Trans>}>
          <p className="text-body">{inbox ? inbox.name : <Trans>All inboxes</Trans>}</p>
        </Field>
      </FormCard>
    </PageSection>
  )
}

export function EventsSection({ e }: { e: WebhookEndpoint }) {
  const patch = usePatchEndpoint(e)
  const [events, setEvents] = useState<WebhookEventType[]>(e.events)
  const [includeNotes, setIncludeNotes] = useState(e.include_notes)
  const notes = includeNotes && events.includes("message.created")
  const dirty = events.join() !== sortedEvents(e.events).join() || notes !== e.include_notes
  return (
    <PageSection id="events" title={<Trans>Events</Trans>}>
      <FormCard
        onSubmit={() =>
          patch.mutate(
            { events, include_notes: notes },
            {
              onSuccess: (saved) => {
                setEvents(sortedEvents(saved.events))
                setIncludeNotes(saved.include_notes)
              },
            }
          )
        }
        footer={
          <FormActions
            pending={patch.isPending}
            saved={patch.isSuccess && !dirty}
            error={patch.error}
            disabled={!dirty || events.length === 0}
          />
        }
      >
        <EventPicker
          events={events}
          includeNotes={includeNotes}
          onEvents={(next) => {
            patch.reset()
            setEvents(next)
          }}
          onIncludeNotes={(on) => {
            patch.reset()
            setIncludeNotes(on)
          }}
        />
        {events.length === 0 && (
          <p role="alert" className="text-small text-destructive">
            <Trans>Pick at least one event.</Trans>
          </p>
        )}
      </FormCard>
    </PageSection>
  )
}

export function SendingSection({ e }: { e: WebhookEndpoint }) {
  const { t, i18n } = useLingui()
  const patch = usePatchEndpoint(e)
  const disabledAt = e.disabled_at ? formatDateTime(e.disabled_at, i18n.locale) : null
  const failingSince = e.failing_since ? formatDateTime(e.failing_since, i18n.locale) : ""
  return (
    <PageSection id="sending" title={<Trans>Sending</Trans>}>
      <Card>
        {!e.enabled && e.disabled_reason && (
          <Notice tone="danger" data-testid="webhook-disabled">
            <p className="font-medium">
              {disabledAt ? (
                <Trans>Yuva turned this endpoint off on {disabledAt}.</Trans>
              ) : (
                <Trans>Yuva turned this endpoint off.</Trans>
              )}
            </p>
            <p className="break-words text-muted-foreground">{e.disabled_reason}</p>
            <p className="text-muted-foreground">
              <Trans>Turn it on again once the receiver works; send failed deliveries again from the log below.</Trans>
            </p>
          </Notice>
        )}
        {e.enabled && e.failing_since && (
          <Notice tone="warning" data-testid="webhook-failing">
            <p>
              <Trans>
                Every attempt has failed since {failingSince}. After 24 hours of failures Yuva turns the endpoint off.
              </Trans>
            </p>
          </Notice>
        )}
        <ToggleRow title={<Trans>Send events</Trans>} hint={<Trans>While off, no deliveries are made.</Trans>}>
          <Switch
            checked={e.enabled}
            disabled={patch.isPending}
            onCheckedChange={(on) => patch.mutate({ enabled: on }, { onSuccess: () => toast(<Trans>Saved</Trans>) })}
            aria-label={t`Send events`}
            data-testid="webhook-enabled"
          />
        </ToggleRow>
        <ErrorLine error={patch.error} className="text-caption" />
      </Card>
    </PageSection>
  )
}

export function SecretSection({ e }: { e: WebhookEndpoint }) {
  const { i18n } = useLingui()
  const setEndpoint = useSetEndpoint()
  const [secret, setSecret] = useState<string | null>(null)
  const [confirm, confirmDialog] = useConfirm()
  const rotate = useMutation({
    mutationFn: () => unwrap(api.POST("/v1/webhooks/{webhookId}/secret", { params: { path: { webhookId: e.id } } })),
    onSuccess: (r) => {
      setEndpoint(r.endpoint)
      setSecret(r.secret)
    },
  })
  const rotatedUntil = e.secret_rotated_at
    ? formatDateTime(new Date(new Date(e.secret_rotated_at).getTime() + 24 * 3600_000).toISOString(), i18n.locale)
    : null
  return (
    <PageSection id="secret" title={<Trans>Signing secret</Trans>}>
      <Card>
        <div className="flex flex-wrap items-center gap-3">
          <p className="min-w-0 flex-1 text-body text-muted-foreground">
            <Trans>The secret is shown once, when it is made. Rotate it to get a new one.</Trans>
          </p>
          <Button
            variant="outline"
            size="sm"
            disabled={rotate.isPending}
            data-testid="webhook-rotate"
            onClick={() =>
              confirm({
                title: <Trans>Rotate the signing secret?</Trans>,
                description: (
                  <Trans>
                    You get a new secret once. For 24 hours every delivery carries two signatures, one with the new and
                    one with the old secret, so you can update your receiver without losing events.
                  </Trans>
                ),
                confirm: <Trans>Rotate</Trans>,
                run: () => rotate.mutate(),
              })
            }
          >
            <Trans>Rotate secret</Trans>
          </Button>
        </div>
        {rotatedUntil && (
          <p className="text-caption text-muted-foreground" data-testid="webhook-rotated">
            <Trans>The previous secret also signs until {rotatedUntil}.</Trans>
          </p>
        )}
        <ErrorLine error={rotate.error} className="text-caption" />
      </Card>
      <SecretDialog
        secret={secret}
        title={<Trans>New signing secret</Trans>}
        description={
          <Trans>
            Copy it now and update your receiver; the old secret keeps signing for 24 hours. It is not shown again.
          </Trans>
        }
        onClose={() => setSecret(null)}
      />
      {confirmDialog}
    </PageSection>
  )
}

export function RemoveSection({ e }: { e: WebhookEndpoint }) {
  const qc = useQueryClient()
  const navigate = useNavigate()
  const { workspaceId: ws } = useSession()
  const [confirm, confirmDialog] = useConfirm()
  const remove = useMutation({
    mutationFn: () => unwrap(api.DELETE("/v1/webhooks/{webhookId}", { params: { path: { webhookId: e.id } } })),
    onSuccess: () => {
      qc.removeQueries({ queryKey: keys.webhook(ws, e.id) })
      void qc.invalidateQueries({ queryKey: keys.webhooks(ws) })
      navigate("/settings/webhooks")
    },
  })
  return (
    <PageSection id="remove" title={<Trans>Remove</Trans>}>
      <Card tone="danger">
        <div className="flex flex-wrap items-center gap-3">
          <p className="min-w-0 flex-1 text-body text-muted-foreground">
            <Trans>Pending deliveries are dropped with it.</Trans>
          </p>
          <Button
            variant="destructive"
            size="sm"
            disabled={remove.isPending}
            data-testid="webhook-remove"
            onClick={() =>
              confirm({
                title: <Trans>Remove this endpoint?</Trans>,
                description: <Trans>Pending deliveries are dropped with it.</Trans>,
                confirm: <Trans>Remove</Trans>,
                run: () => remove.mutate(),
              })
            }
          >
            <Trans>Remove endpoint</Trans>
          </Button>
        </div>
        <ErrorLine error={remove.error} className="text-caption" />
      </Card>
      {confirmDialog}
    </PageSection>
  )
}

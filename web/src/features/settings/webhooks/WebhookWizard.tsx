import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation } from "@tanstack/react-query"
import { GlobeIcon, InboxIcon, PartyPopperIcon } from "lucide-react"
import { useEffect, useState } from "react"
import { Link, Navigate, useLocation, useNavigate, useSearchParams } from "react-router"

import { ChoiceCards, ErrorLine, StepActions, StepHeader, SummaryList, WizardFrame } from "@/components/common"
import { CopyButton } from "@/components/common/CopyButton"
import { sortedEvents } from "@/features/settings/webhooks/events"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { useWebhookDraft, webhookPaths, type WebhookWizardState } from "@/features/settings/webhooks/draft"
import { EventPicker, UrlProblem, urlProblem, validUrl } from "@/features/settings/webhooks/events"
import { useSetEndpoint } from "@/features/settings/webhooks/queries"
import { SignatureHint } from "@/features/settings/webhooks/SignatureHint"
import { Field } from "@/features/settings/ui"
import { idempotencyKey } from "@/features/setup/draft"
import { PreviewCaption } from "@/features/setup/previews"
import { api, unwrap } from "@/lib/api"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

type Stage = "endpoint" | "events" | "done"

function useLockedInbox() {
  const [params] = useSearchParams()
  return params.get("inbox")
}

function Envelope({ event }: { event: string }) {
  const sample = {
    type: event,
    timestamp: "2026-01-15T10:24:00Z",
    workspace_id: "0194b2c0-7a1e-7c3d-9f40-2d6a8e51b3f7",
    inbox_id: "0194b2c1-0d52-7a8e-b6c9-41e7f3a09d28",
    data: { id: "0194b2c4-5e83-7f1a-8d27-90c4b1a6e3d5" },
  }
  return (
    <div className="flex w-full max-w-[26rem] flex-col items-center" aria-hidden="true">
      <pre className="w-full rounded-2xl border bg-card p-5 font-mono text-caption leading-relaxed break-all whitespace-pre-wrap shadow-[0_1px_2px_rgb(15_23_42/0.04),0_24px_60px_-28px_rgb(15_23_42/0.35)]">
        {JSON.stringify(sample, null, 2)}
      </pre>
    </div>
  )
}

function Preview({ event }: { event: string }) {
  return (
    <>
      <Envelope event={event} />
      <PreviewCaption>
        <Trans>Yuva sends a signed POST request with a JSON body for every event you pick.</Trans>
      </PreviewCaption>
    </>
  )
}

function Layout({
  stage,
  locked,
  event,
  onLeave,
  children,
}: {
  stage: Stage
  locked: string | null
  event: string
  onLeave?: () => void
  children: React.ReactNode
}) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const state = (useLocation().state ?? {}) as WebhookWizardState
  const paths = webhookPaths(locked)
  const home = locked ? `/settings/inboxes/${locked}/webhooks` : "/settings/webhooks"
  const steps = [
    { key: "endpoint", label: t`Endpoint` },
    { key: "events", label: t`Events` },
    { key: "done", label: t`Done` },
  ]
  return (
    <WizardFrame
      steps={steps}
      current={stage}
      stepHref={(key) => (key === "endpoint" ? paths.endpoint : undefined)}
      stepsLabel={t`Setup steps`}
      back={stage === "events" ? paths.endpoint : undefined}
      leave={
        stage === "done" ? null : (
          <Button
            variant="ghost"
            size="sm"
            data-testid="setup-later"
            onClick={() => {
              onLeave?.()
              navigate(state.back ?? home, { replace: true })
            }}
          >
            <Trans>Cancel</Trans>
          </Button>
        )
      }
      preview={<Preview event={event} />}
    >
      {children}
    </WizardFrame>
  )
}

export function WebhookEndpointStep() {
  const { canManage } = useSession()
  const locked = useLockedInbox()
  const navigate = useNavigate()
  const state = (useLocation().state ?? {}) as WebhookWizardState
  const inboxes = useInboxes().data ?? []
  const { t } = useLingui()
  const [draft, patch, clear] = useWebhookDraft(locked)
  const [problem, setProblem] = useState(state.urlError)
  const [touched, setTouched] = useState(false)
  if (!canManage) return <Navigate to="/" replace />
  const urlOk = validUrl(draft.url)
  const inboxId = locked ?? (draft.scope === "inbox" ? draft.inboxId || (inboxes[0]?.id ?? "") : "")
  const lockedName = locked ? inboxes.find((i) => i.id === locked)?.name : undefined
  const inboxNames: Record<string, string> = {}
  for (const i of inboxes) inboxNames[i.id] = i.name
  const shownProblem = problem ?? (touched && !urlOk ? "invalid" : null)
  return (
    <Layout stage="endpoint" locked={locked} event={draft.events[0] ?? "conversation.created"} onLeave={clear}>
      <StepHeader title={<Trans>Where should Yuva send events?</Trans>}>
        <Trans>Yuva sends a signed POST request with a JSON body for every event you pick.</Trans>
      </StepHeader>
      <form
        className="flex flex-col gap-6"
        data-testid="setup-webhook-endpoint"
        onSubmit={(e) => {
          e.preventDefault()
          setTouched(true)
          if (!urlOk) return
          patch({ inboxId })
          navigate(webhookPaths(locked).events, { state: { back: state.back } })
        }}
      >
        <Field
          label={<Trans>URL</Trans>}
          htmlFor="webhook-url"
          error={shownProblem && <UrlProblem problem={shownProblem} />}
        >
          <Input
            id="webhook-url"
            type="url"
            autoFocus
            required
            maxLength={2000}
            className="h-10 rounded-xl font-mono text-caption"
            value={draft.url}
            placeholder="https://api.example.com/yuva/webhook"
            onChange={(e) => {
              setProblem(undefined)
              patch({ url: e.target.value })
            }}
            aria-invalid={!!shownProblem || undefined}
            data-testid="webhook-url-input"
          />
        </Field>
        <Field label={<Trans>Description (optional)</Trans>} htmlFor="webhook-description">
          <Input
            id="webhook-description"
            maxLength={200}
            className="h-10 rounded-xl"
            value={draft.description}
            placeholder={t`Push notifications`}
            onChange={(e) => patch({ description: e.target.value })}
          />
        </Field>
        {locked ? (
          lockedName && (
            <p className="text-body text-muted-foreground">
              <Trans>This endpoint receives only the events of {lockedName}.</Trans>
            </p>
          )
        ) : (
          <div className="flex flex-col gap-3">
            <ChoiceCards
              label={t`Inbox`}
              value={draft.scope}
              onValueChange={(v) => patch({ scope: v === "inbox" ? "inbox" : "all" })}
              items={[
                {
                  value: "all",
                  icon: <GlobeIcon />,
                  title: <Trans>All inboxes</Trans>,
                  hint: <Trans>Every inbox of the workspace, including ones you add later.</Trans>,
                  testId: "webhook-scope-all",
                },
                {
                  value: "inbox",
                  icon: <InboxIcon />,
                  title: <Trans>One inbox</Trans>,
                  hint: <Trans>The inbox cannot be changed later.</Trans>,
                  testId: "webhook-scope-inbox",
                },
              ]}
            />
            {draft.scope === "inbox" && (
              <Select value={inboxId} onValueChange={(v) => patch({ inboxId: String(v ?? "") })} items={inboxNames}>
                <SelectTrigger className="w-full" aria-label={t`Inbox`} data-testid="webhook-inbox">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {inboxes.map((i) => (
                    <SelectItem key={i.id} value={i.id}>
                      {i.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
        )}
        <StepActions testId="setup-webhook-continue" disabled={draft.scope === "inbox" && !inboxId}>
          <Trans>Continue</Trans>
        </StepActions>
      </form>
    </Layout>
  )
}

export function WebhookEventsStep() {
  const { canManage } = useSession()
  const locked = useLockedInbox()
  const navigate = useNavigate()
  const state = (useLocation().state ?? {}) as WebhookWizardState
  const inboxes = useInboxes().data ?? []
  const setEndpoint = useSetEndpoint()
  const [draft, patch, clear] = useWebhookDraft(locked)
  const [touched, setTouched] = useState(false)
  const [created, setCreated] = useState(false)
  const paths = webhookPaths(locked)
  const events = sortedEvents(draft.events)
  const create = useMutation({
    mutationFn: () => {
      const inboxId = locked ?? (draft.scope === "inbox" ? draft.inboxId : "")
      const body = {
        url: draft.url.trim(),
        description: draft.description.trim(),
        events,
        include_notes: draft.includeNotes && events.includes("message.created"),
        enabled: true,
        ...(inboxId ? { inbox_id: inboxId } : {}),
      }
      return unwrap(
        api.POST("/v1/webhooks", {
          body,
          params: { header: { "Idempotency-Key": idempotencyKey(draft.idempotencyKey, "webhook", body) } },
        })
      )
    },
    onSuccess: (r) => {
      setEndpoint(r.endpoint)
      const inbox = r.endpoint.inbox_id ? inboxes.find((i) => i.id === r.endpoint.inbox_id) : undefined
      setCreated(true)
      navigate(paths.done, {
        replace: true,
        state: {
          back: state.back,
          done: {
            endpointId: r.endpoint.id,
            url: r.endpoint.url,
            inboxName: inbox?.name ?? null,
            events: r.endpoint.events.length,
            secret: r.secret,
          },
        } satisfies WebhookWizardState,
      })
      clear()
    },
    onError: (err) => {
      const urlError = urlProblem(err)
      if (urlError) navigate(paths.endpoint, { replace: true, state: { back: state.back, urlError } })
    },
  })
  if (!canManage) return <Navigate to="/" replace />
  if (!draft.url.trim() && !created) return <Navigate to={paths.endpoint} replace state={{ back: state.back }} />
  return (
    <Layout stage="events" locked={locked} event={events[0] ?? "conversation.created"} onLeave={clear}>
      <StepHeader title={<Trans>Which events do you want?</Trans>}>
        <Trans>Pick what Yuva should tell your server about. You can change this later.</Trans>
      </StepHeader>
      <form
        className="flex flex-col gap-6"
        data-testid="setup-webhook-events"
        onSubmit={(e) => {
          e.preventDefault()
          setTouched(true)
          if (events.length > 0) create.mutate()
        }}
      >
        <div className="flex flex-col gap-2">
          <EventPicker
            events={draft.events}
            includeNotes={draft.includeNotes}
            onEvents={(next) => patch({ events: next })}
            onIncludeNotes={(on) => patch({ includeNotes: on })}
          />
          {touched && events.length === 0 && (
            <p role="alert" className="text-small text-destructive">
              <Trans>Pick at least one event.</Trans>
            </p>
          )}
        </div>
        <ErrorLine error={create.error} />
        <StepActions testId="setup-webhook-create" pending={create.isPending}>
          <Trans>Add endpoint</Trans>
        </StepActions>
      </form>
    </Layout>
  )
}

export function WebhookDoneStep() {
  const { canManage } = useSession()
  const locked = useLockedInbox()
  const navigate = useNavigate()
  const location = useLocation()
  const state = (location.state ?? {}) as WebhookWizardState
  const done = state.done
  const [secret] = useState(done?.secret ?? null)
  useEffect(() => {
    if (!done?.secret) return
    navigate(location.pathname + location.search, {
      replace: true,
      state: { ...state, done: { ...done, secret: undefined } } satisfies WebhookWizardState,
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
  if (!canManage) return <Navigate to="/" replace />
  if (!done) return <Navigate to="/settings/webhooks" replace />
  const back = { back: state.back }
  return (
    <Layout stage="done" locked={locked} event="conversation.created">
      <StepHeader icon={<PartyPopperIcon />} title={<Trans>The endpoint is ready</Trans>}>
        <Trans>Yuva now sends the events you picked to this address.</Trans>
      </StepHeader>
      <div className="flex flex-col gap-8" data-testid="setup-webhook-done">
        <SummaryList
          items={[
            [
              <Trans key="u">URL</Trans>,
              <span key="uv" className="font-mono">
                {done.url}
              </span>,
            ],
            [<Trans key="i">Inbox</Trans>, done.inboxName ?? <Trans key="a">All inboxes</Trans>],
            [
              <Trans key="e">Events</Trans>,
              done.events === 1 ? <Trans key="ev">1 event</Trans> : <Trans key="ev">{done.events} events</Trans>,
            ],
          ]}
        />
        {secret ? (
          <div className="flex flex-col gap-2" data-testid="setup-webhook-secret">
            <p className="text-body font-medium">
              <Trans>Signing secret</Trans>
            </p>
            <div className="flex items-center gap-2">
              <code className="min-w-0 flex-1 rounded-xl border bg-surface px-3 py-2 font-mono text-caption break-all">
                {secret}
              </code>
              <CopyButton value={secret} />
            </div>
            <p className="text-small text-faint">
              <Trans>Shown only now. Copy it and store it with your receiver.</Trans>
            </p>
          </div>
        ) : (
          <p className="text-body text-muted-foreground" data-testid="setup-webhook-no-secret">
            <Trans>The secret is not shown again. Rotate the secret on the endpoint page if you did not copy it.</Trans>
          </p>
        )}
        <SignatureHint />
        <div className="flex flex-wrap gap-2">
          <Button
            size="lg"
            render={<Link to={`/settings/webhooks/${done.endpointId}`} />}
            data-testid="setup-open-endpoint"
          >
            <Trans>Open the endpoint</Trans>
          </Button>
          <Button size="lg" variant="outline" render={<Link to={webhookPaths(locked).endpoint} state={back} />}>
            <Trans>Add another</Trans>
          </Button>
          <Button
            size="lg"
            variant="outline"
            onClick={() =>
              navigate(state.back ?? (locked ? `/settings/inboxes/${locked}/webhooks` : "/settings/webhooks"), {
                replace: true,
              })
            }
          >
            <Trans>Done</Trans>
          </Button>
        </div>
      </div>
    </Layout>
  )
}

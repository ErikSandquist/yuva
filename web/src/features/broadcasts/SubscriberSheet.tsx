import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation } from "@tanstack/react-query"
import { useState } from "react"
import { Link } from "react-router"

import { ErrorLine, toast } from "@/components/common"
import { formatDateTime, useEnumText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { useRefreshList, useSubscription } from "@/features/broadcasts/queries"
import { StatusTag } from "@/features/settings/ui"
import { api, unwrap, type ConsentEvent, type Subscription, type SubscriptionState } from "@/lib/api"
import { useSession } from "@/lib/session"
import { useMemberMap } from "@/lib/workspace"

export function StateTag({ state }: { state: SubscriptionState }) {
  const text = useEnumText()
  const tone = state === "subscribed" ? "success" : state === "pending" ? "warning" : "muted"
  return (
    <StatusTag tone={tone} data-testid="subscription-state">
      {text.subscription[state]}
    </StatusTag>
  )
}

function detailText(detail: ConsentEvent["detail"], key: string) {
  const v = detail[key]
  return typeof v === "string" && v ? v : undefined
}

type Row = [string, React.ReactNode]

function useConsentRows() {
  const { t, i18n } = useLingui()
  return (e: ConsentEvent, fallback?: string) => {
    const rows: Row[] = []
    const obtained = detailText(e.detail, "obtained_at") ?? fallback
    const reference = detailText(e.detail, "reference")
    if (obtained) rows.push([t`Agreed on`, formatDateTime(obtained, i18n.locale)])
    if (e.consent_text) rows.push([t`What they agreed to`, <q key="q">{e.consent_text}</q>])
    if (e.page_url)
      rows.push([
        t`Page`,
        <span key="p" dir="ltr" className="break-all">
          {e.page_url}
        </span>,
      ])
    if (reference) rows.push([t`Reference`, reference])
    if (e.ip)
      rows.push([
        t`IP address`,
        <span key="ip" dir="ltr" className="font-mono">
          {e.ip}
        </span>,
      ])
    if (e.user_agent)
      rows.push([
        t`Browser`,
        <span key="ua" dir="ltr" className="break-all">
          {e.user_agent}
        </span>,
      ])
    return rows
  }
}

function Facts({ rows }: { rows: Row[] }) {
  return (
    <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 rounded-xl border bg-surface px-3.5 py-3 text-small">
      {rows.map(([k, v]) => (
        <div key={k} className="contents">
          <dt className="text-faint">{k}</dt>
          <dd className="min-w-0 break-words">{v}</dd>
        </div>
      ))}
    </dl>
  )
}

function EventItem({ e, extra }: { e: ConsentEvent; extra: Row[] }) {
  const { t, i18n } = useLingui()
  const text = useEnumText()
  const members = useMemberMap()
  const member = e.member_id ? members.get(e.member_id) : undefined
  const by = member ? member.name || member.email : e.api_key_id ? t`An API key` : ""
  return (
    <li className="relative flex flex-col gap-1 ps-5 pb-4 last:pb-0" data-testid="consent-event">
      <span className="absolute -start-[4.5px] top-1.5 size-2 rounded-full bg-brand" aria-hidden />
      <span className="flex flex-wrap items-baseline gap-x-2">
        <span className="text-body font-medium">{text.consentEvent[e.kind]}</span>
        <time className="text-caption text-faint" dateTime={e.created_at}>
          {formatDateTime(e.created_at, i18n.locale)}
        </time>
      </span>
      <span className="text-caption text-faint" data-testid="consent-source">
        {text.consentSource[e.source] ?? e.source}
        {by && ` · ${by}`}
      </span>
      {extra.length > 0 && (
        <details className="text-small">
          <summary className="w-fit cursor-pointer text-muted-foreground hover:text-foreground">
            <Trans>Details</Trans>
          </summary>
          <dl className="mt-1 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1">
            {extra.map(([k, v]) => (
              <div key={k} className="contents">
                <dt className="text-faint">{k}</dt>
                <dd className="min-w-0 break-words">{v}</dd>
              </div>
            ))}
          </dl>
        </details>
      )}
    </li>
  )
}

export function SubscriberSheet({
  listId,
  subscriptionId,
  row,
  names,
  onClose,
}: {
  listId: string
  subscriptionId: string | null
  row?: Subscription
  names?: Map<string, string>
  onClose: () => void
}) {
  const { t, i18n } = useLingui()
  const text = useEnumText()
  const { canManage } = useSession()
  const refresh = useRefreshList(listId)
  const consentRows = useConsentRows()
  const [asking, setAsking] = useState<"unsubscribe" | "delete" | null>(null)
  const sub = useSubscription(listId, subscriptionId, canManage)
  const path = { params: { path: { listId, subscriptionId: subscriptionId ?? "" } } }
  const resend = useMutation({
    mutationFn: () => unwrap(api.POST("/v1/lists/{listId}/subscriptions/{subscriptionId}/confirmation", path)),
    onSuccess: () => {
      toast(t`The confirmation mail is on its way.`)
      refresh()
    },
  })
  const unsubscribe = useMutation({
    mutationFn: () =>
      unwrap(
        api.PATCH("/v1/lists/{listId}/subscriptions/{subscriptionId}", { ...path, body: { state: "unsubscribed" } })
      ),
    onSuccess: () => {
      toast(t`Unsubscribed`)
      setAsking(null)
      refresh()
    },
  })
  const remove = useMutation({
    mutationFn: () => unwrap(api.DELETE("/v1/lists/{listId}/subscriptions/{subscriptionId}", path)),
    onSuccess: () => {
      toast(t`Subscription deleted`)
      refresh()
      onClose()
    },
  })
  const reset = () => {
    ;[resend, unsubscribe, remove].forEach((m) => m.reset())
    setAsking(null)
  }
  const s: (Subscription & { events?: ConsentEvent[] }) | undefined = canManage ? sub.data : row
  const events = s?.events ?? []
  const proof = events.find((e) => consentRows(e).length > 0)
  const facts = proof ? consentRows(proof, s?.consent_at) : []
  return (
    <Sheet
      open={!!subscriptionId}
      onOpenChange={(o) => {
        if (!o) {
          reset()
          onClose()
        }
      }}
    >
      <SheetContent side="right" className="w-full sm:w-[30rem]" data-testid="subscriber-sheet">
        <SheetHeader>
          <SheetTitle className="pe-8 break-all">{s?.email ?? "…"}</SheetTitle>
          <SheetDescription>
            {s?.contact.name ? (
              <Link to={`/contacts/${s.contact.id}`} className="text-brand underline-offset-4 hover:underline">
                {s.contact.name}
              </Link>
            ) : (
              s && (
                <Link to={`/contacts/${s.contact.id}`} className="text-brand underline-offset-4 hover:underline">
                  <Trans>Open the contact</Trans>
                </Link>
              )
            )}
          </SheetDescription>
        </SheetHeader>
        {canManage && sub.isPending ? (
          <div className="flex flex-col gap-3 p-5">
            <Skeleton className="h-6 w-40" />
            <Skeleton className="h-40 w-full" />
          </div>
        ) : !s ? (
          <ErrorLine error={sub.error} className="p-5" />
        ) : (
          <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto p-5">
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <StateTag state={s.state} />
              <span className="text-caption text-muted-foreground">{text.subscriptionSource[s.source]}</span>
            </div>
            {names && (
              <p className="text-body" data-testid="subscriber-sheet-components">
                <span className="text-muted-foreground">
                  <Trans>Components</Trans>
                </span>{" "}
                ·{" "}
                {s.components?.length ? (
                  s.components.map((c) => names.get(c) ?? "…").join(", ")
                ) : (
                  <Trans>All components</Trans>
                )}
              </p>
            )}
            {s.state === "unsubscribed" && s.unsubscribe_reason && (
              <p className="text-body text-muted-foreground" data-testid="unsubscribe-reason">
                <Trans>How they left:</Trans> {text.unsubscribeReason[s.unsubscribe_reason]}
              </p>
            )}
            {s.events && (
              <section className="flex flex-col gap-2" data-testid="consent-record">
                <h3 className="text-body font-medium">
                  <Trans>Consent record</Trans>
                </h3>
                {facts.length > 0 ? (
                  <Facts rows={facts} />
                ) : (
                  <p className="text-body text-muted-foreground">
                    <Trans>No events recorded.</Trans>
                  </p>
                )}
              </section>
            )}
            {events.length > 0 && (
              <section className="flex flex-col gap-3">
                <h3 className="text-body font-medium">
                  <Trans>What happened</Trans>
                </h3>
                <ol className="border-s ps-0" data-testid="consent-events">
                  {events.map((e) => (
                    <EventItem key={e.id} e={e} extra={e === proof ? [] : consentRows(e)} />
                  ))}
                </ol>
              </section>
            )}
            {!s.events && (
              <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-body">
                {[
                  [t`Added`, s.created_at],
                  [t`Agreed`, s.consent_at],
                  [t`Confirmed`, s.confirmed_at],
                  [t`Unsubscribed`, s.unsubscribed_at],
                ]
                  .filter((d): d is [string, string] => !!d[1])
                  .map(([label, at]) => (
                    <div key={label} className="contents">
                      <dt className="text-muted-foreground">{label}</dt>
                      <dd>{formatDateTime(at, i18n.locale)}</dd>
                    </div>
                  ))}
              </dl>
            )}
            <ErrorLine error={resend.error ?? unsubscribe.error ?? remove.error} className="text-small" />
            {canManage && asking && (
              <div
                className="flex flex-col gap-3 rounded-xl border border-destructive/30 bg-destructive/5 p-3.5"
                data-testid="subscriber-confirm"
              >
                <p className="font-medium">
                  {asking === "delete" ? (
                    <Trans>Delete this subscription?</Trans>
                  ) : (
                    <Trans>Unsubscribe {s.email}?</Trans>
                  )}
                </p>
                <p className="text-small text-muted-foreground">
                  {asking === "unsubscribe" ? (
                    <Trans>
                      They get no more mail from this list, and it is recorded that the team unsubscribed them. Only
                      they can subscribe again, by confirming a mail.
                    </Trans>
                  ) : s.state === "unsubscribed" ? (
                    <Trans>
                      Its consent record is deleted with it. Because they unsubscribed, the address is remembered as
                      opted out, so nobody on the team can add it again.
                    </Trans>
                  ) : (
                    <Trans>
                      Its consent record is deleted with it, so you can no longer show how they agreed. To stop mailing
                      them, unsubscribe them instead.
                    </Trans>
                  )}
                </p>
                <div className="flex flex-wrap justify-end gap-2">
                  <Button variant="ghost" size="sm" onClick={() => setAsking(null)}>
                    <Trans>Cancel</Trans>
                  </Button>
                  <Button
                    variant="destructive"
                    size="sm"
                    disabled={unsubscribe.isPending || remove.isPending}
                    onClick={() => (asking === "delete" ? remove.mutate() : unsubscribe.mutate())}
                    data-testid="subscriber-confirm-go"
                  >
                    {asking === "delete" ? <Trans>Delete</Trans> : <Trans>Unsubscribe</Trans>}
                  </Button>
                </div>
              </div>
            )}
            {canManage && !asking && (
              <div className="mt-auto flex flex-wrap items-center gap-2 border-t pt-4">
                {s.state === "pending" && (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => resend.mutate()}
                    disabled={resend.isPending}
                    data-testid="resend-confirmation"
                  >
                    <Trans>Send the confirmation again</Trans>
                  </Button>
                )}
                {s.state !== "unsubscribed" && (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setAsking("unsubscribe")}
                    data-testid="unsubscribe"
                  >
                    <Trans>Unsubscribe</Trans>
                  </Button>
                )}
                <Button
                  variant="ghost"
                  size="sm"
                  className="ms-auto text-destructive hover:text-destructive"
                  onClick={() => setAsking("delete")}
                  data-testid="delete-subscription"
                >
                  <Trans>Delete subscription</Trans>
                </Button>
              </div>
            )}
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}

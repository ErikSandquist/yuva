import { Trans, useLingui } from "@lingui/react/macro"
import { Link, useLocation, useNavigate } from "react-router"

import { ErrorLine } from "@/components/common"
import { formatRelative } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Card, EmptyRow, LinkRow, Rows, Section } from "@/features/settings/ui"
import { EndpointStatus } from "@/features/settings/webhooks/parts"
import { useWebhooks } from "@/features/settings/webhooks/queries"
import { SignatureHint } from "@/features/settings/webhooks/SignatureHint"
import { useSession } from "@/lib/session"
import type { WebhookEndpoint } from "@/lib/api"
import { cn } from "@/lib/utils"
import { useInboxes } from "@/lib/workspace"

function Dot({ e }: { e: WebhookEndpoint }) {
  return (
    <span
      aria-hidden
      className={cn(
        "size-2.5 shrink-0 rounded-full",
        !e.enabled ? (e.disabled_reason ? "bg-destructive" : "bg-input") : e.failing_since ? "bg-warning" : "bg-success"
      )}
    />
  )
}

function StatusText({ e }: { e: WebhookEndpoint }) {
  const { i18n } = useLingui()
  const since = e.failing_since ? formatRelative(e.failing_since, i18n.locale) : ""
  const [status, text] =
    !e.enabled && e.disabled_reason
      ? ["disabled", <Trans key="d">Turned off by Yuva</Trans>]
      : !e.enabled
        ? ["off", <Trans key="o">Off</Trans>]
        : e.failing_since
          ? ["failing", <Trans key="f">Failing since {since} · retrying</Trans>]
          : ["on", <Trans key="n">Delivering</Trans>]
  return (
    <span
      className={cn(
        "w-56 shrink-0 truncate text-caption text-muted-foreground",
        status === "disabled" && "text-destructive"
      )}
      data-testid="webhook-status"
      data-status={status}
    >
      {text}
    </span>
  )
}

export function WebhookTable() {
  const { canManage } = useSession()
  const inboxes = useInboxes().data ?? []
  const list = useWebhooks(undefined)
  if (!canManage) return null
  return (
    <>
      <section className="overflow-hidden rounded-xl border bg-card">
        {list.isPending ? (
          <Skeleton className="m-4 h-10" />
        ) : list.data && list.data.length === 0 ? (
          <EmptyRow>
            <Trans>No endpoints yet.</Trans>
          </EmptyRow>
        ) : (
          <ul>
            {(list.data ?? []).map((e) => {
              const inbox = e.inbox_id ? inboxes.find((i) => i.id === e.inbox_id) : undefined
              return (
                <li key={e.id} className="flex items-center gap-3 border-b px-4 py-[11px] last:border-b-0">
                  <Dot e={e} />
                  <Link
                    to={`/settings/webhooks/${e.id}`}
                    className="flex min-w-0 flex-1 flex-col leading-[1.3] outline-none focus-visible:underline"
                    data-testid="webhook-row"
                  >
                    <span className="truncate font-mono text-[13px]" dir="ltr">
                      {e.url}
                    </span>
                    <span className="truncate text-caption text-faint">
                      {inbox && <>{inbox.name} · </>}
                      {e.events.join(", ")}
                    </span>
                  </Link>
                  <StatusText e={e} />
                  <Button
                    variant="outline"
                    className="h-[30px] rounded-lg border-input px-2.5 font-normal"
                    render={<Link to={`/settings/webhooks/${e.id}`} />}
                  >
                    <Trans>Deliveries</Trans>
                  </Button>
                </li>
              )
            })}
          </ul>
        )}
        <ErrorLine error={list.error} className="px-4 pb-4" />
      </section>
      <SignatureHint />
    </>
  )
}

export function WebhookList({ inboxId }: { inboxId?: string }) {
  const { canManage } = useSession()
  const inboxes = useInboxes().data ?? []
  const list = useWebhooks(inboxId)
  const navigate = useNavigate()
  const location = useLocation()
  if (!canManage) return null
  return (
    <Section
      title={inboxId ? <Trans>Webhooks</Trans> : <Trans>Endpoints</Trans>}
      description={
        inboxId ? (
          <Trans>Signed HTTP calls to your backend for this inbox's conversations, messages and feedback.</Trans>
        ) : (
          <Trans>
            Signed HTTP calls to your backend when conversations, messages, feedback or contacts change. Endpoints of
            the workspace receive every inbox; inbox endpoints only their inbox.
          </Trans>
        )
      }
      action={
        <Button
          variant="outline"
          size="sm"
          onClick={() =>
            navigate(inboxId ? `/setup/webhook?inbox=${inboxId}` : "/setup/webhook", {
              state: { back: location.pathname },
            })
          }
          data-testid="webhook-add"
        >
          <Trans>Add endpoint</Trans>
        </Button>
      }
    >
      <Card flush>
        {list.isPending ? (
          <Skeleton className="m-5 h-10" />
        ) : list.data && list.data.length === 0 ? (
          <EmptyRow>
            <Trans>No endpoints yet.</Trans>
          </EmptyRow>
        ) : (
          <Rows>
            {(list.data ?? []).map((e) => {
              const inbox = e.inbox_id ? inboxes.find((i) => i.id === e.inbox_id) : undefined
              const count = e.events.length
              return (
                <LinkRow key={e.id} to={`/settings/webhooks/${e.id}`} testId="webhook-row">
                  <div className="min-w-0 flex-1">
                    <p className="truncate font-mono text-caption font-medium">{e.url}</p>
                    <p className="truncate text-caption text-muted-foreground">
                      {e.description && <>{e.description} · </>}
                      {!inboxId && (
                        <>
                          {inbox ? inbox.name : <Trans>All inboxes</Trans>}
                          {" · "}
                        </>
                      )}
                      {count === 1 ? <Trans>1 event</Trans> : <Trans>{count} events</Trans>}
                    </p>
                  </div>
                  <EndpointStatus e={e} />
                </LinkRow>
              )
            })}
          </Rows>
        )}
        <ErrorLine error={list.error} className="px-5 pb-4" />
      </Card>
      <SignatureHint />
    </Section>
  )
}

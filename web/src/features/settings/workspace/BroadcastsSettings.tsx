import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"

import { ErrorLine } from "@/components/common"
import { formatDateTime } from "@/components/common/text"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { useRequestApproval } from "@/features/broadcasts/broadcastQueries"
import { StatusNotice, useNumber } from "@/features/broadcasts/BroadcastParts"
import { Card, Field, FormActions, FormCard, Section, StatusTag, ToggleRow } from "@/features/settings/ui"
import { api, unwrap, type BroadcastsStatus, type WorkspaceBroadcasts } from "@/lib/api"
import { keys } from "@/lib/keys"
import { meKey, useSession } from "@/lib/session"

function useStatusText(): Record<BroadcastsStatus, string> {
  const { t } = useLingui()
  return {
    allowed: t`Allowed`,
    approval_required: t`Needs the operator’s approval`,
    approval_requested: t`Waiting for the operator`,
    suspended: t`Suspended`,
  }
}

function ApprovalForm() {
  const { t } = useLingui()
  const [website, setWebsite] = useState("")
  const [sender, setSender] = useState("")
  const [consent, setConsent] = useState("")
  const request = useRequestApproval()
  return (
    <FormCard
      onSubmit={() =>
        request.mutate({
          website: website.trim(),
          sender_description: sender.trim(),
          consent_description: consent.trim(),
        })
      }
      footer={
        <FormActions
          pending={request.isPending}
          error={request.error}
          disabled={!website.trim() || !sender.trim() || !consent.trim()}
          label={<Trans>Ask for approval</Trans>}
        />
      }
    >
      <p className="text-body text-muted-foreground">
        <Trans>
          The server’s operator reads this before letting the workspace send. New workspaces start with a small daily
          limit that grows with every clean day.
        </Trans>
      </p>
      <Field label={<Trans>Your website</Trans>} htmlFor="approval-website">
        <Input
          id="approval-website"
          type="url"
          required
          maxLength={2000}
          placeholder="https://example.com"
          value={website}
          onChange={(e) => setWebsite(e.target.value)}
          data-testid="approval-website"
        />
      </Field>
      <Field
        label={<Trans>Who sends, and what about?</Trans>}
        htmlFor="approval-sender"
        hint={<Trans>For example: a monthly product newsletter from our company to our customers.</Trans>}
      >
        <Textarea
          id="approval-sender"
          required
          rows={3}
          maxLength={2000}
          value={sender}
          onChange={(e) => setSender(e.target.value)}
          data-testid="approval-sender"
        />
      </Field>
      <Field
        label={<Trans>How did your subscribers agree?</Trans>}
        htmlFor="approval-consent"
        hint={<Trans>For example: a subscribe form with confirmation on our site, or a checkbox at sign-up.</Trans>}
      >
        <Textarea
          id="approval-consent"
          required
          rows={3}
          maxLength={2000}
          placeholder={t`They signed up on our website and confirmed their address.`}
          value={consent}
          onChange={(e) => setConsent(e.target.value)}
          data-testid="approval-consent"
        />
      </Field>
    </FormCard>
  )
}

function KeysSwitch({ b }: { b: WorkspaceBroadcasts }) {
  const { t } = useLingui()
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  const save = useMutation({
    mutationFn: (on: boolean) => unwrap(api.PATCH("/v1/workspace", { body: { broadcasts_by_keys: on } })),
    onSuccess: (w) => {
      qc.setQueryData(keys.workspace(ws), w)
      void qc.invalidateQueries({ queryKey: meKey })
    },
  })
  const on = save.isPending ? save.variables : b.by_keys
  return (
    <Card>
      <ToggleRow
        title={<Trans>Keys and assistants may send broadcasts</Trans>}
        hint={
          <Trans>
            When off, API keys and connected assistants can write drafts and send tests, and a member schedules them.
            When on, a key or assistant with the “Schedule and send broadcasts” permission can schedule and resume them.
          </Trans>
        }
      >
        <Switch
          checked={on}
          disabled={save.isPending}
          onCheckedChange={(v) => save.mutate(v)}
          aria-label={t`Keys and assistants may send broadcasts`}
          data-testid="broadcasts-by-keys"
        />
      </ToggleRow>
      <ErrorLine error={save.error} />
    </Card>
  )
}

export function BroadcastsSettings() {
  const { i18n } = useLingui()
  const num = useNumber()
  const statusText = useStatusText()
  const { workspaceId: ws, membership, canManage } = useSession()
  const workspace = useQuery({ queryKey: keys.workspace(ws), queryFn: () => unwrap(api.GET("/v1/workspace")) })
  const b = workspace.data?.broadcasts ?? membership.workspace.broadcasts
  if (!b) return null
  const owner = membership.role === "owner"
  const sent = num(b.sent_today)
  const limit = b.daily_limit !== undefined ? num(b.daily_limit) : null
  const since = b.status_at ? formatDateTime(b.status_at, i18n.locale) : null
  const tone = b.status === "allowed" ? "success" : b.status === "suspended" ? "danger" : "warning"
  return (
    <Section title={<Trans>Broadcasts</Trans>}>
      {b.mode === "off" ? (
        <StatusNotice settingsLink={false} />
      ) : (
        <>
          <Card data-testid="broadcasts-settings">
            <div className="flex flex-col gap-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="text-body font-medium">
                  <Trans>Sending</Trans>
                </span>
                <StatusTag tone={tone} data-testid="broadcasts-status">
                  {statusText[b.status]}
                </StatusTag>
              </div>
              {b.reason && b.status !== "suspended" && <p className="text-body text-muted-foreground">{b.reason}</p>}
              {since && b.status !== "allowed" && (
                <p className="text-small text-faint">
                  <Trans>Since {since}</Trans>
                </p>
              )}
              <p className="text-body text-muted-foreground tabular-nums" data-testid="broadcasts-today">
                {limit ? (
                  <Trans>
                    Sent today: {sent} of {limit} allowed. The day starts at 00:00 UTC.
                  </Trans>
                ) : (
                  <Trans>Sent today: {sent}. There is no daily limit.</Trans>
                )}
              </p>
            </div>
          </Card>
          {b.status !== "allowed" && <StatusNotice settingsLink={false} />}
          {b.status === "approval_required" && owner && <ApprovalForm />}
          {canManage && <KeysSwitch b={b} />}
        </>
      )}
    </Section>
  )
}

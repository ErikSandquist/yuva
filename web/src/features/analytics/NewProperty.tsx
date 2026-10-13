import { Trans, useLingui } from "@lingui/react/macro"
import { CheckCircle2Icon } from "lucide-react"
import { useMemo, useState } from "react"
import { Link, Navigate, useNavigate, useParams } from "react-router"

import { ErrorLine, Notice } from "@/components/common"
import { Pane, PaneBack } from "@/components/common/Column"
import { formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { Textarea } from "@/components/ui/textarea"
import { InstallSnippets } from "@/features/analytics/Install"
import { useCreateProperty, useProperty, useWaitForFirstEvent } from "@/features/analytics/queries"
import { ChoiceSelect, Field, FormCard, PageHeader, Section } from "@/features/settings/ui"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"
import { timeZones } from "@/lib/utils"

const NO_INBOX = "none"

export function splitLines(s: string) {
  return s
    .split(/[\s,]+/)
    .map((x) => x.trim())
    .filter(Boolean)
}

export function hostOnly(s: string) {
  return s
    .replace(/^[a-z]+:\/\//i, "")
    .replace(/[/:].*$/, "")
    .toLowerCase()
}

export function useZones() {
  return useMemo(() => timeZones(), [])
}

export function NewPropertyPage() {
  const { t } = useLingui()
  const { canManage } = useSession()
  const navigate = useNavigate()
  const inboxes = useInboxes().data ?? []
  const zones = useZones()
  const [name, setName] = useState("")
  const [domains, setDomains] = useState("")
  const [inbox, setInbox] = useState(NO_INBOX)
  const [zone, setZone] = useState(() => Intl.DateTimeFormat().resolvedOptions().timeZone)
  const create = useCreateProperty()
  if (!canManage) return <Navigate to="/analytics" replace />
  const hosts = splitLines(domains).map(hostOnly).filter(Boolean)
  return (
    <Pane testId="analytics-new">
      <PageHeader
        back={{ to: "/analytics", label: t`Analytics` }}
        title={<Trans>Add a website</Trans>}
        description={
          <Trans>
            Count visitors, pages, sources, places and devices without cookies and without storing IP addresses.
          </Trans>
        }
      />
      <FormCard
        onSubmit={() =>
          create.mutate(
            {
              name: name.trim(),
              domains: hosts,
              inbox_id: inbox === NO_INBOX ? undefined : inbox,
              timezone: zones.includes(zone) ? zone : "UTC",
            },
            { onSuccess: (p) => navigate(`/analytics/new/${p.id}`, { replace: true }) }
          )
        }
        footer={
          <>
            <ErrorLine error={create.error} className="min-w-0 flex-1" />
            <Button
              type="submit"
              className="ms-auto"
              disabled={!name.trim() || hosts.length === 0 || create.isPending}
              data-testid="analytics-create"
            >
              <Trans>Add and install</Trans>
            </Button>
          </>
        }
      >
        <Field label={<Trans>Name</Trans>} htmlFor="analytics-name">
          <Input
            id="analytics-name"
            autoFocus
            value={name}
            maxLength={100}
            onChange={(e) => setName(e.target.value)}
            placeholder={t`For example Fieldnote website`}
            data-testid="analytics-name"
          />
        </Field>
        <Field
          label={<Trans>Domains</Trans>}
          htmlFor="analytics-domains"
          hint={
            <Trans>
              One per line. Pages on other domains cannot send events. *.example.com covers every subdomain.
            </Trans>
          }
        >
          <Textarea
            id="analytics-domains"
            dir="ltr"
            rows={3}
            value={domains}
            onChange={(e) => setDomains(e.target.value)}
            placeholder="www.example.com"
            data-testid="analytics-domains"
          />
        </Field>
        <div className="grid gap-5 sm:grid-cols-2">
          <Field
            label={<Trans>Inbox</Trans>}
            htmlFor="analytics-inbox"
            hint={<Trans>Agents of this inbox can see the numbers.</Trans>}
          >
            <ChoiceSelect
              id="analytics-inbox"
              label={t`Inbox`}
              value={inbox}
              onChange={setInbox}
              options={[[NO_INBOX, t`None, owners and admins only`], ...inboxes.map((i) => [i.id, i.name] as const)]}
              className="w-full"
            />
          </Field>
          <Field
            label={<Trans>Time zone</Trans>}
            htmlFor="analytics-zone"
            hint={<Trans>Where the property’s day starts and ends.</Trans>}
          >
            <Input id="analytics-zone" list="analytics-zones" value={zone} onChange={(e) => setZone(e.target.value)} />
            <datalist id="analytics-zones">
              {zones.map((z) => (
                <option key={z} value={z} />
              ))}
            </datalist>
          </Field>
        </div>
      </FormCard>
    </Pane>
  )
}

export function WaitForFirstEvent({ propertyId }: { propertyId: string }) {
  const { i18n } = useLingui()
  const known = useProperty(propertyId).data?.first_event_at
  const wait = useWaitForFirstEvent(propertyId, !known)
  const first = known ?? wait.data?.first_event_at
  if (first) {
    const at = formatDateTime(first, i18n.locale)
    return (
      <Notice icon={CheckCircle2Icon} className="bg-success/8 [&>svg]:text-success" data-testid="first-event">
        <p className="font-medium text-foreground">
          <Trans>The first event arrived at {at}.</Trans>
        </p>
        <div className="flex flex-wrap gap-2">
          <Button size="sm" render={<Link to={`/analytics/${propertyId}/realtime`} />}>
            <Trans>Watch it live</Trans>
          </Button>
          <Button size="sm" variant="outline" render={<Link to={`/analytics/${propertyId}`} />}>
            <Trans>Open the overview</Trans>
          </Button>
        </div>
      </Notice>
    )
  }
  return (
    <Notice data-testid="waiting-first-event">
      <p className="flex items-center gap-2 font-medium text-foreground">
        <Spinner className="size-4" />
        <Trans>Waiting for the first event</Trans>
      </p>
      <p>
        <Trans>
          Open a page of your site with the snippet in place. This turns green as soon as the server has stored the
          first page view. Browsers with Do Not Track or Global Privacy Control send nothing, so use another one to
          test.
        </Trans>
      </p>
      <ErrorLine error={wait.error} />
    </Notice>
  )
}

export function InstallPropertyPage() {
  const { t } = useLingui()
  const { propertyId = "" } = useParams()
  const property = useProperty(propertyId)
  const p = property.data
  return (
    <Pane testId="analytics-install-page">
      <PaneBack to="/analytics" label={t`Analytics`} always />
      {!p ? (
        property.error ? (
          <ErrorLine error={property.error} />
        ) : (
          <Skeleton className="h-48 rounded-2xl" />
        )
      ) : (
        <>
          <PageHeader
            back={false}
            title={<Trans>Install on {p.name}</Trans>}
            description={
              <Trans>
                Pick how the site is built and copy the snippet. The key is public, like a chat key: it only works on
                the property’s domains.
              </Trans>
            }
          />
          <Section>
            <div className="flex flex-col gap-5 rounded-2xl border bg-card p-4">
              <InstallSnippets property={p} />
            </div>
          </Section>
          <WaitForFirstEvent propertyId={p.id} />
        </>
      )}
    </Pane>
  )
}

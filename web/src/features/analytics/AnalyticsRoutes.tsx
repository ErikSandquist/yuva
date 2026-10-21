import "@/features/analytics/i18n"

import { Trans, useLingui } from "@lingui/react/macro"
import { ChartLineIcon, ChevronDownIcon, PlusIcon, Share2Icon } from "lucide-react"
import { Link, Navigate, Outlet, Route, Routes, useLocation, useNavigate, useParams } from "react-router"

import { Dot, ErrorLine } from "@/components/common"
import { Pane, PaneBack } from "@/components/common/Column"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { UsageBanner } from "@/features/analytics/AnalyticsColumn"
import { BreakdownView } from "@/features/analytics/Breakdown"
import { Controls } from "@/features/analytics/Controls"
import { EventsView } from "@/features/analytics/Events"
import { FunnelsView } from "@/features/analytics/Funnels"
import { GoalsView } from "@/features/analytics/Goals"
import { WaitForFirstEvent } from "@/features/analytics/NewProperty"
import { Overview } from "@/features/analytics/Overview"
import { useAnalyticsProperties } from "@/features/analytics/properties"
import { useProperty } from "@/features/analytics/queries"
import { Realtime } from "@/features/analytics/Realtime"
import { RetentionView } from "@/features/analytics/Retention"
import { SettingsView } from "@/features/analytics/Settings"
import { SupportView } from "@/features/analytics/Support"
import { useViewSearch } from "@/features/analytics/state"
import { useAnalyticsText } from "@/features/analytics/text"
import { useIsPhone } from "@/hooks/use-media-query"
import { PageHeader, PageTabs, pageTabClass } from "@/features/settings/ui"
import type { SetupState } from "@/features/setup/setup"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

const VIEWS = [
  "overview",
  "realtime",
  "pages",
  "sources",
  "locations",
  "devices",
  "events",
  "goals",
  "funnels",
  "retention",
  "support",
  "settings",
] as const
type ViewKey = (typeof VIEWS)[number]

const PRIMARY: ViewKey[] = ["overview", "support", "goals", "realtime"]

function AnalyticsHome() {
  const { canManage } = useSession()
  const phone = useIsPhone()
  const list = useAnalyticsProperties()
  const first = list.data?.items[0]
  if (first && !phone) return <Navigate to={`/analytics/${first.id}`} replace />
  return (
    <Pane wide testId="analytics-home" className="gap-6">
      <PageHeader
        back={false}
        title={<Trans>Analytics</Trans>}
        description={<Trans>Cookieless visitors across your sites and apps.</Trans>}
      />
      <div className="flex flex-col items-center gap-2 rounded-2xl border bg-card px-6 py-12 text-center">
        <span className="mb-2 grid size-12 place-items-center rounded-2xl bg-brand-wash text-brand">
          <ChartLineIcon className="size-6" />
        </span>
        <p className="text-title">
          <Trans>Count your website’s visitors</Trans>
        </p>
        <p className="max-w-md text-body text-muted-foreground">
          <Trans>
            Pages, sources, places and devices, without cookies, without storing IP addresses and without a consent
            banner where the law allows it.
          </Trans>
        </p>
        {canManage && (
          <Button className="mt-3" render={<Link to="/analytics/new" />} data-testid="analytics-home-add">
            <PlusIcon />
            <Trans>Add a website</Trans>
          </Button>
        )}
      </div>
    </Pane>
  )
}

function PropertyLayout() {
  const { t } = useLingui()
  const text = useAnalyticsText()
  const { propertyId = "" } = useParams()
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const { canManage } = useSession()
  const search = useViewSearch(true)
  const property = useProperty(propertyId)
  const usage = useAnalyticsProperties().data?.usage
  const inbox = (useInboxes().data ?? []).find((i) => i.id === property.data?.inbox_id)
  const last = pathname.split("/")[3] as ViewKey | undefined
  const current: ViewKey = last && VIEWS.includes(last) ? last : "overview"
  const names: Record<ViewKey, string> = {
    overview: t`Overview`,
    realtime: t`Realtime`,
    pages: text.title.pages,
    sources: t`Sources`,
    locations: t`Locations`,
    devices: t`Devices`,
    events: t`Events`,
    goals: t`Goals`,
    funnels: t`Funnels`,
    retention: t`Retention`,
    support: t`Support`,
    settings: t`Settings`,
  }
  const p = property.data
  if (!p) {
    return (
      <Pane wide>
        <PaneBack to="/analytics" label={t`Analytics`} />
        {property.error ? <ErrorLine error={property.error} /> : <Skeleton className="h-64 rounded-2xl" />}
      </Pane>
    )
  }
  const controls = current !== "realtime" && current !== "settings" && current !== "retention"
  const to = (v: ViewKey) => `/analytics/${p.id}${v === "overview" ? "" : `/${v}`}${search}`
  const more = VIEWS.filter((v) => !PRIMARY.includes(v))
  const inMore = more.includes(current)
  return (
    <Pane wide testId="analytics-property" className="gap-5" key={p.id}>
      <div className="flex flex-col">
        <PaneBack to="/analytics" label={t`Analytics`} />
        <PageHeader
          back={false}
          title={<span data-testid="analytics-property-name">{p.name}</span>}
          description={
            <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
              {inbox && (
                <span className="inline-flex items-center gap-1.5">
                  <Dot color={inbox.branding.color} className="size-[7px] rounded-[2px]" />
                  {inbox.name}
                </span>
              )}
              <span className="min-w-0 truncate" dir="ltr">
                {p.domains.join(", ")}
              </span>
              <span aria-hidden>·</span>
              <span title={t`Days are counted in this time zone`}>{p.timezone}</span>
            </span>
          }
          action={
            canManage && (
              <Button
                variant="outline"
                size="sm"
                render={<Link to={`/analytics/${p.id}/settings?tab=sharing`} />}
                data-testid="analytics-share"
              >
                <Share2Icon />
                <Trans>Share</Trans>
              </Button>
            )
          }
          tabs={
            <PageTabs
              label={t`Analytics views`}
              current={current}
              items={PRIMARY.map((v) => ({ key: v, to: to(v), label: names[v], testId: `analytics-tab-${v}` }))}
              extra={
                <DropdownMenu>
                  <DropdownMenuTrigger
                    className={pageTabClass}
                    aria-current={inMore ? "page" : undefined}
                    data-testid="analytics-tab-more"
                  >
                    {inMore ? names[current] : <Trans>More</Trans>}
                    <ChevronDownIcon className="size-3.5" />
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="start" className="w-48">
                    {more.map((v) => (
                      <DropdownMenuItem
                        key={v}
                        onClick={() => navigate(to(v))}
                        aria-current={v === current ? "page" : undefined}
                        className="aria-[current=page]:font-medium"
                        data-testid={`analytics-tab-${v}`}
                      >
                        {names[v]}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              }
            />
          }
        />
      </div>
      {usage && <UsageBanner usage={usage} />}
      {!p.first_event_at && current !== "settings" && <WaitForFirstEvent propertyId={p.id} />}
      {controls && (
        <Controls
          propertyId={p.id}
          compare={current !== "events" && current !== "funnels"}
          filters={current !== "funnels"}
        />
      )}
      <Outlet context={{ property: p }} />
    </Pane>
  )
}

function ToSetup() {
  const { propertyId } = useParams()
  return (
    <Navigate
      to={propertyId ? `/setup/analytics/${propertyId}` : "/setup/analytics"}
      state={{ back: "/analytics" } satisfies SetupState}
      replace
    />
  )
}

export default function AnalyticsRoutes() {
  return (
    <Routes>
      <Route index element={<AnalyticsHome />} />
      <Route path="new" element={<ToSetup />} />
      <Route path="new/:propertyId" element={<ToSetup />} />
      <Route path=":propertyId" element={<PropertyLayout />}>
        <Route index element={<Overview />} />
        <Route path="realtime" element={<Realtime />} />
        <Route path="pages" element={<BreakdownView group="pages" />} />
        <Route path="sources" element={<BreakdownView group="sources" />} />
        <Route path="locations" element={<BreakdownView group="locations" />} />
        <Route path="devices" element={<BreakdownView group="devices" />} />
        <Route path="events" element={<EventsView />} />
        <Route path="goals" element={<GoalsView />} />
        <Route path="funnels" element={<FunnelsView />} />
        <Route path="retention" element={<RetentionView />} />
        <Route path="support" element={<SupportView />} />
        <Route path="settings" element={<SettingsView />} />
        <Route path="*" element={<Navigate to=".." replace />} />
      </Route>
    </Routes>
  )
}

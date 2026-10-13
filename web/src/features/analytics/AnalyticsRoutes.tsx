import "@/features/analytics/i18n"

import { Trans, useLingui } from "@lingui/react/macro"
import { ChartLineIcon, PlusIcon } from "lucide-react"
import { Link, Navigate, Outlet, Route, Routes, useLocation, useNavigate, useParams } from "react-router"

import { Dot, ErrorLine } from "@/components/common"
import { Pane, PaneBack, PaneEmpty } from "@/components/common/Column"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { UsageBanner } from "@/features/analytics/AnalyticsColumn"
import { BreakdownView } from "@/features/analytics/Breakdown"
import { Controls } from "@/features/analytics/Controls"
import { EventsView } from "@/features/analytics/Events"
import { FunnelsView } from "@/features/analytics/Funnels"
import { GoalsView } from "@/features/analytics/Goals"
import { InstallPropertyPage, NewPropertyPage, WaitForFirstEvent } from "@/features/analytics/NewProperty"
import { Overview } from "@/features/analytics/Overview"
import { useAnalyticsProperties } from "@/features/analytics/properties"
import { useProperty } from "@/features/analytics/queries"
import { Realtime } from "@/features/analytics/Realtime"
import { SettingsView } from "@/features/analytics/Settings"
import { useViewSearch } from "@/features/analytics/state"
import { useAnalyticsText } from "@/features/analytics/text"
import { Segmented } from "@/components/common"
import { useIsPhone } from "@/hooks/use-media-query"
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
  "settings",
] as const
type ViewKey = (typeof VIEWS)[number]

function AnalyticsHome() {
  const { canManage } = useSession()
  const phone = useIsPhone()
  const list = useAnalyticsProperties()
  const first = list.data?.items[0]
  if (first && !phone) return <Navigate to={`/analytics/${first.id}`} replace />
  return (
    <PaneEmpty icon={ChartLineIcon} title={<Trans>Count your website’s visitors</Trans>} testId="analytics-home">
      <p>
        <Trans>
          Pages, sources, places and devices, without cookies, without storing IP addresses and without a consent banner
          where the law allows it.
        </Trans>
      </p>
      {canManage && (
        <Button className="mt-4" render={<Link to="/analytics/new" />} data-testid="analytics-home-add">
          <PlusIcon />
          <Trans>Add a website</Trans>
        </Button>
      )}
    </PaneEmpty>
  )
}

function PropertyLayout() {
  const { t } = useLingui()
  const text = useAnalyticsText()
  const { propertyId = "" } = useParams()
  const { pathname } = useLocation()
  const navigate = useNavigate()
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
  const controls = current !== "realtime" && current !== "settings"
  return (
    <Pane wide testId="analytics-property" className="gap-5" key={p.id}>
      <header className="flex flex-col">
        <PaneBack to="/analytics" label={t`Analytics`} />
        <h1 className="min-w-0 text-page break-words" data-testid="analytics-property-name">
          {p.name}
        </h1>
        <p className="mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-body text-muted-foreground">
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
        </p>
      </header>
      {usage && <UsageBanner usage={usage} />}
      <div className="-mx-1 max-w-[calc(100%+0.5rem)] overflow-x-auto px-1 py-1">
        <Segmented
          value={current}
          onChange={(v) => navigate(`/analytics/${p.id}${v === "overview" ? "" : `/${v}`}${search}`)}
          label={t`Analytics views`}
          items={VIEWS.map((v) => ({ value: v, label: names[v], testId: `analytics-tab-${v}` }))}
        />
      </div>
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

export default function AnalyticsRoutes() {
  return (
    <Routes>
      <Route index element={<AnalyticsHome />} />
      <Route path="new" element={<NewPropertyPage />} />
      <Route path="new/:propertyId" element={<InstallPropertyPage />} />
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
        <Route path="settings" element={<SettingsView />} />
        <Route path="*" element={<Navigate to=".." replace />} />
      </Route>
    </Routes>
  )
}

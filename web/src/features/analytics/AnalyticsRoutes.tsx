import "@/features/analytics/i18n"

import { Trans, useLingui } from "@lingui/react/macro"
import { useEffect, useRef } from "react"
import { ChartLineIcon, ChevronsUpDownIcon, EllipsisIcon, PlusIcon } from "lucide-react"
import {
  Link,
  Navigate,
  Outlet,
  Route,
  Routes,
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router"

import { Dot, ErrorLine } from "@/components/common"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { UsageBanner } from "@/features/analytics/AnalyticsColumn"
import { BreakdownView } from "@/features/analytics/Breakdown"
import { Controls, FilterBar, RangePicker } from "@/features/analytics/Controls"
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
import { useInboxFilter } from "@/features/inbox/inboxFilter"
import { useIsPhone } from "@/hooks/use-media-query"
import { PagesView } from "@/features/analytics/Pages"
import { headButton, Page, PageHeader } from "@/features/settings/ui"
import type { SetupState } from "@/features/setup/setup"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
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

function AnalyticsHome() {
  const { canManage } = useSession()
  const phone = useIsPhone()
  const [inboxId] = useInboxFilter()
  const list = useAnalyticsProperties()
  const first = list.data?.items.find((p) => !inboxId || p.inbox_id === inboxId)
  if (first && !phone) return <Navigate to={`/analytics/${first.id}`} replace />
  return (
    <Page testId="analytics-home">
      <PageHeader
        back={false}
        title={<Trans>Analytics</Trans>}
        description={<Trans>Cookieless visitors across your sites and apps</Trans>}
      />
      <div className="flex flex-col items-center gap-2 rounded-xl border bg-card px-6 py-12 text-center">
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
    </Page>
  )
}

const HEADED: ViewKey[] = ["overview", "pages", "goals"]

function PropertyLayout() {
  const { t } = useLingui()
  const text = useAnalyticsText()
  const { propertyId = "" } = useParams()
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const { canManage } = useSession()
  const search = useViewSearch(true)
  const [params] = useSearchParams()
  const [inboxId] = useInboxFilter()
  const property = useProperty(propertyId)
  const list = useAnalyticsProperties().data
  const usage = list?.usage
  const others = (list?.items ?? []).filter((o) => !inboxId || o.inbox_id === inboxId)
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
  const seenInbox = useRef(inboxId)
  useEffect(() => {
    if (seenInbox.current === inboxId || !p || !list) return
    seenInbox.current = inboxId
    if (!inboxId || p.inbox_id === inboxId) return
    const next = list.items.find((o) => o.inbox_id === inboxId)
    const view = current === "settings" ? "" : current === "overview" ? "" : `/${current}`
    navigate(next ? `/analytics/${next.id}${view}${search}` : "/analytics")
  }, [inboxId, p, list, current, search, navigate])
  if (!p) {
    return (
      <Page>
        <PageHeader back={false} title={<Trans>Analytics</Trans>} />
        {property.error ? <ErrorLine error={property.error} /> : <Skeleton className="h-64 rounded-xl" />}
      </Page>
    )
  }
  const controls = current !== "realtime" && current !== "settings" && current !== "retention"
  const headed = HEADED.includes(current)
  const to = (v: ViewKey, id = p.id) => `/analytics/${id}${v === "overview" ? "" : `/${v}`}${search}`
  const where = (
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
  )
  const head: Partial<Record<ViewKey, { title: React.ReactNode; description: React.ReactNode }>> = {
    overview: {
      title: <Trans>Analytics</Trans>,
      description: <Trans>Cookieless visitors across your sites and apps</Trans>,
    },
    pages: { title: names.pages, description: <Trans>Which pages people visit, and which lead to questions</Trans> },
    goals: { title: names.goals, description: <Trans>Visits and events that count as success</Trans> },
  }
  const picked = params.get("goal")
  const newGoal = new URLSearchParams(search)
  newGoal.set("goal", "new")
  return (
    <Page testId="analytics-property" key={p.id}>
      <PageHeader
        back={false}
        title={head[current]?.title ?? names[current]}
        description={head[current]?.description ?? where}
        action={
          <>
            {others.length > 1 ? (
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={<Button variant="outline" className={cn(headButton.secondary, "max-w-56")} />}
                  data-testid="analytics-property-name"
                >
                  <span className="truncate">{p.name}</span>
                  <ChevronsUpDownIcon className="text-faint" />
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-56">
                  {others.map((o) => (
                    <DropdownMenuItem
                      key={o.id}
                      onClick={() => navigate(to(current === "settings" ? "overview" : current, o.id))}
                      aria-current={o.id === p.id ? "page" : undefined}
                      className="aria-[current=page]:font-medium"
                    >
                      <span className="truncate">{o.name}</span>
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            ) : (
              <span className="sr-only" data-testid="analytics-property-name">
                {p.name}
              </span>
            )}
            <DropdownMenu>
              <DropdownMenuTrigger
                render={<Button variant="outline" size="icon" className={cn(headButton.secondary, "w-[34px] px-0")} />}
                aria-label={t`More views`}
                title={t`More views`}
                data-testid="analytics-tab-more"
              >
                <EllipsisIcon />
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-48">
                {VIEWS.map((v) => (
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
            {controls && current !== "goals" && <RangePicker plain className={headButton.secondary} />}
            {current === "goals"
              ? canManage &&
                picked !== "new" && (
                  <Button
                    className={headButton.primary}
                    render={<Link to={`?${newGoal.toString()}`} />}
                    data-testid="analytics-goal-new"
                  >
                    <Trans>New goal</Trans>
                  </Button>
                )
              : canManage &&
                current !== "pages" && (
                  <Button
                    className={headButton.primary}
                    render={<Link to={`/analytics/${p.id}/settings?tab=sharing`} />}
                    data-testid="analytics-share"
                  >
                    <Trans>Share</Trans>
                  </Button>
                )}
          </>
        }
      />
      {usage && <UsageBanner usage={usage} />}
      {!p.first_event_at && current !== "settings" && <WaitForFirstEvent propertyId={p.id} />}
      {controls &&
        (headed ? (
          current !== "pages" && <FilterBar />
        ) : (
          <Controls
            propertyId={p.id}
            range={false}
            compare={current !== "events" && current !== "funnels"}
            filters={current !== "funnels"}
          />
        ))}
      <Outlet context={{ property: p }} />
    </Page>
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
        <Route path="pages" element={<PagesView />} />
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

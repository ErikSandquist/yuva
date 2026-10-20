import { lazy, Suspense, useCallback, useEffect } from "react"
import { Outlet, useLocation, useMatch, useNavigate } from "react-router"

import { Rail, TabBar, useSection } from "@/app/Rail"
import { isInboxRoot, isSectionHome } from "@/app/shell"
import { BroadcastsColumn } from "@/features/broadcasts/BroadcastsColumn"
import { ContactsColumn } from "@/features/contact/ContactsColumn"
import { DocsColumn } from "@/features/docs/DocsColumn"
import { FeedbackList } from "@/features/feedback/FeedbackList"
import { ChatList } from "@/features/inbox/ChatList"
import { setRailView, useRailView } from "@/features/inbox/railView"
import { AssistantsPanel, MentionsPanel } from "@/features/inbox/SidePanels"
import { ReportsColumn } from "@/features/reports/ReportsColumn"
import { SettingsColumn } from "@/features/settings/SettingsColumn"
import { StatusColumn } from "@/features/status/StatusColumn"
import { TeamColumn } from "@/features/team/TeamColumn"
import { useIsPhone } from "@/hooks/use-media-query"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"

const HelpColumn = lazy(() => import("@/features/help/HelpColumn"))
const RoadmapColumn = lazy(() => import("@/features/roadmap/RoadmapColumn"))
const AnalyticsColumn = lazy(() => import("@/features/analytics/AnalyticsColumn"))

function LeftColumn({ keepList }: { keepList: boolean }) {
  const section = useSection()
  const navigate = useNavigate()
  const currentId = useMatch("/conversations/:conversationId")?.params.conversationId ?? null
  const open = useCallback((id: string) => navigate(`/conversations/${id}`), [navigate])
  const feedbackId = useMatch("/feedback/:conversationId")?.params.conversationId ?? null
  const openFeedback = useCallback((id: string) => navigate(`/feedback/${id}`), [navigate])
  const chats = section === "conversations"
  return (
    <>
      {(chats || keepList) && (
        <div className={cn("flex min-h-0 flex-1 flex-col", !chats && "hidden")}>
          <ChatList currentId={currentId} onOpen={open} active={chats} />
        </div>
      )}
      {section === "feedback" && <FeedbackList currentId={feedbackId} onOpen={openFeedback} />}
      {section === "mentions" && <MentionsPanel />}
      {section === "assistants" && <AssistantsPanel />}
      {section === "contacts" && <ContactsColumn />}
      {section === "team" && <TeamColumn />}
      {section === "reports" && <ReportsColumn />}
      {section === "docs" && <DocsColumn />}
      {section === "broadcasts" && <BroadcastsColumn />}
      {section === "status" && <StatusColumn />}
      {section === "knowledge" && (
        <Suspense fallback={<div className="flex-1 bg-background" />}>
          <HelpColumn />
        </Suspense>
      )}
      {section === "board" && (
        <Suspense fallback={<div className="flex-1 bg-background" />}>
          <RoadmapColumn />
        </Suspense>
      )}
      {section === "analytics" && (
        <Suspense fallback={<div className="flex-1 bg-background" />}>
          <AnalyticsColumn />
        </Suspense>
      )}
      {section === "settings" && <SettingsColumn />}
    </>
  )
}

function useMentionsView(pathname: string, manager: boolean) {
  const view = useRailView()
  useEffect(() => {
    if (pathname === "/mentions") setRailView("mentions")
    else if (isInboxRoot(pathname, manager) && view === "mentions") setRailView("conversations")
  }, [pathname, view, manager])
}

export function Layout() {
  const phone = useIsPhone()
  const { pathname } = useLocation()
  const { canManage } = useSession()
  const section = useSection()
  useMentionsView(pathname, canManage)
  if (section === "home") {
    return (
      <div className={cn("flex h-svh min-h-0", phone && "flex-col")} data-testid="shell">
        {!phone && <Rail />}
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <Outlet />
        </div>
        {phone && <TabBar />}
      </div>
    )
  }
  if (phone) {
    if (!isSectionHome(pathname)) {
      return (
        <div className="flex h-svh min-h-0 flex-col" data-testid="shell">
          <Outlet />
        </div>
      )
    }
    return (
      <div className="flex h-svh min-h-0 flex-col" data-testid="shell">
        <div className="flex min-h-0 flex-1 flex-col">
          <LeftColumn keepList={false} />
        </div>
        <TabBar />
      </div>
    )
  }
  return (
    <div className="flex h-svh min-h-0" data-testid="shell">
      <Rail />
      <div className="flex w-[clamp(288px,30vw,420px)] shrink-0 flex-col border-e">
        <LeftColumn keepList />
      </div>
      <div className="flex min-w-0 flex-1 flex-col">
        <Outlet />
      </div>
    </div>
  )
}

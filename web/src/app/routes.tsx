import { lazy, Suspense } from "react"
import { Navigate, Route, Routes } from "react-router"

import { Gate } from "@/app/Gate"
import { Layout } from "@/app/Layout"
import { AccountPage } from "@/features/account/AccountPage"
import { ConnectPage } from "@/features/account/ConnectPage"
import { NewWorkspacePage } from "@/features/auth/CreateWorkspace"
import { SignInPage } from "@/features/auth/SignInPage"
import { BroadcastPage, BroadcastsHome } from "@/features/broadcasts/BroadcastPage"
import { ListPage, ListsHome } from "@/features/broadcasts/ListPage"
import { ContactPage } from "@/features/contact/ContactPage"
import { ContactsHome } from "@/features/contact/ContactsColumn"
import { DocsHome, DocsOverview, DocsPagePane } from "@/features/docs/DocsPages"
import { DiscussionsPane } from "@/features/docs/discussions/DiscussionsPane"
import { QueuePane } from "@/features/docs/discussions/QueuePane"
import { ThreadPane } from "@/features/docs/discussions/ThreadPane"
import { ChatsPage } from "@/features/inbox/ChatsPage"
import { ConsentPage } from "@/features/oauth/ConsentPage"
import { ApiKeysPage } from "@/features/settings/api-keys/ApiKeysPage"
import { AppearancePage } from "@/features/settings/appearance/AppearancePage"
import { CannedRepliesPage } from "@/features/settings/canned/CannedRepliesPage"
import { AssistantPage } from "@/features/settings/connected-apps/AssistantPage"
import { ConnectedAppsPage } from "@/features/settings/connected-apps/ConnectedAppsPage"
import { InboxPage } from "@/features/settings/inboxes/InboxPage"
import { ChannelSendingPage } from "@/features/settings/channels/email/ChannelSendingPage"
import { InboxWebhooksPage } from "@/features/settings/inboxes/InboxWebhooksPage"
import { LabelsPage } from "@/features/settings/labels/LabelsPage"
import { MembersPage } from "@/features/settings/members/MembersPage"
import { NotificationsPage } from "@/features/settings/notifications/NotificationsPage"
import { ProfilePage } from "@/features/settings/profile/ProfilePage"
import { MembersReport, RatingsReport, ReportsPane, TodayReport } from "@/features/reports/ReportPages"
import { SettingsHome, SettingsPane } from "@/features/settings/SettingsColumn"
import { WebhookPage } from "@/features/settings/webhooks/WebhookPage"
import { WebhooksPage } from "@/features/settings/webhooks/WebhooksPage"
import { WorkspacePage } from "@/features/settings/workspace/WorkspacePage"
import { SetupPage } from "@/features/setup/SetupPage"
import { MemberPage } from "@/features/team/MemberPage"
import { TeamHome } from "@/features/team/TeamColumn"

const GalleryPage = lazy(() => import("@/features/designs/GalleryPage"))
const DesignerPage = lazy(() => import("@/features/designs/designer/DesignerPage"))

function Loading() {
  return <div className="flex-1 bg-background" data-testid="route-loading" />
}

export function AppRoutes() {
  return (
    <Routes>
      <Route path="sign-in" element={<SignInPage />} />
      <Route path="oauth/consent" element={<ConsentPage />} />
      <Route path="account" element={<AccountPage />} />
      <Route path="account/connect" element={<ConnectPage />} />
      <Route element={<Gate />}>
        <Route path="workspaces/new" element={<NewWorkspacePage />} />
        <Route path="setup" element={<SetupPage />} />
        <Route path="setup/:inboxId" element={<SetupPage />} />
        <Route path="setup/:inboxId/add/:kind" element={<SetupPage />} />
        <Route path="setup/:inboxId/:channelId" element={<SetupPage />} />
        <Route path="setup/:inboxId/:channelId/wait" element={<SetupPage wait />} />
        <Route
          path="broadcasts/designs/:designId"
          element={
            <Suspense fallback={<Loading />}>
              <DesignerPage />
            </Suspense>
          }
        />
        <Route element={<Layout />}>
          <Route index element={<ChatsPage />} />
          <Route path="conversations/:conversationId" element={<ChatsPage />} />
          <Route path="mentions" element={<ChatsPage />} />
          <Route path="contacts" element={<ContactsHome />} />
          <Route path="contacts/:contactId" element={<ContactPage />} />
          <Route path="team" element={<TeamHome />} />
          <Route path="team/:memberId" element={<MemberPage />} />
          <Route path="reports" element={<ReportsPane />}>
            <Route index element={<Navigate to="today" replace />} />
            <Route path="today" element={<TodayReport />} />
            <Route path="ratings" element={<RatingsReport />} />
            <Route path="members" element={<MembersReport />} />
            <Route path="*" element={<Navigate to="/reports" replace />} />
          </Route>
          <Route path="docs" element={<DocsHome />} />
          <Route path="docs/overview" element={<DocsOverview />} />
          <Route path="docs/page" element={<DocsPagePane />} />
          <Route path="docs/discussions" element={<DiscussionsPane />} />
          <Route path="docs/discussions/queue" element={<QueuePane />} />
          <Route path="docs/discussions/:discussionId" element={<ThreadPane />} />
          <Route path="broadcasts" element={<BroadcastsHome />} />
          <Route path="broadcasts/lists" element={<ListsHome />} />
          <Route path="broadcasts/lists/:listId" element={<ListPage />} />
          {["broadcasts/designs", "broadcasts/designs/new"].map((path) => (
            <Route
              key={path}
              path={path}
              element={
                <Suspense fallback={<Loading />}>
                  <GalleryPage />
                </Suspense>
              }
            />
          ))}
          <Route path="broadcasts/:broadcastId" element={<BroadcastPage />} />
          <Route path="settings">
            <Route index element={<SettingsHome />} />
            <Route element={<SettingsPane />}>
              <Route path="profile" element={<ProfilePage />} />
              <Route path="notifications" element={<NotificationsPage />} />
              <Route path="appearance" element={<AppearancePage />} />
              <Route path="workspace" element={<WorkspacePage />} />
              <Route path="members" element={<MembersPage />} />
              <Route path="inboxes/:inboxId" element={<InboxPage />} />
              <Route path="inboxes/:inboxId/webhooks" element={<InboxWebhooksPage />} />
              <Route path="inboxes/:inboxId/channels/:channelId" element={<ChannelSendingPage />} />
              <Route path="labels" element={<LabelsPage />} />
              <Route path="canned-replies" element={<CannedRepliesPage />} />
              <Route path="api-keys" element={<ApiKeysPage />} />
              <Route path="connected-apps" element={<ConnectedAppsPage />} />
              <Route path="connected-apps/:assistant" element={<AssistantPage />} />
              <Route path="webhooks" element={<WebhooksPage />} />
              <Route path="webhooks/:webhookId" element={<WebhookPage />} />
            </Route>
            <Route path="new/*" element={<Navigate to="/setup" replace />} />
            <Route path="*" element={<Navigate to="/settings" replace />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Route>
    </Routes>
  )
}

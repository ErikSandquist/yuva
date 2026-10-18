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
import { HomePage } from "@/features/home/HomePage"
import { ChatsPage } from "@/features/inbox/ChatsPage"
import { MigrationsPage } from "@/features/migrations/MigrationsPage"
import { MigrationPage, NewMigrationPage } from "@/features/migrations/Wizard"
import { ConsentPage } from "@/features/oauth/ConsentPage"
import { ApiKeysPage } from "@/features/settings/api-keys/ApiKeysPage"
import { AppearancePage } from "@/features/settings/appearance/AppearancePage"
import { CannedRepliesPage } from "@/features/settings/canned/CannedRepliesPage"
import { AssistantPage } from "@/features/settings/connected-apps/AssistantPage"
import { ConnectedAppsPage } from "@/features/settings/connected-apps/ConnectedAppsPage"
import { InboxPage } from "@/features/settings/inboxes/InboxPage"
import { ChannelPage } from "@/features/settings/channels/ChannelPage"
import {
  ApiKeyAccessStep,
  ApiKeyDoneStep,
  ApiKeyNameStep,
  ApiKeyReviewStep,
} from "@/features/settings/api-keys/ApiKeyWizard"
import { InboxWebhooksPage } from "@/features/settings/inboxes/InboxWebhooksPage"
import { LabelsPage } from "@/features/settings/labels/LabelsPage"
import { MembersPage } from "@/features/settings/members/MembersPage"
import { NotificationsPage } from "@/features/settings/notifications/NotificationsPage"
import { ProfilePage } from "@/features/settings/profile/ProfilePage"
import { MembersReport, RatingsReport, ReportsPane, TodayReport } from "@/features/reports/ReportPages"
import { SettingsHome, SettingsPane } from "@/features/settings/SettingsColumn"
import { WebhookPage } from "@/features/settings/webhooks/WebhookPage"
import { WebhookDoneStep, WebhookEndpointStep, WebhookEventsStep } from "@/features/settings/webhooks/WebhookWizard"
import { NewsletterRulesStep, NewsletterSenderStep } from "@/features/setup/NewsletterStep"
import { WebhooksPage } from "@/features/settings/webhooks/WebhooksPage"
import { WorkspacePage } from "@/features/settings/workspace/WorkspacePage"
import { SetupPage } from "@/features/setup/SetupPage"
import { MemberPage } from "@/features/team/MemberPage"
import { TeamHome } from "@/features/team/TeamColumn"
import { useSession } from "@/lib/session"

const GalleryPage = lazy(() => import("@/features/designs/GalleryPage"))
const DesignerPage = lazy(() => import("@/features/designs/designer/DesignerPage"))
const AnalyticsRoutes = lazy(() => import("@/features/analytics/AnalyticsRoutes"))
const SharePage = lazy(() => import("@/features/analytics/SharePage"))
const AnalyticsSetup = lazy(() => import("@/features/analytics/NewProperty"))

function Loading() {
  return <div className="flex-1 bg-background" data-testid="route-loading" />
}

function IndexPage() {
  const { canManage } = useSession()
  return canManage ? <HomePage /> : <ChatsPage />
}

export function AppRoutes() {
  return (
    <Routes>
      <Route path="sign-in" element={<SignInPage />} />
      <Route path="oauth/consent" element={<ConsentPage />} />
      <Route path="account" element={<AccountPage />} />
      <Route path="account/connect" element={<ConnectPage />} />
      <Route
        path="share/:token/*"
        element={
          <Suspense fallback={<Loading />}>
            <SharePage />
          </Suspense>
        }
      />
      <Route element={<Gate />}>
        <Route path="workspaces/new" element={<NewWorkspacePage />} />
        <Route path="setup" element={<SetupPage flow="hub" />} />
        <Route path="setup/new" element={<SetupPage flow="new" />} />
        <Route path="setup/new/pick" element={<SetupPage flow="pick" />} />
        <Route path="setup/new/add/email/replies" element={<SetupPage flow="replies" />} />
        <Route path="setup/new/add/:kind" element={<SetupPage flow="add" />} />
        <Route path="setup/new/:kind" element={<SetupPage flow="new" />} />
        <Route path="setup/start/:kind" element={<SetupPage flow="start" />} />
        <Route path="setup/webhook" element={<WebhookEndpointStep />} />
        <Route path="setup/webhook/events" element={<WebhookEventsStep />} />
        <Route path="setup/webhook/done" element={<WebhookDoneStep />} />
        <Route path="setup/api-key" element={<ApiKeyNameStep />} />
        <Route path="setup/api-key/access" element={<ApiKeyAccessStep />} />
        <Route path="setup/api-key/review" element={<ApiKeyReviewStep />} />
        <Route path="setup/api-key/done" element={<ApiKeyDoneStep />} />
        <Route path="setup/newsletter" element={<SetupPage flow="newsletter" />} />
        <Route path="setup/newsletter/sender" element={<NewsletterSenderStep />} />
        <Route path="setup/newsletter/rules" element={<NewsletterRulesStep />} />
        <Route path="setup/newsletter/:listId" element={<SetupPage flow="newsletter" />} />
        {["setup/analytics", "setup/analytics/:propertyId"].map((path) => (
          <Route
            key={path}
            path={path}
            element={
              <Suspense fallback={<Loading />}>
                <AnalyticsSetup />
              </Suspense>
            }
          />
        ))}
        <Route path="setup/:inboxId" element={<SetupPage />} />
        <Route path="setup/:inboxId/add/email/replies" element={<SetupPage flow="replies" />} />
        <Route path="setup/:inboxId/add/:kind" element={<SetupPage />} />
        <Route path="setup/:inboxId/:channelId" element={<SetupPage />} />
        <Route path="setup/:inboxId/:channelId/wait" element={<SetupPage flow="wait" />} />
        <Route path="setup/:inboxId/:channelId/test" element={<SetupPage flow="test" />} />
        <Route path="setup/:inboxId/:channelId/done" element={<SetupPage flow="done" />} />
        <Route path="migrations/new" element={<NewMigrationPage />} />
        <Route path="migrations/:migrationId" element={<MigrationPage />} />
        <Route path="migrations/:migrationId/:step" element={<MigrationPage />} />
        <Route
          path="broadcasts/designs/:designId"
          element={
            <Suspense fallback={<Loading />}>
              <DesignerPage />
            </Suspense>
          }
        />
        <Route element={<Layout />}>
          <Route index element={<IndexPage />} />
          <Route path="conversations" element={<ChatsPage />} />
          <Route path="conversations/:conversationId" element={<ChatsPage />} />
          <Route path="mentions" element={<ChatsPage />} />
          <Route path="feedback" element={<ChatsPage section="feedback" />} />
          <Route path="feedback/:conversationId" element={<ChatsPage section="feedback" />} />
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
          <Route
            path="analytics/*"
            element={
              <Suspense fallback={<Loading />}>
                <AnalyticsRoutes />
              </Suspense>
            }
          />
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
              <Route path="inboxes/:inboxId/channels/:channelId" element={<ChannelPage />} />
              <Route path="labels" element={<LabelsPage />} />
              <Route path="canned-replies" element={<CannedRepliesPage />} />
              <Route path="api-keys" element={<ApiKeysPage />} />
              <Route path="connected-apps" element={<ConnectedAppsPage />} />
              <Route path="connected-apps/:assistant" element={<AssistantPage />} />
              <Route path="webhooks" element={<WebhooksPage />} />
              <Route path="webhooks/:webhookId" element={<WebhookPage />} />
              <Route path="migrations" element={<MigrationsPage />} />
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

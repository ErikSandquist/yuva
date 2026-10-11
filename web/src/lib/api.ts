import { useQuery } from "@tanstack/react-query"
import createClient from "openapi-fetch"

import type { components, paths } from "@/lib/schema.gen"

type S = components["schemas"]
export type Me = S["Me"]
export type Membership = S["Membership"]
export type Role = S["Role"]
export type Locale = S["Locale"]
export type Member = S["Member"]
export type Stats = S["Stats"]
export type Invite = S["Invite"]
export type ApiKey = S["ApiKey"]
export type ApiKeyScope = S["ApiKeyScope"]
export type ApiKeyCreate = S["ApiKeyCreate"]
export type ApiKeyUpdate = S["ApiKeyUpdate"]
export type Passkey = S["Passkey"]
export type OAuthRequest = S["OAuthRequest"]
export type OAuthGrant = S["OAuthGrant"]
export type OAuthResource = S["OAuthResource"]
export type Inbox = S["Inbox"]
export type InboxUpdate = S["InboxUpdate"]
export type InboxMode = S["InboxMode"]
export type BusinessHours = S["BusinessHours"]
export type Weekday = S["Weekday"]
export type Channel = S["Channel"]
export type ChannelKind = S["ChannelKind"]
export type EmailChannel = S["EmailChannel"]
export type EmailChannelInput = S["EmailChannelInput"]
export type SmtpTls = S["SmtpTls"]
export type ChatChannel = S["ChatChannel"]
export type ChatChannelInput = S["ChatChannelInput"]
export type ChatLauncherPosition = S["ChatLauncherPosition"]
export type AppChannel = S["AppChannel"]
export type AppChannelInput = S["AppChannelInput"]
export type AppPlatform = S["AppPlatform"]
export type Availability = S["Availability"]
export type Typing = S["Typing"]
export type TypingAuthor = S["TypingAuthor"]
export type UndeliverableEmail = S["UndeliverableEmail"]
export type Contact = S["Contact"]
export type ContactUpdate = S["ContactUpdate"]
export type ContactNote = S["ContactNote"]
export type ContactSummary = S["ContactSummary"]
export type ConversationCreate = S["ConversationCreate"]
export type Conversation = S["Conversation"]
export type ConversationListItem = S["ConversationListItem"]
export type ConversationRead = S["ConversationRead"]
export type ConversationSummary = S["ConversationSummary"]
export type Mention = S["Mention"]
export type DraftItem = S["DraftItem"]
export type DraftAuthorKind = S["DraftAuthorKind"]
export type ConversationPin = S["ConversationPin"]
export type ConversationStatus = S["ConversationStatus"]
export type ConversationUpdate = S["ConversationUpdate"]
export type ConversationBulkUpdate = S["ConversationBulkUpdate"]
export type ConversationBulkFailure = S["ConversationBulkFailure"]
export type Feedback = S["Feedback"]
export type FeedbackCategory = S["FeedbackCategory"]
export type Priority = S["Priority"]
export type Rating = S["Rating"]
export type PageRating = S["PageRating"]
export type SourcePage = S["SourcePage"]
export type Discussion = S["Discussion"]
export type DiscussionPost = S["DiscussionPost"]
export type DiscussionDetail = S["DiscussionDetail"]
export type DiscussionAuthor = S["DiscussionAuthor"]
export type DiscussionState = S["DiscussionState"]
export type DiscussionMode = S["DiscussionMode"]
export type DiscussionTeamSort = S["DiscussionTeamSort"]
export type DiscussionPostItem = S["DiscussionPostItem"]
export type DiscussionFromConversation = S["DiscussionFromConversation"]
export type ReaderAccount = S["ReaderAccount"]
export type ReaderDiscussion = S["ReaderDiscussion"]
export type ReaderNotification = S["ReaderNotification"]
export type ReaderToken = S["ReaderToken"]
export type DiscussionQuery = NonNullable<paths["/v1/discussions"]["get"]["parameters"]["query"]>
export type DocsPage = S["DocsPage"]
export type DocsPageDay = S["DocsPageDay"]
export type DocsPageSort = S["DocsPageSort"]
export type DocsSummary = S["DocsSummary"]
export type DocsSite = S["DocsSite"]
export type Message = S["Message"]
export type MessageAuthor = S["MessageAuthor"]
export type MessageDelivery = S["MessageDelivery"]
export type MessageCreate = S["MessageCreate"]
export type Attachment = S["Attachment"]
export type Label = S["Label"]
export type CannedReply = S["CannedReply"]
export type WebhookEndpoint = S["WebhookEndpoint"]
export type WebhookEventType = S["WebhookEventType"]
export type WebhookDeliveryState = S["WebhookDeliveryState"]
export type WebhookAttempt = S["WebhookAttempt"]
export type PushSubscriptionItem = S["PushSubscription"]
export type NotificationSettings = S["NotificationSettings"]
export type NotificationEvents = S["NotificationEvents"]
export type NotificationEventsUpdate = S["NotificationEventsUpdate"]
export type NotificationChannels = S["NotificationChannels"]
export type InboxNotifications = S["InboxNotifications"]
export type InboxSender = S["InboxSender"]
export type MailingList = S["MailingList"]
export type MailingListCreate = S["MailingListCreate"]
export type MailingListUpdate = S["MailingListUpdate"]
export type MailingListGrowth = S["MailingListGrowth"]
export type Subscription = S["Subscription"]
export type SubscriptionState = S["SubscriptionState"]
export type SubscriptionSource = S["SubscriptionSource"]
export type SubscriptionCreate = S["SubscriptionCreate"]
export type SubscriptionDetail = S["SubscriptionDetail"]
export type UnsubscribeReason = S["UnsubscribeReason"]
export type ConsentEvent = S["ConsentEvent"]
export type ConsentEventKind = S["ConsentEventKind"]
export type ContactSubscription = S["ContactSubscription"]
export type EmailTemplate = S["EmailTemplate"]
export type EmailTemplateLocale = S["EmailTemplateLocale"]
export type EmailDesign = S["EmailDesign"]
export type EmailDesignSummary = S["EmailDesignSummary"]
export type EmailDesignDocument = S["EmailDesignDocument"]
export type EmailDesignIssue = S["EmailDesignIssue"]
export type EmailDesignRender = S["EmailDesignRender"]
export type EmailDesignVersion = S["EmailDesignVersion"]
export type EmailDesignProblem = S["EmailDesignProblem"]
export type EmailImage = S["EmailImage"]
export type EmailImageProblem = S["EmailImageProblem"]
export type EmailTheme = S["EmailTheme"]
export type EmailThemeColors = S["EmailThemeColors"]
export type EmailFont = S["EmailFont"]
export type EmailBlock = S["EmailBlock"]
export type EmailSectionChild = S["EmailSectionChild"]
export type EmailColumnChild = S["EmailColumnChild"]
export type EmailSocialNetwork = S["EmailSocialNetwork"]
export type Broadcast = S["Broadcast"]
export type BroadcastState = S["BroadcastState"]
export type BroadcastPauseReason = S["BroadcastPauseReason"]
export type BroadcastCounts = S["BroadcastCounts"]
export type BroadcastCondition = S["BroadcastCondition"]
export type BroadcastConditionOp = S["BroadcastConditionOp"]
export type BroadcastAudience = S["BroadcastAudience"]
export type BroadcastAudienceInput = S["BroadcastAudienceInput"]
export type BroadcastAudienceCount = S["BroadcastAudienceCount"]
export type BroadcastAudienceExcluded = S["BroadcastAudienceExcluded"]
export type BroadcastUpdate = S["BroadcastUpdate"]
export type BroadcastReview = S["BroadcastReview"]
export type BroadcastReviewItem = S["BroadcastReviewItem"]
export type BroadcastReviewKey = S["BroadcastReviewKey"]
export type BroadcastReviewStatus = S["BroadcastReviewStatus"]
export type BroadcastDelivery = S["BroadcastDelivery"]
export type BroadcastDeliveryState = S["BroadcastDeliveryState"]
export type BroadcastSkipReason = S["BroadcastSkipReason"]
export type BroadcastReport = S["BroadcastReport"]
export type BroadcastProgress = S["BroadcastProgress"]
export type BroadcastProblem = S["BroadcastProblem"]
export type WorkspaceBroadcasts = S["WorkspaceBroadcasts"]
export type BroadcastsStatus = S["BroadcastsStatus"]
export type SmtpSettings = S["SmtpSettings"]
type Problem = S["Problem"]
export type RealtimeMessage = S["RealtimeMessage"]
export type ConversationQuery = NonNullable<paths["/v1/conversations"]["get"]["parameters"]["query"]>

export class ApiError extends Error {
  status: number
  code?: string
  detail?: string
  body?: unknown

  constructor(status: number, problem?: Partial<Problem>) {
    super(problem?.detail ?? problem?.title ?? `HTTP ${status}`)
    this.status = status
    this.code = problem?.code
    this.detail = problem?.detail
    this.body = problem
  }
}

export function isGone(err: unknown) {
  return err instanceof ApiError && (err.status === 404 || err.status === 403)
}

let workspaceId: string | null = null

export function setWorkspace(id: string | null) {
  workspaceId = id
}

export const api = createClient<paths>({ baseUrl: window.location.origin, credentials: "same-origin" })

api.use({
  onRequest({ request }) {
    if (workspaceId && !request.headers.has("Yuva-Workspace")) {
      request.headers.set("Yuva-Workspace", workspaceId)
    }
    return request
  },
})

export async function unwrap<T>(call: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<T> {
  const { data, error, response } = await call
  if (!response.ok) {
    throw new ApiError(response.status, (error ?? undefined) as Partial<Problem> | undefined)
  }
  return data as T
}

export function attachmentUrl(a: Attachment, ws: string) {
  return `/v1/attachments/${encodeURIComponent(a.id)}?workspace_id=${encodeURIComponent(ws)}`
}

export function rawMessageUrl(messageId: string, ws: string) {
  return `/v1/messages/${encodeURIComponent(messageId)}/raw?workspace_id=${encodeURIComponent(ws)}`
}

export function useVersion() {
  return useQuery({
    queryKey: ["version"],
    queryFn: () => unwrap(api.GET("/v1/version")),
    staleTime: Infinity,
    retry: false,
  })
}

export function useSignup() {
  return useVersion().data?.signup
}

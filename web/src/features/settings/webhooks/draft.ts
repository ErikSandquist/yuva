import { DEFAULT_EVENTS } from "@/features/settings/webhooks/events"
import { useWizardDraft } from "@/hooks/use-wizard-draft"
import type { WebhookEventType } from "@/lib/api"

export type WebhookDraft = {
  url: string
  description: string
  scope: "all" | "inbox"
  inboxId: string
  events: WebhookEventType[]
  includeNotes: boolean
}

export function useWebhookDraft(lockedInbox: string | null) {
  return useWizardDraft<WebhookDraft>(`webhook:${lockedInbox ?? "any"}`, {
    url: "",
    description: "",
    scope: "all",
    inboxId: "",
    events: DEFAULT_EVENTS,
    includeNotes: false,
  })
}

export type WebhookDone = {
  endpointId: string
  url: string
  inboxName: string | null
  events: number
  secret?: string
}

export type WebhookWizardState = { back?: string; urlError?: "private" | "invalid"; done?: WebhookDone }

export const webhookPaths = (lockedInbox: string | null) => {
  const search = lockedInbox ? `?inbox=${encodeURIComponent(lockedInbox)}` : ""
  return {
    endpoint: `/setup/webhook${search}`,
    events: `/setup/webhook/events${search}`,
    done: `/setup/webhook/done${search}`,
  }
}

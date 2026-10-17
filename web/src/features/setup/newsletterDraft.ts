import { useWizardDraft } from "@/hooks/use-wizard-draft"

export type NewsletterDraft = {
  inboxId: string
  name: string
  description: string
  channelId: string | null
  double_opt_in: boolean
  public: boolean
  welcome: boolean
  commercial: boolean
  back: string
}

export function useNewsletterDraft() {
  return useWizardDraft<NewsletterDraft>("newsletter:new", {
    inboxId: "",
    name: "",
    description: "",
    channelId: null,
    double_opt_in: true,
    public: true,
    welcome: true,
    commercial: true,
    back: "",
  })
}

export type NewsletterState = { back?: string; nameTaken?: boolean }

export const NEWSLETTER_PATHS = {
  list: "/setup/newsletter",
  sender: "/setup/newsletter/sender",
  rules: "/setup/newsletter/rules",
}

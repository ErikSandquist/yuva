import { useWizardDraft } from "@/hooks/use-wizard-draft"
import type { SmtpTls } from "@/lib/api"

export type EmailMethod = "ses" | "smtp" | "none"

export type EmailDraft = {
  address: string
  displayName: string
  fromAddress: string
  method: EmailMethod | ""
  region: string
  host: string
  port: string
  tls: SmtpTls
  username: string
  sesTenant: string
  sesSet: string
}

export const SES_REGIONS = [
  "us-east-1",
  "us-east-2",
  "us-west-1",
  "us-west-2",
  "ca-central-1",
  "sa-east-1",
  "eu-west-1",
  "eu-west-2",
  "eu-west-3",
  "eu-central-1",
  "eu-north-1",
  "eu-south-1",
  "il-central-1",
  "me-south-1",
  "af-south-1",
  "ap-south-1",
  "ap-northeast-1",
  "ap-northeast-2",
  "ap-northeast-3",
  "ap-southeast-1",
  "ap-southeast-2",
]

export function sesHost(region: string) {
  return `email-smtp.${region}.amazonaws.com`
}

export function sesRegion(host: string) {
  return /^email-smtp(?:-fips)?\.([a-z0-9-]+)\.amazonaws\.com$/i.exec(host.trim())?.[1]
}

export function useEmailDraft(inboxId: string) {
  return useWizardDraft<EmailDraft>(`email:${inboxId}`, {
    address: "",
    displayName: "",
    fromAddress: "",
    method: "",
    region: "",
    host: "",
    port: "",
    tls: "starttls",
    username: "",
    sesTenant: "",
    sesSet: "",
  })
}

export const emailPaths = (inboxId: string, channelId?: string) => ({
  address: `/setup/${inboxId}/add/email`,
  replies: `/setup/${inboxId}/add/email/replies`,
  receive: `/setup/${inboxId}/${channelId}`,
  test: `/setup/${inboxId}/${channelId}/test`,
  done: `/setup/${inboxId}/${channelId}/done`,
})

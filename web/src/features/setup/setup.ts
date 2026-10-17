import type { HomeSetupItem } from "@/lib/api"

export type SetupKind = "chat" | "email" | "app" | "docs"
export const SETUP_KINDS: SetupKind[] = ["chat", "email", "app", "docs"]

export type SetupState = { secret?: string; back?: string; existing?: boolean; emailError?: "taken" | "invalid" }

export type SetupStage = "name" | "pick" | "fields" | "install" | "wait"

export function isTrimmed(kind: SetupKind | null | undefined, existing: boolean | undefined) {
  return !!existing && !!kind && kind !== "email"
}

export function stepNumber(stage: SetupStage, trimmed: boolean) {
  if (trimmed) return { name: 1, pick: 1, fields: 1, install: 2, wait: 3 }[stage]
  return { name: 1, pick: 2, fields: 2, install: 3, wait: 4 }[stage]
}

export type HubId = HomeSetupItem["id"]
export const HUB_IDS: HubId[] = ["chat", "email", "app", "docs", "analytics", "newsletter"]

export function startPath(id: HubId) {
  if (id === "analytics") return "/setup/analytics"
  if (id === "newsletter") return "/setup/newsletter"
  return `/setup/start/${id}`
}

export function browserLanguage() {
  const tag = (navigator.language || "en").split("-")[0]!.toLowerCase()
  return /^[a-z]{2,3}$/.test(tag) ? tag : "en"
}

export function browserTimeZone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC"
}

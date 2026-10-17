import { useCallback } from "react"
import { useQueryClient } from "@tanstack/react-query"

import { useWizardDraft } from "@/hooks/use-wizard-draft"
import type { SetupKind } from "@/features/setup/setup"
import { browserLanguage, browserTimeZone } from "@/features/setup/setup"
import { api, unwrap } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

export type InboxDraft = { name: string; locale: string; zone: string; inboxId?: string; kind?: SetupKind }

export function useInboxDraft() {
  return useWizardDraft<InboxDraft>("inbox:new", { name: "", locale: browserLanguage(), zone: browserTimeZone() })
}

export function useLeaveSetup() {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  const [, , clear] = useInboxDraft()
  return useCallback(() => {
    let created: string | undefined
    try {
      const raw = sessionStorage.getItem(`yuva:wizard:${ws}:inbox:new`)
      created = raw ? (JSON.parse(raw) as Partial<InboxDraft>).inboxId : undefined
    } catch {
      created = undefined
    }
    clear()
    if (!created) return
    const path = { inboxId: created }
    void unwrap(api.GET("/v1/inboxes/{inboxId}/channels", { params: { path } }))
      .then((r) =>
        r.items.length === 0 ? unwrap(api.DELETE("/v1/inboxes/{inboxId}", { params: { path } })) : undefined
      )
      .catch(() => undefined)
      .finally(() => qc.invalidateQueries({ queryKey: keys.inboxes(ws) }))
  }, [clear, qc, ws])
}

export function idempotencyKey(key: string, part: string, body: unknown) {
  let h = 5381
  for (const c of JSON.stringify(body)) h = ((h * 33) ^ c.charCodeAt(0)) >>> 0
  return `${key}:${part}:${h.toString(36)}`
}

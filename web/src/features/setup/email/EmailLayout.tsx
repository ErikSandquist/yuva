import { Trans, useLingui } from "@lingui/react/macro"
import { useLocation, useNavigate } from "react-router"

import { Button } from "@/components/ui/button"
import { emailPaths } from "@/features/setup/email/draft"
import { type SetupState } from "@/features/setup/setup"
import { SetupLayout } from "@/features/setup/SetupLayout"

export type EmailStage = "address" | "replies" | "receive" | "test" | "done"
const STAGES: EmailStage[] = ["address", "replies", "receive", "test", "done"]

export function EmailLayout({
  stage,
  isNew,
  inboxId,
  channelId,
  onLeave,
  back,
  preview,
  previewOpen,
  children,
}: {
  stage: EmailStage
  isNew: boolean
  inboxId: string
  channelId?: string
  onLeave?: () => void
  back?: string
  preview: React.ReactNode
  previewOpen?: boolean
  children: React.ReactNode
}) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const state = (useLocation().state ?? {}) as SetupState
  const offset = isNew ? 2 : 0
  const labels = [...(isNew ? [t`Name`, t`Channel`] : []), t`Address`, t`Replies`, t`Receiving`, t`Test`, t`Done`]
  const paths = emailPaths(inboxId, channelId)
  const stepHref = (key: string) => {
    if (!channelId) return undefined
    const n = Number(key) - offset
    return n === 3 ? paths.receive : n === 4 ? paths.test : undefined
  }
  const finish = () => navigate(state.back ?? `/settings/inboxes/${inboxId}`, { replace: true })
  return (
    <SetupLayout
      step={offset + STAGES.indexOf(stage) + 1}
      steps={labels}
      stepHref={stepHref}
      leave={
        channelId && stage !== "done" ? (
          <Button variant="ghost" size="sm" onClick={finish} data-testid="setup-later">
            <Trans>Finish later</Trans>
          </Button>
        ) : undefined
      }
      onLeave={onLeave}
      back={back}
      preview={preview}
      previewOpen={previewOpen}
    >
      {children}
    </SetupLayout>
  )
}

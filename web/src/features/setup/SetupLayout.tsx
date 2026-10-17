import { useLingui } from "@lingui/react/macro"

import { WizardFrame } from "@/components/common/Wizard"
import type { SetupKind } from "@/features/setup/setup"

export function useSetupSteps(kind?: SetupKind | null, trimmed?: boolean) {
  const { t } = useLingui()
  if (trimmed && kind && kind !== "email") {
    const first = { chat: t`Sites`, app: t`Platforms`, docs: t`Site` }[kind]
    return [first, t`Install`, t`First message`]
  }
  return [t`Name`, t`Channel`, t`Install`, t`First message`, t`Done`]
}

export function SetupLayout({
  step,
  steps,
  stepsLabel,
  stepHref,
  leave,
  onLeave,
  kind,
  trimmed,
  preview,
  previewOpen,
  back,
  children,
}: {
  step?: number
  steps?: string[]
  stepsLabel?: string
  stepHref?: (key: string) => string | undefined
  leave?: React.ReactNode
  onLeave?: () => void
  kind?: SetupKind | null
  trimmed?: boolean
  preview: React.ReactNode
  previewOpen?: boolean
  back?: string
  children: React.ReactNode
}) {
  const { t } = useLingui()
  const setupSteps = useSetupSteps(kind, trimmed)
  const names = steps ?? setupSteps
  return (
    <WizardFrame
      steps={step === undefined ? undefined : names.map((label, i) => ({ key: String(i + 1), label }))}
      current={step === undefined ? undefined : String(step)}
      stepHref={stepHref}
      stepsLabel={stepsLabel ?? t`Setup steps`}
      leave={leave}
      onLeave={onLeave}
      preview={preview}
      previewOpen={previewOpen}
      back={back}
    >
      {children}
    </WizardFrame>
  )
}

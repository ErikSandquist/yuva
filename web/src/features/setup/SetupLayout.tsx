import { useLingui } from "@lingui/react/macro"

import { WizardFrame } from "@/components/common/Wizard"

function useSetupSteps() {
  const { t } = useLingui()
  return [t`Name`, t`Channel`, t`Install`, t`First message`, t`Done`]
}

export function SetupLayout({
  step,
  steps,
  stepsLabel,
  stepHref,
  leave,
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
  preview: React.ReactNode
  previewOpen?: boolean
  back?: string
  children: React.ReactNode
}) {
  const { t } = useLingui()
  const setupSteps = useSetupSteps()
  const names = steps ?? setupSteps
  return (
    <WizardFrame
      steps={step === undefined ? undefined : names.map((label, i) => ({ key: String(i + 1), label }))}
      current={step === undefined ? undefined : String(step)}
      stepHref={stepHref}
      stepsLabel={stepsLabel ?? t`Setup steps`}
      leave={leave}
      preview={preview}
      previewOpen={previewOpen}
      back={back}
    >
      {children}
    </WizardFrame>
  )
}

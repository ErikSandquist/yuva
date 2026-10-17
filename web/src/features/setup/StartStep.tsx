import { Trans } from "@lingui/react/macro"
import { PlusIcon } from "lucide-react"
import { useLocation } from "react-router"

import { ChoiceCards, Dot, ErrorLine, StepHeader } from "@/components/common"
import { Skeleton } from "@/components/ui/skeleton"
import { useKindText } from "@/features/setup/ChannelStep"
import { NameStep } from "@/features/setup/NameStep"
import { DEFAULT_COLOR, KindPreview, PreviewCaption } from "@/features/setup/previews"
import type { SetupKind, SetupState } from "@/features/setup/setup"
import { SetupLayout } from "@/features/setup/SetupLayout"
import { useInboxes } from "@/lib/workspace"

export function StartStep({ kind }: { kind: SetupKind }) {
  const state = (useLocation().state ?? {}) as SetupState
  const inboxes = useInboxes()
  const text = useKindText()
  const list = inboxes.data
  if (list && list.length === 0) return <NameStep kind={kind} />
  const first = list?.[0]
  return (
    <SetupLayout
      step={1}
      preview={
        <>
          <KindPreview
            kind={kind}
            name={first?.name ?? ""}
            color={first?.branding.color ?? DEFAULT_COLOR}
            host="www.example.com"
          />
          <PreviewCaption>{text[kind].hint}</PreviewCaption>
        </>
      }
    >
      <StepHeader title={<Trans>Which product is it for?</Trans>}>
        <Trans>Each product or brand has its own inbox. Add it to one you have, or start a new one.</Trans>
      </StepHeader>
      <ErrorLine error={inboxes.error} />
      {!list ? (
        !inboxes.error && <Skeleton className="h-[74px] rounded-2xl" />
      ) : (
        <ChoiceCards
          testId="setup-start"
          items={[
            ...list.map((i) => ({
              value: i.id,
              icon: <Dot color={i.branding.color} className="size-3 rounded-[4px]" />,
              title: i.name,
              to: `/setup/${i.id}/add/${kind}`,
              state,
              testId: `setup-start-${i.id}`,
            })),
            {
              value: "new",
              icon: <PlusIcon />,
              title: <Trans>A new product</Trans>,
              hint: <Trans>With its own name, language and time zone</Trans>,
              to: `/setup/new/${kind}`,
              state,
              testId: "setup-start-new",
            },
          ]}
        />
      )}
    </SetupLayout>
  )
}

import { Trans } from "@lingui/react/macro"
import { ChevronRightIcon, PlusIcon } from "lucide-react"
import { Link, useLocation } from "react-router"

import { Dot, ErrorLine } from "@/components/common"
import { Skeleton } from "@/components/ui/skeleton"
import { useKindText } from "@/features/setup/ChannelStep"
import { NameStep } from "@/features/setup/NameStep"
import { DEFAULT_COLOR, KindPreview, PreviewCaption } from "@/features/setup/previews"
import type { SetupKind, SetupState } from "@/features/setup/setup"
import { SetupLayout, StepHeader } from "@/features/setup/SetupLayout"
import { useInboxes } from "@/lib/workspace"

const rowClass =
  "group flex w-full items-center gap-4 rounded-2xl border bg-card px-4 py-4 text-start transition-colors outline-none hover:border-faint focus-visible:ring-3 focus-visible:ring-ring/30"

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
        <ul className="flex flex-col gap-2.5" data-testid="setup-start">
          {list.map((i) => (
            <li key={i.id}>
              <Link
                to={`/setup/${i.id}/add/${kind}`}
                state={state}
                className={rowClass}
                data-testid={`setup-start-${i.id}`}
              >
                <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-surface">
                  <Dot color={i.branding.color} className="size-3 rounded-[4px]" />
                </span>
                <span className="min-w-0 flex-1 truncate font-medium">{i.name}</span>
                <ChevronRightIcon className="size-4 shrink-0 text-faint transition-transform group-hover:translate-x-0.5 rtl:rotate-180" />
              </Link>
            </li>
          ))}
          <li>
            <Link to={`/setup/new/${kind}`} state={state} className={rowClass} data-testid="setup-start-new">
              <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-surface text-muted-foreground [&_svg]:size-[18px]">
                <PlusIcon />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block font-medium">
                  <Trans>A new product</Trans>
                </span>
                <span className="mt-0.5 block text-small text-faint">
                  <Trans>With its own name, language and time zone</Trans>
                </span>
              </span>
              <ChevronRightIcon className="size-4 shrink-0 text-faint transition-transform group-hover:translate-x-0.5 rtl:rotate-180" />
            </Link>
          </li>
        </ul>
      )}
    </SetupLayout>
  )
}

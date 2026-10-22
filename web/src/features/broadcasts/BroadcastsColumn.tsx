import { Trans, useLingui } from "@lingui/react/macro"
import { lazy, Suspense } from "react"
import { useNavigate } from "react-router"

import { Segmented } from "@/components/common"
import { Column } from "@/components/common/Column"
import { Skeleton } from "@/components/ui/skeleton"
import { InboxPicker } from "@/features/inbox/InboxPicker"

const DesignsList = lazy(() => import("@/features/designs/DesignsList"))

type Tab = "broadcasts" | "lists" | "designs"

const TAB_PATHS: Record<Tab, string> = {
  broadcasts: "/broadcasts",
  lists: "/broadcasts/lists",
  designs: "/broadcasts/designs",
}

export function BroadcastsColumn() {
  const { t } = useLingui()
  const navigate = useNavigate()
  return (
    <Column
      title={<Trans>Broadcasts</Trans>}
      testId="broadcasts-panel"
      actions={<InboxPicker className="max-w-40" />}
      bodyClassName="px-2"
    >
      <div className="px-1 pt-1 pb-3">
        <Segmented
          value="designs"
          onChange={(v: Tab) => navigate(TAB_PATHS[v])}
          label={t`Broadcasts`}
          className="w-full"
          itemClassName="flex-1 px-2"
          items={[
            { value: "broadcasts", label: t`Broadcasts`, testId: "broadcasts-tab-broadcasts" },
            { value: "lists", label: t`Lists`, testId: "broadcasts-tab-lists" },
            { value: "designs", label: t`Designs`, testId: "broadcasts-tab-designs" },
          ]}
        />
      </div>
      <Suspense
        fallback={
          <div className="flex flex-col gap-2 p-2">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        }
      >
        <DesignsList />
      </Suspense>
    </Column>
  )
}

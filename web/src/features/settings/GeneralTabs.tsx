import { useLingui } from "@lingui/react/macro"

import { PageTabs } from "@/features/settings/ui"
import { useSession } from "@/lib/session"

type Tab = "workspace" | "labels" | "canned-replies" | "migrations"

export function GeneralTabs({ current }: { current: Tab }) {
  const { t } = useLingui()
  const { canManage } = useSession()
  return (
    <PageTabs
      label={t`General`}
      current={current}
      items={[
        { key: "workspace", to: "/settings/workspace", label: t`Workspace`, testId: "general-tab-workspace" },
        { key: "labels", to: "/settings/labels", label: t`Labels`, testId: "general-tab-labels" },
        {
          key: "canned-replies",
          to: "/settings/canned-replies",
          label: t`Canned replies`,
          testId: "general-tab-canned-replies",
        },
        ...(canManage
          ? [
              {
                key: "migrations",
                to: "/settings/migrations",
                label: t`Move to Yuva`,
                testId: "general-tab-migrations",
              },
            ]
          : []),
      ]}
    />
  )
}

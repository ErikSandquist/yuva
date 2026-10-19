import { Trans } from "@lingui/react/macro"

import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import { useNoticeReach } from "@/features/status/queries"
import type { StatusPage } from "@/lib/api"

export function NotifyCheck({
  page,
  components,
  checked,
  onChange,
  testId,
}: {
  page: StatusPage
  components: string[]
  checked: boolean
  onChange: (v: boolean) => void
  testId: string
}) {
  const reach = useNoticeReach(page.id, components, page.email_subscribers.enabled)
  if (!page.email_subscribers.enabled) return null
  return (
    <Label className="items-start gap-2.5 font-normal">
      <Checkbox checked={checked} onCheckedChange={onChange} className="mt-0.5" data-testid={testId} />
      <span className="flex flex-col">
        <span className="text-body font-medium">
          <Trans>Notify subscribers</Trans>
        </span>
        {reach.data !== undefined && (
          <span className="text-small text-faint" data-testid={`${testId}-reach`}>
            <Trans>Subscribers reached: {reach.data}</Trans>
          </span>
        )}
      </span>
    </Label>
  )
}

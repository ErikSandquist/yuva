import { Plural, Trans } from "@lingui/react/macro"

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
    <Label className="items-center gap-2 font-normal">
      <Checkbox checked={checked} onCheckedChange={onChange} data-testid={testId} />
      {reach.data !== undefined ? (
        <span className="text-body" data-testid={`${testId}-reach`}>
          <Plural value={reach.data} one="E-mail # subscriber" other="E-mail # subscribers" />
        </span>
      ) : (
        <span className="text-body">
          <Trans>Notify subscribers</Trans>
        </span>
      )}
    </Label>
  )
}

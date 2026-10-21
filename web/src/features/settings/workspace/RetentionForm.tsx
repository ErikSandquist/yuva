import { Trans } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"

import { ErrorLine } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Block, BlockRow, headButton } from "@/features/settings/ui"
import { api, unwrap } from "@/lib/api"
import { meKey, useSession } from "@/lib/session"

export function RetentionForm() {
  const qc = useQueryClient()
  const { membership } = useSession()
  const current = membership.workspace.retention_days?.toString() ?? ""
  const [days, setDays] = useState(current)
  const isOwner = membership.role === "owner"
  const save = useMutation({
    mutationFn: () =>
      unwrap(api.PATCH("/v1/workspace", { body: { retention_days: days === "" ? null : Number(days) } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: meKey }),
  })
  const dirty = days !== current
  return (
    <Block
      title={<Trans>Workspace</Trans>}
      description={<Trans>Shown to your team and in e-mails from Yuva</Trans>}
      onSubmit={() => save.mutate()}
      testId="workspace-general"
      footer={
        isOwner ? (
          <>
            <ErrorLine error={save.error} className="min-w-0 flex-1" />
            {save.isSuccess && !dirty && !save.error && (
              <span className="flex-1 text-muted-foreground" role="status">
                <Trans>Saved</Trans>
              </span>
            )}
            <Button type="submit" className={headButton.primary} disabled={save.isPending || !dirty}>
              <Trans>Save</Trans>
            </Button>
          </>
        ) : (
          <p className="me-auto text-muted-foreground">
            <Trans>Only owners can change this.</Trans>
          </p>
        )
      }
    >
      <BlockRow label={<Trans>Name</Trans>}>
        <span className="truncate">{membership.workspace.name}</span>
      </BlockRow>
      <BlockRow
        label={<Trans>Delete after (days)</Trans>}
        htmlFor="retention-days"
        hint={
          <Trans>
            Closed conversations untouched for this many days are deleted with their messages and attachments, and raw
            e-mails older than this are deleted. Leave empty to keep everything.
          </Trans>
        }
      >
        <Input
          id="retention-days"
          type="number"
          min={1}
          max={36500}
          value={days}
          disabled={!isOwner}
          onChange={(e) => setDays(e.target.value)}
          className="h-[34px] w-[200px] rounded-lg border-input"
        />
      </BlockRow>
    </Block>
  )
}

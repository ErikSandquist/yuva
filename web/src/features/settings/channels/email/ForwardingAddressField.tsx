import { Trans } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"

import { CopyButton, ErrorLine, useConfirm } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Field } from "@/features/settings/ui"
import { api, unwrap, type Channel } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

export function ForwardingAddressField({ channel }: { channel: Channel }) {
  const qc = useQueryClient()
  const { workspaceId: ws } = useSession()
  const [confirm, confirmDialog] = useConfirm()
  const [value, setValue] = useState(channel.forwarding_address ?? "")
  const own = channel.email?.address ?? ""
  const rotate = useMutation({
    mutationFn: () =>
      unwrap(
        api.POST("/v1/channels/{channelId}/forwarding-address/rotate", { params: { path: { channelId: channel.id } } })
      ),
    onSuccess: (data) => {
      if (data.forwarding_address) setValue(data.forwarding_address)
      void qc.invalidateQueries({ queryKey: keys.channels(ws, channel.inbox_id) })
      qc.setQueryData(keys.channel(ws, channel.id), data)
    },
  })
  if (!value) return null
  return (
    <Field
      label={<Trans>Forwarding address</Trans>}
      hint={
        <Trans>
          Another address this channel receives mail at. Forward a mailbox here, for example {own} at its provider;
          forwarding to {own} itself would loop.
        </Trans>
      }
    >
      <div className="flex min-w-0 items-center gap-2">
        <code
          className="min-w-0 flex-1 rounded-xl border bg-surface px-3 py-2 font-mono text-caption break-all"
          data-testid="channel-forwarding-address"
        >
          {value}
        </code>
        <CopyButton value={value} />
        <Button
          type="button"
          variant="ghost"
          size="sm"
          disabled={rotate.isPending}
          data-testid="rotate-forwarding-address"
          onClick={() =>
            confirm({
              title: <Trans>Give the channel a new forwarding address?</Trans>,
              description: (
                <Trans>
                  Mail to {value} is refused from then on. Change the forwarding at the provider to the new address
                  right away, or forwarded mail no longer reaches Yuva.
                </Trans>
              ),
              confirm: <Trans>Rotate</Trans>,
              run: () => rotate.mutate(),
            })
          }
        >
          <Trans>Rotate</Trans>
        </Button>
      </div>
      <ErrorLine error={rotate.error} className="text-caption" />
      {confirmDialog}
    </Field>
  )
}

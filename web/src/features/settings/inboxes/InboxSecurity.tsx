import { Trans } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import { useNavigate } from "react-router"

import { ErrorLine, SecretDialog, TypeToConfirmDialog, useConfirm } from "@/components/common"
import { Button } from "@/components/ui/button"
import { useInboxOutlet } from "@/features/settings/inboxes/InboxPage"
import { Block, BlockRow } from "@/features/settings/ui"
import { api, unwrap } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

export function IdentitySecretRow() {
  const { inbox } = useInboxOutlet()
  const [secret, setSecret] = useState<string | null>(null)
  const [confirm, confirmDialog] = useConfirm()
  const rotate = useMutation({
    mutationFn: () =>
      unwrap(api.POST("/v1/inboxes/{inboxId}/identity-secret", { params: { path: { inboxId: inbox.id } } })),
    onSuccess: (r) => setSecret(r.identity_secret),
  })
  return (
    <BlockRow
      label={<Trans>Identity secret</Trans>}
      hint={
        <Trans>
          Your backend signs identity tokens with this secret so Yuva knows who your signed-in users are. It is shown
          only when created or rotated.
        </Trans>
      }
    >
      <div className="flex flex-wrap items-center gap-3">
        <Button
          variant="outline"
          size="sm"
          disabled={rotate.isPending}
          onClick={() =>
            confirm({
              title: <Trans>Rotate the identity secret?</Trans>,
              description: <Trans>Tokens signed with the current secret stop working at once.</Trans>,
              confirm: <Trans>Rotate</Trans>,
              run: () => rotate.mutate(),
            })
          }
        >
          <Trans>Rotate secret</Trans>
        </Button>
        <ErrorLine error={rotate.error} />
      </div>
      <SecretDialog
        secret={secret}
        title={<Trans>New identity secret</Trans>}
        description={<Trans>Copy it now and update your backend; it is not shown again.</Trans>}
        onClose={() => setSecret(null)}
      />
      {confirmDialog}
    </BlockRow>
  )
}

export function DeleteProduct() {
  const qc = useQueryClient()
  const navigate = useNavigate()
  const { inbox } = useInboxOutlet()
  const { workspaceId: ws } = useSession()
  const [open, setOpen] = useState(false)
  const remove = useMutation({
    mutationFn: () => unwrap(api.DELETE("/v1/inboxes/{inboxId}", { params: { path: { inboxId: inbox.id } } })),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: keys.inboxes(ws) })
      navigate("/settings")
    },
  })
  const name = inbox.name
  return (
    <Block
      tone="danger"
      title={<Trans>Delete product</Trans>}
      description={<Trans>Deletes its channels, conversations, messages and attachments. This cannot be undone.</Trans>}
      footer={
        <Button variant="destructive" size="sm" onClick={() => setOpen(true)} data-testid="delete-product">
          <Trans>Delete {name}…</Trans>
        </Button>
      }
    >
      <TypeToConfirmDialog
        open={open}
        onOpenChange={setOpen}
        title={<Trans>Delete {name}?</Trans>}
        description={
          <Trans>
            All of its conversations, messages and attachments are deleted with it. Webhooks and API keys limited to it
            stop working.
          </Trans>
        }
        label={<Trans>Type {name} to confirm</Trans>}
        expected={name}
        confirm={<Trans>Delete this product</Trans>}
        pending={remove.isPending}
        error={<ErrorLine error={remove.error} />}
        onConfirm={() => remove.mutate()}
      />
    </Block>
  )
}

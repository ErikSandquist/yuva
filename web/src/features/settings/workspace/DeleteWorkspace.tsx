import { Trans } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import { useNavigate } from "react-router"

import { ErrorLine, TypeToConfirmDialog } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Card, RowText, Section } from "@/features/settings/ui"
import { api, setWorkspace, unwrap } from "@/lib/api"
import { meKey, useSession } from "@/lib/session"

export function DeleteWorkspace() {
  const qc = useQueryClient()
  const navigate = useNavigate()
  const { membership } = useSession()
  const [open, setOpen] = useState(false)
  const name = membership.workspace.name
  const remove = useMutation({
    mutationFn: () => unwrap(api.DELETE("/v1/workspace", { body: { name } })),
    onSuccess: async () => {
      setOpen(false)
      setWorkspace(null)
      qc.removeQueries({ predicate: (q) => q.queryKey[0] !== meKey[0] })
      await qc.invalidateQueries({ queryKey: meKey })
      navigate("/", { replace: true })
    },
  })
  return (
    <Section
      title={<Trans>Danger zone</Trans>}
      description={
        <Trans>
          Deleting {name} removes its inboxes, channels, conversations, contacts, files, labels, webhooks and API keys
          for everyone, at once. Members keep their accounts. This cannot be undone.
        </Trans>
      }
    >
      <Card tone="danger">
        <div className="flex flex-wrap items-center gap-3">
          <RowText
            title={
              <span className="font-medium text-destructive">
                <Trans>Delete this workspace</Trans>
              </span>
            }
            detail={<Trans>Removes every inbox, conversation and contact for everyone.</Trans>}
          />
          {membership.role === "owner" ? (
            <Button
              type="button"
              variant="destructive"
              size="sm"
              onClick={() => {
                remove.reset()
                setOpen(true)
              }}
              data-testid="delete-workspace"
            >
              <Trans>Delete workspace…</Trans>
            </Button>
          ) : (
            <p className="text-small text-faint">
              <Trans>Only owners can delete the workspace.</Trans>
            </p>
          )}
        </div>
      </Card>
      <TypeToConfirmDialog
        open={open}
        onOpenChange={setOpen}
        title={<Trans>Delete {name}?</Trans>}
        description={
          <Trans>
            Everything in this workspace is deleted for every member, and its widgets, apps, e-mail addresses and API
            keys stop working immediately.
          </Trans>
        }
        label={<Trans>Type the workspace name to confirm</Trans>}
        expected={name}
        confirm={<Trans>Delete this workspace</Trans>}
        pending={remove.isPending}
        error={<ErrorLine error={remove.error} />}
        onConfirm={() => remove.mutate()}
      />
    </Section>
  )
}

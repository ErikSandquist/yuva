import { Trans } from "@lingui/react/macro"
import { Link } from "react-router"

import { Button } from "@/components/ui/button"
import { GeneralTabs } from "@/features/settings/GeneralTabs"
import { Block, BlockRow, headButton, PageHeader } from "@/features/settings/ui"
import { BroadcastsSettings } from "@/features/settings/workspace/BroadcastsSettings"
import { DeleteWorkspace } from "@/features/settings/workspace/DeleteWorkspace"
import { RetentionForm } from "@/features/settings/workspace/RetentionForm"
import { useSession } from "@/lib/session"

function MoveToYuva() {
  return (
    <Block
      title={<Trans>Move to Yuva</Trans>}
      description={<Trans>Bring your mail and help-desk history, read only from the source</Trans>}
      footer={
        <Button
          variant="outline"
          className={headButton.secondary}
          render={<Link to="/settings/migrations" />}
          data-testid="workspace-migrations"
        >
          <Trans>Start a move</Trans>
        </Button>
      }
    >
      <BlockRow label={<Trans>From a mailbox</Trans>}>
        <span className="text-muted-foreground">
          <Trans>Gmail, Outlook, Yandex, any IMAP</Trans>
        </span>
      </BlockRow>
      <BlockRow label={<Trans>From a help desk</Trans>}>
        <span className="text-muted-foreground">
          <Trans>Zendesk, Help Scout</Trans>
        </span>
      </BlockRow>
    </Block>
  )
}

export function WorkspacePage() {
  const { membership, canManage } = useSession()
  return (
    <>
      <PageHeader
        title={<Trans>General</Trans>}
        tabs={<GeneralTabs current="workspace" />}
        description={<Trans>Workspace name and data</Trans>}
      />
      <RetentionForm key={membership.workspace.retention_days ?? 0} />
      {canManage && <MoveToYuva />}
      <BroadcastsSettings />
      <DeleteWorkspace />
    </>
  )
}

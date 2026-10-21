import { Trans } from "@lingui/react/macro"
import { ArrowRightLeftIcon } from "lucide-react"
import { Link } from "react-router"

import { Button } from "@/components/ui/button"
import { Card, PageHeader, RowIcon, RowText, Section } from "@/features/settings/ui"
import { BroadcastsSettings } from "@/features/settings/workspace/BroadcastsSettings"
import { DeleteWorkspace } from "@/features/settings/workspace/DeleteWorkspace"
import { RetentionForm } from "@/features/settings/workspace/RetentionForm"
import { useSession } from "@/lib/session"

function MoveToYuva() {
  return (
    <Section title={<Trans>Move to Yuva</Trans>}>
      <Card>
        <div className="flex flex-wrap items-center gap-3">
          <RowIcon>
            <ArrowRightLeftIcon />
          </RowIcon>
          <RowText
            title={<Trans>Bring your mail history</Trans>}
            detail={<Trans>From Gmail, Outlook, Yandex or any IMAP server, read only from the source.</Trans>}
          />
          <Button
            variant="outline"
            size="sm"
            render={<Link to="/settings/migrations" />}
            data-testid="workspace-migrations"
          >
            <Trans>Start a move</Trans>
          </Button>
        </div>
      </Card>
    </Section>
  )
}

export function WorkspacePage() {
  const { membership, canManage } = useSession()
  return (
    <>
      <PageHeader title={<Trans>General</Trans>} description={membership.workspace.name} />
      <RetentionForm key={membership.workspace.retention_days ?? 0} />
      <BroadcastsSettings />
      {canManage && <MoveToYuva />}
      <DeleteWorkspace />
    </>
  )
}

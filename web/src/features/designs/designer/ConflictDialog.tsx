import { Trans, useLingui } from "@lingui/react/macro"
import { DownloadIcon } from "lucide-react"

import { formatRelative } from "@/components/common/text"
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import type { EmailDesign } from "@/lib/api"
import { useMemberMap } from "@/lib/workspace"

export function ConflictDialog({
  theirs,
  busy,
  onTheirs,
  onMine,
  onCopy,
  onDownload,
}: {
  theirs: EmailDesign | null
  busy: boolean
  onTheirs: () => void
  onMine: () => void
  onCopy: () => void
  onDownload: () => void
}) {
  const { i18n } = useLingui()
  const members = useMemberMap()
  const who = theirs?.updated_by ? members.get(theirs.updated_by)?.name : undefined
  const when = theirs ? formatRelative(theirs.updated_at, i18n.locale) : ""
  return (
    <AlertDialog open={!!theirs}>
      <AlertDialogContent className="sm:max-w-lg" data-testid="conflict-dialog">
        <AlertDialogHeader>
          <AlertDialogTitle>
            <Trans>This design changed somewhere else</Trans>
          </AlertDialogTitle>
          <AlertDialogDescription>
            {who ? (
              <Trans>
                {who} saved it {when}, after you started editing. Your changes are kept on this device until you choose.
              </Trans>
            ) : (
              <Trans>
                It was saved {when} in another tab or by someone else, after you started editing. Your changes are kept
                on this device until you choose.
              </Trans>
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <div className="flex flex-col gap-2">
          <Button
            onClick={onMine}
            disabled={busy}
            className="h-auto justify-start py-2.5 text-start"
            data-testid="conflict-mine"
          >
            <span className="flex flex-col items-start whitespace-normal">
              <span>
                <Trans>Keep mine</Trans>
              </span>
              <span className="text-caption font-normal opacity-85">
                <Trans>Save your version over theirs. Theirs is kept in Versions as “Kept from a conflict”.</Trans>
              </span>
            </span>
          </Button>
          <Button
            variant="outline"
            onClick={onCopy}
            disabled={busy}
            className="h-auto justify-start py-2.5 text-start"
            data-testid="conflict-copy"
          >
            <span className="flex flex-col items-start whitespace-normal">
              <span>
                <Trans>Save mine as a copy</Trans>
              </span>
              <span className="text-caption font-normal text-faint">
                <Trans>Your version becomes a new design; this one keeps theirs.</Trans>
              </span>
            </span>
          </Button>
          <Button
            variant="outline"
            onClick={onTheirs}
            disabled={busy}
            className="h-auto justify-start py-2.5 text-start"
            data-testid="conflict-theirs"
          >
            <span className="flex flex-col items-start whitespace-normal">
              <span>
                <Trans>Load theirs</Trans>
              </span>
              <span className="text-caption font-normal text-faint">
                <Trans>Show their version. Undo brings yours back.</Trans>
              </span>
            </span>
          </Button>
        </div>
        <Button variant="link" size="sm" className="w-fit" onClick={onDownload}>
          <DownloadIcon />
          <Trans>Download my version as JSON</Trans>
        </Button>
      </AlertDialogContent>
    </AlertDialog>
  )
}

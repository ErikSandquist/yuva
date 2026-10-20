import { Trans, useLingui } from "@lingui/react/macro"
import { LoaderIcon } from "lucide-react"

import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { useChangelogPreview } from "@/features/changelog/queries"
import { ErrorLine } from "@/features/changelog/text"
import type { Changelog } from "@/lib/api"

export function PreviewDialog({
  changelog,
  entry,
  locale,
  onClose,
}: {
  changelog: Changelog
  entry?: string
  locale?: string
  onClose: () => void
}) {
  const { t } = useLingui()
  const preview = useChangelogPreview(changelog.id, { entry, locale })
  const html = preview.data ? `<base target="_blank">${preview.data}` : undefined
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        className="h-[calc(100svh-2rem)] grid-rows-[auto_1fr] sm:max-w-4xl"
        data-testid="changelog-preview-dialog"
      >
        <DialogHeader>
          <DialogTitle>
            <Trans>Preview</Trans>
          </DialogTitle>
          <DialogDescription>
            <Trans>The page as visitors would see it, drafts included. Only your team sees this.</Trans>
          </DialogDescription>
        </DialogHeader>
        <div className="relative min-h-0 overflow-hidden rounded-xl border bg-background">
          {preview.isPending && (
            <div className="grid h-full place-items-center text-faint">
              <LoaderIcon className="animate-spin" />
            </div>
          )}
          {preview.error && <ErrorLine error={preview.error} className="p-4" />}
          {html && (
            <iframe
              title={t`Preview`}
              srcDoc={html}
              sandbox="allow-popups allow-popups-to-escape-sandbox"
              className="size-full bg-white"
              data-testid="changelog-preview-frame"
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

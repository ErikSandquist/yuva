import { Trans, useLingui } from "@lingui/react/macro"
import { LoaderIcon } from "lucide-react"

import { ErrorLine } from "@/components/common"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { useStatusPreview } from "@/features/status/queries"
import type { StatusPage } from "@/lib/api"

export function PreviewDialog({ page, onClose }: { page: StatusPage; onClose: () => void }) {
  const { t } = useLingui()
  const preview = useStatusPreview(page.id, true)
  const html = preview.data ? `<base target="_blank">${preview.data}` : undefined
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        className="h-[calc(100svh-2rem)] grid-rows-[auto_1fr] sm:max-w-4xl"
        data-testid="status-preview-dialog"
      >
        <DialogHeader>
          <DialogTitle>
            <Trans>Preview</Trans>
          </DialogTitle>
          <DialogDescription>
            {page.published ? (
              <Trans>The page as visitors see it now. Drafts are left out.</Trans>
            ) : (
              <Trans>The page is not published. Only your team sees this.</Trans>
            )}
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
              title={t`Preview of ${page.name}`}
              srcDoc={html}
              sandbox="allow-popups allow-popups-to-escape-sandbox"
              className="size-full bg-white"
              data-testid="status-preview-frame"
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

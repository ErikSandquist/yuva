import { Trans } from "@lingui/react/macro"
import { useMutation } from "@tanstack/react-query"
import { DownloadIcon } from "lucide-react"

import { ErrorLine } from "@/components/common/ErrorLine"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export type ExportFormat = "csv" | "json" | "iys"

export function ExportButtons<F extends ExportFormat = "csv" | "json">({
  run,
  className,
  testId,
  formats = ["csv", "json"] as readonly ExportFormat[] as readonly F[],
}: {
  run: (format: F) => Promise<void>
  formats?: readonly F[]
  className?: string
  testId?: string
}) {
  const save = useMutation({ mutationFn: run })
  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <div className="flex flex-wrap gap-2">
        {formats.map((f) => (
          <Button
            key={f}
            type="button"
            variant="outline"
            size="sm"
            disabled={save.isPending}
            onClick={() => save.mutate(f)}
            data-testid={testId && `${testId}-${f}`}
          >
            <DownloadIcon />
            {f === "csv" ? (
              <Trans>Download CSV</Trans>
            ) : f === "json" ? (
              <Trans>Download JSON</Trans>
            ) : (
              <Trans>Download for İYS</Trans>
            )}
          </Button>
        ))}
      </div>
      <ErrorLine error={save.error} className="text-small" />
    </div>
  )
}

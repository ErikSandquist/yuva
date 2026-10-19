import { Trans, useLingui } from "@lingui/react/macro"
import { CheckIcon, CopyIcon } from "lucide-react"
import { useState } from "react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export function useCopy() {
  const [copied, setCopied] = useState(false)
  const copy = async (value: string) => {
    await navigator.clipboard.writeText(value)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }
  return [copied, copy] as const
}

export function CopyButton({ value, className }: { value: string; className?: string }) {
  const { t } = useLingui()
  const [copied, copy] = useCopy()
  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      className={className}
      onClick={() => void copy(value)}
      aria-label={t`Copy`}
    >
      {copied ? <CheckIcon /> : <CopyIcon />}
      {copied ? <Trans>Copied</Trans> : <Trans>Copy</Trans>}
    </Button>
  )
}

export function softBreaks(value: string) {
  return value.split(/(?<=\/)|(?=@)/).map((part, i) => (
    <span key={i}>
      {i > 0 && <wbr />}
      {part}
    </span>
  ))
}

export function CodeLine({ value, testId }: { value: string; testId?: string }) {
  return (
    <div className="flex min-w-0 items-center gap-2">
      <code
        className="min-w-0 flex-1 rounded-xl border bg-surface px-3 py-2 font-mono text-caption [overflow-wrap:anywhere]"
        data-testid={testId}
      >
        {softBreaks(value)}
      </code>
      <CopyButton value={value} />
    </div>
  )
}

export function CodeBlock({ value, testId, wrap }: { value: string; testId?: string; wrap?: boolean }) {
  return (
    <div className="flex min-w-0 items-start gap-3 rounded-xl border bg-surface py-2 ps-3.5 pe-2">
      <pre
        className={cn(
          "min-w-0 flex-1 py-1 font-mono text-caption leading-relaxed",
          wrap ? "whitespace-pre-wrap [overflow-wrap:anywhere]" : "overflow-x-auto whitespace-pre"
        )}
        data-testid={testId}
        tabIndex={0}
      >
        {value}
      </pre>
      <CopyButton value={value} className="shrink-0" />
    </div>
  )
}

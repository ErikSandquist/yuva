import { Trans } from "@lingui/react/macro"
import { KeyRoundIcon } from "lucide-react"

import { CodeBlock, CodeLine } from "@/components/common"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import type { StatusMonitorWithToken } from "@/lib/api"

function cronEvery(interval: number) {
  const minutes = Math.max(1, Math.round(interval / 60))
  if (minutes < 60) return `*/${minutes} * * * *`
  const hours = Math.round(minutes / 60)
  return hours < 24 ? `0 */${hours} * * *` : "0 0 * * *"
}

function Snippet({
  title,
  hint,
  value,
  testId,
}: {
  title: React.ReactNode
  hint?: React.ReactNode
  value: string
  testId?: string
}) {
  return (
    <section className="flex min-w-0 flex-col gap-2">
      <h3 className="text-body font-medium">{title}</h3>
      {hint && <p className="text-small text-faint">{hint}</p>}
      <CodeBlock value={value} testId={testId} wrap />
    </section>
  )
}

function HeartbeatSnippets({ url, interval }: { url: string; interval: number }) {
  const curl = `curl -fsS -m 10 --retry 3 -o /dev/null ${url}`
  return (
    <>
      <Snippet title={<Trans>Send a beat</Trans>} value={curl} testId="status-monitor-snippet-curl" />
      <Snippet
        title={<Trans>Cron</Trans>}
        hint={<Trans>Beats only when the job succeeded.</Trans>}
        value={`${cronEvery(interval)} /path/to/job.sh && ${curl}`}
        testId="status-monitor-snippet-cron"
      />
      <Snippet
        title={<Trans>Report a failed run</Trans>}
        hint={<Trans>Fails the monitor at once, without waiting for the grace.</Trans>}
        value={`curl -fsS -m 10 -o /dev/null "${url}?fail=1"`}
        testId="status-monitor-snippet-fail"
      />
    </>
  )
}

function PushSnippets({ url }: { url: string }) {
  const post = (body: string) => `curl -fsS -X POST -H "Content-Type: application/json" -d '${body}' ${url}`
  return (
    <>
      <Snippet
        title={<Trans>Push a state</Trans>}
        hint={<Trans>operational recovers; degraded, partial_outage and major_outage fail.</Trans>}
        value={post('{"state":"major_outage","message":"Checkout is failing"}')}
        testId="status-monitor-snippet-push"
      />
      <Snippet title={<Trans>Push recovery</Trans>} value={post('{"state":"operational"}')} />
      <Snippet
        title={<Trans>Alertmanager</Trans>}
        hint={<Trans>Firing alerts fail the monitor, resolved ones recover it.</Trans>}
        value={`receivers:\n  - name: yuva-status\n    webhook_configs:\n      - url: ${url}\n        send_resolved: true`}
        testId="status-monitor-snippet-alertmanager"
      />
      <section className="flex min-w-0 flex-col gap-1">
        <h3 className="text-body font-medium">Grafana</h3>
        <p className="text-small text-muted-foreground">
          <Trans>
            Add a contact point of type Webhook, paste the address above, keep the method POST and use the contact point
            in a notification policy. Firing alerts fail the monitor, resolved ones recover it.
          </Trans>
        </p>
      </section>
    </>
  )
}

export function MonitorTokenDialog({
  monitor,
  rotated,
  onClose,
}: {
  monitor: StatusMonitorWithToken
  rotated: boolean
  onClose: () => void
}) {
  const name = monitor.name
  const url = monitor.url ?? ""
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-2xl" data-testid="status-monitor-token">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <KeyRoundIcon className="size-5 text-faint" />
            {rotated ? <Trans>New address for {name}</Trans> : <Trans>Set up {name}</Trans>}
          </DialogTitle>
          <DialogDescription>
            {rotated ? (
              <Trans>The old address no longer works. Copy the new one now: it is shown only once.</Trans>
            ) : (
              <Trans>Copy the address now: it is shown only once. Anyone who has it can report for this monitor.</Trans>
            )}
          </DialogDescription>
        </DialogHeader>
        <section className="flex min-w-0 flex-col gap-2">
          <h3 className="text-body font-medium">
            <Trans>Address</Trans>
          </h3>
          <CodeLine value={url} testId="status-monitor-url" />
        </section>
        {monitor.kind === "heartbeat" ? (
          <HeartbeatSnippets url={url} interval={monitor.interval ?? 300} />
        ) : (
          <PushSnippets url={url} />
        )}
        <DialogFooter>
          <Button onClick={onClose} data-testid="status-monitor-token-done">
            <Trans>Done</Trans>
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

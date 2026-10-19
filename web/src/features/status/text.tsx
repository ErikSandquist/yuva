import { useLingui } from "@lingui/react/macro"

import type {
  StatusComponentStateValue,
  StatusIncident,
  StatusIncidentImpact,
  StatusIncidentKind,
  StatusIncidentState,
  StatusCheckSource,
  StatusMonitorFailureState,
  StatusMonitorKind,
  StatusMonitorOnFailure,
  StatusMonitorState,
} from "@/lib/api"
import { cn } from "@/lib/utils"

export const COMPONENT_STATES: StatusComponentStateValue[] = [
  "operational",
  "degraded",
  "partial_outage",
  "major_outage",
  "maintenance",
]

export const FAILURE_STATES: StatusMonitorFailureState[] = ["degraded", "partial_outage", "major_outage"]
export const ON_FAILURE: StatusMonitorOnFailure[] = ["none", "draft", "publish"]
export const MONITOR_KINDS: StatusMonitorKind[] = ["heartbeat", "push"]

export const IMPACTS: StatusIncidentImpact[] = ["none", "minor", "major", "critical"]

export const INCIDENT_STATES: StatusIncidentState[] = ["investigating", "identified", "monitoring", "resolved"]
export const MAINTENANCE_STATES: StatusIncidentState[] = ["scheduled", "in_progress", "completed"]

const stateColor: Record<StatusComponentStateValue, string> = {
  operational: "bg-success",
  degraded: "bg-warning",
  partial_outage: "bg-warning",
  major_outage: "bg-destructive",
  maintenance: "bg-brand",
}

export function StateDot({ state, className }: { state: StatusComponentStateValue; className?: string }) {
  return <span aria-hidden className={cn("inline-block size-2 shrink-0 rounded-full", stateColor[state], className)} />
}

export function isClosed(i: Pick<StatusIncident, "state">) {
  return i.state === "resolved" || i.state === "completed"
}

export function nextStates(i: Pick<StatusIncident, "kind" | "state">): StatusIncidentState[] {
  if (i.kind === "incident") return INCIDENT_STATES
  return i.state === "scheduled" ? MAINTENANCE_STATES : MAINTENANCE_STATES.filter((s) => s !== "scheduled")
}

export function useStatusText() {
  const { t } = useLingui()
  const component: Record<StatusComponentStateValue, string> = {
    operational: t`Operational`,
    degraded: t`Degraded performance`,
    partial_outage: t`Partial outage`,
    major_outage: t`Major outage`,
    maintenance: t`Under maintenance`,
  }
  const impact: Record<StatusIncidentImpact, string> = {
    none: t`No impact`,
    minor: t`Minor`,
    major: t`Major`,
    critical: t`Critical`,
  }
  const incident: Record<StatusIncidentState, string> = {
    investigating: t`Investigating`,
    identified: t`Identified`,
    monitoring: t`Monitoring`,
    resolved: t`Resolved`,
    scheduled: t`Scheduled`,
    in_progress: t`In progress`,
    completed: t`Completed`,
  }
  const kind: Record<StatusIncidentKind, string> = {
    incident: t`Incident`,
    maintenance: t`Maintenance`,
  }
  const monitorKind: Record<StatusMonitorKind, string> = {
    heartbeat: t`Heartbeat`,
    push: t`Pushed state`,
  }
  const monitorState: Record<StatusMonitorState | "paused", string> = {
    pending: t`Waiting for the first result`,
    up: t`Up`,
    down: t`Down`,
    paused: t`Paused`,
  }
  const onFailure: Record<StatusMonitorOnFailure, string> = {
    none: t`Open nothing`,
    draft: t`Open a draft incident`,
    publish: t`Publish an incident`,
  }
  const checkSource: Record<StatusCheckSource, string> = {
    beat: t`Beat`,
    fail: t`Reported failure`,
    missed: t`Missed beat`,
    push: t`Pushed`,
    alertmanager: t`Alertmanager`,
    grafana: t`Grafana`,
  }
  return { component, impact, incident, kind, monitorKind, monitorState, onFailure, checkSource }
}

const URL_RE = /(https?:\/\/[^\s<>"]+[^\s<>".,;:!?)\]])/g

export function PlainText({ text, className }: { text: string; className?: string }) {
  return (
    <p className={cn("text-reading break-words whitespace-pre-wrap", className)}>
      {text.split(URL_RE).map((part, i) =>
        i % 2 === 1 ? (
          <a
            key={i}
            href={part}
            target="_blank"
            rel="noreferrer noopener"
            className="text-brand underline-offset-4 hover:underline"
          >
            {part}
          </a>
        ) : (
          part
        )
      )}
    </p>
  )
}

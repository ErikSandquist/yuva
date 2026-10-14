import { Trans, useLingui } from "@lingui/react/macro"
import { BellIcon, BellRingIcon, CheckCircle2Icon, PencilIcon, PlusIcon, Trash2Icon } from "lucide-react"
import { useState } from "react"

import { ErrorLine, useConfirm } from "@/components/common"
import { formatDateTime } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { useApiRoutes } from "@/features/analytics/queries"
import {
  useAlertRules,
  useChecks,
  useDeleteRule,
  useMonitorAlerts,
  useSaveRule,
  type RuleScope,
} from "@/features/monitoring/queries"
import {
  CHECK_KINDS,
  formatMs,
  hasThreshold,
  hasWindow,
  isCheckKind,
  RULE_KINDS,
  useMonitoringText,
} from "@/features/monitoring/text"
import { Card, ChoiceSelect, EmptyRow, Field, Rows, Section } from "@/features/settings/ui"
import type { MonitorAlert, MonitorAlertRule, MonitorAlertRuleKind } from "@/lib/api"
import { useSession } from "@/lib/session"
import { cn } from "@/lib/utils"
import { useMembers } from "@/lib/workspace"

const ALL = "all"

function useValue() {
  const { i18n } = useLingui()
  return (kind: MonitorAlertRuleKind, v: number | undefined) => {
    if (v === undefined || v === null) return "–"
    switch (kind) {
      case "error_rate":
      case "client_error_rate":
        return new Intl.NumberFormat(i18n.locale, { style: "percent", maximumFractionDigits: 2 }).format(v / 100)
      case "p95":
      case "p99":
        return formatMs(v, i18n.locale)
      case "apdex":
        return new Intl.NumberFormat(i18n.locale, { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(v)
      default:
        return new Intl.NumberFormat(i18n.locale, { maximumFractionDigits: 1 }).format(v)
    }
  }
}

function useSubject() {
  const { t } = useLingui()
  const checks = new Map((useChecks().data?.items ?? []).map((c) => [c.id, c.name]))
  return (x: { kind: MonitorAlertRuleKind; route?: string; service?: string; check_id?: string; key?: string }) => {
    if (x.check_id) return checks.get(x.check_id) ?? t`A deleted check`
    if (isCheckKind(x.kind)) return x.key ? (checks.get(x.key) ?? x.key) : t`Every check`
    if (x.route) return x.route
    if (x.key) return x.key
    const service = x.service
    if (service) return t`Service ${service}`
    return t`The whole API`
  }
}

function RuleLine({ r }: { r: MonitorAlertRule }) {
  const text = useMonitoringText()
  const value = useValue()
  const th = r.threshold
  const unit = r.kind
  if (r.kind === "certificate") {
    const days = th !== undefined ? value(unit, th) : ""
    return days ? <Trans>Within {days} days</Trans> : <Trans>Within the check’s warning days</Trans>
  }
  if (!hasThreshold(r.kind) || th === undefined) return <>{text.kindHelp[r.kind]}</>
  const shown = value(unit, th)
  const window = r.window_minutes
  const min = r.min_requests
  return r.kind === "apdex" ? (
    <Trans>
      Below {shown} over {window} minutes, with at least {min} requests
    </Trans>
  ) : (
    <Trans>
      Above {shown} over {window} minutes, with at least {min} requests
    </Trans>
  )
}

function RuleDialog({
  scope,
  rule,
  api,
  onClose,
}: {
  scope: RuleScope
  rule?: MonitorAlertRule
  api: boolean
  onClose: () => void
}) {
  const { t } = useLingui()
  const text = useMonitoringText()
  const { membership } = useSession()
  const members = useMembers().data ?? []
  const routes = useApiRoutes(scope.property_id ?? "", api && !rule).data ?? []
  const checks = useChecks(scope.property_id).data?.items ?? []
  const save = useSaveRule(rule?.id)
  const kinds = api && !scope.check_id ? RULE_KINDS : CHECK_KINDS
  const [kind, setKind] = useState<MonitorAlertRuleKind>(rule?.kind ?? kinds[0]!)
  const [name, setName] = useState(rule?.name ?? "")
  const [route, setRoute] = useState(rule?.route ?? ALL)
  const [service, setService] = useState(rule?.service ?? "")
  const [check, setCheck] = useState(rule?.check_id ?? scope.check_id ?? ALL)
  const [threshold, setThreshold] = useState(rule?.threshold !== undefined ? String(rule.threshold) : "")
  const [window, setWindowValue] = useState(String(rule?.window_minutes ?? 5))
  const [minRequests, setMinRequests] = useState(String(rule?.min_requests ?? 20))
  const [recipients, setRecipients] = useState<string[]>(rule?.recipients ?? [membership.member_id])
  const [enabled, setEnabled] = useState(rule?.enabled ?? true)
  const check_kind = isCheckKind(kind)
  const needThreshold = hasThreshold(kind) && kind !== "certificate"
  const n = Number(threshold)
  const thresholdOk =
    !hasThreshold(kind) ||
    (kind === "certificate" && threshold === "") ||
    (threshold !== "" &&
      Number.isFinite(n) &&
      (kind === "apdex" ? n > 0 && n < 1 : kind === "certificate" ? Number.isInteger(n) && n >= 1 && n <= 90 : n > 0))
  const windowOk = !hasWindow(kind) || (Number.isInteger(Number(window)) && Number(window) >= 5 && Number(window) <= 60)
  const minOk =
    !hasWindow(kind) ||
    (Number.isInteger(Number(minRequests)) && Number(minRequests) >= 1 && Number(minRequests) <= 1e6)
  const valid = thresholdOk && windowOk && minOk && recipients.length <= 20
  const subject = check_kind
    ? { check_id: check === ALL ? undefined : check, property_id: check === ALL ? scope.property_id : undefined }
    : {
        property_id: scope.property_id,
        route: route === ALL ? undefined : route,
        service: kind !== "silent" && service.trim() ? service.trim() : undefined,
      }
  const unit = text.unit[kind]
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg" data-testid="rule-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            if (!valid) return
            const fields = {
              name: name.trim() || undefined,
              threshold: hasThreshold(kind) ? (threshold === "" ? null : n) : undefined,
              window_minutes: hasWindow(kind) ? Number(window) : undefined,
              min_requests: hasWindow(kind) ? Number(minRequests) : undefined,
              recipients,
              enabled,
            }
            save.mutate(rule ? fields : { ...fields, kind, ...subject }, { onSuccess: onClose })
          }}
        >
          <DialogHeader>
            <DialogTitle>{rule ? <Trans>Change the alert rule</Trans> : <Trans>Add an alert rule</Trans>}</DialogTitle>
          </DialogHeader>
          {rule ? (
            <p className="text-body text-muted-foreground">
              {text.kind[rule.kind]} · {text.kindHelp[rule.kind]}
            </p>
          ) : (
            <Field label={<Trans>When</Trans>} hint={text.kindHelp[kind]} htmlFor="rule-kind">
              <ChoiceSelect
                id="rule-kind"
                label={t`When`}
                value={kind}
                onChange={setKind}
                options={kinds.map((k) => [k, text.kind[k]] as const)}
                className="w-full"
              />
            </Field>
          )}
          {!rule && !check_kind && (
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label={<Trans>Route</Trans>} htmlFor="rule-route">
                <ChoiceSelect
                  id="rule-route"
                  label={t`Route`}
                  value={route}
                  onChange={setRoute}
                  options={[
                    [ALL, kind === "silent" ? t`Each route` : t`Every route`],
                    ...routes.filter((r) => !r.hidden).map((r) => [r.pattern, r.pattern] as const),
                  ]}
                  className="w-full"
                />
              </Field>
              {kind !== "silent" && (
                <Field label={<Trans>Service</Trans>} htmlFor="rule-service" hint={<Trans>Optional.</Trans>}>
                  <Input
                    id="rule-service"
                    value={service}
                    maxLength={100}
                    onChange={(e) => setService(e.target.value)}
                  />
                </Field>
              )}
            </div>
          )}
          {!rule && check_kind && !scope.check_id && (
            <Field label={<Trans>Check</Trans>} htmlFor="rule-check">
              <ChoiceSelect
                id="rule-check"
                label={t`Check`}
                value={check}
                onChange={setCheck}
                options={[
                  [ALL, scope.property_id ? t`Every check of this property` : t`Every check`],
                  ...checks.map((c) => [c.id, c.name] as const),
                ]}
                className="w-full"
              />
            </Field>
          )}
          {hasThreshold(kind) && (
            <div className="grid gap-4 sm:grid-cols-3">
              <Field
                label={<Trans>Threshold</Trans>}
                htmlFor="rule-threshold"
                hint={unit || undefined}
                error={
                  thresholdOk ? undefined : kind === "apdex" ? (
                    <Trans>Between 0 and 1.</Trans>
                  ) : (
                    <Trans>A positive number.</Trans>
                  )
                }
              >
                <Input
                  id="rule-threshold"
                  type="number"
                  step="any"
                  value={threshold}
                  required={needThreshold}
                  placeholder={kind === "certificate" ? t`The check’s` : undefined}
                  onChange={(e) => setThreshold(e.target.value)}
                  data-testid="rule-threshold"
                />
              </Field>
              {hasWindow(kind) && (
                <>
                  <Field
                    label={<Trans>Window, minutes</Trans>}
                    htmlFor="rule-window"
                    error={windowOk ? undefined : <Trans>From 5 to 60.</Trans>}
                  >
                    <Input
                      id="rule-window"
                      type="number"
                      min={5}
                      max={60}
                      value={window}
                      onChange={(e) => setWindowValue(e.target.value)}
                    />
                  </Field>
                  <Field
                    label={<Trans>At least, requests</Trans>}
                    htmlFor="rule-min"
                    error={minOk ? undefined : <Trans>From 1 to 1,000,000.</Trans>}
                  >
                    <Input
                      id="rule-min"
                      type="number"
                      min={1}
                      value={minRequests}
                      onChange={(e) => setMinRequests(e.target.value)}
                    />
                  </Field>
                </>
              )}
            </div>
          )}
          <Field
            label={<Trans>Name</Trans>}
            htmlFor="rule-name"
            hint={<Trans>Optional; named after what it watches.</Trans>}
          >
            <Input id="rule-name" value={name} maxLength={100} onChange={(e) => setName(e.target.value)} />
          </Field>
          <Field
            label={<Trans>Tell</Trans>}
            hint={
              <Trans>
                By e-mail and push notification, when an alert fires and when it resolves. Up to 20 members.
              </Trans>
            }
          >
            <div
              className="flex max-h-44 flex-col gap-2 overflow-y-auto rounded-xl border p-3"
              data-testid="rule-recipients"
            >
              {members.map((m) => (
                <Label key={m.id} className="font-normal">
                  <Checkbox
                    checked={recipients.includes(m.id)}
                    onCheckedChange={(on) =>
                      setRecipients(on ? [...recipients, m.id] : recipients.filter((x) => x !== m.id))
                    }
                  />
                  <span className="min-w-0 truncate">{m.name || m.email}</span>
                </Label>
              ))}
            </div>
          </Field>
          <Label className="font-normal">
            <Switch checked={enabled} onCheckedChange={setEnabled} />
            <Trans>On</Trans>
          </Label>
          <ErrorLine error={save.error} />
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              <Trans>Cancel</Trans>
            </Button>
            <Button type="submit" disabled={!valid || save.isPending} data-testid="rule-save">
              <Trans>Save</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export function AlertRules({ scope, api = false }: { scope: RuleScope; api?: boolean }) {
  const { t } = useLingui()
  const text = useMonitoringText()
  const subject = useSubject()
  const { canManage } = useSession()
  const rules = useAlertRules(scope)
  const remove = useDeleteRule()
  const [ask, confirm] = useConfirm()
  const [editing, setEditing] = useState<MonitorAlertRule | "new" | null>(null)
  const items = rules.data ?? []
  return (
    <Section
      title={<Trans>Alert rules</Trans>}
      description={
        <Trans>
          Checked every minute. An alert fires once when its rule holds and resolves once it has been back to normal for
          a whole window.
        </Trans>
      }
    >
      <Card flush>
        {rules.isPending ? (
          <Skeleton className="m-4 h-10" />
        ) : rules.error ? (
          <ErrorLine error={rules.error} className="p-4" />
        ) : items.length === 0 ? (
          <EmptyRow>
            <Trans>No alert rules yet.</Trans>
          </EmptyRow>
        ) : (
          <Rows data-testid="alert-rules">
            {items.map((r) => (
              <li key={r.id} className="flex items-start gap-3 px-4 py-3" data-testid="alert-rule">
                <span
                  className={cn(
                    "grid size-[34px] shrink-0 place-items-center rounded-[10px] border bg-surface",
                    r.firing > 0 ? "text-destructive" : "text-muted-foreground"
                  )}
                >
                  {r.firing > 0 ? <BellRingIcon className="size-4" /> : <BellIcon className="size-4" />}
                </span>
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="flex flex-wrap items-baseline gap-x-2">
                    <span className="text-body font-medium break-words">{r.name}</span>
                    <span className="text-caption text-faint">{text.kind[r.kind]}</span>
                    {r.firing > 0 && (
                      <span className="rounded-full bg-destructive/10 px-2 text-caption font-medium text-destructive">
                        <Trans>Firing</Trans>
                      </span>
                    )}
                  </span>
                  <span className="text-small text-muted-foreground">
                    <span className="break-all" dir="auto">
                      {subject(r)}
                    </span>
                    {" · "}
                    <RuleLine r={r} />
                  </span>
                </span>
                {canManage ? (
                  <span className="flex shrink-0 items-center gap-1">
                    <RuleToggle rule={r} />
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={t`Change`}
                      onClick={() => setEditing(r)}
                      data-testid="alert-rule-edit"
                    >
                      <PencilIcon />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={t`Delete`}
                      onClick={() =>
                        ask({
                          title: <RuleTitle name={r.name} />,
                          description: <Trans>Its alerts go too. The notes on the charts stay.</Trans>,
                          confirm: <Trans>Delete rule</Trans>,
                          run: () => remove.mutate(r.id),
                        })
                      }
                    >
                      <Trash2Icon />
                    </Button>
                  </span>
                ) : (
                  !r.enabled && (
                    <span className="text-caption text-faint">
                      <Trans>Off</Trans>
                    </span>
                  )
                )}
              </li>
            ))}
          </Rows>
        )}
        {canManage && (
          <div className="border-t p-2">
            <Button variant="ghost" size="sm" onClick={() => setEditing("new")} data-testid="alert-rule-add">
              <PlusIcon />
              <Trans>Add a rule</Trans>
            </Button>
          </div>
        )}
      </Card>
      <ErrorLine error={remove.error} />
      {editing && (
        <RuleDialog
          scope={scope}
          api={api}
          rule={editing === "new" ? undefined : editing}
          onClose={() => setEditing(null)}
        />
      )}
      {confirm}
    </Section>
  )
}

function RuleTitle({ name }: { name: string }) {
  return <Trans>Delete the rule “{name}”?</Trans>
}

function RuleToggle({ rule }: { rule: MonitorAlertRule }) {
  const { t } = useLingui()
  const save = useSaveRule(rule.id)
  return (
    <Switch
      checked={rule.enabled}
      disabled={save.isPending}
      onCheckedChange={(enabled) => save.mutate({ enabled })}
      aria-label={t`Rule on`}
      data-testid="alert-rule-enabled"
    />
  )
}

function AlertSentence({ a }: { a: MonitorAlert }) {
  const value = useValue()
  const v = value(a.kind, a.value)
  const th = value(a.kind, a.threshold)
  const base = value(a.kind, a.baseline)
  switch (a.kind) {
    case "check_state":
      return <Trans>The check was down.</Trans>
    case "certificate":
      return <Trans>The certificate had {v} days left.</Trans>
    case "spike":
      return (
        <Trans>
          {v} requests in 15 minutes, against {base} usually.
        </Trans>
      )
    case "drop":
      return (
        <Trans>
          {v} requests in 15 minutes, against {base} usually.
        </Trans>
      )
    case "silent":
      return <Trans>No requests for 15 minutes, after {base} a minute the day before.</Trans>
    case "apdex":
      return (
        <Trans>
          Apdex was {v}, below {th}.
        </Trans>
      )
    default:
      return (
        <Trans>
          It was {v}, above {th}.
        </Trans>
      )
  }
}

export function AlertHistory({ scope }: { scope: RuleScope }) {
  const { i18n } = useLingui()
  const text = useMonitoringText()
  const subject = useSubject()
  const alerts = useMonitorAlerts(scope)
  const items = alerts.data?.pages.flatMap((x) => x.items) ?? []
  return (
    <Section
      title={<Trans>Alerts</Trans>}
      description={<Trans>Newest first. Resolved alerts are kept 400 days.</Trans>}
    >
      <Card flush>
        {alerts.isPending ? (
          <Skeleton className="m-4 h-10" />
        ) : alerts.error ? (
          <ErrorLine error={alerts.error} className="p-4" />
        ) : items.length === 0 ? (
          <EmptyRow>
            <Trans>No alerts yet.</Trans>
          </EmptyRow>
        ) : (
          <Rows data-testid="monitor-alerts">
            {items.map((a) => {
              const firing = a.state === "firing"
              const fired = formatDateTime(a.fired_at, i18n.locale)
              const resolved = a.resolved_at ? formatDateTime(a.resolved_at, i18n.locale) : ""
              return (
                <li key={a.id} className="flex items-start gap-3 px-4 py-3" data-testid={`monitor-alert-${a.state}`}>
                  <span
                    className={cn(
                      "grid size-[34px] shrink-0 place-items-center rounded-[10px] border bg-surface",
                      firing ? "text-destructive" : "text-success"
                    )}
                  >
                    {firing ? <BellRingIcon className="size-4" /> : <CheckCircle2Icon className="size-4" />}
                  </span>
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="flex flex-wrap items-baseline gap-x-2">
                      <span className="text-body font-medium break-words">{a.rule_name}</span>
                      <span className={cn("text-caption font-medium", firing ? "text-destructive" : "text-success")}>
                        {firing ? <Trans>Firing</Trans> : <Trans>Resolved</Trans>}
                      </span>
                    </span>
                    <span className="text-small break-words text-muted-foreground">
                      {text.kind[a.kind]} · <span dir="auto">{subject(a)}</span> · <AlertSentence a={a} />
                    </span>
                    <span className="text-caption text-faint tabular-nums">
                      {resolved ? (
                        <Trans>
                          Fired {fired}, resolved {resolved}
                        </Trans>
                      ) : (
                        <Trans>Fired {fired}</Trans>
                      )}
                    </span>
                  </span>
                </li>
              )
            })}
          </Rows>
        )}
        {alerts.hasNextPage && (
          <div className="flex justify-center border-t p-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void alerts.fetchNextPage()}
              disabled={alerts.isFetchingNextPage}
            >
              <Trans>Load more</Trans>
            </Button>
          </div>
        )}
      </Card>
    </Section>
  )
}

export function PropertyAlerts({ propertyId, api }: { propertyId: string; api: boolean }) {
  return (
    <div className="flex flex-col gap-6" data-testid="property-monitor-alerts">
      <AlertHistory scope={{ property_id: propertyId }} />
      <AlertRules scope={{ property_id: propertyId }} api={api} />
    </div>
  )
}

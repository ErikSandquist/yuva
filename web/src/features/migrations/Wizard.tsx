import { Trans, useLingui } from "@lingui/react/macro"
import { ArrowDownIcon, MailIcon, ShieldCheckIcon } from "lucide-react"
import { Link, Navigate, useParams, useSearchParams } from "react-router"

import { ErrorLine, YuvaMark } from "@/components/common"
import { formatDate } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { useNumber } from "@/features/broadcasts/BroadcastParts"
import { ChooseStep } from "@/features/migrations/ChooseStep"
import { ConnectStep } from "@/features/migrations/ConnectStep"
import { MapStep } from "@/features/migrations/MapStep"
import { useMigration, usePresets } from "@/features/migrations/queries"
import { SourceStep } from "@/features/migrations/SourceStep"
import { useMigrationText } from "@/features/migrations/text"
import { PreviewCaption } from "@/features/setup/previews"
import { SetupLayout } from "@/features/setup/SetupLayout"
import type { Migration, MigrationPreset } from "@/lib/api"
import { useSession } from "@/lib/session"
import { useInboxes } from "@/lib/workspace"

export const STEPS = ["source", "connect", "choose", "map", "review", "copy", "switch", "done"] as const
export type StepKey = (typeof STEPS)[number]

const BUILT: StepKey[] = ["source", "connect", "choose", "map"]
const EARLY = ["draft", "connected", "scanning", "ready"]

export function currentStep(m: Migration): StepKey {
  switch (m.state) {
    case "draft":
      return "connect"
    case "connected":
      return "choose"
    case "scanning":
      return m.estimate ? "review" : "choose"
    case "ready":
      return m.channel_id ? "review" : "map"
    case "copying":
    case "paused":
      return "copy"
    case "needs_attention":
      return m.started_at ? "copy" : "connect"
    case "syncing":
      return "switch"
    default:
      return "done"
  }
}

function reachable(step: StepKey, m: Migration) {
  if (step === "connect" && m.actions.includes("set_credentials")) return true
  if (!EARLY.includes(m.state)) return step === currentStep(m)
  if (step === "source") return m.state === "draft"
  if (step === "connect") return true
  if (step === "choose" || step === "map") return m.state !== "draft"
  return step === currentStep(m)
}

function useStepNames() {
  const { t } = useLingui()
  return {
    source: t`Source`,
    connect: t`Connect`,
    choose: t`Choose`,
    map: t`Map`,
    review: t`Review`,
    copy: t`Copy`,
    switch: t`Switch`,
    done: t`Done`,
  } satisfies Record<StepKey, string>
}

export function WizardLayout({
  step,
  m,
  preset,
  address,
  children,
}: {
  step: StepKey
  m?: Migration
  preset?: MigrationPreset
  address?: string
  children: React.ReactNode
}) {
  const { t } = useLingui()
  const names = useStepNames()
  const index = STEPS.indexOf(step)
  const prev = index > 0 ? STEPS[index - 1] : undefined
  const back = m && prev && BUILT.includes(prev) && reachable(prev, m) ? `/migrations/${m.id}/${prev}` : undefined
  return (
    <SetupLayout
      step={index + 1}
      steps={STEPS.map((s) => names[s])}
      stepsLabel={t`Migration steps`}
      back={back ?? (step === "source" ? "/settings/migrations" : undefined)}
      leave={
        <Button variant="ghost" size="sm" render={<Link to="/settings/migrations" />} data-testid="migration-close">
          <Trans>Close</Trans>
        </Button>
      }
      preview={<Journey m={m} preset={preset} address={address ?? m?.source_address} />}
    >
      {children}
    </SetupLayout>
  )
}

function Journey({ m, preset, address }: { m?: Migration; preset?: MigrationPreset; address?: string }) {
  const { t, i18n } = useLingui()
  const num = useNumber()
  const text = useMigrationText()
  const inboxes = useInboxes().data ?? []
  const inbox = m?.inbox_id ? inboxes.find((i) => i.id === m.inbox_id) : undefined
  const provider = preset?.name ?? t`Your mail provider`
  const est = m?.estimate
  const since = m?.options.since
  const facts: [string, string][] = []
  if (m) {
    const host = m.credentials?.server.host ?? ""
    facts.push([t`Connection`, host ? t`Connected to ${host}` : t`Not connected yet`])
    facts.push([t`State`, text.state[m.state]])
    facts.push([t`Since`, since ? formatDate(since, i18n.locale) : t`Everything`])
    if (est) {
      const conversations = num(est.conversations)
      const messages = num(est.messages)
      facts.push([t`Dry run`, t`${conversations} conversations, ${messages} messages`])
    }
  }
  return (
    <>
      <div
        className="flex w-full max-w-sm flex-col gap-4 rounded-2xl border bg-card p-5 shadow-[0_24px_60px_-28px_rgb(15_23_42/0.35)]"
        data-testid="migration-journey"
      >
        <div className="flex items-center gap-3">
          <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-surface text-muted-foreground [&_svg]:size-[18px]">
            <MailIcon />
          </span>
          <span className="min-w-0">
            <span className="block text-caption text-faint">{provider}</span>
            <span className="block truncate font-medium">{address || t`Your mailbox`}</span>
          </span>
        </div>
        <ArrowDownIcon className="ms-3 size-4 text-faint" aria-hidden="true" />
        <div className="flex items-center gap-3">
          <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-brand-wash">
            <YuvaMark className="size-5" />
          </span>
          <span className="min-w-0">
            <span className="block text-caption text-faint">
              <Trans>Yuva inbox</Trans>
            </span>
            <span className="block truncate font-medium">{inbox?.name ?? t`Chosen in the Map step`}</span>
          </span>
        </div>
        {facts.length > 0 && (
          <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 border-t pt-4 text-small">
            {facts.map(([k, v]) => (
              <div key={k} className="contents">
                <dt className="text-faint">{k}</dt>
                <dd className="truncate text-end">{v}</dd>
              </div>
            ))}
          </dl>
        )}
        <p className="flex items-center gap-2 border-t pt-4 text-small text-muted-foreground">
          <ShieldCheckIcon className="size-4 shrink-0 text-success" />
          <Trans>Changes made to {provider}: none</Trans>
        </p>
      </div>
      <PreviewCaption>
        <Trans>Yuva only reads the mailbox. History arrives quietly: no notifications, no auto-replies.</Trans>
      </PreviewCaption>
    </>
  )
}

export function NewMigrationPage() {
  const { canManage } = useSession()
  const [params] = useSearchParams()
  if (!canManage) return <Navigate to="/" replace />
  return <SourceStep inboxId={params.get("inbox") ?? undefined} />
}

export function MigrationPage() {
  const { canManage } = useSession()
  const { migrationId, step } = useParams()
  const query = useMigration(migrationId)
  const presets = usePresets()
  if (!canManage) return <Navigate to="/" replace />
  const m = query.data
  const error = query.error ?? presets.error
  if (error)
    return (
      <WizardLayout step="source">
        <ErrorLine error={error} />
        <Button variant="outline" className="mt-6" render={<Link to="/settings/migrations" />}>
          <Trans>Back to the migrations</Trans>
        </Button>
      </WizardLayout>
    )
  if (!m || !presets.data) return <div className="min-h-svh bg-background" />
  const wanted = STEPS.includes(step as StepKey) ? (step as StepKey) : currentStep(m)
  if (!BUILT.includes(wanted)) return <Navigate to="/settings/migrations" replace />
  if (!reachable(wanted, m)) {
    const now = currentStep(m)
    return <Navigate to={BUILT.includes(now) ? `/migrations/${m.id}/${now}` : "/settings/migrations"} replace />
  }
  if (!step) return <Navigate to={`/migrations/${m.id}/${wanted}`} replace />
  const preset = presets.data.find((p) => p.id === m.preset)!
  switch (wanted) {
    case "source":
      return <SourceStep migration={m} />
    case "connect":
      return <ConnectStep key={m.id} m={m} preset={preset} />
    case "choose":
      return <ChooseStep key={m.id} m={m} preset={preset} />
    default:
      return <MapStep key={m.id} m={m} preset={preset} />
  }
}

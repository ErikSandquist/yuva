import { Trans, useLingui } from "@lingui/react/macro"
import { ArrowRightIcon, CheckIcon, InfoIcon, LoaderIcon, SearchIcon } from "lucide-react"
import { useState } from "react"
import { useNavigate } from "react-router"

import { ErrorLine, Notice } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { useCreateMigration, useDetect, usePresets, useUpdateMigration } from "@/features/migrations/queries"
import { usePresetName } from "@/features/migrations/text"
import { WizardLayout } from "@/features/migrations/Wizard"
import { Field } from "@/features/settings/ui"
import { StepHeader } from "@/features/setup/SetupLayout"
import type { Migration, MigrationDetection, MigrationPresetId } from "@/lib/api"
import { cn } from "@/lib/utils"

const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export type ConnectState = { server?: MigrationDetection["server"] }

export function SourceStep({ migration, inboxId }: { migration?: Migration; inboxId?: string }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const presets = usePresets()
  const detect = useDetect()
  const create = useCreateMigration()
  const update = useUpdateMigration(migration?.id ?? "")
  const [address, setAddress] = useState(migration?.source_address ?? "")
  const [preset, setPreset] = useState<MigrationPresetId | null>(migration?.preset ?? null)
  const [found, setFound] = useState<MigrationDetection | null>(null)
  const clean = address.trim()
  const valid = EMAIL.test(clean)
  const list = presets.data ?? []
  const chosen = list.find((p) => p.id === preset)
  const detection = found && found.address === clean ? found : null

  const find = (pick: boolean) => {
    if (!valid) return
    detect.mutate(clean, {
      onSuccess: (d) => {
        setFound(d)
        if (d.available) setPreset((p) => (pick || !p ? d.preset : p))
      },
    })
  }

  const go = () => {
    if (!valid || !chosen?.available) return
    const state: ConnectState = { server: detection?.preset === chosen.id ? detection.server : undefined }
    if (migration) {
      update.mutate(
        { preset: chosen.id, source_address: clean },
        { onSuccess: (m) => navigate(`/migrations/${m.id}/connect`, { state }) }
      )
      return
    }
    create.mutate(
      { preset: chosen.id, source_address: clean, ...(inboxId ? { inbox_id: inboxId } : {}) },
      { onSuccess: (m) => navigate(`/migrations/${m.id}/connect`, { state }) }
    )
  }

  const pending = create.isPending || update.isPending
  return (
    <WizardLayout step="source" m={migration} preset={chosen} address={clean}>
      <StepHeader title={<Trans>Where is your mail now?</Trans>}>
        <Trans>
          Enter the address of the mailbox you answer from today. Yuva finds its provider; nothing is read until you
          connect it.
        </Trans>
      </StepHeader>
      <form
        className="flex flex-col gap-6"
        onSubmit={(e) => {
          e.preventDefault()
          if (detection || chosen) go()
          else find(true)
        }}
      >
        <Field label={<Trans>Mailbox address</Trans>} htmlFor="migration-address">
          <div className="flex gap-2">
            <Input
              id="migration-address"
              type="email"
              autoFocus
              required
              autoComplete="email"
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              onBlur={() => valid && !detection && !detect.isPending && find(false)}
              placeholder={t`support@example.com`}
              className="h-12 rounded-xl px-3.5 text-title md:text-title"
              data-testid="migration-address"
            />
            <Button
              type="button"
              variant="outline"
              size="lg"
              className="h-12"
              onClick={() => find(true)}
              disabled={!valid || detect.isPending}
              aria-label={t`Find the provider`}
              data-testid="migration-detect"
            >
              {detect.isPending ? <LoaderIcon className="animate-spin" /> : <SearchIcon />}
            </Button>
          </div>
        </Field>
        <div role="status" aria-live="polite" className="empty:hidden">
          {detect.isPending ? (
            <p className="text-body text-faint">
              <Trans>Looking up the provider…</Trans>
            </p>
          ) : (
            detection && <DetectionNote d={detection} />
          )}
        </div>
        <ErrorLine error={detect.error} />
        <section className="flex flex-col gap-2.5" aria-labelledby="migration-providers">
          <h2 id="migration-providers" className="text-body font-medium">
            {detection ? <Trans>Not right? Pick your provider</Trans> : <Trans>Or pick your provider</Trans>}
          </h2>
          {presets.isPending ? (
            <Skeleton className="h-40 rounded-2xl" />
          ) : (
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
              {list.map((p) => {
                const on = preset === p.id
                return (
                  <button
                    key={p.id}
                    type="button"
                    disabled={!p.available}
                    aria-pressed={on}
                    onClick={() => setPreset(p.id)}
                    className={cn(
                      "flex min-h-16 flex-col items-start justify-center gap-0.5 rounded-xl border bg-card px-3 py-2.5 text-start text-body transition-colors outline-none hover:border-faint focus-visible:ring-3 focus-visible:ring-ring/30 disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:border-border",
                      on && "border-primary ring-1 ring-primary"
                    )}
                    data-testid={`migration-preset-${p.id}`}
                  >
                    <span className="flex w-full items-center gap-1.5 font-medium">
                      <span className="min-w-0 flex-1">{p.name}</span>
                      {on && <CheckIcon className="size-4 shrink-0 text-brand" aria-hidden="true" />}
                    </span>
                    {!p.available && (
                      <span className="text-caption text-faint">
                        <Trans>Coming later</Trans>
                      </span>
                    )}
                  </button>
                )
              })}
            </div>
          )}
          <ErrorLine error={presets.error} />
        </section>
        <ErrorLine error={create.error ?? update.error} />
        <Button
          type="submit"
          size="lg"
          className="self-start"
          disabled={!valid || !chosen?.available || pending}
          data-testid="migration-source-continue"
        >
          <Trans>Continue</Trans>
          <ArrowRightIcon className="rtl:rotate-180" />
        </Button>
      </form>
    </WizardLayout>
  )
}

function DetectionNote({ d }: { d: MigrationDetection }) {
  const { t } = useLingui()
  const name = usePresetName()(d.preset, d.name)
  const host = d.server?.host ?? ""
  const domain = d.domain
  const how = {
    domain: t`Recognised by the address.`,
    mx: t`Found from the domain's mail servers.`,
    autoconfig: t`Found in the domain's mail settings file.`,
    srv: t`Found from the domain's IMAP record.`,
    guess: t`The server ${host} answered.`,
    none: "",
  }[d.found_by]
  if (d.note === "microsoft_later" || (!d.available && !d.note))
    return (
      <Notice tone="warning" icon={InfoIcon} data-testid="migration-detected">
        <p className="font-medium">{name}</p>
        <p>
          <Trans>
            This mailbox connects with “Sign in with Microsoft”, which comes in a later version. Until then it cannot be
            moved here.
          </Trans>
        </p>
      </Notice>
    )
  if (d.note === "proton")
    return (
      <Notice tone="warning" icon={InfoIcon} data-testid="migration-detected">
        <p className="font-medium">Proton Mail</p>
        <p>
          <Trans>
            Proton Mail is reachable over IMAP only through Proton Mail Bridge on your own computer, which a server
            cannot reach. Moving exported files comes in a later version.
          </Trans>
        </p>
      </Notice>
    )
  if (d.note === "cloudflare")
    return (
      <Notice icon={InfoIcon} data-testid="migration-detected">
        <p>
          <Trans>
            Your domain forwards its mail through Cloudflare Email Routing, so there is no mailbox to read here. Enter
            the address of the mailbox where that mail ends up.
          </Trans>
        </p>
      </Notice>
    )
  if (d.found_by === "none")
    return (
      <Notice icon={InfoIcon} data-testid="migration-detected">
        <p>
          <Trans>
            No mail server was found for {domain}. Pick your provider below, or “Other IMAP server” and enter the server
            on the next step.
          </Trans>
        </p>
      </Notice>
    )
  return (
    <Notice icon={CheckIcon} data-testid="migration-detected">
      <p className="font-medium text-foreground">{name}</p>
      <p>
        {how}
        {host && d.found_by !== "guess" && <span className="font-mono text-small"> {host}</span>}
      </p>
    </Notice>
  )
}

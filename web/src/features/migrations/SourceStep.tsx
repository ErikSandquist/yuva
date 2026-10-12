import { Trans, useLingui } from "@lingui/react/macro"
import { ArrowRightIcon, CheckIcon, InfoIcon, LoaderIcon, SearchIcon } from "lucide-react"
import { useState } from "react"
import { useNavigate } from "react-router"

import { ErrorLine, Notice } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import {
  useCreateMigration,
  useDetect,
  usePresets,
  useReplaceDraft,
  useUpdateMigration,
} from "@/features/migrations/queries"
import { usePresetName } from "@/features/migrations/text"
import { WizardLayout } from "@/features/migrations/Wizard"
import { Field } from "@/features/settings/ui"
import { StepHeader } from "@/features/setup/SetupLayout"
import type { Migration, MigrationDetection, MigrationPreset, MigrationPresetId } from "@/lib/api"
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
  const replace = useReplaceDraft()
  const [address, setAddress] = useState(migration?.source_address ?? "")
  const [preset, setPreset] = useState<MigrationPresetId | null>(migration?.preset ?? null)
  const [found, setFound] = useState<MigrationDetection | null>(null)
  const clean = address.trim()
  const valid = EMAIL.test(clean)
  const list = presets.data ?? []
  const mailboxes = list.filter((p) => p.source !== "file")
  const files = list.filter((p) => p.source === "file")
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
    const open = (m: Migration) => navigate(`/migrations/${m.id}/connect`, { state })
    if (migration && migration.source !== chosen.source) {
      replace.mutate(
        {
          draft: migration,
          body: {
            source: chosen.source,
            preset: chosen.id,
            source_address: clean,
            ...(migration.inbox_id ? { inbox_id: migration.inbox_id } : {}),
            ...(migration.channel_id ? { channel_id: migration.channel_id } : {}),
          },
        },
        { onSuccess: open }
      )
      return
    }
    if (migration) {
      update.mutate(
        { preset: chosen.id, source_address: clean },
        { onSuccess: (m) => navigate(`/migrations/${m.id}/connect`, { state }) }
      )
      return
    }
    create.mutate(
      { source: chosen.source, preset: chosen.id, source_address: clean, ...(inboxId ? { inbox_id: inboxId } : {}) },
      { onSuccess: (m) => navigate(`/migrations/${m.id}/connect`, { state }) }
    )
  }

  const pending = create.isPending || update.isPending || replace.isPending
  const tile = (p: MigrationPreset) => (
    <PresetTile key={p.id} p={p} on={preset === p.id} onPick={() => setPreset(p.id)} />
  )
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
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">{mailboxes.map(tile)}</div>
          )}
          <ErrorLine error={presets.error} />
        </section>
        {files.length > 0 && (
          <section className="flex flex-col gap-2.5" aria-labelledby="migration-files">
            <div>
              <h2 id="migration-files" className="text-body font-medium">
                <Trans>Or upload an export file</Trans>
              </h2>
              <p className="text-small text-faint">
                <Trans>
                  For a very large mailbox, or one Yuva cannot sign in to: Google Takeout, mbox, or .eml or Maildir in a
                  zip. Enter the address its mail was sent to above.
                </Trans>
              </p>
            </div>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">{files.map(tile)}</div>
          </section>
        )}
        <ErrorLine error={create.error ?? update.error ?? replace.error} />
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

function PresetTile({ p, on, onPick }: { p: MigrationPreset; on: boolean; onPick: () => void }) {
  return (
    <button
      type="button"
      disabled={!p.available}
      aria-pressed={on}
      onClick={onPick}
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
            cannot reach. Export the mailbox with Proton Mail's export tool, put the folder in a .zip and upload it as
            an export file below.
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

import { useLingui } from "@lingui/react/macro"

import { useErrorText } from "@/components/common/text"
import {
  ApiError,
  type ChangelogEntryState,
  type ChangelogLabel,
  type ChangelogScheduleFailure,
  type ChangelogTranslationState,
} from "@/lib/api"
import { cn } from "@/lib/utils"

export const LABELS: ChangelogLabel[] = ["new", "improved", "fixed"]

export const LABEL_TONES: Record<ChangelogLabel, string> = {
  new: "border-success/40 bg-success/10 text-success",
  improved: "border-blue-500/40 bg-blue-500/10 text-blue-700 dark:text-blue-300",
  fixed: "border-warning/50 bg-warning/10 text-warning",
}

export function useLabelText(): Record<ChangelogLabel, string> {
  const { t } = useLingui()
  return { new: t`New`, improved: t`Improved`, fixed: t`Fixed` }
}

export function useEntryStateText(): Record<ChangelogEntryState, string> {
  const { t } = useLingui()
  return { draft: t`Draft`, scheduled: t`Scheduled`, published: t`Published` }
}

export function useTranslationStateText(): Record<ChangelogTranslationState, string> {
  const { t } = useLingui()
  return { draft: t`Draft`, published: t`Published`, changed: t`Unpublished changes` }
}

export function useFailureText(): Record<ChangelogScheduleFailure["reason"], string> {
  const { t } = useLingui()
  return {
    lost_right: t`The member who scheduled it may no longer publish.`,
    key_revoked: t`The API key that scheduled it was revoked.`,
    key_scope_lost: t`The API key that scheduled it can no longer publish.`,
    no_default_translation: t`It had no text in the main language.`,
  }
}

export function toLocalInput(d: Date) {
  const p = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`
}

export function useChangelogErrorText() {
  const { t } = useLingui()
  const base = useErrorText()
  return (err: unknown): string => {
    if (err instanceof ApiError) {
      if (err.code === "slug_taken") return t`Another changelog already uses this address, now or before.`
      if (err.code === "changelog_exists") return t`This inbox already has a changelog.`
      if (err.code === "last_translation") return t`This is the entry’s only text. Delete the entry instead.`
      if (err.code === "no_default_translation") return t`Write the entry in the main language before publishing.`
      if (err.code === "too_many_scheduled") return t`A changelog holds at most 50 scheduled entries.`
      if (err.code === "entry_scheduled") return t`This entry is scheduled. A member who may publish does it.`
      if (err.code === "member_session_required") return t`Only owners and admins can do this.`
      if (err.code === "hosted_limit")
        return t`This workspace has as many entries as the server allows. Delete old ones first.`
    }
    return base(err)
  }
}

export function ErrorLine({ error, className }: { error: unknown; className?: string }) {
  const text = useChangelogErrorText()
  if (!error) return null
  return (
    <p role="alert" className={cn("text-body text-destructive", className)}>
      {text(error)}
    </p>
  )
}

export function EntryLabel({ label, className }: { label: ChangelogLabel; className?: string }) {
  const text = useLabelText()
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md border px-1.5 py-0.5 text-caption leading-none font-medium",
        LABEL_TONES[label],
        className
      )}
      data-testid={`label-${label}`}
    >
      {text[label]}
    </span>
  )
}

export function entryTitle(translations: { locale: string; title: string }[], ...prefer: string[]): string {
  for (const l of prefer) {
    const tr = translations.find((x) => x.locale === l)
    if (tr?.title) return tr.title
  }
  return translations[0]?.title ?? ""
}

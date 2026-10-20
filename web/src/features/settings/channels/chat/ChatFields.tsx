import { Trans, useLingui } from "@lingui/react/macro"

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import {
  DISCUSSION_MODES,
  POSITIONS,
  type ChatField,
  type ChatForm,
  type ChatInputResult,
} from "@/features/settings/channels/chat/form"
import { FieldError, FormBlock } from "@/features/settings/channels/parts"
import { Field, ToggleRow } from "@/features/settings/ui"
import type { ChatLauncherPosition, DiscussionMode } from "@/lib/api"
import { Input } from "@/components/ui/input"

type Props = {
  f: ChatForm
  set: (patch: Partial<ChatForm>) => void
  invalid: Extract<ChatInputResult, { ok: false }> | null
  serverError: ChatField | null
}

export function SitesFields({ f, set, invalid, serverError }: Props) {
  const notOrigins = invalid?.bad.join(", ") ?? ""
  const originsError =
    serverError === "allowed_origins" ? (
      <Trans>Use origins such as https://www.example.com, without a path.</Trans>
    ) : invalid && (f.touched || invalid.bad.length > 0) ? (
      invalid.bad.length > 0 ? (
        <Trans>Not an origin: {notOrigins}. Use scheme and host only, such as https://www.example.com.</Trans>
      ) : invalid.tooMany ? (
        <Trans>List at most 20 origins.</Trans>
      ) : (
        <Trans>Add at least one origin.</Trans>
      )
    ) : null
  return (
    <>
      <Field
        label={<Trans>Allowed origins</Trans>}
        htmlFor="chat-origins"
        hint={<Trans>One per line, 1 to 20. Only pages on these origins can use the widget.</Trans>}
      >
        <Textarea
          id="chat-origins"
          rows={3}
          spellCheck={false}
          className="font-mono text-caption"
          value={f.origins}
          placeholder={"https://www.example.com\nhttps://app.example.com"}
          onChange={(e) => set({ origins: e.target.value })}
          aria-invalid={!!originsError || undefined}
          aria-describedby={originsError ? "err-chat-origins" : undefined}
          data-testid="chat-origins"
        />
        <FieldError id="err-chat-origins">{originsError}</FieldError>
      </Field>
    </>
  )
}

export function VisitorsFields({ f, set }: Pick<Props, "f" | "set">) {
  const { t } = useLingui()
  return (
    <>
      <ToggleRow
        title={<Trans>Allow anonymous visitors</Trans>}
        hint={<Trans>Visitors can chat without an identity token from your backend.</Trans>}
      >
        <Switch
          checked={f.allowAnonymous}
          onCheckedChange={(allowAnonymous) => set({ allowAnonymous })}
          aria-label={t`Allow anonymous visitors`}
        />
      </ToggleRow>
      <ToggleRow
        title={<Trans>Ask for an e-mail address when offline</Trans>}
        hint={<Trans>When nobody is available, the widget asks where to send the reply.</Trans>}
      >
        <Switch
          checked={f.askEmailOffline}
          onCheckedChange={(askEmailOffline) => set({ askEmailOffline })}
          aria-label={t`Ask for an e-mail address when offline`}
        />
      </ToggleRow>
    </>
  )
}

export function LookFields({ f, set, serverError }: Pick<Props, "f" | "set" | "serverError">) {
  const { t } = useLingui()
  const positionText: Record<ChatLauncherPosition, string> = { right: t`Bottom right`, left: t`Bottom left` }
  return (
    <>
      <Field
        label={<Trans>Greeting</Trans>}
        htmlFor="chat-greeting"
        hint={<Trans>Shown before the first message. The inbox greeting when empty.</Trans>}
      >
        <Textarea
          id="chat-greeting"
          rows={2}
          maxLength={500}
          value={f.greeting}
          placeholder={t`Hi! How can we help?`}
          onChange={(e) => set({ greeting: e.target.value })}
          aria-invalid={serverError === "greeting" || undefined}
        />
        <FieldError id="err-chat-greeting">
          {serverError === "greeting" && <Trans>Use at most 500 characters.</Trans>}
        </FieldError>
      </Field>
      <div className="grid gap-5 sm:grid-cols-2">
        <Field label={<Trans>Launcher position</Trans>}>
          <Select
            value={f.position}
            onValueChange={(v) => set({ position: v as ChatLauncherPosition })}
            items={positionText}
          >
            <SelectTrigger className="w-full" aria-label={t`Launcher position`}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {POSITIONS.map((x) => (
                <SelectItem key={x} value={x}>
                  {positionText[x]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
        <Field label={<Trans>Launcher color</Trans>} htmlFor="chat-color">
          <div className="flex h-9 items-center gap-2">
            <Switch
              checked={f.customColor}
              onCheckedChange={(customColor) => set({ customColor })}
              aria-label={t`Use a custom launcher color`}
            />
            {f.customColor ? (
              <>
                <Input
                  id="chat-color"
                  type="color"
                  value={f.color}
                  onChange={(e) => set({ color: e.target.value })}
                  className="h-8 w-10 cursor-pointer rounded-lg p-0.5"
                  aria-label={t`Launcher color`}
                />
                <code className="font-mono text-caption text-muted-foreground">{f.color}</code>
              </>
            ) : (
              <span className="text-body text-muted-foreground">
                <Trans>Inbox color</Trans>
              </span>
            )}
          </div>
          <FieldError id="err-chat-color">
            {serverError === "launcher.color" && <Trans>Pick a color.</Trans>}
          </FieldError>
        </Field>
      </div>
    </>
  )
}

export function DiscussionsFields({ f, set }: Pick<Props, "f" | "set">) {
  const { t } = useLingui()
  const modeText: Record<DiscussionMode, string> = { off: t`Off`, moderated: t`Moderated`, open: t`Open` }
  const modeHint: Record<DiscussionMode, React.ReactNode> = {
    off: <Trans>No discussions under the pages of these sites.</Trans>,
    moderated: (
      <Trans>
        A reader's posts wait for a member until one of theirs is approved. Signed-in users of your app publish at once.
      </Trans>
    ),
    open: <Trans>Everyone's posts are published at once, except those of authors you marked as spam.</Trans>,
  }
  return (
    <>
      <Field label={<Trans>Discussions on documentation pages</Trans>} hint={modeHint[f.discussions]}>
        <Select value={f.discussions} onValueChange={(v) => set({ discussions: v as DiscussionMode })} items={modeText}>
          <SelectTrigger
            className="w-full sm:w-64"
            aria-label={t`Discussions on documentation pages`}
            data-testid="chat-discussions"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {DISCUSSION_MODES.map((x) => (
              <SelectItem key={x} value={x} data-testid={`chat-discussions-${x}`}>
                {modeText[x]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
    </>
  )
}

export function UpdatesFields({ f, set }: Pick<Props, "f" | "set">) {
  const { t } = useLingui()
  return (
    <ToggleRow
      title={<Trans>Show the number of new updates on the chat button</Trans>}
      hint={
        <Trans>
          The What’s new count shows on the chat button while no messages are unread. The widget’s Home always shows it.
        </Trans>
      }
    >
      <Switch
        checked={f.announceUpdates}
        onCheckedChange={(announceUpdates) => set({ announceUpdates })}
        aria-label={t`Show the number of new updates on the chat button`}
        data-testid="chat-announce-updates"
      />
    </ToggleRow>
  )
}

export function ChatFields(p: Props) {
  return (
    <FormBlock title={<Trans>Web chat</Trans>}>
      <SitesFields {...p} />
      <VisitorsFields {...p} />
      <LookFields {...p} />
      <DiscussionsFields {...p} />
      <UpdatesFields {...p} />
    </FormBlock>
  )
}

import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { ArrowRightIcon } from "lucide-react"
import { useState } from "react"
import { useLocation, useNavigate } from "react-router"

import { ErrorLine, StepHeader } from "@/components/common"
import { PLATFORMS, useEnumText } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { appChannelInput, appForm, type AppForm } from "@/features/settings/channels/app/form"
import { chatChannelInput, chatErrorField, chatForm, type ChatForm } from "@/features/settings/channels/chat/form"
import { Sending } from "@/features/settings/channels/email/EmailFields"
import {
  emailErrorField,
  emailForm,
  emailInput,
  useEmailFieldErrors,
  type EmailField,
  type EmailForm,
} from "@/features/settings/channels/email/form"
import { FieldError } from "@/features/settings/channels/parts"
import { useInbox } from "@/features/settings/inboxes/queries"
import { Field, ToggleRow } from "@/features/settings/ui"
import { useKindText } from "@/features/setup/ChannelStep"
import { DEFAULT_COLOR, DocsMock, MailMock, PhoneMock, PreviewCaption, SiteMock } from "@/features/setup/previews"
import type { SetupKind, SetupState } from "@/features/setup/setup"
import { SetupLayout } from "@/features/setup/SetupLayout"
import { api, ApiError, unwrap } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

function hostOf(origins: string) {
  for (const line of origins.split(/[\n,]+/)) {
    try {
      const u = new URL(line.trim())
      if (u.protocol === "https:" || u.protocol === "http:") return u.host
    } catch {
      continue
    }
  }
  return "www.example.com"
}

function originOfUrl(url: string) {
  try {
    const u = new URL(url.trim())
    return u.protocol === "https:" || u.protocol === "http:" ? u.origin : null
  } catch {
    return null
  }
}

export function AddChannelStep({ inboxId, kind }: { inboxId: string; kind: SetupKind }) {
  const { t } = useLingui()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const state = (useLocation().state ?? {}) as SetupState
  const { workspaceId: ws } = useSession()
  const inbox = useInbox(inboxId).data
  const kindText = useKindText()
  const enumText = useEnumText()
  const fieldErrors = useEmailFieldErrors()
  const [chat, setChat] = useState<ChatForm>(() => ({ ...chatForm(), allowAnonymous: true }))
  const [email, setEmail] = useState<EmailForm>(() => emailForm())
  const [app, setApp] = useState<AppForm>(() => appForm())
  const [docs, setDocs] = useState({ name: "", url: "", touched: false })
  const docsOrigin = kind === "docs" ? originOfUrl(docs.url) : null
  const name = inbox?.name ?? ""
  const color = inbox?.branding.color ?? DEFAULT_COLOR
  const chatInput = kind === "chat" ? chatChannelInput(chat) : null
  const appInput = kind === "app" ? appChannelInput(app) : null

  const save = useMutation({
    mutationFn: () => {
      if (kind === "docs")
        return unwrap(
          api.POST("/v1/inboxes/{inboxId}/channels", {
            params: { path: { inboxId } },
            body: {
              kind: "chat",
              name: docs.name.trim(),
              chat: { allowed_origins: [docsOrigin!], allow_anonymous: true },
            },
          })
        )
      const typed =
        kind === "email"
          ? { email: emailInput(email) }
          : kind === "chat" && chatInput?.ok
            ? { chat: chatInput.value }
            : { app: appInput! }
      const channelKind = kind as "chat" | "email" | "app"
      const channelName = kind === "email" ? name || kindText.email.title : kindText[kind].title
      return unwrap(
        api.POST("/v1/inboxes/{inboxId}/channels", {
          params: { path: { inboxId } },
          body: { kind: channelKind, name: channelName, ...typed },
        })
      )
    },
    onSuccess: (ch) => {
      void qc.invalidateQueries({ queryKey: keys.channels(ws, inboxId) })
      qc.setQueryData(keys.channel(ws, ch.id), ch)
      navigate(`/setup/${inboxId}/${ch.id}${kind === "docs" ? "?docs" : ""}`, { state })
    },
  })

  const bad = emailErrorField(save.error)
  const badChat = chatErrorField(save.error)
  const taken = save.error instanceof ApiError && save.error.code === "email_address_taken"
  const emailError = (f: EmailField) => (bad === f && !taken ? fieldErrors[f] : undefined)
  const valid =
    kind === "chat"
      ? chatInput?.ok === true
      : kind === "app"
        ? !!appInput
        : kind === "docs"
          ? !!docs.name.trim() && !!docsOrigin
          : email.address.trim() !== ""
  const urlError =
    kind !== "docs" ? null : badChat === "allowed_origins" || (docs.touched && docs.url.trim() && !docsOrigin) ? (
      <Trans>Use the site's address, such as https://docs.example.com.</Trans>
    ) : docs.touched && !docs.url.trim() ? (
      <Trans>Add the site's address.</Trans>
    ) : null

  const notOrigins = chatInput && !chatInput.ok ? chatInput.bad.join(", ") : ""
  const originsError =
    badChat === "allowed_origins" ? (
      <Trans>Use origins such as https://www.example.com, without a path.</Trans>
    ) : chatInput && !chatInput.ok && (chat.touched || chatInput.bad.length > 0) ? (
      chatInput.bad.length > 0 ? (
        <Trans>Not an origin: {notOrigins}. Use scheme and host only, such as https://www.example.com.</Trans>
      ) : chatInput.tooMany ? (
        <Trans>List at most 20 origins.</Trans>
      ) : (
        <Trans>Add at least one origin.</Trans>
      )
    ) : null

  const preview =
    kind === "chat" ? (
      <>
        <SiteMock host={hostOf(chat.origins)} name={name} color={color} />
        <PreviewCaption>
          <Trans>The chat appears in the corner of every page that has the snippet.</Trans>
        </PreviewCaption>
      </>
    ) : kind === "docs" ? (
      <>
        <DocsMock host={docsOrigin ? new URL(docsOrigin).host : "docs.example.com"} color={color} />
        <PreviewCaption>
          <Trans>Under each page: “Was this page helpful?”, then feedback and the page’s discussions.</Trans>
        </PreviewCaption>
      </>
    ) : kind === "email" ? (
      <>
        <MailMock name={name} to={email.address.trim()} />
        <PreviewCaption>
          <Trans>Each new e-mail starts a conversation; replies continue it.</Trans>
        </PreviewCaption>
      </>
    ) : (
      <>
        <PhoneMock name={name} color={color} />
        <PreviewCaption>
          <Trans>The SDKs bring ready-made screens, or a client for your own.</Trans>
        </PreviewCaption>
      </>
    )

  const title = {
    chat: <Trans>Which sites will show the chat?</Trans>,
    email: <Trans>Which address do customers write to?</Trans>,
    app: <Trans>Which apps will use it?</Trans>,
    docs: <Trans>Which documentation site?</Trans>,
  }[kind]
  const lead = {
    chat: <Trans>Only pages on these addresses can open the chat. Enter scheme and host, without a path.</Trans>,
    email: <Trans>Use the support address you already have. Mail sent to it will arrive here.</Trans>,
    app: <Trans>Pick the platforms; the next step shows the lines to add to each app.</Trans>,
    docs: (
      <Trans>
        A chat channel named after the site is created for it, open to anonymous readers, with moderated discussions:
        you approve a reader's first post. Add your dev server's address and change the discussions later in the inbox
        settings.
      </Trans>
    ),
  }[kind]
  const submit = {
    chat: <Trans>Get the snippet</Trans>,
    email: <Trans>Continue</Trans>,
    app: <Trans>Get the SDK lines</Trans>,
    docs: <Trans>Get the snippets</Trans>,
  }[kind]

  return (
    <SetupLayout step={3} back={`/setup/${inboxId}`} preview={preview}>
      <StepHeader title={title}>{lead}</StepHeader>
      <form
        className="flex flex-col gap-6"
        onSubmit={(e) => {
          e.preventDefault()
          setApp((a) => ({ ...a, touched: true }))
          setChat((c) => ({ ...c, touched: true }))
          setDocs((d) => ({ ...d, touched: true }))
          if (valid) save.mutate()
        }}
        data-testid={`setup-add-${kind}`}
      >
        {kind === "chat" && (
          <>
            <Field
              label={<Trans>Website addresses</Trans>}
              htmlFor="setup-origins"
              hint={<Trans>One per line, up to 20.</Trans>}
            >
              <Textarea
                id="setup-origins"
                autoFocus
                rows={3}
                spellCheck={false}
                className="rounded-xl font-mono text-body"
                value={chat.origins}
                placeholder={"https://www.example.com\nhttps://app.example.com"}
                onChange={(e) => setChat((c) => ({ ...c, origins: e.target.value }))}
                aria-invalid={!!originsError || undefined}
                aria-describedby={originsError ? "err-setup-origins" : undefined}
                data-testid="setup-origins"
              />
              <FieldError id="err-setup-origins">{originsError}</FieldError>
            </Field>
            <ToggleRow
              title={<Trans>Allow anonymous visitors</Trans>}
              hint={<Trans>Anyone on your site can write. Turn it off when only signed-in users may chat.</Trans>}
            >
              <Switch
                checked={chat.allowAnonymous}
                onCheckedChange={(allowAnonymous) => setChat((c) => ({ ...c, allowAnonymous }))}
                aria-label={t`Allow anonymous visitors`}
              />
            </ToggleRow>
          </>
        )}
        {kind === "docs" && (
          <>
            <Field label={<Trans>Site name</Trans>} htmlFor="setup-docs-name" hint={<Trans>Shown in the panel.</Trans>}>
              <Input
                id="setup-docs-name"
                autoFocus
                required
                maxLength={200}
                className="h-10 rounded-xl"
                value={docs.name}
                placeholder={t`Acme docs`}
                onChange={(e) => setDocs((d) => ({ ...d, name: e.target.value }))}
                data-testid="setup-docs-name"
              />
            </Field>
            <Field label={<Trans>Site address</Trans>} htmlFor="setup-docs-url">
              <Input
                id="setup-docs-url"
                type="url"
                required
                spellCheck={false}
                className="h-10 rounded-xl font-mono"
                value={docs.url}
                placeholder="https://docs.example.com"
                onChange={(e) => setDocs((d) => ({ ...d, url: e.target.value }))}
                aria-invalid={!!urlError || undefined}
                aria-describedby={urlError ? "err-setup-docs-url" : undefined}
                data-testid="setup-docs-url"
              />
              <FieldError id="err-setup-docs-url">{urlError}</FieldError>
            </Field>
          </>
        )}
        {kind === "email" && (
          <>
            <Field label={<Trans>Support address</Trans>} htmlFor="setup-address">
              <Input
                id="setup-address"
                type="email"
                autoFocus
                required
                maxLength={320}
                className="h-10 rounded-xl"
                value={email.address}
                placeholder="support@example.com"
                onChange={(e) => setEmail((f) => ({ ...f, address: e.target.value }))}
                aria-invalid={taken || bad === "address" || undefined}
                data-testid="setup-address"
              />
              <FieldError id="err-address">
                {taken ? <Trans>Another channel already receives mail at this address.</Trans> : emailError("address")}
              </FieldError>
            </Field>
            <Sending
              f={email}
              set={(patch) => setEmail((f) => ({ ...f, ...patch }))}
              error={emailError}
              taken={taken}
            />
          </>
        )}
        {kind === "app" && (
          <>
            <Field label={<Trans>Platforms</Trans>}>
              <div className="flex gap-5" role="group" aria-label={t`Platforms`}>
                {PLATFORMS.map((p) => (
                  <Label key={p} className="font-normal">
                    <Checkbox
                      checked={app.platforms.includes(p)}
                      onCheckedChange={(on) =>
                        setApp((a) => ({
                          ...a,
                          platforms: on ? [...a.platforms, p] : a.platforms.filter((x) => x !== p),
                        }))
                      }
                      aria-invalid={(app.platforms.length === 0 && app.touched) || undefined}
                      data-testid={`setup-platform-${p}`}
                    />
                    {enumText.platform[p]}
                  </Label>
                ))}
              </div>
              <FieldError id="err-setup-platforms">
                {app.platforms.length === 0 && <Trans>Pick at least one platform.</Trans>}
              </FieldError>
            </Field>
            <ToggleRow
              title={<Trans>Allow anonymous users</Trans>}
              hint={
                <Trans>
                  People who are not signed in to your app can write without an identity token from your backend.
                </Trans>
              }
            >
              <Switch
                checked={app.allowAnonymous}
                onCheckedChange={(allowAnonymous) => setApp((a) => ({ ...a, allowAnonymous }))}
                aria-label={t`Allow anonymous users`}
              />
            </ToggleRow>
          </>
        )}
        {!bad && !badChat && <ErrorLine error={save.error} />}
        <Button type="submit" size="lg" className="self-start" disabled={save.isPending} data-testid="setup-add-submit">
          {submit}
          <ArrowRightIcon className="rtl:rotate-180" />
        </Button>
      </form>
    </SetupLayout>
  )
}

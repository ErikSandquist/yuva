import { Trans, useLingui } from "@lingui/react/macro"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { ArrowRightIcon } from "lucide-react"
import { useRef } from "react"
import { Navigate, useLocation, useNavigate } from "react-router"

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
import { FieldError } from "@/features/settings/channels/parts"
import { useInbox } from "@/features/settings/inboxes/queries"
import { Field, ToggleRow } from "@/features/settings/ui"
import { useKindText } from "@/features/setup/ChannelStep"
import { idempotencyKey, useEnsureInbox, useInboxDraft, useLeaveSetup } from "@/features/setup/draft"
import { DEFAULT_COLOR, DocsMock, PhoneMock, PreviewCaption, SiteMock } from "@/features/setup/previews"
import { isTrimmed, stepNumber, type SetupKind, type SetupState } from "@/features/setup/setup"
import { SetupLayout } from "@/features/setup/SetupLayout"
import { api, unwrap } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"
import { useWizardDraft } from "@/hooks/use-wizard-draft"

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

export function AddChannelStep({ inboxId, kind }: { inboxId: string; kind: Exclude<SetupKind, "email"> }) {
  const { t } = useLingui()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const state = (useLocation().state ?? {}) as SetupState
  const { workspaceId: ws } = useSession()
  const leaveSetup = useLeaveSetup()
  const isNew = inboxId === "new"
  const inbox = useInbox(inboxId).data
  const [inboxDraft, , clearInbox] = useInboxDraft()
  const ensureInbox = useEnsureInbox()
  const [fields, patchFields, clearFields] = useWizardDraft(`channel:${inboxId}:${kind}`, {
    chat: { ...chatForm(), allowAnonymous: true } as ChatForm,
    app: appForm() as AppForm,
    docs: { name: "", url: "", touched: false },
  })
  const { chat, app, docs } = fields
  const setChat = (f: (c: ChatForm) => ChatForm) => patchFields({ chat: f(chat) })
  const setApp = (f: (a: AppForm) => AppForm) => patchFields({ app: f(app) })
  const setDocs = (f: (d: typeof docs) => typeof docs) => patchFields({ docs: f(docs) })
  const kindText = useKindText()
  const enumText = useEnumText()
  const secret = useRef<string>(undefined)
  const docsOrigin = kind === "docs" ? originOfUrl(docs.url) : null
  const name = isNew ? inboxDraft.name : (inbox?.name ?? "")
  const color = inbox?.branding.color ?? DEFAULT_COLOR
  const chatInput = kind === "chat" ? chatChannelInput(chat) : null
  const appInput = kind === "app" ? appChannelInput(app) : null
  const trimmed = isTrimmed(kind, !isNew)
  const leave = () => {
    clearFields()
    if (isNew) leaveSetup()
  }

  const save = useMutation({
    mutationFn: async () => {
      let target = inboxId
      if (isNew) {
        const created = await ensureInbox()
        target = created.id
        secret.current = created.secret ?? secret.current
      }
      const params = (body: unknown) => ({
        path: { inboxId: target },
        header: { "Idempotency-Key": idempotencyKey(fields.idempotencyKey, "channel", [target, body]) },
      })
      if (kind === "docs") {
        const body = {
          kind: "chat" as const,
          name: docs.name.trim(),
          chat: { allowed_origins: [docsOrigin!], allow_anonymous: true },
        }
        return unwrap(api.POST("/v1/inboxes/{inboxId}/channels", { params: params(body), body }))
      }
      const typed = kind === "chat" && chatInput?.ok ? { chat: chatInput.value } : { app: appInput! }
      const body = { kind: kind as "chat" | "app", name: kindText[kind].title, ...typed }
      return unwrap(api.POST("/v1/inboxes/{inboxId}/channels", { params: params(body), body }))
    },
    onSuccess: (ch) => {
      const target = ch.inbox_id
      void qc.invalidateQueries({ queryKey: keys.channels(ws, target) })
      qc.setQueryData(keys.channel(ws, ch.id), ch)
      clearFields()
      if (isNew) clearInbox()
      navigate(`/setup/${target}/${ch.id}${kind === "docs" ? "?docs" : ""}`, {
        state: { ...state, existing: !isNew, secret: secret.current ?? state.secret } satisfies SetupState,
      })
    },
  })

  const badChat = chatErrorField(save.error)
  const valid =
    kind === "chat" ? chatInput?.ok === true : kind === "app" ? !!appInput : !!docs.name.trim() && !!docsOrigin
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
    app: <Trans>Which apps will use it?</Trans>,
    docs: <Trans>Which documentation site?</Trans>,
  }[kind]
  const lead = {
    chat: <Trans>Only pages on these addresses can open the chat. Enter scheme and host, without a path.</Trans>,
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
    app: <Trans>Get the SDK lines</Trans>,
    docs: <Trans>Get the snippets</Trans>,
  }[kind]

  if (isNew && !inboxDraft.name.trim() && !save.isSuccess) return <Navigate to="/setup/new" replace state={state} />

  return (
    <SetupLayout
      step={stepNumber("fields", trimmed)}
      kind={kind}
      trimmed={trimmed}
      onLeave={leave}
      back={isNew ? "/setup/new/pick" : undefined}
      preview={preview}
    >
      <StepHeader title={title}>{lead}</StepHeader>
      <form
        className="flex flex-col gap-6"
        onSubmit={(e) => {
          e.preventDefault()
          patchFields({
            app: { ...app, touched: true },
            chat: { ...chat, touched: true },
            docs: { ...docs, touched: true },
          })
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
        {!badChat && <ErrorLine error={save.error} />}
        <Button type="submit" size="lg" className="self-start" disabled={save.isPending} data-testid="setup-add-submit">
          {submit}
          <ArrowRightIcon className="rtl:rotate-180" />
        </Button>
      </form>
    </SetupLayout>
  )
}

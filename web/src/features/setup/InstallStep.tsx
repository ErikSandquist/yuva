import { Trans, useLingui } from "@lingui/react/macro"
import { useQuery } from "@tanstack/react-query"
import { ArrowRightIcon, KeyRoundIcon } from "lucide-react"
import { useState } from "react"
import { Link, useLocation, useSearchParams } from "react-router"

import { CodeLine, CopyButton, Notice, Segmented } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useInbox } from "@/features/settings/inboxes/queries"
import { LiveChat, useForgetPreviewVisitor } from "@/features/setup/LiveChat"
import { DEFAULT_COLOR, DocsMock, MailMock, PhoneMock, PreviewCaption } from "@/features/setup/previews"
import type { SetupKind, SetupState } from "@/features/setup/setup"
import { SetupLayout, StepHeader } from "@/features/setup/SetupLayout"
import { api, unwrap, useVersion, type Channel, type EmailChannel } from "@/lib/api"
import { keys } from "@/lib/keys"
import { useSession } from "@/lib/session"

const DOCS = "https://github.com/productdevbook/yuva/blob/main/docs"

export function hostOfOrigin(origin?: string) {
  try {
    return origin ? new URL(origin).host : "docs.example.com"
  } catch {
    return origin ?? ""
  }
}

export function useChannel(channelId: string) {
  const { workspaceId: ws } = useSession()
  return useQuery({
    queryKey: keys.channel(ws, channelId),
    queryFn: () => unwrap(api.GET("/v1/channels/{channelId}", { params: { path: { channelId } } })),
  })
}

export function setupKind(ch: Channel, docs = false): SetupKind | null {
  return ch.chat ? (docs ? "docs" : "chat") : ch.email ? "email" : ch.app ? "app" : null
}

export function CodeBlock({ value, label, testId }: { value: string; label: string; testId?: string }) {
  return (
    <div className="min-w-0 overflow-hidden rounded-xl border bg-surface">
      <div className="flex items-center justify-between gap-3 border-b py-1.5 ps-3.5 pe-1.5">
        <span className="text-caption text-faint">{label}</span>
        <CopyButton value={value} className="h-7 border-transparent bg-transparent" />
      </div>
      <pre
        className="max-w-full overflow-x-auto px-3.5 py-3 font-mono text-caption leading-5 break-all whitespace-pre-wrap sm:break-normal sm:whitespace-pre"
        data-testid={testId}
      >
        {value}
      </pre>
    </div>
  )
}

function Numbered({ n, title, children }: { n: number; title: React.ReactNode; children: React.ReactNode }) {
  return (
    <li className="grid grid-cols-[1.75rem_minmax(0,1fr)] gap-x-3">
      <span className="grid size-7 place-items-center rounded-full border text-small font-medium text-muted-foreground">
        {n}
      </span>
      <div className="flex min-w-0 flex-col gap-3 pt-0.5">
        <p className="font-medium">{title}</p>
        {children}
      </div>
    </li>
  )
}

function IdentitySecret({ secret }: { secret: string }) {
  return (
    <Notice icon={KeyRoundIcon} className="text-small" data-testid="setup-identity-secret">
      <p className="font-medium text-foreground">
        <Trans>For signed-in users: the identity secret, shown only now</Trans>
      </p>
      <p>
        <Trans>
          Your backend signs identity tokens with it so customers write as themselves. Keep it on your server; you can
          rotate it in the inbox settings.
        </Trans>
      </p>
      <CodeLine value={secret} />
    </Notice>
  )
}

function ChatInstallSteps({ channel, secret }: { channel: Channel; secret?: string }) {
  const chat = channel.chat!
  const origin = window.location.origin
  const snippet = [
    `<script defer`,
    `  src="${origin}/yuva.js"></script>`,
    `<yuva-chat`,
    `  channel="${chat.public_key}"`,
    `  server="${origin}"`,
    `></yuva-chat>`,
  ].join("\n")
  return (
    <ol className="flex flex-col gap-7">
      <Numbered n={1} title={<Trans>Paste this before the closing body tag of your pages</Trans>}>
        <CodeBlock value={snippet} label="HTML" testId="setup-snippet" />
      </Numbered>
      <Numbered n={2} title={<Trans>Open your site</Trans>}>
        <p className="text-muted-foreground">
          <Trans>The chat bubble appears on these addresses:</Trans>
        </p>
        <ul className="flex flex-wrap gap-1.5">
          {chat.allowed_origins.map((o) => (
            <li key={o} className="rounded-full border bg-card px-2.5 py-0.5 font-mono text-caption">
              {o}
            </li>
          ))}
        </ul>
      </Numbered>
      {secret && <IdentitySecret secret={secret} />}
    </ol>
  )
}

type DocsStack = "html" | "nuxt" | "react"

function docsLines(stack: DocsStack, server: string, key: string) {
  if (stack === "nuxt") {
    return [
      `// npm install useyuva`,
      `// nuxt.config.ts`,
      `export default defineNuxtConfig({`,
      `  modules: ["useyuva/nuxt"],`,
      `  yuva: {`,
      `    server: "${server}",`,
      `    channel: "${key}",`,
      `  },`,
      `})`,
      ``,
      `<!-- your docs page, after the content -->`,
      `<YuvaDiscussions :page-title="page.title" />`,
      `<YuvaPageFeedback :page-title="page.title" />`,
    ].join("\n")
  }
  if (stack === "react") {
    return [
      `// npm install useyuva`,
      `"use client"`,
      `import { YuvaDiscussions, YuvaPageFeedback } from "useyuva/react"`,
      ``,
      `const server = "${server}"`,
      `const channel = "${key}"`,
      ``,
      `export function PageHelp({ title }: { title: string }) {`,
      `  return (`,
      `    <>`,
      `      <YuvaDiscussions channel={channel} server={server} pageTitle={title} />`,
      `      <YuvaPageFeedback channel={channel} server={server} pageTitle={title} />`,
      `    </>`,
      `  )`,
      `}`,
    ].join("\n")
  }
  return [
    `<script src="${server}/yuva-docs.js" defer></script>`,
    ``,
    `<yuva-discussions`,
    `  channel="${key}"`,
    `  server="${server}"`,
    `></yuva-discussions>`,
    `<yuva-page-feedback`,
    `  channel="${key}"`,
    `  server="${server}"`,
    `></yuva-page-feedback>`,
  ].join("\n")
}

function DocsInstallSteps({ channel, secret }: { channel: Channel; secret?: string }) {
  const { t } = useLingui()
  const chat = channel.chat!
  const [stack, setStack] = useState<DocsStack>("html")
  const names: Record<DocsStack, string> = { html: "HTML", nuxt: "Nuxt", react: t`Fumadocs / React` }
  const langs: Record<DocsStack, string> = { html: "HTML", nuxt: "Nuxt", react: "TSX" }
  return (
    <ol className="flex flex-col gap-7">
      <Numbered n={1} title={<Trans>Add the two elements under every docs page</Trans>}>
        <Segmented
          value={stack}
          onChange={setStack}
          label={t`Site generator`}
          items={(["html", "nuxt", "react"] as const).map((s) => ({
            value: s,
            label: names[s],
            testId: `setup-docs-${s}`,
          }))}
          className="self-start"
        />
        <CodeBlock
          value={docsLines(stack, window.location.origin, chat.public_key)}
          label={langs[stack]}
          testId="setup-docs-snippet"
        />
        <a
          href={`${DOCS}/documentation-pages.md`}
          target="_blank"
          rel="noreferrer"
          className="w-fit text-body text-brand underline-offset-4 hover:underline"
          data-testid="setup-docs-guide"
        >
          <Trans>Full guide, with Starlight, VitePress, Docusaurus and the options</Trans>
        </a>
      </Numbered>
      <Numbered n={2} title={<Trans>Decide who may write in discussions</Trans>}>
        <p className="text-muted-foreground" data-testid="setup-docs-discussions">
          <Trans>
            Readers sign in with their e-mail address on this server to write. Discussions start moderated: a reader's
            first post waits for you in Docs › Waiting for approval, later ones publish at once. Turn them off or open
            them to everyone in the inbox settings, under the site's channel.
          </Trans>
        </p>
      </Numbered>
      <Numbered n={3} title={<Trans>Open a page of your docs</Trans>}>
        <p className="text-muted-foreground">
          <Trans>The elements work on these addresses; add your dev server in the inbox settings:</Trans>
        </p>
        <ul className="flex flex-wrap gap-1.5">
          {chat.allowed_origins.map((o) => (
            <li key={o} className="rounded-full border bg-card px-2.5 py-0.5 font-mono text-caption">
              {o}
            </li>
          ))}
        </ul>
      </Numbered>
      {secret && <IdentitySecret secret={secret} />}
    </ol>
  )
}

function EmailInstallSteps({ email }: { email: EmailChannel }) {
  const ingress = `${window.location.origin}/ingress/email`
  return (
    <ol className="flex flex-col gap-7">
      <Numbered n={1} title={<Trans>Send mail for {email.address} to Yuva</Trans>}>
        <p className="text-muted-foreground">
          <Trans>
            With Cloudflare Email Routing, deploy the Email Worker from edge/ of the Yuva repository and add a routing
            rule for the address with the action Send to a Worker. Any other mail server can post each message, signed
            with YUVA_INGRESS_SECRET, to:
          </Trans>
        </p>
        <CodeLine value={ingress} testId="setup-ingress" />
        <a
          href={`${DOCS}/email.md#inbound`}
          target="_blank"
          rel="noreferrer"
          className="w-fit text-body text-brand underline-offset-4 hover:underline"
        >
          <Trans>Step-by-step guide</Trans>
        </a>
      </Numbered>
      <Numbered n={2} title={<Trans>Replies</Trans>}>
        {email.smtp ? (
          <p className="text-muted-foreground">
            <Trans>
              Replies go out through {email.smtp.host}. Add the SPF and DKIM records your mail provider gives you, so
              they reach the inbox and not spam.
            </Trans>
          </p>
        ) : (
          <Notice tone="warning" className="text-small">
            <Trans>
              No SMTP account yet: mail arrives, but replies cannot go out until you add one in the inbox settings.
            </Trans>
          </Notice>
        )}
        <a
          href={`${DOCS}/email.md#dns-checklist`}
          target="_blank"
          rel="noreferrer"
          className="w-fit text-body text-brand underline-offset-4 hover:underline"
        >
          <Trans>DNS checklist</Trans>
        </a>
      </Numbered>
    </ol>
  )
}

type Platform = "ios" | "android" | "js"

function sdkLines(platform: Platform, server: string, key: string, version: string | undefined) {
  const release = version && /^\d+\.\d+\.\d+$/.test(version) ? version : null
  if (platform === "ios") {
    return [
      `// Package.swift`,
      `.package(`,
      `    url: "https://github.com/productdevbook/yuva.git",`,
      `    ${release ? `exact: "${release}"` : `branch: "main"`}`,
      `),`,
      `.product(name: "YuvaKit", package: "yuva"),`,
      ``,
      `import YuvaKit`,
      ``,
      `let yuva = YuvaClient(configuration: YuvaConfiguration(`,
      `    serverURL: URL(string: "${server}")!,`,
      `    channelKey: "${key}"`,
      `))`,
    ].join("\n")
  }
  if (platform === "android") {
    return [
      `// settings.gradle.kts: maven("https://jitpack.io")`,
      `implementation("com.github.productdevbook:yuva:${release ? `v${release}` : "main-SNAPSHOT"}")`,
      ``,
      `val yuva = YuvaClient(`,
      `    context,`,
      `    YuvaConfiguration(`,
      `        serverUrl = "${server}",`,
      `        channelKey = "${key}",`,
      `    ),`,
      `)`,
    ].join("\n")
  }
  return [
    `// npm install useyuva`,
    `import { createYuvaClient } from "useyuva"`,
    ``,
    `const yuva = createYuvaClient({`,
    `  server: "${server}",`,
    `  channel: "${key}",`,
    `})`,
    `await yuva.connect()`,
  ].join("\n")
}

function AppInstallSteps({ channel, secret }: { channel: Channel; secret?: string }) {
  const { t } = useLingui()
  const app = channel.app!
  const version = useVersion().data?.version
  const platforms: Platform[] = [...app.platforms, "js"]
  const [platform, setPlatform] = useState<Platform>(platforms[0]!)
  const names: Record<Platform, string> = { ios: "iOS", android: "Android", js: "JavaScript" }
  const langs: Record<Platform, string> = { ios: "Swift", android: "Kotlin", js: "TypeScript" }
  return (
    <ol className="flex flex-col gap-7">
      <Numbered n={1} title={<Trans>Add the SDK and create one client</Trans>}>
        <Segmented
          value={platform}
          onChange={setPlatform}
          label={t`Platform`}
          items={platforms.map((p) => ({ value: p, label: names[p], testId: `setup-sdk-${p}` }))}
          className="self-start"
        />
        <CodeBlock
          value={sdkLines(platform, window.location.origin, app.public_key, version)}
          label={langs[platform]}
          testId="setup-sdk"
        />
        <a
          href={`${DOCS}/mobile.md`}
          target="_blank"
          rel="noreferrer"
          className="w-fit text-body text-brand underline-offset-4 hover:underline"
        >
          <Trans>Full guide, with the screens and push notifications</Trans>
        </a>
      </Numbered>
      <Numbered n={2} title={<Trans>Show the conversation screen</Trans>}>
        <p className="text-muted-foreground">
          <Trans>Put the SDK's conversations view behind a Help button in your app, then send a message from it.</Trans>
        </p>
      </Numbered>
      {secret && <IdentitySecret secret={secret} />}
    </ol>
  )
}

export function InstallStep({ inboxId, channelId }: { inboxId: string; channelId: string }) {
  const state = (useLocation().state ?? {}) as SetupState
  const inbox = useInbox(inboxId).data
  const channel = useChannel(channelId).data
  const docs = useSearchParams()[0].has("docs")
  const kind = channel ? setupKind(channel, docs) : null
  const name = inbox?.name ?? ""
  const color = inbox?.branding.color ?? DEFAULT_COLOR
  useForgetPreviewVisitor(channel?.chat?.public_key, `/setup/${inboxId}/${channelId}`)
  const preview = !channel ? null : kind === "docs" ? (
    <>
      <DocsMock host={hostOfOrigin(channel.chat!.allowed_origins[0])} color={color} />
      <PreviewCaption>
        <Trans>Under each page: “Was this page helpful?”, then feedback and the page’s discussions.</Trans>
      </PreviewCaption>
    </>
  ) : kind === "chat" ? (
    <>
      <LiveChat publicKey={channel.chat!.public_key} />
      <PreviewCaption>
        <Trans>This is the live chat, as it looks on your site. You can try it here.</Trans>
      </PreviewCaption>
    </>
  ) : kind === "email" ? (
    <>
      <MailMock name={name} to={channel.email!.address} />
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
    chat: <Trans>Add the chat to your site</Trans>,
    email: <Trans>Route your mail to Yuva</Trans>,
    app: <Trans>Add Yuva to your app</Trans>,
    docs: <Trans>Add Yuva to your documentation</Trans>,
  }
  return (
    <SetupLayout step={3} preview={preview}>
      {!channel || !kind ? (
        <div className="flex flex-col gap-4">
          <Skeleton className="h-8 w-3/4" />
          <Skeleton className="h-24 w-full" />
        </div>
      ) : (
        <>
          <StepHeader title={title[kind]}>
            {kind === "email" ? (
              <Trans>Yuva does not run a mail server: your domain's mail is passed to it.</Trans>
            ) : (
              <Trans>
                Copy the lines below. They already contain this server's address and the channel's public key, which is
                not a secret.
              </Trans>
            )}
          </StepHeader>
          <div data-testid={`setup-install-${kind}`}>
            {kind === "chat" && <ChatInstallSteps channel={channel} secret={state.secret} />}
            {kind === "email" && <EmailInstallSteps email={channel.email!} />}
            {kind === "app" && <AppInstallSteps channel={channel} secret={state.secret} />}
            {kind === "docs" && <DocsInstallSteps channel={channel} secret={state.secret} />}
          </div>
          <Button
            size="lg"
            className="mt-9"
            render={<Link to={`/setup/${inboxId}/${channelId}/wait${docs ? "?docs" : ""}`} state={state} />}
            data-testid="setup-install-continue"
          >
            <Trans>Continue</Trans>
            <ArrowRightIcon className="rtl:rotate-180" />
          </Button>
        </>
      )}
    </SetupLayout>
  )
}

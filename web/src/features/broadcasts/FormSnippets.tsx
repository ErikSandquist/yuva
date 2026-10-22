import { Trans, useLingui } from "@lingui/react/macro"
import { EyeOffIcon, MessageSquareIcon } from "lucide-react"
import { useState } from "react"
import { Link } from "react-router"

import { CodeBlock, Notice, Segmented } from "@/components/common"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { listLink, useInboxChannels } from "@/features/broadcasts/queries"
import { Card, Field, Section } from "@/features/settings/ui"
import type { MailingList } from "@/lib/api"
import { useInboxes } from "@/lib/workspace"

type Stack = "html" | "react" | "nuxt" | "shadcn" | "api"

const STACKS: Stack[] = ["html", "react", "nuxt", "shadcn", "api"]

export function subscribeLines(stack: Stack, server: string, key: string, listId: string, nameRequired: boolean) {
  const name = nameRequired ? [`  name="required"`] : []
  if (stack === "react") {
    return [
      `// npm install useyuva`,
      `"use client"`,
      `import { YuvaSubscribe } from "useyuva/react"`,
      ``,
      `export function Newsletter() {`,
      `  return (`,
      `    <YuvaSubscribe`,
      `      channel="${key}"`,
      `      server="${server}"`,
      `      lists={["${listId}"]}`,
      ...(nameRequired ? [`      name="required"`] : []),
      `    />`,
      `  )`,
      `}`,
    ].join("\n")
  }
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
      `<!-- any page or component -->`,
      `<YuvaSubscribe :lists="['${listId}']"${nameRequired ? ` name="required"` : ""} />`,
    ].join("\n")
  }
  if (stack === "api") {
    return [
      `const base = "${server}/client/v1/channels/${key}"`,
      ``,
      `const { form_token } = await (await fetch(base + "/lists")).json()`,
      ``,
      `await fetch(base + "/subscriptions", {`,
      `  method: "POST",`,
      `  headers: { "Content-Type": "application/json" },`,
      `  body: JSON.stringify({`,
      `    email: "name@example.com",`,
      `    list_ids: ["${listId}"],`,
      `    page_url: location.href,`,
      `    consent_text: "I want to get e-mails from this list.",`,
      `    form_token,`,
      `    website: "",`,
      `  }),`,
      `})`,
    ].join("\n")
  }
  if (stack === "shadcn") {
    return [
      `npx shadcn@latest add https://useyuva.com/r/yuva-subscribe.json`,
      ``,
      `import { YuvaSubscribe } from "@/components/yuva-subscribe"`,
      ``,
      `<YuvaSubscribe`,
      `  server="${server}"`,
      `  channel="${key}"`,
      `  lists={["${listId}"]}`,
      ...(nameRequired ? [`  name="required"`] : []),
      `/>`,
    ].join("\n")
  }
  return [
    `<script src="${server}/yuva-subscribe.js" defer></script>`,
    ``,
    `<yuva-subscribe`,
    `  channel="${key}"`,
    `  server="${server}"`,
    `  lists="${listId}"`,
    ...name,
    `></yuva-subscribe>`,
  ].join("\n")
}

export function FormSnippets({ list }: { list: MailingList }) {
  const { t } = useLingui()
  const inbox = (useInboxes().data ?? []).find((i) => i.id === list.inbox_id)
  const channels = useInboxChannels(list.inbox_id)
  const chats = (channels.data ?? []).filter((c) => c.kind === "chat" && c.chat)
  const [picked, setPicked] = useState("")
  const [stack, setStack] = useState<Stack>("html")
  const channel = chats.find((c) => c.id === picked) ?? chats[0]
  const names: Record<Stack, string> = { html: "HTML", react: "React", nuxt: "Nuxt", shadcn: "shadcn/ui", api: "API" }
  const items = Object.fromEntries(chats.map((c) => [c.id, c.name]))
  return (
    <Section
      title={<Trans>Subscribe form</Trans>}
      description={
        <Trans>
          The form shows an unticked consent box and stores the exact words the person agreed to. It works only on the
          sites the chat channel allows; the channel key is public.
        </Trans>
      }
    >
      {!list.public && (
        <Notice tone="warning" icon={EyeOffIcon} data-testid="form-private">
          <p>
            <Trans>This list is private, so forms do not offer it. Make it public in the settings to use a form.</Trans>
          </p>
          <Link to={listLink(list.id, "settings")} className="w-fit text-brand underline-offset-4 hover:underline">
            <Trans>Open the settings</Trans>
          </Link>
        </Notice>
      )}
      {channels.isPending ? (
        <Skeleton className="h-48 rounded-2xl" />
      ) : !channel ? (
        <Notice icon={MessageSquareIcon} data-testid="form-no-chat">
          <p>
            <Trans>
              Forms run on a website chat channel of this inbox, which lists the sites they may be used on. This inbox
              has none yet.
            </Trans>
          </p>
          <Link
            to={`/settings/inboxes/${list.inbox_id}`}
            className="w-fit text-brand underline-offset-4 hover:underline"
          >
            <Trans>Add a website chat in the inbox settings</Trans>
          </Link>
        </Notice>
      ) : (
        <Card>
          {chats.length > 1 && (
            <Field label={<Trans>Website chat channel</Trans>}>
              <Select value={channel.id} items={items} onValueChange={(v) => v && setPicked(String(v))}>
                <SelectTrigger className="w-full min-w-0 sm:max-w-80" aria-label={t`Website chat channel`}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {chats.map((c) => (
                    <SelectItem key={c.id} value={c.id}>
                      {c.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
          )}
          <div className="flex flex-col gap-2">
            <p className="text-body text-muted-foreground">
              <Trans>The form works on these addresses:</Trans>
            </p>
            <ul className="flex flex-wrap gap-1.5">
              {channel.chat!.allowed_origins.map((o) => (
                <li key={o} className="rounded-full border bg-surface px-2.5 py-0.5 font-mono text-caption" dir="ltr">
                  {o}
                </li>
              ))}
            </ul>
          </div>
          <div className="max-w-full overflow-x-auto">
            <Segmented
              value={stack}
              onChange={setStack}
              label={t`Site generator`}
              items={STACKS.map((s) => ({ value: s, label: names[s], testId: `form-stack-${s}` }))}
            />
          </div>
          <div dir="ltr">
            <CodeBlock
              value={subscribeLines(
                stack,
                window.location.origin,
                channel.chat!.public_key,
                list.id,
                inbox?.default_locale === "tr"
              )}
              testId="form-snippet"
            />
          </div>
        </Card>
      )}
    </Section>
  )
}

import { Trans, useLingui } from "@lingui/react/macro"
import { useState } from "react"

import { CodeBlock, Segmented } from "@/components/common"
import type { AnalyticsProperty } from "@/lib/api"

type Tab = "html" | "react" | "nuxt" | "other" | "optout" | "consent" | "domain"

const TABS: Tab[] = ["html", "react", "nuxt", "other", "optout", "consent", "domain"]

export function snippet(tab: Tab, server: string, key: string, optOut: string, optIn: string) {
  switch (tab) {
    case "html":
      return `<script defer data-key="${key}" src="${server}/yuva-analytics.js"></script>`
    case "react":
      return [
        `// npm install useyuva`,
        `// app/layout.tsx in Next.js, or your root component`,
        `import { YuvaAnalytics } from "useyuva/react"`,
        ``,
        `export default function RootLayout({ children }: { children: React.ReactNode }) {`,
        `  return (`,
        `    <html>`,
        `      <body>`,
        `        {children}`,
        `        <YuvaAnalytics server="${server}" publicKey="${key}" />`,
        `      </body>`,
        `    </html>`,
        `  )`,
        `}`,
      ].join("\n")
    case "nuxt":
      return [
        `// npm install useyuva`,
        `// nuxt.config.ts`,
        `export default defineNuxtConfig({`,
        `  modules: ["useyuva/nuxt"],`,
        `  yuva: {`,
        `    server: "${server}",`,
        `    analytics: { key: "${key}" },`,
        `  },`,
        `})`,
      ].join("\n")
    case "other":
      return [
        `// npm install useyuva`,
        `// Vue: main.ts. Astro: a <script> in your layout. Anything else: your entry file.`,
        `import { createAnalytics } from "useyuva/analytics"`,
        ``,
        `createAnalytics({ server: "${server}", key: "${key}" })`,
      ].join("\n")
    case "optout":
      return [`<a href="#yuva-optout">${optOut}</a>`, `<a href="#yuva-optin">${optIn}</a>`].join("\n")
    case "consent":
      return [
        `<script defer data-key="${key}" data-consent="required" src="${server}/yuva-analytics.js"></script>`,
        ``,
        `<script>`,
        `  // in your consent tool's callback, once the visitor agreed to analytics`,
        `  window.yuva.consent({ analytics: true })`,
        `  // before the script has loaded, queue the call instead`,
        `  ;(window.yuvaq = window.yuvaq || []).push(["consent", { analytics: true }])`,
        `</script>`,
      ].join("\n")
    case "domain":
      return [
        `# nginx, in your site's server block`,
        `location = /yuva-analytics.js {`,
        `  proxy_pass ${server}/yuva-analytics.js;`,
        `  proxy_ssl_server_name on;`,
        `}`,
        `location /client/v1/analytics/ {`,
        `  proxy_pass ${server}/client/v1/analytics/;`,
        `  proxy_ssl_server_name on;`,
        `  proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`,
        `}`,
        ``,
        `<script defer data-key="${key}" src="/yuva-analytics.js"></script>`,
      ].join("\n")
  }
}

function Hint({ tab }: { tab: Tab }) {
  switch (tab) {
    case "html":
      return (
        <Trans>
          Put the script in the head of every page. It counts page views, also when a single-page app changes the
          address, and stores nothing on the visitor’s device.
        </Trans>
      )
    case "react":
      return (
        <Trans>
          Render the component once, in the root layout. It renders nothing; useYuvaAnalytics() gives the methods
          anywhere below it.
        </Trans>
      )
    case "nuxt":
      return (
        <Trans>
          The module adds a client plugin and the useYuvaAnalytics() composable. NUXT_PUBLIC_YUVA_ANALYTICS_KEY
          overrides the key at runtime.
        </Trans>
      )
    case "other":
      return <Trans>Call it once in the browser. During server rendering it does nothing.</Trans>
    case "optout":
      return (
        <Trans>
          Put these links on your privacy page. Opting out is the only thing the script stores on the visitor’s device,
          and only when they ask for it. Visitors with Do Not Track or Global Privacy Control are never counted.
        </Trans>
      )
    case "consent":
      return (
        <Trans>
          With data-consent="required" the script sends nothing until your consent tool says the visitor agreed. Set
          Collection to “Only with consent” in the general settings too, so the server refuses events without it.
        </Trans>
      )
    case "domain":
      return (
        <Trans>
          Content blockers often stop scripts from other domains. Serving the script and its endpoint from your own site
          keeps those visitors counted. The Yuva server has to trust your proxy’s forwarded address; otherwise all
          visitors look like one.
        </Trans>
      )
  }
}

export function InstallSnippets({ property }: { property: AnalyticsProperty }) {
  const { t } = useLingui()
  const [tab, setTab] = useState<Tab>("html")
  const names: Record<Tab, string> = {
    html: "HTML",
    react: "React, Next.js",
    nuxt: "Nuxt",
    other: t`Vue, Astro, other`,
    optout: t`Opt-out link`,
    consent: t`Consent mode`,
    domain: t`Own domain`,
  }
  return (
    <div className="flex min-w-0 flex-col gap-3" data-testid="analytics-install">
      <div className="-mx-1 max-w-[calc(100%+0.5rem)] overflow-x-auto px-1 py-1">
        <Segmented
          value={tab}
          onChange={setTab}
          label={t`How to install`}
          items={TABS.map((v) => ({ value: v, label: names[v], testId: `install-tab-${v}` }))}
        />
      </div>
      <p className="text-body text-muted-foreground">
        <Hint tab={tab} />
      </p>
      <div dir="ltr">
        <CodeBlock
          value={snippet(
            tab,
            window.location.origin,
            property.public_key,
            t`Do not count my visits`,
            t`Count my visits again`
          )}
          testId="install-snippet"
        />
      </div>
    </div>
  )
}

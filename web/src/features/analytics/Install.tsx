import { Trans, useLingui } from "@lingui/react/macro"
import { InfoIcon } from "lucide-react"
import { useState } from "react"
import { Link, useSearchParams } from "react-router"

import { CodeBlock, Notice, Segmented } from "@/components/common"
import { hasApps } from "@/features/analytics/text"
import { useVersion, type AnalyticsProperty } from "@/lib/api"
import { useSession } from "@/lib/session"

type Tab =
  | "html"
  | "react"
  | "nuxt"
  | "other"
  | "swift"
  | "kotlin"
  | "go"
  | "curl"
  | "events"
  | "optout"
  | "consent"
  | "domain"

const WEB: Tab[] = ["html", "react", "nuxt", "other"]
const APPS: Tab[] = ["swift", "kotlin"]
const SERVER: Tab[] = ["go", "curl"]
const WEB_MORE: Tab[] = ["events", "optout", "consent", "domain"]

function tabsFor(p: AnalyticsProperty): Tab[] {
  const web = p.platforms.includes("web")
  return [...(web ? WEB : []), ...APPS, ...SERVER, ...(web ? WEB_MORE : [])]
}

type Values = {
  server: string
  key: string
  id: string
  page: string
  release: string | null
  optOut: string
  optIn: string
}

function examplePage(p: AnalyticsProperty) {
  const host = p.domains.find((d) => !d.startsWith("*.")) ?? p.domains[0]?.replace(/^\*\./, "")
  if (p.platforms.includes("web") || !hasApps(p)) return `https://${host ?? "example.com"}/signup`
  return "app://com.example.app/Signup"
}

export function snippet(tab: Tab, v: Values) {
  const { server, key, optOut, optIn } = v
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
    case "swift":
      return [
        `// Package.swift`,
        `.package(`,
        `    url: "https://github.com/productdevbook/yuva",`,
        `    ${v.release ? `exact: "${v.release}"` : `branch: "main"`}`,
        `),`,
        `.product(name: "YuvaAnalytics", package: "yuva"),`,
        ``,
        `import SwiftUI`,
        `import YuvaAnalytics`,
        ``,
        `@main`,
        `struct MyApp: App {`,
        `    init() {`,
        `        YuvaAnalytics.configure(YuvaAnalyticsConfiguration(`,
        `            serverURL: URL(string: "${server}")!,`,
        `            key: "${key}"`,
        `        ))`,
        `    }`,
        ``,
        `    var body: some Scene {`,
        `        WindowGroup {`,
        `            HomeView()`,
        `                .yuvaScreen("Home")`,
        `        }`,
        `    }`,
        `}`,
        ``,
        `// UIKit: YuvaAnalytics.screen("Settings") in viewDidAppear`,
        `YuvaAnalytics.track(`,
        `    "Purchase",`,
        `    props: ["plan": "pro"],`,
        `    revenue: YuvaRevenue(amount: 9.99, currency: "EUR")`,
        `)`,
      ].join("\n")
    case "kotlin":
      return [
        `// settings.gradle.kts: maven("https://jitpack.io")`,
        `implementation("com.github.productdevbook:yuva-analytics:${v.release ? `v${v.release}` : "main-SNAPSHOT"}")`,
        ``,
        `import dev.yuva.analytics.TrackScreen`,
        `import dev.yuva.analytics.YuvaAnalytics`,
        `import dev.yuva.analytics.YuvaAnalyticsConfiguration`,
        `import dev.yuva.analytics.YuvaRevenue`,
        ``,
        `class MyApp : Application() {`,
        `    override fun onCreate() {`,
        `        super.onCreate()`,
        `        YuvaAnalytics.configure(`,
        `            this,`,
        `            YuvaAnalyticsConfiguration(serverUrl = "${server}", key = "${key}"),`,
        `        )`,
        `    }`,
        `}`,
        ``,
        `@Composable`,
        `fun HomeScreen() {`,
        `    TrackScreen("Home")`,
        `    // ...`,
        `}`,
        ``,
        `YuvaAnalytics.track(`,
        `    "Purchase",`,
        `    props = mapOf("plan" to "pro"),`,
        `    revenue = YuvaRevenue(BigDecimal("9.99"), "EUR"),`,
        `)`,
      ].join("\n")
    case "go":
      return [
        `// go get github.com/productdevbook/yuva/sdk/go/analytics`,
        `import "github.com/productdevbook/yuva/sdk/go/analytics"`,
        ``,
        `yuva, err := analytics.New(analytics.Config{`,
        `\tBaseURL:    "${server}",`,
        `\tAPIKey:     os.Getenv("YUVA_API_KEY"), // a key with analytics:write`,
        `\tPropertyID: "${v.id}",`,
        `})`,
        `if err != nil {`,
        `\tlog.Fatal(err)`,
        `}`,
        `defer yuva.Close(context.Background())`,
        ``,
        `// in a handler: the visitor's address and browser come from r`,
        `yuva.Track(r, analytics.Event{`,
        `\tName:  "Signup",`,
        `\tProps: map[string]any{"plan": "pro"},`,
        `})`,
      ].join("\n")
    case "curl":
      return [
        `curl ${server}/v1/analytics/properties/${v.id}/events \\`,
        `  -H "Authorization: Bearer $YUVA_API_KEY" \\`,
        `  -H "Content-Type: application/json" \\`,
        `  -d '{`,
        `    "events": [{`,
        `      "name": "Signup",`,
        `      "url": "${v.page}",`,
        `      "ip": "203.0.113.7",`,
        `      "user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",`,
        `      "props": { "plan": "pro" }`,
        `    }]`,
        `  }'`,
        ``,
        `# {"accepted": 1, "errors": []}`,
      ].join("\n")
    case "events":
      return [
        `<!-- data-outbound, data-files and data-chat are each optional -->`,
        `<script defer data-key="${key}" data-outbound data-files data-chat src="${server}/yuva-analytics.js"></script>`,
        ``,
        `<script>`,
        `  // a custom event with props and revenue (amount in the currency, never converted)`,
        `  window.yuva.track("Signup", {`,
        `    props: { plan: "pro", seats: 3 },`,
        `    revenue: { amount: "29.90", currency: "EUR" },`,
        `  })`,
        `  // on your 404 page`,
        `  window.yuva.notFound()`,
        `</script>`,
        ``,
        `// React: <YuvaAnalytics server="${server}" publicKey="${key}" outbound files chat />`,
        `// then const { track } = useYuvaAnalytics()`,
      ].join("\n")
    case "optout":
      return [`<a href="#yuva-optout">${optOut}</a>`, `<a href="#yuva-optin">${optIn}</a>`].join("\n")
    case "consent":
      return [
        `<!-- with the persistent or identify consent features, load yuva-analytics-consent.js instead -->`,
        `<script defer data-key="${key}" data-consent="required" src="${server}/yuva-analytics.js"></script>`,
        ``,
        `<script>`,
        `  // in your consent tool's callback, once the visitor agreed to analytics`,
        `  window.yuva.consent({ analytics: true })`,
        `  // or instead, with yuva-analytics-consent.js, if they also agreed to be recognised on later visits`,
        `  window.yuva.consent({ analytics: true, persistent: true })`,
        `  // and after sign-in, with a fresh identity token from your backend`,
        `  window.yuva.identify(token)`,
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
  const { canManage } = useSession()
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
    case "swift":
      return (
        <Trans>
          Add the YuvaAnalytics library of the Yuva package and configure it once at launch. .yuvaScreen counts a
          SwiftUI screen; in UIKit, call YuvaAnalytics.screen when a screen appears. Without a connection, events wait
          on the device for up to 72 hours and arrive with their own times.
        </Trans>
      )
    case "kotlin":
      return (
        <Trans>
          Add yuva-analytics from JitPack and configure it once in Application.onCreate. TrackScreen counts a Compose
          screen. Without a connection, events wait on the device for up to 72 hours and arrive with their own times.
        </Trans>
      )
    case "go":
      return canManage ? (
        <Trans>
          For what only your backend sees, such as a signup or a payment. Track forwards the visitor’s address and
          browser, so the event joins their visit; requests with Do Not Track or Global Privacy Control are not sent. It
          needs an{" "}
          <Link to="/settings/api-keys" className="text-brand underline-offset-4 hover:underline">
            API key
          </Link>{" "}
          with the analytics:write scope, which stays on your server.
        </Trans>
      ) : (
        <Trans>
          For what only your backend sees, such as a signup or a payment. Track forwards the visitor’s address and
          browser, so the event joins their visit; requests with Do Not Track or Global Privacy Control are not sent. It
          needs an API key with the analytics:write scope from an owner or admin, which stays on your server.
        </Trans>
      )
    case "curl":
      return (
        <Trans>
          Up to 100 events per request, each with the visitor’s IP address and user agent or your own visitor_key.
          Neither is stored. Events left out as invalid come back in errors with their position; see what arrived under
          Test events.
        </Trans>
      )
    case "events":
      return (
        <Trans>
          data-outbound counts clicks on links to other sites, data-files downloads of documents and archives, and
          data-chat each time a visitor opens the chat. track() sends your own events with up to 30 props; e-mail
          addresses in props are dropped. Make goals from events in the Goals view.
        </Trans>
      )
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

function PlatformOff({ tab, property }: { tab: Tab; property: AnalyticsProperty }) {
  const { canManage } = useSession()
  const platform = tab === "swift" ? "ios" : tab === "kotlin" ? "android" : null
  if (!platform || property.platforms.includes(platform)) return null
  const name = platform === "ios" ? "iOS" : "Android"
  return (
    <Notice icon={InfoIcon} data-testid="install-platform-off">
      {canManage ? (
        <Trans>
          Turn on {name} under Platforms in the general settings first. Until then the server refuses the app’s events.
        </Trans>
      ) : (
        <Trans>
          {name} is not one of this property’s platforms, so the server refuses the app’s events. An owner or admin can
          turn it on.
        </Trans>
      )}
    </Notice>
  )
}

export function InstallSnippets({ property }: { property: AnalyticsProperty }) {
  const { t } = useLingui()
  const [params] = useSearchParams()
  const version = useVersion().data?.version
  const tabs = tabsFor(property)
  const asked = params.get("snippet") as Tab | null
  const [picked, setTab] = useState<Tab>(asked && tabs.includes(asked) ? asked : tabs[0]!)
  const tab = tabs.includes(picked) ? picked : tabs[0]!
  const names: Record<Tab, string> = {
    html: "HTML",
    react: "React, Next.js",
    nuxt: "Nuxt",
    other: t`Vue, Astro, other`,
    swift: "Swift",
    kotlin: "Kotlin",
    go: "Go",
    curl: "curl",
    events: t`Events`,
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
          items={tabs.map((v) => ({ value: v, label: names[v], testId: `install-tab-${v}` }))}
        />
      </div>
      <p className="text-body text-muted-foreground">
        <Hint tab={tab} />
      </p>
      <PlatformOff tab={tab} property={property} />
      <div dir="ltr">
        <CodeBlock
          value={snippet(tab, {
            server: window.location.origin,
            key: property.public_key,
            id: property.id,
            page: examplePage(property),
            release: version && /^\d+\.\d+\.\d+$/.test(version) ? version : null,
            optOut: t`Do not count my visits`,
            optIn: t`Count my visits again`,
          })}
          testId="install-snippet"
        />
      </div>
    </div>
  )
}

type ApiTab = "go" | "curl"

export function apiSnippet(tab: ApiTab, v: { server: string; id: string }) {
  if (tab === "go") {
    return [
      `// go get github.com/productdevbook/yuva/sdk/go/analytics`,
      `import "github.com/productdevbook/yuva/sdk/go/analytics"`,
      ``,
      `yuva, err := analytics.New(analytics.Config{`,
      `\tBaseURL:    "${v.server}",`,
      `\tAPIKey:     os.Getenv("YUVA_API_KEY"), // a key with analytics:write`,
      `\tPropertyID: "${v.id}",`,
      `})`,
      `if err != nil {`,
      `\tlog.Fatal(err)`,
      `}`,
      `defer yuva.Close(context.Background())`,
      ``,
      `mux := http.NewServeMux()`,
      `mux.HandleFunc("GET /users/{id}", getUser)`,
      ``,
      `// wrap the mux itself, so the route pattern it matched is recorded`,
      `handler := yuva.Middleware(analytics.Monitor{`,
      `\tService:     "shop",`,
      `\tEnvironment: "production",`,
      `})(mux)`,
      `log.Fatal(http.ListenAndServe(":8080", handler))`,
    ].join("\n")
  }
  return [
    `curl ${v.server}/v1/analytics/properties/${v.id}/requests \\`,
    `  -H "Authorization: Bearer $YUVA_API_KEY" \\`,
    `  -H "Content-Type: application/json" \\`,
    `  -d '{`,
    `    "records": [{`,
    `      "method": "GET",`,
    `      "route": "/users/{id}",`,
    `      "status": 200,`,
    `      "duration_us": 12500`,
    `    }]`,
    `  }'`,
    ``,
    `# {"accepted": 1, "errors": []}`,
  ].join("\n")
}

function ApiHint({ tab }: { tab: ApiTab }) {
  const { canManage } = useSession()
  if (tab === "curl") {
    return (
      <Trans>
        Up to 1,000 records per request, gzip allowed. Send route when your router knows the pattern, else path, and ids
        are taken out of it. Records left out as invalid come back in errors with their position.
      </Trans>
    )
  }
  return canManage ? (
    <Trans>
      Middleware records every request the mux answers, with its route pattern, status and duration, and sends them in
      batches. It needs an{" "}
      <Link to="/settings/api-keys" className="text-brand underline-offset-4 hover:underline">
        API key
      </Link>{" "}
      with the analytics:write scope.
    </Trans>
  ) : (
    <Trans>
      Middleware records every request the mux answers, with its route pattern, status and duration, and sends them in
      batches. It needs an API key with the analytics:write scope from an owner or admin.
    </Trans>
  )
}

export function ApiInstallSnippets({ property }: { property: AnalyticsProperty }) {
  const { t } = useLingui()
  const [tab, setTab] = useState<ApiTab>("go")
  return (
    <div className="flex min-w-0 flex-col gap-3" data-testid="api-install">
      <div className="-mx-1 max-w-[calc(100%+0.5rem)] overflow-x-auto px-1 py-1">
        <Segmented
          value={tab}
          onChange={setTab}
          label={t`How to install`}
          items={[
            { value: "go", label: t`Go middleware`, testId: "install-tab-go" },
            { value: "curl", label: "curl", testId: "install-tab-curl" },
          ]}
        />
      </div>
      <p className="text-body text-muted-foreground">
        <ApiHint tab={tab} />
      </p>
      <div dir="ltr">
        <CodeBlock
          value={apiSnippet(tab, { server: window.location.origin, id: property.id })}
          testId="install-snippet"
        />
      </div>
    </div>
  )
}

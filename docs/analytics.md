# Analytics

Yuva Analytics counts the visitors, pages, sources, places and devices of your own websites, without
cookies and without keeping IP addresses. It lives in the same panel as your inbox, and its numbers
are open through the [API](#api).

## What it counts

| It does | It does not |
|---|---|
| Counts page views and how long a page was in view and how far it was scrolled | Set a cookie or write anything to the visitor's device |
| Shortens the IP address (IPv4 `/24`, IPv6 `/48`) before anything else uses it, then looks up the place from that and forgets it | Store an IP address, or a full one anywhere |
| Counts a visitor once per day, from a hash of the shortened address and the browser, under a salt that exists for one day | Follow a visitor from one day to the next, or from one site to another |
| Keeps the page's host and path, the referrer's host and path, and the `utm_*` parameters | Keep the rest of the query string, or anything that looks like an e-mail address |
| Leaves out bots, and visitors whose browser sends Do Not Track or Global Privacy Control | Count those visitors at all: nothing is sent |

So **visitors are counted per day**. Over a week, visitors is the sum of each day's visitors: someone
who comes back on another day counts again. A **visit** is a visitor's page views with no gap of 30
minutes, within one day.

The only thing the script ever stores on a device is a visitor's own choice to opt out
([below](#opt-out-links)).

What is kept and for how long, whether your site needs a cookie banner, and text for your privacy notice are in
[Analytics and privacy](analytics-privacy.md).

## Add a website

1. In the panel open **Analytics → Add a website**. Owners and admins can add websites.
2. Give it a name and list its **Domains**, one per line. Pages on other domains cannot send events.
   `*.example.com` covers every subdomain, but not `example.com` itself.
3. Pick the **Time zone** (days are counted in it), the **Inbox** whose agents may see the numbers (or
   none, then only owners and admins), and the **Audience**. Choose Children for a site made for children:
   only the country is kept, never the region or city.
4. Select **Add and install**, copy the snippet for how your site is built (below) and put it on your
   pages. The page turns green when the first page view arrives. A browser with Do Not Track or Global
   Privacy Control sends nothing, so test with another one.

The website's key (`yuva_ak_…`) is public, like a chat key: every page carries it, and it only works on the
website's domains. A workspace can have a limited number of websites; one over it is refused with
`403 property_limit`.

## Install

Replace `https://support.example.com` with your Yuva server and the key with your website's. The panel
shows these snippets ready to copy under **Settings → Install**.

### Script tag

```html
<script defer data-key="yuva_ak_xxxxxxxx" src="https://support.example.com/yuva-analytics.js"></script>
```

Put it in the `<head>` of every page. It counts page views, and also when a single-page app changes the
address, so a single-page app needs nothing more. A server started with `YUVA_WIDGET=off` or
`YUVA_ANALYTICS=off` does not serve the script.

| Attribute | Meaning |
|---|---|
| `data-key` | The website's key. Required. |
| `data-api` | The Yuva server's address, when it differs from where the script was loaded from ([own domain](#serve-it-from-your-own-domain)). |
| `data-hash` | The site routes with the part after `#`: it becomes part of the page path. |
| `data-manual` | No automatic page views; you call `yuva.pageview()` yourself. |
| `data-consent="required"` | Sends nothing until `yuva.consent({ analytics: true })` ([consent mode](#consent-mode)). |

With `data-manual`, call `window.yuva.pageview()` when a page is shown, or
`window.yuva.pageview({ url, referrer })` to name the page yourself.

### React and Next.js

```sh
npm install useyuva
```

Render the component once, in the root layout. It renders nothing:

```tsx
import { YuvaAnalytics } from "useyuva/react"

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html>
      <body>
        {children}
        <YuvaAnalytics server="https://support.example.com" publicKey="yuva_ak_xxxxxxxx" />
      </body>
    </html>
  )
}
```

`hash`, `consent="required"` and `manual` are props too. `useYuvaAnalytics()` returns `pageview`,
`consent`, `optOut` and `optIn` anywhere below it.

### Nuxt

```ts
// nuxt.config.ts
export default defineNuxtConfig({
  modules: ["useyuva/nuxt"],
  yuva: {
    server: "https://support.example.com",
    analytics: { key: "yuva_ak_xxxxxxxx" },
  },
})
```

The module adds a client plugin and the `useYuvaAnalytics()` composable. `NUXT_PUBLIC_YUVA_ANALYTICS_KEY`
overrides the key at runtime.

### Vue, Astro and other bundlers

```ts
import { createAnalytics } from "useyuva/analytics"

const analytics = createAnalytics({ server: "https://support.example.com", key: "yuva_ak_xxxxxxxx" })
```

Call it once in the browser, in `main.ts` for Vue, in a `<script>` of your layout for Astro, or in your entry
file. During server rendering it does nothing. It returns `pageview`, `consent`, `optOut`, `optIn` and
`destroy`, and takes the same `hash`, `consent: "required"` and `manual` options.

## Consent mode

Counting without cookies needs no banner under most privacy laws, but some sites must ask first. For them:

1. Install the script with `data-consent="required"` (or `consent="required"` in React, Nuxt and
   `createAnalytics`). It sends nothing until you say the visitor agreed.
2. In the callback of your own consent tool, call this. Do it on **every page load** when the visitor
   has already agreed, because the script does not remember the answer:

   ```js
   window.yuva.consent({ analytics: true })
   ```

   Before the script has loaded, queue the call instead:
   `;(window.yuvaq = window.yuvaq || []).push(["consent", { analytics: true }])`.
3. Set **Collection** to **Only with consent** in the website's **Settings → General**. The server then
   refuses any event that does not say the visitor agreed.

`yuva.consent({ analytics: false })` stops sending again.

Visitors who agree can also be recognised across days, with an id on their device for up to 13 months. That is
opt-in per property: see [Consent features](analytics-consent.md).

## Opt-out links

Put these two links on your privacy page:

```html
<a href="#yuva-optout">Do not count my visits</a>
<a href="#yuva-optin">Count my visits again</a>
```

Opting out is the only thing the script stores on the visitor's device, and only when they ask. You can also
call `window.yuva.optOut()` and `window.yuva.optIn()`.

## What sends nothing

The script sends nothing, and the server counts nothing, for: browsers with Do Not Track or Global Privacy
Control, automated browsers (`navigator.webdriver`), visitors who opted out, and known bots. Pages and
addresses you list under **Settings → Tracking** (**Pages not counted**, **Addresses not counted**) are
left out as well, and **Path rules** count pages that match a pattern under one path, so every order page
adds up as `/orders/:id`.

## Serve it from your own domain

Content blockers often stop scripts from other domains. Serving the script and its endpoint from your own
site keeps those visitors counted. Two things to know:

- Browsers send events to `/client/v1/analytics/<key>` relative to the script's server. With `data-api` (or
  `server`) you can give that server a path prefix such as `https://www.example.com/stats`.
- Your Yuva server sees the proxy, not the visitor. Make the proxy send the visitor's address in the
  `X-Forwarded-For` header, as the examples below do. Without it every visitor looks like the proxy: they
  are counted as one, and the limit of 120 events a minute per address applies to all of them together.

**nginx**, in your site's `server` block:

```nginx
location = /yuva-analytics.js {
  proxy_pass https://support.example.com/yuva-analytics.js;
  proxy_ssl_server_name on;
}
location /client/v1/analytics/ {
  proxy_pass https://support.example.com/client/v1/analytics/;
  proxy_ssl_server_name on;
  proxy_set_header X-Forwarded-For $remote_addr;
}
```

```html
<script defer data-key="yuva_ak_xxxxxxxx" src="/yuva-analytics.js"></script>
```

**Next.js**, under a neutral path: a rewrite serves the script and a route handler passes the events on with the
visitor's address. Use the address your hosting platform sets (`x-real-ip` or the first `x-forwarded-for`
entry), never one the visitor could write. In `next.config.js`:

```js
module.exports = {
  async rewrites() {
    return [{ source: "/stats/yuva-analytics.js", destination: "https://support.example.com/yuva-analytics.js" }]
  },
}
```

In `app/stats/client/v1/analytics/[key]/route.ts`:

```ts
export async function POST(request: Request, { params }: { params: Promise<{ key: string }> }) {
  const { key } = await params
  const visitor = request.headers.get("x-real-ip") ?? request.headers.get("x-forwarded-for")?.split(",")[0]?.trim() ?? ""
  const headers = new Headers({ "Content-Type": "text/plain", "X-Forwarded-For": visitor })
  for (const name of ["origin", "user-agent", "accept-language", "dnt", "sec-gpc"]) {
    const value = request.headers.get(name)
    if (value) headers.set(name, value)
  }
  const upstream = await fetch(`https://support.example.com/client/v1/analytics/${key}`, {
    method: "POST",
    headers,
    body: await request.text(),
  })
  return new Response(null, { status: upstream.status })
}
```

```html
<script defer data-key="yuva_ak_xxxxxxxx" data-api="https://www.example.com/stats" src="/stats/yuva-analytics.js"></script>
```

**Cloudflare Workers**, with a route such as `www.example.com/stats/*`:

```js
export default {
  async fetch(request) {
    const url = new URL(request.url)
    const target = new URL(url.pathname.slice("/stats".length) + url.search, "https://support.example.com")
    const headers = new Headers(request.headers)
    headers.set("X-Forwarded-For", request.headers.get("CF-Connecting-IP") ?? "")
    return fetch(target, { method: request.method, headers, body: request.body })
  },
}
```

Use the same `<script>` tag as for Next.js. In both cases the browser's `Origin` still has to be one of the
website's domains.

## The views

Open **Analytics** in the panel and pick a website. Every view shows the range you chose (Today to Last 12
months, or a custom range of at most 400 days), and its tabs are:

| View | Shows |
|---|---|
| **Overview** | Visitors, visits, pageviews, views per visit, bounce rate and visit duration over time (per hour, day or week), and the top pages, sources, locations and devices. |
| **Realtime** | Visitors and pageviews of the last 30 minutes, per minute, with the top pages, sources and countries. Refreshes every 15 seconds. |
| **Pages** | Pages, entry pages, exit pages and hosts, with time on page and scroll depth. |
| **Sources** | Channels, sources, referrers and the five `utm_*` parameters. |
| **Locations** | Countries, regions and cities (countries only for a Children audience). |
| **Devices** | Device, browser and version, operating system and version, screen size and language. |
| **Events** | Custom and built-in events with their props and revenue. |
| **Goals** | Conversions, conversion rate and revenue of the goals you set. |
| **Funnels** | How visitors go through a chain of goals, and where they leave. |
| **Settings** | General, Tracking, Install, Health and Data. |

Events, goals and funnels are in [Events, goals and funnels](analytics-events.md).

A website linked to an inbox also shows the support its pages lead to, with insights, alerts and e-mail reports:
[Support and traffic](analytics-support.md).

The same property counts your iOS and Android apps ([Analytics in apps](analytics-apps.md)) and events your backend
sends ([Server-side events](analytics-server.md)).

**Channels** sort where a visit came from: Direct, Organic search, Paid search, Social, Paid social,
E-mail, Referral, **AI assistants** (visits from ChatGPT and other assistants) and Internal.

**Locations** come from the DB-IP Lite database on a default server. DB-IP asks for a credit
wherever locations are shown, and the panel shows "IP Geolocation by DB-IP" under them.

### Filters, comparison and CSV

- Select a row of a table (a page, a source, a country) to **filter by this**; filters show above the
  numbers, can be changed or removed, and combine. Operators: is, is not, contains, does not contain,
  matches (`*` is one part of a path, `**` any number).
- **Comparison** sets the range against the period before, a year earlier, or other days of your choice.
- **Download CSV** saves the table you see, one column per breakdown and metric. Larger files, daily numbers and
  raw events are [exports](analytics-sharing.md#exports); a report for people without an account is a
  [share link](analytics-sharing.md#share-links).
- Some filter combinations need the detailed records, which are kept for the last
  [`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analytics) days (90 by default). Older days only have totals per
  page, source, place and device, and the panel says so.

### Health

**Settings → Health** shows, for the last 30 days, the events counted and dropped per day and why (bots,
rate limits, other sites, excluded, monthly limit, invalid, too late, server busy), when the last event
arrived, since when detailed records are kept, and which location database is in use. Of a dropped event
nothing is kept but its count.

## Keys and data

- **Make a new key** (Settings → Install) gives the website a new key. The old one keeps working for 24
  hours so you can update your pages without losing events. Making another key within that time ends the
  old one at once.
- **Delete days** (Settings → Data) removes the visits and events of a range of days, and their totals are
  counted again without them. **Delete all data** empties the website but keeps it and its key.
  **Delete the property** removes the website, its key and all its data. None of these can be undone.
- Detailed records are kept for [`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analytics) days, daily totals for
  25 months unless you set `retention_months` on the website through the API.

## Limits on hosted servers

On a server with open sign-up each workspace may count 1,000,000 events a month, unless the operator
set another [limit](configuration.md#analytics). Events over it are dropped until the month ends, and the
panel says so. One address may send 120 events a minute to a website.

## API

Every number in the panel comes from the API. A key needs the scope `analytics:read` to read and
`analytics:manage` to change websites; a key limited to inboxes cannot hold `analytics:manage`, and keys
made before these scopes existed do not get them ([scopes](headless.md#scopes)). Agents read the websites of
their inboxes. The API calls a website a property. A server started with `YUVA_ANALYTICS=off` answers `404`
for all of these.

| Call | Does |
|---|---|
| `GET /v1/analytics/properties` | Lists the properties you can see, with visitors today and this month's usage and limit. |
| `POST /v1/analytics/properties` | Creates one. Answers with its `public_key`. |
| `GET`, `PATCH`, `DELETE /v1/analytics/properties/{propertyId}` | Reads, changes and deletes one with its data. |
| `POST /v1/analytics/properties/{propertyId}/rotate-key` | Gives it a new key; the old one works for 24 more hours. |
| `POST /v1/analytics/properties/{propertyId}/delete-data` | Deletes the days `from` to `to`, or everything with `all: true`. Answers `202`. |
| `POST /v1/analytics/properties/{propertyId}/query` | The numbers. `?format=csv` answers CSV. |
| `GET /v1/analytics/properties/{propertyId}/realtime` | The last 30 minutes. |
| `GET /v1/analytics/properties/{propertyId}/health` | Collection health of the last `days` (1 to 90). |
| `POST /client/v1/analytics/{key}` | What the script calls: one event, or up to 100 as `{"e": [...]}`, at most 64 KiB, answered `202`. No key or cookie. |

Metrics: `visitors`, `visits`, `pageviews`, `views_per_visit`, `bounce_rate`, `visit_duration`,
`time_on_page`, `scroll_depth`, `events`. Dimensions are the columns of the views above plus the time grains
`time:hour`, `time:day`, `time:week` and `time:month`. A query takes up to two dimensions, ten filters and
a comparison. This asks for the ten most visited pages of the last 30 days against the 30 before:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/query \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{
    "metrics": ["visitors", "pageviews"],
    "dimensions": ["page"],
    "range": {"period": "30d"},
    "compare": {"with": "previous"},
    "limit": 10
  }'
```

The answer has `rows`, `totals`, `comparison` and `meta`. A query that needs detailed records for days
that are no longer kept answers `422 needs_raw_window` with the first day it can serve; a range over 400
days answers `422 range_too_large`. Four queries run at once per workspace; one more waits two seconds
and then gets `429`. All fields are in the [API contract](../openapi/openapi.yaml).

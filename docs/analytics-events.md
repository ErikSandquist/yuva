# Events, goals and funnels

Events are the things visitors do besides viewing a page: sign up, buy, download a file, follow a link. Send them
from your pages or apps, turn the ones that matter into **goals** to see how many visitors reached them and
how much revenue they brought, and chain goals into **funnels** to see where visitors drop off. This builds
on [Analytics](analytics.md); install it first.

## Send events

### From the script

```js
window.yuva.track("Signup", { props: { plan: "pro", seats: 3 }, revenue: { amount: "29.90", currency: "EUR" } })
```

`track(name, { props, revenue })` sends a custom event on the current page. The name has 1 to 120
characters. `pageview`, `engagement` and `screen`, and names starting with `$` or `yuva_`, are reserved and refused.
Events follow everything a page view follows: [consent mode](analytics.md#consent-mode), opt-out, Do Not Track,
Global Privacy Control and automated browsers.

The script is loaded with `defer`, so `window.yuva` does not exist yet when your own code runs early. Queue
the call instead; it runs as soon as the script is there, and works after it too:

```js
;(window.yuvaq = window.yuvaq || []).push(["track", "Signup", { props: { plan: "pro" } }])
```

`window.yuva.notFound()` marks the current page as your site's 404 page ([recipes below](#404-pages)).

### From React and Nuxt

In React, render `<YuvaAnalytics>` once as in [Analytics](analytics.md#react-and-nextjs) and call the hook
anywhere below it. Calls made before the script has loaded wait for it.

```tsx
import { useYuvaAnalytics } from "useyuva/react"

function UpgradeButton() {
  const { track } = useYuvaAnalytics()
  return (
    <button onClick={() => track("Upgrade", { revenue: { amount: "49.00", currency: "USD" } })}>Upgrade</button>
  )
}
```

`<YuvaAnalytics>` takes `server`, `publicKey` (not `key`, which React keeps for itself), `hash`, `consent`,
`manual`, `outbound`, `files` and `chat`; the last three are the [attributes below](#links-downloads-and-chat).
`useYuvaAnalytics()` returns `pageview`, `track`, `notFound`, `consent`, `optOut` and `optIn`.

In Nuxt, the module options under `yuva.analytics` are `key`, `server`, `hash`, `consent`, `manual`, `outbound`,
`files` and `chat`, and `useYuvaAnalytics()` is auto-imported with the same methods:

```ts
export default defineNuxtConfig({
  modules: ["useyuva/nuxt"],
  yuva: {
    server: "https://support.example.com",
    analytics: { key: "yuva_ak_xxxxxxxx", outbound: true, files: true },
  },
})
```

```vue
<script setup>
const { track } = useYuvaAnalytics()
</script>
```

In Vue, Astro and other bundlers, `createAnalytics()` from `useyuva/analytics` returns `track` and
`notFound` as well.

### Links, downloads and chat

Three attributes of the script tag (or props and options of the same name) send events without any code. They
are off until you add them:

| Attribute | Option | Sends |
|---|---|---|
| `data-outbound` | `outbound: true` | `outbound_click` when a visitor clicks a link to another host, with the prop `url`. |
| `data-files` | `files: true` | `file_download` when a visitor clicks a link to a pdf, zip, dmg, exe, apk, csv, xls, xlsx, doc, docx, ppt, pptx, mp3 or mp4 file, with the prop `url`. |
| `data-chat` | `chat: true` | `chat_open` when a visitor opens a [`<yuva-chat>`](widget.md) launcher on the page. |

```html
<script defer data-key="yuva_ak_xxxxxxxx" data-outbound data-files data-chat
  src="https://support.example.com/yuva-analytics.js"></script>
```

The `url` is kept without its query string and fragment, and only for `http` and `https` links. A chat shown
with `layout="embedded"` sits in your page and is never "opened", so it sends no `chat_open`.

`not_found` (from `notFound()`) is the fourth built-in event. Together they are `outbound_click`,
`file_download`, `not_found` and `chat_open`; the Events view names them Outbound link, File download, Page not
found and Chat opened.

## Props

Props add detail to an event: the plan chosen, the button clicked, the form used.

| Rule | |
|---|---|
| Count | Up to 30 per event. |
| Keys | 1 to 300 characters, and not starting with `$` or `yuva_`. |
| Values | Text, numbers or booleans, kept as text of at most 2,000 characters. |
| E-mail addresses | A value that looks like an e-mail address is dropped, not stored. The event itself is kept, and **Settings → Health** counts it as "had an address removed". |
| Children's websites | No props are kept, except the `url` of `outbound_click` and `file_download`. |

An event that breaks a rule (too many props, a reserved key, a value that is a list or an object) is not
stored. Sent alone it answers `400` with the reason; in a batch (the script batches events of one moment) only
that event is dropped and the rest are stored. Either way it is counted as `invalid` in **Settings → Health**,
so look there when an event does not show up.

Props of any name can be broken down on the [Events view](#the-events-view) for the days the detailed records are
kept ([`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analytics), 90 by default). A prop you also list under
**Settings → Tracking → Tracked props** (up to 10, not on a children's website) is kept in the daily totals
too, so its breakdown still works after the detailed records are gone. A prop you add is counted again over the days
the detailed records are kept; earlier days do not have it, so add the props you will want to compare over
months early.

## Revenue

An event can carry the money it made:

```js
track("Purchase", { revenue: { amount: "29.90", currency: "EUR" } })
```

- `amount` is a decimal string such as `"29.90"` (a number works too), `currency` an ISO 4217 code.
- Yuva stores it in the currency's **minor units**: 2990 for 29.90 EUR, 1500 for 1500 JPY. The panel and
  the API (`digits` in the revenue answer) turn it back.
- **Currencies are never converted or added up.** A site selling in EUR and USD sees one revenue figure per currency.
- The amount may have no more decimals than the currency has: two for EUR, none for JPY and KRW, three for
  KWD. `"29.999"` in EUR is invalid and not stored.
- Only custom and built-in events carry revenue. Revenue on a page view is refused.

Revenue is reported by goals and by the Events view, never by the page-view numbers.

## The Events view

**Analytics → Events** lists every event name with its visitors, visits, events and revenue for the range. Select
an event to break its props down: pick a prop, or type any prop name, and see the values it carried with
their counts. Select a value to keep only the visits with that value. Tracked props show as chips.

Use **How to send events** at the bottom for a snippet with your key in it.

## Goals

A goal names something you want visitors to do. **Analytics → Goals** lists them with their conversions; owners
and admins make them (**New goal**). A goal is one of:

| Kind | Reached when a visitor |
|---|---|
| Page view | Views a page whose path matches the pattern. `*` stands for one part of a path, `**` for any number: `/thanks`, `/blog/**`. |
| Event | Sends the event, and, if you set **Only with prop**, with that prop equal to the value: `Signup` with `plan` equal to `pro`. |
| Scroll depth | Scrolls to at least a percentage (1 to 100) of a page whose path matches. |

A goal reports, for the range:

| Number | Meaning |
|---|---|
| **Conversions** | Visitors who reached the goal, per day, added up. |
| **Conversion rate** | Conversions per 100 visitors. |
| **Completions** | Times it was reached, also several times by one visitor. |
| **Revenue** | What the goal's events carried, per currency, with the number of payments and their average. There is no separate revenue goal: an event goal shows the revenue its events carry. |

Select a goal for its trend and for where its conversions come from: channels, sources, entry pages,
countries and devices.

A property has up to 100 goals, with unique names of up to 120 characters.

**From which day a goal has numbers.** A new goal counts over all the days the detailed records are kept
(90 by default), not only from today. Days before that have none, and the goal says "This goal has numbers
from this day on". A goal whose definition you change is counted again over those days the same way; older
days keep what they counted before. A goal that is a step of a funnel cannot be deleted until you take it out
of the funnel. Making a goal adds a note to your charts.

## Funnels

A funnel shows how many visitors go through a chain of goals, in order, and where they leave. **Analytics →
Funnels** lists them; owners and admins make them (**New funnel**) from two to eight goals, in the order
visitors reach them. Up to 20 funnels per website, with unique names.

Per step, the report shows the visitors who completed it after every step before it, the drop-off from the
step before, the share of the first step's visitors and the share of the step before's visitors.

How a funnel is counted:

- **Within one visit.** A visitor only counts for step 3 if they reached steps 1 and 2 first, in the same
  visit. A visitor exists for one day, so a funnel never crosses midnight; visitors over a range are added
  up per day.
- **From the detailed records.** Funnels need each visit's steps, so they work only inside the days the
  detailed records are kept ([`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analytics)). A range that starts
  earlier answers `422 needs_raw_window`, and the panel offers to show from the first day it can.
- **At most 90 days** per report, or 31 days on a website that counted more than 10 million events in the
  last 30 days. A longer range answers `422 range_too_large`.
- Answers are cached for ten minutes.

## Segments

Filters narrow every view to some visitors: one country, one channel, visits with an event. To reuse a set of
filters, open **Segment** above the numbers and select **Save the filters as a new segment**. Select a segment
later to apply its filters (at most 10) to the view. Segments are shared with everyone who sees the
website, with unique names; there are up to 100 per website. Anyone who sees the website can make one. Its author, owners and
admins can change or delete it, and deleting a segment does not delete any numbers.

## Notes on charts

A note marks a day or a range of days on the charts, such as a launch or a campaign: **Add note** on the Overview chart,
with a text of up to 300 characters and a first and last day. Everyone who sees the website sees it. Anyone who sees the
website can add one (up to 1,000 per website); its author, owners and admins can edit or delete it.

Yuva adds notes of its own, for a new key of the website and for each goal you make. Only owners and admins
can delete these, and nobody can edit them.

## 404 pages

Call `notFound()` once on the page you show for missing pages. It sends `not_found` for the address the visitor
asked for, and the visit still counts as a bounce if that was all they did. Then `not_found` shows in the Events
view like any event, and a goal can count it.

**Next.js**, in `app/not-found.tsx` (`pages/404.tsx` works the same), with `<YuvaAnalytics>` in the root layout:

```tsx
"use client"

import { useEffect } from "react"
import { useYuvaAnalytics } from "useyuva/react"

export default function NotFound() {
  const { notFound } = useYuvaAnalytics()
  useEffect(() => notFound(), [notFound])
  return <h1>Page not found</h1>
}
```

**Nuxt**, in `error.vue`, with `analytics` set in `nuxt.config`:

```vue
<script setup>
const props = defineProps({ error: Object })
const { notFound } = useYuvaAnalytics()
onMounted(() => props.error?.statusCode === 404 && notFound())
</script>

<template>
  <h1>{{ error?.statusCode === 404 ? "Page not found" : "Something went wrong" }}</h1>
</template>
```

**Astro**, in `src/pages/404.astro`, with the script tag in your layout. The queue works whether or not the script has
loaded:

```astro
<Layout>
  <h1>Page not found</h1>
  <script is:inline>
    ;(window.yuvaq = window.yuvaq || []).push(["notFound"])
  </script>
</Layout>
```

On a static host the 404 page is served at the address the visitor asked for, so the page view and the
event carry that address.

## API

The panel uses these calls. Reading needs the scope `analytics:read`; making and changing goals and funnels
needs `analytics:manage` and an owner, an admin or a key without an inbox limit. Segments and notes are written
with `analytics:read`, by anyone who sees the website.

| Call | Does |
|---|---|
| `POST /client/v1/analytics/{key}` | What the script calls. An event is `{"n": "Signup", "u": "https://www.example.com/pricing", "p": {"plan": "pro"}, "$": {"a": "29.90", "c": "EUR"}}`. |
| `GET`, `POST /v1/analytics/properties/{propertyId}/goals` | Lists and makes goals. |
| `PATCH`, `DELETE /v1/analytics/properties/{propertyId}/goals/{goalId}` | Changes and deletes a goal. `409 goal_in_use` while it is a funnel step. |
| `GET`, `POST /v1/analytics/properties/{propertyId}/funnels` | Lists and makes funnels. |
| `PATCH`, `DELETE /v1/analytics/properties/{propertyId}/funnels/{funnelId}` | Changes and deletes a funnel. |
| `GET /v1/analytics/properties/{propertyId}/funnels/{funnelId}/report` | The steps for a `period`, or `from` and `to`. |
| `GET`, `POST /v1/analytics/properties/{propertyId}/segments` | Lists and saves segments. |
| `PATCH`, `DELETE /v1/analytics/properties/{propertyId}/segments/{segmentId}` | Renames, changes and deletes a segment. |
| `GET`, `POST /v1/analytics/properties/{propertyId}/annotations` | Lists notes (`from`, `to`) and adds one. |
| `PATCH`, `DELETE /v1/analytics/properties/{propertyId}/annotations/{annotationId}` | Changes and deletes a note. `403 annotation_read_only` for Yuva's own. |
| `POST /v1/analytics/properties/{propertyId}/query` | The numbers. Add `goal_id` for `conversions`, `conversion_rate`, `completions` and `revenue`; break down by `event` or by `prop` (with `prop` naming the key); add `segment_id` to apply a segment. |

Limits answer `403` with `goal_limit`, `funnel_limit`, `segment_limit` or `annotation_limit`, and a name taken
by another goal, funnel or segment answers `409` with `goal_taken`, `funnel_taken` or `segment_taken`. All fields are in the
[API contract](../openapi/openapi.yaml).

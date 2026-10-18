---
title: "Yuva Analytics: one set of numbers for your website and your apps"
description: "Yuva Analytics counts websites, iOS and Android apps and backend events without cookies. Use it on its own, or link it to the inbox and see which pages leave people with questions. A walk through one small product, and a few others."
date: 2026-10-18
author: productdevbook
---

Most teams we talk to are looking for analytics right now. Some want to leave a tool that needs a cookie banner. Some found that their website numbers and their app numbers live in two products that never agree. And some simply want to know why the same questions keep landing in their support inbox.

Yuva Analytics is our answer to all three. It counts visitors, pages, sources, places and devices without cookies and without keeping IP addresses. It counts your iOS and Android apps and your backend in the same property as your website. And when you want it to, it puts the support each page causes next to that page's traffic.

You do not need the rest of Yuva to use it. Analytics works as a product on its own, and it gets more useful the more of Yuva you connect to it.

## One product, followed from the start

Take a made-up but very ordinary product: **Pinecone**, a habit tracker. It has a marketing site with a blog and a pricing page, an iPhone app, an Android app and a small backend that handles subscriptions. Two people run it.

### Week one: just the numbers

Pinecone starts with analytics only. In the panel they open **Analytics → Add a website**, list their domains and copy one line into the `<head>` of the site:

```html
<script defer data-key="yuva_ak_xxxxxxxx" src="https://support.example.com/yuva-analytics.js"></script>
```

There is a [React and Next.js component, a Nuxt module](/docs/analytics/#install) and a plain function for Vue and Astro too. The page turns green when the first page view arrives.

That is enough for the overview: visitors, visits, page views, bounce rate and visit duration, the top pages and where people came from. The **Sources** view sorts visits into channels, and one of them is **AI assistants**, so Pinecone can see how many people arrive from ChatGPT and the like. **Realtime** shows the last 30 minutes, which is fun on the day a blog post goes out.

Nothing in this needs a banner on its own. The script sets no cookie and writes nothing to the device. The IP address is shortened before anything else uses it and is never stored. A visitor is counted once per day from a hash that changes every day, so nobody is followed from one day to the next or from one site to another. Browsers that send Do Not Track or Global Privacy Control send nothing at all. [Analytics and privacy](/docs/analytics-privacy/) has the details and text for your privacy notice.

### Week two: the apps join

The blog brings visitors, but most of Pinecone's users live in the app. So they tick **iOS** and **Android** under the property's platforms and add the [Swift and Kotlin libraries](/docs/analytics-apps/). In SwiftUI a screen is one modifier:

```swift
HomeView()
    .yuvaScreen("Home")
```

In Compose it is `TrackScreen(route)`, and with Navigation Compose the route pattern keeps `habit/{id}` one screen instead of a thousand.

Now the site and both apps are in **one property**. The Devices view shows how the iPhone and Android audiences compare, and the Pages view lists `app://` screens next to the blog posts. The apps keep the same rules as the site: no device id, no advertising id, no cookie. A launch gets a random key that lives in memory only. Events wait in the app's private storage, kept out of backups, until the server confirms it has them, so a train ride without signal loses nothing. The guide even has the answers for the App Store and Google Play privacy forms.

### Week three: what actually matters

Page views do not pay the bills. Pinecone wants to know who starts a trial and who pays. In the app:

```swift
YuvaAnalytics.track("Trial started", props: ["plan": "yearly"])
```

The payment itself only the backend sees, so it goes in as a [server-side event](/docs/analytics-server/) with an API key, carrying the money it made. Revenue is stored per currency and never converted, so EUR and USD stay two honest figures instead of one guessed one.

With events in place they make [goals](/docs/analytics-events/#goals) ("Viewed pricing", "Trial started", "Paid") and put them in a [funnel](/docs/analytics-events/#funnels). The funnel shows that plenty of people open the pricing page and start a trial, and that a lot of them leave between the trial and the payment.

The numbers say *where* people leave. They do not say *why*.

### Week four: linking the inbox

This is where the rest of Yuva comes in. Pinecone puts the Yuva chat widget on the site and links the analytics property to their support inbox. From then on every page shows its [support numbers](/docs/analytics-support/) next to its views: conversations, page feedback, ratings and discussions, by page and day.

The number to watch is the **support rate**: conversations per 1,000 page views. A busy page does not win just by being busy. For Pinecone the top of the list is not the home page. It is `/pricing`, with a support rate four times the site's average, and when they open those conversations most of them ask the same thing: does the trial charge my card when it ends?

One sentence on the pricing page fixes that. The next week the support rate of `/pricing` drops, and the funnel step between trial and payment gets a little wider. Neither number would have explained the other on its own.

This join stays at the page. No visitor of Analytics is ever linked to a contact or a conversation. A conversation remembers the page it started on, without its query string, and the counts never say who was reading.

On top of the same numbers Pinecone gets, without checking anything:

- **Insights** on the overview: short sentences about what changed and how much.
- **Alerts**, checked every hour: a spike or drop in traffic, a goal converting at half its usual rate, a jump in conversations per page view, a new site sending visitors. They arrive by e-mail and as a webhook.
- **Weekly and monthly reports** by e-mail, in each member's language.

Each of these is a fixed rule written in the guide. No model guesses on your behalf.

## Other ways it fits

Pinecone is one shape. A few others we see:

- **A documentation site.** Add `<yuva-page-feedback>` and the "Was this page helpful?" rating to your [documentation pages](/docs/documentation-pages/). Analytics then ranks the guides by the questions and bad ratings they cause, so you know which page to rewrite first, before you have written a single support reply.
- **A small online shop.** Send `Purchase` events with their revenue, make a funnel from product page to checkout to thank-you page, and compare this month with last month. Path rules count every order page as `/orders/:id`, so the tables stay readable.
- **A newsletter.** When a [broadcast](/docs/broadcasts/) goes out from Yuva, its links get UTM parameters, and the visits it brings show up under Sources with the campaign's name. You see which issue brought readers back, not only which one was opened.
- **An app with no website at all.** A property can count only apps. Screens, events and revenue from Swift and Kotlin, retention for users who agreed to be remembered, and nothing else.
- **Numbers for people outside the team.** A [share link](/docs/analytics-sharing/) opens a read-only report without signing in, with the tabs you choose, an optional password and an expiry date. Send it to an investor, or embed it on your own page.
- **Questions from an assistant.** The [MCP server](/docs/mcp/#analytics-from-an-assistant) gives an AI assistant tools to read the same numbers: "which pages caused the most conversations last month?" is one question away.

## When you need more than a day

Counting visitors per day is what keeps Yuva Analytics free of cookies. Some products need returning visitors, retention cohorts or funnels that span several days. For those there are [consent features](/docs/analytics-consent/): only for visitors who said yes in your own consent tool, a random id is kept on their device for at most 13 months. Everyone else is counted exactly as before, and withdrawing consent removes the id and makes the server forget it.

## Your data stays yours

Every number in the panel comes from the [API](/docs/analytics/#api), with up to two dimensions, ten filters and a comparison per query. Exports write daily numbers to CSV and raw events to NDJSON for your own tools. Yuva is open source, so you can also run the whole thing on your own server: the analytics, the inbox and everything between them.

## Try it

- **Hosted:** create an account from the [home page](/), open **Analytics** and add your site. You can leave the inbox for later.
- **Self-hosted:** run Yuva with Postgres using the [install guide](/docs/install/).

Start with the [Analytics guide](/docs/analytics/). Yuva is still pre-alpha: the API and the database schema can change. Try it, and tell us what you miss in an [issue](https://github.com/productdevbook/yuva/issues).

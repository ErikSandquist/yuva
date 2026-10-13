# Support and traffic

Link a website of [Yuva Analytics](analytics.md) to an inbox and its pages show how much support they lead to:
conversations, page feedback, ratings and discussions, counted per page and day next to the page's views. The
same numbers feed insights, alerts, weekly and monthly e-mail reports and the analytics tools of
[AI assistants](mcp.md#analytics-from-an-assistant). Yuva runs no model for any of it: every insight and alert is a
fixed rule you can read below.

## What is joined

Choose the inbox in the website's settings (**Inbox**, [Add a website](analytics.md#add-a-website)). From then on,
by **page and day**, Yuva counts what that inbox received from the website's pages:

| Number | Counts |
|---|---|
| `conversations` | Conversations started from the page with the [chat widget](widget.md), on the day they started. |
| `feedback` | Feedback sent from the page with [`<yuva-page-feedback>`](documentation-pages.md). |
| `ratings_up`, `ratings_down` | The page's "Was this page helpful?" ratings, on the day they were given (UTC). |
| `helpful_rate` | Percent of the ratings that are up. |
| `discussions` | Discussion threads started on the page. |
| `support_rate` | Conversations per 1,000 page views of the page. |

They stand next to `visitors` and `pageviews` of the same page. Pages are matched by host and path, after the
website's path rules, so `/orders/:id` is one page here too.

**What is never joined:** no visitor of Analytics is ever linked to a contact or a conversation. The two sides meet
only at the page. To make the page known, the chat widget sends the address of the page a conversation starts on
**without its query string and fragment**, and the page title, and nothing else from Analytics. A conversation shows
the page it started on; the counts do not say who was reading. Analytics itself still stores no cookie and no address
([Analytics and privacy](analytics-privacy.md)).

A website without a linked inbox has no support numbers, and a support query answers `422 no_linked_inbox`.

## Support rate

The support rate is **conversations per 1,000 page views**: 12 conversations on a page with 3,000 views is 4. It is
a ratio, so a busy page does not win just by being busy. A page with fewer than 100 views in the range is listed after
the others, because a rate over a handful of views says little.

Open the website's support view in the panel to see pages ranked by it, with feedback, ratings and discussions,
and to compare the range with the one before. Through the API it is one query with `dataset: "support"`:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/query \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{
    "dataset": "support",
    "metrics": ["pageviews", "conversations", "support_rate"],
    "dimensions": ["page"],
    "range": {"period": "30d"},
    "order_by": [{"metric": "conversations"}],
    "limit": 10
  }'
```

The support dataset takes the dimensions `page`, `hostname` and a day, week or month grain (a page or host and a
grain at most), filters on `page` and `hostname`, and no goals, props or segments. Its rows are the pages with any
support in the range.

## Insights

Add `"insights": true` to a query, or open the panel's overview, and Yuva adds short findings worked out from the
answer's own numbers. They are fixed rules:

| Insight | Rule | Example |
|---|---|---|
| `metric_change` | A total moved by 20% or more against the comparison range, with at least 10 before or after. | "Visitors are up 34%." |
| `row_change` | With one breakdown: a row moved by 50% or more, at least 10 before or after. The three largest. | "Visits from `news.example.com` doubled." |
| `new_value` | A row with at least 10 that had none in the comparison range: a new referrer, page or campaign. The three largest. | "`/pricing` is new this month." |
| `support_hotspot` | In the support dataset by page: a page with at least 3 conversations whose rate is at least twice the range's. The three highest. | "`/docs/install` has 5 conversations, 9 per 1,000 views, against 3 overall." |

The change rules need a comparison (`compare`). Insights that name a page, a referrer or a campaign carry text that
visitors' browsers sent.

## Alerts

Alerts tell you when something changes without asking you to look. They are checked every hour and **only for
websites where at least one member has chosen alerts** (below). Each kind is a fixed rule:

| Kind | Fires when | Needs |
|---|---|---|
| `spike` | Page views of the last two finished hours are 3 times or more the same two hours averaged over the 7 days before. | A baseline of at least 20 views. |
| `drop` | Page views of the last two finished hours are half or less of that average. | A baseline of at least 20 views. |
| `goal_drop` | A goal's conversions yesterday are half or less of its average over the 7 days before. Checked once a day, early in the morning of the website's time zone. | At least 10 conversions a day on average. |
| `support_load` | Yesterday's conversations per 1,000 page views are twice the rate of the 28 days before or more. Checked once a day. | A linked inbox and at least 5 conversations yesterday. |
| `new_referrer` | A site that sent at least 10 visits yesterday had sent none in the 28 days before. Checked once a day. | None. |

A kind fires **once a day** per website (a new referrer once per site). When it fires, Yuva:

1. puts a note on the chart, on the day it is about (an [annotation](analytics-events.md) with the finding's text, which you can also add yourself);
2. stores an `analytics.alert` event and sends the `analytics.alert` [webhook](#the-analyticsalert-webhook);
3. e-mails the members who chose that kind.

`GET /v1/analytics/properties/{propertyId}/alerts` lists the alerts a website raised, newest first, 100 at a time
(`before` takes the `created_at` of the last one for older ones; scope `analytics:read`).

### The analytics.alert webhook

Add the event `analytics.alert` to a [webhook endpoint](webhooks.md). An endpoint limited to an inbox gets the alerts
of websites linked to that inbox; one for the whole workspace gets all of them. The body is the usual
[envelope](webhooks.md#events) (`type`, `timestamp`, `workspace_id`, `inbox_id` when the website has an inbox, `data`):

```json
{
  "type": "analytics.alert",
  "timestamp": "2026-10-13T06:00:05Z",
  "workspace_id": "6f1e0c5a-0d2e-4a39-9f55-5d0c0d7f2a10",
  "inbox_id": "b1c4d8e2-7a3f-4c0e-8d61-2f9a5e3b7c40",
  "data": {
    "alert": {
      "id": "0a8f3c1d-6b52-4e17-9c44-7d2e1f0b9a63",
      "property_id": "c3d5e7f9-1a2b-4c6d-8e0f-3a5b7c9d1e2f",
      "kind": "support_load",
      "day": "2026-10-12",
      "metric": "support_rate",
      "current": 6.4,
      "baseline": 2.1,
      "text": "6.4 conversations per 1,000 page views yesterday against 2.1 on the 28 days before",
      "annotation_id": "5e9b2d7c-3f41-4a86-b0d5-8c1a6f4e2b97",
      "created_at": "2026-10-13T06:00:05Z"
    },
    "property": { "id": "c3d5e7f9-1a2b-4c6d-8e0f-3a5b7c9d1e2f", "name": "Acme website" }
  }
}
```

`metric` is `pageviews`, `conversions`, `support_rate` or `visits`; `baseline` is what the rule compared `current`
with (0 for a new referrer). `spike` and `drop` also have `hours_from` and `hours_to`, `goal_drop` the `goal_id` and
`goal_name`, and `new_referrer` the `referrer` host. `text` is English, as on the note.

## E-mail reports and alert mails

Every member chooses for themselves, per website:

- **Report**: none (the default), weekly or monthly.
- **Alerts**: any of the five kinds above.

Find the report and alert settings in the website's pages in the panel; or call
`GET` and `PUT /v1/analytics/properties/{propertyId}/subscription` (a member's session, not an API key):

```json
{ "report": "weekly", "alerts": ["spike", "drop", "support_load"] }
```

| Report | Goes out | Covers |
|---|---|---|
| Weekly | Monday at 08:00 in the website's time zone | The week before, Monday to Sunday. |
| Monthly | The 1st of the month at 08:00 in the website's time zone | The month before. |

A report is written in the member's language and has plain numbers: the headline figures against the period before,
the top five pages and sources, the conversions of the goals, and for a website with an inbox the conversations,
feedback and ratings, and the page that led to the most conversations. It has **no images and nothing that tracks
opens or clicks**. An alert mail says what was found and links to the website in the panel.

To see what a report looks like, use the send-now button in the same place, or
`POST /v1/analytics/properties/{propertyId}/subscription/send-report`. It sends the last seven finished days, or the
last month if you chose monthly; five an hour per member.

## Tracking broadcast campaigns

When a [broadcast](broadcasts.md) goes out, Yuva adds UTM parameters to its web links so that the visits it brings
show up in Analytics under Sources, `utm_campaign` and the `E-mail` channel. This is **on by default**. A link gets:

```
?utm_source=newsletter&utm_medium=email&utm_campaign=<campaign>
```

The campaign is made from the broadcast's name when you create it (lowercase letters, digits, `-`, `_` and `.`, up
to 100 characters); you can change it on the draft. Links left alone:

- links that already have a `utm_` parameter of their own;
- links that are not `http` or `https`, such as `mailto:`;
- Yuva's own links: unsubscribe, preferences and click tracking.

Turn it off in the broadcast's options, or with `"utm": false` on a draft (`PATCH /v1/broadcasts/{broadcastId}`);
`utm_campaign` is the campaign field. It works with or without [click tracking](broadcasts.md#click-tracking) and does
not send anything to a third party: the visit is counted by your own Analytics. Broadcasts that were already past
draft when UTM links arrived keep their links as sent.

The broadcast's report then shows **visits from this campaign** per website: visitors, visits and page views whose
`utm_campaign` is the broadcast's, from the day sending started, for the websites you may see (it needs
`analytics:read`; `campaign_visits` in the API).

## Ask an assistant

An AI assistant connected over [MCP](mcp.md) can read the same numbers. For example:

> Which pages lead to the most conversations this month?

The assistant calls `analytics_support_report` and answers with the pages, their conversations, views and rate. The
tools, prompts and scopes are in [Analytics from an assistant](mcp.md#analytics-from-an-assistant).

## Reference

| Call | Does |
|---|---|
| `POST /v1/analytics/properties/{propertyId}/query` | The numbers; `dataset: "support"` and `insights: true` as above. Scope `analytics:read`. |
| `GET /v1/analytics/properties/{propertyId}/alerts` | Alerts the website raised. Scope `analytics:read`. |
| `GET`, `PUT /v1/analytics/properties/{propertyId}/subscription` | My report and alert choices. Member session. |
| `POST /v1/analytics/properties/{propertyId}/subscription/send-report` | Mails me a report now. Member session. |
| `GET /v1/analytics/properties/{propertyId}/annotations` | Notes on the chart, alerts included. |
| `GET /v1/broadcasts/{broadcastId}/report` | The broadcast's report with `campaign_visits`. |

All fields are in the [API contract](../openapi/openapi.yaml).

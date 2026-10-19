# Roadmap

Each milestone is a GitHub issue. A milestone is done when every acceptance item is shown working
on a running instance, not when the code exists.

## M0 — Foundation

Status: done.

- `api/` Go module, `web/` panel, `edge/`, `sdk/js`, `sdk/go` skeletons with their check commands.
- `openapi/openapi.yaml` with health and version, linted; generated strict server.
- Goose migrations, sqlc, River set up; `deploy/compose.yaml` starts Postgres and the server.
- CI: every check command runs on pull requests.

Accept: `docker compose up` serves `/healthz` and an empty panel; CI is green.

## M1 — Core model and panel sign-in

Status: done.

- Workspaces, members, roles, per-inbox access; sign-in with e-mailed code and passkeys.
- Inboxes and channels (settings only), contacts, conversations, messages, notes, events,
  attachments in object storage.
- Workspace API keys; `/v1` endpoints for all of the above; usage counters.

Accept: through the API alone, create an inbox, a contact and a conversation, add messages and a
note, assign and close it.

## M2 — Agent panel

Status: done.

- Conversation list with filters and search, thread view, composer (reply/note, canned replies,
  attachments), contact sidebar, assignment, status, labels.
- Settings screens for M1 objects. Realtime updates over WebSocket. Turkish and English.

Accept: two members answer and hand over a conversation in the panel and see each other's changes
live.

## M3 — E-mail channel

Status: done.

- `/ingress/email`, the Cloudflare Email Worker in `edge/`, threading, quote stripping, HTML
  sanitizing, attachments, raw message storage.
- Outbound SMTP, thread headers, loop protection, SES bounce/complaint ingress.

Accept: a mail to the support address opens a conversation, the panel reply arrives in the
sender's mailbox in the same thread, their reply joins the conversation, an auto-reply does not
loop.

## M4 — Web widget: live chat and embedded threads

Status: done.

- `/client/v1`, contact sessions, identity tokens, anonymous visitors, allowed origins.
- `<yuva-chat>` launcher and embedded layouts, presence, typing, read receipts, business hours,
  `live`/`async` modes, e-mail continuity.
- `sdk/go`: identity token signing.

Accept: on a test page an anonymous visitor chats live with a member; a signed-in user of a host
app sees their earlier conversations; after leaving, the visitor gets the reply by e-mail.

## M5 — Mobile SDKs, feedback and webhooks

Status: done.

- `sdk/swift` and `sdk/kotlin`: client, conversation list, thread, composer, feedback form.
- Feedback kind and metadata; `api` channel for server-side forms.
- Standard Webhooks delivery with retries and a delivery log; contact deletion by external id.
- `sdk/go`: webhook verification.

Accept: in a sample iOS and Android app a user sends feedback, a member answers in the panel, the
host backend receives the webhook and the app shows the reply.

## M6 — Member notifications

Status: done.

- PWA install, Web Push, per-member notification preferences, e-mail fallback.

Accept: a new conversation in a `live` inbox notifies a member's phone within seconds.

## M6.1 — Satisfaction ratings

Status: in progress; API done, widget, mobile SDKs and panel next. Decisions in `architecture.md` ›
Satisfaction ratings.

- An inbox setting; contacts rate a closed conversation `good` or `bad` with an optional comment,
  once per close, in the widget, the mobile SDKs and through two links in an e-mail.
- The rating in the panel thread and on the conversation, a `conversation.rated` webhook, ratings
  per inbox in the team stats.

Accept: a widget visitor and an e-mail contact each rate a closed conversation; the panel shows
both, the host backend receives `conversation.rated`, and a second rating of the same close is
refused.

## M7 — First internal rollout

Status: in progress.

- Our own instance; an existing in-app feedback form moved from e-mail to Yuva; e-mail channels for the products
  that already receive support mail.

Accept: one week of real support handled only in Yuva.

## M8 — First public release (0.0.1)

Status: in progress; ships as `v0.0.1` (see `CHANGELOG.md`).

- Docker image, install guide, configuration reference, upgrade notes, backup guide.
- Security review of authentication, client sessions, ingress and attachment handling.

Accept: a fresh install from the guide works on a clean host.

## M9 — Headless and MCP (0.0.4)

Status: done in `v0.0.4`; plan in issue #17, decisions in `architecture.md` › Headless access and
› OAuth and MCP. Checked on a public server with claude.ai, Claude Code and Codex.

- Scoped API keys, bot authors, drafts, an event feed, idempotency keys, a published headless JS
  client and an API-only mode.
- An MCP server at `/mcp` with OAuth for members, tools, resources and prompts over the existing
  `/v1` access rules; replies from assistants are drafts unless the workspace allows sending.
  Yuva runs no model itself.

Accept: Claude Code and claude.ai triage an inbox and draft replies a member sends from the panel;
a prompt-injection e-mail sends nothing; a headless chat works with the panel turned off.

## M10 — Native apps for members (0.0.25)

Status: planned; iOS in issue #18, Android in issue #19.

- iOS 27 (Swift 6.4, SwiftUI) and Android 16+ (Compose, Material 3 Expressive) apps for members,
  on the App Store and Google Play. One build signs in to any Yuva server: the member picks
  useyuva.com or enters their own server's URL, and signs in through OAuth (M9) with a code or a
  passkey on the server's own page.
- Push through a relay we run, with end-to-end encrypted payloads so the relay never sees content;
  self-hosted servers use it without setup. Quick reply from the notification.
- Offline cache and outbox, tablet and foldable layouts, English, Turkish and German.

Accept: the same store build works against useyuva.com and a self-hosted server; a push reaches a
real phone through the relay within seconds and the relay holds no message text.

## M11 — Feedback and questions on documentation pages

Status: done in `v0.0.7`; questions and published answers were replaced by discussions in M12. Plan in
issue #23, decisions in `architecture.md` › Documentation pages.

- `<yuva-page-feedback>` (was this page helpful, then optional text and e-mail) and
  `<yuva-page-questions>` (published answers and a question form) for any documentation site,
  on an existing chat channel; guides for Nuxt Content and Fumadocs.
- Per-page rating counters, page feedback as feedback conversations, questions as `question`
  conversations, answers members publish to the page.
- A Docs view in the panel: pages by rating, their feedback, questions and published answers.

Accept: on a Nuxt Content and a Fumadocs site, a visitor rates a page, sends feedback and asks a
question; the panel shows the counts and both conversations; a member answers the question and
publishes it, and the answer appears on that page without the visitor's name or address; an
element on an origin the channel does not allow is refused.

## M12 — Discussions on documentation pages and reader accounts

Status: done in `v0.0.7`; plan in issue #24, decisions in `architecture.md` › Discussions (M12).

- Public discussions under each documentation page, like GitHub Discussions: threads, replies,
  votes, accepted answers, followers, moderation. They replace page questions and published answers.
- Server-wide reader accounts (e-mail code or passkey), connected to documentation sites through a
  popup; identity tokens keep working for host apps with their own login.
- A reader account page: my discussions, followed threads, notifications, e-mail settings.
- `<yuva-discussions>` for any site, Nuxt and React wrappers, and a shadcn registry for React sites.
- A Discussions view and moderation queue in the team panel.

Accept: on demo-nuxt.useyuva.com a reader signs in with an e-mail code, starts a discussion and
waits for approval; a member approves it and answers; the reader is e-mailed, sees the answer on
the page and in their account, and marks it accepted; a second reader replies and votes; a React
site installs the shadcn components and shows the same thread; nothing private of another
workspace or another reader is visible.

## M13 — Broadcasts and e-mail designs

Status: M13.1–M13.5 done and deployed on 2026-10-12 (#26–#30); M13.6 (#91) and M13.7 (#92) done,
verified on 2026-10-19. Umbrella issue #25, decisions in `architecture.md` › Broadcasts and e-mail designs (M13).

- **M13.1 Lists and subscribing:** mailing lists with recorded consent and double opt-in,
  `<yuva-subscribe>` (HTML, React, Nuxt, shadcn), unsubscribe and preference pages with one-click, an API
  for host backends, consent export.
- **M13.2 Designs:** the design document, the server renderer to responsive HTML and text, images, the
  block designer in the panel, six starter templates in `templates/email/`.
- **M13.3 Sending:** broadcasts to lists narrowed by attributes, review, test sends, scheduling, pause and
  cancel, sending through an inbox's e-mail channel (optionally its own SMTP account), bounces,
  complaints and suppression, replies as conversations, reports, daily caps and the approval gate for
  hosted servers.
- **M13.4 Import and deliverability:** CSV import with attestation and progress; SPF, DKIM and DMARC
  check by probe; optional DKIM signing by Yuva.
- **M13.5 Reach:** the template gallery on the website with "Use this template", MCP tools and prompts,
  subscriptions in the mobile SDKs, the İYS export.
- **M13.6 Pace:** a broadcast's own pace, at most so many mails an hour, a day and in 30 days, spread
  evenly below the channel's rate and the workspace's daily limit; set in the composer, changed while it
  runs, checked by the review (at most 365 days), in the API and MCP.
- **M13.7 SES tenants:** on a shared Amazon SES account each inbox sends in its own SES tenant, created
  and watched by the server with an operator's IAM key limited to its tenants; support replies and
  broadcasts carry `X-SES-TENANT`; a channel's own tenant for self-hosters without keys; pauses shown
  on the inbox and its channels and acted on by its broadcasts; the operator's `inbox ses-tenant`
  commands, with the back-fill.

Accept: a visitor subscribes through `<yuva-subscribe>` and confirms; a member designs a newsletter from a
gallery template, sends a test and schedules it to the list; the deliverability check on one of our own
sending domains behind the edge shows SPF, DKIM and DMARC passing with the list headers signed, and its
WebKit and Chromium renders match the preview, in light and dark; one recipient replies and the reply is a conversation in the inbox;
an out-of-office reply is not; another unsubscribes with one click and gets no further broadcast; a bounce
suppresses the address and a complaint unsubscribes it; a hosted workspace cannot send before approval;
another workspace sees none of it.
A broadcast with a pace of 60 an hour sends one mail a minute to Mailpit, shows when its next mail goes
out, and goes on at the channel's rate within five minutes of removing the pace.
With the server's SES keys set, two inboxes sending through the server's SES account from one domain
get a tenant each, holding the domain and the configuration set; a reply and a broadcast to the SES
success simulator count under their inbox's tenant in CloudWatch; pausing one inbox's tenant in SES
pauses its running broadcast within five minutes, blocks resuming and shows on the inbox and its
channels, while the other inbox's broadcast finishes; `inbox ses-tenant reinstate` lets mail go again;
the IAM key cannot read a tenant outside its prefix; deleting the workspace deletes its tenants.

## M14 — Move to Yuva

Status: done and deployed on 2026-10-12 (all four steps, #31–#34); umbrella issue #35, decisions in
`architecture.md` › Moving to Yuva (M14).

- **M14.1 IMAP with presets:** every mailbox over IMAP with app passwords (Gmail, Google Workspace,
  Yandex, iCloud, Zoho, Yahoo, Fastmail, GMX, any server), detection from the address and MX, a
  connection tester with precise errors, the read-only guard and its tests, scan and dry run,
  resumable copy with live progress, catch-up sync, forwarding addresses, delivery verification,
  finish with credential deletion, the wizard and list in the panel, guides in en, tr and de, MCP
  tools and the `move_to_yuva` prompt.
- **M14.2 Microsoft over IMAP:** "Sign in with Microsoft" for Outlook.com and Microsoft 365, only to
  obtain an IMAP XOAUTH2 token (`IMAP.AccessAsUser.All`); the same read-only IMAP client.
- **M14.3 Export files:** Google Takeout, mbox, .eml and Maildir archives with resumable upload; the
  fast way for very large Gmail mailboxes, followed by IMAP for newer mail and the cutover.
- **M14.4 Help desks:** Zendesk and Help Scout.

Accept: an owner moves a mailbox with an app password through the wizard; the dry run matches the
mailbox; killing the server mid-copy loses and duplicates nothing; the inbox shows the threads with
original dates, members as authors and labels, nothing unread and no notification, webhook or push;
mail arriving during the switch is stored once and notifies; the forwarding is verified by a probe;
finishing deletes the credentials; the source mailbox is byte-identical (UIDs, flags, modseq) before
and after, on the Dovecot fixture; the wizard and the guides tell people where each provider keeps
its IMAP settings and app passwords. A Takeout file imports the same way, and the help-desk imports
pass against recorded API answers; no check needs a real provider account. An assistant runs the
whole flow through MCP without ever handling a password.

## M15 — Analytics

Status: done in `v0.0.11`; umbrella issue #42, decisions in `architecture.md` › Analytics
(M15). Ships in six steps; M15.1 first, then M15.2, M15.3 and M15.5 in parallel, then M15.4 (after M15.2; its broadcast part
after M13.3) and M15.6 (after M15.2 and M15.3).

- **M15.1 Count the web:** properties, the cookieless collect endpoint with daily salts and shortened
  addresses, GeoIP, bot filtering, partitions and rollups, the ~1 KB script and `useyuva/analytics` with
  React and Nuxt helpers, overview, realtime, pages, sources, locations and devices with filters and
  comparisons, consent mode, usage and limits, the privacy guide.
- **M15.2 Events, goals, funnels:** custom and built-in events, props, revenue, goals, funnels, segments,
  annotations.
- **M15.3 Apps and servers:** Swift `YuvaAnalytics` and Kotlin `yuva-analytics` with offline queues and
  store privacy answers, server-side events and the Go helper.
- **M15.4 Support, alerts, reports, MCP:** support load per page next to traffic, docs ratings and
  discussions with views, broadcast campaigns to visits, alerts, insights, e-mail reports, MCP tools.
- **M15.5 Sharing and export:** read-only share links with password, expiry and embedding; CSV and
  NDJSON exports.
- **M15.6 Consent features:** a stored id and identified users only with consent: returning visitors,
  retention, cross-day funnels, a contact's recent visits.

Accept: a demo site with the script and the Swift and Kotlin samples report pageviews, screens and a
revenue goal that appear in Realtime and, the next day, from rollups with the same numbers; the
browser holds no cookie or storage entry and Postgres holds no IP address; GPC stops collection; a
children's property keeps country only; the Support report ranks pages by conversations per 1,000 views
without joining any visitor to a contact; Claude Code answers a traffic question through the MCP tools;
a share link shows only its views; after consent and identify a contact's page lists their visits and
without consent it never does; another workspace sees none of it; the load test meets the targets at
10M events a month.

## M17 — Home and one setup

Status: planned; issue #74, decisions in `architecture.md` › Panel (`web/`) › Home and setup (M17).

- `GET /v1/home` with inbox counts, visitors per property (today, last 30 minutes, seven days), the
  last broadcast and the setup checklist shared with `get_setup_state`.
- `/setup` as one hub of six cards with done states; the analytics and newsletter flows inside it.
- Home at `/` for owners and admins, the inbox at `/conversations`; agents keep the inbox at `/`.

Accept: a new workspace's owner lands on Home, sets up chat and analytics from `/setup` and sees both
turn done; Home shows the inbox counts, each site's visitors and the last broadcast from one request;
an agent opens the inbox at `/` and never sees an inbox or property they may not see.

## M18 — Feedback on its own

Status: done. API, panel, SDKs and progress verified on 2026-10-19 with the Swift and Kotlin samples against a server; the widget shows Resolved for a closed feedback (`5a73a1e`) and the SDK threads keep the resolved line when a progress is set (`2e344c1`). Issue #93, decisions in `architecture.md` › Feedback (M18).

- `kind` on `GET /v1/conversations/counts` and `GET /client/v1/conversations`; `can_rate` false for
  feedback.
- A Feedback section in the panel: Open and Resolved, category tabs with counts, triage rows, the
  conversation beside the list; Conversations without feedback; Resolve, Reopen, Reply and resolve.
- Swift and Kotlin: Messages and Feedback tabs, feedback states (Sent, Answered, Resolved), the
  resolved line in the thread, View feedback after sending. The widget calls a resolved feedback
  Resolved.
- Progress on feedback (Planned, In progress, Done, Won't do) set in the panel, shown to the contact
  in the list and the thread; the Swift and Kotlin views redrawn as a native messaging app.

Accept: in the Swift and Kotlin samples a user sends a bug report with a screenshot and finds it under
Feedback as Sent; a member finds it in the panel's Feedback section under Bug, not in Conversations,
answers it and the app shows Answered, then resolves it with Reply and resolve and the app shows
Resolved with no rating asked; the user writes again and it is open in the panel again; an agent sees
only feedback of inboxes they may see; a member sets In progress and then Done and the app shows each
in the row and as a line in the thread.

## M19 — Status pages

Status: planned; decisions in `architecture.md` › Status pages (M19). Built in this order, each its own
issue; each part is done when its Accept holds on a running instance.

### M19.1 — Pages, components and incidents

Status: done, issue #95; every Accept item verified on 2026-10-19 against a server built from `015570a`.

- Tables, `/v1/status-pages` with components, groups, incidents, maintenance and updates; scopes
  `status:read`, `status:manage`; stored events and webhook types; MCP tools.
- The public page rendered by the server: `/status/{slug}`, history, incident pages, Atom feed, badge,
  `GET /client/v1/status/{slug}`; en, tr, de; cache and rate limit.
- Panel: a Status section per inbox's page with components, incidents, maintenance, preview and
  publish.

Accept: an owner makes a page with two components, publishes it and opens `/status/{slug}` signed out;
an agent opens an incident that sets one component to `major_outage`, posts an update and resolves it,
and the page, the feed, the badge and the JSON follow each step; a scheduled maintenance starts and
completes on its own; an agent of another inbox sees neither the page nor its incidents in the panel or
the API.

### M19.2 — Heartbeats and pushed states

Status: done, issue #96; every Accept item verified on 2026-10-19 against a server built from `015570a`.

- `heartbeat` and `push` monitors with `/ingress/heartbeat/{token}` and `/ingress/status/{token}`
  (own JSON, Alertmanager, Grafana); thresholds, draft or published incidents on failure; member
  notifications; `status.monitor.down` and `.up`.

Accept: a heartbeat monitor whose beats stop sets its component to its failure state and opens a draft
incident after the grace; beats again bring it back; an Alertmanager firing body fails a push monitor
and its resolved body recovers it; a member who set a state by hand later than the monitor keeps it.

### M19.3 — Status in the widget, the apps and the inbox

- `status` on the client inbox and `channel.updated`; the line in `<yuva-chat>`, Swift and Kotlin.
- Linking conversations to an incident, its list of conversations, "Reply to linked".

Accept: while an incident is published the widget and both sample apps show its line and it goes away
when it is resolved; a member links three conversations (e-mail, chat, app), replies once with "Reply
to linked" and each contact gets the reply on their own channel.

### M19.4 — E-mail subscribers

- `mailing_lists.kind` `status` and its rules (no audiences, imports, public forms, İYS export or
  opt-out register); per-component choices.
- `/status/{slug}/subscribe` with double opt-in, limits and the sweep of unconfirmed contacts; `/s/` and
  `/m/` for status lists.
- Notices: `notify`, the 60-second hold, coalescing, the 15-minute gap and 10 an hour per page; sending
  through the inbox's channel with list headers; bounces, complaints and replies linked to the incident.
- Panel: e-mail subscribers setting, Subscribers tab, Notify subscribers with counts, notice lines and
  Undo. Guides in en, tr, de.

Accept: a simulator address subscribes from the page, confirms, gets a notice for a published incident
and its resolution and none for a draft or an update sent without notifying, and unsubscribes with one
click.

### M19.5 — Outside checks

- `http` and `tcp` monitors with the outbound address rules, keyword, certificate expiry, response
  times in `status_checks` and their chart in the panel; hosted limits.

Accept: an `http` monitor on a URL that starts answering 500 fails after its threshold and recovers;
a monitor whose URL resolves to a private or metadata address is refused when saved and at check time;
the panel shows the response times of the last 24 hours.

### M19.6 — Own domains

- `custom_domain` with CNAME and TXT checks; `YUVA_STATUS_DOMAINS=ask` with `/status-domains/check` and
  the Caddy example in `deploy/`, and `cloudflare` with Cloudflare for SaaS custom hostnames.

Accept: a page on a verified domain answers at `/` behind Caddy with a certificate issued on demand, and
on a server behind Cloudflare with the custom hostname active; an unverified or removed domain gets 404
from the check and is not (or no longer) a custom hostname.

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

## M10 — Native apps for members (0.0.9)

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

Status: in progress; plan in issue #24, decisions in `architecture.md` › Discussions (M12).

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

Status: planned; umbrella issue #25, decisions in `architecture.md` › Broadcasts and e-mail designs
(M13). Ships in five steps; M13.1 and M13.2 in parallel, then M13.3, then M13.4 and M13.5 in parallel.

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

Accept: a visitor subscribes through `<yuva-subscribe>` and confirms; a member designs a newsletter from a
gallery template, sends a test, schedules it to the list and it lands in the inbox (not spam) of the project's seed
mailboxes at Gmail, Outlook and iCloud with SPF, DKIM and DMARC passing, and its WebKit and Chromium renders
match the preview, in light and dark; one recipient replies and the reply is a conversation in the inbox;
an out-of-office reply is not; another unsubscribes with one click and gets no further broadcast; a bounce
suppresses the address and a complaint unsubscribes it; a hosted workspace cannot send before approval;
another workspace sees none of it.

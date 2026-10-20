# useyuva

JavaScript SDK for [Yuva](https://github.com/productdevbook/yuva), open-source customer messaging.

| Import | What |
|---|---|
| `useyuva` | `createYuvaClient()`: the contact side (`/client/v1`) without DOM. Browsers, Node 22+, Bun, Deno, workers. |
| `useyuva/chat` | The `<yuva-chat>` web component, built on that client. Browser only. |
| `useyuva/docs` | `<yuva-page-feedback>` and `<yuva-discussions>` for documentation pages. Registers in the browser only, so it is safe to import during SSR. |
| `useyuva/subscribe` | `<yuva-subscribe>`, a mailing list sign-up form. Registers in the browser only. |
| `useyuva/status` | `<yuva-status>`, a status band for the bottom of a site footer. Registers in the browser only. |
| `useyuva/analytics` | `createAnalytics()`: cookieless pageviews and engagement for an analytics property. Does nothing during SSR. |
| `useyuva/react` | `YuvaProvider`, `useConversations`, `useMessages`, `YuvaAnalytics`, `useYuvaAnalytics`. React 18+ is an optional peer. |
| `useyuva/nuxt` | A Nuxt module: `YuvaChat`, `YuvaPageFeedback`, `YuvaDiscussions` and `YuvaSubscribe` components, configured once in `nuxt.config`, and the identity and webhook routes on the server. |
| `useyuva/nitro` | A Nitro v3 module serving `/_yuva/identity` and `/_yuva/webhook` and sending server-side analytics; `defineYuvaServer` from `useyuva/nitro/runtime`. |
| `useyuva/api` | A typed `/v1` client for backends (API key), generated from the OpenAPI contract. |
| `useyuva/server` | `signIdentityToken`, `verifyWebhook` and `createAnalytics` for backends. Web Crypto and `fetch` only: Node 22+, Bun, Deno, workers, srvx. |

```sh
npm install useyuva
```

Without a bundler, the server hosts the widget itself: `<script src="https://<your-yuva>/yuva.js" defer></script>`,
the documentation page elements without the chat: `<script src="https://<your-yuva>/yuva-docs.js" defer></script>`,
the subscribe form: `<script src="https://<your-yuva>/yuva-subscribe.js" defer></script>`,
the status band: `<script src="https://<your-yuva>/yuva-status.js" defer></script>`,
and analytics: `<script src="https://<your-yuva>/yuva-analytics.js" data-key="yuva_ak_…" defer></script>`.

## Client

```ts
import { createYuvaClient } from "useyuva";

const yuva = createYuvaClient({
  server: "https://support.example.com",
  channel: "yuva_pk_…",
  identityToken: () => fetch("/my/yuva-token").then((r) => r.text()),
});

yuva.on("event", (event) => {
  if (event.type === "message.created") console.log(event.data.author.type, event.data.body);
});
await yuva.connect();

const { conversation } = await yuva.startConversation({ body: "Hello" });
await yuva.sendMessage(conversation.id, { body: "Here is a screenshot", files: [file] });
```

Options:

| Option | Meaning |
|---|---|
| `server` | Base URL of the Yuva server. |
| `channel` | Public key of a `chat` or `app` channel. Outside a browser use an `app` channel: `chat` channels accept only their allowed origins. |
| `identityToken` | `() => string \| null \| Promise<…>`: an identity token signed by your backend. Without it the contact is an anonymous visitor (if the channel allows them). |
| `storage` | `{ getItem, setItem }` keeping the session and visitor id. Defaults to `localStorage`, else memory. |
| `fetch`, `WebSocket` | Replacements for the global ones. |
| `idempotencyKeys` | Send `client_id` as `Idempotency-Key` too. On by default outside browsers; off in browsers until the server allows the header cross-origin. |

Session:

- `start()` opens a session, or resumes the stored one for the same identity. Every other call starts one when needed; a session that ended (`401`) is renewed once and the call retried.
- `session` (contact, inbox settings, expiry), `token`, `refresh()`, `channelSettings()` (the inbox's public settings without a session).
- `setIdentityToken(source)` switches identity; `reset()` forgets the session in memory; `signOut()` ends it on the server and in storage.
- `openApp(kind, params)` opens the panel on an app: `"help"` (`{ article }`, `{ query }`), `"feedback"`, `"status"`, `"subscribe"` or `"roadmap"` (`{ idea }` opens that idea). The Roadmap tile and app appear only when the inbox has a public roadmap board; voting and suggesting open the reader sign-in popup unless `identity-token` is set.

Conversations and messages:

- `listConversations({ cursor, limit })`, `getConversation(id)`, `startConversation({ body, subject, files, client_id })`.
- `listMessages(id, { order, cursor, limit })`, `sendMessage(id, { body, files, client_id })`. `client_id` (generated when absent) makes a retry safe: the server returns the stored message for a repeated `client_id`, and with `idempotencyKeys` the request also carries it as `Idempotency-Key`.
- `markRead(id, messageId?)`, `setTyping(id, typing)`, `setEmail(email)`, `attachment(id)` (a `Blob`).

Realtime:

- `connect()` opens `/client/v1/realtime`, resumes after the last event on reconnect, retries with backoff and renews an ended session. `disconnect()`, `reconnect()` (try now, e.g. when the page becomes visible), `connected`.
- `on(type, handler)` returns an unsubscribe function. `event`: every realtime frame (`message.created`, `conversation.updated`, `typing`, `presence`, `resync_required`, …). `connection`: `{ state: "connecting" | "open" | "reconnecting" | "closed", attempt }`. `session`: `{ session, renewed }`; after `renewed` or `resync_required`, reload what you show.

Errors are `ApiError` with `status` and the problem `code` (`status` 0 and `network` when offline).

### Discussions

The threads of documentation pages, the same calls `<yuva-discussions>` makes. Reading needs no
session; writing needs a reader (signed in with their account on the Yuva server) or a contact
session started with an identity token. Bodies are Markdown; show `body_html`, which the server
renders and sanitizes, never the Markdown as HTML.

```ts
const yuva = createYuvaClient({ server, channel });

const { items, next_cursor } = await yuva.listDiscussions(location.href, { sort: "top" });

button.onclick = async () => {
  // Opens <server>/account/connect in a popup; call it from the click handler itself.
  const { reader } = await yuva.readerSignIn();
  console.log(reader.initials);
};

const thread = await yuva.startDiscussion({ page_url: location.href, page_title: document.title, title, body });
const { discussion, posts } = await yuva.getDiscussion(thread.id);
const reply = await yuva.replyToDiscussion(thread.id, { body: "Same here", parent_post_id: posts[0]?.id });
```

- `listDiscussions(page, { sort, cursor, limit })` → `DiscussionPage`. `sort` is `new` (default), `top` or `unanswered`.
- `getDiscussion(id)` → `DiscussionDetail` (`{ discussion, posts }`, the accepted answer first, then oldest first; replies to replies carry `parent_post_id`).
- `startDiscussion({ page_url, page_title, title, body })` → `Discussion`; `replyToDiscussion(id, { body, parent_post_id })` → `DiscussionPost`. On a moderated channel a reader's first posts come back with `state: "pending"`, visible only to them until a member approves one.
- `editDiscussionPost(postId, { body, title })` (within 30 minutes; `title` on the opening post only), `deleteDiscussionPost(postId)`.
- `voteDiscussionPost(postId, voted = true)` → the post with its count; the thread's votes are its opening post's (`opening_post_id`).
- `followDiscussion(id, following = true)`; `acceptDiscussionAnswer(id, postId | null)` (the thread's author; `null` clears it) → `Discussion`.
- `channelSettings()` tells whether discussions are on: `chat.discussions` is `off`, `moderated` or `open`.
- With a reader or identified contact, lists and threads also carry the caller's pending posts and `mine`, `voted` and `following`.

Reader sign-in:

- `readerSignIn({ width, height })` opens `<server>/account/connect?channel=<key>&origin=<origin>` in a popup and resolves with `{ token, expires_at, reader: { name, initials } }` once the popup posts it back. Only a message from the server's origin, for this channel and this page's origin, is taken. Rejects with `ApiError` code `popup_blocked` or `sign_in_cancelled`.
- The token is kept in `storage` per server and channel until it expires (24 hours); `reader` returns it or `null`. On `401` it is dropped and writes fail with `sign_in_required`; sign in again (the popup closes at once while the account is still signed in on the server).
- `readerSignOut()` ends the token on the server and forgets it. `on("reader", handler)` reports sign-in and sign-out.
- `discussionWriter()` resolves to `"reader"`, `"contact"` (an identity token) or `null` (read only).
- `openReaderPopup(server, channel)` and `readerConnectUrl(server, channel, origin)` are the same popup without a client.

Mailing lists:

- `lists()` returns the channel's public lists and a `form_token`; `subscribe({ email, list_ids, page_url, consent_text, form_token })` answers `{ confirmation }` the same way for every address. Send the token back 2 seconds to 2 hours after `lists()`.
- With an identity token: `subscriptions()` lists the public lists with the contact's `state`; `subscribeList(listId, { consent_text })` and `unsubscribeList(listId)` change it.

Errors worth handling: `403 sign_in_required`, `discussions_off`, `discussion_locked`, `edit_window_closed`, `forbidden`; `400 page_origin`; `429 rate_limited`.

## Chat element

```ts
import "useyuva/chat";
```

```html
<yuva-chat channel="yuva_pk_…" server="https://support.example.com"></yuva-chat>
```

Attributes, methods and events are in [docs/widget.md](https://github.com/productdevbook/yuva/blob/main/docs/widget.md).

## Documentation pages

```ts
import "useyuva/docs";
```

```html
<yuva-page-feedback channel="yuva_pk_…" server="https://support.example.com"></yuva-page-feedback>
<yuva-discussions channel="yuva_pk_…" server="https://support.example.com"></yuva-discussions>
```

Both take `channel`, `server`, `locale`, `dir` and `identity-token` like `<yuva-chat>`, plus `page`
and `page-title`, which default to the URL without query and fragment and `document.title`. They
follow client-side navigation.

`<yuva-discussions>` shows the page's threads (New, Top, Unanswered), a thread with its replies
and accepted answer, and forms to start a thread, reply, edit, delete, vote, follow and accept an
answer. Readers sign in with their account on the Yuva server in a popup; with `identity-token`
your signed-in users write as themselves without it. Methods: `reload()`, `open(id)`, `signIn()`
(from a click), `signOut()`.

Events: `yuva-rating` (`{ page, rating, previous }`), `yuva-feedback` (`{ page, rating, conversation_id }`),
`yuva-discussion` (`{ page, discussion_id, state }`), `yuva-post` (`{ page, discussion_id, post_id, state }`)
and `yuva-reader` (`{ name, initials }` or `null`). In React use `YuvaPageFeedback` and
`YuvaDiscussions` from `useyuva/react` (`onDiscussion`, `onPost`, `onReader`).

## Subscribe form

```ts
import "useyuva/subscribe";
```

```html
<yuva-subscribe channel="yuva_pk_…" server="https://support.example.com" layout="inline"></yuva-subscribe>
```

Takes `channel`, `server`, `locale`, `dir` and `identity-token`, plus `lists` (comma-separated ids;
empty for every public list, with a checkbox each when there is more than one), `layout` (`stacked`
or `inline`), `name` (`optional` or `required` adds a name field), `button-text`, `consent` (your
own consent sentence) and `privacy-url`. The consent sentence next to the unticked box is sent as it
is shown. With an identity token it shows a switch per list instead of the e-mail field. Style it
with `--yuva-accent`, `--yuva-on-accent`, `--yuva-radius`, `--yuva-font` and
`::part(field|button|consent|status)`. Events: `yuva-subscribe` (`{ lists, confirmation }`) and
`yuva-subscribe-error` (`{ code }`); `YuvaSubscribe` in `useyuva/react` (`onSubscribe`, `onError`)
and Nuxt (`@subscribe`, `@error`).

## Status band

```ts
import "useyuva/status";
```

```html
<yuva-status page="acme" server="https://support.example.com" components></yuva-status>
```

Put it at the very bottom of a footer. It reads the public status page `page` (its slug) and shows
the state with the open incident, the last 90 days with the uptime and, with `components`, a second
row with every visible component. It needs no credentials and refreshes every 60 seconds while the
tab is visible. `theme` is `auto` (follows `prefers-color-scheme`), `light` or `dark`; `lang`
defaults to the document's language, then the browser's. The background is transparent. Override
the colours with `--yuva-status-paper`, `-ink`, `-soft`, `-faint`, `-rule`, `-cross`, `-ok`, `-warn`,
`-partial`, `-maint`, `-bad`, `-ok-text`, `-warn-text`, `-bad-text`, the side padding with
`--yuva-status-gutter` and the font with `--yuva-status-font`.

## Analytics

```html
<script defer data-key="yuva_ak_…" src="https://support.example.com/yuva-analytics.js"></script>
```

```ts
import { createAnalytics } from "useyuva/analytics";

const analytics = createAnalytics({ server: "https://support.example.com", key: "yuva_ak_…" });
```

Counts a pageview on load and on every History API navigation (`pushState`, back and forward; not
`replaceState`, not a query change), and one `engagement` per view with the visible and focused
time and the deepest scroll when the page is hidden or left. Events go out as `text/plain`, so the
browser sends no preflight; events of one moment share a request, and the last one is sent with
`sendBeacon`. A `403` from the server stops sending for the page.

Nothing is put on the device and no cookie is read: the only storage access is reading
`localStorage.yuva_optout`, written by `optOut()`, unless the visitor consents to `persistent`
(below). With `navigator.globalPrivacyControl`, Do Not
Track or `navigator.webdriver` nothing is sent at all.

| Attribute | Option | |
|---|---|---|
| `data-key` | `key` | The property's public key. |
| `data-api` | `server` | The Yuva server; the script defaults to the origin it was loaded from. Set it when you serve the endpoint from your own domain. |
| `data-hash` | `hash: true` | Hash routing: the fragment is part of the page. |
| `data-consent="required"` | `consent: "required"` | Send nothing until `consent({ analytics: true })`. |
| `data-manual` | `manual: true` | No automatic pageviews; call `pageview()`. |
| `data-outbound` | `outbound: true` | A click on a link to another host sends `outbound_click` with prop `url`. |
| `data-files` | `files: true` | A click on a link to a pdf, zip, dmg, exe, apk, csv, xls(x), doc(x), ppt(x), mp3 or mp4 file sends `file_download` with prop `url`. |
| `data-chat` | `chat: true` | Opening a `<yuva-chat>` on the page sends `chat_open`. |

Methods (`window.yuva` for the script): `pageview({ url?, referrer? })`,
`track(name, { props?, revenue? })`, `notFound()`, `consent({ analytics })` (or `consent(false)`),
`optOut()`, `optIn()`, and `destroy()`. Calls made before the script loads go to
`(window.yuvaq = window.yuvaq || []).push(["track", "Signup", { props: { plan: "pro" } }])`.

```ts
analytics.track("Signup", { props: { plan: "pro", seats: 3 }, revenue: { amount: "29.90", currency: "EUR" } });
analytics.notFound(); // on the site's 404 page
```

`track` sends a custom event on the current page: up to 30 props (keys of 1 to 300 characters, not
starting with `$` or `yuva_`; string, number or boolean values) and revenue as a decimal amount in an
ISO 4217 currency, which Yuva never converts. The server refuses `pageview`, `engagement`, `screen`
and names starting with `$` or `yuva_`. `notFound()` sends `not_found`, which does not stop the visit
from bouncing. Every event follows consent mode, the opt-out, GPC, Do Not Track and `webdriver`
like pageviews do. `<yuva-chat>` dispatches `yuva-open` (bubbling, composed) when its launcher
opens it; the embedded layout does not.
Links to `#yuva-optout` and `#yuva-optin` opt the visitor out and back in.

### Consent features

For a property whose consent features offer them, from `useyuva/analytics`, React and Nuxt, or with
the `/yuva-analytics-consent.js` script tag instead of `/yuva-analytics.js` (which leaves them out
to stay within 1.5 KB; `window.yuva` then has `identify` and `persistentId` too):

```ts
analytics.consent({ analytics: true, persistent: true }); // from your consent tool, on every page load
analytics.identify(await fetchIdentityToken()); // after sign-in, a fresh token from your backend
analytics.persistentId(); // the id, for an erasure request (DELETE …/visitors/{persistentId})
```

`persistent: true` keeps `{ id, createdAt }` (a random UUID) in `localStorage.yuva_pid` and sends the
id as `c.p` on every event, engagement included; a new id is made when none exists or it is 13 months
old, and none ever under GPC or Do Not Track. Without that call nothing is stored and no id is sent.
Pass `persistent: true` on every `consent` call while the visitor agrees to it:
`consent({ analytics: true })` or `persistent: false` with an id stored withdraws it, as below.
`identify(token)` needs the id; it sends the identity token once in a `yuva_consent` event and keeps
nothing. `consent(false)`, `consent` without `persistent: true` and `optOut()` send a `yuva_consent`
withdrawal for a stored id, also under GPC or Do Not Track, and delete the id.

In React, render `<YuvaAnalytics server="…" publicKey="yuva_ak_…" outbound />` once (it renders nothing; the
other options are props too) and call
`useYuvaAnalytics()` anywhere for `pageview`, `track`, `notFound`, `consent`, `identify`, `persistentId`, `optOut` and `optIn`; calls before it has
loaded wait for it. The prop is `publicKey` because React keeps `key` for itself. In Nuxt, set `yuva.analytics` (below) and use `useYuvaAnalytics()`.

## Nuxt

```ts
// nuxt.config.ts
export default defineNuxtConfig({
  modules: ["useyuva/nuxt"],
  yuva: {
    server: "https://support.example.com",
    channel: "yuva_pk_…",
    analytics: { key: "yuva_ak_…" },
  },
});
```

`NUXT_PUBLIC_YUVA_SERVER`, `NUXT_PUBLIC_YUVA_CHANNEL`, `NUXT_PUBLIC_YUVA_LOCALE` and, when
`analytics` is set, `NUXT_PUBLIC_YUVA_ANALYTICS_KEY` override the options at runtime. `analytics`
takes `key`, `server` (defaults to `yuva.server`), `hash`, `consent`, `manual`, `outbound`, `files` and `chat`, adds a client
plugin and auto-imports `useYuvaAnalytics()`.

For signed-in users, the only code you write is who they are:

```ts
// server/yuva.ts
export default defineYuvaServer({
  user: async (event) => {
    const { user } = await getUserSession(event); // your session; null when signed out
    return user && { subject: user.id, email: user.email, emailVerified: user.emailConfirmed, name: user.name };
  },
  webhooks: {
    "message.created": async (payload) => {
      if (!payload.data.contact?.online) await sendPush(payload.data);
    },
  },
});
```

```sh
NUXT_YUVA_IDENTITY_SECRET=yuva_is_…   # at build and at runtime
NUXT_YUVA_WEBHOOK_SECRET=whsec_…
NUXT_YUVA_ANALYTICS_API_KEY=yuva_…
NUXT_YUVA_ANALYTICS_PROPERTY_ID=…
```

With an identity secret, the module serves `GET /_yuva/identity` (the [h3](#h3) identity handler for
`user(event)`) and every component fetches its token there on its own; pass `identityToken` to
override that, or `null` for anonymous. With a webhook secret it serves `POST /_yuva/webhook`, which
verifies the request and calls `webhooks[type]` like `defineYuvaWebhookHandler`. With an analytics API
key (`analytics:write`) and property id, `event.context.yuva.track(event)` sends server-side events
to `yuva.server` (or `NUXT_YUVA_SERVER`), flushed through `waitUntil` after the request where the
platform has one and when Nitro closes. The secrets are the options `identitySecret`,
`webhookSecret`, `analytics.apiKey` and `analytics.propertyId` or the variables above; they go only
into the server runtime config (`useRuntimeConfig().yuva`), never into the public one. A route is
added only when its secret is known at build time, from `nuxt.config` or the environment; a secret
that is only in the build environment is not written into the build, so set it at runtime too. A
route whose secret is empty at runtime answers `404`. `defineYuvaServer` is auto-imported on the
server (`useyuva/nitro/runtime`), and `event` has your Nuxt's `H3Event` type. Works on Nitro v2 and
v3. `<YuvaChat />`, `<YuvaPageFeedback />`, `<YuvaDiscussions />` and `<YuvaSubscribe />` are
auto-imported, render on the server and load their element in the browser when they mount. Props
are the attributes in camelCase (`pageTitle`, `identityToken`, which also takes a function);
events are `@rating`, `@feedback`, `@discussion`, `@post`, `@reader` and `@unread`. `yuva-*` tags are custom elements in
your templates.

## React

```tsx
import { createYuvaClient } from "useyuva";
import { YuvaProvider, useConversations, useMessages } from "useyuva/react";

const yuva = createYuvaClient({ server, channel });

function App() {
  return (
    <YuvaProvider client={yuva}>
      <Inbox />
    </YuvaProvider>
  );
}

function Thread({ id }: { id: string }) {
  const { messages, send, loadOlder, hasMore } = useMessages(id);
  // …
}
```

`YuvaProvider` connects realtime while mounted (`connect={false}` to leave that to you).
`useConversations()` returns `{ conversations, loading, error, hasMore, loadMore, reload }`;
`useMessages(id)` returns `{ messages, loading, error, hasMore, loadOlder, send, markRead }`. Both
follow realtime events. Start a new conversation with `client.startConversation()`.

## Typed /v1 client

```ts
import { createYuvaApi } from "useyuva/api";

const api = createYuvaApi({ server: "https://support.example.com", apiKey: process.env.YUVA_API_KEY! });
const { data, error } = await api.GET("/v1/inboxes");
```

Built on [openapi-fetch](https://openapi-ts.dev/openapi-fetch/); paths, parameters and bodies are
typed from the contract. Keep API keys on the server.

## Server

For your backend: sign identity tokens, verify webhooks and send server-side analytics events. It
uses only Web Crypto, `Request` and `fetch`, so it runs on Node, Bun, Deno and workers, and directly
under [srvx](https://srvx.h3.dev):

```ts
import { serve } from "srvx";
import { createAnalytics, signIdentityToken, verifyWebhook, WebhookVerificationError } from "useyuva/server";

const analytics = createAnalytics({
  server: "https://support.example.com",
  apiKey: process.env.YUVA_API_KEY!, // analytics:write
  propertyId: process.env.YUVA_ANALYTICS_PROPERTY!,
  onError: (error) => console.warn("yuva analytics", error),
});

serve({
  async fetch(request) {
    const { pathname } = new URL(request.url);
    if (pathname === "/yuva-token") {
      const user = await currentUser(request); // your session
      const token = await signIdentityToken(process.env.YUVA_IDENTITY_SECRET!, {
        subject: user.id, // required: your user id
        email: user.email,
        emailVerified: user.emailConfirmed, // only then does Yuva match the address to a contact
        name: user.name,
        attrs: { plan: user.plan },
      });
      return Response.json({ token });
    }
    if (pathname === "/webhooks/yuva" && request.method === "POST") {
      try {
        const event = await verifyWebhook(process.env.YUVA_WEBHOOK_SECRET!, request);
        // Ignore a webhook-id you have already handled; Yuva retries until it gets a 2xx.
        if (event.type === "message.created" && !event.data.contact?.online) {
          // send your push notification
        }
        return new Response(null, { status: 204 });
      } catch (error) {
        if (error instanceof WebhookVerificationError) return new Response("bad signature", { status: 401 });
        throw error;
      }
    }
    if (pathname === "/signup" && request.method === "POST") {
      // ...
      analytics.track(request, { name: "Signup", props: { plan: "pro" }, revenue: { amount: "29.90", currency: "EUR" } });
      request.waitUntil?.(analytics.flush());
      return Response.redirect(new URL("/welcome", request.url), 303);
    }
    return new Response("Not found", { status: 404 });
  },
});
```

`signIdentityToken(secret, claims)` returns the HS256 token the widget and the mobile SDKs exchange
for a contact session, signed with the inbox's identity secret (shown once when the inbox is created
or its secret is rotated). Claims: `subject` (1 to 200 characters), `email`, `emailVerified`, `name`,
`locale`, `attrs` (at most 16 KiB as JSON), `id` (the `jti`: Yuva then accepts the token only once),
`issuedAt` and `ttl` in seconds (300 by default, at most 600). Sign a new one whenever the page or
app starts a session.

`verifyWebhook(secret, request, { tolerance })` checks the [Standard Webhooks](https://www.standardwebhooks.com)
signature over the raw body with the endpoint's `whsec_…` secret and returns the parsed event, typed
by its `type`. It throws a `WebhookVerificationError` (`code`: `invalid_secret`, `missing_headers`,
`invalid_timestamp`, `timestamp_too_old`, `timestamp_too_new`, `invalid_signature`) when no
signature matches or `webhook-timestamp` is more than `tolerance` seconds (300 by default) from now.
During a secret rotation either secret verifies. When the body is already read,
`verifyWebhookBody(secret, headers, bytes, options)` does the same; `signWebhook(secret, id, time, body)`
signs a request for testing a handler.

`createAnalytics({ server, apiKey, propertyId })` reports events only your backend sees, such as a
signup or a payment, to an analytics property with an API key that has the `analytics:write` scope.
`track(request, event)` forwards the visitor's address, `User-Agent`, `Sec-CH-UA-Platform`,
`Sec-CH-UA-Mobile` and `Accept-Language` (never stored by Yuva), so the event counts as the same
visitor as their browser that day, and takes the URL and referrer from the request when the event
leaves them out; requests with `DNT: 1` or `Sec-GPC: 1` send nothing. `send(event)` queues an event
as it is, such as one with your own `visitorKey` (a session id, never an e-mail address) instead of
`ip` and `userAgent`. Both throw for an event without a name or URL and after `close()`.

The visitor's address is read only from `request.ip`, which srvx sets from the connection. Behind a
proxy, or on a runtime that does not set it, pass `clientIP: (request) => …` returning the address
your proxy reports, such as `request.headers.get("cf-connecting-ip")` on Cloudflare Workers; a
forwarded header is trustworthy only when your own proxy sets it.

Events are sent in batches of up to 100 every `flushInterval` (5000 ms) or as soon as a batch is
full, holding at most `queueSize` (10,000) events. Network errors, `429` and `5xx` are retried
`maxRetries` times (5) with a backoff from `minBackoff` (1000 ms) doubling up to a minute, never
sooner than `Retry-After`. Dropped events, refused batches (`AnalyticsBatchError`) and events the
server left out (`AnalyticsEventError`) go to `onError`. The timers never keep a process alive:
`flush()` sends what is queued and resolves to the last batch error, if any, without rejecting, so
it can go to `waitUntil`; `close()` flushes and stops the client before the process exits.

### h3

`useyuva/h3` wraps the same helpers for an [h3](https://h3.dev) v2 app (`h3` is an optional peer
dependency):

```ts
import { H3, serve } from "h3";
import { defineYuvaAnalytics, defineYuvaIdentityHandler, defineYuvaWebhookHandler } from "useyuva/h3";
import { createAnalytics } from "useyuva/server";

const app = new H3()
  .use(
    defineYuvaAnalytics({
      analytics: createAnalytics({
        server: "https://support.example.com",
        apiKey: process.env.YUVA_API_KEY!,
        propertyId: process.env.YUVA_ANALYTICS_PROPERTY!,
      }),
    })
  )
  .get(
    "/yuva-token",
    defineYuvaIdentityHandler({
      secret: process.env.YUVA_IDENTITY_SECRET!,
      user: async (event) => {
        const user = await currentUser(event); // your session; null when signed out
        return user && { subject: user.id, email: user.email, emailVerified: user.emailConfirmed, name: user.name };
      },
    })
  )
  .post(
    "/webhooks/yuva",
    defineYuvaWebhookHandler({
      secret: process.env.YUVA_WEBHOOK_SECRET!,
      on: {
        "message.created": async (payload) => {
          if (!payload.data.contact?.online) await sendPush(payload.data);
        },
        "conversation.rated": (payload) => saveRating(payload.data.conversation),
      },
    })
  )
  .post("/signup", async (event) => {
    // ...
    event.context.yuva?.track({ name: "Signup", props: { plan: "pro" } });
    return Response.redirect(new URL("/welcome", event.req.url), 303);
  });

serve(app);
```

```html
<yuva-chat channel="yuva_pk_…" server="https://support.example.com"></yuva-chat>
<script type="module">
  import "useyuva/chat";
  document.querySelector("yuva-chat").setIdentityToken(async () => {
    const response = await fetch("/yuva-token", { credentials: "same-origin" });
    return response.ok ? (await response.json()).token : null;
  });
</script>
```

`defineYuvaIdentityHandler({ secret, user, ttl })` answers `{ "token": "…" }` with a token for the
claims `user(event)` returns, with `Cache-Control: no-store`, and `401` when it returns nothing;
`ttl` is the default for claims without their own. `defineYuvaWebhookHandler({ secret, tolerance, on })`
verifies the raw request with `verifyWebhook` and calls the handler for the event's `type` with the
event typed for it; it answers `401` when the signature does not verify, `204` for a type without a
handler, and otherwise the `Response` the handler returns or `204`. A handler that throws answers
`500`, so Yuva retries. `defineYuvaAnalytics({ analytics })` is a middleware that puts
`track(event)` (`analytics.track` with the request) on `event.context.yuva` and, after a request
that tracked something, hands `flush()` to the request's `waitUntil`, which srvx sets on every
runtime it serves; without one the queue goes out on the client's timer.

### Nitro

`useyuva/nitro` is the same server part for a [Nitro](https://nitro.build) v3 app:

```ts
// nitro.config.ts
import { defineConfig } from "nitro";

export default defineConfig({
  serverDir: "./server",
  modules: ["useyuva/nitro"],
});
```

```ts
// server/yuva.ts
import { getCookie } from "nitro/h3";
import { defineYuvaServer } from "useyuva/nitro/runtime";

export default defineYuvaServer({
  user: async (event) => {
    const user = await findSession(getCookie(event, "session")); // null when signed out
    return user && { subject: user.id, email: user.email, emailVerified: user.emailConfirmed, name: user.name };
  },
  webhooks: { "conversation.rated": (payload) => saveRating(payload.data.conversation) },
});
```

The variables are `NITRO_YUVA_IDENTITY_SECRET`, `NITRO_YUVA_WEBHOOK_SECRET`,
`NITRO_YUVA_ANALYTICS_API_KEY`, `NITRO_YUVA_ANALYTICS_PROPERTY_ID` and `NITRO_YUVA_SERVER` (the Yuva
server analytics goes to), or `yuva({ identitySecret, webhookSecret, analytics: { apiKey, propertyId }, server })`
from `useyuva/nitro` in `modules`; routes, `track` and the build-time rule are as in [Nuxt](#nuxt).
`yuva.ts` is read from `serverDir`, or the root without one. The page asks for the token as in the
h3 example, at `/_yuva/identity`.

## License

MIT

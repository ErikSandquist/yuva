# useyuva

JavaScript SDK for [Yuva](https://github.com/productdevbook/yuva), open-source customer messaging.

| Import | What |
|---|---|
| `useyuva` | `createYuvaClient()`: the contact side (`/client/v1`) without DOM. Browsers, Node 22+, Bun, Deno, workers. |
| `useyuva/chat` | The `<yuva-chat>` web component, built on that client. Browser only. |
| `useyuva/docs` | `<yuva-page-feedback>` and `<yuva-discussions>` for documentation pages. Registers in the browser only, so it is safe to import during SSR. |
| `useyuva/subscribe` | `<yuva-subscribe>`, a mailing list sign-up form. Registers in the browser only. |
| `useyuva/react` | `YuvaProvider`, `useConversations`, `useMessages`. React 18+ is an optional peer. |
| `useyuva/nuxt` | A Nuxt module: `YuvaChat`, `YuvaPageFeedback`, `YuvaDiscussions` and `YuvaSubscribe` components, configured once in `nuxt.config`. |
| `useyuva/api` | A typed `/v1` client for backends (API key), generated from the OpenAPI contract. |

```sh
npm install useyuva
```

Without a bundler, the server hosts the widget itself: `<script src="https://<your-yuva>/yuva.js" defer></script>`,
the documentation page elements without the chat: `<script src="https://<your-yuva>/yuva-docs.js" defer></script>`,
and the subscribe form: `<script src="https://<your-yuva>/yuva-subscribe.js" defer></script>`.

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

## Nuxt

```ts
// nuxt.config.ts
export default defineNuxtConfig({
  modules: ["useyuva/nuxt"],
  yuva: { server: "https://support.example.com", channel: "yuva_pk_…" },
});
```

`NUXT_PUBLIC_YUVA_SERVER`, `NUXT_PUBLIC_YUVA_CHANNEL` and `NUXT_PUBLIC_YUVA_LOCALE` override the
options at runtime. `<YuvaChat />`, `<YuvaPageFeedback />`, `<YuvaDiscussions />` and `<YuvaSubscribe />` are
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

## License

MIT

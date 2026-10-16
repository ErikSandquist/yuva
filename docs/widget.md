# Web widget

`<yuva-chat>` puts live chat on your website, or a conversation thread inside your own product. It
works in any page or framework.

## 1. Create a chat channel

In the panel: Settings → the inbox → Channels → Chat. Add every origin that may show the widget,
such as `https://www.example.com`, then copy the channel's public key (`yuva_pk_…`). The key is not
a secret.

## 2. Embed it

```html
<script src="https://support.example.com/yuva.js" defer></script>
<yuva-chat channel="yuva_pk_xxxxxxxxxxxxxxxx"></yuva-chat>
```

This shows a floating launcher. With a bundler: `npm install useyuva`, `import "useyuva/chat"` and
set the `server` attribute. In Nuxt: `modules: ["useyuva/nuxt"]`, `yuva: { server, channel }` in
`nuxt.config` and `<YuvaChat />`. If your site sends a Content Security Policy, allow your Yuva
server in `script-src` and `connect-src` (`https://` and `wss://`).

## 3. Signed-in users

Your backend signs an [identity token](identity.md) for the current user; the widget asks for it:

```js
document.querySelector("yuva-chat").setIdentityToken(async () => {
  const response = await fetch("/api/yuva-identity-token");
  return response.ok ? (await response.json()).token : null;
});
```

Return `null` for anonymous visitors. Call it again after sign-in, and `signOut()` after sign-out.

## What the widget stores

The widget keeps its session in your page's `localStorage`, so a visitor's conversation survives a
reload. Everything is scoped to your page's origin.

| Key | Holds |
|---|---|
| `yuva:<channel>` | The session token and its expiry, the anonymous visitor id and the launcher state. |
| `yuva:reader:<server> <channel>` | The reader token, after a reader signs in on a documentation page. |

The tokens are random strings that Yuva stores only hashed and can end at any time. Because the
storage belongs to your origin, a script injected into your page (XSS) can read them and act as that
visitor until the token expires or is ended. What you can do:

- **Sign out.** `signOut()` ends the session on the server and removes it from storage; the visitor id
  and launcher state stay. `readerSignOut()` ends the reader token and clears its key. Call them when
  your user signs out of your app.
- **Use identity tokens** for signed-in users. A [short-lived token](identity.md) signed by your
  backend proves who the user is, so the stored session is not the only thing that vouches for them.
- **Send a Content Security Policy** that limits `script-src` to sources you trust; it is the main
  protection against injected scripts.
- **Keep the token out of `localStorage`** when you build your own UI: `createYuvaClient()` takes a
  `storage` option, any object with `getItem` and `setItem`, for example one that wraps
  `sessionStorage` or keeps the values in memory. The `<yuva-chat>` element has no such option and
  always uses `localStorage`; if `localStorage` is unavailable, memory is used.

## Troubleshooting

- **Nothing appears**: the page's origin is not allowed (the console names it), or the key is wrong.
- **"Sign in to chat with us."**: the channel is closed to anonymous visitors and no token was given.

## Reference

### Attributes

| Attribute | Meaning |
|---|---|
| `channel` | The chat channel's public key. Required. |
| `server` | Yuva server URL. Defaults to where `yuva.js` came from. |
| `layout` | `launcher` (default) or `embedded`, for a thread inside your page; give it a height. |
| `locale` | `en`, `tr` or `de`. Defaults to the browser's language, then English. |
| `dir` | `ltr` or `rtl`. Defaults to the locale's direction. |
| `identity-token` | A token string; `setIdentityToken` is usually better. |
| `open` | Present: the panel is open. |

### Methods and events

| Member | Meaning |
|---|---|
| `open()`, `close()`, `toggle()`, `isOpen` | The launcher's panel. |
| `unread`, `yuva-unread` event | Replies not yet seen; the event has `detail.count`. |
| `setIdentityToken(fn)`, `signOut()` | Signed-in users, as above. |
| `rate(conversationId, "good" \| "bad", comment?)` | Rates a closed conversation; `409 already_rated` or `rating_unavailable` when it cannot. |

### Channel settings

| Setting (API field) | Meaning |
|---|---|
| Allowed origins (`allowed_origins`) | 1 to 20 origins, compared exactly. |
| Anonymous visitors (`allow_anonymous`) | Visitors without an identity token may chat. |
| Ask for e-mail (`ask_email_offline`) | Ask for an address when nobody is available. Unread replies are mailed after `YUVA_CHAT_EMAIL_DELAY` (default 5 minutes). Needs an e-mail channel in the inbox. |
| Greeting (`greeting`) | Shown before the first message. |
| Launcher (`launcher`) | `position` and `color` (`#rrggbb`). |

Over the API: `POST /v1/inboxes/{inboxId}/channels` with `kind: "chat"` and a `chat` object, as an
owner or admin.

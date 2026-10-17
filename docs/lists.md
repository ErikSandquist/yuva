# Mailing lists and subscribe forms

A mailing list belongs to an inbox and holds the people who agreed to hear from you. Visitors join
with a form on your site, your backend adds people it already has consent for, and every change is
written to a consent log you can export.

A list's confirmation and welcome mails go out through the inbox's [e-mail channel](email.md), from
its address and SMTP account. Yuva sends nothing for a list that has no e-mail channel.

## 1. Set the sender identity

Mail with marketing content must say who sends it and how to reach them. In the panel: Settings →
the inbox → Sender identity. Fill in the legal name, a postal address, a registration (a MERSİS
number, trade register or VAT id) and a contact (an e-mail address, phone number or URL). Yuva puts
these in the footer of the inbox's list mail.

Over the API: `PATCH /v1/inboxes/{inboxId}` with a `sender` object.

## 2. Create a list

In the panel: Broadcasts, then New list. Pick the inbox and the e-mail channel that sends its mail.

| Setting (API field) | Meaning |
|---|---|
| Name (`name`) | Up to 100 characters, unique in the inbox. |
| Description (`description`) | Up to 500 characters, shown on forms and the preference page. |
| Double opt-in (`double_opt_in`) | On by default. Form subscribers confirm by e-mail before they are subscribed. |
| Public (`public`) | On by default. Offered on forms and the preference page. Private lists are filled by the API and your team only. |
| Commercial (`commercial`) | On by default. Marks marketing content; it is part of the consent export. |
| Welcome (`welcome`) | On by default. Sends a receipt with an unsubscribe link when someone subscribes. |
| Sending channel (`channel_id`) | An e-mail channel of the same inbox with an SMTP account. Without it, forms send nothing and subscriptions stay `pending`. |

Over the API: `POST /v1/lists` with `inbox_id` and `name`.

## 3. Add the form to your site

The form uses a [chat channel](widget.md#1-create-a-chat-channel) of the same inbox: its public key
and allowed origins decide where the form may appear. The list's page in the panel has a Form tab
with these snippets filled in for you.

```html
<script src="https://support.example.com/yuva-subscribe.js" defer></script>
<yuva-subscribe
  channel="yuva_pk_xxxxxxxxxxxxxxxx"
  lists="LIST-ID"
  privacy-url="https://www.example.com/privacy"
></yuva-subscribe>
```

Leave `lists` out to offer every public list of the inbox, with a checkbox each. The element asks for
an e-mail address and shows an unticked consent box. The visitor cannot subscribe without ticking
it, and the exact sentence they saw is stored as proof.

In a bundler project, run `npm install useyuva` and use one of these:

| Stack | Code |
|---|---|
| Any bundler | `import "useyuva/subscribe"`, then the element above with a `server` attribute. |
| React | `import { YuvaSubscribe } from "useyuva/react"` and `<YuvaSubscribe channel server lists={["LIST-ID"]} />`. |
| Nuxt | `modules: ["useyuva/nuxt"]` and `yuva: { server, channel }` in `nuxt.config`, then `<YuvaSubscribe :lists="['LIST-ID']" />`. |
| shadcn/ui | `npx shadcn@latest add https://useyuva.com/r/yuva-subscribe.json`, then `<YuvaSubscribe server channel lists={["LIST-ID"]} />`. |

If your site sends a Content Security Policy, allow your Yuva server in `script-src` and
`connect-src`. For signed-in users, give the element an [identity token](identity.md) with
`setIdentityToken`: it then shows a switch per list instead of the e-mail field, and subscribes at
once when the user's address is verified.

For in-app subscribe toggles, see [Mobile SDKs](mobile.md#5-newsletter-subscriptions).

## 4. What subscribers see

1. They fill in the form. The answer is always the same, whether the address is new, already
   subscribed, unsubscribed or unknown, so the form tells nobody who is on your list.
2. With double opt-in they get a mail with a link to `/s/…`. Opening the page changes nothing; the
   button on it confirms. Then the welcome mail follows, if the list sends one.
3. Every list mail has a link to `/m/…`, where the person unsubscribes from a list, from
   everything, or picks lists. Opening it changes nothing either. Mail programs that support
   one-click unsubscribe (RFC 8058) unsubscribe the list with a single request.

The pages and mails come in English, Turkish and German. Only the person can subscribe again after
unsubscribing: your team and your keys cannot, and an address that unsubscribed from everything,
reported a mail as spam or was erased after objecting is refused (`409 opted_out`). Pending
subscriptions are deleted after 30 days; unsubscribed ones after three years.

## 5. Add subscribers from your backend

For a whole file of people, see [Importing subscribers](importing.md). Use this for people who agreed
somewhere else, such as at sign-up. You declare how they agreed, and
Yuva stores it as proof. The key needs the `lists:write` scope.

```sh
curl -X POST https://support.example.com/v1/lists/LIST-ID/subscriptions \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Idempotency-Key: signup-1234" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "ada@example.com",
    "name": "Ada",
    "consent": {
      "confirmed": true,
      "obtained_at": "2026-10-01T09:30:00Z",
      "text": "Send me product news by e-mail.",
      "page_url": "https://www.example.com/signup",
      "reference": "order-1234"
    }
  }'
```

With `confirmed: true` the subscription is `subscribed` at once and the welcome mail goes out. With
`confirmed: false` it is `pending` and the person gets a confirmation mail first; use that when you
are not sure they agreed. `201` is a new subscription, `200` an address that was already on the list.

## 6. Export consent records

Open the list, then Export, or call `GET /v1/lists/{listId}/consent-export` (`format=csv` or `json`,
`since` for newer events; for Turkish senders see Türkiye: İYS below). Each row is one event with the address, state, time, source, IP address,
page and the words the person saw. `GET /v1/contacts/{contactId}/consent` returns one person's
records across lists, for an access request under GDPR Art. 15 or KVKK Art. 11. Cells that start
with `=`, `+`, `-` or `@` are prefixed so spreadsheets do not run them.

## Türkiye: İYS

Yuva does not connect to İYS. It gives you a file to upload there yourself; registering as a sender in
İYS is your task too.

Open the list, then Export, and choose **Download for İYS**, or call
`GET /v1/lists/{listId}/consent-export?format=iys`. Set a date in "From (optional)", or pass `since`, to
get only the people who agreed or left from then on. The file is a CSV with one row per subscriber:

| Column | Value |
|---|---|
| `recipient` | The e-mail address. |
| `type` | `EPOSTA`. |
| `source` | `HS_WEB` for subscriptions from a form or the preference page, `HS_EORTAM` for the rest. |
| `status` | `ONAY` for subscribed, `RET` for unsubscribed. |
| `consentDate` | When the person agreed or left, in Türkiye's time (`YYYY-MM-DD HH:MM:SS`). |
| `recipientType` | `BIREYSEL`. |

Pending subscriptions, which have not confirmed yet, are left out. The columns follow İYS's consent
fields as we understand them. Check them against what your İYS account asks for before you upload, and
register only the lists that send commercial mail. This page is not legal advice.

## Troubleshooting

- **Form shows no list**: the list is private or archived, or `lists` names an id that is not a
  public list of the channel's inbox.
- **The form shows an error**: the page's origin is not one of the chat channel's allowed
  origins.
- **No confirmation mail**: the list has no sending channel (the panel marks it), or the address
  already got 3 confirmations today.
- **`409 email_channel_required`**: the channel is not an e-mail channel of the list's inbox with an
  SMTP account.

## Reference

### `<yuva-subscribe>` attributes

| Attribute | Meaning |
|---|---|
| `channel` | The chat channel's public key. Required. |
| `server` | Yuva server URL. Defaults to where `yuva-subscribe.js` came from. |
| `lists` | Comma-separated list ids. Empty: every public list. |
| `layout` | `stacked` (default) or `inline`. |
| `name` | `off` (default), `optional` or `required`: adds a name field. |
| `button-text` | The button's label. |
| `consent` | Your own consent sentence. It is stored exactly as shown. |
| `privacy-url` | Link to your privacy notice, shown next to the box. |
| `locale`, `dir` | One of the 47 widget languages (`en`, `tr`, `de`, `pt-BR`, `ar`, ...; see the [web widget](widget.md#languages)), and `ltr` or `rtl`. |
| `identity-token` | A token string; `setIdentityToken` is usually better. |

Style it with `--yuva-accent`, `--yuva-on-accent`, `--yuva-radius`, `--yuva-font` and
`::part(field|button|consent|status)`.

### Events

| Event | `detail` |
|---|---|
| `yuva-subscribe` | `{ lists, confirmation }`: the ids offered and whether a confirmation mail is expected. |
| `yuva-subscribe-error` | `{ code }`. |

React and shadcn: `onSubscribe`, `onError`. Nuxt: `@subscribe`, `@error`.

### Endpoints

| Endpoint | Use |
|---|---|
| `GET`, `POST /v1/lists` | List and create lists (`archived=true` lists archived ones). |
| `GET`, `PATCH`, `DELETE /v1/lists/{listId}` | Read, change, archive (`archived: true`) or delete a list. |
| `GET /v1/lists/{listId}/growth` | New, confirmed and left per day, `days=30` or `90`. |
| `GET`, `POST /v1/lists/{listId}/subscriptions` | List (`state`, `q`) and add subscribers. |
| `GET`, `PATCH`, `DELETE /v1/lists/{listId}/subscriptions/{subscriptionId}` | Read with consent events, unsubscribe (`state: unsubscribed`), delete. |
| `POST /v1/lists/{listId}/subscriptions/{subscriptionId}/confirmation` | Mail the confirmation again. |
| `GET /v1/lists/{listId}/consent-export` | Consent records of a list. |
| `GET /v1/contacts/{contactId}/subscriptions` | A contact's subscriptions. |
| `GET /v1/contacts/{contactId}/consent` | A contact's consent records. |
| `GET /client/v1/channels/{channel_key}/lists` | Public lists and a form token, for the form. |
| `POST /client/v1/channels/{channel_key}/subscriptions` | Subscribe from a form. |
| `GET /client/v1/subscriptions`, `PUT`, `DELETE /client/v1/subscriptions/{listId}` | A signed-in user's lists, subscribe, unsubscribe. |

Full request and response shapes are in the [API contract](../openapi/openapi.yaml). For your own
forms, `useyuva` has `lists()`, `subscribe()`, `subscriptions()`, `subscribeList()` and
`unsubscribeList()` on the [headless client](headless.md).

### Scopes and events

| Name | Meaning |
|---|---|
| `lists:read` | Read lists, subscriptions, growth and consent exports. |
| `lists:write` | Create, change and delete lists; add, unsubscribe and delete subscribers; resend confirmations. |
| `subscription.created` | [Webhook](webhooks.md) event: someone subscribed, in any state. |
| `subscription.updated` | Webhook event: the state changed; `data.previous_state` holds the old one. |

Keys made before these scopes existed do not get them.

### Limits

| Limit | Value |
|---|---|
| Lists offered by one form | 10 |
| Form requests per IP address | 10 per hour |
| Form requests per channel | 600 per hour |
| Confirmation mails per address | 3 per day, and 1 per list in 10 minutes |
| Confirmation mails per channel | 1,000 per day |
| Confirmations resent from the panel or API | 3 per subscription and day, 10 in all |
| Form token | Valid from 2 seconds to 2 hours after it was issued |

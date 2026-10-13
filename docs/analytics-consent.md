# Consent features

By default Yuva Analytics cannot recognise a visitor from one day to the next ([Analytics and privacy](analytics-privacy.md)).
Consent features change that for visitors who agreed: with their yes, your site or app keeps a random id on
their device, and you get returning visitors, retention, funnels that run over several days and, for signed-in
users, their recent visits on their contact page. Everyone who did not agree is counted exactly as before.

This builds on [Analytics](analytics.md) and its [consent mode](analytics.md#consent-mode). You bring your own consent tool;
Yuva never shows a banner.

## What changes with consent

| With consent to `persistent` | What you get |
|---|---|
| A random id on the device | Made by the script or app, kept for at most 13 months from when it was made, and sent with every event. The server stores only a hash of it under a secret of the property, so the same id means nothing on another property. |
| Visitors counted once | A visitor counts once over a range, a week or a month instead of once a day. |
| New and returning | The **New and returning** card on the overview, and the `visitor_type` dimension: `new` on the day the id was first seen, `returning` after it. Empty for everyone else. |
| Retention | The **Retention** tab, and `GET /v1/analytics/properties/{propertyId}/retention`: cohorts by day, week or month and how many of each came back. |
| Funnels over days | A funnel's `consented_visitors` follows these visitors across visits and days, each counted once. |
| With `identify` as well | The contact page's **Recent visits**, and `GET /v1/contacts/{contactId}/visits`. |

Only consenting visitors are in these views. A property that does not use the feature has no retention cohorts.

### What never happens without consent

- Nothing is stored on the device. The only exception is a visitor's own opt-out.
- No id is made, sent or read, and every visitor is counted by the hash of the day, as in the default mode.
- A visitor is never joined to a contact or a conversation: the contact page lists no visits.
- A browser that sends Do Not Track or Global Privacy Control gets no id, whatever the consent tool says.
- A token sent without consent, or with a forged, expired or already used signature, is ignored.

## Turn it on for a property

In the panel, open the website's **Settings → Tracking → Consent features**. An owner or admin sets two switches
(they are off and disabled for children's properties); through the API they are the property's `consent_features`:

| Switch | `consent_features` | Does |
|---|---|---|
| **Persistent id** | `persistent` | Lets visitors who agree get the 13-month id. |
| **Link signed-in users** | `identify` | Lets the id be linked to a contact. Needs **Persistent id** and a website linked to an inbox, whose [identity secret](identity.md) signs the tokens. |

Or with an `analytics:manage` key:

```sh
curl -X PATCH -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  https://support.example.com/v1/analytics/properties/$PROPERTY \
  -d '{"consent_features": ["persistent", "identify"]}'
```

Without the setting, `persistent` and `identify` are ignored and visitors are counted in the default mode.

**Children's properties cannot use them.** A property with **Audience: Children** refuses `consent_features`
(`400`): a child's device is never given an id, and no child is linked to a contact.

Tell your visitors. Add to your privacy notice that a consenting visitor gets an id on their device for up to 13
months, what it is for and how they withdraw ([withdrawal](#withdrawal-and-erasure)).

## Call the script from your consent tool

The script keeps no answer, so call it on **every page load** with the visitor's current choice, and pass
`persistent: true` on **every** call while the visitor agrees to it: any `consent` call without it, including
`consent({ analytics: true })`, withdraws a stored id.

```js
window.yuva.consent({ analytics: true, persistent: true })
window.yuva.consent(false)
```

`consent({ analytics: true })` counts the visitor without an id. Adding `persistent: true` makes the id and keeps it
in the browser's local storage. `consent(false)`, a call without `persistent: true` and an opt-out stop the id: it
is deleted from the device and the server forgets its link to a contact and its retention days, even when the
browser sends Do Not Track. Before the script has loaded, queue the call:
`;(window.yuvaq = window.yuvaq || []).push(["consent", { analytics: true, persistent: true }])`.

**Script tag.** The usual `/yuva-analytics.js` leaves the consent features out to stay small. To use them, load
`/yuva-analytics-consent.js` (under 2 KB gzip) from the same place instead, with the same attributes, and set
`data-consent="required"` as in [consent mode](analytics.md#consent-mode). `window.yuva` then has `identify` and
`persistentId` too. With `useyuva/analytics`, React and Nuxt the calls are there already.

### Recipes

These are examples for the consent tools below, using the callbacks their documentation names. Check
your tool's own documentation, because names and categories change; what matters is that `consent(...)` is
called with the visitor's choice every time a page loads and every time the choice changes.

**Any tool.** Write one function and call it from whatever your tool calls on load and on change:

```js
function syncYuva(analytics, persistent) {
  if (analytics) window.yuva.consent({ analytics: true, persistent })
  else window.yuva.consent(false)
}
```

**Cookiebot.** Cookiebot fires `CookiebotOnConsentReady` on every page load and when the choice changes; the
**statistics** category is `Cookiebot.consent.statistics`.

```js
window.addEventListener("CookiebotOnConsentReady", () => {
  const ok = Cookiebot.consent.statistics
  ok ? window.yuva.consent({ analytics: true, persistent: true }) : window.yuva.consent(false)
})
```

**OneTrust.** `OptanonWrapper` runs on load and after each change, and `OnetrustActiveGroups` lists the
accepted category ids, `C0002` being **Performance**:

```js
function OptanonWrapper() {
  const ok = window.OnetrustActiveGroups.includes("C0002")
  ok ? window.yuva.consent({ analytics: true, persistent: true }) : window.yuva.consent(false)
}
```

**Klaro.** Add a service to the Klaro config; its `callback` gets the choice on load and on change:

```js
services: [{
  name: "yuva",
  purposes: ["analytics"],
  callback: (consent) => {
    consent ? window.yuva.consent({ analytics: true, persistent: true }) : window.yuva.consent(false)
  },
}]
```

**CookieYes.** The `cookieyes_consent_update` event on `document` carries the accepted categories in
`event.detail.accepted` when the choice changes, and `getCkyConsent()` answers on load:

```js
const apply = (ok) =>
  ok ? window.yuva.consent({ analytics: true, persistent: true }) : window.yuva.consent(false)

document.addEventListener("cookieyes_consent_update", (event) => {
  apply(event.detail.accepted.includes("analytics"))
})
apply(getCkyConsent().categories.analytics)
```

If your tool has a separate switch for the id, pass its answer as `persistent` instead of `true`.

## Link the visit to a contact

For signed-in users on a property that offers `identify`, hand the script a token your backend signs, as
for the [widget](identity.md): the same inbox, the same secret and the same claims, with `exp` at most 10
minutes ahead and a `jti` that is used once. After the visitor signs in:

```js
const { token } = await fetch("/yuva-identity").then((r) => r.json())
window.yuva.identify(token)
```

`/yuva-identity` is your own endpoint, the one the [identity guide](identity.md#2-sign-a-token-for-the-current-user)
describes. Yuva checks the token as it checks one for a chat session, finds or makes the contact for its `sub`,
and links the id and its visits to it. The visits show on the contact page and at
`GET /v1/contacts/{contactId}/visits` (scopes `contacts:read` and `analytics:read`), for as long as the detailed
records are kept. Call `identify` again after every sign-in; a token cannot be used twice. It does nothing for a
visitor who has not agreed to both analytics and `persistent`.

Deleting a contact deletes the visits linked to it; merging two contacts moves them.

## Withdrawal and erasure

A visitor who withdraws in your consent tool: your callback calls `consent(false)`. The id leaves the
device, and the server deletes the id's link to a contact and its retention days. A withdrawal is taken even
when the browser sends Do Not Track.

A person who asks you to **erase** what you hold about them: delete everything stored under their id with a
key that has `analytics:manage`:

```sh
curl -X DELETE -H "Authorization: Bearer $YUVA_API_KEY" \
  https://support.example.com/v1/analytics/properties/$PROPERTY/visitors/$PERSISTENT_ID
```

It deletes the id's raw visits and events, its retention days and its link to a contact, rolls the finished
days up again without them, and answers `204` whether or not anything was stored. The id is 16 to 64 letters,
digits, `-` and `_`. The person's browser knows it: `window.yuva.persistentId()` returns it, so a "Delete my data" button on your site can send it to your backend, which makes the call
with your key. Yuva stores only a hash of the id, so you cannot find it from a name or an e-mail address; to
erase a signed-in user's contact as well, see [Delete a user](identity.md#delete-a-user).

## Server events

A backend that reports events for a visitor who agreed can name them. Send the id the browser holds as
`persistent_id`, with `consent: true`, to `POST /v1/analytics/properties/{propertyId}/events`:

```json
{
  "events": [{
    "name": "Signup",
    "url": "https://www.example.com/signup",
    "persistent_id": "0f8fad5b-d9cb-469f-a165-70867728950e",
    "consent": true
  }]
}
```

The event then belongs to the visitor's days, as the browser's own events do. With `identity_token` as well
(on a property that offers `identify`) it identifies the visitor. Without `consent: true` the id is ignored. The
other fields are in [Server events](analytics-server.md).

## Apps

The Swift and Kotlin libraries have the same calls as the script. Your own consent screen decides when to ask. The
id lives in the app's private storage for at most 13 months. As on the web, pass `persistent: true` on every
`consent` call while the user agrees: a call without it withdraws the id.

```swift
YuvaAnalytics.consent(true, persistent: true)
YuvaAnalytics.identify(token)
let id = YuvaAnalytics.persistentId
YuvaAnalytics.consent(false)
```

```kotlin
YuvaAnalytics.consent(true, persistent = true)
YuvaAnalytics.identify(token)
val id = YuvaAnalytics.persistentId
YuvaAnalytics.consent(false)
```

`identify` and a withdrawal are sent once and are never queued. `persistentId` is `nil` or `null` when the user has
no id. Call `consent` on every launch, as in [Consent and opt-out](analytics-apps.md#consent-and-opt-out).

### Answers for the stores

These apply when `persistent` is on and the user agreed; without it, the [answers on the apps page](analytics-apps.md#answers-for-the-stores) stand.
The id is random, made by the library, not the advertising id, and it is shared with no one.

**App Store.** The Swift library's privacy manifest declares **Device ID** (collected for analytics, not used for
tracking), so Xcode's privacy report already lists it. In App Store Connect answer:

| Data type | Linked to the user | Used for tracking | Purpose |
|---|---|---|---|
| Identifiers → Device ID | No, unless your app calls `identify` | No | Analytics |
| Identifiers → User ID, only if you call `identify` | Yes | No | Analytics |

Yuva combines the id with no data from other companies' apps or sites, so it is not tracking in Apple's sense;
answer **No** to tracking unless your own use says otherwise.

**Google Play.** In **Data safety**, still without `AD_ID`:

| Data type | Collected | Shared | Notes |
|---|---|---|---|
| Device or other IDs | Yes | No | The random id, optional (the user chooses). Purpose: Analytics. |
| Personal info → User IDs, only if you call `identify` | Yes | No | Optional. Purpose: Analytics. |

Mark both as **optional** in the form, because the user can decline, and add that data can be deleted on request.

## Reference

| Call | Does |
|---|---|
| `PATCH /v1/analytics/properties/{propertyId}` | Sets `consent_features`. Scope `analytics:manage`. |
| `GET /v1/analytics/properties/{propertyId}/retention` | Cohorts of consenting visitors (`grain`: `day`, `week`, `month`; `from`, `to`). Scope `analytics:read`. |
| `DELETE /v1/analytics/properties/{propertyId}/visitors/{persistentId}` | Erases everything stored under an id. Scope `analytics:manage`. |
| `GET /v1/contacts/{contactId}/visits` | A linked contact's recent visits. |
| `POST /v1/analytics/properties/{propertyId}/events` | Server events, with `persistent_id`, `consent` and `identity_token`. |

Retention cohorts go up to 30 days or 12 weeks or months and are rebuilt every hour. An id is forgotten 13 months after
it was first seen. Per address and property, 20 consent changes a minute are taken.

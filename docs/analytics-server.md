# Server-side events

Some things only your backend sees: an account that was created, a payment that went through, a subscription
that renewed. Your server can send them to an analytics property, and an event that carries the visitor's
address and browser joins that visitor's visit on your website. This builds on
[Analytics](analytics.md) and the event rules in [Events, goals and funnels](analytics-events.md).

## Make a key

Server events need an API key; the website's public key (`yuva_ak_…`) does not work for them. Owners and
admins make one in **Settings → API keys** with the scope `analytics:write` and keep it on the server. A key
limited to inboxes reaches the properties of those inboxes. A member's session is refused (`403 forbidden`).

You also need the property's id. The **Go** and **curl** tabs under **Settings → Install** have the snippets
below filled in with it. Events from your backend need no platform ticked in the property's settings.

## Send events

```sh
curl https://support.example.com/v1/analytics/properties/PROPERTY_ID/events \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "events": [{
      "name": "Signup",
      "url": "https://www.example.com/signup",
      "ip": "203.0.113.7",
      "user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
      "props": { "plan": "pro" }
    }]
  }'
```

```json
{ "accepted": 1, "errors": [] }
```

`POST /v1/analytics/properties/{propertyId}/events` takes 1 to 100 events and answers `202`.

| Field | |
|---|---|
| `name` | Required, 1 to 120 characters: `pageview`, a built-in event or your own event's name. The rules are those of [events](analytics-events.md#send-events); `engagement` is for browsers only. |
| `url` | Required, up to 2,000 characters. A page on one of the property's domains, or for a property with iOS or Android an app screen, `app://<app id>/<screen>`. Only host and path are kept. |
| `referrer` | The page's referrer, for a page view that starts a visit. |
| `timestamp` | When it happened, RFC 3339. Now when left out. |
| `props` | Up to 30 text, number or boolean values, with the [rules for props](analytics-events.md#props). |
| `revenue` | `{"a": "29.90", "c": "EUR"}`, as in [Revenue](analytics-events.md#revenue). |
| `interactive` | `false` makes a custom event non-interactive. |
| `ip`, `user_agent`, `sec_ch_ua_platform`, `sec_ch_ua_mobile`, `accept_language` | The visitor's request, [forwarded](#join-the-visitors-visit). |
| `visitor_key` | Your own key for the visitor, instead of `ip` and `user_agent`. |
| `dnt`, `gpc` | The visitor's request had `DNT: 1` or `Sec-GPC: 1`. The event is not stored. |
| `consent` | The visitor agreed to analytics. |

## Join the visitor's visit

Forward what your server received from the visitor, not your server's own address. Yuva shortens the `ip` on
arrival, hashes it with the `user_agent` exactly as it hashes the browser's own request to the website, and
forgets both. The event then belongs to the visitor's visit of that day: the signup shows up in the funnel
after the page views that led to it. Add `sec_ch_ua_platform` and `sec_ch_ua_mobile` (the `Sec-CH-UA-*`
headers, as sent) and `accept_language` when the request had them. The address is also used for the place.

For a visitor without a browser request in hand, such as a webhook or a background job, give a `visitor_key`
instead: your own key for them, such as a session id, **never an e-mail address**. It is hashed under the day's
salt and never stored, and it makes a visitor of its own, apart from the browser's. With it, `ip` only places
the visitor and `user_agent` only describes the device.

A visit that an event starts has the platform **Server**.

## Consent and Do Not Track

Respect the visitor's choices before you send:

- A visitor whose request had `DNT: 1` or `Sec-GPC: 1` should not be sent at all. If you send the event with
  `dnt: true` or `gpc: true`, Yuva takes it but does not store it.
- A property whose **Collection** is **Only with consent** stores an event only with `consent: true`.

## What comes back

The answer is `202` when the request was valid, whatever happened to the events.

- **`errors`** lists the events left out as invalid, each with its `index` in `events` and a `detail` that says why. The others are taken.
- **Taken but not stored**, and counted in **Settings → Health**: events with `dnt` or `gpc`; events without
  consent on a consent property; bots, excluded addresses and paths; events over the per-address or
  per-property limit or the monthly limit; events older than 72 hours (**Too late**).
- **The whole request is refused** for a body that is not JSON or breaks the shape above (`400`), a missing or
  wrong key (`401`), a key without `analytics:write`, or a member's session (`403`), and an unknown property
  (`404`). A busy server answers `503` with `Retry-After`: send the request again after that.

**Settings → Test events** shows what the server counted and dropped while you try it.

## Go

```sh
go get github.com/productdevbook/yuva/sdk/go/analytics
```

```go
import "github.com/productdevbook/yuva/sdk/go/analytics"

yuva, err := analytics.New(analytics.Config{
	BaseURL:    "https://support.example.com",
	APIKey:     os.Getenv("YUVA_API_KEY"),
	PropertyID: "PROPERTY_ID",
})
if err != nil {
	log.Fatal(err)
}
defer yuva.Close(context.Background())

// in a handler: the visitor's address and browser come from r
yuva.Track(r, analytics.Event{
	Name:  "Signup",
	Props: map[string]any{"plan": "pro"},
})
```

`Track` forwards the address, `User-Agent`, `Sec-CH-UA-Platform`, `Sec-CH-UA-Mobile` and `Accept-Language` of `r`,
and takes the `URL` and referrer from `r` when the event leaves them empty. That URL must be on one of the
property's domains, so behind a proxy or an internal host name, set `URL` yourself. A request with `DNT: 1` or
`Sec-GPC: 1` sends nothing and `Track` returns `nil`. Set `Consent: true` on the event when the visitor agreed.
With `VisitorKey` set, `Track` forwards neither the address nor the user agent; put `IP` and `UserAgent` on the
event as well if you want the place and the device.

`Send` queues an event as it is, for a background job without a request.

| `Config` | Default | |
|---|---|---|
| `HTTPClient` | `http.DefaultClient` | Sends the requests. |
| `FlushInterval` | 5 seconds | How often queued events are sent. A full batch of 100 goes at once. |
| `QueueSize` | 10,000 | Events that wait at most; more are dropped with `ErrQueueFull`. |
| `MaxRetries` | 5 | Retries of a batch after a network error, `429` or `5xx`; negative for none. |
| `MinBackoff` | 1 second | The first retry's delay, doubled up to a minute. A longer `Retry-After` wins. |
| `ClientIP` | Host of `r.RemoteAddr` | The visitor's address for `Track`. Behind a proxy, return the address it reports. |
| `OnError` | none | Called with an `*EventError` for an event Yuva left out as invalid, a `*BatchError` for a batch it refused (`400`, `401`, `403`, `404`) or that failed after every retry, and `ErrQueueFull`. |

Events are sent in the background in batches. `Flush(ctx)` sends what is queued now, and `Close(ctx)` sends it
and stops the client: call it before the process exits, or the events still waiting are lost.

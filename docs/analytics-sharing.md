# Sharing and exporting analytics

A share link shows part of a website's numbers to people who have no Yuva account, read-only, and can sit in an
iframe on your own pages. An export writes a website's numbers or raw events to a file you download. Both belong to a
website of [Yuva Analytics](analytics.md); the API calls it a property.

## Share links

A share link is a web address, `<your Yuva address>/share/<token>`, that opens a read-only report of one website
without signing in. Anyone who holds the address sees what the link allows, so send it as carefully as the numbers
deserve.

In the panel open the website, open the Share dialog, and set:

| Setting | Meaning |
|---|---|
| Name | What the link is for. Only members see it. |
| Views | At least one of `overview`, `realtime`, `pages`, `sources`, `locations`, `devices`, `events`, `goals`. These are the tabs of [the views](analytics.md#the-views); a tab that is not listed is not there for the viewer. |
| Password | Optional, 8 to 200 characters. The viewer types it before seeing anything. |
| Expires | Optional. After this time the link stops working. |
| Embed on | Optional list of sites that may show the link in an iframe ([below](#embed-a-share-link)). |

The address and its token (`yuva_sh_…`) are shown **once**, when the link is made. Yuva keeps only a hash of the token
and of the password, so neither can be shown again; make a new link if you lost it. The list keeps the first
characters of the token so you can tell links apart, and when each link was last opened.

To stop a link, **revoke** it: it stops working at once and stays in the list. **Delete** removes it from the list as
well. Changing a link's password, or removing it, ends the access of everyone who had already unlocked it.
Expiry works the same: past `expires_at` the address answers `404`.

Who may do what:

- Owners, admins and API keys without an inbox limit that hold `analytics:manage` make, change, revoke and delete
  links.
- Anyone who can read the website (`analytics:read`) sees the list.
- A website has at most 50 links (`403 share_limit`).

### What a share shows

A link shows the numbers of its views for this website only. Totals and time grains come with every view. Breaking
numbers down or filtering needs the view that owns the column:

| View | Adds |
|---|---|
| `pages` | `page`, `entry_page`, `exit_page`, `hostname` |
| `sources` | `channel`, `source`, `referrer` and the `utm_*` columns |
| `locations` | `country`, `region`, `city` |
| `devices` | `device`, `browser`, `browser_version`, `os`, `os_version`, `screen_size`, `language`, `platform`, `app_version` |
| `events` | `event`, `prop` and the events metric |
| `goals` | the website's goals and their metrics |

A query outside the link's views is refused (`403 view_not_shared`). A share **never** shows saved segments (a
`segment_id` is refused), other websites, the settings, the data tools, the members, or any export.

Viewers use the same query slots and cache as the panel, and the share endpoints are limited per address: 120
requests a minute, and 10 password attempts a minute per link (30 in all). A viewer who unlocks a link gets access
for 12 hours.

### Embed a share link

Put a link on your own page with an iframe:

```html
<iframe
  src="https://support.example.com/share/yuva_sh_xxxxxxxx"
  title="Website statistics"
  width="100%"
  height="800"
  style="border: 0"
  loading="lazy"
></iframe>
```

Add the page's site under **Embed on**: `https://www.example.com` for one site, `https://*.example.com` for every
subdomain of it. Up to 20 entries, each with `https://` or `http://` and a host (and port), nothing after it.

A browser only shows the share page inside a page whose site is listed. Every other site, and every link with an empty
list, gets a page that forbids framing (`frame-ancestors 'none'`), so nobody can put your report in a frame on their
own site. The same goes for an unknown, expired or revoked link. The address itself still opens in a browser tab for
anyone who holds it; the list only decides who may frame it. The share page carries no analytics script, and the
token is kept out of the server's request logs.

## Exports

An export writes numbers to a file on the server in the background. Start it in the panel under Data, then Export, or through the [API](#api). Progress is shown until the file is ready; download it from the same place. A
file is deleted **7 days** after it was made, and you can delete it earlier. A file over 1 GiB fails with
`too_large`, and a workspace runs at most three exports at a time (`429 rate_limited`).

| Kind | What it holds | Formats | Range |
|---|---|---|---|
| `view` | Every row of one query, the table you would see in the panel (no comparison). | CSV, NDJSON | The query's own, at most 400 days |
| `daily` | One row per day with visitors, visits, page views, views per visit, bounce rate, visit duration, time on page and scroll depth, optionally split by one dimension. | CSV, NDJSON | At most 400 days |
| `raw` | Every stored event with its visit's description. | NDJSON only | At most 31 days, all inside the raw window |

`view` and `daily` need `analytics:read`, so anyone who can read the website can export its numbers.

### Raw exports

A `raw` export is for pipelines and your own analysis. It needs `analytics:manage`, and a caller who is an owner, an
admin or an API key without an inbox limit. Raw rows are pseudonymous personal data, so Yuva keeps them to the people
who run the workspace; a plain reader cannot make, list or download one.

The range must be inside the days detailed records are kept
([`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analytics), 90 by default); an older day answers `422 needs_raw_window`.
Bots are left out.

One line per event:

```json
{"day":"2026-10-07","time":"2026-10-07T09:14:31Z","visit":3,"type":"pageview","hostname":"www.example.com","path":"/pricing","channel":"Organic search","source":"Google","referrer":"www.google.com","country":"DE","city":"Berlin","device":"Desktop","browser":"Firefox","os":"Linux","screen_size":"Desktop","language":"de","platform":"web"}
```

| Field | Meaning |
|---|---|
| `day`, `time` | The day in the website's time zone, and the event's time. |
| `visit` | A number that counts up from 1 within this file and joins the events of one visit. |
| `type` | `pageview`, `screen`, `event` or `engagement`. |
| `name`, `props`, `revenue_minor`, `currency` | A custom event's name, props and revenue (in the currency's minor units). |
| `engaged_ms`, `scroll` | On `engagement`: milliseconds in view, deepest scroll in percent. |
| `hostname`, `path` | The page, or in an app the screen. |
| `channel`, `source`, `referrer`, `utm_*` | Where the visit came from, as its first event described it. The referrer is a host, never a path. |
| `country`, `region`, `city`, `device`, `browser`, `browser_version`, `os`, `os_version`, `screen_size`, `language`, `platform`, `app_version` | The visit's place and device, as its first event described them. |

Fields that do not apply are left out of the line.

A raw file **never** holds a visitor hash, the daily salt, an IP address, a contact, a stored visit id or a
referrer's path. `visit` is renumbered for every file, so two files cannot be joined on it and it says nothing about
the visitor. Do not combine a raw file with other data to recognise people; if you hand it to others, treat it
as personal data under your own [privacy notice](analytics-privacy.md).

### CSV and spreadsheets

A CSV cell that starts with `=`, `+`, `-`, `@`, a tab or a carriage return gets a leading `'`. Page paths, referrers
and event names come from visitors, and a cell such as `=HYPERLINK(...)` would otherwise run as a formula when
the file opens in a spreadsheet. The effect: a value like `-5` or `+49 30` is shown as `'-5` or `'+49 30`. A path
such as `/pricing` is untouched. Strip the leading `'` if you read the file with a program; use NDJSON when you need
the values exactly as stored.

## API

Calls take `Authorization: Bearer $YUVA_API_KEY`. Set `PROPERTY` to the website's id.

| Call | What it does |
|---|---|
| `GET /v1/analytics/properties/{propertyId}/shares` | Lists the share links, revoked and expired ones included (`analytics:read`). |
| `POST /v1/analytics/properties/{propertyId}/shares` | Makes a link. The answer carries `token` and `url` once. |
| `PATCH /v1/analytics/properties/{propertyId}/shares/{shareId}` | Changes name, views, password (`null` removes it), embed origins or expiry (`null` keeps the link until revoked). A revoked link answers `409 share_revoked`. |
| `POST /v1/analytics/properties/{propertyId}/shares/{shareId}/revoke` | Revokes a link. |
| `DELETE /v1/analytics/properties/{propertyId}/shares/{shareId}` | Deletes a link. |
| `GET /client/v1/analytics-shares/{token}` | What the share page reads first. No account. |
| `POST /client/v1/analytics-shares/{token}/access` | Unlocks a link with its password and answers a `yuva_sa_…` token for 12 hours. |
| `POST /client/v1/analytics-shares/{token}/query` | The numbers, limited to the link's views. |
| `GET /client/v1/analytics-shares/{token}/realtime` | The last 30 minutes, with the `realtime` view. |
| `GET /v1/analytics/properties/{propertyId}/exports` | Lists the exports until they expire (`analytics:read`). |
| `POST /v1/analytics/properties/{propertyId}/exports` | Starts an export. Answers `202`. |
| `GET`, `DELETE /v1/analytics/properties/{propertyId}/exports/{exportId}` | Reads an export and its progress, or deletes it and its file. |
| `GET /v1/analytics/properties/{propertyId}/exports/{exportId}/download` | Downloads the file. |

Make a link for the overview and the pages, with a password, for one site, ending on a date:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/shares \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{
    "name": "Board report",
    "views": ["overview", "pages"],
    "password": "a long password",
    "embed_origins": ["https://www.example.com"],
    "expires_at": "2027-01-01T00:00:00Z"
  }'
```

Revoke it:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/shares/$SHARE/revoke \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

A viewer's script unlocks a link and queries it:

```sh
TOKEN=$(curl -s -X POST https://support.example.com/client/v1/analytics-shares/yuva_sh_xxxxxxxx/access \
  -H "Content-Type: application/json" -d '{"password": "a long password"}' | jq -r .access_token)

curl -X POST https://support.example.com/client/v1/analytics-shares/yuva_sh_xxxxxxxx/query \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"metrics": ["visitors"], "dimensions": ["time:day"], "range": {"period": "7d"}}'
```

Export a month of daily numbers by country, wait for it and download it:

```sh
EXPORT=$(curl -s -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/exports \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "daily", "format": "csv", "range": {"period": "30d"}, "dimension": "country"}' | jq -r .id)

curl -s https://support.example.com/v1/analytics/properties/$PROPERTY/exports/$EXPORT \
  -H "Authorization: Bearer $YUVA_API_KEY" | jq '{status, progress, rows}'

curl -o daily.csv https://support.example.com/v1/analytics/properties/$PROPERTY/exports/$EXPORT/download \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

A raw export of one week, for a key with `analytics:manage`:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/exports \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "raw", "format": "ndjson", "range": {"from": "2026-10-01", "to": "2026-10-07"}}'
```

The download answers `409 export_not_ready` until `status` is `done`, and `404` once the file has expired. A link
you click cannot send headers, so the download also takes `?workspace_id=<id>`. All fields are in the
[API contract](../openapi/openapi.yaml).

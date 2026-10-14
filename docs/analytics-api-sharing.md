# Share, export and keep API monitoring

A share link shows part of an [API property](analytics-api.md) to people without a Yuva account, an export writes its
numbers to a file, and retention decides how long each kind of number is kept. An erasure request about one consumer
is answered with a single call.

## Share links

API properties use the same links as websites: [make, change, revoke and delete them](analytics-sharing.md#share-links)
with the same calls, and the same token, password, expiry, embedding and limit of 50 links. What differs is the views.
A link to an API property lists `api_` views, a link to a website lists website views, and mixing them answers
`422 property_kind`.

| View | What the viewer sees |
|---|---|
| `api_overview` | Totals and charts of every metric, broken down by `status`, `status_class`, `environment`, `service`, `api_version`, `source`, `platform`, `country` and `app_version`. |
| `api_routes` | Adds `route`, `method` and `routes` (new and silent routes). |
| `api_consumers` | Adds `consumer`, as a breakdown or a filter. |

Totals and time grains come with any view. A query for a dimension or filter that none of the link's views owns is
refused with `403 view_not_shared`. Consumer names are therefore shown **only** when the link lists
`api_consumers`; a link without it never shows who called your API.

A link to an API property never shares `uptime`, `support` or `conversations`: a query for the `uptime` or `support`
dataset, or for the `conversations` metric, answers `403 view_not_shared`. Saved segments are refused, as on websites,
and a share has no export.

Make a link for the overview and the routes, without consumers:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/shares \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{
    "name": "Status for partners",
    "views": ["api_overview", "api_routes"],
    "expires_at": "2027-01-01T00:00:00Z"
  }'
```

A viewer's script reads it with `POST /client/v1/analytics-shares/{token}/query`; the body is the same as
`POST /v1/analytics/properties/{propertyId}/query`, with `"dataset": "api"`:

```sh
curl -X POST https://support.example.com/client/v1/analytics-shares/yuva_sh_xxxxxxxx/query \
  -H "Content-Type: application/json" \
  -d '{"dataset": "api", "metrics": ["requests", "error_rate", "p95"], "dimensions": ["route"], "range": {"period": "24h"}}'
```

Revoking a link stops it at once, as for a website. The share page tells the viewer's browser which kind of property it
shows, so it draws the API charts.

## Exports

An export runs in the background and keeps its file for **7 days**, as described in [Exports](analytics-sharing.md#exports).
For an API property:

| Kind | What it holds | Datasets |
|---|---|---|
| `view` | Every row of one `query`, the table you would see in the panel, without comparison. | `api`, `uptime` |
| `daily` | One row per day of `range`, at most 400 days, optionally split by one `dimension`. | `api` (the default), `uptime` |

Both come as CSV (the default) or NDJSON. There is **no `raw` export** for an API property, because it keeps no row per
request: a `raw` export answers `422 property_kind`, and so does the `traffic` dataset. The `support` dataset is
not exported.

The range is fixed when the export is made. A `view` export with `"period": "24h"` holds the 24 hours that ended
when you asked for it, however long the file waits; asking again later gives a newer range. The same query that the
panel runs gives the same rows. A query that is not valid is refused at once, not after the job started.

A `daily` export has the day first, then the dimension if you gave one, then these columns:

| Dataset | Columns, in order | Days |
|---|---|---|
| `api` | `requests`, `errors`, `error_rate`, `client_errors`, `client_error_rate`, `p50`, `p95`, `p99`, `avg_duration`, `apdex`, `request_bytes`, `response_bytes` | Local days of the property |
| `uptime` | `runs`, `failures`, `uptime`, `downtime`, `response_time_p50`, `response_time_p95`, `response_time_p99`, `avg_response_time` | UTC days |

The `dimension` of an `api` export is any API dimension, of an `uptime` export `check`. Rates are percent, durations
milliseconds.

Export a month of requests by route and download it:

```sh
EXPORT=$(curl -s -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/exports \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "daily", "dataset": "api", "format": "csv", "range": {"period": "30d"}, "dimension": "route"}' | jq -r .id)

curl -o routes.csv https://support.example.com/v1/analytics/properties/$PROPERTY/exports/$EXPORT/download \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

Wait until `GET …/exports/{exportId}` shows `status: done` before downloading. The calls, scopes and limits are the
ones of [the website exports](analytics-sharing.md#api).

## How long numbers are kept

API monitoring keeps its numbers in tiers, each as long as it needs to be:

| Numbers | Kept | Set by |
|---|---|---|
| Minutes (`time:minute`) | 8 days by default | The operator, server-wide: [`YUVA_MONITOR_MINUTE_DAYS`](configuration.md#api-monitoring), 2 to 31 |
| Hours | 90 days by default | `monitor_hour_days` of the property, 7 to 400 |
| Days, and the breakdowns by dimension | 25 months by default | `retention_months` of the property, 1 to 120, or `null` for ever |
| Runs of an uptime check | 30 days by default | The operator: [`YUVA_MONITOR_CHECK_DAYS`](configuration.md#api-monitoring), 2 to 90 |
| Daily numbers of an uptime check, state changes and service history | 400 days | Fixed |

Change the two per-property values with `PATCH /v1/analytics/properties/{propertyId}`:

```sh
curl -X PATCH https://support.example.com/v1/analytics/properties/$PROPERTY \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{"monitor_hour_days": 30, "retention_months": 13}'
```

A background job removes what is past its tier. A shorter value removes the older numbers for good; a longer one
does not bring back what was already gone. The workspace-wide `retention_days` of
websites does not apply to API properties.

Deleting an API property deletes its numbers in every tier. Its uptime checks are not deleted: they keep running
without a property, and their own results stay under their own retention.

## Erase a consumer

A consumer is the name your records give to who called your API. If someone asks you to erase theirs, find its id
and delete it. Both calls are for API properties; a website answers `422 property_kind`.

```sh
curl "https://support.example.com/v1/analytics/properties/$PROPERTY/consumers?q=acme" \
  -H "Authorization: Bearer $YUVA_API_KEY"

curl -X DELETE https://support.example.com/v1/analytics/properties/$PROPERTY/consumers/$CONSUMER \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

`GET …/consumers` lists the consumers the property counted, with their `id`, `name` and `first_seen_at`; `q` keeps
the names that contain it, and a cursor pages the list. It needs `analytics:read`. `(other)`, where requests past the
cap of 10,000 consumers are counted, is not listed and cannot be erased (`400 validation_failed`). `DELETE` needs
`analytics:manage`, and an owner, an admin or an API key without an inbox limit. It answers `204`.

What the erasure does:

- It deletes the consumer's hourly and daily rows in every tier, and its name. The panel and the API no longer show it
  in a `consumer` breakdown or accept it as a filter.
- Your totals do not change. Minute, hour and day totals never held the consumer, so its requests stay counted, just
  not attributed.
- It is repeated twice, 2 and 11 minutes later, to delete what a server process was still adding up in memory when
  you asked.
- The id is never given to another consumer. If your records name the same consumer again, it counts under a new id.
- Export files made before keep the name until they expire, 7 days after they were made. Delete an export earlier with
  `DELETE /v1/analytics/properties/{propertyId}/exports/{exportId}`.

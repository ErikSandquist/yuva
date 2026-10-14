# Analyze API monitoring

This page explains what each number of an API property means, how the percentiles are computed, and which
breakdowns and comparisons you can ask for. It builds on [API monitoring](analytics-api.md). The panel shows these
views on the property's **Overview** and on each route's page; the same numbers come from the analytics query.

## What the numbers mean

Every number is counted over the range you pick. A request that was sampled (`sample_rate` above 1) counts as that
many requests in every number, so an answer built on sampled records is an estimate and carries `meta.estimated:
true`.

| Metric | Meaning |
|---|---|
| `requests` | Requests answered. |
| `requests_per_minute` | `requests` divided by the minutes of the range or bucket, up to now: the throughput. |
| `errors`, `error_rate` | Requests with status 500 to 599, or `0` when no response was sent, and their share of all requests in percent. |
| `client_errors`, `client_error_rate` | The same for status 400 to 499. |
| `p50`, `p75`, `p90`, `p95`, `p99` | The duration in milliseconds that this share of requests stayed within. `p95` of 240 means 95 of 100 requests took 240 ms or less. |
| `avg_duration`, `max_duration` | The mean and the longest duration in milliseconds. Both are exact. |
| `apdex` | A score from 0 to 1 for how many requests were fast enough; see below. |
| `request_bytes`, `response_bytes` | Body bytes added up, of the records that gave a size. |
| `response_size_p50`, `response_size_p95` | The size in bytes that this share of responses stayed within. |
| `conversations` | Conversations the property's linked inbox received, by day. |
| `latency_bands` | Requests per duration band: the data behind the latency heatmap. |

### How percentiles are computed

Yuva does not keep a row per request, so it cannot sort durations. Instead each minute, hour and day keeps a small
histogram whose buckets grow by a factor of 2^(1/32), about 2.2%. A request is counted in the bucket its duration
falls in, histograms of different minutes are added bucket by bucket, and a percentile is read as the bucket that
holds that rank of the requests. The value reported for a bucket is its middle, so a percentile is within 1.1% of
the exact value, whatever the range, the breakdown or the number of requests. A second histogram with buckets of
2^(1/4) keeps response sizes, so `response_size_p50` and `response_size_p95` are within 9%.

Percentiles of a group cannot be worked out from the percentiles of its parts, but histograms add up exactly, so a
route's `p95` over a month is the true `p95` of that month, not an average of daily numbers.

### Apdex

Apdex puts the durations in three groups, against a threshold T in milliseconds: **satisfied** requests took at
most T, **tolerating** ones more than T and at most 4T, and the rest were **frustrated**. The score is
(satisfied + tolerating / 2) / requests, so 1 means every request was fast enough and 0 that none was.

T is the property's `apdex_ms` (500 ms by default, 1 to 60,000). A route that is slow by nature, such as an export,
has its own `apdex_ms`; `null` goes back to the property's. Set it on the route's page in the panel or with
`PATCH /v1/analytics/properties/{propertyId}/routes/{routeId}` (scope `analytics:manage`). Apdex is read from the
duration histograms, so a request on the edge of a bucket may count one group over or under, as with the
percentiles.

### Latency bands

`latency_bands` gives 20 counts that add up to `requests`. Band 0 holds requests of at most 256 microseconds, each
next band those up to the next power of two (512 µs, 1.02 ms, and so on up to about 67 seconds), and the last band
everything slower. The upper bounds come in `meta.latency_band_bounds`, the same in every answer. Because the
histogram buckets end on these bounds, the counts are exact, not estimated. The panel draws them as the **Latency
heatmap** on a route's page: time along one axis, the bands along the other.

## Break down and filter

Break down by `route`, `method`, `status`, `status_class`, `api_version`, `environment`, `service`, `source`
(`server` for your server, `otlp` for [OpenTelemetry](analytics-api-otel.md), `app` for your [apps](analytics-api-apps.md))
and `platform` (`ios` or `android` for app records), in any combination, over any range, with at most two
dimensions of which one may be a time grain. Filters use the same dimensions. These are the **Breakdown** and the
filters in the panel.

Three more dimensions, `consumer`, `country` and `app_version`, can be added once each, with limits:

- **One extra dimension at a time.** Two of them in one query, or one as dimension and another as filter, are refused.
- **Only with `route`, `method` and `status_class`.** An extra dimension cannot be combined with `status`,
  `api_version`, `environment`, `service`, `source`, `platform` or a second extra one, as a dimension or a filter.
- **Only over whole local hours.** Use `time:hour` or longer grains, ranges in days or weeks, or `since` and `until`
  on the hour. `15m`, `1h`, `6h`, `24h` and `time:minute` are refused, since minutes are not kept per extra
  dimension.
- **Requests without a value are left out.** A breakdown by `consumer` omits requests that named no consumer (or
  country, or app version), so its rows can add up to less than the overall total.

A combination that is not kept answers `422 dimension_combination`, and its `detail` says what to change. The panel
shows the same message instead of an empty chart.

A property counts at most 10,000 consumers and 200 countries or app versions; further values are counted as `(other)`
and `meta.caps_reached` says so.

## Consumers

The **Consumers** page shows requests, errors and latency per consumer, and **Top consumers** on a route's page the
same for that route. The consumer is what your server sends in `consumer`: an API key name, a client, an app. A consumer is a
dimension like any other extra one, so the limits above apply: whole hours, and `route`, `method` or `status_class`
as the only other dimension.

## Compare periods

`compare` adds the same numbers for another range to the answer: `previous` (as long a range just before), `year`
(the same days a year earlier) or `range` (the days `from` to `to`). For ranges of minutes (`15m` to `24h`, or
`since` and `until`), `previous` is as many minutes just before and `year` the same instants a year earlier. The
panel's compare control draws the compared range as a second line. `insights` stay unavailable for API queries.

## New and silent routes

`routes: new` lists the routes your server received for the first time inside the range, `routes: silent` those
called in the previous period of the same length and not since the range began. It needs the dimension `route`
alone. Every listed route has a row, with zeros when it had no request, and `compare` with `previous` shows a
silent route's earlier numbers. `(unmatched)`, `(other)` and hidden routes are never listed. In the panel they are
the **New** and **Silent** tabs of the route list.

## Deploy markers

When a record brings a `release` that is new for its service and environment, Yuva adds a `deploy` annotation at that
record's time with the service, environment and release. The first release a service ever sends counts too; going
back to a release seen before adds none; at most 50 are added per property and day. The panel draws them as lines
on the time charts and lists them. They are read-only like Yuva's other annotations. To get markers, send `release`
with your records (the [OpenTelemetry](analytics-api-otel.md) `service.version` becomes it).

## Ranges and freshness

Besides the periods of [analytics](analytics.md), an API query takes `15m`, `1h`, `6h` and `24h` up to the current
minute, and `since` and `until` as instants on a whole minute (`until` not included, at most 400 days apart). Minute
and hour labels in an answer are the property's local time with its UTC offset, such as `2026-10-12T14:05+03:00`,
so a repeated hour at a clock change stays apart. Closed days and hours are read from daily and hourly numbers, the
rest from minutes; a new record shows within about 15 seconds. The minutes are kept for
[`YUVA_MONITOR_MINUTE_DAYS`](configuration.md#api-monitoring) days.

## API

All queries are `POST /v1/analytics/properties/{propertyId}/query` with a key that has `analytics:read`. Pass the
JSON below as the body:

```sh
curl https://support.example.com/v1/analytics/properties/PROPERTY_ID/query \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '<BODY>'
```

Percentiles, Apdex and sizes, slowest routes first:

```json
{
  "dataset": "api",
  "metrics": ["requests", "p75", "p90", "p95", "p99", "max_duration", "apdex", "response_size_p95"],
  "dimensions": ["route"],
  "range": { "period": "7d" },
  "order_by": [{ "metric": "p95", "dir": "desc" }],
  "limit": 10
}
```

Filter and break down (5xx on one service, by route and API version):

```json
{
  "dataset": "api",
  "metrics": ["requests", "error_rate"],
  "dimensions": ["route", "api_version"],
  "filters": [
    { "dimension": "service", "op": "is", "values": ["checkout"] },
    { "dimension": "status_class", "op": "is", "values": ["5xx"] }
  ],
  "range": { "period": "24h" }
}
```

Consumers of one route (an extra dimension with `route`, over whole days):

```json
{
  "dataset": "api",
  "metrics": ["requests", "errors", "p95"],
  "dimensions": ["consumer"],
  "filters": [{ "dimension": "route", "op": "is", "values": ["/users/{id}"] }],
  "range": { "period": "30d" }
}
```

The same totals against the previous period:

```json
{
  "dataset": "api",
  "metrics": ["requests", "error_rate", "p95"],
  "range": { "period": "7d" },
  "compare": { "with": "previous" }
}
```

New routes of the last week (use `"silent"` for routes that stopped):

```json
{
  "dataset": "api",
  "metrics": ["requests"],
  "dimensions": ["route"],
  "routes": "new",
  "range": { "period": "7d" }
}
```

The latency heatmap, one set of bands per hour of today (bounds in `meta.latency_band_bounds`):

```json
{
  "dataset": "api",
  "metrics": ["latency_bands"],
  "dimensions": ["time:hour"],
  "range": { "period": "today" }
}
```

A window of minutes with `since` and `until`:

```json
{
  "dataset": "api",
  "metrics": ["requests_per_minute", "p95"],
  "dimensions": ["time:minute"],
  "range": { "since": "2026-10-12T14:00:00+03:00", "until": "2026-10-12T15:00:00+03:00" }
}
```

Conversations the linked inbox received, per day (no filter, no other dimension; `422 no_linked_inbox` when the
property has no linked inbox):

```json
{
  "dataset": "api",
  "metrics": ["conversations"],
  "dimensions": ["time:day"],
  "range": { "period": "30d" }
}
```

A route's own Apdex threshold:

```sh
curl -X PATCH https://support.example.com/v1/analytics/properties/PROPERTY_ID/routes/ROUTE_ID \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "apdex_ms": 5000 }'
```

Deploy markers are annotations of the property, with the kind `deploy`:

```sh
curl "https://support.example.com/v1/analytics/properties/PROPERTY_ID/annotations?from=2026-10-01&to=2026-10-14" \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

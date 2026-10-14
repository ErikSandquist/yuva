# API monitoring

An API property counts the requests your own API serves: how many, how many failed, and how long they took, by
route. Your server reports each request it answered, and Yuva adds them up per minute and hour, without
keeping a row per request, a header, a body or an IP address. This builds on [Analytics](analytics.md).

## Make an API property

An API property is an analytics property with `kind: api`. It has no domains and no tracking script. Owners and
admins create it with a key that has the scope `analytics:manage`:

```sh
curl https://support.example.com/v1/analytics/properties \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "kind": "api", "name": "Shop API", "domains": [], "timezone": "Europe/Berlin" }'
```

The answer carries the property's `id`. The property's time zone decides which hours and days its numbers are
counted in, and the minutes of a `time:minute` query are in it too.

Then make the key that sends the records, in **Settings → API keys**, with the scope `analytics:write`, and keep it
on the server. A member's session is refused (`403 forbidden`), and a site property answers
`422 property_kind`. A server started with `YUVA_MONITOR=off` has no API properties.

## Send requests from Go

```sh
go get github.com/productdevbook/yuva/sdk/go/analytics
```

`Middleware` wraps a `net/http` handler and reports every request it answers. Wrap the `http.ServeMux` itself, so
the pattern the mux matched is known: `GET /users/{id}` is recorded as the route `/users/{id}`.

```go
import "github.com/productdevbook/yuva/sdk/go/analytics"

mux := http.NewServeMux()
mux.HandleFunc("GET /users/{id}", getUser)

monitor, err := analytics.New(analytics.Config{
	BaseURL:    "https://support.example.com",
	APIKey:     os.Getenv("YUVA_API_KEY"),
	PropertyID: os.Getenv("YUVA_API_PROPERTY"),
})
if err != nil {
	log.Fatal(err)
}
defer monitor.Close(context.Background())

handler := monitor.Middleware(analytics.Monitor{
	Service:  "shop-api",
	Consumer: func(r *http.Request) string { return apiKeyName(r) },
})(mux)
log.Fatal(http.ListenAndServe(":8080", handler))
```

Records go out in gzip batches of up to 500 every 5 seconds, with the retries of [server-side events](analytics-server.md#go).
Call `Close` before the process exits, or the records still waiting are lost.

| `Monitor` | Default | |
|---|---|---|
| `Route` | the mux's pattern | Names the route of a request when your router is not a `ServeMux`. |
| `Consumer` | none | Who called: an API key's name or a client id. Never a person's address; a value shaped like an e-mail address is stored as `(scrubbed)`. |
| `APIVersion` | none | The API version the request used. |
| `Environment` | `production` | Tells apart `production`, `staging` and the like. |
| `Service` | none | The service that served the request. |
| `Release` | none | The service's release. It is accepted but not shown yet. |
| `SampleRate` | every request | `10` sends one in ten successful requests (status 1 to 399), each standing for ten. Errors and client errors are always sent. |
| `Country` | off | Sends the caller's address (`Config.ClientIP`) so Yuva can count the request's country. It is shortened on arrival and never stored. |

A request no pattern matched is sent with its path. A handler that panics is recorded with the status it wrote,
or `0` when it wrote none.

## Send requests with curl

Any other stack posts the same records. `POST /v1/analytics/properties/{propertyId}/requests` takes 1 to 1,000
records and answers `202`. The body may be gzip-compressed with `Content-Encoding: gzip`.

```sh
curl https://support.example.com/v1/analytics/properties/PROPERTY_ID/requests \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "records": [{
      "method": "GET",
      "route": "/users/{id}",
      "status": 200,
      "duration_us": 18400,
      "response_size": 612,
      "consumer": "mobile-app",
      "environment": "production"
    }]
  }'
```

```json
{ "accepted": 1, "errors": [] }
```

| Field | |
|---|---|
| `method` | Required, 1 to 20 characters. The RFC 9110 methods and `PATCH` are kept; anything else counts as `_OTHER`. |
| `route` | The pattern the router matched, starting with `/`, up to 300 characters, without method or host. Give this or `path`. |
| `path` | The request path, for a router that knows no pattern. Only the pattern worked out from it is kept. |
| `status` | Required, 100 to 599, or `0` when no response was sent. |
| `duration_us` | Required, the duration in microseconds, up to 24 hours. |
| `request_size`, `response_size` | Bytes of the bodies. |
| `time` | When the request started, RFC 3339. Now when left out. |
| `consumer` | Up to 100 characters; see above. |
| `api_version`, `environment`, `service`, `release` | Up to 40, 40, 100 and 80 characters. `environment` is `production` when left out. |
| `ip` | The caller's address, only to find its country. |
| `sample_rate` | 1 to 10,000: this record stands for that many requests. Only a status of 1 to 399 may be sampled; a record with `sample_rate` above 1 and an error status is refused. |

## Routes

A route names a part of your API, never one record, so an id never becomes a route of its own.

- **Routes you send are cleaned.** In a `route` or a `path`, every segment that looks like an id becomes `{id}`:
  numbers, UUIDs, dates, hex strings of 16 characters or more, other tokens of 16 or more letters, digits, `-` and
  `_` with at least two digits (or both cases and a digit), e-mail-shaped values, and anything over 40
  characters. Segments that start like a router's placeholder (`{id}`, `:id`, `*`, `[id]`, `<id>`) stay as they are.
  So `/users/42` and `/users/9f8e7d6c5b4a3210` both count as `/users/{id}`.
- **A 404 or 405 without a pattern is `(unmatched)`.** A record that gives only a `path` and was answered with
  `404` or `405` is counted under the route `(unmatched)`, so scanners probing `/wp-login.php` never make
  routes.
- **Route rules.** The property's `path_rules` map a route or path onto a route of your own, before the cleaning
  above. A rule is `{"pattern": "/legacy/**", "replacement": "/legacy"}`: `*` matches one segment and `**` any
  number of them, the first matching rule wins, and the replacement is used as it is. Set them with
  `PATCH /v1/analytics/properties/{propertyId}`.
- **Caps.** A property counts at most 1,000 routes, 10,000 consumers, 50 API versions and 20 environments, and a
  workspace 100 services. Past a cap, new values are counted as `(other)`, and the query answer lists the cap
  in `meta.caps_reached`.

### List and hide routes

`GET /v1/analytics/properties/{propertyId}/routes` (scope `analytics:read`) lists every route the property
counted, by pattern, each with its `id`, whether it was `inferred` from paths, whether it is `hidden`, and when it
was first and last seen. `(unmatched)` is in the list, and so is `(other)` once a route has been counted under it.

A route that only adds noise, such as the health check your load balancer calls, can be hidden:

```sh
curl -X PATCH https://support.example.com/v1/analytics/properties/PROPERTY_ID/routes/ROUTE_ID \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "hidden": true }'
```

Hiding takes the route out of every `dataset: api` answer from then on, rows and totals alike, and a filter on it
matches nothing. Its requests are still counted and nothing is deleted: `"hidden": false` brings it back with all
its numbers. Hiding needs the scope `analytics:manage` and is for owners and admins. To count a path under another
route instead, use a route rule.

## What is recorded

Recorded, per minute, hour and day: the number of requests, those with status 500 to 599 or `0` (errors) and
with status 400 to 499 (client errors), and the distribution of durations that gives p50, p95 and p99, by route,
method, status and environment. Service, API version and consumer are kept with them.

Never recorded, and not accepted by the API: request or response bodies, headers, query strings, fragments
and user agents. Postgres holds no row per request. An `ip` is shortened (IPv4 `/24`, IPv6 `/48`), looked up for
the country and dropped in the same call; no address is stored. The Go middleware sends an address only with
`Country: true`.

## Read the numbers

In the panel, the property's **Overview** shows requests, error rates and latency over time, and **Routes** the
routes with their traffic, errors and latency.

The same numbers come from the analytics query, `POST /v1/analytics/properties/{propertyId}/query`, with
`"dataset": "api"` (scope `analytics:read`):

```sh
curl https://support.example.com/v1/analytics/properties/PROPERTY_ID/query \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "dataset": "api",
    "metrics": ["requests", "error_rate", "p95"],
    "dimensions": ["route"],
    "range": { "period": "24h" },
    "limit": 20
  }'
```

| | |
|---|---|
| Metrics | `requests`, `requests_per_minute`, `errors`, `error_rate`, `client_errors`, `client_error_rate`, `p50`, `p95`, `p99`, `avg_duration`. Rates are percent, durations milliseconds. The percentiles are accurate to 1.1%. |
| Dimensions and filters | `route`, `method`, `status`, `status_class` (`2xx`, `3xx`, `4xx`, `5xx`, or `none` for status `0`), `environment`; at most two dimensions, one of them may be a time grain. |
| Time grains | `time:minute`, `time:hour`, `time:day`. Minutes and hours are in the property's local time. |
| Range | `15m`, `1h`, `6h`, `24h` up to the current minute, or `today`, `7d`, `30d` and the other periods, or `from` and `to`. |

`time:minute` reaches back [`YUVA_MONITOR_MINUTE_DAYS`](configuration.md#api-monitoring) days (8 by default) and
returns at most 1,440 minutes; a longer or older range is refused with `422 range_too_large`. Hourly numbers are
kept for the property's `monitor_hour_days` (90 by default, 7 to 400), daily numbers as long as the property's
`retention_months`. An answer that rests on records sent with `sample_rate` above 1 has `meta.estimated: true`.
`dataset: api` on a site property, and a query without it on an API property, answer `422 property_kind`. An API
query takes no `goal_id`, `prop`, `segment_id`, `compare` or
`insights`.

A record shows up in a query within about 15 seconds.

## What comes back

The answer is `202` when the request was valid, whatever happened to the records.

- **`errors`** lists the records left out as invalid, each with its `index` in `records` and a `detail`. The others are taken.
- **Taken but not counted:** records older than 72 hours; records over `YUVA_MONITOR_RECORDS_PER_MINUTE` for the
  property or for the key; records over the workspace's monthly limit.
- **The whole request is refused** for a body that breaks the shape above (`400`), a missing or wrong key (`401`),
  a key without `analytics:write` or a member's session (`403`), an unknown property (`404`), a site property
  (`422`) and an encoding other than gzip (`415`). A server that holds as many open rows as it may answers `503`
  with `Retry-After`: send the batch again after that.

The monthly limit counts the records received, not the requests they stand for: a sampled record is one record
whatever its `sample_rate`. A batch that crosses the limit is taken up to it: the records that still fit are
counted, and the rest are dropped as over the limit. When a month's records reach the limit, the workspace's owners
get one e-mail, once a month. `GET /v1/analytics/properties` shows the month's `requests` and the `requests_limit`
under `usage`, and `requests_limit_reached_at` once the limit is reached.

## Change the time zone

Changing an API property's `timezone` (`PATCH /v1/analytics/properties/{propertyId}`) does not cut a day in two. The
property keeps counting in the old zone until that zone's next local midnight, and from then on in the new one.
Until then the property carries `timezone_applies_at`, the instant the new zone starts. Days already counted keep
the zone they were counted in.

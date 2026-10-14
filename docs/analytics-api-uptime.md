# Uptime checks

An uptime check calls a URL of yours from the Yuva server on an interval and records whether it answered as you
expected, how long each step took and when its certificate expires. Use it for what request records cannot tell you:
an API that is down gets no requests, so nothing reports that it is. This builds on
[API monitoring](analytics-api.md); [alerts](analytics-api-alerts.md) tell you when a check goes down.

## Add a check

Owners and admins, and API keys without an inbox limit, create checks with the scope `analytics:manage`:

```sh
curl https://support.example.com/v1/monitoring/checks \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Shop API health",
    "url": "https://api.example.com/healthz",
    "property_id": "PROPERTY_ID"
  }'
```

The check runs right away and then on its interval. A member who creates a check gets an alert rule for it, sent to
them: see [default rules](analytics-api-alerts.md#default-rules).

| Field | Default | Meaning |
|---|---|---|
| `name` | required | 1 to 100 characters. |
| `url` | required | An `http` or `https` URL with a host, no credentials and no fragment, up to 2,000 characters. |
| `method` | `GET` | `GET` or `HEAD`. |
| `headers` | none | Up to 5 request headers, each `{ "name": "Authorization", "value": "Bearer …" }`. See below. |
| `expected_statuses` | any 200 to 399 | Up to 20 status codes from 200 to 399 that count as up. |
| `keyword` | none | Text the first 64 KiB of the answer must contain; `GET` only. |
| `keyword_absent` | `false` | Turns `keyword` around: the answer must not contain it. |
| `timeout_seconds` | `10` | 1 to 30. |
| `interval_seconds` | `60`, or the server's minimum when longer | 30 to 86,400, and at least `YUVA_MONITOR_MIN_INTERVAL` (`422 interval_too_short`). |
| `failures_before_down` | `2` | 1 to 10 failed runs in a row before the check is down. |
| `cert_warning_days` | `14` | 1 to 90. An `https` check whose certificate expires within this many days raises a `certificate` alert. |
| `max_redirects` | `5` | 0 to 5. With `0`, the redirect's own status is judged. |
| `service` | none | The service the check watches, by the name your records and spans give. |
| `property_id` | none | The API property whose `uptime` numbers and reports include the check. |
| `paused` | `false` | A paused check does not run. |

`PATCH /v1/monitoring/checks/{checkId}` changes any of these; the check is validated whole afterwards. A changed URL,
method or interval runs the check next right away, `paused: true` stops it and `paused: false` runs it now.
`DELETE` removes the check with its results, daily rows, alert rules and alerts.

### Headers

Headers let a check pass an `Authorization` header or a token your health endpoint wants. The values are sealed under
the server's master key and are write-only: the API returns only `header_names`, never a value. `headers` replaces
all headers on an update, values included, so send the full list. `Host`, `Cookie`, `Content-Length`,
`Transfer-Encoding`, `Connection`, `Upgrade`, `TE`, `Trailer` and `Proxy-*` cannot be set.

## What the server will and will not call

The server calls only public addresses. A URL is refused with `422 refused_address` when it names credentials,
`localhost`, or an address in one of these ranges:

- loopback,
- private,
- link-local,
- carrier-grade NAT (CGNAT),
- multicast,
- reserved,
- cloud metadata.

A host name is resolved when the check is saved and again on every run, and refused when any address it resolves to
is one of these. Every redirect hop is checked the same way, and the connection is made to the address that was
checked, so a name that changes its answer between lookups cannot lead the server to a private one. A check refused
on a later run fails with the error `refused_address`.

A self-hosted server that needs to watch internal services sets `YUVA_MONITOR_ALLOW_PRIVATE=true`
([Configuration](configuration.md#api-monitoring)). That lets checks reach private and loopback addresses. Link-local
and cloud metadata addresses stay refused even then.

The server sends `User-Agent: Yuva-Uptime/<version>`, no cookies and no body, and reads at most 64 KiB of the answer.
Each connection attempt gives up after 10 seconds, whatever the check's `timeout_seconds`.

## Limits

| Limit | Value |
|---|---|
| Checks per workspace | `YUVA_MONITOR_CHECKS`: 500, or 50 on a server with open sign-up (`403 check_limit`). |
| Checks on one host | 10 (`403 host_limit`). |
| Shortest interval | `YUVA_MONITOR_MIN_INTERVAL`: 30 seconds, or 60 on a server with open sign-up. |
| Runs kept | `YUVA_MONITOR_CHECK_DAYS` days, 30 by default; older days stay as daily numbers for 400 days. |

## When a check is down

A run succeeds when the answer arrives in time, its status is one of `expected_statuses` and the keyword condition
holds. A run that fails is tried again after 20 seconds, and only that second try counts. The check is `down` once
`failures_before_down` counted runs in a row have failed, and `up` again after one that succeeded. Before its first
counted run it is `unknown`.

A failed run carries one of these errors: `dns`, `refused_address`, `connect`, `tls`, `timeout`, `status`,
`keyword`, `redirects` or `other`.

**Unknown minutes.** When at least 20 checks ran in one minute on the server and more than half of them failed, the
fault is likelier Yuva's own network than your endpoints. That minute's runs are recorded `unknown`: they count
neither as up nor as down, and appear in no uptime figure.

**Delay.** A minute is judged about 40 seconds after it closes, so a stopped endpoint shows as down after 2 to 3
minutes with the default two failures. A run not yet judged has the state `pending` in the results.

## Read the results

`GET /v1/monitoring/checks` lists the checks with their state, `state_since`, `next_run_at`, `cert_expires_at` and
last result. `GET /v1/monitoring/checks/{checkId}/results` lists the counted runs, newest first, up to 500 a page
(`limit`, and `before` for the next page). Each run has:

| Field | Meaning |
|---|---|
| `at` | When the counted try started. |
| `ok`, `state` | Whether it succeeded, and the state after it: `up`, `down`, `unknown` or `pending`. |
| `status` | The last answer's status; `0` when none came. |
| `error`, `detail` | Why it failed. The detail is a fixed text; it never shows resolved addresses. |
| `retried` | The first try failed and this is the try 20 seconds later. |
| `redirects` | Hops followed. |
| `timings` | `dns`, `connect`, `tls`, `ttfb` and `total` in milliseconds. |
| `cert_days_left` | `https`: whole days until the certificate expires. |

### The uptime dataset

Numbers over time come from the query operation with `dataset: uptime`:
`POST /v1/monitoring/query` reads every check of the workspace, including those without a property, and
`POST /v1/analytics/properties/{propertyId}/query` reads the checks of that property. The dimension is `check`, with
a time grain; use `time:minute` or `time:hour` inside the runs kept, and `time:day`, `time:week` or `time:month` for
any range. Buckets and days are in UTC, whatever the property's time zone.

| Metric | Meaning |
|---|---|
| `runs`, `failures` | Counted runs, and those that failed. |
| `up` | Checks whose last counted run of the range or bucket succeeded. |
| `uptime` | Percent of the monitored time the check was not down, each run standing for its interval. |
| `downtime` | Seconds down. |
| `response_time_p50`, `response_time_p95`, `response_time_p99` | Total time of runs that got an answer, in milliseconds, accurate to 1.1%. |
| `avg_response_time`, `ttfb` | Mean total time and time to first byte, in milliseconds. |
| `cert_days_left` | The fewest days until a certificate expires. |

Unknown runs count in none of them. Add `?format=csv` to `POST /v1/monitoring/query` for a CSV file.

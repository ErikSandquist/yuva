# Alerts and reports for your API

Alert rules watch an [API property](analytics-api.md) and its [uptime checks](analytics-api-uptime.md) and tell you
when something goes wrong, and when it is over. Each problem raises one alert when it starts and one message when it
ends, not a message every minute. Weekly and monthly reports summarize the period.

## How an alert works

The server applies every rule once a minute, over the minutes that have closed. For each rule and subject it keeps at
most one open alert:

1. When the condition holds, the alert **fires** once.
2. While it stays open nothing more is sent.
3. When the value has been back inside the threshold, with a margin, for a whole window, the alert **resolves**
   once.

The margin is 10%: a rule for an error rate above 5% resolves when the rate has stayed below 4.5% for the whole
window. For Apdex, which should stay high, the margin is a tenth of the distance to 1: a rule for Apdex below 0.8
resolves above 0.82. Spike and drop have their own margins, below.

An alert also resolves, without waiting for a window, when:

- the window holds fewer requests than the rule's `min_requests`, since there is nothing left to judge,
- a silent route is called again,
- the check is up again, or its certificate is renewed,
- the check is paused or the rule is disabled (a disabled rule resolves its open alerts without telling anyone).

### Rule kinds

Rules on a property's requests need `property_id`. `route` narrows a rule to one route (a route pattern, as
`GET /v1/analytics/properties/{propertyId}/routes` lists it) and `service` to one service; with neither, the rule
watches the whole property. They are judged over the last `window_minutes` closed minutes (5 to 60, 5 by default) and
only when the window has at least `min_requests` requests (20 by default).

| `kind` | Fires when | `threshold` |
|---|---|---|
| `error_rate` | The share of requests with status 500 to 599 (or no response) is above the threshold. | Percent. |
| `client_error_rate` | The share with status 400 to 499 is above the threshold. | Percent. |
| `p95`, `p99` | That percentile of the duration is above the threshold. | Milliseconds. |
| `apdex` | The [Apdex](analytics-api-analysis.md#apdex) score is below the threshold. | 0 to 1. |
| `spike` | Requests of the last 15 closed minutes are at least three times the same 15 minutes averaged over the 7 days before. | None. |
| `drop` | They are at most half of that average. | None. |
| `silent` | A route that averaged at least one request a minute over the day before has had none for 15 minutes. | None. |

`spike` and `drop` are judged only when the 7-day average is at least 50 requests, and have no window of their own.
A spike resolves when the 15 minutes are back below 2.7 times the average, a drop when they are back above 0.55
times. A `silent` rule with a `route` watches that route; without one it watches each route of the property.

Rules on uptime checks name `check_id`, or `property_id` for every check of that property, or neither for every check
of the workspace:

| `kind` | Fires when | `threshold` |
|---|---|---|
| `check_state` | The check is down. | None. |
| `certificate` | The check's certificate expires within the threshold. | Days; the check's `cert_warning_days` when absent. |

### Default rules

A new API property gets one rule: the 5xx rate above 5% over 5 minutes, sent to the member who created it. A member
who creates an uptime check gets a `check_state` rule for it, sent to them. Edit or delete them like any other rule.

## Make a rule

Owners and admins, and API keys without an inbox limit, manage rules with the scope `analytics:manage`:

```sh
curl https://support.example.com/v1/monitoring/alert-rules \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "kind": "p95",
    "property_id": "PROPERTY_ID",
    "route": "/checkout",
    "threshold": 800,
    "window_minutes": 15,
    "recipients": ["MEMBER_ID"]
  }'
```

| Field | Meaning |
|---|---|
| `kind` | One of the kinds above. Fixed after the rule exists, like its subject (`property_id`, `route`, `service`, `check_id`). |
| `name` | 1 to 100 characters; named after its kind and subject when absent. |
| `threshold`, `window_minutes`, `min_requests` | As above. `min_requests` is 1 to 1,000,000. |
| `recipients` | Up to 20 members who get each alert by e-mail and Web Push. They must be able to see the rule's property. |
| `enabled` | `false` pauses the rule. |

A workspace has at most 200 rules (`403 rule_limit`). `PATCH /v1/monitoring/alert-rules/{ruleId}` changes the rest
and applies from the next minute, to open alerts too. `DELETE` removes the rule and its alerts; their annotations
stay. Members read rules and alerts with `analytics:read`; rules on a property follow the property's visibility.

## Where an alert goes

Firing and resolving each do all of this:

- **E-mail** to the rule's `recipients`, in each member's language.
- **Web Push** to the same members, as an urgent notification.
- **Webhook** `monitor.alert` to the endpoints subscribed to it ([Webhooks](webhooks.md)). `data.alert.state` is
  `firing` or `resolved`. An endpoint limited to an inbox gets the alerts of properties linked to that inbox.
- **Annotation** on the property's charts, so the moment shows next to the numbers. An alert on a check that has no
  property annotates the check's property, if it has one.
- **Realtime** message `monitor.alert` to connected members and agents.

`analytics.alert` webhooks and the alerts of a website's report subscription are separate and unchanged. An API
property takes no `alerts` in its subscription; its alerts are these rules.

## Read alerts

```sh
curl "https://support.example.com/v1/monitoring/alerts?state=firing" \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

`GET /v1/monitoring/alerts` lists alerts newest first, up to `limit` (50 by default, at most 100), filtered by
`state` (`firing` or `resolved`), `property_id`, `check_id` or `rule_id`, with `before` for the next page. Resolved
alerts are kept 400 days. An alert has:

| Field | Meaning |
|---|---|
| `rule_id`, `rule_name`, `kind` | The rule that raised it. |
| `key` | What it is about within the rule: empty for the rule's whole subject, the route pattern for `silent`, the check id for check rules. |
| `state` | `firing` or `resolved`. |
| `value`, `threshold` | The value when it fired, and the rule's threshold. For `spike` and `drop` the value is the requests of the 15 minutes, for `certificate` the days left, for `check_state` 1. |
| `baseline` | `spike` and `drop`: the average it was compared with. `silent`: requests a minute over the day before. |
| `resolved_value`, `resolved_at` | When it resolved. |
| `fired_at` | The end of the window that fired it. |
| `text` | What was found, in English, as on the annotation. |

`GET /v1/monitoring/alert-rules` lists the rules, each with `firing`, the number of its open alerts.

## Weekly and monthly reports

An API property sends its own report to the members who ask for one, with the same settings as a website's
([E-mail reports](analytics-support.md#e-mail-reports-and-alert-mails)):

```sh
curl -X PUT https://support.example.com/v1/analytics/properties/PROPERTY_ID/subscription \
  -H "Content-Type: application/json" \
  -d '{ "report": "weekly" }'
```

This call needs a member's session, not an API key. Weekly reports go out on Monday at 08:00 in the property's time
zone and cover the week before; monthly ones go out on the 1st and cover the month before. The report has the
requests, the error and client error rates and p95 against the period before, the slowest and most failing routes,
the uptime of the property's checks and the alerts of the period. It has no images and nothing that tracks opening or
clicks. `POST …/subscription/send-report` sends one now.

## From an assistant

The `monitoring_alerts` and `uptime_checks` tools answer from the same numbers; see
[API monitoring from an assistant](mcp.md#api-monitoring-from-an-assistant).

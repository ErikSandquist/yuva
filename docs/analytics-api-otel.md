# API monitoring with OpenTelemetry

Any service that is instrumented with OpenTelemetry can report its requests to an API property: point its trace
exporter at Yuva and the server spans become request records, with no Yuva code in the service. This is the way
for every language without a Yuva package. You need an API property and a key with the scope `analytics:write`: see
[Make an API property](analytics-api.md#make-an-api-property).

## Set up the exporter

Two environment variables are enough:

```sh
OTEL_EXPORTER_OTLP_ENDPOINT=https://support.example.com/otlp/PROPERTY_ID
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer YUVA_API_KEY
```

The exporter adds `/v1/traces` itself, so traces go to `POST /otlp/{propertyId}/v1/traces`. If you set
`OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` instead, give it the whole URL, ending in `/v1/traces`. The key is an API key
with `analytics:write`; a member's session and OAuth tokens are refused (`403`).

- **Protocol.** Binary protobuf (`Content-Type: application/x-protobuf`, `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`)
  and JSON (`application/json`, `http/json`) both work. gRPC does not. Any other content type answers `415`.
- **gzip.** The body may be gzip-compressed with `Content-Encoding: gzip` (`OTEL_EXPORTER_OTLP_COMPRESSION=gzip`).
  Either way a request is at most 8 MiB.
- **Sampling.** Export every server span: set `OTEL_TRACES_SAMPLER=always_on`, or keep your sampler only for the other
  exporters. See [Sampling](#sampling).

Name the service and its environment with the standard resource attributes, for example
`OTEL_SERVICE_NAME=shop-api` and `OTEL_RESOURCE_ATTRIBUTES=deployment.environment.name=production,service.version=1.4.2`.

## What is read

Only spans of kind `SERVER` that carry an HTTP method count, one request each. Client spans, internal spans and
gRPC servers are skipped without being refused. Yuva reads a span's attributes by the current HTTP semantic
conventions and, where one is missing, by the names of version 1.20 and before, which many deployed SDKs and
auto-instrumentations still send.

| Request record | Attribute | Older name used when it is missing |
|---|---|---|
| Method | `http.request.method` | `http.method` |
| Route | `http.route` | |
| Path, to infer a route from | `url.path` | `http.target` (without its query), the path of `http.url` |
| Status | `http.response.status_code` | `http.status_code` |
| Request body size | `http.request.body.size` | `http.request_content_length` |
| Response body size | `http.response.body.size` | `http.response_content_length` |
| Caller's address, for the country | `client.address` | `http.client_ip`, `net.sock.peer.addr`, `net.peer.ip` |
| Consumer | `yuva.consumer` | |
| API version | `yuva.api_version` | |

The span's start is the request's time and its end minus its start is the duration. A span without a status counts
as status `0`. The resource gives the rest: `service.name` is the service, `deployment.environment.name` (or the older
`deployment.environment`) the environment, which is `production` without one, and `service.version` the release.

A route comes from `http.route`. A span without one is counted by its path, from which Yuva infers a route as
described in [Routes](analytics-api.md#routes), so the instrumentation should set `http.route`. The address is
shortened on arrival, used for the country and dropped; no query string, header or body is read.

`yuva.consumer` and `yuva.api_version` are not OpenTelemetry attributes: set them on the server span yourself, for
example in a middleware, with the name of the API key or client and the API version of the request. Never a person's
address.

```js
import { trace } from "@opentelemetry/api"

trace.getActiveSpan()?.setAttribute("yuva.consumer", apiKeyName)
trace.getActiveSpan()?.setAttribute("yuva.api_version", "2026-10")
```

A span goes through the same checks as a record sent with
[curl](analytics-api.md#send-requests-with-curl), including the caps on routes, consumers, versions and
environments, the 72 hours after which a record is not counted, and the
[monthly limit](configuration.md#api-monitoring). Records from OpenTelemetry are kept with the source `otlp`, so they
add up with the ones from the SDKs under the same routes.

## Sampling

A span stands for as many requests as its sampling says. Yuva reads the threshold in the span's `tracestate`
(`ot=th:…`, OpenTelemetry's probability sampling): `th:8` makes the span count 2, `th:c` count 4, and without a
threshold it counts 1. A sampler that drops spans without writing the threshold makes the counts too low. So export
every server span to Yuva, and sample only what you send to other backends. A threshold below 1 in 10,000 makes
the span invalid.

Unlike the `sample_rate` of a record sent directly, a sampled span may carry an error status, because head sampling
decides before the response is known.

## Check it by hand

The same endpoint takes OTLP/JSON, which is enough for a first test. The times are nanoseconds since 1970 and must
be recent:

```sh
NOW=$(date +%s)000000000
curl https://support.example.com/otlp/PROPERTY_ID/v1/traces \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "resourceSpans": [{
      "resource": { "attributes": [{ "key": "service.name", "value": { "stringValue": "shop-api" } }] },
      "scopeSpans": [{ "spans": [{
        "kind": 2,
        "startTimeUnixNano": "'$NOW'",
        "endTimeUnixNano": "'$((NOW + 12500000))'",
        "attributes": [
          { "key": "http.request.method", "value": { "stringValue": "GET" } },
          { "key": "http.route", "value": { "stringValue": "/users/{id}" } },
          { "key": "http.response.status_code", "value": { "intValue": "200" } }
        ]
      }] }]
    }]
  }'
```

```json
{}
```

## What comes back

| Answer | |
|---|---|
| `200` | The spans were taken. `{}` in JSON (an empty body in protobuf) when every span was valid. Otherwise `partialSuccess` has `rejectedSpans`, the number left out as invalid, and `errorMessage`, the first reason. Spans older than 72 hours and spans over the workspace's monthly limit are taken but not counted. |
| `429`, `503` | With `Retry-After`: the property's or the key's `YUVA_MONITOR_RECORDS_PER_MINUTE` took none of the spans, or the server holds as many open rows as it may. The exporter retries these on its own. |
| `400`, `401`, `403`, `404`, `415`, `422` | Refused: a body that breaks the shape, a missing or wrong key, a key without `analytics:write` or a member's session, an unknown property, a content type or encoding Yuva does not read, a site property. |

An error is a `google.rpc.Status` in the request's encoding, whose message starts with the problem code, for example
`insufficient_scope: …`. The exporter does not retry a refusal, so look in its log if numbers do not appear. A span
shows up in a query within about 15 seconds; see [Read the numbers](analytics-api.md#read-the-numbers).

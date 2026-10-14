# OTLP export fixtures

`traces.pb` and `traces.json` hold the same eleven spans (eight HTTP `SERVER` spans, one of them
invalid, two sampled through a parent tracestate `ot=th`; a `CLIENT`, an `INTERNAL` and a gRPC
`SERVER` span) as captured from real exporters posting to a local listener:

- `traces.pb`: OpenTelemetry Go `otlptracehttp` v1.47.0 (binary protobuf).
- `traces.json`: OpenTelemetry JS `@opentelemetry/exporter-trace-otlp-http` 0.223.0 with
  `@opentelemetry/sdk-trace-base` 2.12.0 (OTLP/JSON).

`traces-semconv-1.20.pb` holds the same spans from the same Go exporter under the HTTP semantic
conventions of 1.20 and before (`http.method`, `http.status_code`, `http.target`, `http.url`,
`http.request_content_length`, `http.response_content_length`, `http.client_ip`,
`net.sock.peer.addr`, `net.peer.ip` and the resource's `deployment.environment`).

All three are exporter output for span data written for these tests; nothing is copied from the
exporters' sources.

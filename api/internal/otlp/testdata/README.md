# OTLP export fixtures

The same eleven spans (eight HTTP `SERVER` spans, one of them invalid, two sampled through a parent
tracestate `ot=th`; a `CLIENT`, an `INTERNAL` and a gRPC `SERVER` span) as captured from real
exporters posting to a local listener:

- `traces.pb`: OpenTelemetry Go `otlptracehttp` v1.47.0 (binary protobuf).
- `traces.json`: OpenTelemetry JS `@opentelemetry/exporter-trace-otlp-http` 0.223.0 with
  `@opentelemetry/sdk-trace-base` 2.12.0 (OTLP/JSON).

Both are exporter output for span data written for these tests; nothing is copied from the
exporters' sources.

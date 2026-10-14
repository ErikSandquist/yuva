# API-Monitoring mit OpenTelemetry

Jeder mit OpenTelemetry instrumentierte Service kann seine Requests an eine API-Property melden: Richten Sie den
Trace-Exporter auf Yuva, und die Server-Spans werden zu Request-Records, ohne Yuva-Code im Service. Das ist der Weg für
jede Sprache ohne Yuva-Paket. Sie brauchen eine API-Property und einen Schlüssel mit dem Scope `analytics:write`: siehe
[Eine API-Property anlegen](analytics-api.md#eine-api-property-anlegen).

## Den Exporter einrichten

Zwei Umgebungsvariablen genügen:

```sh
OTEL_EXPORTER_OTLP_ENDPOINT=https://support.example.com/otlp/PROPERTY_ID
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer YUVA_API_KEY
```

Der Exporter hängt `/v1/traces` selbst an, die Traces gehen also an `POST /otlp/{propertyId}/v1/traces`. Setzen Sie
stattdessen `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`, geben Sie die ganze URL an, die auf `/v1/traces` endet. Der Schlüssel
ist ein API-Schlüssel mit `analytics:write`; die Sitzung eines Mitglieds und OAuth-Tokens werden abgewiesen (`403`).

- **Protokoll.** Binäres Protobuf (`Content-Type: application/x-protobuf`, `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`)
  und JSON (`application/json`, `http/json`) funktionieren. gRPC nicht. Jeder andere Content-Type antwortet mit `415`.
- **gzip.** Der Body darf mit `Content-Encoding: gzip` komprimiert sein (`OTEL_EXPORTER_OTLP_COMPRESSION=gzip`). So oder
  so ist ein Request höchstens 8 MiB groß.
- **Sampling.** Exportieren Sie jeden Server-Span: Setzen Sie `OTEL_TRACES_SAMPLER=always_on`, oder behalten Sie Ihren
  Sampler nur für die anderen Exporter. Siehe [Sampling](#sampling).

Benennen Sie Service und Umgebung mit den Standard-Ressourcenattributen, zum Beispiel `OTEL_SERVICE_NAME=shop-api` und
`OTEL_RESOURCE_ATTRIBUTES=deployment.environment.name=production,service.version=1.4.2`.

## Was gelesen wird

Nur Spans der Art `SERVER`, die eine HTTP-Methode tragen, zählen, jeder als ein Request. Client-Spans, interne Spans und
gRPC-Server werden übersprungen, ohne abgewiesen zu werden. Yuva liest die Attribute eines Spans nach den aktuellen
HTTP-Semantic-Conventions und, wo eines fehlt, nach den Namen von Version 1.20 und früher, die viele verbreitete SDKs
und Auto-Instrumentierungen noch senden.

| Request-Record | Attribut | Älterer Name, wenn es fehlt |
|---|---|---|
| Methode | `http.request.method` | `http.method` |
| Route | `http.route` | |
| Pfad, um eine Route abzuleiten | `url.path` | `http.target` (ohne Query), der Pfad von `http.url` |
| Status | `http.response.status_code` | `http.status_code` |
| Größe des Request-Body | `http.request.body.size` | `http.request_content_length` |
| Größe des Response-Body | `http.response.body.size` | `http.response_content_length` |
| Adresse des Aufrufers, für das Land | `client.address` | `http.client_ip`, `net.sock.peer.addr`, `net.peer.ip` |
| Consumer | `yuva.consumer` | |
| API-Version | `yuva.api_version` | |

Der Start des Spans ist die Zeit des Requests, sein Ende minus der Start die Dauer. Ein Span ohne Status zählt als
Status `0`. Den Rest liefert die Ressource: `service.name` ist der Service, `deployment.environment.name` (oder das ältere
`deployment.environment`) die Umgebung, ohne beides `production`, und `service.version` das Release.

Die Route kommt aus `http.route`. Ein Span ohne sie wird nach seinem Pfad gezählt, aus dem Yuva eine Route ableitet, wie
unter [Routen](analytics-api.md#routen) beschrieben; die Instrumentierung sollte also `http.route` setzen. Die Adresse
wird beim Eingang gekürzt, für das Land genutzt und verworfen; Query-String, Header und Body werden nicht gelesen.

`yuva.consumer` und `yuva.api_version` sind keine OpenTelemetry-Attribute: Setzen Sie sie selbst am Server-Span, etwa in
einer Middleware, mit dem Namen des API-Schlüssels oder Clients und der API-Version des Requests. Nie die Adresse einer
Person.

```js
import { trace } from "@opentelemetry/api"

trace.getActiveSpan()?.setAttribute("yuva.consumer", apiKeyName)
trace.getActiveSpan()?.setAttribute("yuva.api_version", "2026-10")
```

Ein Span durchläuft dieselben Prüfungen wie ein mit [curl](analytics-api.md#requests-mit-curl-senden) gesendeter Record,
einschließlich der Obergrenzen für Routen, Consumer, Versionen und Umgebungen, der 72 Stunden, nach denen ein Record
nicht mehr gezählt wird, und des [Monatslimits](configuration.md#api-monitoring). Records aus OpenTelemetry werden mit
der Quelle `otlp` gespeichert und addieren sich mit denen aus den SDKs unter denselben Routen.

## Sampling

Ein Span steht für so viele Requests, wie sein Sampling sagt. Yuva liest den Schwellenwert im `tracestate` des Spans
(`ot=th:…`, das Wahrscheinlichkeits-Sampling von OpenTelemetry): `th:8` lässt den Span 2 zählen, `th:c` 4, ohne
Schwellenwert zählt er 1. Ein Sampler, der Spans verwirft, ohne den Schwellenwert zu schreiben, macht die Zahlen zu
niedrig. Exportieren Sie also jeden Server-Span an Yuva und samplen Sie nur, was Sie an andere Backends senden. Ein
Schwellenwert unter 1 zu 10.000 macht den Span ungültig.

Anders als das `sample_rate` eines direkt gesendeten Records darf ein gesampelter Span einen Fehlerstatus tragen, weil
Head-Sampling entscheidet, bevor die Antwort bekannt ist.

## Von Hand prüfen

Derselbe Endpunkt nimmt OTLP/JSON, das für einen ersten Test reicht. Die Zeiten sind Nanosekunden seit 1970 und müssen
aktuell sein:

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

## Was zurückkommt

| Antwort | |
|---|---|
| `200` | Die Spans wurden angenommen. `{}` in JSON (ein leerer Body in Protobuf), wenn jeder Span gültig war. Sonst trägt `partialSuccess` die Zahl der als ungültig ausgelassenen Spans (`rejectedSpans`) und den ersten Grund (`errorMessage`). Spans älter als 72 Stunden und Spans über dem Monatslimit des Workspace werden angenommen, aber nicht gezählt. |
| `429`, `503` | Mit `Retry-After`: Das `YUVA_MONITOR_RECORDS_PER_MINUTE` der Property oder des Schlüssels hat keinen der Spans angenommen, oder der Server hält so viele offene Zeilen, wie er darf. Der Exporter wiederholt diese von selbst. |
| `400`, `401`, `403`, `404`, `415`, `422` | Abgewiesen: ein Body, der die Form verletzt, ein fehlender oder falscher Schlüssel, ein Schlüssel ohne `analytics:write` oder die Sitzung eines Mitglieds, eine unbekannte Property, ein Content-Type oder eine Kodierung, die Yuva nicht liest, eine Site-Property. |

Ein Fehler ist ein `google.rpc.Status` in der Kodierung des Requests, dessen Meldung mit dem Problemcode beginnt, zum
Beispiel `insufficient_scope: …`. Der Exporter wiederholt eine Abweisung nicht; erscheinen keine Zahlen, sehen Sie in
sein Log. Ein Span erscheint innerhalb von etwa 15 Sekunden in einer Abfrage; siehe
[Die Zahlen lesen](analytics-api.md#die-zahlen-lesen).

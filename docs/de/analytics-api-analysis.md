# API-Monitoring auswerten

Diese Seite erklärt, was jede Zahl einer API-Property bedeutet, wie die Perzentile berechnet werden und welche
Aufschlüsselungen und Vergleiche Sie abfragen können. Sie baut auf [API-Monitoring](analytics-api.md) auf. Das Panel
zeigt diese Ansichten in der **Übersicht** der Property und auf der Seite jeder Route; dieselben Zahlen kommen aus der
Analytics-Abfrage.

## Was die Zahlen bedeuten

Jede Zahl wird über den gewählten Zeitraum gezählt. Ein gesampelter Request (`sample_rate` über 1) zählt in jeder Zahl
als so viele Requests; eine Antwort, die auf gesampelten Records beruht, ist daher eine Schätzung und trägt
`meta.estimated: true`.

| Metrik | Bedeutung |
|---|---|
| `requests` | Beantwortete Requests. |
| `requests_per_minute` | `requests` geteilt durch die Minuten des Zeitraums oder Buckets bis jetzt: der Durchsatz. |
| `errors`, `error_rate` | Requests mit Status 500 bis 599 oder `0`, wenn keine Antwort gesendet wurde, und ihr Anteil an allen Requests in Prozent. |
| `client_errors`, `client_error_rate` | Dasselbe für Status 400 bis 499. |
| `p50`, `p75`, `p90`, `p95`, `p99` | Die Dauer in Millisekunden, innerhalb der dieser Anteil der Requests blieb. Ein `p95` von 240 heißt: 95 von 100 Requests dauerten 240 ms oder weniger. |
| `avg_duration`, `max_duration` | Mittlere und längste Dauer in Millisekunden. Beide sind exakt. |
| `apdex` | Ein Wert von 0 bis 1 dafür, wie viele Requests schnell genug waren; siehe unten. |
| `request_bytes`, `response_bytes` | Summe der Body-Bytes der Records, die eine Größe angegeben haben. |
| `response_size_p50`, `response_size_p95` | Die Größe in Bytes, innerhalb der dieser Anteil der Antworten blieb. |
| `conversations` | Konversationen, die der verknüpfte Posteingang der Property erhalten hat, nach Tag. |
| `latency_bands` | Requests je Dauerband: die Daten der Latenz-Heatmap. |

### Wie Perzentile berechnet werden

Yuva speichert keine Zeile pro Request und kann die Dauern daher nicht sortieren. Stattdessen führt jede Minute, Stunde
und jeder Tag ein kleines Histogramm, dessen Buckets um den Faktor 2^(1/32) wachsen, etwa 2,2 %. Ein Request wird in dem
Bucket gezählt, in den seine Dauer fällt; die Histogramme verschiedener Minuten werden Bucket für Bucket addiert, und ein
Perzentil wird als der Bucket gelesen, der diesen Rang der Requests enthält. Als Wert eines Buckets gilt seine Mitte,
darum liegt ein Perzentil innerhalb von 1,1 % des exakten Werts, egal bei welchem Zeitraum, welcher Aufschlüsselung und
welcher Anzahl Requests. Ein zweites Histogramm mit Buckets von 2^(1/4) hält die Antwortgrößen; `response_size_p50` und
`response_size_p95` liegen deshalb innerhalb von 9 %.

Das Perzentil einer Gruppe lässt sich nicht aus den Perzentilen ihrer Teile berechnen, Histogramme aber addieren sich
exakt. Das `p95` einer Route über einen Monat ist also das wahre `p95` dieses Monats, kein Mittel von Tageswerten.

### Apdex

Apdex teilt die Dauern anhand eines Schwellenwerts T in Millisekunden in drei Gruppen: **zufrieden** sind Requests, die
höchstens T dauerten, **tolerierbar** die mit mehr als T und höchstens 4T, der Rest ist **frustrierend**. Der Wert ist
(zufrieden + tolerierbar / 2) / Requests; 1 heißt, jeder Request war schnell genug, 0, keiner war es.

T ist das `apdex_ms` der Property (standardmäßig 500 ms, 1 bis 60.000). Eine von Natur aus langsame Route, etwa ein
Export, hat ein eigenes `apdex_ms`; `null` kehrt zu dem der Property zurück. Setzen Sie es auf der Seite der Route im
Panel oder mit `PATCH /v1/analytics/properties/{propertyId}/routes/{routeId}` (Scope `analytics:manage`). Apdex wird aus
den Dauer-Histogrammen gelesen; ein Request am Rand eines Buckets kann daher wie bei den Perzentilen eine Gruppe daneben
landen.

### Latenzbänder

`latency_bands` liefert 20 Zahlen, die zusammen `requests` ergeben. Band 0 enthält Requests von höchstens 256
Mikrosekunden, jedes weitere Band die bis zur nächsten Zweierpotenz (512 µs, 1,02 ms und so weiter bis etwa 67
Sekunden), das letzte alles Langsamere. Die oberen Grenzen stehen in `meta.latency_band_bounds` und sind in jeder
Antwort gleich. Weil die Buckets des Histogramms auf diesen Grenzen enden, sind die Zahlen exakt, nicht geschätzt. Das
Panel zeichnet sie auf der Seite einer Route als **Latenz-Heatmap**: auf der einen Achse die Zeit, auf der anderen die
Bänder.

## Aufschlüsseln und filtern

Sie können nach `route`, `method`, `status`, `status_class`, `api_version`, `environment`, `service`, `source`
(`server` für Ihren Server, `otlp` für [OpenTelemetry](analytics-api-otel.md), `app` für Ihre
[Apps](analytics-api-apps.md)) und `platform` (`ios` oder `android` bei App-Records) aufschlüsseln, in jeder
Kombination und über jeden Zeitraum, mit höchstens zwei Dimensionen, von denen eine eine Zeitauflösung sein darf. Filter
nutzen dieselben Dimensionen. Im Panel sind das die **Aufschlüsselung** und die Filter.

Drei weitere Dimensionen, `consumer`, `country` und `app_version`, lassen sich je einmal hinzufügen, mit Grenzen:

- **Eine zusätzliche Dimension auf einmal.** Zwei davon in einer Abfrage, oder eine als Dimension und eine als Filter,
  werden abgelehnt.
- **Nur mit `route`, `method` und `status_class`.** Eine zusätzliche Dimension lässt sich weder als Dimension noch als
  Filter mit `status`, `api_version`, `environment`, `service`, `source`, `platform` oder einer zweiten zusätzlichen
  verbinden.
- **Nur über ganze lokale Stunden.** Nutzen Sie `time:hour` oder gröbere Auflösungen, Zeiträume in Tagen oder Wochen oder
  `since` und `until` auf der vollen Stunde. `15m`, `1h`, `6h`, `24h` und `time:minute` werden abgelehnt, weil Minuten
  nicht je zusätzlicher Dimension gespeichert werden.
- **Requests ohne Wert fehlen.** Eine Aufschlüsselung nach `consumer` lässt Requests aus, die keinen Consumer (oder kein
  Land, keine App-Version) genannt haben; ihre Zeilen können deshalb weniger ergeben als die Gesamtsumme.

Eine nicht gespeicherte Kombination antwortet mit `422 dimension_combination`; das `detail` sagt, was zu ändern ist. Das
Panel zeigt dieselbe Meldung statt eines leeren Diagramms.

Eine Property zählt höchstens 10.000 Consumer und 200 Länder oder App-Versionen; weitere Werte werden als `(other)`
gezählt, und `meta.caps_reached` zeigt das an.

## Consumer

Die Seite **Consumer** zeigt Requests, Fehler und Latenz je Consumer, **Top-Consumer** auf der Seite einer Route
dasselbe für diese Route. Der Consumer ist der Wert, den Ihr Server in `consumer` sendet: der Name eines API-Schlüssels,
ein Client, eine App. Er ist eine zusätzliche Dimension wie die anderen, es gelten also die Grenzen oben: ganze Stunden,
und nur `route`, `method` oder `status_class` als weitere Dimension.

## Zeiträume vergleichen

`compare` fügt der Antwort dieselben Zahlen eines anderen Zeitraums hinzu: `previous` (ein gleich langer Zeitraum direkt
davor), `year` (dieselben Tage ein Jahr früher) oder `range` (die Tage `from` bis `to`). Bei Minutenzeiträumen (`15m`
bis `24h` oder `since` und `until`) ist `previous` die gleiche Zahl Minuten direkt davor und `year` dieselben Zeitpunkte
ein Jahr früher. Die Vergleichsanzeige des Panels zeichnet den Vergleichszeitraum als zweite Linie. `insights` bleiben
für API-Abfragen nicht verfügbar.

## Neue und stille Routen

`routes: new` listet die Routen, die Ihr Server im Zeitraum zum ersten Mal erhalten hat, `routes: silent` die im
vorherigen, gleich langen Zeitraum aufgerufenen und seit Beginn des Zeitraums nicht mehr. Es braucht die Dimension
`route` allein. Jede gelistete Route hat eine Zeile, bei null Requests mit Nullen, und `compare` mit `previous` zeigt die
früheren Zahlen einer stillen Route. `(unmatched)`, `(other)` und ausgeblendete Routen werden nie gelistet. Im Panel sind
das die Tabs **Neu** und **Still** der Routenliste.

## Deploy-Markierungen

Bringt ein Record ein `release`, das für seinen Service und seine Umgebung neu ist, legt Yuva zur Zeit dieses Records eine
`deploy`-Annotation mit Service, Umgebung und Release an. Das erste Release, das ein Service sendet, zählt mit; die
Rückkehr zu einem schon gesehenen Release legt keine an; höchstens 50 je Property und Tag. Das Panel zeichnet sie als
Linien in den Zeitdiagrammen und listet sie auf. Sie sind schreibgeschützt wie Yuvas andere Annotationen. Senden Sie für
Markierungen `release` mit Ihren Records (bei [OpenTelemetry](analytics-api-otel.md) wird `service.version` dazu).

## Zeiträume und Aktualität

Neben den Zeiträumen von [Analytics](analytics.md) nimmt eine API-Abfrage `15m`, `1h`, `6h` und `24h` bis zur aktuellen
Minute sowie `since` und `until` als Zeitpunkte auf der vollen Minute (`until` nicht eingeschlossen, höchstens 400 Tage
auseinander). Minuten- und Stundenbezeichnungen in einer Antwort sind die Ortszeit der Property mit UTC-Offset, etwa
`2026-10-12T14:05+03:00`, sodass eine bei der Zeitumstellung wiederholte Stunde getrennt bleibt. Abgeschlossene Tage und
Stunden werden aus Tages- und Stundenwerten gelesen, der Rest aus Minuten; ein neuer Record erscheint nach etwa 15
Sekunden. Die Minuten werden [`YUVA_MONITOR_MINUTE_DAYS`](configuration.md#api-monitoring) Tage aufbewahrt.

## API

Alle Abfragen sind `POST /v1/analytics/properties/{propertyId}/query` mit einem Schlüssel mit `analytics:read`. Geben
Sie das JSON unten als Body mit:

```sh
curl https://support.example.com/v1/analytics/properties/PROPERTY_ID/query \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '<BODY>'
```

Perzentile, Apdex und Größen, langsamste Routen zuerst:

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

Filtern und aufschlüsseln (5xx eines Services, nach Route und API-Version):

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

Consumer einer Route (eine zusätzliche Dimension mit `route`, über ganze Tage):

```json
{
  "dataset": "api",
  "metrics": ["requests", "errors", "p95"],
  "dimensions": ["consumer"],
  "filters": [{ "dimension": "route", "op": "is", "values": ["/users/{id}"] }],
  "range": { "period": "30d" }
}
```

Dieselben Summen gegen den Vorzeitraum:

```json
{
  "dataset": "api",
  "metrics": ["requests", "error_rate", "p95"],
  "range": { "period": "7d" },
  "compare": { "with": "previous" }
}
```

Neue Routen der letzten Woche (für Routen, die aufgehört haben, `"silent"` nehmen):

```json
{
  "dataset": "api",
  "metrics": ["requests"],
  "dimensions": ["route"],
  "routes": "new",
  "range": { "period": "7d" }
}
```

Die Latenz-Heatmap, je Stunde von heute ein Satz Bänder (Grenzen in `meta.latency_band_bounds`):

```json
{
  "dataset": "api",
  "metrics": ["latency_bands"],
  "dimensions": ["time:hour"],
  "range": { "period": "today" }
}
```

Ein Minutenfenster mit `since` und `until`:

```json
{
  "dataset": "api",
  "metrics": ["requests_per_minute", "p95"],
  "dimensions": ["time:minute"],
  "range": { "since": "2026-10-12T14:00:00+03:00", "until": "2026-10-12T15:00:00+03:00" }
}
```

Konversationen, die der verknüpfte Posteingang erhalten hat, je Tag (kein Filter, keine andere Dimension;
`422 no_linked_inbox`, wenn die Property keinen verknüpften Posteingang hat):

```json
{
  "dataset": "api",
  "metrics": ["conversations"],
  "dimensions": ["time:day"],
  "range": { "period": "30d" }
}
```

Der eigene Apdex-Schwellenwert einer Route:

```sh
curl -X PATCH https://support.example.com/v1/analytics/properties/PROPERTY_ID/routes/ROUTE_ID \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "apdex_ms": 5000 }'
```

Deploy-Markierungen sind Annotationen der Property mit der Art `deploy`:

```sh
curl "https://support.example.com/v1/analytics/properties/PROPERTY_ID/annotations?from=2026-10-01&to=2026-10-14" \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

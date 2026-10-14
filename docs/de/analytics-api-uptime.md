# Uptime-Checks

Ein Uptime-Check ruft eine URL von Ihnen vom Yuva-Server aus in einem festen Intervall auf und hält fest, ob sie wie
erwartet geantwortet hat, wie lange jeder Schritt gedauert hat und wann ihr Zertifikat abläuft. Nutzen Sie ihn für das,
was Request-Records nicht sagen können: Eine API, die ausgefallen ist, erhält keine Requests, also meldet nichts, dass
sie fehlt. Diese Seite baut auf [API-Monitoring](analytics-api.md) auf; [Alerts](analytics-api-alerts.md) sagen Ihnen,
wann ein Check ausfällt.

## Einen Check hinzufügen

Owner und Admins sowie API-Schlüssel ohne Posteingangs-Beschränkung legen Checks mit dem Scope `analytics:manage` an:

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

Der Check läuft sofort und danach in seinem Intervall. Ein Mitglied, das einen Check anlegt, erhält dafür eine
Alert-Regel, die an es geht: siehe [Standardregeln](analytics-api-alerts.md#standardregeln).

| Feld | Standard | Bedeutung |
|---|---|---|
| `name` | erforderlich | 1 bis 100 Zeichen. |
| `url` | erforderlich | Eine `http`- oder `https`-URL mit Host, ohne Zugangsdaten und ohne Fragment, bis zu 2.000 Zeichen. |
| `method` | `GET` | `GET` oder `HEAD`. |
| `headers` | keine | Bis zu 5 Request-Header, jeweils `{ "name": "Authorization", "value": "Bearer …" }`. Siehe unten. |
| `expected_statuses` | beliebig von 200 bis 399 | Bis zu 20 Statuscodes von 200 bis 399, die als erreichbar zählen. |
| `keyword` | keines | Text, den die ersten 64 KiB der Antwort enthalten müssen; nur `GET`. |
| `keyword_absent` | `false` | Kehrt `keyword` um: Die Antwort darf den Text nicht enthalten. |
| `timeout_seconds` | `10` | 1 bis 30. |
| `interval_seconds` | `60`, oder das Minimum des Servers, wenn länger | 30 bis 86.400 und mindestens `YUVA_MONITOR_MIN_INTERVAL` (`422 interval_too_short`). |
| `failures_before_down` | `2` | 1 bis 10 fehlgeschlagene Läufe in Folge, bevor der Check als ausgefallen gilt. |
| `cert_warning_days` | `14` | 1 bis 90. Läuft das Zertifikat eines `https`-Checks innerhalb dieser Tage ab, entsteht ein `certificate`-Alert. |
| `max_redirects` | `5` | 0 bis 5. Mit `0` wird der Status der Weiterleitung selbst bewertet. |
| `service` | keiner | Der Service, den der Check überwacht, unter dem Namen, den Ihre Records und Spans angeben. |
| `property_id` | keine | Die API-Property, deren `uptime`-Zahlen und Berichte den Check enthalten. |
| `paused` | `false` | Ein pausierter Check läuft nicht. |

`PATCH /v1/monitoring/checks/{checkId}` ändert jedes davon; der Check wird danach als Ganzes geprüft. Eine geänderte URL,
Methode oder ein geändertes Intervall lässt den Check sofort als Nächstes laufen, `paused: true` stoppt ihn,
`paused: false` lässt ihn sofort laufen. `DELETE` entfernt den Check samt Ergebnissen, Tageszeilen, Alert-Regeln und
Alerts.

### Header

Mit Headern übergibt ein Check einen `Authorization`-Header oder ein Token, das Ihr Health-Endpunkt verlangt. Die Werte
werden mit dem Hauptschlüssel des Servers versiegelt und sind nur schreibbar: Die API gibt nur `header_names` zurück,
nie einen Wert. `headers` ersetzt bei einer Änderung alle Header einschließlich der Werte, senden Sie also die ganze
Liste. `Host`, `Cookie`, `Content-Length`, `Transfer-Encoding`, `Connection`, `Upgrade`, `TE`, `Trailer` und `Proxy-*`
lassen sich nicht setzen.

## Was der Server aufruft und was nicht

Der Server ruft nur öffentliche Adressen auf. Eine URL wird mit `422 refused_address` abgelehnt, wenn sie Zugangsdaten,
`localhost` oder eine Adresse aus einem dieser Bereiche nennt:

- Loopback,
- privat,
- Link-Local,
- Carrier-Grade NAT (CGNAT),
- Multicast,
- reserviert,
- Cloud-Metadaten.

Ein Hostname wird beim Speichern des Checks und bei jedem Lauf aufgelöst und abgelehnt, wenn eine seiner Adressen zu
diesen gehört. Jeder Weiterleitungsschritt wird genauso geprüft, und die Verbindung geht an die geprüfte Adresse; ein
Name, der zwischen zwei Abfragen seine Antwort ändert, kann den Server also nicht zu einer privaten Adresse führen. Ein
Check, der bei einem späteren Lauf abgelehnt wird, schlägt mit dem Fehler `refused_address` fehl.

Ein selbst betriebener Server, der interne Dienste überwachen soll, setzt `YUVA_MONITOR_ALLOW_PRIVATE=true`
([Konfiguration](configuration.md#api-monitoring)). Damit erreichen Checks private und Loopback-Adressen. Link-Local- und
Cloud-Metadaten-Adressen bleiben auch dann abgelehnt.

Der Server sendet `User-Agent: Yuva-Uptime/<Version>`, keine Cookies und keinen Body und liest höchstens 64 KiB der
Antwort. Jeder Verbindungsversuch gibt nach 10 Sekunden auf, was auch immer `timeout_seconds` des Checks sagt.

## Grenzen

| Grenze | Wert |
|---|---|
| Checks pro Workspace | `YUVA_MONITOR_CHECKS`: 500, oder 50 auf einem Server mit offener Registrierung (`403 check_limit`). |
| Checks auf einem Host | 10 (`403 host_limit`). |
| Kürzestes Intervall | `YUVA_MONITOR_MIN_INTERVAL`: 30 Sekunden, oder 60 auf einem Server mit offener Registrierung. |
| Aufbewahrte Läufe | `YUVA_MONITOR_CHECK_DAYS` Tage, standardmäßig 30; ältere Tage bleiben 400 Tage als Tageszahlen. |

## Wenn ein Check ausfällt

Ein Lauf ist erfolgreich, wenn die Antwort rechtzeitig kommt, ihr Status in `expected_statuses` liegt und die
Keyword-Bedingung erfüllt ist. Ein fehlgeschlagener Lauf wird nach 20 Sekunden wiederholt, und nur dieser zweite
Versuch zählt. Der Check ist `down`, sobald `failures_before_down` gezählte Läufe in Folge fehlgeschlagen sind, und
nach einem erfolgreichen wieder `up`. Vor seinem ersten gezählten Lauf ist er `unknown`.

Ein fehlgeschlagener Lauf trägt einen dieser Fehler: `dns`, `refused_address`, `connect`, `tls`, `timeout`, `status`,
`keyword`, `redirects` oder `other`.

**Unbekannte Minuten.** Liefen auf dem Server in einer Minute mindestens 20 Checks und fiel mehr als die Hälfte davon
aus, liegt der Fehler eher im eigenen Netz von Yuva als bei Ihren Endpunkten. Die Läufe dieser Minute werden als
`unknown` festgehalten: Sie zählen weder als erreichbar noch als ausgefallen und tauchen in keiner Uptime-Zahl auf.

**Verzögerung.** Eine Minute wird etwa 40 Sekunden nach ihrem Ende bewertet; ein gestoppter Endpunkt erscheint daher mit
den zwei Standard-Fehlschlägen nach 2 bis 3 Minuten als ausgefallen. Ein noch nicht bewerteter Lauf hat in den
Ergebnissen den Status `pending`.

## Ergebnisse lesen

`GET /v1/monitoring/checks` listet die Checks mit Status, `state_since`, `next_run_at`, `cert_expires_at` und letztem
Ergebnis. `GET /v1/monitoring/checks/{checkId}/results` listet die gezählten Läufe, neueste zuerst, bis zu 500 pro Seite
(`limit`, und `before` für die nächste Seite). Jeder Lauf hat:

| Feld | Bedeutung |
|---|---|
| `at` | Wann der gezählte Versuch begann. |
| `ok`, `state` | Ob er erfolgreich war, und der Status danach: `up`, `down`, `unknown` oder `pending`. |
| `status` | Der Status der letzten Antwort; `0`, wenn keine kam. |
| `error`, `detail` | Warum er fehlschlug. Das Detail ist ein fester Text; er zeigt nie aufgelöste Adressen. |
| `retried` | Der erste Versuch schlug fehl, und dies ist der Versuch 20 Sekunden später. |
| `redirects` | Gefolgte Weiterleitungen. |
| `timings` | `dns`, `connect`, `tls`, `ttfb` und `total` in Millisekunden. |
| `cert_days_left` | `https`: ganze Tage bis zum Ablauf des Zertifikats. |

### Das Dataset uptime

Zahlen über die Zeit kommen aus der Abfrage mit `dataset: uptime`: `POST /v1/monitoring/query` liest jeden Check des
Workspace, auch solche ohne Property, und `POST /v1/analytics/properties/{propertyId}/query` die Checks dieser Property.
Die Dimension ist `check`, mit einer Zeitgranularität; nutzen Sie `time:minute` oder `time:hour` innerhalb der
aufbewahrten Läufe und `time:day`, `time:week` oder `time:month` für jeden Zeitraum. Buckets und Tage sind in UTC, was
auch immer die Zeitzone der Property ist.

| Metrik | Bedeutung |
|---|---|
| `runs`, `failures` | Gezählte Läufe und davon fehlgeschlagene. |
| `up` | Checks, deren letzter gezählter Lauf des Zeitraums oder Buckets erfolgreich war. |
| `uptime` | Prozent der überwachten Zeit, in der der Check nicht ausgefallen war; jeder Lauf steht für sein Intervall. |
| `downtime` | Sekunden im Ausfall. |
| `response_time_p50`, `response_time_p95`, `response_time_p99` | Gesamtzeit der Läufe mit Antwort in Millisekunden, auf 1,1 % genau. |
| `avg_response_time`, `ttfb` | Mittlere Gesamtzeit und Zeit bis zum ersten Byte in Millisekunden. |
| `cert_days_left` | Die wenigsten Tage bis zum Ablauf eines Zertifikats. |

Unbekannte Läufe zählen in keiner davon. Hängen Sie `?format=csv` an `POST /v1/monitoring/query` für eine CSV-Datei.

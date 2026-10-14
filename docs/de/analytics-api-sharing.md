# API-Monitoring teilen, exportieren und aufbewahren

Ein Freigabelink zeigt einen Teil einer [API-Property](analytics-api.md) Menschen ohne Yuva-Konto, ein Export schreibt
ihre Zahlen in eine Datei, und die Aufbewahrung legt fest, wie lange jede Art von Zahlen bleibt. Eine
Löschanfrage zu einem einzelnen Consumer beantworten Sie mit einem Aufruf.

## Freigabelinks

API-Properties nutzen dieselben Links wie Websites: Sie
[erstellen, ändern, widerrufen und löschen sie](analytics-sharing.md#freigabelinks) mit denselben Aufrufen, mit
demselben Token, Passwort, Ablaufdatum, Einbetten und Limit von 50 Links. Anders sind die Ansichten. Ein Link auf eine
API-Property listet `api_`-Ansichten, ein Link auf eine Website listet Website-Ansichten; beides zu mischen
antwortet `422 property_kind`.

| Ansicht | Was der Betrachter sieht |
|---|---|
| `api_overview` | Summen und Diagramme aller Metriken, aufgeschlüsselt nach `status`, `status_class`, `environment`, `service`, `api_version`, `source`, `platform`, `country` und `app_version`. |
| `api_routes` | Ergänzt `route`, `method` und `routes` (neue und stille Routen). |
| `api_consumers` | Ergänzt `consumer`, als Aufschlüsselung oder Filter. |

Summen und Zeitraster gehören zu jeder Ansicht. Eine Abfrage nach einer Dimension oder einem Filter, den keine
Ansicht des Links besitzt, wird mit `403 view_not_shared` abgelehnt. Consumer-Namen erscheinen deshalb **nur**, wenn der
Link `api_consumers` listet; ohne diese Ansicht zeigt ein Link nie, wer Ihre API aufruft.

Ein Link auf eine API-Property teilt `uptime`, `support` und `conversations` nie: eine Abfrage des Datensatzes `uptime`
oder `support` oder der Metrik `conversations` antwortet `403 view_not_shared`. Gespeicherte Segmente werden wie bei
Websites abgelehnt, und eine Freigabe hat keinen Export.

Einen Link für Übersicht und Routen ohne Consumer erstellen:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/shares \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{
    "name": "Status für Partner",
    "views": ["api_overview", "api_routes"],
    "expires_at": "2027-01-01T00:00:00Z"
  }'
```

Das Skript eines Betrachters liest ihn mit `POST /client/v1/analytics-shares/{token}/query`; der Body ist derselbe wie
bei `POST /v1/analytics/properties/{propertyId}/query`, mit `"dataset": "api"`:

```sh
curl -X POST https://support.example.com/client/v1/analytics-shares/yuva_sh_xxxxxxxx/query \
  -H "Content-Type: application/json" \
  -d '{"dataset": "api", "metrics": ["requests", "error_rate", "p95"], "dimensions": ["route"], "range": {"period": "24h"}}'
```

Ein Link, den Sie widerrufen, hört sofort auf zu funktionieren, wie bei einer Website. Die Freigabeseite teilt dem
Browser des Betrachters mit, welche Art Property sie zeigt, und zeichnet so die API-Diagramme.

## Exporte

Ein Export läuft im Hintergrund und behält seine Datei **7 Tage**, wie unter
[Exporte](analytics-sharing.md#exporte) beschrieben. Für eine API-Property:

| Art | Inhalt | Datensätze |
|---|---|---|
| `view` | Jede Zeile einer `query`, die Tabelle, die Sie im Panel sähen, ohne Vergleich. | `api`, `uptime` |
| `daily` | Eine Zeile pro Tag von `range` (höchstens 400 Tage), optional nach einer `dimension` aufgeteilt. | `api` (Standard), `uptime` |

Beide kommen als CSV (Standard) oder NDJSON. Für eine API-Property gibt es **keinen `raw`-Export**, weil sie keine Zeile
pro Request speichert: ein `raw`-Export antwortet `422 property_kind`, der Datensatz `traffic` ebenso. Der Datensatz
`support` wird nicht exportiert.

Der Zeitraum wird beim Anlegen des Exports festgelegt. Ein `view`-Export mit `"period": "24h"` enthält die 24 Stunden,
die endeten, als Sie ihn angefordert haben, egal wie lange die Datei wartet; später erneut anzufordern ergibt einen
neueren Zeitraum. Dieselbe Abfrage, die das Panel ausführt, liefert dieselben Zeilen. Eine ungültige Abfrage wird sofort
abgelehnt, nicht erst nach dem Start des Jobs.

Ein `daily`-Export hat zuerst den Tag, dann die Dimension, falls Sie eine angegeben haben, dann diese Spalten:

| Datensatz | Spalten, in dieser Reihenfolge | Tage |
|---|---|---|
| `api` | `requests`, `errors`, `error_rate`, `client_errors`, `client_error_rate`, `p50`, `p95`, `p99`, `avg_duration`, `apdex`, `request_bytes`, `response_bytes` | Lokale Tage der Property |
| `uptime` | `runs`, `failures`, `uptime`, `downtime`, `response_time_p50`, `response_time_p95`, `response_time_p99`, `avg_response_time` | UTC-Tage |

Die `dimension` eines `api`-Exports ist eine beliebige API-Dimension, die eines `uptime`-Exports `check`. Raten sind
Prozent, Dauern Millisekunden.

Einen Monat Requests nach Route exportieren und herunterladen:

```sh
EXPORT=$(curl -s -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/exports \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "daily", "dataset": "api", "format": "csv", "range": {"period": "30d"}, "dimension": "route"}' | jq -r .id)

curl -o routes.csv https://support.example.com/v1/analytics/properties/$PROPERTY/exports/$EXPORT/download \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

Warten Sie vor dem Download, bis `GET …/exports/{exportId}` `status: done` zeigt. Aufrufe, Scopes und Limits sind die
der [Website-Exporte](analytics-sharing.md#api).

## Wie lange Zahlen bleiben

API-Monitoring hält seine Zahlen in Stufen, jede so lange wie nötig:

| Zahlen | Aufbewahrt | Festgelegt durch |
|---|---|---|
| Minuten (`time:minute`) | Standard 8 Tage | Der Betreiber, serverweit: [`YUVA_MONITOR_MINUTE_DAYS`](configuration.md#api-monitoring), 2 bis 31 |
| Stunden | Standard 90 Tage | `monitor_hour_days` der Property, 7 bis 400 |
| Tage und die Aufschlüsselungen nach Dimension | Standard 25 Monate | `retention_months` der Property, 1 bis 120, oder `null` für unbegrenzt |
| Läufe eines Uptime-Checks | Standard 30 Tage | Der Betreiber: [`YUVA_MONITOR_CHECK_DAYS`](configuration.md#api-monitoring), 2 bis 90 |
| Tageszahlen eines Checks, Zustandswechsel und Dienstverlauf | 400 Tage | Fest |

Die beiden Werte pro Property ändern Sie mit `PATCH /v1/analytics/properties/{propertyId}`:

```sh
curl -X PATCH https://support.example.com/v1/analytics/properties/$PROPERTY \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{"monitor_hour_days": 30, "retention_months": 13}'
```

Ein Hintergrundjob entfernt, was seine Stufe überschreitet. Ein kürzerer Wert löscht die älteren Zahlen endgültig; ein
längerer bringt nicht zurück, was schon weg war. Das workspaceweite `retention_days` der Websites gilt nicht für
API-Properties.

Das Löschen einer API-Property löscht ihre Zahlen in jeder Stufe. Ihre Uptime-Checks werden nicht gelöscht: Sie laufen
ohne Property weiter, und ihre eigenen Ergebnisse unterliegen ihrer eigenen Aufbewahrung.

## Einen Consumer löschen

Ein Consumer ist der Name, den Ihre Records dem Aufrufer Ihrer API geben. Verlangt jemand die Löschung seines Namens,
suchen Sie die ID und löschen Sie ihn. Beide Aufrufe gelten für API-Properties; eine Website antwortet
`422 property_kind`.

```sh
curl "https://support.example.com/v1/analytics/properties/$PROPERTY/consumers?q=acme" \
  -H "Authorization: Bearer $YUVA_API_KEY"

curl -X DELETE https://support.example.com/v1/analytics/properties/$PROPERTY/consumers/$CONSUMER \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

`GET …/consumers` listet die Consumer, die die Property gezählt hat, mit `id`, `name` und `first_seen_at`; `q` behält
die Namen, die den Text enthalten, ein Cursor blättert durch die Liste. Es braucht `analytics:read`. `(other)`, wo
Requests über dem Limit von 10.000 Consumern gezählt werden, wird nicht gelistet und lässt sich nicht löschen
(`400 validation_failed`). `DELETE` braucht `analytics:manage` und einen Owner, einen Admin oder einen API-Schlüssel
ohne Posteingangs-Beschränkung; es antwortet `204`.

Was die Löschung tut:

- Sie löscht die stündlichen und täglichen Zeilen des Consumers in jeder Stufe und seinen Namen. Panel und API zeigen
  ihn danach in keiner `consumer`-Aufschlüsselung mehr und akzeptieren ihn nicht als Filter.
- Ihre Summen ändern sich nicht. Minuten-, Stunden- und Tagessummen haben den Consumer nie gehalten; seine Requests
  bleiben gezählt, nur keinem zugeordnet.
- Sie wird 2 und 11 Minuten später noch zweimal wiederholt, um zu löschen, was ein Serverprozess beim Auslösen noch im
  Speicher aufsummierte.
- Die ID wird nie an einen anderen Consumer vergeben. Nennen Ihre Records denselben Consumer erneut, zählt er unter
  einer neuen ID.
- Export-Dateien, die vorher entstanden sind, behalten den Namen, bis sie 7 Tage nach ihrer Erstellung ablaufen.
  Früher löschen Sie sie mit `DELETE /v1/analytics/properties/{propertyId}/exports/{exportId}`.

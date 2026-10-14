# API-Monitoring

Eine API-Property zählt die Requests, die Ihre eigene API beantwortet: wie viele es waren, wie viele fehlschlugen
und wie lange sie dauerten, nach Route. Ihr Server meldet jeden Request, den er beantwortet hat; Yuva addiert sie
pro Minute und Stunde, ohne eine Zeile pro Request, ohne Header, Body oder IP-Adresse zu speichern. Diese Seite
baut auf [Analytics](analytics.md) auf.

## Eine API-Property anlegen

Eine API-Property ist eine Analytics-Property mit `kind: api`. Sie hat keine Domains und kein Tracking-Skript.
Owner und Admins legen sie mit einem Schlüssel mit dem Scope `analytics:manage` an:

```sh
curl https://support.example.com/v1/analytics/properties \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "kind": "api", "name": "Shop API", "domains": [], "timezone": "Europe/Berlin" }'
```

Die Antwort enthält die `id` der Property. Die Zeitzone der Property bestimmt, nach welchen Stunden und Tagen ihre
Zahlen gezählt werden; auch die Minuten einer `time:minute`-Abfrage liegen in ihr.

Legen Sie dann unter **Settings → API keys** den Schlüssel an, der die Records sendet, mit dem Scope
`analytics:write`, und behalten Sie ihn auf dem Server. Die Sitzung eines Mitglieds wird abgelehnt
(`403 forbidden`), eine Website-Property antwortet mit `422 property_kind`. Ein mit `YUVA_MONITOR=off` gestarteter
Server hat keine API-Properties.

## Requests senden

Wählen Sie Ihren Stack. Alle senden dieselben Records, die Zahlen sind also überall gleich.

- [Go](analytics-api-go.md): Middleware für `net/http`.
- [Node, Bun und Deno](analytics-api-js.md): srvx, h3, Nitro, Nuxt, Hono, Express, Fastify und Next.js-Route-Handler.
- [OpenTelemetry](analytics-api-otel.md): jede andere Sprache, mit zwei Umgebungsvariablen und ohne Yuva-Code.
- [iOS- und Android-Apps](analytics-api-apps.md): die Aufrufe Ihrer App an Ihre eigene API, auf dem Gerät gemessen.
- curl, weiter unten, für alles andere.

## Requests mit curl senden

Jeder andere Stack sendet dieselben Records. `POST /v1/analytics/properties/{propertyId}/requests` nimmt 1 bis
1.000 Records und antwortet mit `202`. Der Body darf mit `Content-Encoding: gzip` komprimiert sein.

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

| Feld | |
|---|---|
| `method` | Pflicht, 1 bis 20 Zeichen. Die Methoden aus RFC 9110 und `PATCH` bleiben erhalten; alles andere zählt als `_OTHER`. |
| `route` | Das Muster, das der Router gefunden hat, beginnt mit `/`, bis zu 300 Zeichen, ohne Methode und Host. Geben Sie dies oder `path` an. |
| `path` | Der Pfad des Requests, für einen Router, der kein Muster kennt. Nur das daraus ermittelte Muster wird behalten. |
| `status` | Pflicht, 100 bis 599, oder `0`, wenn keine Antwort gesendet wurde. |
| `duration_us` | Pflicht, die Dauer in Mikrosekunden, bis zu 24 Stunden. |
| `request_size`, `response_size` | Bytes der Bodys. |
| `time` | Wann der Request begann, RFC 3339. Jetzt, wenn leer. |
| `consumer` | Bis zu 100 Zeichen; siehe oben. |
| `api_version`, `environment`, `service`, `release` | Bis zu 40, 40, 100 und 80 Zeichen. `environment` ist `production`, wenn leer. |
| `ip` | Die Adresse des Aufrufers, nur um sein Land zu finden. |
| `sample_rate` | 1 bis 10.000: Dieser Record steht für so viele Requests. Nur ein Status von 1 bis 399 darf gesampelt werden; ein Record mit `sample_rate` über 1 und einem Fehlerstatus wird abgelehnt. |

## Routen

Eine Route benennt einen Teil Ihrer API, nie einen einzelnen Datensatz; eine ID wird also nie zu einer eigenen Route.

- **Gesendete Routen werden bereinigt.** In einer `route` oder einem `path` wird jedes Segment, das wie eine ID
  aussieht, zu `{id}`: Zahlen, UUIDs, Datumsangaben, Hex-Strings ab 16 Zeichen, andere Token ab 16 Buchstaben,
  Ziffern, `-` und `_` mit mindestens zwei Ziffern (oder Groß- und Kleinbuchstaben und einer Ziffer),
  Werte in E-Mail-Form und alles über 40 Zeichen. Segmente, die wie ein Platzhalter des Routers beginnen (`{id}`,
  `:id`, `*`, `[id]`, `<id>`), bleiben unverändert. So zählen `/users/42` und `/users/9f8e7d6c5b4a3210` beide als
  `/users/{id}`.
- **Ein 404 oder 405 ohne Muster ist `(unmatched)`.** Ein Record, der nur einen `path` angibt und mit `404` oder
  `405` beantwortet wurde, zählt unter der Route `(unmatched)`, damit Scanner, die `/wp-login.php` anfragen, keine
  Routen erzeugen.
- **Routenregeln.** Die `path_rules` der Property bilden vor der obigen Bereinigung eine Route oder einen Pfad auf
  eine Route Ihrer Wahl ab. Eine Regel ist `{"pattern": "/legacy/**", "replacement": "/legacy"}`: `*` passt auf
  ein Segment und `**` auf beliebig viele, die erste passende Regel gewinnt, und die Ersetzung wird unverändert
  verwendet. Setzen Sie sie mit `PATCH /v1/analytics/properties/{propertyId}`.
- **Obergrenzen.** Eine Property zählt höchstens 1.000 Routen, 10.000 Consumer, 50 API-Versionen und 20
  Environments, ein Workspace 100 Services. Über einer Grenze zählen neue Werte als `(other)`, und die Antwort der
  Abfrage nennt die Grenze in `meta.caps_reached`.

### Routen auflisten und ausblenden

`GET /v1/analytics/properties/{propertyId}/routes` (Scope `analytics:read`) listet jede Route auf, die die
Property gezählt hat, nach Muster, jeweils mit `id`, ob sie aus Pfaden `inferred` (abgeleitet) wurde, ob sie
`hidden` (ausgeblendet) ist, sowie erster und letzter Sichtung. `(unmatched)` steht in der Liste, und `(other)` ab der ersten
gezählten Route, ohne Anfragen, bis eine Route darunter gezählt wird.

Eine Route, die nur Rauschen erzeugt, etwa der Health-Check, den Ihr Load Balancer aufruft, lässt sich ausblenden:

```sh
curl -X PATCH https://support.example.com/v1/analytics/properties/PROPERTY_ID/routes/ROUTE_ID \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "hidden": true }'
```

Das Ausblenden nimmt die Route ab sofort aus jeder `dataset: api`-Antwort, aus Zeilen wie aus Summen, und ein
Filter auf sie trifft nichts. Ihre Requests werden weiter gezählt und nichts wird gelöscht: `"hidden": false`
bringt sie mit allen Zahlen zurück. Ausblenden braucht den Scope `analytics:manage` und ist Owners und Admins
vorbehalten. Um einen Pfad unter einer anderen Route zu zählen, nutzen Sie eine Routenregel.

## Was erfasst wird

Erfasst wird pro Minute, Stunde und Tag: die Zahl der Requests, die mit Status 500 bis 599 oder `0` (Fehler) und
mit Status 400 bis 499 (Client-Fehler) sowie die Verteilung der Dauern, aus der p50, p95 und p99 entstehen, nach
Route, Methode, Status und Environment. Service, API-Version und Consumer werden dazu behalten.

Nie erfasst und von der API nicht angenommen: Request- oder Response-Bodys, Header, Query-Strings, Fragmente und
User-Agents. Postgres hält keine Zeile pro Request. Eine `ip` wird gekürzt (IPv4 `/24`, IPv6 `/48`), für das Land
nachgeschlagen und im selben Aufruf verworfen; keine Adresse wird gespeichert. Die Go-Middleware sendet eine Adresse
nur mit `Country: true`.

## Die Zahlen lesen

Im Panel zeigt die **Übersicht** der Property Requests, Fehlerraten und Latenz im Zeitverlauf, **Routen** die Routen
mit Traffic, Fehlern und Latenz.

Dieselben Zahlen kommen aus der Analytics-Abfrage, `POST /v1/analytics/properties/{propertyId}/query`, mit
`"dataset": "api"` (Scope `analytics:read`):

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
| Metriken | `requests`, `requests_per_minute`, `errors`, `error_rate`, `client_errors`, `client_error_rate`, `p50`, `p75`, `p90`, `p95`, `p99`, `avg_duration`, `max_duration`, `apdex`, Größen und mehr; siehe [API-Monitoring auswerten](analytics-api-analysis.md). Raten sind Prozent, Dauern Millisekunden. Die Perzentile sind auf 1,1 % genau. |
| Dimensionen und Filter | `route`, `method`, `status`, `status_class` (`2xx`, `3xx`, `4xx`, `5xx`, oder `none` für Status `0`), `environment`, `api_version`, `service`, `source`, `platform` und, mit Grenzen, `consumer`, `country` und `app_version`; höchstens zwei Dimensionen, eine davon darf eine Zeitauflösung sein. |
| Zeitauflösungen | `time:minute`, `time:hour`, `time:day`, `time:week`, `time:month`. Minuten und Stunden gelten in der Ortszeit der Property. |
| Zeitraum | `15m`, `1h`, `6h`, `24h` bis zur aktuellen Minute, oder `today`, `7d`, `30d` und die anderen Zeiträume, oder `from` und `to`, oder `since` und `until`. |

`time:minute` reicht [`YUVA_MONITOR_MINUTE_DAYS`](configuration.md#api-monitoring) Tage zurück (standardmäßig 8) und
liefert höchstens 1.440 Minuten; ein längerer oder älterer Zeitraum wird mit `422 range_too_large` abgelehnt.
Stundenwerte werden für `monitor_hour_days` der Property aufbewahrt (standardmäßig 90, 7 bis 400), Tageswerte so
lange wie ihre `retention_months`. Eine Antwort, die auf mit `sample_rate` über 1 gesendeten Records beruht, hat
`meta.estimated: true`. `dataset: api` bei einer Website-Property und eine Abfrage ohne dieses Feld bei einer
API-Property antworten mit `422 property_kind`. Eine API-Abfrage nimmt kein `goal_id`, `prop`, `segment_id`
und kein `insights`; `compare` funktioniert.

Ein Record erscheint innerhalb von etwa 15 Sekunden in einer Abfrage.

## Was zurückkommt

Die Antwort ist `202`, wenn die Anfrage gültig war, was auch mit den Records geschah.

- **`errors`** listet die als ungültig ausgelassenen Records, jeweils mit ihrem `index` in `records` und einem
  `detail`. Die übrigen werden angenommen.
- **Angenommen, aber nicht gezählt:** Records, die älter als 72 Stunden sind; Records über
  `YUVA_MONITOR_RECORDS_PER_MINUTE` für die Property oder den Schlüssel; Records über der Monatsgrenze des
  Workspace.
- **Die ganze Anfrage wird abgelehnt** bei einem Body, der die obige Form verletzt (`400`), einem fehlenden oder
  falschen Schlüssel (`401`), einem Schlüssel ohne `analytics:write` oder der Sitzung eines Mitglieds (`403`),
  einer unbekannten Property (`404`), einer Website-Property (`422`) und einer anderen Kodierung als gzip (`415`).
  Ein Server, der so viele offene Zeilen hält, wie er darf, antwortet mit `503` und `Retry-After`: Senden Sie den
  Batch danach erneut.

Die Monatsgrenze zählt die empfangenen Records, nicht die Requests, für die sie stehen: Ein gesampelter Record ist
ein Record, egal welche `sample_rate` er hat. Ein Batch, der die Grenze überschreitet, wird bis zu ihr angenommen:
die Records, die noch passen, werden gezählt, der Rest wird als über der Grenze verworfen. Erreichen die Records
eines Monats die Grenze, erhalten die Owner des Workspace einmal im Monat eine E-Mail.
`GET /v1/analytics/properties` zeigt die `requests` des Monats und das `requests_limit` unter `usage`, bei
erreichter Grenze auch `requests_limit_reached_at`.

## Bescheid bekommen, wenn etwas kaputtgeht

- [Uptime-Checks](analytics-api-uptime.md) rufen Ihren Health-Endpunkt vom Yuva-Server aus auf; so erfahren Sie von einer
  ausgefallenen API auch dann, wenn sie keine Requests mehr sendet.
- [Alerts und Berichte](analytics-api-alerts.md) überwachen Fehlerraten, Latenz, Apdex, Traffic und Zertifikate, melden
  per E-Mail, Web Push und Webhook einmal, wenn ein Problem beginnt, und einmal, wenn es endet, und senden wöchentliche
  und monatliche Berichte. Eine neue API-Property startet mit einem Alert für 5xx-Fehler.

## Die Zeitzone ändern

Eine Änderung der `timezone` einer API-Property (`PATCH /v1/analytics/properties/{propertyId}`) schneidet keinen
Tag entzwei. Die Property zählt bis zur nächsten lokalen Mitternacht der alten Zone weiter in dieser und danach in
der neuen. Bis dahin trägt sie `timezone_applies_at`, den Zeitpunkt, ab dem die neue Zone gilt. Bereits gezählte
Tage behalten die Zone, in der sie gezählt wurden.

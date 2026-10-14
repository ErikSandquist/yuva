# Alerts und Berichte für Ihre API

Alert-Regeln überwachen eine [API-Property](analytics-api.md) und ihre [Uptime-Checks](analytics-api-uptime.md) und
sagen Ihnen, wenn etwas schiefgeht und wenn es vorbei ist. Jedes Problem löst einen Alert aus, wenn es beginnt, und
eine Nachricht, wenn es endet, nicht jede Minute eine neue. Wöchentliche und monatliche Berichte fassen den Zeitraum
zusammen.

## Wie ein Alert funktioniert

Der Server wendet jede Regel einmal pro Minute auf die abgeschlossenen Minuten an. Pro Regel und Gegenstand hält er
höchstens einen offenen Alert:

1. Gilt die Bedingung, wird der Alert einmal **ausgelöst**.
2. Solange er offen bleibt, wird nichts weiter gesendet.
3. Lag der Wert mit Abstand ein ganzes Fenster lang wieder innerhalb des Schwellenwerts, wird der Alert einmal
   **aufgelöst**.

Der Abstand beträgt 10 %: Eine Regel für eine Fehlerrate über 5 % löst sich auf, wenn die Rate das ganze Fenster lang
unter 4,5 % blieb. Bei Apdex, der hoch bleiben soll, ist der Abstand ein Zehntel der Entfernung zu 1: Eine Regel für
Apdex unter 0,8 löst sich über 0,82 auf. Spike und Drop haben eigene Abstände, siehe unten.

Ein Alert löst sich auch ohne Fensterwartezeit, wenn:

- das Fenster weniger Requests enthält als das `min_requests` der Regel, weil nichts mehr zu bewerten ist,
- eine stille Route wieder aufgerufen wurde,
- der Check wieder erreichbar ist oder sein Zertifikat erneuert wurde,
- der Check pausiert oder die Regel deaktiviert wird (eine deaktivierte Regel löst ihre offenen Alerts auf, ohne
  jemandem Bescheid zu geben).

### Regelarten

Regeln auf die Requests einer Property brauchen `property_id`. `route` engt eine Regel auf eine Route ein (ein
Routenmuster, wie `GET /v1/analytics/properties/{propertyId}/routes` es auflistet), `service` auf einen Service; ohne
beides überwacht die Regel die ganze Property. Bewertet werden sie über die letzten `window_minutes` abgeschlossenen
Minuten (5 bis 60, standardmäßig 5) und nur, wenn das Fenster mindestens `min_requests` Requests hat (standardmäßig 20).

| `kind` | Löst aus, wenn | `threshold` |
|---|---|---|
| `error_rate` | Der Anteil der Requests mit Status 500 bis 599 (oder ohne Antwort) über dem Schwellenwert liegt. | Prozent. |
| `client_error_rate` | Der Anteil mit Status 400 bis 499 über dem Schwellenwert liegt. | Prozent. |
| `p95`, `p99` | Dieses Perzentil der Dauer über dem Schwellenwert liegt. | Millisekunden. |
| `apdex` | Der [Apdex](analytics-api-analysis.md#apdex)-Wert unter dem Schwellenwert liegt. | 0 bis 1. |
| `spike` | Die Requests der letzten 15 abgeschlossenen Minuten mindestens das Dreifache derselben 15 Minuten im Mittel der 7 Tage davor sind. | Keiner. |
| `drop` | Sie höchstens die Hälfte dieses Mittels sind. | Keiner. |
| `silent` | Eine Route, die am Vortag im Mittel mindestens einen Request pro Minute hatte, seit 15 Minuten keinen hatte. | Keiner. |

`spike` und `drop` werden nur bewertet, wenn das 7-Tage-Mittel mindestens 50 Requests beträgt, und haben kein eigenes
Fenster. Ein Spike löst sich auf, wenn die 15 Minuten wieder unter dem 2,7-Fachen des Mittels liegen, ein Drop, wenn
sie wieder über dem 0,55-Fachen liegen. Eine `silent`-Regel mit `route` überwacht diese Route; ohne überwacht sie jede
Route der Property.

Regeln auf Uptime-Checks nennen `check_id`, oder `property_id` für jeden Check dieser Property, oder keines von beiden
für jeden Check des Workspace:

| `kind` | Löst aus, wenn | `threshold` |
|---|---|---|
| `check_state` | Der Check ausgefallen ist. | Keiner. |
| `certificate` | Das Zertifikat des Checks innerhalb des Schwellenwerts abläuft. | Tage; ohne Angabe das `cert_warning_days` des Checks. |

### Standardregeln

Eine neue API-Property erhält eine Regel: die 5xx-Rate über 5 % in 5 Minuten, gesendet an das Mitglied, das sie
angelegt hat. Ein Mitglied, das einen Uptime-Check anlegt, erhält eine `check_state`-Regel dafür, gesendet an es.
Bearbeiten oder löschen Sie sie wie jede andere Regel.

## Eine Regel anlegen

Owner und Admins sowie API-Schlüssel ohne Posteingangs-Beschränkung verwalten Regeln mit dem Scope `analytics:manage`:

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

| Feld | Bedeutung |
|---|---|
| `kind` | Eine der obigen Arten. Nach dem Anlegen unveränderlich, wie der Gegenstand der Regel (`property_id`, `route`, `service`, `check_id`). |
| `name` | 1 bis 100 Zeichen; ohne Angabe nach Art und Gegenstand benannt. |
| `threshold`, `window_minutes`, `min_requests` | Wie oben. `min_requests` ist 1 bis 1.000.000. |
| `recipients` | Bis zu 20 Mitglieder, die jeden Alert per E-Mail und Web Push erhalten. Sie müssen die Property der Regel sehen können. |
| `enabled` | `false` pausiert die Regel. |

Ein Workspace hat höchstens 200 Regeln (`403 rule_limit`). `PATCH /v1/monitoring/alert-rules/{ruleId}` ändert den Rest
und gilt ab der nächsten Minute, auch für offene Alerts. `DELETE` entfernt die Regel und ihre Alerts; deren Annotationen
bleiben. Mitglieder lesen Regeln und Alerts mit `analytics:read`; Regeln einer Property folgen der Sichtbarkeit der
Property.

## Wohin ein Alert geht

Auslösen und Auflösen tun jeweils all dies:

- **E-Mail** an die `recipients` der Regel, in der Sprache des jeweiligen Mitglieds.
- **Web Push** an dieselben Mitglieder, als dringende Benachrichtigung.
- **Webhook** `monitor.alert` an die Endpunkte, die ihn abonniert haben ([Webhooks](webhooks.md)). `data.alert.state` ist
  `firing` oder `resolved`. Ein auf einen Posteingang beschränkter Endpunkt erhält die Alerts der mit diesem Posteingang
  verknüpften Properties.
- **Annotation** in den Diagrammen der Property, damit der Zeitpunkt neben den Zahlen steht. Der Alert eines Checks ohne
  Property annotiert, falls vorhanden, die Property des Checks.
- **Echtzeit**-Nachricht `monitor.alert` an verbundene Mitglieder und Agenten.

`analytics.alert`-Webhooks und die Alerts im Berichts-Abo einer Website sind getrennt und unverändert. Eine API-Property
nimmt in ihrem Abo keine `alerts` an; ihre Alerts sind diese Regeln.

## Alerts lesen

```sh
curl "https://support.example.com/v1/monitoring/alerts?state=firing" \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

`GET /v1/monitoring/alerts` listet Alerts, neueste zuerst, bis zu `limit` (standardmäßig 50, höchstens 100), gefiltert
nach `state` (`firing` oder `resolved`), `property_id`, `check_id` oder `rule_id`, mit `before` für die nächste Seite.
Aufgelöste Alerts werden 400 Tage aufbewahrt. Ein Alert hat:

| Feld | Bedeutung |
|---|---|
| `rule_id`, `rule_name`, `kind` | Die Regel, die ihn ausgelöst hat. |
| `key` | Worum es innerhalb der Regel geht: leer für den ganzen Gegenstand der Regel, das Routenmuster bei `silent`, die Check-ID bei Check-Regeln. |
| `state` | `firing` oder `resolved`. |
| `value`, `threshold` | Der Wert beim Auslösen und der Schwellenwert der Regel. Bei `spike` und `drop` sind es die Requests der 15 Minuten, bei `certificate` die verbleibenden Tage, bei `check_state` 1. |
| `baseline` | `spike` und `drop`: das Mittel, mit dem verglichen wurde. `silent`: Requests pro Minute am Vortag. |
| `resolved_value`, `resolved_at` | Wann er sich aufgelöst hat. |
| `fired_at` | Das Ende des Fensters, das ihn ausgelöst hat. |
| `text` | Was gefunden wurde, auf Englisch, wie in der Annotation. |

`GET /v1/monitoring/alert-rules` listet die Regeln, jede mit `firing`, der Zahl ihrer offenen Alerts.

## Wöchentliche und monatliche Berichte

Eine API-Property sendet den Mitgliedern, die einen wollen, einen eigenen Bericht, mit denselben Einstellungen wie bei
einer Website ([E-Mail-Berichte](analytics-support.md#e-mail-berichte-und-alert-mails)):

```sh
curl -X PUT https://support.example.com/v1/analytics/properties/PROPERTY_ID/subscription \
  -H "Content-Type: application/json" \
  -d '{ "report": "weekly" }'
```

Dieser Aufruf braucht die Sitzung eines Mitglieds, keinen API-Schlüssel. Wöchentliche Berichte gehen montags um 08:00 in
der Zeitzone der Property hinaus und decken die Vorwoche ab; monatliche gehen am 1. hinaus und decken den Vormonat ab.
Der Bericht enthält die Requests, die Fehler- und Client-Fehlerraten und p95 im Vergleich zum Zeitraum davor, die
langsamsten und fehlerreichsten Routen, die Uptime der Checks der Property und die Alerts des Zeitraums. Er hat keine
Bilder und nichts, was Öffnen oder Klicken verfolgt. `POST …/subscription/send-report` sendet sofort einen.

## Von einem Assistenten

Die Tools `monitoring_alerts` und `uptime_checks` antworten aus denselben Zahlen; siehe
[API-Monitoring über einen Assistenten](mcp.md#api-monitoring-über-einen-assistenten).

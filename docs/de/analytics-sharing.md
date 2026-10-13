# Analytics teilen und exportieren

Ein Freigabelink zeigt Menschen ohne Yuva-Konto einen Teil der Zahlen einer Website, nur lesend, und kann in einem
iframe auf Ihren eigenen Seiten stehen. Ein Export schreibt die Zahlen oder die Rohereignisse einer Website in eine
Datei, die Sie herunterladen. Beides gehört zu einer Website von [Yuva Analytics](analytics.md); die API nennt sie
Property.

## Freigabelinks

Ein Freigabelink ist eine Webadresse, `<Ihre Yuva-Adresse>/share/<Token>`, die einen schreibgeschützten Bericht
einer Website ohne Anmeldung öffnet. Wer die Adresse hat, sieht, was der Link erlaubt; geben Sie sie so sorgfältig
weiter, wie die Zahlen es verdienen.

Öffnen Sie im Panel die Website, öffnen Sie den Dialog Teilen und legen Sie fest:

| Einstellung | Bedeutung |
|---|---|
| Name | Wofür der Link ist. Nur Mitglieder sehen ihn. |
| Ansichten | Mindestens eine von `overview`, `realtime`, `pages`, `sources`, `locations`, `devices`, `events`, `goals`. Das sind die Tabs der [Ansichten](analytics.md#die-ansichten); ein Tab, der nicht aufgeführt ist, fehlt für den Betrachter. |
| Passwort | Optional, 8 bis 200 Zeichen. Der Betrachter gibt es ein, bevor er etwas sieht. |
| Läuft ab | Optional. Danach funktioniert der Link nicht mehr. |
| Einbetten auf | Optional: Liste der Sites, die den Link in einem iframe zeigen dürfen ([unten](#einen-freigabelink-einbetten)). |

Adresse und Token (`yuva_sh_…`) werden **einmal** angezeigt, wenn der Link entsteht. Yuva speichert nur einen Hash
des Tokens und des Passworts, deshalb lässt sich keines von beiden wieder anzeigen; haben Sie den Link verloren,
erstellen Sie einen neuen. Die Liste führt die ersten Zeichen des Tokens, damit Sie Links unterscheiden können, und
wann jeder Link zuletzt geöffnet wurde.

Um einen Link zu stoppen, **widerrufen** Sie ihn: Er funktioniert sofort nicht mehr und bleibt in der Liste.
**Löschen** entfernt ihn auch aus der Liste. Ein neues oder entferntes Passwort beendet den Zugriff aller, die den
Link schon entsperrt hatten. Mit dem Ablauf ist es genauso: Nach `expires_at` antwortet die Adresse mit `404`.

Wer was darf:

- Owner, Admins und API-Keys ohne Posteingangsbeschränkung mit `analytics:manage` erstellen, ändern, widerrufen und
  löschen Links.
- Wer die Website lesen darf (`analytics:read`), sieht die Liste.
- Eine Website hat höchstens 50 Links (`403 share_limit`).

### Was eine Freigabe zeigt

Ein Link zeigt die Zahlen seiner Ansichten, nur für diese Website. Summen und Zeitraster kommen mit jeder Ansicht.
Aufschlüsseln oder Filtern erfordert die Ansicht, zu der die Spalte gehört:

| Ansicht | Fügt hinzu |
|---|---|
| `pages` | `page`, `entry_page`, `exit_page`, `hostname` |
| `sources` | `channel`, `source`, `referrer` und die `utm_*`-Spalten |
| `locations` | `country`, `region`, `city` |
| `devices` | `device`, `browser`, `browser_version`, `os`, `os_version`, `screen_size`, `language`, `platform`, `app_version` |
| `events` | `event`, `prop` und die Events-Metrik |
| `goals` | die Goals der Website und ihre Metriken |

Eine Abfrage außerhalb der Ansichten des Links wird abgelehnt (`403 view_not_shared`). Eine Freigabe zeigt **nie**
gespeicherte Segmente (eine `segment_id` wird abgelehnt), andere Websites, die Einstellungen, die Datenwerkzeuge,
die Mitglieder oder einen Export.

Betrachter nutzen dieselben Abfrage-Slots und denselben Cache wie das Panel; die Freigabe-Endpunkte sind pro Adresse
begrenzt: 120 Anfragen pro Minute und 10 Passwortversuche pro Minute und Link (30 insgesamt). Wer einen Link
entsperrt, hat 12 Stunden Zugriff.

### Einen Freigabelink einbetten

Setzen Sie den Link mit einem iframe auf Ihre eigene Seite:

```html
<iframe
  src="https://support.example.com/share/yuva_sh_xxxxxxxx"
  title="Website statistics"
  width="100%"
  height="800"
  style="border: 0"
  loading="lazy"
></iframe>
```

Tragen Sie die Site der Seite unter **Einbetten auf** ein: `https://www.example.com` für eine Site,
`https://*.example.com` für alle ihre Subdomains. Bis zu 20 Einträge, jeder mit `https://` oder `http://` und einem
Host (und Port), danach nichts.

Ein Browser zeigt die Freigabeseite nur innerhalb einer Seite, deren Site in der Liste steht. Jede andere Site und
jeder Link mit leerer Liste bekommt eine Seite, die das Einrahmen verbietet (`frame-ancestors 'none'`); niemand kann
Ihren Bericht in einen Frame auf seiner eigenen Site setzen. Dasselbe gilt für einen unbekannten, abgelaufenen oder
widerrufenen Link. Die Adresse selbst öffnet sich weiterhin in einem Browser-Tab für jeden, der sie hat; die Liste
entscheidet nur, wer sie einrahmen darf. Die Freigabeseite enthält kein Analytics-Skript, und der Token bleibt aus
den Anfrage-Logs des Servers heraus.

## Exporte

Ein Export schreibt Zahlen im Hintergrund in eine Datei auf dem Server. Starten Sie ihn im Panel unter Daten und
dann Export oder über die [API](#api). Bis die Datei fertig ist, wird der Fortschritt angezeigt; laden Sie sie von
derselben Stelle herunter. Eine Datei wird **7 Tage** nach ihrer Erstellung gelöscht, früher können Sie sie selbst
löschen. Eine Datei über 1 GiB schlägt mit `too_large` fehl, und ein Workspace führt höchstens drei Exporte
gleichzeitig aus (`429 rate_limited`).

| Art | Inhalt | Formate | Zeitraum |
|---|---|---|---|
| `view` | Jede Zeile einer Abfrage, die Tabelle, die Sie im Panel sehen würden (ohne Vergleich). | CSV, NDJSON | Der der Abfrage, höchstens 400 Tage |
| `daily` | Eine Zeile pro Tag mit Besuchern, Besuchen, Seitenaufrufen, Aufrufen pro Besuch, Absprungrate, Besuchsdauer, Zeit auf der Seite und Scrolltiefe, optional nach einer Dimension aufgeteilt. | CSV, NDJSON | Höchstens 400 Tage |
| `raw` | Jedes gespeicherte Ereignis mit der Beschreibung seines Besuchs. | Nur NDJSON | Höchstens 31 Tage, alle innerhalb des Rohdaten-Fensters |

`view` und `daily` brauchen `analytics:read`; wer die Website lesen darf, kann also ihre Zahlen exportieren.

### Rohexporte

Ein `raw`-Export ist für Datenpipelines und eigene Analysen. Er braucht `analytics:manage` und einen Aufrufer, der
Owner, Admin oder ein API-Key ohne Posteingangsbeschränkung ist. Rohzeilen sind pseudonyme personenbezogene Daten;
deshalb beschränkt Yuva sie auf die Menschen, die den Workspace führen: Ein einfacher Leser kann keinen erstellen,
auflisten oder herunterladen.

Der Zeitraum muss innerhalb der Tage liegen, für die Detaildaten aufbewahrt werden
([`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analytics), standardmäßig 90); ein älterer Tag antwortet mit
`422 needs_raw_window`. Bots bleiben außen vor.

Eine Zeile pro Ereignis:

```json
{"day":"2026-10-07","time":"2026-10-07T09:14:31Z","visit":3,"type":"pageview","hostname":"www.example.com","path":"/pricing","channel":"Organic search","source":"Google","referrer":"www.google.com","country":"DE","city":"Berlin","device":"Desktop","browser":"Firefox","os":"Linux","screen_size":"Desktop","language":"de","platform":"web"}
```

| Feld | Bedeutung |
|---|---|
| `day`, `time` | Der Tag in der Zeitzone der Website und die Zeit des Ereignisses. |
| `visit` | Eine Zahl, die in dieser Datei bei 1 beginnt und die Ereignisse eines Besuchs verbindet. |
| `type` | `pageview`, `screen`, `event` oder `engagement`. |
| `name`, `props`, `revenue_minor`, `currency` | Name, Props und Umsatz eines eigenen Events (in der kleinsten Einheit der Währung). |
| `engaged_ms`, `scroll` | Bei `engagement`: Millisekunden im Blick, tiefste Scrollposition in Prozent. |
| `hostname`, `path` | Die Seite, in einer App der Screen. |
| `channel`, `source`, `referrer`, `utm_*` | Woher der Besuch kam, wie sein erstes Ereignis es beschrieb. Der Referrer ist ein Host, nie ein Pfad. |
| `country`, `region`, `city`, `device`, `browser`, `browser_version`, `os`, `os_version`, `screen_size`, `language`, `platform`, `app_version` | Ort und Gerät des Besuchs, wie sein erstes Ereignis sie beschrieb. |

Felder, die nicht zutreffen, fehlen in der Zeile.

Eine Rohdatei enthält **nie** einen Besucher-Hash, das Tagessalz, eine IP-Adresse, einen Kontakt, eine gespeicherte
Besuchs-ID oder den Pfad eines Referrers. `visit` wird für jede Datei neu nummeriert; zwei Dateien lassen sich
darüber nicht verbinden, und es sagt nichts über den Besucher. Kombinieren Sie eine Rohdatei nicht mit anderen Daten,
um Personen wiederzuerkennen; geben Sie sie weiter, behandeln Sie sie nach Ihrer eigenen
[Datenschutzerklärung](analytics-privacy.md) als personenbezogene Daten.

### CSV und Tabellenkalkulationen

Eine CSV-Zelle, die mit `=`, `+`, `-`, `@`, einem Tabulator oder einem Wagenrücklauf beginnt, bekommt ein
vorangestelltes `'`. Seitenpfade, Referrer und Eventnamen stammen von Besuchern, und eine Zelle wie
`=HYPERLINK(...)` würde sonst beim Öffnen in einer Tabellenkalkulation als Formel laufen. Die Folge: Ein Wert wie
`-5` oder `+49 30` erscheint als `'-5` oder `'+49 30`. Ein Pfad wie `/pricing` bleibt unverändert. Entfernen Sie das
führende `'`, wenn Sie die Datei mit einem Programm lesen; brauchen Sie die Werte genau wie gespeichert, nutzen Sie
NDJSON.

## API

Aufrufe senden `Authorization: Bearer $YUVA_API_KEY`. Setzen Sie `PROPERTY` auf die ID der Website.

| Aufruf | Tut |
|---|---|
| `GET /v1/analytics/properties/{propertyId}/shares` | Listet die Freigabelinks, auch widerrufene und abgelaufene (`analytics:read`). |
| `POST /v1/analytics/properties/{propertyId}/shares` | Erstellt einen Link. Die Antwort trägt `token` und `url` einmalig. |
| `PATCH /v1/analytics/properties/{propertyId}/shares/{shareId}` | Ändert Name, Ansichten, Passwort (`null` entfernt es), Einbettungs-Origins oder Ablauf (`null` behält den Link bis zum Widerruf). Ein widerrufener Link antwortet mit `409 share_revoked`. |
| `POST /v1/analytics/properties/{propertyId}/shares/{shareId}/revoke` | Widerruft einen Link. |
| `DELETE /v1/analytics/properties/{propertyId}/shares/{shareId}` | Löscht einen Link. |
| `GET /client/v1/analytics-shares/{token}` | Was die Freigabeseite zuerst liest. Kein Konto nötig. |
| `POST /client/v1/analytics-shares/{token}/access` | Entsperrt einen Link mit seinem Passwort und antwortet mit einem `yuva_sa_…`-Token für 12 Stunden. |
| `POST /client/v1/analytics-shares/{token}/query` | Die Zahlen, begrenzt auf die Ansichten des Links. |
| `GET /client/v1/analytics-shares/{token}/realtime` | Die letzten 30 Minuten, mit der Ansicht `realtime`. |
| `GET /v1/analytics/properties/{propertyId}/exports` | Listet die Exporte bis zu ihrem Ablauf (`analytics:read`). |
| `POST /v1/analytics/properties/{propertyId}/exports` | Startet einen Export. Antwortet `202`. |
| `GET`, `DELETE /v1/analytics/properties/{propertyId}/exports/{exportId}` | Liest einen Export und seinen Fortschritt oder löscht ihn samt Datei. |
| `GET /v1/analytics/properties/{propertyId}/exports/{exportId}/download` | Lädt die Datei herunter. |

Einen Link für Übersicht und Seiten erstellen, mit Passwort, für eine Site, mit Ablaufdatum:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/shares \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{
    "name": "Board report",
    "views": ["overview", "pages"],
    "password": "a long password",
    "embed_origins": ["https://www.example.com"],
    "expires_at": "2027-01-01T00:00:00Z"
  }'
```

Widerrufen:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/shares/$SHARE/revoke \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

Das Skript eines Betrachters entsperrt einen Link und fragt ihn ab:

```sh
TOKEN=$(curl -s -X POST https://support.example.com/client/v1/analytics-shares/yuva_sh_xxxxxxxx/access \
  -H "Content-Type: application/json" -d '{"password": "a long password"}' | jq -r .access_token)

curl -X POST https://support.example.com/client/v1/analytics-shares/yuva_sh_xxxxxxxx/query \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"metrics": ["visitors"], "dimensions": ["time:day"], "range": {"period": "7d"}}'
```

Einen Monat Tageszahlen nach Land exportieren, warten und herunterladen:

```sh
EXPORT=$(curl -s -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/exports \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "daily", "format": "csv", "range": {"period": "30d"}, "dimension": "country"}' | jq -r .id)

curl -s https://support.example.com/v1/analytics/properties/$PROPERTY/exports/$EXPORT \
  -H "Authorization: Bearer $YUVA_API_KEY" | jq '{status, progress, rows}'

curl -o daily.csv https://support.example.com/v1/analytics/properties/$PROPERTY/exports/$EXPORT/download \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

Ein Rohexport einer Woche, mit einem Key mit `analytics:manage`:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/exports \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "raw", "format": "ndjson", "range": {"from": "2026-10-01", "to": "2026-10-07"}}'
```

Der Download antwortet mit `409 export_not_ready`, bis `status` `done` ist, und mit `404`, wenn die Datei abgelaufen
ist. Ein Link, den Sie anklicken, kann keine Header senden; deshalb akzeptiert der Download auch
`?workspace_id=<id>`. Alle Felder stehen im [API-Vertrag](../openapi/openapi.yaml).

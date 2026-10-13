# Serverseitige Events

Manche Dinge sieht nur Ihr Backend: ein angelegtes Konto, eine durchgegangene Zahlung, ein verlängertes Abo.
Ihr Server kann sie an eine Analytics-Property senden, und ein Event mit Adresse und Browser des Besuchers
gehört zum Besuch dieses Besuchers auf Ihrer Website. Diese Seite baut auf [Analytics](analytics.md) und den
Event-Regeln in [Events, Goals und Funnels](analytics-events.md) auf.

## Einen Schlüssel anlegen

Serverevents brauchen einen API-Schlüssel; der öffentliche Schlüssel der Website (`yuva_ak_…`) funktioniert
dafür nicht. Owner und Admins legen unter **Settings → API keys** einen Schlüssel mit dem Scope
`analytics:write` an und behalten ihn auf dem Server. Ein auf Posteingänge (Inboxes) beschränkter Schlüssel
erreicht die Properties dieser Posteingänge. Die Sitzung eines Mitglieds wird abgelehnt (`403 forbidden`).

Außerdem brauchen Sie die ID der Property. Die Tabs **Go** und **curl** unter **Settings → Install** haben die
Snippets unten damit ausgefüllt. Für Events aus Ihrem Backend muss in den Einstellungen der Property keine
Plattform angehakt sein.

## Events senden

```sh
curl https://support.example.com/v1/analytics/properties/PROPERTY_ID/events \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "events": [{
      "name": "Signup",
      "url": "https://www.example.com/signup",
      "ip": "203.0.113.7",
      "user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
      "props": { "plan": "pro" }
    }]
  }'
```

```json
{ "accepted": 1, "errors": [] }
```

`POST /v1/analytics/properties/{propertyId}/events` nimmt 1 bis 100 Events und antwortet mit `202`.

| Feld | |
|---|---|
| `name` | Pflicht, 1 bis 120 Zeichen: `pageview`, ein eingebautes Event oder der Name Ihres eigenen Events. Es gelten die Regeln der [Events](analytics-events.md#events-senden); `engagement` ist nur für Browser. |
| `url` | Pflicht, bis zu 2.000 Zeichen. Eine Seite auf einer der Domains der Property oder, bei einer Property mit iOS oder Android, ein App-Screen, `app://<App-ID>/<Screen>`. Nur Host und Pfad werden gespeichert. |
| `referrer` | Der Referrer der Seite, für einen Seitenaufruf, der einen Besuch beginnt. |
| `timestamp` | Wann es geschah, RFC 3339. Ohne Angabe jetzt. |
| `props` | Bis zu 30 Text-, Zahl- oder Boolean-Werte, mit den [Regeln für Props](analytics-events.md#props). |
| `revenue` | `{"a": "29.90", "c": "EUR"}`, siehe [Umsatz](analytics-events.md#umsatz). |
| `interactive` | `false` macht ein eigenes Event nicht interaktiv. |
| `ip`, `user_agent`, `sec_ch_ua_platform`, `sec_ch_ua_mobile`, `accept_language` | Die Anfrage des Besuchers, [weitergereicht](#zum-besuch-des-besuchers-gehören). |
| `visitor_key` | Ihr eigener Schlüssel für den Besucher, statt `ip` und `user_agent`. |
| `dnt`, `gpc` | Die Anfrage des Besuchers hatte `DNT: 1` oder `Sec-GPC: 1`. Das Event wird nicht gespeichert. |
| `consent` | Der Besucher hat Analytics zugestimmt. |

## Zum Besuch des Besuchers gehören

Reichen Sie weiter, was Ihr Server vom Besucher erhalten hat, nicht die eigene Adresse Ihres Servers. Yuva
kürzt die `ip` bei der Ankunft, hasht sie mit dem `user_agent` genau so, wie es die eigene Anfrage des
Browsers an die Website hasht, und vergisst beides. Das Event gehört dann zum Besuch des Besuchers an diesem
Tag: Die Anmeldung erscheint im Funnel nach den Seitenaufrufen, die zu ihr führten. Fügen Sie
`sec_ch_ua_platform` und `sec_ch_ua_mobile` (die `Sec-CH-UA-*`-Header, wie gesendet) und `accept_language`
hinzu, wenn die Anfrage sie hatte. Die Adresse dient auch dem Ort.

Für einen Besucher, zu dem Sie keine Browser-Anfrage zur Hand haben, etwa bei einem Webhook oder
Hintergrundjob, geben Sie stattdessen einen `visitor_key` an: Ihr eigener Schlüssel für ihn, etwa eine
Sitzungs-ID, **niemals eine E-Mail-Adresse**. Er wird unter dem Salt des Tages gehasht, nie gespeichert und
bildet einen eigenen Besucher, getrennt von dem des Browsers. Mit ihm ordnet `ip` den Besucher nur örtlich ein,
und `user_agent` beschreibt nur das Gerät.

Ein Besuch, den ein Event beginnt, hat die Plattform **Server**.

## Einwilligung und Do Not Track

Beachten Sie die Entscheidungen des Besuchers, bevor Sie senden:

- Für einen Besucher, dessen Anfrage `DNT: 1` oder `Sec-GPC: 1` hatte, sollte gar nicht gesendet werden. Senden
  Sie das Event dennoch mit `dnt: true` oder `gpc: true`, nimmt Yuva es an, speichert es aber nicht.
- Eine Property mit **Collection** **Only with consent** speichert ein Event nur mit `consent: true`.

## Was zurückkommt

Die Antwort ist `202`, wenn die Anfrage gültig war, egal was mit den Events geschah.

- **`errors`** listet die als ungültig ausgelassenen Events, jedes mit seinem `index` in `events` und einem
  `detail`, das den Grund nennt. Die übrigen werden angenommen.
- **Angenommen, aber nicht gespeichert** und unter **Settings → Health** gezählt: Events mit `dnt` oder `gpc`;
  Events ohne Einwilligung bei einer Consent-Property; Bots, ausgeschlossene Adressen und Pfade; Events über
  dem Limit pro Adresse, pro Property oder dem Monatslimit; Events, die älter als 72 Stunden sind (**Too late**).
- **Die ganze Anfrage wird abgelehnt** bei einem Body, der kein JSON ist oder die obige Form bricht (`400`),
  einem fehlenden oder falschen Schlüssel (`401`), einem Schlüssel ohne `analytics:write` oder der Sitzung
  eines Mitglieds (`403`) und einer unbekannten Property (`404`). Ein ausgelasteter Server antwortet mit `503`
  und `Retry-After`: Senden Sie die Anfrage danach erneut.

**Settings → Test events** zeigt, was der Server zählt und verwirft, während Sie es ausprobieren.

## Go

```sh
go get github.com/productdevbook/yuva/sdk/go/analytics
```

```go
import "github.com/productdevbook/yuva/sdk/go/analytics"

yuva, err := analytics.New(analytics.Config{
	BaseURL:    "https://support.example.com",
	APIKey:     os.Getenv("YUVA_API_KEY"),
	PropertyID: "PROPERTY_ID",
})
if err != nil {
	log.Fatal(err)
}
defer yuva.Close(context.Background())

// in a handler: the visitor's address and browser come from r
yuva.Track(r, analytics.Event{
	Name:  "Signup",
	Props: map[string]any{"plan": "pro"},
})
```

`Track` reicht Adresse, `User-Agent`, `Sec-CH-UA-Platform`, `Sec-CH-UA-Mobile` und `Accept-Language` von `r`
weiter und nimmt `URL` und Referrer aus `r`, wenn das Event sie leer lässt. Diese URL muss auf einer der
Domains der Property liegen; hinter einem Proxy oder einem internen Hostnamen setzen Sie `URL` deshalb selbst.
Eine Anfrage mit `DNT: 1` oder `Sec-GPC: 1` sendet nichts, und `Track` gibt `nil` zurück. Setzen Sie
`Consent: true` am Event, wenn der Besucher zugestimmt hat. Ist `VisitorKey` gesetzt, reicht `Track` weder die
Adresse noch den User-Agent weiter; setzen Sie `IP` und `UserAgent` zusätzlich am Event, wenn Sie Ort und
Gerät wollen.

`Send` reiht ein Event so ein, wie es ist, für einen Hintergrundjob ohne Anfrage.

| `Config` | Standard | |
|---|---|---|
| `HTTPClient` | `http.DefaultClient` | Sendet die Anfragen. |
| `FlushInterval` | 5 Sekunden | Wie oft eingereihte Events gesendet werden. Ein voller Batch von 100 geht sofort. |
| `QueueSize` | 10.000 | Wie viele Events höchstens warten; weitere werden mit `ErrQueueFull` verworfen. |
| `MaxRetries` | 5 | Wiederholungen eines Batches nach Netzwerkfehler, `429` oder `5xx`; negativ: keine. |
| `MinBackoff` | 1 Sekunde | Die Verzögerung der ersten Wiederholung, verdoppelt bis eine Minute. Ein längeres `Retry-After` gilt. |
| `ClientIP` | Host von `r.RemoteAddr` | Die Adresse des Besuchers für `Track`. Hinter einem Proxy geben Sie die Adresse zurück, die er meldet. |
| `OnError` | keiner | Wird aufgerufen mit einem `*EventError` für ein Event, das Yuva als ungültig ausgelassen hat, einem `*BatchError` für einen Batch, den es abgelehnt hat (`400`, `401`, `403`, `404`) oder der nach allen Wiederholungen scheiterte, und mit `ErrQueueFull`. |

Events werden im Hintergrund in Batches gesendet. `Flush(ctx)` sendet, was eingereiht ist, sofort, und
`Close(ctx)` sendet es und stoppt den Client: Rufen Sie es vor dem Prozessende auf, sonst gehen die wartenden
Events verloren.

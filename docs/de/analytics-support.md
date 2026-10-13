# Support und Traffic

Verknüpfen Sie eine Website von [Yuva Analytics](analytics.md) mit einem Posteingang, und ihre Seiten zeigen, wie
viel Support sie auslösen: Unterhaltungen, Seiten-Feedback, Bewertungen und Diskussionen, gezählt pro Seite und Tag
neben den Aufrufen der Seite. Dieselben Zahlen speisen Insights, Alerts, wöchentliche und monatliche E-Mail-Berichte
und die Analytics-Tools von [KI-Assistenten](mcp.md#analytics-über-einen-assistenten). Yuva lässt dafür kein Modell
laufen: Jedes Insight und jeder Alert ist eine feste Regel, die Sie unten nachlesen können.

## Was verknüpft wird

Wählen Sie den Posteingang in den Einstellungen der Website (**Posteingang**,
[Eine Website hinzufügen](analytics.md#eine-website-hinzufügen)). Ab dann zählt Yuva **pro Seite und Tag**, was dieser
Posteingang von den Seiten der Website erhalten hat:

| Zahl | Zählt |
|---|---|
| `conversations` | Unterhaltungen, die auf der Seite mit dem [Chat-Widget](widget.md) begonnen haben, am Tag ihres Beginns. |
| `feedback` | Feedback, das von der Seite mit [`<yuva-page-feedback>`](documentation-pages.md) gesendet wurde. |
| `ratings_up`, `ratings_down` | Die Bewertungen „War diese Seite hilfreich?“ der Seite, an dem Tag, an dem sie abgegeben wurden (UTC). |
| `helpful_rate` | Prozent der Bewertungen, die positiv sind. |
| `discussions` | Auf der Seite begonnene Diskussionsthreads. |
| `support_rate` | Unterhaltungen pro 1.000 Seitenaufrufe der Seite. |

Sie stehen neben `visitors` und `pageviews` derselben Seite. Seiten werden nach den Pfadregeln der Website über Host
und Pfad zugeordnet, `/orders/:id` ist also auch hier eine Seite.

**Was nie verknüpft wird:** Kein Besucher von Analytics wird je mit einem Kontakt oder einer Unterhaltung verbunden.
Beide Seiten treffen sich nur an der Seite. Damit die Seite bekannt ist, sendet das Chat-Widget die Adresse der Seite,
auf der eine Unterhaltung beginnt, **ohne Query-String und Fragment**, und den Seitentitel, sonst nichts. Eine
Unterhaltung zeigt die Seite, auf der sie begann; die Zahlen sagen nicht, wer gelesen hat. Analytics selbst speichert
weiterhin weder Cookie noch Adresse ([Analytics und Datenschutz](analytics-privacy.md)).

Eine Website ohne verknüpften Posteingang hat keine Support-Zahlen; eine Support-Abfrage antwortet mit
`422 no_linked_inbox`.

## Support-Rate

Die Support-Rate ist **Unterhaltungen pro 1.000 Seitenaufrufe**: 12 Unterhaltungen auf einer Seite mit 3.000 Aufrufen
sind 4. Es ist ein Verhältnis, eine stark besuchte Seite gewinnt also nicht schon dadurch, dass viel los ist. Eine
Seite mit weniger als 100 Aufrufen im Zeitraum steht hinter den anderen, weil eine Rate über eine Handvoll Aufrufe
wenig aussagt.

Öffnen Sie im Panel die Support-Ansicht der Website, um die Seiten danach geordnet zu sehen, mit Feedback, Bewertungen
und Diskussionen, und um den Zeitraum mit dem davor zu vergleichen. Über die API ist es eine Abfrage mit
`dataset: "support"`:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/query \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{
    "dataset": "support",
    "metrics": ["pageviews", "conversations", "support_rate"],
    "dimensions": ["page"],
    "range": {"period": "30d"},
    "order_by": [{"metric": "conversations"}],
    "limit": 10
  }'
```

Das Support-Dataset nimmt die Dimensionen `page`, `hostname` und ein Tages-, Wochen- oder Monatsraster (höchstens eine
Seite oder ein Host und ein Raster), Filter auf `page` und `hostname`, aber keine Goals, Props oder Segmente. Seine
Zeilen sind die Seiten mit irgendeinem Support im Zeitraum.

## Insights

Fügen Sie einer Abfrage `"insights": true` hinzu oder öffnen Sie die Übersicht im Panel, und Yuva ergänzt kurze
Befunde, die aus den Zahlen der Antwort selbst berechnet sind. Es sind feste Regeln:

| Insight | Regel | Beispiel |
|---|---|---|
| `metric_change` | Eine Summe hat sich gegenüber dem Vergleichszeitraum um 20 % oder mehr bewegt, mit mindestens 10 davor oder danach. | „Die Besucher sind um 34 % gestiegen.“ |
| `row_change` | Bei einer Aufschlüsselung: Eine Zeile hat sich um 50 % oder mehr bewegt, mindestens 10 davor oder danach. Die drei größten. | „Besuche von `news.example.com` haben sich verdoppelt.“ |
| `new_value` | Eine Zeile mit mindestens 10, die im Vergleichszeitraum keine hatte: ein neuer Referrer, eine neue Seite oder Kampagne. Die drei größten. | „`/pricing` ist diesen Monat neu.“ |
| `support_hotspot` | Im Support-Dataset nach Seite: eine Seite mit mindestens 3 Unterhaltungen, deren Rate mindestens doppelt so hoch ist wie die des Zeitraums. Die drei höchsten. | „`/docs/install` hat 5 Unterhaltungen, 9 pro 1.000 Aufrufe, insgesamt 3.“ |

Die Änderungsregeln brauchen einen Vergleich (`compare`). Insights, die eine Seite, einen Referrer oder eine Kampagne
nennen, tragen Text, den die Browser der Besucher gesendet haben.

## Alerts

Alerts melden, wenn sich etwas ändert, ohne dass Sie nachsehen müssen. Sie werden jede Stunde geprüft und **nur für
Websites, bei denen mindestens ein Mitglied Alerts gewählt hat** (unten). Jede Art ist eine feste Regel:

| Art | Löst aus, wenn | Braucht |
|---|---|---|
| `spike` | Die Seitenaufrufe der letzten zwei abgeschlossenen Stunden sind 3-mal oder mehr so hoch wie der Durchschnitt derselben zwei Stunden an den 7 Tagen davor. | Eine Basis von mindestens 20 Aufrufen. |
| `drop` | Die Seitenaufrufe der letzten zwei abgeschlossenen Stunden sind halb so hoch wie dieser Durchschnitt oder weniger. | Eine Basis von mindestens 20 Aufrufen. |
| `goal_drop` | Die Conversions eines Goals gestern sind halb so hoch wie ihr Durchschnitt der 7 Tage davor oder weniger. Einmal am Tag geprüft, früh am Morgen in der Zeitzone der Website. | Im Schnitt mindestens 10 Conversions am Tag. |
| `support_load` | Die Unterhaltungen pro 1.000 Seitenaufrufe gestern sind doppelt so hoch wie die Rate der 28 Tage davor oder mehr. Einmal am Tag geprüft. | Ein verknüpfter Posteingang und gestern mindestens 5 Unterhaltungen. |
| `new_referrer` | Eine Seite, die gestern mindestens 10 Besuche gesendet hat, hatte in den 28 Tagen davor keinen gesendet. Einmal am Tag geprüft. | Nichts. |

Eine Art löst **einmal am Tag** pro Website aus (ein neuer Referrer einmal pro Seite). Wenn sie auslöst, tut Yuva:

1. eine Notiz ins Diagramm setzen, an den Tag, um den es geht ([Notiz](analytics-events.md) mit dem Text des Befunds;
   Sie können auch selbst Notizen hinzufügen);
2. ein Event `analytics.alert` speichern und den [Webhook](#der-webhook-analyticsalert) `analytics.alert` senden;
3. den Mitgliedern, die diese Art gewählt haben, eine E-Mail schicken.

`GET /v1/analytics/properties/{propertyId}/alerts` listet die Alerts einer Website, neueste zuerst, 100 auf einmal
(`before` nimmt das `created_at` des letzten für ältere; Scope `analytics:read`).

### Der Webhook analytics.alert

Fügen Sie einem [Webhook-Endpunkt](webhooks.md) das Ereignis `analytics.alert` hinzu. Ein auf einen Posteingang
beschränkter Endpunkt erhält die Alerts der mit diesem Posteingang verknüpften Websites; einer für den ganzen Workspace
erhält alle. Der Body ist der übliche [Umschlag](webhooks.md#ereignisse) (`type`, `timestamp`, `workspace_id`,
`inbox_id`, wenn die Website einen Posteingang hat, `data`):

```json
{
  "type": "analytics.alert",
  "timestamp": "2026-10-13T06:00:05Z",
  "workspace_id": "6f1e0c5a-0d2e-4a39-9f55-5d0c0d7f2a10",
  "inbox_id": "b1c4d8e2-7a3f-4c0e-8d61-2f9a5e3b7c40",
  "data": {
    "alert": {
      "id": "0a8f3c1d-6b52-4e17-9c44-7d2e1f0b9a63",
      "property_id": "c3d5e7f9-1a2b-4c6d-8e0f-3a5b7c9d1e2f",
      "kind": "support_load",
      "day": "2026-10-12",
      "metric": "support_rate",
      "current": 6.4,
      "baseline": 2.1,
      "text": "6.4 conversations per 1,000 page views yesterday against 2.1 on the 28 days before",
      "annotation_id": "5e9b2d7c-3f41-4a86-b0d5-8c1a6f4e2b97",
      "created_at": "2026-10-13T06:00:05Z"
    },
    "property": { "id": "c3d5e7f9-1a2b-4c6d-8e0f-3a5b7c9d1e2f", "name": "Acme website" }
  }
}
```

`metric` ist `pageviews`, `conversions`, `support_rate` oder `visits`; `baseline` ist das, womit die Regel `current`
verglichen hat (0 bei einem neuen Referrer). `spike` und `drop` haben außerdem `hours_from` und `hours_to`,
`goal_drop` `goal_id` und `goal_name`, `new_referrer` den Host als `referrer`. `text` ist Englisch, wie auf der Notiz.

## E-Mail-Berichte und Alert-Mails

Jedes Mitglied wählt für sich, pro Website:

- **Bericht**: keiner (Standard), wöchentlich oder monatlich.
- **Alerts**: beliebige der fünf Arten oben.

Die Einstellungen für Bericht und Alerts finden Sie im Panel bei den Seiten der Website; oder rufen Sie
`GET` und `PUT /v1/analytics/properties/{propertyId}/subscription` auf (mit der Sitzung eines Mitglieds, nicht mit
einem API-Key):

```json
{ "report": "weekly", "alerts": ["spike", "drop", "support_load"] }
```

| Bericht | Geht raus | Deckt ab |
|---|---|---|
| Wöchentlich | Montag um 08:00 in der Zeitzone der Website | Die Woche davor, Montag bis Sonntag. |
| Monatlich | Am 1. des Monats um 08:00 in der Zeitzone der Website | Den Monat davor. |

Ein Bericht ist in der Sprache des Mitglieds geschrieben und enthält schlichte Zahlen: die Kennzahlen gegenüber dem
Zeitraum davor, die fünf wichtigsten Seiten und Quellen, die Conversions der Goals und, bei einer Website mit
Posteingang, die Unterhaltungen, das Feedback und die Bewertungen sowie die Seite mit den meisten Unterhaltungen. Er
hat **keine Bilder und nichts, was Öffnungen oder Klicks verfolgt**. Eine Alert-Mail sagt, was gefunden wurde, und
verlinkt die Website im Panel.

Um zu sehen, wie ein Bericht aussieht, nutzen Sie die Schaltfläche zum sofortigen Senden an derselben Stelle oder
`POST /v1/analytics/properties/{propertyId}/subscription/send-report`. Sie sendet die letzten sieben abgeschlossenen
Tage, bei monatlicher Wahl den letzten Monat; fünf pro Stunde und Mitglied.

## Broadcast-Kampagnen verfolgen

Wenn ein [Broadcast](broadcasts.md) rausgeht, hängt Yuva UTM-Parameter an seine Web-Links, damit die Besuche, die er
bringt, in Analytics unter Quellen, `utm_campaign` und dem Channel `E-mail` erscheinen. Das ist **standardmäßig an**.
Ein Link bekommt:

```
?utm_source=newsletter&utm_medium=email&utm_campaign=<Kampagne>
```

Die Kampagne wird beim Anlegen aus dem Namen des Broadcasts gebildet (Kleinbuchstaben, Ziffern, `-`, `_` und `.`, bis
zu 100 Zeichen); im Entwurf können Sie sie ändern. Unberührt bleiben:

- Links, die schon einen eigenen `utm_`-Parameter haben;
- Links, die nicht `http` oder `https` sind, etwa `mailto:`;
- Yuvas eigene Links: Abmelden, Einstellungen und Klick-Tracking.

Schalten Sie es in den Optionen des Broadcasts aus oder mit `"utm": false` im Entwurf
(`PATCH /v1/broadcasts/{broadcastId}`); `utm_campaign` ist das Kampagnenfeld. Es funktioniert mit oder ohne
[Klick-Tracking](broadcasts.md#klick-tracking) und sendet nichts an Dritte: Den Besuch zählt Ihr eigenes Analytics.
Broadcasts, die schon über den Entwurf hinaus waren, als es UTM-Links gab, behalten ihre Links, wie sie gesendet wurden.

Der Bericht des Broadcasts zeigt dann **Besuche aus dieser Kampagne** pro Website: Besucher, Besuche und Seitenaufrufe,
deren `utm_campaign` die des Broadcasts ist, ab dem Tag, an dem der Versand begann, für die Websites, die Sie sehen
dürfen (braucht `analytics:read`; `campaign_visits` in der API).

## Einen Assistenten fragen

Ein über [MCP](mcp.md) verbundener KI-Assistent kann dieselben Zahlen lesen. Zum Beispiel:

> Welche Seiten lösen diesen Monat die meisten Unterhaltungen aus?

Der Assistent ruft `analytics_support_report` auf und antwortet mit den Seiten, ihren Unterhaltungen, Aufrufen und der
Rate. Tools, Prompts und Scopes stehen unter [Analytics über einen Assistenten](mcp.md#analytics-über-einen-assistenten).

## Referenz

| Aufruf | Tut |
|---|---|
| `POST /v1/analytics/properties/{propertyId}/query` | Die Zahlen; `dataset: "support"` und `insights: true` wie oben. Scope `analytics:read`. |
| `GET /v1/analytics/properties/{propertyId}/alerts` | Alerts, die die Website ausgelöst hat. Scope `analytics:read`. |
| `GET`, `PUT /v1/analytics/properties/{propertyId}/subscription` | Meine Wahl von Bericht und Alerts. Mitgliedssitzung. |
| `POST /v1/analytics/properties/{propertyId}/subscription/send-report` | Schickt mir sofort einen Bericht. Mitgliedssitzung. |
| `GET /v1/analytics/properties/{propertyId}/annotations` | Notizen im Diagramm, Alerts eingeschlossen. |
| `GET /v1/broadcasts/{broadcastId}/report` | Der Bericht des Broadcasts mit `campaign_visits`. |

Alle Felder stehen im [API-Vertrag](../../openapi/openapi.yaml).

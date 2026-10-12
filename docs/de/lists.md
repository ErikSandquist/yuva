# Mailinglisten und Anmeldeformulare

Eine Mailingliste gehört zu einem Posteingang und enthält die Menschen, die zugestimmt haben, von
Ihnen zu hören. Besucher treten über ein Formular auf Ihrer Website bei, Ihr Backend trägt Personen
ein, deren Einwilligung Sie schon haben, und jede Änderung landet in einem Einwilligungsprotokoll, das
Sie exportieren können.

Die Bestätigungs- und Willkommensmails einer Liste gehen über den [E-Mail-Kanal](email.md) des
Posteingangs hinaus, mit dessen Adresse und SMTP-Konto. Ohne E-Mail-Kanal versendet Yuva für eine
Liste nichts.

## 1. Absenderangaben eintragen

Mails mit Werbeinhalt müssen sagen, wer sie sendet und wie man ihn erreicht. Im Panel: Settings → den
Posteingang → Sender identity. Tragen Sie den rechtlichen Namen, eine Postanschrift, eine
Registrierung (eine MERSİS-Nummer, Handelsregister- oder USt-IdNr.) und einen Kontakt (E-Mail-Adresse,
Telefonnummer oder URL) ein. Yuva setzt sie in die Fußzeile der Listenmails des Posteingangs.

Per API: `PATCH /v1/inboxes/{inboxId}` mit einem `sender`-Objekt.

## 2. Eine Liste anlegen

Im Panel: Broadcasts, dann New list. Wählen Sie den Posteingang und den E-Mail-Kanal, der die Mails
der Liste versendet.

| Einstellung (API-Feld) | Bedeutung |
|---|---|
| Name (`name`) | Bis zu 100 Zeichen, im Posteingang eindeutig. |
| Beschreibung (`description`) | Bis zu 500 Zeichen, auf Formularen und der Einstellungsseite angezeigt. |
| Double-Opt-in (`double_opt_in`) | Standardmäßig an. Wer sich über ein Formular anmeldet, bestätigt per E-Mail, bevor er angemeldet ist. |
| Öffentlich (`public`) | Standardmäßig an. Wird auf Formularen und der Einstellungsseite angeboten. Private Listen füllen nur die API und Ihr Team. |
| Kommerziell (`commercial`) | Standardmäßig an. Kennzeichnet Werbeinhalt; steht im Export der Einwilligungen. |
| Willkommen (`welcome`) | Standardmäßig an. Sendet bei der Anmeldung eine Bestätigung mit Abmeldelink. |
| Versandkanal (`channel_id`) | Ein E-Mail-Kanal desselben Posteingangs mit SMTP-Konto. Ohne ihn versenden Formulare nichts, und Anmeldungen bleiben `pending`. |

Per API: `POST /v1/lists` mit `inbox_id` und `name`.

## 3. Das Formular in Ihre Website einbauen

Das Formular nutzt einen [Chat-Kanal](widget.md#1-einen-chat-kanal-anlegen) desselben Posteingangs:
Sein öffentlicher Schlüssel und seine erlaubten Origins bestimmen, wo das Formular erscheinen darf.
Auf der Seite der Liste im Panel füllt der Reiter Form diese Beispiele für Sie aus.

```html
<script src="https://support.example.com/yuva-subscribe.js" defer></script>
<yuva-subscribe
  channel="yuva_pk_xxxxxxxxxxxxxxxx"
  lists="LIST-ID"
  privacy-url="https://www.example.com/privacy"
></yuva-subscribe>
```

Ohne `lists` bietet das Element alle öffentlichen Listen des Posteingangs an, je mit einem Kontrollkästchen.
Es fragt nach einer E-Mail-Adresse und zeigt ein nicht angekreuztes Einwilligungskästchen. Ohne Haken
kann sich der Besucher nicht anmelden; der Satz, den er gesehen hat, wird als Nachweis gespeichert.

In einem Projekt mit Bundler führen Sie `npm install useyuva` aus und nehmen eines davon:

| Stack | Code |
|---|---|
| Beliebiger Bundler | `import "useyuva/subscribe"`, dann das Element oben mit einem `server`-Attribut. |
| React | `import { YuvaSubscribe } from "useyuva/react"` und `<YuvaSubscribe channel server lists={["LIST-ID"]} />`. |
| Nuxt | `modules: ["useyuva/nuxt"]` und `yuva: { server, channel }` in `nuxt.config`, dann `<YuvaSubscribe :lists="['LIST-ID']" />`. |
| shadcn/ui | `npx shadcn@latest add https://useyuva.com/r/yuva-subscribe.json`, dann `<YuvaSubscribe server channel lists={["LIST-ID"]} />`. |

Sendet Ihre Website eine Content Security Policy, erlauben Sie Ihren Yuva-Server in `script-src` und
`connect-src`. Für angemeldete Nutzer geben Sie dem Element mit `setIdentityToken` ein
[Identitäts-Token](identity.md): Es zeigt dann je Liste einen Schalter statt des E-Mail-Felds und
meldet sofort an, wenn die Adresse des Nutzers verifiziert ist.

## 4. Was Abonnenten sehen

1. Sie füllen das Formular aus. Die Antwort ist immer dieselbe, ob die Adresse neu, schon angemeldet,
   abgemeldet oder unbekannt ist; das Formular verrät also niemandem, wer auf Ihrer Liste steht.
2. Bei Double-Opt-in erhalten sie eine Mail mit einem Link zu `/s/…`. Das Öffnen der Seite ändert
   nichts; bestätigt wird mit der Schaltfläche darauf. Danach folgt die Willkommensmail, falls die Liste
   eine sendet.
3. Jede Listenmail hat einen Link zu `/m/…`, wo sich die Person von einer Liste oder von allem abmeldet
   oder Listen auswählt. Auch das Öffnen dieser Seite ändert nichts. Mailprogramme, die
   Ein-Klick-Abmeldung (RFC 8058) unterstützen, melden die Liste mit einer einzigen Anfrage ab.

Seiten und Mails gibt es auf Englisch, Türkisch und Deutsch. Nach einer Abmeldung kann sich nur die
Person selbst wieder anmelden: Ihr Team und Ihre Schlüssel können es nicht, und eine Adresse, die sich
von allem abgemeldet, eine Mail als Spam gemeldet hat oder nach einem Widerspruch gelöscht wurde, wird
abgelehnt (`409 opted_out`). Ausstehende Anmeldungen werden nach 30 Tagen gelöscht, abgemeldete nach
drei Jahren.

## 5. Abonnenten aus dem Backend eintragen

Für eine ganze Datei voller Personen siehe [Abonnenten importieren](importing.md). Das ist für Personen, die anderswo zugestimmt haben, etwa bei der Registrierung. Sie erklären, wie die
Person zugestimmt hat, und Yuva speichert es als Nachweis. Der Schlüssel braucht den Scope `lists:write`.

```sh
curl -X POST https://support.example.com/v1/lists/LIST-ID/subscriptions \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Idempotency-Key: signup-1234" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "ada@example.com",
    "name": "Ada",
    "consent": {
      "confirmed": true,
      "obtained_at": "2026-10-01T09:30:00Z",
      "text": "Ich möchte Produktneuigkeiten per E-Mail erhalten.",
      "page_url": "https://www.example.com/signup",
      "reference": "order-1234"
    }
  }'
```

Mit `confirmed: true` ist die Anmeldung sofort `subscribed`, und die Willkommensmail geht hinaus. Mit
`confirmed: false` ist sie `pending`, und die Person erhält zuerst eine Bestätigungsmail; nutzen Sie das,
wenn Sie sich der Zustimmung nicht sicher sind. `201` ist eine neue Anmeldung, `200` eine Adresse, die
schon auf der Liste war.

## 6. Einwilligungen exportieren

Öffnen Sie die Liste und dann Export, oder rufen Sie `GET /v1/lists/{listId}/consent-export` auf
(`format=csv` oder `json`, `since` für neuere Ereignisse; für Absender in der Türkei siehe Türkei: İYS unten). Jede Zeile ist ein Ereignis mit Adresse,
Status, Zeit, Quelle, IP-Adresse, Seite und den Worten, die die Person gesehen hat.
`GET /v1/contacts/{contactId}/consent` liefert die Einträge einer Person über alle Listen, für ein
Auskunftsersuchen nach Art. 15 DSGVO oder Art. 11 KVKK. Zellen, die mit `=`, `+`, `-` oder `@`
beginnen, erhalten ein Präfix, damit Tabellenprogramme sie nicht als Formel ausführen.

## Türkei: İYS

Yuva verbindet sich nicht mit İYS. Es liefert Ihnen eine Datei, die Sie selbst bei İYS hochladen; die
Registrierung als Absender dort ist ebenfalls Ihre Aufgabe.

Öffnen Sie die Liste, dann Export, und wählen Sie **Für İYS herunterladen**, oder rufen Sie
`GET /v1/lists/{listId}/consent-export?format=iys` auf. Mit einem Datum bei „Ab (optional)“ oder mit
`since` erhalten Sie nur die Personen, die ab dann zugestimmt haben oder gegangen sind. Die Datei ist eine
CSV mit einer Zeile pro Abonnent:

| Spalte | Wert |
|---|---|
| `recipient` | Die E-Mail-Adresse. |
| `type` | `EPOSTA`. |
| `source` | `HS_WEB` für Anmeldungen über ein Formular oder die Einstellungsseite, sonst `HS_EORTAM`. |
| `status` | `ONAY` für angemeldet, `RET` für abgemeldet. |
| `consentDate` | Wann die Person zugestimmt hat oder gegangen ist, in türkischer Zeit (`YYYY-MM-DD HH:MM:SS`). |
| `recipientType` | `BIREYSEL`. |

Ausstehende Anmeldungen, die noch nicht bestätigt haben, fehlen in der Datei. Die Spalten folgen nach
unserem Kenntnisstand den Einwilligungsfeldern von İYS. Prüfen Sie sie vor dem Hochladen gegen das, was
Ihr İYS-Konto verlangt, und registrieren Sie nur Listen, über die Sie Werbe-E-Mails senden. Diese Seite
ist keine Rechtsberatung.

## Fehlerbehebung

- **Das Formular zeigt keine Liste**: Die Liste ist privat oder archiviert, oder `lists` nennt eine ID,
  die keine öffentliche Liste des Posteingangs des Kanals ist.
- **Das Formular zeigt einen Fehler**: Die Origin der Seite gehört nicht zu den erlaubten Origins des
  Chat-Kanals.
- **Keine Bestätigungsmail**: Die Liste hat keinen Versandkanal (das Panel markiert das), oder die Adresse
  hat heute schon 3 Bestätigungen erhalten.
- **`409 email_channel_required`**: Der Kanal ist kein E-Mail-Kanal des Posteingangs der Liste mit
  SMTP-Konto.

## Referenz

### Attribute von `<yuva-subscribe>`

| Attribut | Bedeutung |
|---|---|
| `channel` | Der öffentliche Schlüssel des Chat-Kanals. Pflicht. |
| `server` | URL des Yuva-Servers. Standard: der Ort, von dem `yuva-subscribe.js` kam. |
| `lists` | Kommagetrennte Listen-IDs. Leer: alle öffentlichen Listen. |
| `layout` | `stacked` (Standard) oder `inline`. |
| `name` | `off` (Standard), `optional` oder `required`: fügt ein Namensfeld hinzu. |
| `button-text` | Beschriftung der Schaltfläche. |
| `consent` | Ihr eigener Einwilligungssatz. Er wird so gespeichert, wie er angezeigt wurde. |
| `privacy-url` | Link zu Ihrer Datenschutzerklärung, neben dem Kästchen angezeigt. |
| `locale`, `dir` | `en`, `tr` oder `de`; `ltr` oder `rtl`. |
| `identity-token` | Eine Token-Zeichenkette; meist ist `setIdentityToken` besser. |

Gestalten Sie es mit `--yuva-accent`, `--yuva-on-accent`, `--yuva-radius`, `--yuva-font` und
`::part(field|button|consent|status)`.

### Ereignisse

| Ereignis | `detail` |
|---|---|
| `yuva-subscribe` | `{ lists, confirmation }`: die angebotenen IDs und ob eine Bestätigungsmail erwartet wird. |
| `yuva-subscribe-error` | `{ code }`. |

React und shadcn: `onSubscribe`, `onError`. Nuxt: `@subscribe`, `@error`.

### Endpunkte

| Endpunkt | Zweck |
|---|---|
| `GET`, `POST /v1/lists` | Listen anzeigen und anlegen (`archived=true` zeigt archivierte). |
| `GET`, `PATCH`, `DELETE /v1/lists/{listId}` | Eine Liste lesen, ändern, archivieren (`archived: true`) oder löschen. |
| `GET /v1/lists/{listId}/growth` | Neu, bestätigt und abgemeldet je Tag, `days=30` oder `90`. |
| `GET`, `POST /v1/lists/{listId}/subscriptions` | Abonnenten anzeigen (`state`, `q`) und hinzufügen. |
| `GET`, `PATCH`, `DELETE /v1/lists/{listId}/subscriptions/{subscriptionId}` | Mit Einwilligungsereignissen lesen, abmelden (`state: unsubscribed`), löschen. |
| `POST /v1/lists/{listId}/subscriptions/{subscriptionId}/confirmation` | Die Bestätigungsmail erneut senden. |
| `GET /v1/lists/{listId}/consent-export` | Einwilligungen einer Liste. |
| `GET /v1/contacts/{contactId}/subscriptions` | Die Anmeldungen eines Kontakts. |
| `GET /v1/contacts/{contactId}/consent` | Die Einwilligungen eines Kontakts. |
| `GET /client/v1/channels/{channel_key}/lists` | Öffentliche Listen und ein Formular-Token, für das Formular. |
| `POST /client/v1/channels/{channel_key}/subscriptions` | Über ein Formular anmelden. |
| `GET /client/v1/subscriptions`, `PUT`, `DELETE /client/v1/subscriptions/{listId}` | Listen eines angemeldeten Nutzers, an- und abmelden. |

Die vollständigen Anfrage- und Antwortformen stehen im [API-Vertrag](../../openapi/openapi.yaml). Für
eigene Formulare bietet `useyuva` im [Headless-Client](headless.md) `lists()`, `subscribe()`,
`subscriptions()`, `subscribeList()` und `unsubscribeList()`.

### Scopes und Ereignisse

| Name | Bedeutung |
|---|---|
| `lists:read` | Listen, Anmeldungen, Wachstum und Einwilligungsexporte lesen. |
| `lists:write` | Listen anlegen, ändern, löschen; Abonnenten hinzufügen, abmelden, löschen; Bestätigungen erneut senden. |
| `subscription.created` | [Webhook](webhooks.md)-Ereignis: Jemand hat sich angemeldet, in jedem Status. |
| `subscription.updated` | Webhook-Ereignis: Der Status hat sich geändert; der alte steht in `data.previous_state`. |

Schlüssel, die vor diesen Scopes erstellt wurden, erhalten sie nicht.

### Limits

| Limit | Wert |
|---|---|
| Listen in einem Formular | 10 |
| Formularanfragen je IP-Adresse | 10 pro Stunde |
| Formularanfragen je Kanal | 600 pro Stunde |
| Bestätigungsmails je Adresse | 3 pro Tag und 1 je Liste in 10 Minuten |
| Bestätigungsmails je Kanal | 1.000 pro Tag |
| Erneut gesendete Bestätigungen aus Panel oder API | 3 je Anmeldung und Tag, insgesamt 10 |
| Formular-Token | Gültig von 2 Sekunden bis 2 Stunden nach der Ausgabe |

# Consent-Funktionen

Yuva Analytics kann einen Besucher standardmäßig nicht von einem Tag zum nächsten wiedererkennen ([Analytics und Datenschutz](analytics-privacy.md)).
Die Consent-Funktionen ändern das für Besucher, die zugestimmt haben: Mit ihrem Ja behält Ihre Website oder App eine
zufällige ID auf ihrem Gerät, und Sie erhalten wiederkehrende Besucher, Retention, Funnels über mehrere Tage und für
angemeldete Nutzer ihre letzten Besuche auf der Kontaktseite. Wer nicht zugestimmt hat, wird genau wie zuvor gezählt.

Dies baut auf [Analytics](analytics.md) und dem [Consent-Modus](analytics.md#consent-modus) auf. Das Consent-Tool bringen Sie
selbst mit; Yuva zeigt nie ein Banner.

## Was sich mit Einwilligung ändert

| Mit Einwilligung zu `persistent` | Was Sie bekommen |
|---|---|
| Eine zufällige ID auf dem Gerät | Vom Skript oder der App erzeugt, höchstens 13 Monate ab Erzeugung aufbewahrt und mit jedem Event gesendet. Der Server speichert nur einen Hash davon unter einem Geheimnis der Property; dieselbe ID bedeutet auf einer anderen Property nichts. |
| Besucher einmal gezählt | Ein Besucher zählt über einen Zeitraum, eine Woche oder einen Monat einmal statt einmal pro Tag. |
| Neu und wiederkehrend | Die Karte **New and returning** der Übersicht und die Dimension `visitor_type`: `new` am Tag, an dem die ID zuerst gesehen wurde, danach `returning`. Für alle anderen leer. |
| Retention | Der Tab **Retention** und `GET /v1/analytics/properties/{propertyId}/retention`: Kohorten nach Tag, Woche oder Monat und wie viele davon zurückkamen. |
| Funnels über Tage | `consented_visitors` eines Funnels verfolgt diese Besucher über Besuche und Tage, jeder einmal gezählt. |
| Mit `identify` zusätzlich | **Recent visits** auf der Kontaktseite und `GET /v1/contacts/{contactId}/visits`. |

In diesen Ansichten sind nur Besucher mit Einwilligung. Eine Property ohne die Funktion hat keine Retention-Kohorten.

### Was ohne Einwilligung nie passiert

- Auf dem Gerät wird nichts gespeichert. Die einzige Ausnahme ist das eigene Opt-out eines Besuchers.
- Es wird keine ID erzeugt, gesendet oder gelesen; jeder Besucher wird wie im Standardmodus über den Hash des Tages gezählt.
- Kein Besucher wird mit einem Kontakt oder einer Unterhaltung verbunden: Die Kontaktseite listet keine Besuche.
- Ein Browser, der Do Not Track oder Global Privacy Control sendet, bekommt keine ID, was das Consent-Tool auch sagt.
- Ein Token ohne Einwilligung oder mit gefälschter, abgelaufener oder schon verwendeter Signatur wird ignoriert.

## Für eine Property einschalten

Öffnen Sie im Panel **Settings → Tracking → Consent features** der Website. Ein Owner oder Admin setzt zwei Schalter (bei
Properties für Kinder sind sie aus und deaktiviert); in der API sind es die `consent_features` der Property:

| Schalter | `consent_features` | Wirkung |
|---|---|---|
| **Persistent id** | `persistent` | Besucher, die zustimmen, erhalten die 13-Monate-ID. |
| **Link signed-in users** | `identify` | Die ID darf mit einem Kontakt verknüpft werden. Braucht **Persistent id** und eine mit einem Posteingang verbundene Website, dessen [Identity-Secret](identity.md) die Tokens signiert. |

Oder mit einem Schlüssel mit `analytics:manage`:

```sh
curl -X PATCH -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  https://support.example.com/v1/analytics/properties/$PROPERTY \
  -d '{"consent_features": ["persistent", "identify"]}'
```

Ohne die Einstellung werden `persistent` und `identify` ignoriert und Besucher im Standardmodus gezählt.

**Properties für Kinder können sie nicht nutzen.** Eine Property mit **Audience: Children** lehnt `consent_features` ab
(`400`): Das Gerät eines Kindes bekommt nie eine ID, und kein Kind wird mit einem Kontakt verknüpft.

Informieren Sie Ihre Besucher. Ergänzen Sie in Ihrer Datenschutzerklärung, dass ein zustimmender Besucher bis zu 13
Monate eine ID auf seinem Gerät erhält, wofür sie dient und wie er widerruft ([Widerruf](#widerruf-und-löschung)).

## Das Skript aus Ihrem Consent-Tool aufrufen

Das Skript merkt sich keine Antwort; rufen Sie es deshalb bei **jedem Seitenaufruf** mit der aktuellen Wahl des Besuchers
auf und übergeben Sie `persistent: true` bei **jedem** Aufruf, solange der Besucher zustimmt: Jeder `consent`-Aufruf ohne
es, auch `consent({ analytics: true })`, widerruft eine gespeicherte ID.

```js
window.yuva.consent({ analytics: true, persistent: true })
window.yuva.consent(false)
```

`consent({ analytics: true })` zählt den Besucher ohne ID. Mit `persistent: true` wird die ID erzeugt und im lokalen
Speicher des Browsers gehalten. `consent(false)`, ein Aufruf ohne `persistent: true` und ein Opt-out beenden die ID: Sie wird
vom Gerät gelöscht, und der Server vergisst ihre Verknüpfung mit einem Kontakt und ihre Retention-Tage, auch wenn der Browser
Do Not Track sendet. Bevor das Skript geladen ist, reihen Sie den Aufruf ein:
`;(window.yuvaq = window.yuvaq || []).push(["consent", { analytics: true, persistent: true }])`.

**Script-Tag.** Das übliche `/yuva-analytics.js` lässt die Consent-Funktionen weg, um klein zu bleiben. Laden Sie sie mit
`/yuva-analytics-consent.js` (unter 2 KB gzip) vom selben Ort, mit denselben Attributen, und setzen Sie wie im
[Consent-Modus](analytics.md#consent-modus) `data-consent="required"`. `window.yuva` hat dann auch `identify` und
`persistentId`. Mit `useyuva/analytics`, React und Nuxt sind die Aufrufe schon da.

### Rezepte

Das sind Beispiele für die folgenden Consent-Tools mit den Callbacks, die deren Dokumentation nennt. Prüfen Sie die
Dokumentation Ihres Tools, denn Namen und Kategorien ändern sich; wichtig ist, dass `consent(...)` bei jedem Seitenaufruf
und bei jeder Änderung der Wahl mit der Entscheidung des Besuchers aufgerufen wird.

**Jedes Tool.** Schreiben Sie eine Funktion und rufen Sie sie dort auf, wo Ihr Tool beim Laden und bei Änderungen aufruft:

```js
function syncYuva(analytics, persistent) {
  if (analytics) window.yuva.consent({ analytics: true, persistent })
  else window.yuva.consent(false)
}
```

**Cookiebot.** Cookiebot löst bei jedem Seitenaufruf und bei Änderungen `CookiebotOnConsentReady` aus; die Kategorie
**Statistik** ist `Cookiebot.consent.statistics`.

```js
window.addEventListener("CookiebotOnConsentReady", () => {
  const ok = Cookiebot.consent.statistics
  ok ? window.yuva.consent({ analytics: true, persistent: true }) : window.yuva.consent(false)
})
```

**OneTrust.** `OptanonWrapper` läuft beim Laden und nach jeder Änderung, `OnetrustActiveGroups` listet die akzeptierten
Kategorie-IDs, `C0002` ist **Performance**:

```js
function OptanonWrapper() {
  const ok = window.OnetrustActiveGroups.includes("C0002")
  ok ? window.yuva.consent({ analytics: true, persistent: true }) : window.yuva.consent(false)
}
```

**Klaro.** Fügen Sie der Klaro-Konfiguration einen Dienst hinzu; sein `callback` erhält die Wahl beim Laden und bei Änderungen:

```js
services: [{
  name: "yuva",
  purposes: ["analytics"],
  callback: (consent) => {
    consent ? window.yuva.consent({ analytics: true, persistent: true }) : window.yuva.consent(false)
  },
}]
```

**CookieYes.** Das Event `cookieyes_consent_update` auf `document` trägt bei einer Änderung die akzeptierten Kategorien in
`event.detail.accepted`; beim Laden antwortet `getCkyConsent()`:

```js
const apply = (ok) =>
  ok ? window.yuva.consent({ analytics: true, persistent: true }) : window.yuva.consent(false)

document.addEventListener("cookieyes_consent_update", (event) => {
  apply(event.detail.accepted.includes("analytics"))
})
apply(getCkyConsent().categories.analytics)
```

Hat Ihr Tool einen eigenen Schalter für die ID, übergeben Sie dessen Antwort als `persistent` statt `true`.

## Den Besuch mit einem Kontakt verknüpfen

Geben Sie dem Skript für angemeldete Nutzer einer Property mit `identify` ein Token, das Ihr Backend signiert, wie beim
[Widget](identity.md): derselbe Posteingang, dasselbe Secret, dieselben Claims, mit `exp` höchstens 10 Minuten in der Zukunft
und einer `jti`, die nur einmal verwendet wird. Nachdem sich der Besucher angemeldet hat:

```js
const { token } = await fetch("/yuva-identity").then((r) => r.json())
window.yuva.identify(token)
```

`/yuva-identity` ist Ihr eigener Endpunkt, den die [Identity-Anleitung](identity.md#2-ein-token-für-den-aktuellen-nutzer-signieren)
beschreibt. Yuva prüft das Token wie das einer Chat-Sitzung, findet oder legt den Kontakt für `sub` an und verknüpft die ID
und ihre Besuche mit ihm. Die Besuche erscheinen auf der Kontaktseite und unter `GET /v1/contacts/{contactId}/visits`
(Scopes `contacts:read` und `analytics:read`), solange die Detaildaten aufbewahrt werden. Rufen Sie `identify` nach jeder
Anmeldung erneut auf; ein Token lässt sich nicht zweimal verwenden. Für einen Besucher, der nicht sowohl Analytics als auch
`persistent` zugestimmt hat, bewirkt es nichts.

Das Löschen eines Kontakts löscht die mit ihm verknüpften Besuche; das Zusammenführen zweier Kontakte verschiebt sie.

## Widerruf und Löschung

Widerruft ein Besucher in Ihrem Consent-Tool, ruft Ihr Callback `consent(false)` auf. Die ID verschwindet vom Gerät, und der
Server löscht die Verknüpfung der ID mit einem Kontakt und ihre Retention-Tage. Ein Widerruf wird auch angenommen, wenn der
Browser Do Not Track sendet.

Verlangt eine Person die **Löschung** dessen, was Sie über sie haben, löschen Sie alles, was unter ihrer ID gespeichert ist,
mit einem Schlüssel mit `analytics:manage`:

```sh
curl -X DELETE -H "Authorization: Bearer $YUVA_API_KEY" \
  https://support.example.com/v1/analytics/properties/$PROPERTY/visitors/$PERSISTENT_ID
```

Das löscht die Rohbesuche und -events der ID, ihre Retention-Tage und die Verknüpfung mit einem Kontakt, fasst die
abgeschlossenen Tage ohne sie neu zusammen und antwortet mit `204`, ob etwas gespeichert war oder nicht. Die ID besteht aus
16 bis 64 Buchstaben, Ziffern, `-` und `_`. Der Browser der Person kennt sie: `window.yuva.persistentId()` liefert sie, sodass
ein Button „Meine Daten löschen“ auf Ihrer Website sie an Ihr Backend senden kann, das den Aufruf mit Ihrem Schlüssel macht.
Yuva speichert nur einen Hash der ID; Sie können sie also nicht über einen Namen oder eine E-Mail-Adresse finden. Um auch den
Kontakt eines angemeldeten Nutzers zu löschen, siehe [Einen Nutzer löschen](identity.md#einen-nutzer-löschen).

## Server-Events

Ein Backend, das Events für einen zustimmenden Besucher meldet, kann ihn benennen. Senden Sie die ID, die der Browser hält,
als `persistent_id` mit `consent: true` an `POST /v1/analytics/properties/{propertyId}/events`:

```json
{
  "events": [{
    "name": "Signup",
    "url": "https://www.example.com/signup",
    "persistent_id": "0f8fad5b-d9cb-469f-a165-70867728950e",
    "consent": true
  }]
}
```

Das Event gehört dann wie die eigenen Events des Browsers zu den Tagen des Besuchers. Mit `identity_token` zusätzlich (bei
einer Property mit `identify`) identifiziert es den Besucher. Ohne `consent: true` wird die ID ignoriert. Die übrigen Felder
stehen unter [Server-Events](analytics-server.md).

## Apps

Die Swift- und Kotlin-Bibliotheken haben dieselben Aufrufe wie das Skript. Wann gefragt wird, entscheidet Ihr eigener
Consent-Dialog. Die ID liegt im privaten Speicher der App, höchstens 13 Monate. Übergeben Sie wie im Web bei jedem
`consent`-Aufruf `persistent: true`, solange der Nutzer zustimmt: Ein Aufruf ohne es widerruft die ID.

```swift
YuvaAnalytics.consent(true, persistent: true)
YuvaAnalytics.identify(token)
let id = YuvaAnalytics.persistentId
YuvaAnalytics.consent(false)
```

```kotlin
YuvaAnalytics.consent(true, persistent = true)
YuvaAnalytics.identify(token)
val id = YuvaAnalytics.persistentId
YuvaAnalytics.consent(false)
```

`identify` und ein Widerruf werden einmal gesendet und nie eingereiht. `persistentId` ist `nil` oder `null`, wenn der Nutzer
keine ID hat. Rufen Sie `consent` bei jedem Start auf, wie unter [Einwilligung und Opt-out](analytics-apps.md#einwilligung-und-opt-out).

### Antworten für die Stores

Sie gelten, wenn `persistent` an ist und der Nutzer zugestimmt hat; sonst gelten die [Antworten auf der Apps-Seite](analytics-apps.md#antworten-für-die-stores).
Die ID ist zufällig, wird von der Bibliothek erzeugt, ist nicht die Werbe-ID und wird mit niemandem geteilt.

**App Store.** Das Privacy Manifest der Swift-Bibliothek deklariert **Device ID** (für Analytics erhoben, nicht für Tracking
genutzt); der Datenschutzbericht von Xcode listet sie also schon. Antworten Sie in App Store Connect:

| Datentyp | Mit dem Nutzer verknüpft | Für Tracking genutzt | Zweck |
|---|---|---|---|
| Identifiers → Device ID | Nein, solange Ihre App nicht `identify` aufruft | Nein | Analytics |
| Identifiers → User ID, nur wenn Sie `identify` aufrufen | Ja | Nein | Analytics |

Yuva verbindet die ID mit keinen Daten aus Apps oder Websites anderer Unternehmen; sie ist daher kein Tracking im Sinne von
Apple. Antworten Sie bei Tracking mit **No**, sofern Ihre eigene Nutzung nichts anderes ergibt.

**Google Play.** Unter **Data safety**, weiterhin ohne `AD_ID`:

| Datentyp | Erhoben | Weitergegeben | Hinweise |
|---|---|---|---|
| Device or other IDs | Ja | Nein | Die zufällige ID, optional (der Nutzer entscheidet). Zweck: Analytics. |
| Personal info → User IDs, nur wenn Sie `identify` aufrufen | Ja | Nein | Optional. Zweck: Analytics. |

Markieren Sie beide im Formular als **optional**, weil der Nutzer ablehnen kann, und ergänzen Sie, dass Daten auf Anfrage
gelöscht werden können.

## Referenz

| Aufruf | Wirkung |
|---|---|
| `PATCH /v1/analytics/properties/{propertyId}` | Setzt `consent_features`. Scope `analytics:manage`. |
| `GET /v1/analytics/properties/{propertyId}/retention` | Kohorten der zustimmenden Besucher (`grain`: `day`, `week`, `month`; `from`, `to`). Scope `analytics:read`. |
| `DELETE /v1/analytics/properties/{propertyId}/visitors/{persistentId}` | Löscht alles, was unter einer ID gespeichert ist. Scope `analytics:manage`. |
| `GET /v1/contacts/{contactId}/visits` | Die letzten Besuche eines verknüpften Kontakts. |
| `POST /v1/analytics/properties/{propertyId}/events` | Server-Events mit `persistent_id`, `consent` und `identity_token`. |

Retention-Kohorten reichen bis 30 Tage oder 12 Wochen oder Monate und werden stündlich neu aufgebaut. Eine ID wird 13 Monate
nach dem ersten Sehen vergessen. Pro Adresse und Property werden 20 Consent-Änderungen pro Minute angenommen.

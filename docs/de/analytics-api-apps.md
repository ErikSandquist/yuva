# API-Monitoring aus iOS- und Android-Apps

Die Swift- und Kotlin-Bibliotheken können die Aufrufe messen, die Ihre App an Ihre eigene API sendet, und sie neben
dem, was Ihr Server meldet, einer API-Property hinzufügen. So sehen Sie die API wie Ihre Nutzer: Die Zeit enthält
deren Netzwerk, und Aufrufe ohne Antwort werden mitgezählt. Sie brauchen eine API-Property: siehe
[Eine API-Property anlegen](analytics-api.md#eine-api-property-anlegen). Diese Seite baut auf
[Analytics in Apps](analytics-apps.md) auf; sie ist aus, bis Sie sie einschalten.

## Die App zulassen

1. Nehmen Sie den öffentlichen Schlüssel der API-Property (`yuva_ak_…`) aus dem Feld `public_key` der Antwort von
   `GET /v1/analytics/properties`. Er ist öffentlich wie der Schlüssel einer Site-Property und funktioniert nur für
   die hier beschriebenen Aufrufe.
2. Erlauben Sie die Plattform an der Property (Scope `analytics:manage`, Owner und Admins). Der Schlüssel einer
   API-Property, deren `platforms` weder `ios` noch `android` enthalten, wird abgewiesen (`403`), ebenso eine App
   einer Plattform, die nicht eingetragen ist (`403 platform_not_allowed`).

```sh
curl -X PATCH https://support.example.com/v1/analytics/properties/PROPERTY_ID \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "platforms": ["ios", "android"] }'
```

`YuvaAnalyticsConfiguration` nimmt weiterhin den Schlüssel der eigenen Analytics-Property der App; richten Sie sie ein
wie in [Analytics in Apps](analytics-apps.md).

## Swift

Übergeben Sie beim Konfigurieren der Bibliothek `requests` und geben Sie der URL-Session, die mit Ihrer API spricht,
einen `YuvaRequestDelegate`:

```swift
YuvaAnalytics.configure(
    YuvaAnalyticsConfiguration(
        serverURL: URL(string: "https://support.example.com")!,
        key: "yuva_ak_xxxxxxxx",
        requests: YuvaRequestMonitoring(
            key: "yuva_ak_yyyyyyyy",
            hosts: ["api.example.com"],
            routes: ["/users/{id}", "/users/me", "/orders/{id}/items/{item}"],
            apiVersion: "v2"
        )
    ))

let api = URLSession(configuration: .default, delegate: YuvaRequestDelegate(), delegateQueue: nil)
```

Eine Session, die schon einen Delegate hat, ruft stattdessen `YuvaAnalytics.recordRequest(task, metrics: metrics)`
aus `urlSession(_:task:didFinishCollecting:)` auf. Ein einzelner Task kann `task.delegate = YuvaRequestDelegate()`
bekommen. Die Dauer läuft vom Beginn des Abrufs bis zum Ende der Antwort.

## Kotlin

Die Bibliothek bringt OkHttp nicht mit; sie nutzt das der App. Übergeben Sie `requests` und fügen Sie den Interceptor
dem Client hinzu, der mit Ihrer API spricht:

```kotlin
YuvaAnalytics.configure(
    this,
    YuvaAnalyticsConfiguration(
        serverUrl = "https://support.example.com",
        key = "yuva_ak_xxxxxxxx",
        requests = YuvaRequestMonitoring(
            key = "yuva_ak_yyyyyyyy",
            hosts = listOf("api.example.com"),
            routes = listOf("/users/{id}", "/users/me", "/orders/{id}/items/{item}"),
            apiVersion = "v2",
        ),
    ),
)

val api = OkHttpClient.Builder().addInterceptor(YuvaRequestInterceptor()).build()
```

Die Dauer läuft, bis der Antwort-Body zu Ende gelesen oder geschlossen ist; lesen oder schließen Sie also jeden Body.
Abgebrochene Aufrufe werden nicht erfasst.

## Optionen

`YuvaRequestMonitoring` hat in beiden Bibliotheken dieselben Parameter.

| Parameter | Standard | Bedeutung |
|---|---|---|
| `key` | | Der öffentliche Schlüssel der API-Property. |
| `hosts` | | Die Hostnamen Ihrer API. Nur Aufrufe an sie werden erfasst; der Port wird ignoriert. |
| `routes` | keine | Die Muster Ihrer API, etwa `/users/{id}`. `{name}` passt auf ein Pfadsegment. |
| `route` | keine | Eine Funktion, die die Route einer Anfrage selbst benennt; ihre Antwort geht `routes` vor. |
| `sendUnmatchedPaths` | `false` | Sendet den Pfad eines Aufrufs, der zu keiner Route passt; das Muster ermittelt der Server. |
| `apiVersion` | keine | Die API-Version, die die App aufruft, für die Aufschlüsselung nach API-Version. |
| `environment` | `production` | Unterscheidet `production`, `staging` und Ähnliches. |

Die Route eines Aufrufs ist die Antwort von `route`, sonst die spezifischste aus `routes` (die mit den meisten festen
Segmenten). Ein Aufruf ohne Route wird übersprungen; ist `sendUnmatchedPaths` an, geht sein Pfad ohne Query an den
Server, der Kennungen zu `{id}` macht, wie unter [Routen](analytics-api.md#routen) beschrieben. Listen Sie lieber die
Routen auf, die Sie kennen, statt Pfade zu senden: So verlässt nichts das Gerät, was Ihr Code nicht benannt hat.

## Was erfasst wird

Pro Aufruf: die Methode, die Route, der Status (`0`, wenn keine Antwort kam, etwa bei Timeout oder ohne Netz), die
Dauer, die Größen von Request- und Response-Body und der Beginn des Aufrufs. Nie die URL, die Query, Header oder
Bodys. Die Records tragen keinen Consumer, keinen Service und keine Adresse; ihr Land stammt aus der Adresse der
Verbindung, die der Server kürzt und sofort verwirft. Sie erscheinen in denselben Ansichten und `dataset: api`-Abfragen
wie Server-Records, mit Plattform und Version der App.

- **Einwilligung und Opt-out** folgen den Analytics der App. Mit `requireConsent` wird vor `consent(true)` nichts
  erfasst, und jeder Record trägt dann die Einwilligung des Nutzers, die eine Property mit **Collection**
  **Only with consent** verlangt. Nach `optOut()` wird nichts erfasst, und was wartet, wird gelöscht. Ein Request mit
  `Sec-GPC: 1` oder `DNT: 1` wird angenommen und zählt nichts. Siehe [Einwilligung und Opt-out](analytics-apps.md#einwilligung-und-opt-out).
- **Warteschlange.** Records warten im privaten Speicher der App, bis der Server geantwortet hat: höchstens 1.000,
  keiner älter als 72 Stunden, in Batches zu 100 Records und 64 KiB, mit den Wiederholungen der
  [Offline-Warteschlange](analytics-apps.md#offline-warteschlange). `YuvaAnalytics.queuedRequests()` liefert, wie viele
  warten (in Swift mit `await`; in Kotlin eine `suspend`-Funktion).
- **Rate.** Eine Adresse darf einer Property 600 Records pro Minute senden; mehr wird mit `429` beantwortet.

Die Bibliotheken senden für Sie an `POST /client/v1/analytics/{key}/requests`. Sie rufen es nicht selbst auf.

## Antworten für die Stores

Ergänzen Sie bei eingeschaltetem Request-Timing die Antworten aus [Analytics in Apps](analytics-apps.md#antworten-für-die-stores).
Die Records tragen keine Kennung.

### App Store

Das Privacy Manifest der Bibliothek führt **Performance Data** bereits auf: nicht mit der Identität des Nutzers
verknüpft, nicht für Tracking genutzt, für **Analytics** und **App Functionality**. Antworten Sie in App Store Connect
auch für **Performance Data** mit **Data Not Linked to You**.

### Google Play

Ergänzen Sie unter **Data safety**:

| Datentyp | Erhoben | Weitergegeben | Hinweise |
|---|---|---|---|
| App-Informationen und Leistung → Diagnose | Ja | Nein | Zwecke: Analytics und App-Funktionen. Nicht kurzzeitig verarbeitet, weil der Server aggregierte Zahlen speichert. Für Nutzer optional, wenn Sie `requireConsent` setzen. |

# Analytics in Apps

Die Swift- und Kotlin-Bibliotheken zählen Screens, Events und Umsatz Ihrer iOS- und Android-Apps in derselben
Analytics-Property wie Ihre Website, ohne Cookies, Werbe-ID oder irgendetwas, das einen Menschen von einem
Start zum nächsten verfolgt. Diese Seite baut auf [Analytics](analytics.md) auf; legen Sie die Property
zuerst dort an.

## Die Plattform einschalten

Apps senden ohne Domain; eine Property zählt sie deshalb nur für die Plattformen, die Sie eingeschaltet haben.

1. Haken Sie in **Settings → General** der Property unter **Platforms** **iOS**, **Android** oder beide an.
2. Kopieren Sie den Schlüssel der Property (`yuva_ak_…`) aus **Settings → Install**. Die Tabs **Swift** und
   **Kotlin** dort enthalten den Code unten, für Ihren Server ausgefüllt.

Eine Anfrage von einer Plattform ohne Haken wird abgelehnt (`403 platform_not_allowed`) und unter
[Test events](#prüfen-ob-events-ankommen) als **Refused** gezählt. Beim Anlegen einer Property verlangen
Panel und API mindestens eine Domain, auch wenn nur Apps senden.

## Swift

Erfordert iOS 17 oder macOS 14. Fügen Sie das Paket in Xcode (**File → Add Package Dependencies**) oder in
`Package.swift` hinzu und nutzen Sie das Produkt `YuvaAnalytics`. `YuvaKit` brauchen Sie nicht.

```swift
.package(url: "https://github.com/productdevbook/yuva", exact: "0.0.1"),
.product(name: "YuvaAnalytics", package: "yuva"),
```

Nehmen Sie die Version Ihres Servers. Konfigurieren Sie einmal beim Start, auf dem Main Actor:

```swift
import SwiftUI
import YuvaAnalytics

@main
struct MyApp: App {
    init() {
        YuvaAnalytics.configure(YuvaAnalyticsConfiguration(
            serverURL: URL(string: "https://support.example.com")!,
            key: "yuva_ak_xxxxxxxx"
        ))
    }

    var body: some Scene {
        WindowGroup {
            HomeView()
                .yuvaScreen("Home")
        }
    }
}
```

| Parameter | Bedeutung |
|---|---|
| `serverURL` | Ihr Yuva-Server. |
| `key` | Der Schlüssel der Property. |
| `appId` | Benennt die App in Screen-Adressen. Ohne Angabe Ihre Bundle-ID. |
| `requireConsent` | Standardmäßig `false`. Siehe [Einwilligung und Opt-out](#einwilligung-und-opt-out). |
| `debug` | Standardmäßig `false`. Protokolliert, was eingereiht, gesendet und verworfen wird. |

### Screens

`.yuvaScreen("Home")` zählt einen Screen jedes Mal, wenn die View erscheint. In UIKit rufen Sie `screen`
selbst auf:

```swift
override func viewDidAppear(_ animated: Bool) {
    super.viewDidAppear(animated)
    YuvaAnalytics.screen("Settings")
}
```

Ein Screen ist ein Seitenaufruf von `app://<App-ID>/<Screen>`, etwa `app://com.example.shop/Product%20Detail`.
Die App-ID steht dort als Host und wird nicht gegen die Domains der Property geprüft. Führende und
abschließende Schrägstriche des Namens entfallen, der Rest wird prozentcodiert. Nehmen Sie Namen, die für
jeden Nutzer gleich bleiben (`Product`, nicht `Product 4711`): Die Zahlen werden nach Screen gruppiert.

### Events und Umsatz

```swift
YuvaAnalytics.track("Purchase", props: ["plan": "pro", "seats": 3], revenue: YuvaRevenue(amount: 9.99, currency: "EUR"))
YuvaAnalytics.track("Tapped help", interactive: false)
```

Ein Event gehört zum zuletzt gezählten Screen, vor dem ersten zu `app://<App-ID>/`. Die Regeln für Namen,
Props und Umsatz sind die aus [Events, Goals und Funnels](analytics-events.md): bis zu 30 Props mit Text-,
Zahl- oder Boolean-Werten; `screen`, `pageview`, `engagement` und Namen mit `$` oder `yuva_` am Anfang sind
reserviert. `interactive: false` lässt einen Besuch, der nur dieses Event hat, ein Bounce bleiben.

## Kotlin

Erfordert Android 8.0 (API 26). Die Bibliothek liegt auf JitPack. Fügen Sie das Repository in
`settings.gradle.kts` hinzu:

```kotlin
dependencyResolutionManagement {
    repositories {
        google()
        mavenCentral()
        maven("https://jitpack.io")
    }
}
```

und die Abhängigkeit mit dem Release-Tag Ihres Servers:

```kotlin
implementation("com.github.productdevbook:yuva-analytics:v0.0.1")
```

Konfigurieren Sie einmal in `Application.onCreate`:

```kotlin
import dev.yuva.analytics.YuvaAnalytics
import dev.yuva.analytics.YuvaAnalyticsConfiguration

class MyApp : Application() {
    override fun onCreate() {
        super.onCreate()
        YuvaAnalytics.configure(
            this,
            YuvaAnalyticsConfiguration(serverUrl = "https://support.example.com", key = "yuva_ak_xxxxxxxx"),
        )
    }
}
```

`YuvaAnalyticsConfiguration` nimmt `serverUrl`, `key`, `appId` (ohne Angabe Ihr Paketname), `requireConsent`
und `debug` mit der Bedeutung oben. Debug-Meldungen gehen unter dem Tag `YuvaAnalytics` nach Logcat.

### Screens

`TrackScreen` zählt einen Compose-Screen, wenn sein Name sich ändert oder er in die Komposition kommt. Mit
Navigation Compose übergeben Sie das Routenmuster, damit `product/{id}` ein einziger Screen bleibt:

```kotlin
import dev.yuva.analytics.TrackScreen

@Composable
fun AppNavigation(navController: NavHostController) {
    val entry by navController.currentBackStackEntryAsState()
    entry?.destination?.route?.let { TrackScreen(it) }
    NavHost(navController, startDestination = "home") { /* ... */ }
}
```

Außerhalb von Compose rufen Sie `YuvaAnalytics.screen("Settings")` auf. Screen-Adressen werden wie in Swift
gebildet.

### Events und Umsatz

```kotlin
YuvaAnalytics.track(
    "Purchase",
    props = mapOf("plan" to "pro", "seats" to 3),
    revenue = YuvaRevenue(BigDecimal("9.99"), "EUR"),
)
YuvaAnalytics.track("Tapped help", interactive = false)
```

Ein Prop-Wert, der weder Text noch Zahl noch Boolean ist, wird als Text gesendet. Die Regeln sind dieselben
wie in Swift.

## Einwilligung und Opt-out

Beide Bibliotheken arbeiten gleich.

- **Ohne `requireConsent`** wird alles gezählt, bis der Nutzer aussteigt. Das passt zu einer Property, deren
  **Collection** **Without cookies, for everyone** ist (der Standard).
- **Mit `requireConsent: true`** wird nichts eingereiht, bis Sie `consent(true)` aufrufen; Events davor werden
  verworfen, nicht aufgehoben. Jedes Event trägt dann die Einwilligung des Nutzers, die eine Property mit
  **Collection** **Only with consent** verlangt. Die Entscheidung wird nicht gemerkt: Speichern Sie sie selbst
  und rufen Sie nach `configure` bei jedem Start, bei dem der Nutzer zugestimmt hat, erneut `consent(true)`
  auf. `consent(false)` hört auf zu zählen und löscht alles, was auf das Senden wartet.
- **Consent-Funktionen.** `consent(true, persistent: true)`, `identify` und `persistentId` geben zustimmenden Nutzern eine
  13-Monate-ID und verknüpfen ihre Besuche mit einem Kontakt: siehe [Consent-Funktionen](analytics-consent.md#apps).
- **`optOut()`** hört dauerhaft auf zu zählen: Es wird auf dem Gerät gemerkt, löscht alles, was auf das Senden
  wartet, und `configure` beachtet es bei jedem späteren Start. **`optIn()`** macht es rückgängig,
  `isOptedOut` sagt, welcher Zustand gilt. Bieten Sie das in Ihren Einstellungen an.
- **`flush()`** sendet, was wartet, sofort. Die Bibliotheken senden außerdem wenige Sekunden nach einem Event,
  wenn die App in den Vordergrund zurückkehrt und wenn sie in den Hintergrund geht.

## Was gesendet und was gespeichert wird

Jede Anfrage trägt einen zufälligen Schlüssel, den die App pro Start erzeugt und nur im Speicher hält, die
Plattform der App (`ios` oder `android`), ihre Version, die Betriebssystemversion, eine Geräteklasse (Mobile,
Tablet oder Desktop), die Bildschirmbreite, die Sprache und die Events mit der Zeit, zu der sie geschahen.
Es gibt keine Geräte-ID, keine Werbe-ID und kein Cookie, und nichts wird auf das Gerät geschrieben, um den
Nutzer später wiederzuerkennen. Der Server kürzt die IP-Adresse der Verbindung bei der Ankunft, nutzt sie
für den Ort und speichert sie nicht ([Analytics und Datenschutz](analytics-privacy.md)).

- **Ein Start ist ein Besuch und ein Besucher.** Der Schlüssel wechselt mit jedem Start; wer die App an einem
  Tag zweimal öffnet, zählt also als zwei Besucher. Die 30-Minuten-Lücke wie im Web beendet einen Besuch
  ebenfalls. Lesen Sie die Besucher einer App als Starts des Tages.
- **Die Swift-Bibliothek meldet iOS** für jede Apple-Plattform, auf der sie läuft, auch für eine Mac-App;
  haken Sie dafür **iOS** an. Die Geräteklasse ist dort `desktop`.

### Offline-Warteschlange

Events werden vor dem Senden in den privaten Speicher der App geschrieben, aus Backups ausgenommen, und
verlassen ihn erst, wenn der Server geantwortet hat.

| | |
|---|---|
| Größe | Höchstens 1.000 Events warten. Darüber werden die ältesten verworfen. |
| Alter | Ein Event, das älter als 72 Stunden ist, wird gelöscht, und der Server verwirft es ebenfalls (gezählt als **Too late**). |
| Zeit | Ein Event behält die Zeit, zu der es geschah, und wird an diesem Tag gespeichert. |
| Batches | Bis zu 100 Events und 64 KiB pro Anfrage. |
| Wiederholungen | Ein Netzwerkfehler oder eine `5xx`-Antwort wird nach 5 Sekunden wiederholt, dann nach 10, verdoppelt bis höchstens 5 Minuten. `429` und `503` warten auf das `Retry-After` des Servers (eine Minute, wenn es fehlt). Jede andere Antwort, `2xx` oder `4xx`, nimmt den Batch aus der Warteschlange. |

Was beim Schließen der App noch wartet, wird beim nächsten Start gesendet, unter dem Schlüssel des Starts,
der es erzeugt hat.

### Prüfen, ob Events ankommen

Öffnen Sie **Settings → Test events** der Property und benutzen Sie dann die App. Die Seite aktualisiert sich
alle 5 Sekunden und zeigt, ab dem Moment, in dem Sie sie geöffnet haben, was der Server **Counted** hat und
was er verworfen hat: **Too late**, **Invalid**, **Refused** (eine Plattform ohne Haken) und **Other drops**.
Screens erscheinen innerhalb einer Minute in **Realtime**. In der Ansicht **Devices** schlüsseln **Platform**
und **App version** die Zahlen auf, und Sie können danach filtern.

## Antworten für die Stores

Das sind die Antworten für das, was die Bibliothek selbst sendet. Ergänzen Sie, was Ihre eigenen Props und der
Rest Ihrer App erheben.

### App Store

Die Bibliothek bringt ein Privacy Manifest (`PrivacyInfo.xcprivacy`) mit, das Xcode in den
Datenschutzbericht Ihrer App übernimmt. Es erklärt kein Tracking und keine Tracking-Domains und zwei Arten
erhobener Daten, beide **nicht mit der Identität des Nutzers verknüpft**, **nicht für Tracking genutzt** und für
**Analytics** erhoben: **Produktinteraktion** (Screens und Events) und **Ungefährer Standort** (der Ort, den der
Server aus der gekürzten IP-Adresse ermittelt). Außerdem erklärt es den `UserDefaults`-Zugriff, den es braucht,
um ein Opt-out zu merken, mit dem Grund `CA92.1`.

Antworten Sie in App Store Connect für Product Interaction und Coarse Location mit **Data Not Linked to You**
und für Tracking mit **No**. Die einzige Kennung ist der zufällige Schlüssel eines Starts.

### Google Play

Das Manifest der Bibliothek entfernt die Berechtigung `AD_ID`; sie fügt also keine Werbe-ID hinzu. Unter
**Data safety**:

| Datentyp | Erhoben | Weitergegeben | Hinweise |
|---|---|---|---|
| App-Aktivität → App-Interaktionen | Ja | Nein | Screens und Events. Zweck: Analytics. |
| Standort → Ungefährer Standort | Ja | Nein | Bei der Ankunft aus der IP-Adresse ermittelt, die gekürzt und nicht gespeichert wird: kurzzeitig verarbeitet. Zweck: Analytics. |

Die Daten gehen nur an den Yuva-Server, den Sie konfiguriert haben. Geben Sie an, dass sie bei der Übertragung
verschlüsselt sind, wenn `serverURL` mit `https://` beginnt.

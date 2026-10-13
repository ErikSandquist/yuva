# Analytics in apps

The Swift and Kotlin libraries count screens, events and revenue of your iOS and Android apps in the same
analytics property as your website, without cookies, advertising ids or anything that follows a person from
one launch to the next. This builds on [Analytics](analytics.md); add the property there first.

## Turn on the platform

Apps send without a domain, so a property counts them only for the platforms you turned on.

1. Open the property's **Settings → General** and tick **iOS**, **Android** or both under **Platforms**.
2. Copy the property's key (`yuva_ak_…`) from **Settings → Install**, where the **Swift** and **Kotlin**
   tabs have the code below filled in for your server.

A request from a platform that is not ticked is refused (`403 platform_not_allowed`) and counted as
**Refused** under [Test events](#check-that-events-arrive). The panel and the API still ask for at least one
domain when a property is made, even if only apps send to it.

## Swift

Requires iOS 17 or macOS 14. Add the package in Xcode (**File → Add Package Dependencies**) or in
`Package.swift`, and use the `YuvaAnalytics` product. It does not need `YuvaKit`.

```swift
.package(url: "https://github.com/productdevbook/yuva", exact: "0.0.1"),
.product(name: "YuvaAnalytics", package: "yuva"),
```

Use the version of your server. Configure it once at launch, on the main actor:

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

| Parameter | Meaning |
|---|---|
| `serverURL` | Your Yuva server. |
| `key` | The property's key. |
| `appId` | Names the app in screen addresses. Your bundle identifier when left out. |
| `requireConsent` | `false` by default. See [Consent and opt-out](#consent-and-opt-out). |
| `debug` | `false` by default. Logs what is queued, sent and dropped. |

### Screens

`.yuvaScreen("Home")` counts a screen each time the view appears. In UIKit, call `screen` yourself:

```swift
override func viewDidAppear(_ animated: Bool) {
    super.viewDidAppear(animated)
    YuvaAnalytics.screen("Settings")
}
```

A screen is a page view of `app://<app id>/<screen>`, such as `app://com.example.shop/Product%20Detail`. The
app id is shown as the host, and it is not checked against the property's domains. Leading and trailing slashes
are removed from the name and the rest is percent-encoded. Use names that stay the same for every user
(`Product`, not `Product 4711`): the screen is what the numbers group by.

### Events and revenue

```swift
YuvaAnalytics.track("Purchase", props: ["plan": "pro", "seats": 3], revenue: YuvaRevenue(amount: 9.99, currency: "EUR"))
YuvaAnalytics.track("Tapped help", interactive: false)
```

An event belongs to the screen counted last, or to `app://<app id>/` before the first one. The rules for names,
props and revenue are those of [Events, goals and funnels](analytics-events.md): up to 30 props whose values are
text, numbers or booleans; `screen`, `pageview`, `engagement` and names starting with `$` or `yuva_` are
reserved. `interactive: false` keeps a visit that has only this event a bounce.

## Kotlin

Requires Android 8.0 (API 26). The library is on JitPack. Add the repository in `settings.gradle.kts`:

```kotlin
dependencyResolutionManagement {
    repositories {
        google()
        mavenCentral()
        maven("https://jitpack.io")
    }
}
```

and the dependency, with the release tag of your server:

```kotlin
implementation("com.github.productdevbook:yuva-analytics:v0.0.1")
```

Configure it once, in `Application.onCreate`:

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

`YuvaAnalyticsConfiguration` takes `serverUrl`, `key`, `appId` (your package name when left out),
`requireConsent` and `debug`, with the meaning above. Debug messages go to Logcat under the tag `YuvaAnalytics`.

### Screens

`TrackScreen` counts a Compose screen each time its name changes or it enters the composition. With Navigation
Compose, hand it the route pattern, so `product/{id}` stays one screen:

```kotlin
import dev.yuva.analytics.TrackScreen

@Composable
fun AppNavigation(navController: NavHostController) {
    val entry by navController.currentBackStackEntryAsState()
    entry?.destination?.route?.let { TrackScreen(it) }
    NavHost(navController, startDestination = "home") { /* ... */ }
}
```

Outside Compose, call `YuvaAnalytics.screen("Settings")`. Screen addresses are built as in Swift.

### Events and revenue

```kotlin
YuvaAnalytics.track(
    "Purchase",
    props = mapOf("plan" to "pro", "seats" to 3),
    revenue = YuvaRevenue(BigDecimal("9.99"), "EUR"),
)
YuvaAnalytics.track("Tapped help", interactive = false)
```

A prop value that is not text, a number or a boolean is sent as text. The rules are the same as in Swift.

## Consent and opt-out

Both libraries work the same way.

- **Without `requireConsent`**, everything is counted until the user opts out. That is right for a property
  whose **Collection** is **Without cookies, for everyone** (the default).
- **With `requireConsent: true`**, nothing is queued until you call `consent(true)`; events before that are
  dropped, not kept. Each event then carries the user's consent, which a property whose **Collection** is
  **Only with consent** requires. The choice is not remembered: store it yourself and call `consent(true)`
  again after `configure` on every launch where the user has agreed. `consent(false)` stops counting and
  deletes whatever waits to be sent.
- **Consent features.** `consent(true, persistent: true)`, `identify` and `persistentId` give consenting users a
  13-month id and link their visits to a contact: see [Consent features](analytics-consent.md#apps).
- **`optOut()`** stops counting for good: it is remembered on the device, deletes whatever waits to be sent,
  and `configure` honours it on every later launch. **`optIn()`** reverses it, and `isOptedOut` tells which it
  is. Offer this in your settings screen.
- **`flush()`** sends what waits now. The libraries also send a few seconds after an event, when the app
  returns to the foreground and when it goes to the background.

## What is sent, and what is kept

Each request carries a random key that the app makes for the launch and holds in memory only, the app's
platform (`ios` or `android`), its version, the operating system version, a device class (mobile, tablet or
desktop), the screen width, the language, and the events with the time they happened. There is no device id,
no advertising id and no cookie, and nothing is written to the device to recognise the user later. The server
shortens the connection's IP address on arrival, uses it for the place, and does not store it
([Analytics and privacy](analytics-privacy.md)).

- **A launch is a visit, and a visitor.** The key changes with every launch, so someone who opens the app
  twice in a day is counted as two visitors. The same 30-minute gap rule as on the web ends a visit too. Read
  an app's visitors as launches that day.
- **The Swift library reports iOS** for every Apple platform it runs on, a Mac app included; tick **iOS** for it.
  Its device class is `desktop` there.

### Offline queue

Events are written to the app's private storage, kept out of backups, before they are sent, and leave only
when the server has answered.

| | |
|---|---|
| Size | At most 1,000 events wait. Beyond that the oldest are dropped. |
| Age | An event older than 72 hours is deleted, and the server drops it too (counted as **Too late**). |
| Time | An event keeps the time it happened, and is stored on that day. |
| Batches | Up to 100 events and 64 KiB per request. |
| Retries | A network error or a `5xx` answer is retried after 5 seconds, then 10, doubling up to 5 minutes. `429` and `503` wait for the server's `Retry-After` (a minute when there is none). Any other answer, `2xx` or `4xx`, removes the batch. |

What is still queued when the app is closed is sent at the next launch, under the key of the launch that
made it.

### Check that events arrive

Open **Settings → Test events** of the property, then use the app. The page refreshes every 5 seconds and
counts, from the moment you opened it, what the server **Counted** and what it dropped: **Too late**,
**Invalid**, **Refused** (a platform that is not ticked) and **Other drops**. Screens show up in **Realtime**
within a minute. In the **Devices** view, **Platform** and **App version** break the numbers down, and
you can filter by them.

## Answers for the stores

These are the answers for what the library itself sends. Add what your own props and the rest of your app
collect.

### App Store

The library bundles a privacy manifest (`PrivacyInfo.xcprivacy`) that Xcode merges into your app's privacy
report. It declares no tracking and no tracking domains, and two kinds of collected data, each **not linked to
the user's identity**, **not used for tracking** and collected for **analytics**: **Product interaction**
(screens and events) and **Coarse location** (the place the server finds from the shortened IP address). It
also declares the `UserDefaults` access it needs to remember an opt-out, with reason `CA92.1`.

In App Store Connect, answer **Data Not Linked to You** for Product Interaction and Coarse Location, and **No**
to tracking. The only identifier is the random key of one launch.

### Google Play

The library's manifest removes the `AD_ID` permission, so it adds no advertising id. In **Data safety**:

| Data type | Collected | Shared | Notes |
|---|---|---|---|
| App activity → App interactions | Yes | No | Screens and events. Purpose: Analytics. |
| Location → Approximate location | Yes | No | Found from the IP address on arrival, which is shortened and not stored: processed ephemerally. Purpose: Analytics. |

Data goes only to the Yuva server you configured. Say that it is encrypted in transit if `serverURL` starts
with `https://`.

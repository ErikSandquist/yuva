# API monitoring from iOS and Android apps

The Swift and Kotlin libraries can time the calls your app makes to your own API and add them to an API
property, next to what your server reports. You see the API as your users do: the time includes their network,
and calls that never got an answer are counted too. You need an API property: see
[Make an API property](analytics-api.md#make-an-api-property). This builds on
[Analytics in apps](analytics-apps.md); it is off until you turn it on.

## Allow the app

1. Take the API property's public key (`yuva_ak_…`) from the `public_key` field of
   `GET /v1/analytics/properties`. It is public, like the key of a site property, and only works for the calls
   described here.
2. Allow the platform on the property (scope `analytics:manage`, owners and admins). A key of an API property
   that has neither `ios` nor `android` in its `platforms` is refused (`403`), and so is an app of a platform
   that is not listed (`403 platform_not_allowed`).

```sh
curl -X PATCH https://support.example.com/v1/analytics/properties/PROPERTY_ID \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "platforms": ["ios", "android"] }'
```

`YuvaAnalyticsConfiguration` still takes the key of the app's own analytics property, set up as in
[Analytics in apps](analytics-apps.md).

## Swift

Pass `requests` when you configure the library, and give the URL session that talks to your API a
`YuvaRequestDelegate`:

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

A session that already has a delegate calls `YuvaAnalytics.recordRequest(task, metrics: metrics)` from
`urlSession(_:task:didFinishCollecting:)` instead. A single task can take `task.delegate = YuvaRequestDelegate()`.
The duration runs from the start of the fetch to the end of the response.

## Kotlin

The library does not bring OkHttp; it uses the one your app has. Pass `requests` and add the interceptor to the
client that talks to your API:

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

The duration runs until the response body is read to its end or closed, so read or close every body. Canceled
calls are not recorded.

## Options

`YuvaRequestMonitoring` takes the same parameters in both libraries.

| Parameter | Default | Meaning |
|---|---|---|
| `key` | | The API property's public key. |
| `hosts` | | The host names of your API. Only calls to these are recorded; the port is ignored. |
| `routes` | none | The patterns of your API, such as `/users/{id}`. `{name}` matches one path segment. |
| `route` | none | A function that names the route of a request itself; its answer wins over `routes`. |
| `sendUnmatchedPaths` | `false` | Sends the path of a call that matches no route, so the server works out the pattern. |
| `apiVersion` | none | The API version the app calls, for the API version breakdown. |
| `environment` | `production` | Tells apart `production`, `staging` and the like. |

The route of a call is the answer of `route`, else the most specific of `routes` (the one with the most fixed
segments). A call that maps to no route is skipped, unless `sendUnmatchedPaths` is on: then its path, without
the query, goes to the server, which turns ids into `{id}` as described under [Routes](analytics-api.md#routes).
List the routes you know rather than sending paths: nothing then leaves the device that your code did not name.

## What is recorded

Per call: the method, the route, the status (`0` when no response arrived, such as a timeout or no network),
the duration, the sizes of the request and response bodies, and the time the call started. Never the URL,
the query, headers or bodies. The records carry no consumer, service or address; their country comes from the
connection's address, which the server shortens and drops at once. They show up in the same views and
`dataset: api` queries as server records, with the app's platform and version.

- **Consent and opt-out** follow the app's analytics. With `requireConsent`, nothing is recorded before
  `consent(true)`, and each record then carries the user's consent, which a property whose **Collection** is
  **Only with consent** requires. After `optOut()` nothing is recorded and what waits is deleted. A request with
  `Sec-GPC: 1` or `DNT: 1` is accepted and counts nothing. See [Consent and opt-out](analytics-apps.md#consent-and-opt-out).
- **Queue.** Records wait in the app's private storage until the server has answered: up to 1,000, none older
  than 72 hours, in batches of 100 records and 64 KiB, with the retries of the [offline queue](analytics-apps.md#offline-queue).
  `YuvaAnalytics.queuedRequests()` returns how many are waiting (`await` it in Swift; it is a `suspend`
  function in Kotlin).
- **Rate.** One address may send 600 records a minute to a property; more are answered `429`.

The libraries send to `POST /client/v1/analytics/{key}/requests` for you. You do not call it yourself.

## Answers for the stores

With request timing on, add this to the answers in [Analytics in apps](analytics-apps.md#answers-for-the-stores).
The records carry no identifier.

### App Store

The privacy manifest of the library already lists **Performance Data**, not linked to the user's identity, not
used for tracking, for **Analytics** and **App Functionality**. In App Store Connect, answer **Data Not Linked to
You** for **Performance Data** as well.

### Google Play

In **Data safety**, add:

| Data type | Collected | Shared | Notes |
|---|---|---|---|
| App info and performance → Diagnostics | Yes | No | Purposes: Analytics and App functionality. Not processed ephemerally, because the server keeps aggregated counts. Optional for users when you set `requireConsent`. |

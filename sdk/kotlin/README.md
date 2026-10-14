# Yuva for Android

## Push notifications

Push stays with your app. Yuva sends a `message.created` webhook to your backend; when
`data.contact.online` is false, your backend sends the push itself and puts the conversation's id
(`data.message.conversation_id`) in the FCM message as the `data` entry `yuva_conversation_id`:

```json
{
  "message": {
    "token": "<device registration token>",
    "notification": { "title": "Acme Support", "body": "We have shipped a fix." },
    "data": { "yuva_conversation_id": "0192f0c4-6f1e-7b2a-9c1d-3e4f5a6b7c8d" }
  }
}
```

When the user taps a notification, Android starts your launcher activity with the `data` entries
as intent extras; pass them to `Yuva.handleNotification` and open that conversation:

```kotlin
val extras = intent.extras?.keySet()?.associateWith { intent.extras?.getString(it).orEmpty() }.orEmpty()
val conversationId = Yuva.handleNotification(extras)
```

In `FirebaseMessagingService.onMessageReceived`, `Yuva.handleNotification(message.data)` works the
same way. `YuvaConversationsView(client, openConversationId = conversationId)` opens the thread.

## Timing calls to your own API

`yuva-analytics` can time the calls your app makes to its own API and send them to an API property
(its public key, with `android` in the property's platforms). It is off until you pass `requests`
and add the OkHttp interceptor; the module does not bring OkHttp itself.

```kotlin
YuvaAnalytics.configure(
    this,
    YuvaAnalyticsConfiguration(
        serverUrl = yuvaUrl,
        key = "yuva_ak_app…",
        requests = YuvaRequestMonitoring(
            key = "yuva_ak_api…",
            hosts = listOf("api.example.com"),
            routes = listOf("/users/{id}", "/users/me", "/orders/{id}/items/{item}"),
            apiVersion = "v2",
        ),
    ),
)

val api = OkHttpClient.Builder().addInterceptor(YuvaRequestInterceptor()).build()
```

- Only requests to `hosts` are recorded: method, route, status (0 when no response arrived), duration
  until the response body is read or closed, request and response body size and start time. Never
  the URL, query, headers or bodies. Canceled calls are not recorded.
- The route is the `route` lambda's answer, else the most specific of `routes` (`{name}` matches
  one path segment). A request that maps to no route is skipped, unless `sendUnmatchedPaths` is
  set; then its path, without query, goes to the server, which turns ids into `{id}`.
- Records follow the app's analytics: nothing before `consent(true)` when `requireConsent` is set,
  nothing after `optOut()`, which also clears the queue. They are queued on the device for up to 72
  hours and sent in batches of 100. `YuvaAnalytics.queuedRequests()` returns how many are waiting.

### Google Play data safety

With request timing on, declare **App info and performance › Diagnostics** as collected and not
shared, for **Analytics** and **App functionality**. It is not processed ephemerally (the server keeps
aggregated counts), and it is optional for users when `requireConsent` is set. The records carry no
identifier.

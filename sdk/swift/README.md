# YuvaKit

## Push notifications

Push stays with your app. Yuva sends a `message.created` webhook to your backend; when
`data.contact.online` is false, your backend sends the push itself and puts the conversation's id
(`data.message.conversation_id`) in the payload as `yuva_conversation_id`, a top-level key next to
`aps`:

```json
{
  "aps": {
    "alert": { "title": "Acme Support", "body": "We have shipped a fix." },
    "sound": "default"
  },
  "yuva_conversation_id": "0192f0c4-6f1e-7b2a-9c1d-3e4f5a6b7c8d"
}
```

When the user taps it, pass the notification's `userInfo` to `Yuva.handleNotification` and open
that conversation:

```swift
func userNotificationCenter(
    _ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse
) async {
    if let id = Yuva.handleNotification(response.notification.request.content.userInfo) {
        openConversationId = id
    }
}
```

`YuvaConversationsView(client:openConversationId:)` opens the thread when the binding is set.

## Timing calls to your own API

`YuvaAnalytics` can time the calls your app makes to its own API and send them to an API property
(its public key, with `ios` in the property's platforms). It is off until you pass `requests`:

```swift
YuvaAnalytics.configure(
    YuvaAnalyticsConfiguration(
        serverURL: yuvaURL, key: "yuva_ak_app…",
        requests: YuvaRequestMonitoring(
            key: "yuva_ak_api…", hosts: ["api.example.com"],
            routes: ["/users/{id}", "/users/me", "/orders/{id}/items/{item}"],
            apiVersion: "v2")))

let api = URLSession(configuration: .default, delegate: YuvaRequestDelegate(), delegateQueue: nil)
```

A session with its own delegate calls `YuvaAnalytics.recordRequest(task, metrics: metrics)` from
`urlSession(_:task:didFinishCollecting:)` instead, or sets `task.delegate = YuvaRequestDelegate()`
on single tasks.

- Only requests to `hosts` are recorded: method, route, status (0 when no response arrived), duration,
  request and response body size and start time. Never the URL, query, headers or bodies.
- The route is the first `route` closure answer, else the most specific of `routes` (`{name}`
  matches one path segment). A request that maps to no route is skipped, unless
  `sendUnmatchedPaths` is set; then its path, without query, goes to the server, which turns ids into
  `{id}`.
- Records follow the app's analytics: nothing before `consent(true)` when `requireConsent` is set,
  nothing after `optOut()`, which also clears the queue. They are queued on the device for up to 72
  hours and sent in batches of 100. `YuvaAnalytics.queuedRequests()` returns how many are waiting.
- The privacy manifest of `YuvaAnalytics` declares Performance Data (not linked, no tracking, for
  analytics and app functionality); add the same to your App Store privacy answers when you turn
  this on.

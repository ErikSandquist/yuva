---
title: "Support inside your app: Yuva's iOS and Android screens, redesigned"
description: "The Swift and Kotlin libraries now look like a messaging app on each platform: SwiftUI in the style of Messages on iOS, Material 3 Expressive on Android. Feedback shows where it stands, and a host app can show messages and feedback in two places."
date: 2026-10-18
author: productdevbook
---

Most people who write to a product's team do it from the product. On a phone that means the app, so Yuva ships native libraries for both platforms: [YuvaKit](/docs/mobile/#2-ios) for iOS in SwiftUI and the [Kotlin library](/docs/mobile/#3-android) for Android in Jetpack Compose. Each has a headless client you can drive from your own UI and ready screens on top of it.

Most apps use the ready screens. Until now they looked like a settings page: plain lists, square rows and hairlines between them. In [0.0.24](https://github.com/productdevbook/yuva/releases/tag/v0.0.24) we redesigned them to feel like the platform's own messaging app. Both platforms share the layout and behaviour but follow their own design language.

## Conversations

<figure class="shots">
  <img src="/blog/mobile/ios-list.webp" alt="Yuva's conversation list on iOS: the team name and reply time at the top, Messages and Feedback tabs, two conversations with avatars, times and an unread dot" width="540" height="1174" loading="lazy" />
  <img src="/blog/mobile/android-list.webp" alt="Yuva's conversation list on Android: a large Support title, connected Messages and Feedback buttons, conversations as rounded tiles and a New message button" width="540" height="1212" loading="lazy" />
  <figcaption>The conversation list on iOS (left) and Android (right).</figcaption>
</figure>

The list opens with your team's name and how fast it usually replies. Short waits show in minutes and longer ones in hours or days, so a team that answers within a day no longer shows "1440 minutes". Each row has an avatar, the subject, the last message, a short time and an unread mark.

On iOS the list is edge to edge, the way Messages draws it. On Android it follows Material 3 Expressive:

- Rows sit in rounded tiles under a large title that collapses as you scroll.
- Avatars take the expressive shapes.
- Messages and Feedback are connected pill buttons.
- New message is an extended button that shrinks while you scroll.

## Threads

<figure class="shots">
  <img src="/blog/mobile/ios-thread.webp" alt="A thread on iOS with blue bubbles for the user, grey bubbles for the team, an image, links and a jump-to-latest button with a dot" width="540" height="1174" loading="lazy" />
  <img src="/blog/mobile/android-thread.webp" alt="A thread on Android with a team avatar in the top bar, grouped bubbles, an image, underlined links and a floating composer" width="540" height="1212" loading="lazy" />
  <figcaption>A thread with an image, links and grouped replies.</figcaption>
</figure>

Messages from one sender are grouped into bubbles, with day separators between days. In the threads:

- Links in a message open.
- Images open full screen and can be zoomed.
- A bubble shows while the team is typing.
- If you are reading older messages when a reply arrives, the view stays where it is and a button with a dot takes you to the latest.
- Sending gives a light haptic.
- If a thread fails to load, it offers Try again instead of an empty screen.

On iOS 26 the composer floats in Liquid Glass. On Android it is a floating field whose send button springs in once there is text.

## Feedback that shows where it stands

<figure class="shots">
  <img src="/blog/mobile/ios-progress.webp" alt="A bug report on iOS with a red bug icon, an In progress tag and the line: The team marked this as in progress" width="540" height="1174" loading="lazy" />
  <img src="/blog/mobile/android-progress.webp" alt="The same bug report on Android with the category and an In progress tag in the top bar and the progress line in the thread" width="540" height="1212" loading="lazy" />
  <figcaption>The team marked a bug as in progress, and the person who reported it sees it.</figcaption>
</figure>

Feedback from an app used to vanish after you sent it. Now the team can tell the sender where it stands: **planned**, **in progress**, **done** or **won't do**.

- **In the panel:** a team member sets it from the conversation's header, or with the `O` shortcut. The list shows the state as a chip.
- **From an assistant:** the [MCP server](/docs/mcp/) has a `set_feedback_progress` tool.
- **Through the API:** `PATCH /v1/conversations/{id}` takes `feedback_progress`.

In the app, the state shows as a coloured tag in the Feedback list and at the top of the thread. Each change adds a line to the thread, and it arrives live while the screen is open.

## Sending feedback

<figure class="shots">
  <img src="/blog/mobile/ios-form.webp" alt="The feedback form on iOS: Bug, Idea, Praise and Other as coloured buttons, a message field, a screenshot picker and a reply by e-mail switch" width="540" height="1174" loading="lazy" />
  <img src="/blog/mobile/android-form.webp" alt="The feedback form on Android: large category tiles with expressive icon shapes, a message field, a screenshot button, a reply by e-mail switch and a Send button" width="540" height="1212" loading="lazy" />
  <figcaption>The feedback form.</figcaption>
</figure>

The form asks for a category, a message and an optional screenshot. People can ask for the reply by e-mail too. The app version, device, system version and language are attached, and the form says so. Category icons are vectors drawn by the library, not emoji, so they look the same on every device.

## Two places, or one

Not every app wants messages and feedback behind the same button. `YuvaConversationsView` takes a `section`:

```swift
YuvaConversationsView(client: yuva, section: .feedback)
```

```kotlin
YuvaConversationsView(client = yuva, section = YuvaConversationsSection.FEEDBACK)
```

With `.messages` the view shows only conversations. With `.feedback` it shows only feedback, and its + opens the feedback form. Leave `section` out and you get both as tabs, as before.

## Dark mode and your colours

<figure class="shots">
  <img src="/blog/mobile/ios-dark.webp" alt="A thread on iOS in dark mode" width="540" height="1174" loading="lazy" />
  <img src="/blog/mobile/android-dark.webp" alt="The conversation list on Android in dark mode with tiles on a dark container" width="540" height="1212" loading="lazy" />
  <figcaption>Dark mode on both platforms.</figcaption>
</figure>

The screens follow the system appearance. On Android the colours come from your app's `MaterialTheme`, dynamic colour included, so the screens match the rest of your app. Only the category and state tints are Yuva's own.

The Android library is built only from stable Material 3 components, with no experimental APIs. Adding Yuva will not force your app onto an alpha release of Compose.

Your app can also skip the ready screens and build its own UI on the headless client. The conversations, messages, events and feedback calls are the same either way.

## Upgrading

- **iOS:** update the `YuvaKit` package to `0.0.24`.
- **Android:** use `com.github.productdevbook:yuva:v0.0.24` from JitPack.

The [sample apps](https://github.com/productdevbook/yuva/tree/main/sdk) in the repository open every screen variant and have the new design too.

One change can break your build: in Swift, `YuvaEvent.conversationUpdated` gained a fifth associated value for the feedback progress. If you switch over events yourself, add it to the pattern.

The [mobile guide](/docs/mobile/) has the setup, push notifications and the full reference. Yuva is still pre-alpha, so try it with test data, and tell us what looks wrong in an [issue](https://github.com/productdevbook/yuva/issues), with a screenshot if you can.

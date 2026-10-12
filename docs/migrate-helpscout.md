# Move from Help Scout

Copy the conversations of a Help Scout account into Yuva and send its support address to Yuva. Yuva only
reads Help Scout. The overview is in [Move to Yuva](migrate.md).

## Before you start

- Create an app for Yuva. In Help Scout, open **Your Profile → My apps** and click **Create My App**. Name
  it "Yuva" and enter the **Redirect URL** that Yuva's Connect step shows as its redirection URL. Create it
  and copy its **App ID** and **App Secret**. Help Scout's page:
  [Authentication](https://developer.helpscout.com/mailbox-api/overview/authentication/).
- On the Connect step, enter the **App ID** and **App Secret** and press **Sign in with Help Scout**. Sign
  in as a user who sees every mailbox you want to move.

Help Scout has no read-only permission, so the app could change data. Yuva's connection is what keeps it
read-only: besides signing in, it sends Help Scout nothing but read requests, so no conversation, customer
or setting changes.

## What is copied

| Help Scout | Yuva |
|---|---|
| Conversations | Conversations, with all their messages |
| Customer threads | Messages in |
| Replies | Messages out |
| Chat and phone threads | Messages, in or out by who wrote them |
| Notes | Notes, which only your team sees |
| Status: active | Open |
| Status: pending | Pending |
| Status: closed | Closed |
| Tags | Labels, as you map them (or none) |
| Users | Team members, as you map them; others are kept by name |
| Assignee | The assignee, when the user is mapped |
| Customers | Contacts, with name and phone |
| Mailboxes | Inboxes, one per mailbox as you map them |
| Attachments | Files on the messages |

Spam, drafts and deleted conversations are never imported. Conversations take their original times and
arrive [quietly](migrate.md#how-history-stays-quiet).

## What is not copied

Workflows and their log lines, saved replies, satisfaction ratings, custom fields, Docs (the knowledge
base), Beacon settings and reports. Yuva copies only what the table above lists.

## Original messages

For an e-mail thread Yuva reads the original message, so headers, formatting and attachments arrive as
they were sent. Help Scout keeps original sources only for a while; once one is gone, Yuva uses the
thread's body from Help Scout's data instead, and fetches its attachments separately. A file larger than
the server's attachment limit stays behind.

## Limits

Help Scout limits requests per minute by plan, and all apps of an account share the limit. Yuva stays at
half of the limit Help Scout reports, and slows down when Help Scout asks it to. A large account therefore
takes a while. Help Scout's page:
[Rate limiting](https://developer.helpscout.com/mailbox-api/overview/rate-limiting/).

## The cutover

Today your support address reaches Help Scout, forwarded by the mail provider that hosts it or by a rule
there. On the Switch step, copy the channel's [forwarding address](email.md#forwarding-address) and change
that forwarding so the support address reaches Yuva instead. Until you do, Yuva reads Help Scout every two
minutes, so no conversation is lost. Replies sent from Yuva go out from the channel's address.

Then press **Send a test message**: it works when the message reaches Yuva by the new route. After a quiet
period, or when you press **Finish now**, the migration finishes
([how](migrate.md#the-test-message-the-quiet-period-and-finishing)).

## After it finishes

Delete the app you made for Yuva: in Help Scout, **Your Profile → My apps**. Yuva deletes its token when
the migration ends but cannot revoke it in Help Scout, so deleting the app is what ends its access. Keep
the forwarding as it is, and keep Help Scout as an archive if you like.

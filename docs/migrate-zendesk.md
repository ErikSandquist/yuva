# Move from Zendesk

Copy the tickets of a Zendesk Support account into Yuva and send its support address to Yuva. Yuva only
reads Zendesk. The overview is in [Move to Yuva](migrate.md).

## Before you start

- You need a Zendesk admin: only an admin can read every ticket. The plan must include API access.
- Create an OAuth client for Yuva. In Admin Center, open **Apps and integrations → APIs → OAuth clients**
  and click **Add OAuth client**. Name it "Yuva", choose the client kind **Confidential**, set the scope to
  `read` and enter the **Redirect URL** that Yuva's Connect step shows. Save it and copy the client
  identifier and the secret now: Zendesk shows the secret in full only once. Zendesk's page:
  [Using OAuth authentication with your application](https://support.zendesk.com/hc/en-us/articles/4408845965210-Using-OAuth-authentication-with-your-application).
- On the Connect step, enter your **Subdomain** (`acme` in `acme.zendesk.com`), the **Client identifier**
  and the **Client secret**, and press **Sign in with Zendesk**. Sign in as the admin and allow access.

Yuva asks for the scope `read` only and refuses a sign-in that grants more. Besides signing in, its
connection sends Zendesk nothing but read requests, so no ticket, user or setting changes.

## What is copied

| Zendesk | Yuva |
|---|---|
| Tickets | Conversations, with all their messages |
| Public comments | Messages: from the requester in, from an agent out |
| Private comments | Notes, which only your team sees |
| Status: new, open | Open |
| Status: pending, on-hold | Pending |
| Status: solved, closed | Closed |
| Tags | Labels, as you map them (or none) |
| Agents and admins | Team members, as you map them; others are kept by name |
| Assignee | The assignee, when the agent is mapped |
| Requesters | Contacts |
| Brands | Inboxes, one per brand as you map them |
| Attachments | Files on the messages ([below](#attachments)) |

Spam and deleted tickets are never imported. Conversations take their original times and arrive
[quietly](migrate.md#how-history-stays-quiet).

## What is not copied

Macros, automations, triggers and views; satisfaction ratings; the Help Center (articles and community);
custom ticket and user fields; organizations and groups; reports. Yuva copies only what the table above
lists.

## Attachments

Yuva fetches each comment's files. Zendesk keeps some on outside hosts; those are fetched without
Yuva's token, which goes only to your Zendesk account and to Zendesk's own file host. A file larger than the
server's attachment limit, or one that Zendesk no longer has, stays behind; the message is copied
without it.

## Limits

Zendesk limits requests per minute, by plan; its incremental export, which lists the tickets, allows 10 a
minute. Yuva stays at half: at most 5 export requests a minute, and half of the limit Zendesk reports for
the account for everything else. A large account therefore takes a while, and the copy slows down by itself
when Zendesk asks it to. Zendesk's pages:
[Incremental exports](https://developer.zendesk.com/api-reference/ticketing/ticket-management/incremental_exports/),
[Usage limits](https://developer.zendesk.com/api-reference/ticketing/account-configuration/usage_limits/).

## The cutover

Today your support address reaches Zendesk, forwarded by the mail provider that hosts it or by a rule
there. On the Switch step, copy the channel's [forwarding address](email.md#forwarding-address) and change
that forwarding so the support address reaches Yuva instead. Until you do, Yuva reads Zendesk every two
minutes, so no ticket is lost. Replies sent from Yuva go out from the channel's address.

Then press **Send a test message**: it works when the message reaches Yuva by the new route. After a quiet
period, or when you press **Finish now**, the migration finishes
([how](migrate.md#the-test-message-the-quiet-period-and-finishing)).

## After it finishes

Delete the OAuth client you made for Yuva: in Admin Center, **Apps and integrations → APIs → OAuth
clients**. Yuva deletes its token when the migration ends but cannot revoke it in Zendesk, so deleting the
client is what ends its access. Keep the forwarding as it is, and keep Zendesk as an archive if you like.

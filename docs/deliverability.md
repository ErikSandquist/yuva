# Deliverability

Whether your broadcasts reach the inbox depends mostly on the sending domain being set up properly
and on people not marking your mail as spam. Yuva has a check that tests the set-up of an e-mail
channel by sending it a test mail, and this page shows how to read it and how to fix what it finds.

You need an [e-mail channel](email.md) with an SMTP account that sends [broadcasts](broadcasts.md), and
access to the DNS of the domain it sends from.

## 1. Run the check

In the panel: Settings, the inbox, Where customers write from, open the e-mail channel, then "Check
deliverability and set up DKIM" in the Broadcast sending section. Press "Run a check".

Yuva sends a test mail through the channel's broadcast account to the channel's own address, with the
headers a broadcast carries, then reads how the receiving server judged it and looks up your DNS. The
result shows within a minute, and at most 10 minutes later. You can run six checks an hour per channel.
The test mail is never a conversation.

Run it before the first broadcast, and again after every change to DNS or to the SMTP account. The
broadcast review warns until a channel has been checked.

### Reading the result

| Line | What it tells you |
|---|---|
| SPF | Whether the server that sent the mail is allowed to by the SPF record. It also says whether the checked domain is your sending domain. If SPF passes only for your provider's domain, DMARC depends on DKIM alone, which is fine as long as DKIM passes. |
| DKIM | Whether a signature passed, for which domain, and whether that domain is your sending domain. It also says whether the signature covers `List-Unsubscribe` and `List-Unsubscribe-Post`, which Gmail and Yahoo ask for. |
| DMARC | Whether DMARC passed, and your policy: `none` (receivers only report), `quarantine` (failing mail goes to spam) or `reject`. Without a DMARC record you are told to publish one. |
| MX | Whether the domain of the channel's address has mail servers. Replies to a broadcast arrive there. |
| Yuva DKIM | Only when the channel has [its own key](#4-sign-with-yuva-dkim): whether DNS answers with it. |

The broadcast review uses the same result. It warns when the channel was never checked, the check did not
finish, DMARC is missing, DKIM does not match your sending domain or the unsubscribe headers are not
signed. It blocks the send when no DKIM signature passed.

If the check cannot read a result you see one of these:

| Message | What to do |
|---|---|
| Not back within 10 minutes (`timeout`) | Only DNS was checked. Make sure mail to the channel's address reaches Yuva (forwarding or MX), then check again. |
| Arrived without results from a trusted server (`unverifiable`) | The server's operator has not set the receiving server Yuva trusts ([`YUVA_INGRESS_AUTHSERV_ID`](configuration.md#inbound-e-mail)), so SPF, DKIM and DMARC could not be read. Only DNS was checked. |
| Could not be sent (`failed`) | Check the SMTP settings; the SMTP server's answer is shown. |

## 2. Set up the sending domain

These records go on the domain of the channel's From address. SPF, DKIM and DMARC must pass, and DMARC
needs SPF or DKIM to be for your own domain (aligned).

| Record | What to publish |
|---|---|
| SPF | One TXT record per name that includes your provider. |
| DKIM | The records your provider gives you, or [Yuva DKIM](#4-sign-with-yuva-dkim). |
| DMARC | `_dmarc.example.com TXT "v=DMARC1; p=none; rua=mailto:dmarc@example.com"`. Move to `p=quarantine` once the reports look right. |

Records take a while to spread. Run the check again when your provider says they are verified.

### Amazon SES

1. In the SES console, create a domain identity for your sending domain with Easy DKIM. SES shows three
   CNAME records; publish them. Verification can take up to 72 hours. Easy DKIM signs with your domain.
2. Set up a custom MAIL FROM domain so SPF passes for your own domain too: pick a subdomain of the
   identity that you use for nothing else (such as `bounce.example.com`), then publish one MX record
   (`10 feedback-smtp.<region>.amazonses.com`) and a TXT record (`"v=spf1 include:amazonses.com ~all"`)
   on it, with the values the console shows. The MX record must be the only one on that name.
3. A new SES account is in the sandbox: it can send only to verified addresses and 200 messages a day, at
   one a second. Request production access in the SES console (Account dashboard) before your first
   broadcast. Sandbox status is per region.
4. In the console, SMTP settings, "Create SMTP credentials". The user name and password it shows are not
   your AWS access keys, and they are for one region: use the SMTP endpoint of the same region. Save them
   when shown; they cannot be displayed again.
5. In the channel, switch on "Use a separate account for broadcasts" and enter the endpoint, port,
   encryption and these credentials.
6. For bounces and complaints, connect SNS as described under [Bounces](email.md#bounces).

### Postmark

1. In Postmark, add your sending domain and publish the DKIM and Return-Path
   records it shows.
2. Open the server's Settings and enable SMTP. The user name and password are the server API token
   (or the access key and secret key of an SMTP token).
3. In the channel, use the host `smtp-broadcasts.postmarkapp.com` for the broadcast stream
   (`smtp.postmarkapp.com` is the transactional one), port 587, 2525 or 25, with STARTTLS.

### Any other SMTP relay

1. In your provider's panel, add your sending domain. It gives you the SMTP host, port, encryption, user
   name and password, the DKIM records, and the SPF `include:` to add to your SPF record.
2. Keep to one SPF record per name: merge the `include:` into the record you already have.
3. If the provider cannot sign with your own domain, use [Yuva DKIM](#4-sign-with-yuva-dkim).
4. Enter the SMTP details in the channel under "Use a separate account for broadcasts". A separate
   account keeps complaints about marketing mail away from your support replies.

## 3. Check again

Run the check. Aim for SPF and DKIM passing, DKIM for your own domain and covering the unsubscribe
headers, and a DMARC record. A DMARC record with `p=none` is enough to start.

## 4. Sign with Yuva DKIM

Use it when the check says DKIM passes only for your provider's domain, or the signature does not
cover the unsubscribe headers, or when your provider cannot sign with your domain at all. Yuva then
signs the channel's broadcasts, test mails, confirmation and welcome mails itself, over the unsubscribe
headers. Your provider's signature stays next to it; support replies are not changed.

1. Open the channel's sending page (above) and find Yuva DKIM. Pick a selector (lowercase letters,
   digits and dashes; `yuva` by default) and press "Create key". The key is RSA 2048; the private key
   never leaves the server.
2. Publish the TXT record it shows, at the name shown (it ends with your sending domain). If your DNS
   provider limits a text string to 255 characters, split the value into several strings of one record.
3. Wait a few minutes (some providers take longer), then press "Check DNS and start signing". An owner
   or admin does this once the record is published.
4. Run the check again. The Yuva DKIM line should pass.

"Stop signing" turns it off at once; "Delete key" removes the key, after which you can remove the DNS
record. A new key needs a new record. Changing the channel's sending domain turns signing off until you
turn it on again for the new domain.

## 5. Gmail and Yahoo

Gmail and Yahoo set rules for senders of bulk mail. Gmail applies them from 5,000 messages a day to
Gmail addresses; Yahoo's guidance gives no number.

- **Authenticate.** SPF and DKIM, and a DMARC record (a policy of `none` is allowed). The domain in the
  From address has to align with the SPF or the DKIM domain. Gmail wants DKIM keys of at least 1,024
  bits and recommends 2,048.
- **One-click unsubscribe.** A `List-Unsubscribe` header, `List-Unsubscribe-Post: List-Unsubscribe=One-Click`
  ([RFC 8058](https://www.rfc-editor.org/rfc/rfc8058)), and a visible unsubscribe link in the mail.
  Yahoo says to honour requests within two days. Yuva adds both headers and the link to every broadcast
  and unsubscribes at once.
- **Keep spam complaints low.** Below 0.3%, and Google recommends below 0.1%. Yuva pauses a broadcast
  at 0.3% (see [Pauses Yuva makes](broadcasts.md#pauses-yuva-makes)).
- **TLS, and valid DNS for sending servers.** Gmail wants a TLS connection and forward and reverse DNS
  for the sending IPs. That is for your SMTP provider to do.

Sources: [Google's sender guidelines](https://support.google.com/a/answer/81126) and
[Yahoo's sender best practices](https://senders.yahooinc.com/best-practices/). Both change; read them
when you set up.

### Google Postmaster Tools

[Postmaster Tools](https://support.google.com/mail/answer/9981691) shows what Gmail sees of your mail:
spam rate, reputation, authentication and delivery errors, for mail to personal Gmail addresses. Sign
in with a Google account, add the domain your mail is signed or sent from (the DKIM `d=` domain or the
SPF/Return-Path domain), and verify it with the TXT or CNAME record Google shows. Nothing appears until
the domain is verified, and days with little mail show no data.

## 6. Start slowly

A new sending domain has no reputation, and a big first send is what looks like spam.

- **Hosted servers.** A new workspace may have to [ask for approval](broadcasts.md#approval-on-hosted-servers)
  and then warms up on its own: 500 recipients on the first day, doubling after each clean day (at least
  100 mails sent, bounces under 3%, complaints under 0.1%). A bad day takes one doubling back. The
  operator can set another limit.
- **Your own server.** Nothing limits you except what you set. Set
  [`YUVA_BROADCAST_DAILY_RECIPIENTS`](configuration.md#broadcasts) to cap the recipients per workspace
  and day (a send that reaches it continues the next day), and keep the channel's pace under what your
  provider allows. Raise the cap yourself over the first weeks.
- **Either way:** mail your most engaged subscribers first, keep the list clean (the [import](importing.md)
  leaves out addresses that cannot receive mail), and look at the first broadcast's bounces and
  complaints before sending the next. For a dedicated IP address, follow your provider's warm-up advice.

## From your backend

| Endpoint | Use |
|---|---|
| `POST /v1/channels/{channelId}/deliverability-check` | Start a check (`202`). At most six an hour per channel (`429 rate_limited`); `409 email_channel_required` when the channel cannot send. |
| `GET /v1/channels/{channelId}/deliverability` | The last check: `state`, `spf`, `dkim`, `dmarc`, `mx`, `yuva_dkim`. `404` when never checked. |
| `POST /v1/channels/{channelId}/dkim-key` | Create the key from a `selector`; the answer has the `dns_name` and `dns_value` to publish. `409 dkim_exists` when there is one. |
| `PATCH /v1/channels/{channelId}` | `email.dkim.enabled: true` turns signing on once DNS answers (`409 dkim_dns_missing`); `false` turns it off. |
| `DELETE /v1/channels/{channelId}/dkim-key` | Remove the key. |

`state` is `queued`, `sent` (until the test mail arrives), `received`, `unverifiable`, `timeout` or
`failed`. Creating and checking need `inboxes:manage`; reading needs `inboxes:read`. Full shapes are in
the [API contract](../openapi/openapi.yaml).

# Move from any IMAP server

Copy the history of a mailbox on any server that speaks IMAP into Yuva and send its new mail there. Yuva
only reads the mailbox. The overview is in [Move to Yuva](migrate.md).

## 1. Find the server

On the Source step Yuva looks for the server from the address's domain (its mail servers, its mail
settings file and its IMAP record). If it finds none, choose "Other IMAP server" and enter:

| Field | Value |
|---|---|
| IMAP server | The host name from your provider's help or your mail app's account settings. |
| Port | 993 with TLS, or 143 with STARTTLS. |
| User name | Usually the full address; your provider may want only the part before `@`. |
| Password | The mailbox password, or an app password if your provider offers them. |

If the server's certificate is not valid for its host name and it is your own server, the test shows the
certificate's fingerprint, and you can trust it for this migration after checking it. Servers on a private
or local network are refused unless the operator allowed them
([`YUVA_MIGRATION_ALLOW_PRIVATE`](configuration.md#moving-to-yuva)).

## 2. What is read

Yuva finds the inbox, Sent and Archive folders by the attributes the server sends, then by name, and
reads them with your own folders as labels. Spam, trash and drafts are never read. Yuva keeps to one
connection and 4 MB a second, and slows down when the server asks it to; a server with a connection limit
needs your mail apps closed during the copy.

## 3. Send the mail to Yuva

On the Switch step, copy the channel's [forwarding address](email.md#forwarding-address) and set up
forwarding in your provider's settings or webmail (keep a copy in the mailbox), or ask your mail
server's admin to route the mailbox to it. Without a forwarding address on the server, or for a domain
used only for support, change its MX records ([e-mail guide](email.md)). Then press "Send a test message"
in Yuva ([how it works](migrate.md#the-test-message-the-quiet-period-and-finishing)).

## 4. After it finishes

If you made a password only for Yuva, remove it at your provider.

# Move from Google Workspace

Copy the history of a Google Workspace mailbox (your own domain on Gmail) into Yuva and send its new mail
there. Yuva only reads the mailbox. The overview is in [Move to Yuva](migrate.md); for an address ending
in `@gmail.com`, use the [Gmail guide](migrate-gmail.md).

## 1. Before you start

Since 14 March 2025 Google no longer accepts a plain password for IMAP; Yuva connects with an app password.
Google does not offer app passwords on every work or school account, so the account may need help from
your admin:

- An admin allows IMAP: in the Admin console, **Menu → Apps → Google Workspace → Gmail → End User Access**,
  under **POP and IMAP access**, tick **Enable IMAP access for all users** and allow any mail client (Yuva
  does not use OAuth). Each person also enables IMAP in their own Gmail settings, **Forwarding and
  POP/IMAP**. Google's page:
  [Turn POP & IMAP on or off for users](https://knowledge.workspace.google.com/admin/sync/turn-pop-and-imap-on-or-off-for-users).
- 2-Step Verification must be on for the account. If [App passwords](https://myaccount.google.com/apppasswords)
  is missing even then, Google lists a work or school account among the reasons, so ask your admin; if the
  account offers no app password, Yuva cannot connect to it. Google's page:
  [Sign in with app passwords](https://support.google.com/accounts/answer/185833).

## 2. Create the app password

Open [App passwords](https://myaccount.google.com/apppasswords), name the password "Yuva", create it and
paste it in Yuva's Connect step with the mailbox address as the user name.

## 3. What is read, and what makes it slower

Yuva reads **All Mail** once per message; Gmail labels become Yuva labels; Spam and Trash are never read.
Google limits IMAP downloads to 2,500 MB per user a day and suspends access for 1 to 24 hours when it is
reached. Yuva stays under it at 1,500 MB a day, so a large mailbox takes several days, pausing by itself
and continuing the next day. Google's page:
[Gmail bandwidth limits](https://knowledge.workspace.google.com/admin/gmail/gmail-bandwidth-limits).

## 4. Send the mail to Yuva

On the Switch step, copy the channel's [forwarding address](email.md#forwarding-address). Two ways keep a
copy in Google:

- **The mailbox owner forwards it**, as in the [Gmail guide](migrate-gmail.md#4-forward-the-mailbox-to-yuva).
  An admin may have limited forwarding.
- **An admin routes it.** Google's **Routing** setting in the Admin console delivers a copy of a user's
  mail to another address; **Default routing** with dual delivery does the same for the whole
  organization. Google's page:
  [Email routing and delivery options](https://knowledge.workspace.google.com/admin/gmail/advanced/email-routing-and-delivery-options-for-google-workspace).

For a domain used only for support you can instead change its MX records to Yuva ([e-mail guide](email.md)).
Then press "Send a test message" in Yuva ([how it works](migrate.md#the-test-message-the-quiet-period-and-finishing)).

## 5. After it finishes

Remove the app password at [App passwords](https://myaccount.google.com/apppasswords).

## Very large mailbox

A mailbox of many gigabytes takes days over IMAP because of Google's daily limit. Start with a Google Takeout file
instead: it uploads as fast as your connection allows ([how](migrate-files.md)). Then connect the mailbox
over IMAP as above. Yuva reads only the mail newer than the newest message of the Takeout file, never
imports a message twice, and runs the switch as usual. In the panel, the Connect step offers "Upload a
Takeout file instead"; start there.

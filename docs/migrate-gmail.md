# Move from Gmail

Copy the history of a Gmail mailbox (an address ending in `@gmail.com`) into Yuva and forward its new mail
there. Yuva only reads the mailbox. The overview is in [Move to Yuva](migrate.md); on a work or school
address, use the [Google Workspace guide](migrate-google-workspace.md).

## 1. Create an app password

Gmail accepts Yuva only with an app password: a 16-character password just for Yuva.

1. Turn on 2-Step Verification for the Google Account, if it is not on yet.
2. Open [App passwords](https://myaccount.google.com/apppasswords) in the Google Account, name the
   password "Yuva" and create it.
3. Copy the password and paste it in Yuva's Connect step, with the Gmail address as the user name.

You will not find the option if 2-Step Verification is set up only for security keys, if Advanced
Protection is on, or on a work or school account. Google's page:
[Sign in with app passwords](https://support.google.com/accounts/answer/185833).

## 2. What is read

Yuva reads **All Mail**, so each message is copied once. Gmail labels become Yuva labels. Spam and Trash
are never read. Your Gmail stays as it is.

## 3. What makes a copy slower

Google limits IMAP downloads to 2,500 MB per user a day, and suspends access for 1 to 24 hours when it is
reached. Yuva stays under that at 1,500 MB a day, so a large mailbox takes several days. The copy
pauses by itself ("Waiting for Gmail's limit") and continues the next day; the newest conversations
arrive first. Close other mail apps that sync the mailbox, because they use the same allowance. Google's
page: [Gmail bandwidth limits](https://knowledge.workspace.google.com/admin/gmail/gmail-bandwidth-limits).

## 4. Forward the mailbox to Yuva

On the Switch step, copy the channel's [forwarding address](email.md#forwarding-address).

1. In Gmail, open Settings, **See all settings**, then **Forwarding and POP/IMAP**.
2. Click **Add a forwarding address**, paste the forwarding address and continue.
3. Google sends a confirmation to that address. It appears on Yuva's Switch step; enter its code in
   Gmail, or open its link.
4. In Gmail, reload the settings, select **Forward a copy of incoming mail to** the address, keep Gmail's
   copy in the inbox and press **Save Changes**.

Gmail forwards new mail only, and not spam; the history was copied in the first steps. Google's page:
[Automatically forward Gmail messages](https://support.google.com/mail/answer/10957).

Then press "Send a test message" in Yuva to check it ([how it works](migrate.md#the-test-message-the-quiet-period-and-finishing)).

## 5. After it finishes

Remove the app password: open [App passwords](https://myaccount.google.com/apppasswords), find "Yuva" and
click **Remove**. Changing your Google Account password also revokes app passwords.

## Very large mailbox

A mailbox of many gigabytes takes days over IMAP because of Gmail's daily limit. Start with a Google Takeout file
instead: it uploads as fast as your connection allows ([how](migrate-files.md)). Then connect the mailbox
over IMAP as above. Yuva reads only the mail newer than the newest message of the Takeout file, never
imports a message twice, and runs the switch as usual. In the panel, the Connect step offers "Upload a
Takeout file instead"; start there.

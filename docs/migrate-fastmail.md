# Move from Fastmail

Copy the history of a Fastmail mailbox into Yuva and forward its new mail there. Yuva only reads the
mailbox. The overview is in [Move to Yuva](migrate.md).

## 1. Create an app password

Fastmail accepts Yuva only with an app password, and Basic plans have none: they include no IMAP access.

1. In Fastmail, open **Settings → Privacy & Security**, and under **Connected apps & API tokens** manage
   your app passwords and create a new one. Confirm your password if asked.
2. Name it "Yuva", keep the access **Mail, Contacts & Calendars** and generate the password.
3. Copy it and paste it in Yuva's Connect step. The user name is your full Fastmail address.

Fastmail's pages: [App passwords](https://www.fastmail.help/hc/en-us/articles/360058752854-App-passwords)
and [Server names and ports](https://www.fastmail.help/hc/en-us/articles/1500000278342-Server-names-and-ports).

## 2. What is read

Yuva connects to `imap.fastmail.com`, port 993 with TLS. It reads the inbox, Sent, Archive and your own
folders, and turns the folders into labels. Spam, trash and drafts are never read. Fastmail's page names no
download limit; Yuva keeps to one connection and 4 MB a second, and slows down when Fastmail asks it to.

## 3. Forward the mailbox to Yuva

On the Switch step, copy the channel's [forwarding address](email.md#forwarding-address).

1. In Fastmail, open **Settings → Filters & Rules** and click **Create rule**.
2. Choose the condition **All messages**, then the action **Send a copy to** and enter the forwarding
   address.
3. Save the rule. The old mailbox keeps its copy.

Mail caught by a discard rule or filtered as spam is not forwarded; it stays in Fastmail, where Yuva does
not read it. Fastmail's page: [Set up mail forwarding](https://www.fastmail.help/hc/en-us/articles/360058753434-Set-up-mail-forwarding).
Then press "Send a test message" in Yuva ([how it works](migrate.md#the-test-message-the-quiet-period-and-finishing)).

## 4. After it finishes

In **Settings → Privacy & Security**, open the app passwords and remove "Yuva". Removing cannot be undone.

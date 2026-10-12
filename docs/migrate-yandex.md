# Move from Yandex

Copy the history of a Yandex Mail mailbox into Yuva and forward its new mail there. Yuva only reads the
mailbox. The overview is in [Move to Yuva](migrate.md).

## 1. Allow IMAP and create an app password

1. In Yandex Mail settings, under **Email clients**, turn on **From the imap.yandex.com server via IMAP**
   and **App passwords and OAuth tokens**, and save.
2. In your Yandex ID, open **App passwords**, click **Create an app password**, choose the type
   **Mail**, name it "Yuva" and continue.
3. Copy the password now: Yandex shows it only once. If you lose it, delete it and create a new one.
4. Paste it in Yuva's Connect step. Do not use your account password.

Yandex's page: [Mail clients](https://yandex.com/support/mail/en/mail-clients/others).

## 2. What is read

Yuva reads the inbox, Sent and your own folders, and turns the folders into labels. Spam, trash and
drafts are never read. Yandex publishes no download limit; Yuva keeps to one connection and 4 MB a second
so your mail apps are not slowed, and slows down when Yandex asks it to.

## 3. Forward the mailbox to Yuva

On the Switch step, copy the channel's [forwarding address](email.md#forwarding-address).

1. In Yandex Mail, click the settings icon, **All settings**, **Rules for processing emails**, then
   **Create filter**.
2. Apply it to all messages, turn on **Forward to**, enter the forwarding address and enable the option
   to save a copy.
3. Click **Create filter** and enter your Yandex password.
4. Yandex mails a confirmation to the forwarding address. It arrives in the target inbox as a
   conversation; open it and follow its instructions.

Only new incoming mail is forwarded, not the Inbox's old mail, sent mail or spam. Yandex's page:
[Forwarding email to another address](https://yandex.com/support/mail/web/preferences/filters/forwarding.html).
Then press "Send a test message" in Yuva ([how it works](migrate.md#the-test-message-the-quiet-period-and-finishing)).

## 4. After it finishes

Delete the "Yuva" password from **App passwords** in your Yandex ID.

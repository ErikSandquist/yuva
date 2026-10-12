# Move from Yahoo Mail

Copy the history of a Yahoo Mail mailbox into Yuva and forward its new mail there. Yuva only reads the
mailbox. The overview is in [Move to Yuva](migrate.md).

## 1. Create an app password

Yahoo accepts Yuva only with an app password.

1. Open your Yahoo Account Security page and, under **External connections**, click **Create app password**.
2. Name it "Yuva" and click **Generate password**.
3. Copy the password and paste it in Yuva's Connect step. The user name is your full Yahoo address. Do not
   use your account password.

Yahoo asks for a browser you have used to sign in to Yahoo for several days in a row, and not a private
window. Yahoo's pages: [Generate and manage 3rd-party app passwords](https://help.yahoo.com/kb/SLN15241.html)
and [IMAP server settings](https://help.yahoo.com/kb/SLN4075.html).

## 2. What is read

Yuva connects to `imap.mail.yahoo.com`, port 993 with TLS. It reads the inbox, Sent, Archive and your own
folders, and turns the folders into labels. Spam, trash and drafts are never read. Yahoo's page names no
download limit; Yuva keeps to one connection and 4 MB a second, and slows down when Yahoo asks it to.

## 3. Forward the mailbox to Yuva

Yahoo forwards mail only for paid accounts (Yahoo Mail Plus), and not in every country. On the Switch step,
copy the channel's [forwarding address](email.md#forwarding-address).

1. In Yahoo Mail on a computer, open **More options → Settings → Mailboxes** and click your primary
   mailbox.
2. Enter the forwarding address under **Auto-forwarding** and click **Verify**.
3. Yahoo mails a confirmation to the forwarding address. It arrives in the target inbox as a conversation;
   open it and follow its instructions.

Yahoo's page does not say whether a copy stays in Yahoo. Yahoo's page:
[Enable automatic email forwarding](https://help.yahoo.com/kb/SLN36684.html). Without a paid account, ask
your senders to use the channel's address, or change the MX records of a domain you own
([e-mail guide](email.md)). Then press "Send a test message" in Yuva
([how it works](migrate.md#the-test-message-the-quiet-period-and-finishing)).

## 4. After it finishes

Delete the "Yuva" password: on the Account Security page, under **External connections**, click the app
password and then **Delete**.

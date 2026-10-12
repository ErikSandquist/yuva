# Move from Zoho Mail

Copy the history of a Zoho Mail mailbox into Yuva and forward its new mail there. Yuva only reads the
mailbox. The overview is in [Move to Yuva](migrate.md).

## 1. Allow IMAP and create an app password

1. In Zoho Mail, open **Settings → Mail Accounts**, click your primary address, tick **IMAP Access** in the
   IMAP section and save.
2. If two-factor authentication is on for your Zoho account, Yuva needs an application-specific password.
   In Zoho Accounts, open **Security → App passwords**, click **Generate New Password**, name it "Yuva" and
   click **Generate**.
3. Copy the password now: Zoho shows it only once. Paste it in Yuva's Connect step; the user name is your
   full Zoho Mail address. Without two-factor authentication your account password works.

Zoho's pages: [IMAP and SMTP configuration](https://www.zoho.com/mail/help/imap-access.html) and
[Two-factor authentication](https://www.zoho.com/mail/help/adminconsole/two-factor-authentication.html).

## 2. Which server

Yuva picks the server from your address and the domain's mail servers: `imap.zoho.com` for personal
addresses and `imappro.zoho.com` for addresses on your own domain, both port 993 with TLS. Accounts in
other Zoho data centers use the same names ending `.eu`, `.in`, `.com.au`, `.jp`, `.ca` or `.sa`. Zoho's
page lists only the `.com` servers and points to the server details inside your account for the others;
if the test cannot connect, compare them with your account's.

## 3. What is read

Yuva reads the inbox, Sent and your own folders, and turns the folders into labels. Spam, trash and
drafts are never read. Zoho offers only the folders selected under **Folder Settings** to IMAP, so a folder
missing from the Choose step is hidden there. The page names no download limit; Yuva keeps to one
connection and 4 MB a second, and slows down when Zoho asks it to.

## 4. Forward the mailbox to Yuva

On the Switch step, copy the channel's [forwarding address](email.md#forwarding-address).

1. In Zoho Mail, open **Settings → Mail Accounts**, select your address and enter the forwarding address
   under **Forwards**.
2. Zoho mails a verification code to the forwarding address. It arrives in the target inbox as a
   conversation; in Settings → Mail Accounts click the verify icon next to the address, enter the code and
   confirm.
3. Leave **Delete original email copy after forwarding** unchecked, so the old mailbox stays complete.

Forwarding is not available to new users on the Free plan, and Zoho turns it off after ten failures in a
row. Zoho's page: [Email forwarding](https://www.zoho.com/mail/help/email-forwarding.html).
Then press "Send a test message" in Yuva ([how it works](migrate.md#the-test-message-the-quiet-period-and-finishing)).

## 5. After it finishes

Delete the "Yuva" password under **Security → App passwords** in Zoho Accounts.

# Move from iCloud Mail

Copy the history of an iCloud Mail mailbox into Yuva and forward its new mail there. Yuva only reads the
mailbox. The overview is in [Move to Yuva](migrate.md).

## 1. Create an app-specific password

iCloud Mail accepts Yuva only with an app-specific password, which needs two-factor authentication on the
Apple Account.

1. Sign in at [account.apple.com](https://account.apple.com).
2. In **Sign-In and Security**, open **App-Specific Passwords**.
3. Generate a new password named "Yuva" and follow the steps.
4. Paste it in Yuva's Connect step. The user name is your iCloud address without `@icloud.com`.

You can have up to 25 active app-specific passwords. Apple's page:
[Sign in with app-specific passwords](https://support.apple.com/en-us/102654); the server settings are
on [Mail server settings for iCloud Mail](https://support.apple.com/en-us/102525).

## 2. What is read

Yuva reads the inbox, Sent, Archive and your own folders, and turns the folders into labels.
Junk, trash and drafts are never read. Apple publishes no download limit; Yuva keeps to one connection
and 4 MB a second, and slows down when iCloud asks it to.

## 3. Forward the mailbox to Yuva

On the Switch step, copy the channel's [forwarding address](email.md#forwarding-address).

1. Sign in at [icloud.com/mail](https://www.icloud.com/mail), click the settings button at the top of the
   Mailboxes list and choose **Settings**.
2. Select **Mail Forwarding**, tick **Forward my email to** and enter the forwarding address.
3. Leave **Delete messages after forwarding** off: copies kept in iCloud use your iCloud storage, but the
   old mailbox stays complete.

Mail to a custom domain on iCloud Mail is forwarded too. Apple's page:
[Automatically forward email in Mail on iCloud.com](https://support.apple.com/guide/icloud/automatically-forward-email-mm6b1a3960/icloud).
Then press "Send a test message" in Yuva ([how it works](migrate.md#the-test-message-the-quiet-period-and-finishing)).

## 4. After it finishes

Remove the "Yuva" password at account.apple.com under **App-Specific Passwords** (**Remove**). Changing
your Apple Account password revokes all app-specific passwords.

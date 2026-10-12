# Move from GMX or mail.com

Copy the history of a GMX or mail.com mailbox into Yuva and forward its new mail there. Yuva only reads the
mailbox. The overview is in [Move to Yuva](migrate.md).

## 1. Turn on IMAP and create an app password

1. In GMX (or mail.com), open the **Email** tab, click **Settings → POP3 & IMAP**, tick **Enable access to
   this account via POP3 and IMAP** and save. GMX turns this off by itself if IMAP goes unused for a long
   time, so turn it on shortly before the copy. mail.com offers IMAP only with a Premium account.
2. Sign in on the website, click the profile icon at the top right, then **Security Options → Manage
   application-specific passwords → Create new application-specific password**. Name it "Yuva" and
   continue. GMX shows the password only once.
3. Paste it in Yuva's Connect step; the user name is your full address. If you use two-factor
   authentication, the app password replaces your account password.

The pages: [GMX: activate POP3 and IMAP](https://support.gmx.com/pop-imap/toggle.html),
[GMX: application-specific passwords](https://support.gmx.com/security/2fa/application-specific-passwords.html)
and [mail.com: activate POP3 and IMAP](https://support.mail.com/pop-imap/toggle.html).

## 2. Which server

Yuva picks the server from your address, port 993 with TLS: `imap.gmx.net` for GMX addresses (GMX's German
help names it; its English help names `imap.gmx.com`, which Yuva also knows) and `imap.mail.com` for
mail.com addresses ([GMX server data](https://hilfe.gmx.net/pop-imap/imap/imap-serverdaten.html),
[mail.com server data](https://support.mail.com/premium/imap/server.html)).

## 3. What is read

Yuva reads the inbox, Sent and your own folders, and turns the folders into labels. Spam, trash and
drafts are never read. The server pages name no download limit; Yuva keeps to one connection and 4 MB a
second, and slows down when the server asks it to.

## 4. Forward the mailbox to Yuva

On the Switch step, copy the channel's [forwarding address](email.md#forwarding-address).

1. In GMX, click your initials, **E-Mail-Einstellungen**, **Filterregeln** and **Filterregeln erstellen**.
2. Choose the condition (for example all new e-mails), the action **Weiterleiten an** and enter the
   forwarding address.
3. Save. GMX mails an activation link to the forwarding address. It arrives in the target inbox as a
   conversation; click the link, and forwarding starts.

GMX's page does not say whether a copy stays in GMX: check that the first mails are still in the old
mailbox. GMX's page: [Weiterleitung einrichten](https://hilfe.gmx.net/email/kontakte-organisieren/weiterleitung-einrichten.html).
For mail.com, filter rules have a **Forward** task ([mail.com: tasks](https://support.mail.com/email/filter-rules/tasks.html));
its page does not say whether a copy stays either. Then press "Send a test message" in Yuva
([how it works](migrate.md#the-test-message-the-quiet-period-and-finishing)).

## 5. After it finishes

Delete the "Yuva" app password under **Manage application-specific passwords**. Turn POP3 and IMAP off again
if you do not use them.

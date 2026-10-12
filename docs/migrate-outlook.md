# Move from Outlook.com

Copy the history of an Outlook.com, Hotmail, Live or MSN mailbox into Yuva and send its new mail there.
Yuva only reads the mailbox. The overview is in [Move to Yuva](migrate.md); for a work or school mailbox
use the [Microsoft 365 guide](migrate-microsoft365.md).

## 1. Before you start

Yuva connects with **Sign in with Microsoft**, not with a password or an app password: Microsoft's IMAP
settings for Outlook.com name OAuth 2.0 as the way to connect. Microsoft's pages:
[POP, IMAP and SMTP settings for Outlook.com](https://support.microsoft.com/en-us/outlook/pop-imap-and-smtp-settings-for-outlook-com),
[Authenticate an IMAP connection using OAuth](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth).

Yuva asks for two permissions only: IMAP access to the mailbox and offline access, so it can keep reading
for the days the migration runs. Microsoft's consent page words the first as "Read and write access to
mailboxes via IMAP", because Microsoft has no read-only IMAP permission. Yuva still only reads: its
connection refuses every command that could delete, move or mark mail
([how that is enforced](migrate.md#what-yuva-does-and-does-not-do-with-your-mailbox)). Yuva refuses a
sign-in that grants more than those two permissions.

## 2. Sign in

On the Connect step press **Sign in with Microsoft**, sign in as the mailbox's own account and allow
access. If the browser keeps choosing another Microsoft account, use a private window.

Whether you have to register an app first depends on the server:

- **The server has a Microsoft app of its own.** Nothing more to do: the step shows only the button.
  Hosted servers usually do.
- **It does not.** The step asks for the **Application (client) ID** and **Client secret** of an app that
  you register in your own Microsoft account, as below. Your operator may also tell you to ask them for
  one: see [Move to Yuva for operators](migrate-operators.md#registering-the-microsoft-app).

## 3. Register an app (only when the step asks for one)

The step lists these steps next to the **Redirect URL** to copy. Microsoft's page:
[Register an application](https://learn.microsoft.com/en-us/entra/identity-platform/quickstart-register-app).
Registering needs an Azure account; a free one is enough.

1. In the [Microsoft Entra admin center](https://entra.microsoft.com) go to **App registrations** and
   choose **New registration**. Name it "Yuva".
2. Under **Supported account types** choose **Personal accounts only**.
3. Under **Redirect URI** choose **Web** and enter the Redirect URL from Yuva.
4. In **Certificates & secrets** add a client secret and copy its **Value**; Microsoft shows it once.
5. In **API permissions** add a permission: **APIs my organization uses → Office 365 Exchange Online →
   Delegated permissions → IMAP.AccessAsUser.All**. Add **offline_access** from Microsoft Graph too.
6. Copy the **Application (client) ID** from Overview, enter it in Yuva with the secret and press
   **Sign in with Microsoft**. Leave the **Directory (tenant) ID** empty.

Yuva seals the secret as soon as you enter it and never shows it again.

## 4. Send the mail to Yuva

On the Switch step, copy the channel's [forwarding address](email.md#forwarding-address). In Outlook.com,
open **Settings → Mail → Forwarding**, choose **Enable forwarding**, enter the address, tick **Keep a copy
of forwarded messages** and save. Microsoft asks for two-step verification before it turns forwarding on.
Microsoft's page:
[Turn on or off automatic forwarding in Outlook.com](https://support.microsoft.com/en-us/office/turn-on-or-off-automatic-forwarding-in-outlook-com-6246987c-6c8f-4144-b255-14fc07007dad).

For a domain used only for support you can instead change its MX records to Yuva ([e-mail guide](email.md)).
Then press "Send a test message" in Yuva ([how it works](migrate.md#the-test-message-the-quiet-period-and-finishing)).

## 5. If the sign-in stops working

Yuva shows **Sign in again** when Microsoft stops accepting its sign-in (the password changed, access was
removed or it went unused for too long). Sign in as the same account; the migration continues where it
stopped (**Continue the migration**).

## 6. After it finishes

Yuva deletes its token when the migration ends but cannot revoke it at Microsoft. Remove Yuva's access:
in your Microsoft account open [Privacy → Apps and services](https://account.microsoft.com/privacy/app-access),
choose Yuva and remove its permissions. If you registered an app for Yuva, delete it too, in the Entra
admin center under **App registrations**. Keep the forwarding as it is.

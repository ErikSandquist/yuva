# Move from Microsoft 365

Copy the history of a Microsoft 365 (Exchange Online) mailbox into Yuva and send its new mail there. Yuva
only reads the mailbox. The overview is in [Move to Yuva](migrate.md); for Outlook.com, Hotmail and Live
use the [Outlook.com guide](migrate-outlook.md).

## 1. Before you start

Yuva connects with **Sign in with Microsoft**, not with a password or an app password: Microsoft has
turned Basic authentication off for IMAP in every tenant, and an admin cannot turn it back on. Microsoft's
page: [Deprecation of Basic authentication in Exchange Online](https://learn.microsoft.com/en-us/exchange/clients-and-mobile-in-exchange-online/deprecation-of-basic-authentication-exchange-online).

Yuva asks for two permissions only: IMAP access to the mailbox and offline access, so it can keep reading
for the days the migration runs. Microsoft's consent page words the first as "Read and write access to
mailboxes via IMAP", because Microsoft has no read-only IMAP permission. Yuva still only reads: its
connection refuses every command that could delete, move or mark mail
([how that is enforced](migrate.md#what-yuva-does-and-does-not-do-with-your-mailbox)). Yuva refuses a
sign-in that grants more than those two permissions.

Check three things first, or ask your admin:

- **IMAP is on for the mailbox.** It is on by default, but an admin can turn it off per mailbox: in the
  Exchange admin center, **Recipients → Mailboxes**, select the mailbox, **Manage email apps settings**,
  and set **IMAP** to **Enabled**. Microsoft's page:
  [Managing email apps for user mailboxes](https://learn.microsoft.com/en-us/exchange/recipients-in-exchange-online/manage-user-mailboxes/managing-email-apps-for-user-mailboxes).
- **Your organization lets you consent to the app** (below).
- **No Conditional Access policy blocks the sign-in** (below).

## 2. Sign in

On the Connect step press **Sign in with Microsoft**, sign in as the mailbox's own account and allow
access. If the browser keeps choosing another Microsoft account, use a private window.

The sign-in name may differ from the mailbox address (a user principal name often does): Yuva accepts the
account when Microsoft lets it open the mailbox, which also covers a person with full access to a shared
mailbox. It refuses, storing nothing, an account that cannot open it. Microsoft's page, with the shared
mailbox case: [Authenticate an IMAP connection using OAuth](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth).

Whether you have to register an app first depends on the server:

- **The server has a Microsoft app of its own.** Nothing more to do: the step shows only the button.
- **It does not.** The step asks for the **Application (client) ID** and **Client secret** of an app that
  you register in your own tenant, as in [step 4](#4-register-an-app-only-when-the-step-asks-for-one).

## 3. When Microsoft asks for an admin

- **Consent.** Whether staff may allow an app themselves is the organization's setting; some let users
  consent only to apps of verified publishers, and Yuva's app is not publisher-verified. Then Yuva shows
  **Your organization's admin must allow Yuva** and a **Link for your Microsoft 365 admin**. Send it to an
  admin (the page can copy a ready message). When they open the link and accept, they land in Yuva, and
  you press **Sign in with Microsoft** again. Microsoft's pages:
  [Overview of user and admin consent](https://learn.microsoft.com/en-us/entra/identity/enterprise-apps/user-admin-consent-overview),
  [Grant tenant-wide admin consent](https://learn.microsoft.com/en-us/entra/identity/enterprise-apps/grant-admin-consent).
- **Conditional Access.** If a policy of your organization blocks the sign-in, Yuva says so. Ask your admin
  to allow it, or move the mailbox from an [export file](migrate-files.md) instead.
- **IMAP turned off.** If Microsoft signs you in but the mailbox refuses the token, IMAP is off for the
  mailbox: see step 1.

## 4. Register an app (only when the step asks for one)

The step lists these steps next to the **Redirect URL** to copy. Microsoft's page:
[Register an application](https://learn.microsoft.com/en-us/entra/identity-platform/quickstart-register-app).

1. In the [Microsoft Entra admin center](https://entra.microsoft.com) go to **App registrations** and
   choose **New registration**. Name it "Yuva".
2. Under **Supported account types** choose **Single tenant only** for your organization's mailboxes, or
   **Any Entra ID Tenant + Personal Microsoft accounts** if you are not sure.
3. Under **Redirect URI** choose **Web** and enter the Redirect URL from Yuva.
4. In **Certificates & secrets** add a client secret and copy its **Value**; Microsoft shows it once.
5. In **API permissions** add a permission: **APIs my organization uses → Office 365 Exchange Online →
   Delegated permissions → IMAP.AccessAsUser.All**. Add **offline_access** from Microsoft Graph too.
6. Copy the **Application (client) ID** from Overview and enter it in Yuva with the secret. For a single
   tenant app, also enter its **Directory (tenant) ID** (optional otherwise); then press
   **Sign in with Microsoft**.

Yuva seals the secret as soon as you enter it and never shows it again.

## 5. Send the mail to Yuva

On the Switch step, copy the channel's [forwarding address](email.md#forwarding-address). Two ways keep a
copy in Microsoft 365:

- **An admin forwards it.** In the Microsoft 365 admin center, **Users → Active users**, select the user,
  open the **Mail** tab, **Manage email forwarding**, choose **Forward all emails sent to this mailbox**,
  enter the address and choose to keep a copy. Forwarding to an address outside the organization can be
  limited by an outbound spam policy. Microsoft's pages:
  [Configure email forwarding](https://learn.microsoft.com/en-us/microsoft-365/admin/email/configure-email-forwarding?view=o365-worldwide),
  [Control automatic external email forwarding](https://learn.microsoft.com/en-us/defender-office-365/outbound-spam-policies-external-email-forwarding).
- **For a domain used only for support** change its MX records to Yuva ([e-mail guide](email.md)).
  Mail then no longer reaches Microsoft 365.

Then press "Send a test message" in Yuva ([how it works](migrate.md#the-test-message-the-quiet-period-and-finishing)).

## 6. If the sign-in stops working

Yuva shows **Sign in again** when Microsoft stops accepting its sign-in (the password changed, access was
removed or it went unused for too long). Sign in as the same account; the migration continues where it
stopped (**Continue the migration**).

## 7. After it finishes

Yuva deletes its token when the migration ends but cannot revoke it at Microsoft. Remove Yuva's access:

- **You:** open [My Apps](https://myapps.microsoft.com), choose Yuva, **Manage your application** and
  **Revoke Permissions**. Microsoft's page:
  [Edit or revoke application permissions in the My Apps portal](https://support.microsoft.com/en-us/account-billing/edit-or-revoke-application-permissions-in-the-my-apps-portal-169be2b4-ee26-4338-aea8-d19bb2f329ee).
- **An admin,** for everyone and for permissions an admin granted: in the Microsoft Entra admin center,
  **Enterprise apps**, select Yuva and remove it.
- If you registered an app for Yuva, delete it under **App registrations**.

Keep the forwarding or MX as it is.

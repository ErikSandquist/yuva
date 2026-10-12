# Von Microsoft 365 umziehen

Kopieren Sie den Verlauf eines Microsoft-365-Postfachs (Exchange Online) nach Yuva und schicken Sie seine
neuen E-Mails dorthin. Yuva liest das Postfach nur. Die Übersicht steht unter
[Zu Yuva umziehen](migrate.md); für Outlook.com, Hotmail und Live nutzen Sie die
[Outlook.com-Anleitung](migrate-outlook.md).

## 1. Bevor Sie anfangen

Yuva verbindet sich mit **Sign in with Microsoft**, nicht mit einem Passwort oder App-Passwort: Microsoft hat
die Standardauthentifizierung (Basic Authentication) für IMAP in allen Mandanten abgeschaltet, und ein Admin
kann sie nicht wieder einschalten. Microsofts Seite:
[Deprecation of Basic authentication in Exchange Online](https://learn.microsoft.com/en-us/exchange/clients-and-mobile-in-exchange-online/deprecation-of-basic-authentication-exchange-online).

Yuva fragt nur zwei Berechtigungen an: IMAP-Zugriff auf das Postfach und Offlinezugriff, damit es die Tage,
die der Umzug läuft, weiterlesen kann. Microsofts Zustimmungsseite formuliert die erste als „Read and write
access to mailboxes via IMAP“, weil Microsoft keine schreibgeschützte IMAP-Berechtigung hat. Yuva liest
trotzdem nur: Seine Verbindung lehnt jeden Befehl ab, der E-Mails löschen, verschieben oder markieren könnte
([wie das sichergestellt wird](migrate.md#was-yuva-mit-ihrem-postfach-tut-und-was-nicht)). Yuva lehnt eine
Anmeldung ab, die mehr als diese zwei Berechtigungen gewährt.

Prüfen Sie zuerst drei Dinge, oder fragen Sie Ihren Admin:

- **IMAP ist für das Postfach an.** Es ist standardmäßig an, aber ein Admin kann es je Postfach abschalten:
  Wählen Sie im Exchange Admin Center unter **Recipients → Mailboxes** das Postfach, dann **Manage email apps
  settings**, und setzen Sie **IMAP** auf **Enabled**. Microsofts Seite:
  [Managing email apps for user mailboxes](https://learn.microsoft.com/en-us/exchange/recipients-in-exchange-online/manage-user-mailboxes/managing-email-apps-for-user-mailboxes).
- **Ihre Organisation erlaubt Ihnen, der App zuzustimmen** (unten).
- **Keine Conditional-Access-Richtlinie blockiert die Anmeldung** (unten).

## 2. Anmelden

Drücken Sie im Schritt Connect **Sign in with Microsoft**, melden Sie sich mit dem eigenen Konto des
Postfachs an und erlauben Sie den Zugriff. Wählt der Browser hartnäckig ein anderes Microsoft-Konto, nutzen
Sie ein privates Fenster.

Der Anmeldename darf von der Postfachadresse abweichen (der Benutzerprinzipalname tut das oft): Yuva nimmt
das Konto an, wenn Microsoft es das Postfach öffnen lässt; das gilt auch für eine Person mit Vollzugriff auf
ein freigegebenes Postfach. Ein Konto, das das Postfach nicht öffnen kann, lehnt es ab, ohne etwas zu
speichern. Microsofts Seite, mit dem Fall des freigegebenen Postfachs:
[Authenticate an IMAP connection using OAuth](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth).

Ob Sie zuerst eine App registrieren müssen, hängt vom Server ab:

- **Der Server hat eine eigene Microsoft-App.** Nichts weiter zu tun: Der Schritt zeigt nur die Schaltfläche.
- **Er hat keine.** Der Schritt fragt nach der **Application (client) ID** und dem **Client secret** einer App,
  die Sie wie in [Schritt 4](#4-eine-app-registrieren-nur-wenn-der-schritt-danach-fragt) in Ihrem eigenen
  Mandanten registrieren.

## 3. Wenn Microsoft einen Admin verlangt

- **Zustimmung.** Ob Mitarbeitende einer App selbst zustimmen dürfen, bestimmt die Organisation; manche
  erlauben Nutzern die Zustimmung nur zu Apps verifizierter Herausgeber, und die App von Yuva ist nicht
  herausgeberverifiziert. Dann zeigt Yuva **Your organization's admin must allow Yuva** und einen **Link for
  your Microsoft 365 admin**. Schicken Sie ihn einem Admin (die Seite kann eine fertige Nachricht kopieren).
  Öffnet der Admin den Link und stimmt zu, landet er wieder in Yuva, und Sie drücken erneut
  **Sign in with Microsoft**. Microsofts Seiten:
  [Overview of user and admin consent](https://learn.microsoft.com/en-us/entra/identity/enterprise-apps/user-admin-consent-overview),
  [Grant tenant-wide admin consent](https://learn.microsoft.com/en-us/entra/identity/enterprise-apps/grant-admin-consent).
- **Conditional Access.** Blockiert eine Richtlinie Ihrer Organisation die Anmeldung, sagt Yuva das. Bitten
  Sie Ihren Admin, sie zu erlauben, oder ziehen Sie das Postfach stattdessen mit einer
  [Exportdatei](migrate-files.md) um.
- **IMAP abgeschaltet.** Meldet Microsoft Sie an, aber das Postfach lehnt das Token ab, ist IMAP für das
  Postfach aus: siehe Schritt 1.

## 4. Eine App registrieren (nur wenn der Schritt danach fragt)

Der Schritt listet diese Schritte neben der **Redirect URL** zum Kopieren. Microsofts Seite:
[Register an application](https://learn.microsoft.com/en-us/entra/identity-platform/quickstart-register-app).

1. Gehen Sie im [Microsoft Entra Admin Center](https://entra.microsoft.com) zu **App registrations** und
   wählen Sie **New registration**. Nennen Sie sie „Yuva“.
2. Wählen Sie unter **Supported account types** **Single tenant only** für die Postfächer Ihrer Organisation
   oder, wenn Sie unsicher sind, **Any Entra ID Tenant + Personal Microsoft accounts**.
3. Wählen Sie unter **Redirect URI** **Web** und tragen Sie die Redirect URL aus Yuva ein.
4. Fügen Sie unter **Certificates & secrets** ein Client Secret hinzu und kopieren Sie seinen **Value**;
   Microsoft zeigt ihn nur einmal.
5. Fügen Sie unter **API permissions** eine Berechtigung hinzu: **APIs my organization uses → Office 365
   Exchange Online → Delegated permissions → IMAP.AccessAsUser.All**. Fügen Sie auch **offline_access** aus
   Microsoft Graph hinzu.
6. Kopieren Sie die **Application (client) ID** aus Overview und tragen Sie sie mit dem Secret in Yuva ein.
   Tragen Sie bei einer App für einen einzelnen Mandanten auch ihre **Directory (tenant) ID** ein (sonst
   optional); drücken Sie dann **Sign in with Microsoft**.

Yuva versiegelt das Secret, sobald Sie es eingeben, und zeigt es nie wieder.

## 5. Die E-Mails an Yuva schicken

Kopieren Sie im Schritt Switch die [Weiterleitungsadresse](email.md#weiterleitungsadresse) des Kanals. Zwei
Wege lassen eine Kopie in Microsoft 365:

- **Ein Admin leitet weiter.** Wählen Sie im Microsoft 365 Admin Center unter **Users → Active users** die
  Person, öffnen Sie den Tab **Mail**, dann **Manage email forwarding**, wählen Sie **Forward all emails sent
  to this mailbox**, tragen Sie die Adresse ein und wählen Sie, eine Kopie zu behalten. Die Weiterleitung an
  eine Adresse außerhalb der Organisation kann durch eine Richtlinie gegen ausgehenden Spam eingeschränkt
  sein. Microsofts Seiten:
  [Configure email forwarding](https://learn.microsoft.com/en-us/microsoft-365/admin/email/configure-email-forwarding?view=o365-worldwide),
  [Control automatic external email forwarding](https://learn.microsoft.com/en-us/defender-office-365/outbound-spam-policies-external-email-forwarding).
- **Bei einer Domain, die nur dem Support dient,** stellen Sie ihre MX-Einträge auf Yuva um
  ([E-Mail-Anleitung](email.md)). Die E-Mails erreichen Microsoft 365 dann nicht mehr.

Drücken Sie dann in Yuva „Send a test message“ ([so funktioniert es](migrate.md#testnachricht-ruhige-phase-und-abschluss)).

## 6. Wenn die Anmeldung nicht mehr funktioniert

Yuva zeigt **Sign in again**, wenn Microsoft die Anmeldung von Yuva nicht mehr annimmt (das Passwort wurde
geändert, der Zugriff entfernt oder zu lange nicht genutzt). Melden Sie sich mit demselben Konto an; der Umzug
läuft dort weiter, wo er stehen blieb (**Continue the migration**).

## 7. Nach dem Abschluss

Yuva löscht sein Token, wenn der Umzug endet, kann es bei Microsoft aber nicht widerrufen. Entfernen Sie den
Zugriff von Yuva:

- **Sie selbst:** Öffnen Sie [My Apps](https://myapps.microsoft.com), wählen Sie Yuva, dann **Manage your
  application** und **Revoke Permissions**. Microsofts Seite:
  [Edit or revoke application permissions in the My Apps portal](https://support.microsoft.com/en-us/account-billing/edit-or-revoke-application-permissions-in-the-my-apps-portal-169be2b4-ee26-4338-aea8-d19bb2f329ee).
- **Ein Admin,** für alle und für Berechtigungen, die ein Admin erteilt hat: Wählen Sie im Microsoft Entra
  Admin Center unter **Enterprise apps** Yuva und entfernen Sie es.
- Haben Sie für Yuva eine App registriert, löschen Sie sie unter **App registrations**.

Lassen Sie die Weiterleitung oder den MX, wie sie sind.

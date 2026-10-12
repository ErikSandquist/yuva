# Von Outlook.com umziehen

Kopieren Sie den Verlauf eines Outlook.com-, Hotmail-, Live- oder MSN-Postfachs nach Yuva und schicken Sie
seine neuen E-Mails dorthin. Yuva liest das Postfach nur. Die Übersicht steht unter
[Zu Yuva umziehen](migrate.md); für ein Arbeits- oder Schulpostfach nutzen Sie die
[Microsoft-365-Anleitung](migrate-microsoft365.md).

## 1. Bevor Sie anfangen

Yuva verbindet sich mit **Sign in with Microsoft**, nicht mit einem Passwort oder App-Passwort: Microsofts
IMAP-Einstellungen für Outlook.com nennen OAuth 2.0 als Weg der Verbindung. Microsofts Seiten:
[POP, IMAP and SMTP settings for Outlook.com](https://support.microsoft.com/en-us/outlook/pop-imap-and-smtp-settings-for-outlook-com),
[Authenticate an IMAP connection using OAuth](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth).

Yuva fragt nur zwei Berechtigungen an: IMAP-Zugriff auf das Postfach und Offlinezugriff, damit es die Tage,
die der Umzug läuft, weiterlesen kann. Microsofts Zustimmungsseite formuliert die erste als „Read and write
access to mailboxes via IMAP“, weil Microsoft keine schreibgeschützte IMAP-Berechtigung hat. Yuva liest
trotzdem nur: Seine Verbindung lehnt jeden Befehl ab, der E-Mails löschen, verschieben oder markieren könnte
([wie das sichergestellt wird](migrate.md#was-yuva-mit-ihrem-postfach-tut-und-was-nicht)). Yuva lehnt eine
Anmeldung ab, die mehr als diese zwei Berechtigungen gewährt.

## 2. Anmelden

Drücken Sie im Schritt Connect **Sign in with Microsoft**, melden Sie sich mit dem eigenen Konto des
Postfachs an und erlauben Sie den Zugriff. Wählt der Browser hartnäckig ein anderes Microsoft-Konto, nutzen
Sie ein privates Fenster.

Ob Sie zuerst eine App registrieren müssen, hängt vom Server ab:

- **Der Server hat eine eigene Microsoft-App.** Nichts weiter zu tun: Der Schritt zeigt nur die Schaltfläche.
  Gehostete Server haben meist eine.
- **Er hat keine.** Der Schritt fragt nach der **Application (client) ID** und dem **Client secret** einer App,
  die Sie wie unten in Ihrem eigenen Microsoft-Konto registrieren. Ihr Betreiber kann Ihnen auch eine
  anbieten: siehe [Zu Yuva umziehen für Betreiber](migrate-operators.md#die-microsoft-app-registrieren).

## 3. Eine App registrieren (nur wenn der Schritt danach fragt)

Der Schritt listet diese Schritte neben der **Redirect URL** zum Kopieren. Microsofts Seite:
[Register an application](https://learn.microsoft.com/en-us/entra/identity-platform/quickstart-register-app).
Zum Registrieren brauchen Sie ein Azure-Konto; ein kostenloses genügt.

1. Gehen Sie im [Microsoft Entra Admin Center](https://entra.microsoft.com) zu **App registrations** und
   wählen Sie **New registration**. Nennen Sie sie „Yuva“.
2. Wählen Sie unter **Supported account types** **Personal accounts only**.
3. Wählen Sie unter **Redirect URI** **Web** und tragen Sie die Redirect URL aus Yuva ein.
4. Fügen Sie unter **Certificates & secrets** ein Client Secret hinzu und kopieren Sie seinen **Value**;
   Microsoft zeigt ihn nur einmal.
5. Fügen Sie unter **API permissions** eine Berechtigung hinzu: **APIs my organization uses → Office 365
   Exchange Online → Delegated permissions → IMAP.AccessAsUser.All**. Fügen Sie auch **offline_access** aus
   Microsoft Graph hinzu.
6. Kopieren Sie die **Application (client) ID** aus Overview, tragen Sie sie mit dem Secret in Yuva ein und
   drücken Sie **Sign in with Microsoft**. Lassen Sie die **Directory (tenant) ID** leer.

Yuva versiegelt das Secret, sobald Sie es eingeben, und zeigt es nie wieder.

## 4. Die E-Mails an Yuva schicken

Kopieren Sie im Schritt Switch die [Weiterleitungsadresse](email.md#weiterleitungsadresse) des Kanals. Öffnen
Sie in Outlook.com **Settings → Mail → Forwarding**, wählen Sie **Enable forwarding**, tragen Sie die Adresse
ein, setzen Sie den Haken bei **Keep a copy of forwarded messages** und speichern Sie. Microsoft verlangt die
Bestätigung in zwei Schritten, bevor es die Weiterleitung einschaltet. Microsofts Seite:
[Turn on or off automatic forwarding in Outlook.com](https://support.microsoft.com/en-us/office/turn-on-or-off-automatic-forwarding-in-outlook-com-6246987c-6c8f-4144-b255-14fc07007dad).

Für eine Domain, die nur dem Support dient, können Sie stattdessen ihre MX-Einträge auf Yuva umstellen
([E-Mail-Anleitung](email.md)). Drücken Sie dann in Yuva „Send a test message“
([so funktioniert es](migrate.md#testnachricht-ruhige-phase-und-abschluss)).

## 5. Wenn die Anmeldung nicht mehr funktioniert

Yuva zeigt **Sign in again**, wenn Microsoft die Anmeldung von Yuva nicht mehr annimmt (das Passwort wurde
geändert, der Zugriff entfernt oder zu lange nicht genutzt). Melden Sie sich mit demselben Konto an; der Umzug
läuft dort weiter, wo er stehen blieb (**Continue the migration**).

## 6. Nach dem Abschluss

Yuva löscht sein Token, wenn der Umzug endet, kann es bei Microsoft aber nicht widerrufen. Entfernen Sie den
Zugriff von Yuva: Öffnen Sie in Ihrem Microsoft-Konto
[Privacy → Apps and services](https://account.microsoft.com/privacy/app-access), wählen Sie Yuva und entfernen
Sie seine Berechtigungen. Haben Sie für Yuva eine App registriert, löschen Sie auch sie, im Entra Admin Center
unter **App registrations**. Lassen Sie die Weiterleitung, wie sie ist.

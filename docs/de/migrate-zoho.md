# Von Zoho Mail umziehen

Kopieren Sie den Verlauf eines Zoho-Mail-Postfachs nach Yuva und leiten Sie seine neuen E-Mails dorthin
weiter. Yuva liest das Postfach nur. Die Übersicht steht unter [Zu Yuva umziehen](migrate.md).

## 1. IMAP erlauben und ein App-Passwort erstellen

1. Öffnen Sie in Zoho Mail **Settings → Mail Accounts**, klicken Sie auf Ihre Hauptadresse, aktivieren Sie im
   IMAP-Bereich **IMAP Access** und speichern Sie.
2. Ist für Ihr Zoho-Konto die Zwei-Faktor-Authentifizierung eingeschaltet, braucht Yuva ein
   anwendungsspezifisches Passwort. Öffnen Sie in Zoho Accounts **Security → App passwords**, klicken Sie auf
   **Generate New Password**, nennen Sie es „Yuva“ und klicken Sie auf **Generate**.
3. Kopieren Sie das Passwort sofort: Zoho zeigt es nur einmal. Fügen Sie es im Schritt Connect von Yuva ein;
   der Benutzername ist Ihre vollständige Zoho-Mail-Adresse. Ohne Zwei-Faktor-Authentifizierung gilt Ihr
   Kontopasswort.

Zohos Seiten: [IMAP and SMTP configuration](https://www.zoho.com/mail/help/imap-access.html) und
[Two-factor authentication](https://www.zoho.com/mail/help/adminconsole/two-factor-authentication.html).

## 2. Welcher Server

Yuva wählt den Server aus Ihrer Adresse und den Mailservern der Domain: `imap.zoho.com` für private Adressen
und `imappro.zoho.com` für Adressen auf Ihrer eigenen Domain, beide Port 993 mit TLS. Konten in anderen
Zoho-Rechenzentren nutzen dieselben Namen mit der Endung `.eu`, `.in`, `.com.au`, `.jp`, `.ca` oder `.sa`.
Zohos Seite nennt nur die `.com`-Server und verweist für die übrigen auf die Serverdaten in Ihrem Konto;
kann der Test nicht verbinden, vergleichen Sie sie mit denen Ihres Kontos.

## 3. Was gelesen wird

Yuva liest den Posteingang, Gesendet und Ihre eigenen Ordner und macht Labels aus den Ordnern. Spam,
Papierkorb und Entwürfe werden nie gelesen. Zoho bietet IMAP nur die unter **Folder Settings** gewählten
Ordner an; ein Ordner, der im Schritt Choose fehlt, ist dort ausgeblendet. Die Seite nennt kein
Download-Limit; Yuva bleibt bei einer Verbindung und 4 MB pro Sekunde und drosselt, wenn Zoho darum bittet.

## 4. Das Postfach an Yuva weiterleiten

Kopieren Sie im Schritt Switch die [Weiterleitungsadresse](email.md#weiterleitungsadresse) des Kanals.

1. Öffnen Sie in Zoho Mail **Settings → Mail Accounts**, wählen Sie Ihre Adresse und tragen Sie unter
   **Forwards** die Weiterleitungsadresse ein.
2. Zoho schickt einen Bestätigungscode an die Weiterleitungsadresse. Er landet im Ziel-Posteingang als
   Unterhaltung; klicken Sie unter Settings → Mail Accounts auf das Bestätigen-Symbol neben der Adresse,
   geben Sie den Code ein und bestätigen Sie.
3. Lassen Sie **Delete original email copy after forwarding** ausgeschaltet, damit das alte Postfach
   vollständig bleibt.

Neue Nutzer im Free-Tarif haben keine Weiterleitung, und Zoho schaltet sie nach zehn Fehlern in Folge ab.
Zohos Seite: [Email forwarding](https://www.zoho.com/mail/help/email-forwarding.html).
Drücken Sie dann in Yuva „Send a test message“
([so funktioniert es](migrate.md#testnachricht-ruhige-phase-und-abschluss)).

## 5. Nach dem Abschluss

Löschen Sie das Passwort „Yuva“ in Zoho Accounts unter **Security → App passwords**.

# Von iCloud Mail umziehen

Kopieren Sie den Verlauf eines iCloud-Mail-Postfachs nach Yuva und leiten Sie seine neuen E-Mails dorthin
weiter. Yuva liest das Postfach nur. Die Übersicht steht unter [Zu Yuva umziehen](migrate.md).

## 1. Ein app-spezifisches Passwort erstellen

iCloud Mail akzeptiert Yuva nur mit einem app-spezifischen Passwort; dafür braucht der Apple-Account die
Zwei-Faktor-Authentifizierung.

1. Melden Sie sich auf [account.apple.com](https://account.apple.com) an.
2. Öffnen Sie unter **Sign-In and Security** die **App-Specific Passwords**.
3. Erzeugen Sie ein neues Passwort namens „Yuva“ und folgen Sie den Schritten auf dem Bildschirm.
4. Fügen Sie es im Schritt Connect von Yuva ein. Der Benutzername ist Ihre iCloud-Adresse ohne `@icloud.com`.

Sie können bis zu 25 aktive app-spezifische Passwörter haben. Apples Seite:
[Sign in with app-specific passwords](https://support.apple.com/en-us/102654); die Servereinstellungen
stehen unter [Mail server settings for iCloud Mail](https://support.apple.com/en-us/102525).

## 2. Was gelesen wird

Yuva liest den Posteingang, Gesendet, Archiv und Ihre eigenen Ordner und macht Labels aus den Ordnern. Junk,
Papierkorb und Entwürfe werden nie gelesen. Apple veröffentlicht kein Download-Limit; Yuva bleibt bei einer
Verbindung und 4 MB pro Sekunde und drosselt, wenn iCloud darum bittet.

## 3. Das Postfach an Yuva weiterleiten

Kopieren Sie im Schritt Switch die [Weiterleitungsadresse](email.md#weiterleitungsadresse) des Kanals.

1. Melden Sie sich auf [icloud.com/mail](https://www.icloud.com/mail) an, klicken Sie oben in der
   Postfachliste auf die Einstellungen-Taste und wählen Sie **Settings**.
2. Wählen Sie **Mail Forwarding**, aktivieren Sie „Forward my email to“ und tragen Sie die
   Weiterleitungsadresse ein.
3. Lassen Sie „Delete messages after forwarding“ aus: Kopien in iCloud belegen Ihren
   iCloud-Speicher, aber das alte Postfach bleibt vollständig.

E-Mails an eine eigene Domain bei iCloud Mail werden ebenfalls weitergeleitet. Apples Seite:
[Automatically forward email in Mail on iCloud.com](https://support.apple.com/guide/icloud/automatically-forward-email-mm6b1a3960/icloud).
Drücken Sie dann in Yuva „Send a test message“
([so funktioniert es](migrate.md#testnachricht-ruhige-phase-und-abschluss)).

## 4. Nach dem Abschluss

Entfernen Sie das Passwort „Yuva“ auf account.apple.com unter **App-Specific Passwords** (**Remove**).
Eine Änderung des Apple-Account-Passworts widerruft alle app-spezifischen Passwörter.

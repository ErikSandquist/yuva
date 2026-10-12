# Von einem beliebigen IMAP-Server umziehen

Kopieren Sie den Verlauf eines Postfachs auf einem beliebigen IMAP-Server nach Yuva und schicken Sie seine
neuen E-Mails dorthin. Yuva liest das Postfach nur. Die Übersicht steht unter
[Zu Yuva umziehen](migrate.md).

## 1. Den Server finden

Im Schritt Source sucht Yuva den Server aus der Domain der Adresse (ihre Mailserver, ihre
Mail-Einstellungsdatei und ihr IMAP-Eintrag). Findet es keinen, wählen Sie „Other IMAP server“ und geben Sie
ein:

| Feld | Wert |
|---|---|
| IMAP server | Der Hostname aus der Hilfe Ihres Anbieters oder den Kontoeinstellungen Ihrer Mail-App. |
| Port | 993 mit TLS oder 143 mit STARTTLS. |
| User name | Meist die vollständige Adresse; Ihr Anbieter will vielleicht nur den Teil vor `@`. |
| Password | Das Postfachpasswort oder, wenn Ihr Anbieter welche anbietet, ein App-Passwort. |

Ist das Zertifikat des Servers für seinen Hostnamen nicht gültig und der Server Ihr eigener, zeigt der Test den
Fingerabdruck des Zertifikats, und Sie können ihm nach Prüfung für diesen Umzug vertrauen. Server in einem
privaten oder lokalen Netz werden abgelehnt, sofern der Betreiber sie nicht erlaubt hat
([`YUVA_MIGRATION_ALLOW_PRIVATE`](configuration.md#zu-yuva-umziehen)).

## 2. Was gelesen wird

Yuva findet Posteingang, Gesendet und Archiv zuerst an den Attributen, die der Server sendet, dann am Namen,
und liest sie mit Ihren eigenen Ordnern; Ordner werden zu Labels. Spam, Papierkorb und Entwürfe werden nie
gelesen. Yuva bleibt bei einer Verbindung und 4 MB pro Sekunde und drosselt, wenn der Server darum bittet;
bei einem Server mit Verbindungslimit sollten Sie Ihre Mail-Apps während des Kopierens schließen.

## 3. Die E-Mails an Yuva schicken

Kopieren Sie im Schritt Switch die [Weiterleitungsadresse](email.md#weiterleitungsadresse) des Kanals und
richten Sie die Weiterleitung in den Einstellungen oder im Webmail Ihres Anbieters ein (mit Kopie im
Postfach), oder bitten Sie den Admin Ihres Mailservers, das Postfach dorthin zu routen. Hat der Server keine
Weiterleitungsadresse oder dient die Domain nur dem Support, ändern Sie ihre MX-Einträge
([E-Mail-Anleitung](email.md)). Drücken Sie dann in Yuva „Send a test message“
([so funktioniert es](migrate.md#testnachricht-ruhige-phase-und-abschluss)).

## 4. Nach dem Abschluss

Haben Sie ein Passwort nur für Yuva angelegt, entfernen Sie es bei Ihrem Anbieter.

# Von Yahoo Mail umziehen

Kopieren Sie den Verlauf eines Yahoo-Mail-Postfachs nach Yuva und leiten Sie seine neuen E-Mails dorthin
weiter. Yuva liest das Postfach nur. Die Übersicht steht unter [Zu Yuva umziehen](migrate.md).

## 1. Ein App-Passwort erstellen

Yahoo akzeptiert Yuva nur mit einem App-Passwort.

1. Öffnen Sie Ihre Yahoo-Account-Security-Seite und klicken Sie unter **External connections** auf **Create
   app password**.
2. Nennen Sie es „Yuva“ und klicken Sie auf **Generate password**.
3. Kopieren Sie das Passwort und fügen Sie es im Schritt Connect von Yuva ein. Der Benutzername ist Ihre
   vollständige Yahoo-Adresse. Nehmen Sie nicht Ihr Kontopasswort.

Yahoo verlangt einen Browser, mit dem Sie sich seit mehreren Tagen in Folge bei Yahoo angemeldet haben, und
kein privates Fenster. Yahoos Seiten: [Generate and manage 3rd-party app passwords](https://help.yahoo.com/kb/SLN15241.html)
und [IMAP server settings](https://help.yahoo.com/kb/SLN4075.html).

## 2. Was gelesen wird

Yuva verbindet sich mit `imap.mail.yahoo.com`, Port 993 mit TLS. Es liest den Posteingang, Gesendet, Archiv
und Ihre eigenen Ordner und macht Labels aus den Ordnern. Spam, Papierkorb und Entwürfe werden nie gelesen.
Yahoos Seite nennt kein Download-Limit; Yuva bleibt bei einer Verbindung und 4 MB pro Sekunde und drosselt,
wenn Yahoo darum bittet.

## 3. Das Postfach an Yuva weiterleiten

Yahoo leitet nur bei kostenpflichtigen Konten (Yahoo Mail Plus) und nicht in jedem Land weiter. Kopieren Sie
im Schritt Switch die [Weiterleitungsadresse](email.md#weiterleitungsadresse) des Kanals.

1. Öffnen Sie in Yahoo Mail am Computer **More options → Settings → Mailboxes** und klicken Sie auf Ihr
   Hauptpostfach.
2. Tragen Sie unter **Auto-forwarding** die Weiterleitungsadresse ein und klicken Sie auf **Verify**.
3. Yahoo schickt eine Bestätigung an die Weiterleitungsadresse. Sie landet im Ziel-Posteingang als
   Unterhaltung; öffnen Sie sie und folgen Sie den Anweisungen.

Yahoos Seite sagt nicht, ob eine Kopie in Yahoo bleibt. Yahoos Seite:
[Enable automatic email forwarding](https://help.yahoo.com/kb/SLN36684.html). Ohne kostenpflichtiges Konto
bitten Sie Ihre Absender, die Adresse des Kanals zu verwenden, oder ändern Sie die MX-Einträge einer Domain,
die Ihnen gehört ([E-Mail-Anleitung](email.md)). Drücken Sie dann in Yuva „Send a test message“
([so funktioniert es](migrate.md#testnachricht-ruhige-phase-und-abschluss)).

## 4. Nach dem Abschluss

Löschen Sie das Passwort „Yuva“: Klicken Sie auf der Account-Security-Seite unter **External connections**
auf das App-Passwort und dann auf **Delete**.

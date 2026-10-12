# Von Yandex umziehen

Kopieren Sie den Verlauf eines Yandex-Mail-Postfachs nach Yuva und leiten Sie seine neuen E-Mails dorthin
weiter. Yuva liest das Postfach nur. Die Übersicht steht unter [Zu Yuva umziehen](migrate.md).

## 1. IMAP erlauben und ein App-Passwort erstellen

1. Schalten Sie in den Yandex-Mail-Einstellungen unter **Email clients** „From the imap.yandex.com server via IMAP“
   und „App passwords and OAuth tokens“ ein und speichern Sie.
2. Öffnen Sie in Ihrer Yandex-ID **App passwords**, klicken Sie auf „Create an app password“, wählen Sie den
   Typ **Mail**, nennen Sie es „Yuva“ und fahren Sie fort.
3. Kopieren Sie das Passwort sofort: Yandex zeigt es nur einmal. Haben Sie es verloren, löschen Sie es und
   erstellen Sie ein neues.
4. Fügen Sie es im Schritt Connect von Yuva ein. Nehmen Sie nicht Ihr Kontopasswort.

Yandex' Seite: [Mail clients](https://yandex.com/support/mail/en/mail-clients/others).

## 2. Was gelesen wird

Yuva liest den Posteingang, Gesendet und Ihre eigenen Ordner und macht Labels aus den Ordnern. Spam,
Papierkorb und Entwürfe werden nie gelesen. Yandex veröffentlicht kein Download-Limit; Yuva bleibt bei einer
Verbindung und 4 MB pro Sekunde, damit Ihre Mail-Apps nicht langsamer werden, und drosselt, wenn Yandex darum
bittet.

## 3. Das Postfach an Yuva weiterleiten

Kopieren Sie im Schritt Switch die [Weiterleitungsadresse](email.md#weiterleitungsadresse) des Kanals.

1. Klicken Sie in Yandex Mail auf das Einstellungssymbol, **All settings**, **Rules for processing emails**, dann
   **Create filter**.
2. Wenden Sie ihn auf alle Nachrichten an, schalten Sie **Forward to** ein, tragen Sie die
   Weiterleitungsadresse ein und aktivieren Sie die Option, eine Kopie zu behalten.
3. Klicken Sie auf **Create filter** und geben Sie Ihr Yandex-Passwort ein.
4. Yandex sendet eine Bestätigung an die Weiterleitungsadresse. Sie kommt im Ziel-Posteingang als
   Unterhaltung an; öffnen Sie sie und folgen Sie ihren Hinweisen.

Weitergeleitet werden nur neue eingehende E-Mails, nicht die alten E-Mails im Posteingang, gesendete E-Mails
oder Spam. Yandex' Seite:
[Forwarding email to another address](https://yandex.com/support/mail/web/preferences/filters/forwarding.html).
Drücken Sie dann in Yuva „Send a test message“
([so funktioniert es](migrate.md#testnachricht-ruhige-phase-und-abschluss)).

## 4. Nach dem Abschluss

Löschen Sie das Passwort „Yuva“ unter **App passwords** in Ihrer Yandex-ID.

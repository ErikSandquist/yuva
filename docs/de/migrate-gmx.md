# Von GMX oder mail.com umziehen

Kopieren Sie den Verlauf eines GMX- oder mail.com-Postfachs nach Yuva und leiten Sie seine neuen E-Mails
dorthin weiter. Yuva liest das Postfach nur. Die Übersicht steht unter [Zu Yuva umziehen](migrate.md).

## 1. IMAP einschalten und ein App-Passwort erstellen

1. Öffnen Sie in GMX (oder mail.com) den Tab **Email**, klicken Sie auf **Settings → POP3 & IMAP**,
   aktivieren Sie **Enable access to this account via POP3 and IMAP** und speichern Sie. GMX schaltet das von
   selbst wieder aus, wenn IMAP lange nicht genutzt wird; schalten Sie es kurz vor dem Kopieren ein. mail.com
   bietet IMAP nur mit einem Premium-Konto.
2. Melden Sie sich auf der Website an, klicken Sie oben rechts auf das Profilsymbol, dann auf **Security
   Options → Manage application-specific passwords → Create new application-specific password**. Nennen Sie es
   „Yuva“ und fahren Sie fort. GMX zeigt das Passwort nur einmal.
3. Fügen Sie es im Schritt Connect von Yuva ein; der Benutzername ist Ihre vollständige Adresse. Nutzen Sie die
   Zwei-Faktor-Authentifizierung, ersetzt das App-Passwort Ihr Kontopasswort.

Die Seiten: [GMX: activate POP3 and IMAP](https://support.gmx.com/pop-imap/toggle.html),
[GMX: application-specific passwords](https://support.gmx.com/security/2fa/application-specific-passwords.html)
und [mail.com: activate POP3 and IMAP](https://support.mail.com/pop-imap/toggle.html).

## 2. Welcher Server

Yuva wählt den Server aus Ihrer Adresse, Port 993 mit TLS: `imap.gmx.net` für GMX-Adressen (so nennt ihn die
deutsche GMX-Hilfe; die englische nennt `imap.gmx.com`, das Yuva ebenfalls kennt) und `imap.mail.com` für
mail.com-Adressen ([GMX-Serverdaten](https://hilfe.gmx.net/pop-imap/imap/imap-serverdaten.html),
[mail.com-Serverdaten](https://support.mail.com/premium/imap/server.html)).

## 3. Was gelesen wird

Yuva liest den Posteingang, Gesendet und Ihre eigenen Ordner und macht Labels aus den Ordnern. Spam,
Papierkorb und Entwürfe werden nie gelesen. Die Serverseiten nennen kein Download-Limit; Yuva bleibt bei
einer Verbindung und 4 MB pro Sekunde und drosselt, wenn der Server darum bittet.

## 4. Das Postfach an Yuva weiterleiten

Kopieren Sie im Schritt Switch die [Weiterleitungsadresse](email.md#weiterleitungsadresse) des Kanals.

1. Klicken Sie in GMX auf Ihre Initialen, dann auf **E-Mail-Einstellungen**, **Filterregeln** und
   **Filterregeln erstellen**.
2. Legen Sie die Bedingung fest (zum Beispiel alle neuen E-Mails), wählen Sie die Aktion **Weiterleiten an**
   und tragen Sie die Weiterleitungsadresse ein.
3. Speichern Sie. GMX schickt einen Aktivierungslink an die Weiterleitungsadresse. Er landet im
   Ziel-Posteingang als Unterhaltung; klicken Sie ihn an, dann beginnt die Weiterleitung.

GMX' Seite sagt nicht, ob eine Kopie in GMX bleibt: Prüfen Sie, dass die ersten E-Mails im alten Postfach
bleiben. GMX' Seite: [Weiterleitung einrichten](https://hilfe.gmx.net/email/kontakte-organisieren/weiterleitung-einrichten.html).
Bei mail.com haben Filterregeln eine Aufgabe **Forward** ([mail.com: tasks](https://support.mail.com/email/filter-rules/tasks.html));
auch seine Seite sagt nicht, ob eine Kopie bleibt. Drücken Sie dann in Yuva „Send a test message“
([so funktioniert es](migrate.md#testnachricht-ruhige-phase-und-abschluss)).

## 5. Nach dem Abschluss

Löschen Sie das App-Passwort „Yuva“ unter **Manage application-specific passwords**. Schalten Sie POP3 und
IMAP wieder aus, wenn Sie sie nicht nutzen.

# Von Gmail umziehen

Kopieren Sie den Verlauf eines Gmail-Postfachs (eine Adresse auf `@gmail.com`) nach Yuva und leiten Sie
seine neuen E-Mails dorthin weiter. Yuva liest das Postfach nur. Die Übersicht steht unter
[Zu Yuva umziehen](migrate.md); für eine Arbeits- oder Schuladresse nutzen Sie die
[Google-Workspace-Anleitung](migrate-google-workspace.md).

## 1. Ein App-Passwort erstellen

Gmail akzeptiert Yuva nur mit einem App-Passwort: einem 16-stelligen Passwort nur für Yuva.

1. Schalten Sie die 2-Step Verification für das Google-Konto ein, falls sie noch nicht an ist.
2. Öffnen Sie im Google-Konto [App passwords](https://myaccount.google.com/apppasswords), nennen Sie das
   Passwort „Yuva“ und erstellen Sie es.
3. Kopieren Sie das Passwort in den Schritt Connect von Yuva; Benutzername ist die Gmail-Adresse.

Die Option fehlt, wenn die 2-Step Verification nur mit Sicherheitsschlüsseln eingerichtet ist, der
Advanced Protection an ist oder es ein Arbeits- oder Schulkonto ist. Googles Seite:
[Sign in with app passwords](https://support.google.com/accounts/answer/185833).

## 2. Was gelesen wird

Yuva liest **All Mail**, sodass jede Nachricht einmal kopiert wird. Gmail-Labels werden Yuva-Labels.
Spam und Papierkorb werden nie gelesen. Ihr Gmail bleibt, wie es ist.

## 3. Was das Kopieren verlangsamt

Google begrenzt IMAP-Downloads auf 2.500 MB pro Nutzer und Tag und sperrt den Zugriff für 1 bis 24 Stunden,
wenn das erreicht ist. Yuva bleibt mit 1.500 MB am Tag darunter, ein großes Postfach braucht deshalb mehrere
Tage. Das Kopieren pausiert von selbst („Waiting for Gmail's limit“) und läuft am nächsten Tag weiter; die
neuesten Unterhaltungen kommen zuerst. Schließen Sie andere Mail-Apps, die das Postfach synchronisieren,
denn sie verbrauchen dasselbe Kontingent. Googles Seite:
[Gmail bandwidth limits](https://knowledge.workspace.google.com/admin/gmail/gmail-bandwidth-limits).

## 4. Das Postfach an Yuva weiterleiten

Kopieren Sie im Schritt Switch die [Weiterleitungsadresse](email.md#weiterleitungsadresse) des Kanals.

1. Öffnen Sie in Gmail Settings, **See all settings**, dann **Forwarding and POP/IMAP**.
2. Klicken Sie auf **Add a forwarding address**, fügen Sie die Weiterleitungsadresse ein und fahren
   Sie fort.
3. Google sendet eine Bestätigung an diese Adresse. Sie erscheint im Schritt Switch von Yuva; geben Sie den
   Code in Gmail ein oder öffnen Sie den Link.
4. Laden Sie die Einstellungen in Gmail neu, wählen Sie **Forward a copy of incoming mail to**
   diese Adresse, behalten Sie die Gmail-Kopie im Posteingang und klicken Sie auf **Save Changes**.

Gmail leitet nur neue E-Mails weiter, keinen Spam; der Verlauf wurde in den ersten Schritten kopiert. Googles
Seite: [Automatically forward Gmail messages](https://support.google.com/mail/answer/10957).

Drücken Sie dann in Yuva „Send a test message“, um es zu prüfen
([so funktioniert es](migrate.md#testnachricht-ruhige-phase-und-abschluss)).

## 5. Nach dem Abschluss

Entfernen Sie das App-Passwort: Öffnen Sie [App passwords](https://myaccount.google.com/apppasswords),
suchen Sie „Yuva“ und klicken Sie auf **Remove**. Auch eine Änderung des Google-Kontopassworts widerruft
App-Passwörter.

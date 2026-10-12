# Von Fastmail umziehen

Kopieren Sie den Verlauf eines Fastmail-Postfachs nach Yuva und leiten Sie seine neuen E-Mails dorthin
weiter. Yuva liest das Postfach nur. Die Übersicht steht unter [Zu Yuva umziehen](migrate.md).

## 1. Ein App-Passwort erstellen

Fastmail akzeptiert Yuva nur mit einem App-Passwort; Basic-Tarife haben keinen IMAP-Zugang und daher keine.

1. Öffnen Sie in Fastmail **Settings → Privacy & Security**, verwalten Sie unter **Connected apps & API
   tokens** Ihre App-Passwörter und legen Sie ein neues an. Bestätigen Sie bei Nachfrage Ihr Passwort.
2. Nennen Sie es „Yuva“, lassen Sie den Zugriff auf **Mail, Contacts & Calendars** und erzeugen Sie das
   Passwort.
3. Kopieren Sie es und fügen Sie es im Schritt Connect von Yuva ein. Der Benutzername ist Ihre vollständige
   Fastmail-Adresse.

Fastmails Seiten: [App passwords](https://www.fastmail.help/hc/en-us/articles/360058752854-App-passwords)
und [Server names and ports](https://www.fastmail.help/hc/en-us/articles/1500000278342-Server-names-and-ports).

## 2. Was gelesen wird

Yuva verbindet sich mit `imap.fastmail.com`, Port 993 mit TLS. Es liest den Posteingang, Gesendet, Archiv
und Ihre eigenen Ordner und macht Labels aus den Ordnern. Spam, Papierkorb und Entwürfe werden nie gelesen.
Fastmails Seite nennt kein Download-Limit; Yuva bleibt bei einer Verbindung und 4 MB pro Sekunde und
drosselt, wenn Fastmail darum bittet.

## 3. Das Postfach an Yuva weiterleiten

Kopieren Sie im Schritt Switch die [Weiterleitungsadresse](email.md#weiterleitungsadresse) des Kanals.

1. Öffnen Sie in Fastmail **Settings → Filters & Rules** und klicken Sie auf **Create rule**.
2. Wählen Sie die Bedingung **All messages**, dann die Aktion **Send a copy to** und tragen Sie die
   Weiterleitungsadresse ein.
3. Speichern Sie die Regel. Das alte Postfach behält seine Kopie.

E-Mails, die eine Discard-Regel trifft oder die als Spam gefiltert werden, werden nicht weitergeleitet; sie
bleiben in Fastmail, wo Yuva sie nicht liest. Fastmails Seite: [Set up mail forwarding](https://www.fastmail.help/hc/en-us/articles/360058753434-Set-up-mail-forwarding).
Drücken Sie dann in Yuva „Send a test message“
([so funktioniert es](migrate.md#testnachricht-ruhige-phase-und-abschluss)).

## 4. Nach dem Abschluss

Öffnen Sie unter **Settings → Privacy & Security** die App-Passwörter und entfernen Sie „Yuva“. Das
Entfernen lässt sich nicht rückgängig machen.

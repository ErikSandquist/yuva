# Von Google Workspace umziehen

Kopieren Sie den Verlauf eines Google-Workspace-Postfachs (Ihre eigene Domain auf Gmail) nach Yuva und
schicken Sie seine neuen E-Mails dorthin. Yuva liest das Postfach nur. Die Übersicht steht unter
[Zu Yuva umziehen](migrate.md); für eine Adresse auf `@gmail.com` nutzen Sie die
[Gmail-Anleitung](migrate-gmail.md).

## 1. Bevor Sie anfangen

Seit dem 14. März 2025 nimmt Google für IMAP kein einfaches Passwort mehr an; Yuva verbindet sich mit einem
App-Passwort. Google bietet nicht in jedem Arbeits- oder Schulkonto App-Passwörter an, deshalb kann das Konto
die Hilfe Ihres Admins brauchen:

- Ein Admin erlaubt IMAP: In der Admin-Konsole unter **Menu → Apps → Google Workspace → Gmail → End User
  Access**, im Bereich **POP and IMAP access**, **Enable IMAP access for all users** ankreuzen
  und jeden E-Mail-Client zulassen (Yuva nutzt kein OAuth). Außerdem aktiviert jede Person IMAP in den
  eigenen Gmail-Einstellungen unter **Forwarding and POP/IMAP**. Googles Seite:
  [Turn POP & IMAP on or off for users](https://knowledge.workspace.google.com/admin/sync/turn-pop-and-imap-on-or-off-for-users).
- 2-Step Verification muss für das Konto an sein. Fehlen die
  [App passwords](https://myaccount.google.com/apppasswords) trotzdem, nennt Google ein Arbeits- oder
  Schulkonto unter den Gründen; fragen Sie Ihren Admin. Bietet das Konto kein App-Passwort an, kann Yuva sich
  nicht damit verbinden. Googles Seite:
  [Sign in with app passwords](https://support.google.com/accounts/answer/185833).

## 2. Das App-Passwort erstellen

Öffnen Sie [App passwords](https://myaccount.google.com/apppasswords), nennen Sie das Passwort „Yuva“,
erstellen Sie es und fügen Sie es im Schritt Connect von Yuva ein, mit der Postfachadresse als Benutzername.

## 3. Was gelesen wird und was verlangsamt

Yuva liest **All Mail** einmal je Nachricht; Gmail-Labels werden Yuva-Labels; Spam und Papierkorb
werden nie gelesen. Google begrenzt IMAP-Downloads auf 2.500 MB pro Nutzer und Tag und sperrt den Zugriff für
1 bis 24 Stunden, wenn das erreicht ist. Yuva bleibt mit 1.500 MB am Tag darunter; ein großes Postfach
braucht deshalb mehrere Tage, pausiert von selbst und läuft am nächsten Tag weiter. Googles Seite:
[Gmail bandwidth limits](https://knowledge.workspace.google.com/admin/gmail/gmail-bandwidth-limits).

## 4. Die E-Mails an Yuva schicken

Kopieren Sie im Schritt Switch die [Weiterleitungsadresse](email.md#weiterleitungsadresse) des Kanals. Zwei
Wege lassen eine Kopie bei Google:

- **Der Postfachinhaber leitet weiter**, wie in der
  [Gmail-Anleitung](migrate-gmail.md#4-das-postfach-an-yuva-weiterleiten). Ein Admin kann die Weiterleitung
  eingeschränkt haben.
- **Ein Admin routet.** Die Einstellung **Routing** in der Admin-Konsole stellt eine Kopie der E-Mails einer
  Person an eine weitere Adresse zu; **Default routing** mit Dual Delivery tut das für die ganze
  Organisation. Googles Seite:
  [Email routing and delivery options](https://knowledge.workspace.google.com/admin/gmail/advanced/email-routing-and-delivery-options-for-google-workspace).

Für eine Domain, die nur dem Support dient, können Sie stattdessen ihre MX-Einträge auf Yuva umstellen
([E-Mail-Anleitung](email.md)). Drücken Sie dann in Yuva „Send a test message“
([so funktioniert es](migrate.md#testnachricht-ruhige-phase-und-abschluss)).

## 5. Nach dem Abschluss

Entfernen Sie das App-Passwort unter [App passwords](https://myaccount.google.com/apppasswords).

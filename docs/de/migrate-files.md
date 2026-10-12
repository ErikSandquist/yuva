# Aus einer Exportdatei umziehen

Holen Sie den Verlauf eines Postfachs aus einer exportierten Datei nach Yuva, ohne dass Yuva Zugriff auf das
Konto bekommt. Das passt für ein sehr großes Postfach, ein Programm ohne IMAP oder ein Konto, bei dem sich
Yuva nicht anmelden kann. Die Übersicht steht unter [Zu Yuva umziehen](migrate.md).

Öffnen Sie **Settings → Move to Yuva → New migration**, geben Sie die Adresse ein, an die die E-Mails
gingen, öffnen Sie **Or upload an export file**, wählen Sie **Google Takeout** oder **Export file** und
wählen Sie die Datei. Danach wählen Sie, was mitkommt, scannen und kopieren wie bei jedem anderen Umzug. Die
Datei wird auf dem Server geprüft und gelesen; Yuva verändert sie nie.

## Dateien, die Yuva liest

| Datei | Hinweise |
|---|---|
| Google-Takeout-`.mbox` | Entpacken Sie zuerst das Takeout-Archiv und laden Sie die `.mbox`-Datei aus dem Ordner Mail hoch. Gmails Labels und Threads kommen mit. |
| Eine `.mbox`-Datei | `mboxrd` (was die meisten Programme schreiben) oder unter „mbox options“ das ältere `mboxo`. |
| Eine `.zip` mit `.eml`-Dateien | Ihre Ordner werden Labels. |
| Eine `.zip` mit einem Maildir | Seine Ordner werden Labels. |
| Eine `.zip` mit `.mbox`-Dateien | Jede Datei wird ein Label. Die `.mbox`-Dateien müssen unkomprimiert in der Zip gespeichert sein. |

Yuva liest keine `.tgz`- oder `.gz`-Dateien, keine komprimierten `.mbox`-Dateien in einer Zip und keine
`.pst`-Dateien von Outlook. Entpacken Sie ein Archiv und laden Sie die `.mbox` darin hoch.

## Grenzen

| Grenze | Wert |
|---|---|
| Größe eines Upload-Teils | 8 MiB |
| Größe einer Datei | 20 GiB, festgelegt durch [`YUVA_MIGRATION_UPLOAD_MAX_BYTES`](configuration.md#zu-yuva-umziehen) des Servers |
| Einträge in einer Zip | 1.000.000 |
| Kompression eines Zip-Eintrags | höchstens 100:1 |

Nachrichten über [`YUVA_MIGRATION_MAX_MESSAGE_BYTES`](configuration.md#zu-yuva-umziehen) werden wie in jedem
Umzug übersprungen. Eine Datei pro Umzug; für eine weitere Datei starten Sie einen neuen Umzug.

## Hochladen und fortsetzen

Der Browser schneidet die Datei in Teile von 8 MiB, sendet mehrere gleichzeitig und wiederholt ein Teil, das
fehlschlägt. Lassen Sie die Seite offen. Reißt die Verbindung ab, geht der Upload weiter, sobald sie wieder
da ist. Wurde die Seite geschlossen oder neu geladen, öffnen Sie den Umzug erneut und **wählen Sie dieselbe
Datei noch einmal**: Yuva vergleicht, was es schon hat, und sendet nur die fehlenden Teile. Eine andere Datei
wird nicht untergemischt; die Seite fragt, bevor sie diese von vorn sendet. Sind alle Teile angekommen, liest
Yuva die Datei und nennt ihr Format.

## Wenn die Datei abgelehnt wird

Yuva prüft eine Zip, bevor es etwas daraus liest. Die hochgeladenen Teile einer abgelehnten Datei werden
gelöscht; korrigieren Sie die Datei und laden Sie sie erneut hoch.

| Grund | Bedeutung |
|---|---|
| `unknown_format` | Weder eine mbox noch eine Zip. |
| `empty` | Es ist keine Nachricht darin. |
| `corrupt` | Die Zip lässt sich nicht lesen; sie ist vielleicht beschädigt oder abgeschnitten. Erstellen Sie sie neu. |
| `path_traversal` | Ein Eintrag hat `..` oder einen absoluten Pfad im Namen. |
| `symlink` | Ein Eintrag ist ein symbolischer Link. Erstellen Sie die Zip ohne Links neu. |
| `too_many_entries` | Die Zip hat mehr als 1.000.000 Einträge. |
| `zip_bomb` | Ein Eintrag ist stärker als 100:1 komprimiert. |
| `entry_too_large` | Ein Eintrag ist größer als die Grenze des Servers für eine Datei. |
| `encrypted` | Ein Eintrag hat ein Passwort. |
| `unsupported_compression` | Ein Eintrag ist weder gespeichert noch mit Deflate komprimiert. |
| `compressed_mbox` | Eine `.mbox` in der Zip ist komprimiert. Entpacken Sie das Archiv und laden Sie die `.mbox` selbst hoch. |

Eine Datei über der Grenze des Servers wird abgelehnt, bevor sie gesendet wird. Ist die Grenze des Servers
`0`, sind Dateiimporte aus.

## Die Datei wird gelöscht

Die hochgeladenen Teile, also die Datei, werden gelöscht, wenn der Umzug abgeschlossen, abgebrochen oder
gelöscht wird. Yuva behält nur, was es importiert hat. Die Seite des Umzugs nennt, wann die Datei gelöscht
wurde.

## Eine Datei aus Ihrem Mailprogramm holen

Jede Seite unten ist die Hilfe des Programms selbst.

- **Google Takeout.** Öffnen Sie [Google Takeout](https://takeout.google.com), wählen Sie nur Mail, wählen Sie
  einen einmaligen Export als **Zip-Dateien** und erstellen Sie ihn. Google schickt Ihnen einen Link per
  E-Mail, wenn das Archiv fertig ist; das kann von Minuten bis zu Tagen dauern, der Link gilt etwa 7 Tage und
  für 5 Downloads. Entpacken Sie das Archiv und laden Sie die `.mbox`-Datei aus dem Ordner Mail hoch. Googles
  Seite: [Export your Google data](https://support.google.com/accounts/answer/3024190).
- **Apple Mail.** Wählen Sie die Postfächer und dann **Mailbox > Export Mailbox**. Mail schreibt jedes als
  `.mbox`-Datei. Apples Seite:
  [Import or export mailboxes in Mail](https://support.apple.com/guide/mail/import-or-export-mailboxes-mlhlp1030/mac).
- **Thunderbird.** Thunderbird hält jeden Mailordner als mbox-Datei in Ihrem Profil. Mozillas Seite:
  [Profiles - Where Thunderbird stores user data](https://support.mozilla.org/en-US/kb/profiles-where-thunderbird-stores-user-data).
  Laden Sie die Datei eines Ordners hoch.
- **Outlook.** Outlook exportiert nach `.pst`, das Yuva nicht liest. Wandeln Sie die `.pst` mit einem
  Konvertierungswerkzeug Ihrer Wahl in `.mbox`- oder `.eml`-Dateien um, packen Sie die `.eml`-Dateien in eine
  `.zip` und laden Sie diese hoch. Microsofts Seite:
  [Export or back up email to an Outlook .pst file](https://support.microsoft.com/en-us/office/export-or-backup-email-contacts-and-calendar-to-an-outlook-pst-file-14252b52-3075-4e9b-be4e-ff9ef1068f91).
- **Proton Mail.** Protons Exportwerkzeug schreibt `.eml`-Dateien. Packen Sie sie in eine `.zip` und laden Sie
  sie hoch. Protons Seite: [Export emails](https://proton.me/support/export-emails-import-export-app).

## Sehr große Gmail-Postfächer

Beginnen Sie mit einer Takeout-Datei und verbinden Sie das Postfach danach über IMAP für die neuere Post und
die Umstellung: siehe [Gmail](migrate-gmail.md#sehr-großes-postfach) und
[Google Workspace](migrate-google-workspace.md#sehr-großes-postfach).

## Aus Ihrem Backend

Owner, Admins und API-Schlüssel ohne Inbox-Beschränkung mit `migrations:manage`. Starten Sie einen Umzug mit
dem Preset `file` oder `takeout`, dann:

| Endpoint | Verwendung |
|---|---|
| `GET /v1/migrations/{migrationId}/upload` | Die Teile, die der Server hat, um einen abgebrochenen Upload fortzusetzen. |
| `POST /v1/migrations/{migrationId}/upload/parts/{part}` | Teil `part` senden (ab 0): ein `file` von höchstens 8 MiB und sein `sha256`, als `multipart/form-data`. Teile dürfen in beliebiger Reihenfolge und parallel eintreffen. |
| `POST /v1/migrations/{migrationId}/upload/complete` | Die Teile prüfen und die Datei lesen; der Umzug wird `connected`. |

Fehler, die Sie behandeln sollten: `413 part_too_large`, `422 checksum_mismatch` (das Teil wird nicht
gespeichert), `422 hosted_limit` (die Datei liegt über der Grenze des Servers), `422 upload_incomplete`
(`missing_parts` nennt die ersten fehlenden Teile), `422 file_refused` (`reason` und, wenn einer schuld ist,
der Zip-`entry`) und `409 migration_state`. Der Rest des Ablaufs steht unter
[Zu Yuva umziehen](migrate.md#aus-ihrem-backend); die vollständigen Formen stehen im
[API-Vertrag](../openapi/openapi.yaml).

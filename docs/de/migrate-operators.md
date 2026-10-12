# Zu Yuva umziehen für Betreiber

So betreiben Sie [Zu Yuva umziehen](migrate.md) auf Ihrem eigenen Server: die Weiterleitungsdomain, die
Einstellungen, was Sie prüfen können, um darauf zu vertrauen, dass das alte Postfach nur gelesen wird, und die
Grenzen für einen Server mit vielen Workspaces. IMAP-Postfächer sind bisher die einzige gebaute Quelle.

## Die Weiterleitungsdomain

Ein Postfach umzuziehen geht auch ohne: Teams stellen per MX-Einträgen um. Mit einer Weiterleitungsdomain
bekommt jeder E-Mail-Kanal zusätzlich eine [Weiterleitungsadresse](email.md#weiterleitungsadresse)
`<Token>@<Weiterleitungsdomain>`, an die ein Team sein altes Postfach weiterleiten kann.

1. Wählen Sie einen Namen, den kein anderer Mailserver bedient, etwa `forward.example.com`, und setzen Sie ihn
   auf dem Server:

   ```sh
   YUVA_FORWARDING_DOMAIN=forward.example.com
   ```

2. Leiten Sie alle E-Mails dieser Domain an Yuva, wie bei der eigenen Domain eines Kanals (die drei Wege unter
   [Eingehende E-Mails](email.md#eingehende-e-mails)). Das Token ist der Teil vor dem `@`, daher muss jede
   Adresse der Domain Yuva erreichen: Aktivieren Sie mit dem [Cloudflare Email Worker](../../edge/README.md)
   Email Routing auf der Domain und richten Sie die Catch-all-Regel auf den Worker. Der Worker braucht nichts
   Neues: Er signiert die Nachricht wie jede andere für `/ingress/email`, und Yuva findet den Kanal über das
   Token. Unbekannte Token werden abgelehnt.
3. Starten Sie neu, öffnen Sie **Settings → der Posteingang → Channels → E-mail** und prüfen Sie, dass der
   Kanal seine Adresse zeigt.

Eine durchgesickerte Weiterleitungsadresse lässt sich durch Rotieren ersetzen; E-Mails an die alte werden
danach abgelehnt.

## Einstellungen

Alle stehen in der [Konfigurationsreferenz](configuration.md#zu-yuva-umziehen). Was sie begrenzen:

| Einstellung | Begrenzt |
|---|---|
| `YUVA_MIGRATION_SOURCES` | Welche Quellen Teams starten können; eine andere anzufragen ergibt `422 hosted_limit`. |
| `YUVA_MIGRATION_WORKERS` | Umzüge, die pro Prozess gleichzeitig kopiert werden. Setzen Sie `0` auf Prozessen, die nicht kopieren sollen. |
| `YUVA_MIGRATION_MAX_ACTIVE` | Umzüge eines Workspace, die gleichzeitig scannen, kopieren oder abgleichen. |
| `YUVA_MIGRATION_MAX_BYTES` | Bytes, die ein Workspace im Monat importiert; am Limit wartet das Kopieren auf den nächsten Monat. |
| `YUVA_MIGRATION_MAX_MESSAGE_BYTES` | Eine einzelne Nachricht; größere werden übersprungen und gezählt. |
| `YUVA_MIGRATION_SYNC_MAX_DAYS` | Wie lange ein Umzug nach dem Kopieren das alte Postfach weiter liest, bevor er stoppt und seine Zugangsdaten löscht. |
| `YUVA_MIGRATION_AUDIT_DAYS` | Wie lange die Aktivität eines Umzugs nach seinem Ende aufbewahrt wird. |
| `YUVA_FORWARDING_DOMAIN` | Ob Kanäle Weiterleitungsadressen haben. |
| `YUVA_MIGRATION_ALLOW_PRIVATE` | Ob Mailserver in privaten Netzen erlaubt sind (unten). |

Jeder Verbindungstest ist auf 10 pro Umzug und 30 pro Workspace und Stunde begrenzt, und drei fehlgeschlagene
Anmeldungen in Folge stoppen Tests für 15 Minuten, damit der Server nicht zum Passwort-Raten bei einem Anbieter
taugt.

## Private Adressen

Yuva löst den Namen eines Mailservers einmal auf und lehnt ihn ab, wenn eine Adresse in einem privaten,
Loopback- oder lokalen Netz liegt, wie bei Webhooks und SMTP-Hosts. Link-local- und Cloud-Metadaten-Adressen
sind nie erlaubt. Setzen Sie `YUVA_MIGRATION_ALLOW_PRIVATE=true`, um von einem Server im eigenen Netz
umzuziehen oder Zu Yuva umziehen gegen einen lokalen Testserver auszuprobieren; der Server schreibt beim Start
eine Warnung. Auf einem geteilten Server lassen Sie es aus.

## Prüfen, dass das alte Postfach nur gelesen wird

Das Versprechen ruht auf drei Schichten ([was Yuva tut](migrate.md#was-yuva-mit-ihrem-postfach-tut-und-was-nicht)).
Jede können Sie in einem Checkout des Repositorys selbst prüfen.

- **Die Sperre.** In `api/` führt `go test ./...` `TestIMAPGuardRefusesWrites` (jeder schreibende Befehl, in
  jeder Schreibweise, wird abgelehnt, bevor ein Byte den Socket erreicht) und `TestOnlyImapreadImportsClient`
  (nur das Paket `imapread` darf den IMAP-Client nutzen, und nur mit schreibgeschützten Öffnungen und
  `PEEK`-Abrufen) aus. Schlägt eines fehl, schlägt der Build fehl. Hält die Sperre zur Laufzeit einen Befehl von
  einem Postfach ab, protokolliert der Umzug `source_write_refused`.
- **Ein Postfach, das alles mitschreibt.** Das Profil `migrate-test` in `deploy/compose.yaml` startet Dovecot
  mit Test-Postfächern und schreibt jede Sitzung nach `<fixtures>/<user>/rawlog`:

  ```sh
  cd api && go run ./internal/migrate/testdata/gen -out /tmp/yuva-migrate-fixtures
  cd ../deploy && docker compose --profile migrate-test up -d dovecot
  export YUVA_TEST_IMAP_URL=imaps://:migrate-test-only@127.0.0.1:59993
  ```

  Ziehen Sie von einem Entwicklungsserver ein Test-Postfach um (`YUVA_MIGRATION_ALLOW_PRIVATE` gesetzt) und
  listen Sie dann die Befehle auf, die der Server erhielt. Nur `CAPABILITY`, `NOOP`, `LOGOUT`, `STARTTLS`,
  `LOGIN`, `AUTHENTICATE`, `LIST`, `STATUS`, `EXAMINE`, `UNSELECT`, `SEARCH` und `FETCH` (auch als `UID …`)
  dürfen erscheinen:

  ```sh
  cat /tmp/yuva-migrate-fixtures/<user>/rawlog/*.in | tr -d '\r' \
    | awk 'NF >= 3 { c = toupper($3); if (c == "UID") c = c " " toupper($4); print c }' | sort -u
  ```

- **Das Postfach selbst.** Mit gesetztem `YUVA_TEST_IMAP_URL` zieht `go test ./internal/migrate/imapread -run
  TestDovecotSourceUnchanged` ein Test-Postfach um und vergleicht jede Nachricht, jedes Flag und jeden Zähler
  vorher und nachher.

## Zugangsdaten und Audit

Benutzername und Passwort eines Postfachs werden in dem Moment, in dem sie eingegeben werden, unter
`YUVA_MASTER_KEY` versiegelt und nur für einen Arbeitsabschnitt geöffnet. Sie erscheinen in keinem
Job-Argument, Ereignis, Log oder keiner Antwort. Yuva löscht sie, wenn der Umzug abgeschlossen oder
abgebrochen wird, wenn `YUVA_MIGRATION_SYNC_MAX_DAYS` verstrichen sind, wenn sie ersetzt werden und mit dem
Workspace. Sichern Sie den Master-Key wie jeden anderen ([Installationsanleitung](install.md#master-key)):
Ohne ihn lässt sich ein pausierter Umzug nicht fortsetzen.

Jede Aktion an einem Umzug (wer, von welchem Client, was) wird nach seinem Ende `YUVA_MIGRATION_AUDIT_DAYS`
lang als Aktivität aufbewahrt und vom Retention-Job entfernt. Logs nennen die IDs von Umzug und Workspace,
Zustände, Fehlercodes und Zahlen, nie Betreffs, Adressen oder Inhalte.

## Gehostete Limits

Setzen Sie auf einem Server, der Zu Yuva umziehen vielen Workspaces anbietet, die Limits, die zu Ihren
Tarifen passen. Eine Anfrage über einem Limit wird mit `422 hosted_limit` abgelehnt, und das Feld `limit` des
Problems nennt die Einstellung:

| `limit` | Wann |
|---|---|
| `YUVA_MIGRATION_SOURCES` | Der Workspace hat eine Quelle gewählt, die der Server nicht anbietet. |
| `YUVA_MIGRATION_MAX_ACTIVE` | Der Workspace startet einen weiteren Umzug, während er so viele wie erlaubt laufen lässt. |
| `YUVA_MIGRATION_MAX_BYTES` | Der Workspace hat diesen Monat so viele Bytes importiert wie erlaubt. Das Kopieren protokolliert einen Fehler `hosted_limit`, wartet bis zum Ersten des nächsten Monats (UTC) und läuft von selbst weiter. |

## Kapazität

Ein Umzug liest über eine Verbindung und kopiert im Tempo des Budgets seiner Quelle: 1.500 MB pro Tag bei Gmail
und Google Workspace, 4 MB pro Sekunde bei den übrigen. Langsame Quellen, nicht Ihr Server, bestimmen, wie lange
ein großes Postfach dauert; jeder Prozess führt `YUVA_MIGRATION_WORKERS` Umzüge gleichzeitig aus, erhöhen Sie
den Wert also mit der Zahl der Workspaces, die Sie gleichzeitig erwarten. Importierte E-Mails belegen denselben
Speicher wie Live-E-Mails; „Keep original messages“ speichert zusätzlich die Quelle jeder Nachricht.

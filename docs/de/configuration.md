# Konfiguration

Yuva liest seine Einstellungen beim Start aus Umgebungsvariablen. Ein falscher Wert hält den Server
an, mit einer Meldung, die die Variable nennt.

Einstellungen eines Workspaces, Posteingangs oder Kanals liegen im Panel, in der API und in den
[Operator-Befehlen](operations.md#operator-befehle), nicht hier.

## Einen Wert setzen

1. Tragen Sie die Variable in `.env` neben `compose.yaml` ein:

   ```sh
   YUVA_CHAT_EMAIL_DELAY=10m
   ```

2. Starten Sie den Server neu:

   ```sh
   docker compose up -d
   ```

Eine leere Variable gilt als nicht gesetzt und nimmt den Standardwert, außer bei
`YUVA_METRICS_ADDR`. Listen sind kommagetrennt. Boolesche Werte akzeptieren `true`/`false`, `1`/`0`
und `t`/`f`.

## Erforderlich

| Variable | Bedeutung |
|---|---|
| `YUVA_DATABASE_URL` | Postgres-16+-URL, z. B. `postgres://yuva:secret@db:5432/yuva?sslmode=disable`. |
| `YUVA_MASTER_KEY` | 32 Zufallsbytes, Base64 (`openssl rand -base64 32`). Verschlüsselt gespeicherte Geheimnisse. Siehe [Master-Key](install.md#master-key). |
| `YUVA_PUBLIC_URL` | Die URL, die Menschen und Apps verwenden, z. B. `https://support.example.com`, im Wurzelpfad ihres Hosts. Standard `http://localhost:8080`, nur zum lokalen Ausprobieren. |

## Server

| Variable | Standard | Bedeutung |
|---|---|---|
| `YUVA_LISTEN_ADDR` | `:8080` | HTTP-Adresse für alles außer Metriken. |
| `YUVA_METRICS_ADDR` | `:9090` | Prometheus-Adresse (`GET /metrics`). Halten Sie sie privat. `off` oder ein leerer Wert schaltet sie ab. |
| `YUVA_CLIENT_IP_HEADER` | nicht gesetzt | Header, in den Ihr Proxy die Client-IP schreibt, z. B. `X-Real-IP`, für Rate Limits. In einer Liste wie `X-Forwarded-For` zählt die Adresse ganz rechts, die nicht in `YUVA_TRUSTED_PROXIES` steht. |
| `YUVA_TRUSTED_PROXIES` | nicht gesetzt | Adressen oder CIDR-Bereiche Ihrer Proxys, z. B. `10.0.0.0/8`. Dann wird der Client-IP-Header nur von ihnen gelesen. |
| `YUVA_COOKIE_SECURE` | `true`, wenn `YUVA_PUBLIC_URL` `https` ist | Markiert das Sitzungs-Cookie als `Secure`. |
| `YUVA_MCP` | `on` | `off` schaltet den MCP-Endpunkt `/mcp` ab (er antwortet mit `404`). OAuth bleibt an. |
| `YUVA_PANEL` | `on` | `off` liefert das Panel und die OAuth-Zustimmung nicht mehr aus; API-Schlüssel funktionieren weiter. |
| `YUVA_WIDGET` | `on` | `off` liefert `/yuva.js`, `/yuva-chat.js` und `/yuva-docs.js` nicht mehr aus; `/client/v1` bleibt. |

## Anmeldung und Passkeys

| Variable | Standard | Bedeutung |
|---|---|---|
| `YUVA_WEBAUTHN_RP_ID` | Hostname von `YUVA_PUBLIC_URL` | Relying-Party-ID für Passkeys. Eine Änderung macht vorhandene Passkeys unbrauchbar. |
| `YUVA_WEBAUTHN_RP_NAME` | `Yuva` | Name, den Browser für einen neuen Passkey anzeigen. |
| `YUVA_WEBAUTHN_ORIGINS` | Origin von `YUVA_PUBLIC_URL` | Erlaubte Origins für Passkeys und den Realtime-Socket des Panels. |
| `YUVA_SIGNUP` | `invite` | `invite`: Nur bestehende Personen und eingeladene Adressen erhalten einen Anmeldecode; Workspaces entstehen mit `yuva bootstrap`. `open`: Jeder kann sich mit einem Code anmelden und im Panel Workspaces anlegen. |
| `YUVA_SIGNUP_WORKSPACES_PER_PERSON` | `3` | Bei `open`: Workspaces, die eine Person angelegt und nicht gelöscht hat; weitere erhalten `403`. |
| `YUVA_SIGNUP_WORKSPACES_PER_IP` | `5` | Bei `open`: Workspaces, die von einer IP-Adresse (IPv6 /64) in 24 Stunden angelegt werden; weitere erhalten `429`. |
| `YUVA_OPERATOR_EMAIL` | nicht gesetzt | Adressen, kommagetrennt, die eine E-Mail erhalten, wenn sich eine neue Person mit einem Code registriert (Leser als solche markiert) oder einen Workspace anlegt: Adresse, Workspace-Name, Zeit und Land (hinter Cloudflare, mit `YUVA_CLIENT_IP_HEADER=CF-Connecting-IP`) oder sonst die IP-Adresse. Höchstens eine E-Mail alle 5 Minuten, mit allem seit der vorigen. Auf Englisch, über das SMTP des Servers. Dieselbe E-Mail listet auch [Freigabeanfragen, Sperren und automatische Pausen](operations.md#broadcasts-auf-einem-gehosteten-server) von Broadcasts. Nicht gesetzt heißt aus. |

## Server-E-Mail

Anmeldecodes, Einladungen und Benachrichtigungen. E-Mail-Kanäle haben [ihr eigenes SMTP](email.md#ausgehendes-smtp).

| Variable | Standard | Bedeutung |
|---|---|---|
| `YUVA_SMTP_HOST` | nicht gesetzt | SMTP-Server. Ohne Wert werden E-Mails stattdessen ins Log geschrieben. |
| `YUVA_SMTP_PORT` | `465` mit `tls`, sonst `587` | SMTP-Port. |
| `YUVA_SMTP_TLS` | `starttls` | `starttls`, `tls` (implizites TLS) oder `none`. |
| `YUVA_SMTP_USERNAME` | nicht gesetzt | SMTP-Benutzername. |
| `YUVA_SMTP_PASSWORD` | nicht gesetzt | SMTP-Passwort. |
| `YUVA_SMTP_FROM` | nicht gesetzt | Absender, z. B. `Yuva <yuva@example.com>`. Pflicht zusammen mit `YUVA_SMTP_HOST`. |
| `YUVA_SMTP_ALLOW_PRIVATE` | `false` | Erlaubt E-Mail-Kanälen SMTP-Hosts auf Loopback- und privaten Adressen, etwa Mailpit. |

## Dateien und Speicher

| Variable | Standard | Bedeutung |
|---|---|---|
| `YUVA_STORAGE` | `local` | `local` (ein Verzeichnis) oder `s3` (S3, R2, MinIO, …). |
| `YUVA_STORAGE_DIR` | `data/attachments` | Verzeichnis für `local`; `/data/attachments` im Docker-Image. |
| `YUVA_S3_ENDPOINT` | nicht gesetzt | Endpunkt-URL, z. B. `https://s3.eu-central-1.amazonaws.com`. |
| `YUVA_S3_REGION` | `auto` | Region. |
| `YUVA_S3_BUCKET` | nicht gesetzt | Bucket. |
| `YUVA_S3_ACCESS_KEY_ID` | nicht gesetzt | Access-Key-ID. |
| `YUVA_S3_SECRET_ACCESS_KEY` | nicht gesetzt | Secret Access Key. |
| `YUVA_S3_PATH_STYLE` | `false` | URLs im Path-Style, die MinIO meist braucht. |
| `YUVA_ATTACHMENT_MAX_BYTES` | `26214400` (25 MiB) | Größter Anhang in Bytes. |
| `YUVA_ATTACHMENT_TYPES` | siehe unten | Erlaubte Content-Types; Platzhalter wie `image/*` funktionieren. |
| `YUVA_EMAIL_IMAGES_MAX_BYTES` | `524288000` (500 MiB) | Gesamtgröße der Bilder der [E-Mail-Designs](designs.md) eines Workspace in Bytes; `0` bedeutet keine Grenze. Weitere Uploads werden abgelehnt (`409 image_quota`). |

Standardtypen: `image/png,image/jpeg,image/gif,image/webp,image/heic,application/pdf,text/plain,text/csv,application/zip,application/json,video/mp4,video/quicktime,audio/mpeg,audio/mp4`.

## Eingehende E-Mail

| Variable | Standard | Bedeutung |
|---|---|---|
| `YUVA_INGRESS_SECRET` | nicht gesetzt | Signiert Anfragen an `/ingress/email`; der Email Worker verwendet denselben Wert. Ohne Wert lehnt dieser Endpunkt jede E-Mail ab. |
| `YUVA_INGRESS_ACCEPT_V1` | `false` | Akzeptiert zusätzlich die alte `v1`-Signatur, bis Sie einen älteren Email Worker neu deployen. |
| `YUVA_INGRESS_AUTHSERV_ID` | nicht gesetzt | Authserv-ID Ihres empfangenden Mailservers, z. B. `mx.cloudflare.net`. Nötig, damit DMARC-Ergebnisse genutzt werden. |
| `YUVA_INGRESS_MAX_CONCURRENT` | `8` | Nachrichten, die `/ingress/email` pro Prozess gleichzeitig verarbeitet; weitere erhalten `503`. |
| `YUVA_SES_TOPIC_ARNS` | nicht gesetzt | SNS-Topic-ARNs, deren SES-Bounces und -Beschwerden `/ingress/ses` annimmt. |
| `YUVA_EMAIL_SENDER_HOURLY_CAP` | `500` | Eingehende E-Mails, die ein Absender pro Stunde an einen Workspace senden darf; weitere werden abgelehnt (`429`). |

## Zu Yuva umziehen

Für das [Umziehen eines Postfachs zu Yuva](migrate.md). Die Einstellungen, die einen Workspace begrenzen,
gelten für gehostete Server; eine Anfrage über einer Grenze wird mit `422 hosted_limit` abgelehnt, das die
Einstellung nennt.

| Variable | Standard | Bedeutung |
|---|---|---|
| `YUVA_FORWARDING_DOMAIN` | nicht gesetzt | Ein Domainname wie `forward.example.com`, dessen E-Mails Sie wie die Domains der Kanäle auf Yuva ausrichten (siehe [Eingehende E-Mails](email.md#eingehende-e-mails)). Dann hat jeder E-Mail-Kanal eine [Weiterleitungsadresse](email.md#weiterleitungsadresse) `<Token>@` dieser Domain. Nicht gesetzt, gibt es keine Weiterleitungsadressen, und der Umzug eines Postfachs stellt E-Mails nur per MX um. |
| `YUVA_MIGRATION_SOURCES` | alle gebauten | Kommagetrennte Quellen, von denen Teams umziehen dürfen: `imap`, `file`, `zendesk`, `helpscout`. Bisher gebaut sind Postfächer über IMAP und Exportdateien. Leer schaltet den Umzug aus. |
| `YUVA_MIGRATION_WORKERS` | `2` | Umzüge, die pro Prozess gleichzeitig kopiert werden; `0` führt in diesem Prozess keine aus. |
| `YUVA_MIGRATION_MAX_ACTIVE` | `0` | Höchstzahl der Umzüge eines Workspace, die gleichzeitig scannen, kopieren oder synchronisieren; `0` heißt unbegrenzt. |
| `YUVA_MIGRATION_MAX_BYTES` | `0` | Höchstzahl Bytes, die ein Workspace pro Monat importieren darf; `0` heißt unbegrenzt. |
| `YUVA_MIGRATION_MAX_MESSAGE_BYTES` | `52428800` (50 MiB) | Größte importierte Nachricht; größere werden übersprungen. |
| `YUVA_MIGRATION_UPLOAD_MAX_BYTES` | `21474836480` (20 GiB) | Größte [Exportdatei](migrate-files.md), die ein Umzug annimmt; eine größere wird mit `422 hosted_limit` abgelehnt. `0` schaltet Dateiimporte aus. |
| `YUVA_MIGRATION_SYNC_MAX_DAYS` | `30` | Tage, die ein Umzug nach dem Kopieren das alte Postfach weiterliest; danach stoppt er und seine Zugangsdaten werden gelöscht. 1 bis 365. |
| `YUVA_MIGRATION_AUDIT_DAYS` | `365` | Tage, die die Aktivität eines Umzugs nach seinem Ende aufbewahrt wird. 1 bis 36500. |
| `YUVA_MIGRATION_ALLOW_PRIVATE` | `false` | Erlaubt einem Umzug auch die Verbindung zu Mailservern in einem privaten oder lokalen Netz (Entwicklung, Server im eigenen Netz). |

## Broadcasts

[Broadcasts versenden](broadcasts.md). Listen und ihre Formulare funktionieren unabhängig davon, was hier steht.

| Variable | Standard | Bedeutung |
|---|---|---|
| `YUVA_BROADCASTS` | `approval` bei `YUVA_SIGNUP=open`, sonst `on` | `on`: jeder Workspace sendet. `approval`: ein über die Anmeldung angelegter Workspace sendet erst nach Freigabe durch den Operator, und jeder Workspace unterliegt einem [Warm-up-Limit](broadcasts.md#freigabe-auf-gehosteten-servern). `off`: Broadcasts anlegen, ändern und senden antwortet mit `403 broadcasts_disabled`. |
| `YUVA_BROADCAST_WORKERS` | `2` | Broadcast-Versandjobs, die ein Prozess gleichzeitig ausführt, je einer für einen sendenden Kanal. Support-Antworten haben eine eigene Queue und werden nicht langsamer. Mindestens 1. |
| `YUVA_BROADCAST_DAILY_RECIPIENTS` | `0` (kein Limit) | Höchstzahl der Empfänger der Broadcasts eines Workspaces pro UTC-Tag. Ein Versand, der es erreicht, geht am nächsten Tag weiter. Der Operator kann pro Workspace ein kleineres Limit setzen ([Betrieb](operations.md#broadcasts-auf-einem-gehosteten-server)). |
| `YUVA_IMPORT_MAX_ROWS` | `100000` | Höchstzahl der Datenzeilen einer CSV-Datei beim [Importieren von Abonnenten](importing.md). Von 1 bis 100000; ein höherer Wert verhindert den Start des Servers. Eine Datei darüber schlägt fehl, und nichts wird eingetragen. |
| `YUVA_MAIL_LINK_URL` | `YUVA_PUBLIC_URL` | Ursprung (Schema und Host, kein Pfad), auf den die Abmelde-, Bestätigungs-, Klick- und Bild-Links in Listen- und Broadcast-Mails zeigen, z. B. `https://links.example.com`. Er muss diesen Server erreichen. Dieser Host beantwortet nur diese Links (`/m/`, `/s/`, `/c/`, `/i/`) und liefert für alles andere `404`; Panel und API bleiben auf Ihrem Haupt-Host. |

## Chat

| Variable | Standard | Bedeutung |
|---|---|---|
| `YUVA_CHAT_EMAIL_DELAY` | `5m` | Wie lange eine Antwort ungelesen bleibt, bevor sie einem Kontakt, der den Chat verlassen hat, per E-Mail geschickt wird. Mindestens `1s`. |
| `YUVA_DISCUSSION_EMAIL_DELAY` | `10m` | Pro Follower und Diskussionsthread in dieser Zeit höchstens eine E-Mail mit den neuen Antworten und der akzeptierten Antwort. Mindestens `1s`. |
| `YUVA_ANONYMOUS_CONTACTS_PER_HOUR` | `20` | Neue anonyme Besucher pro IP-Adresse (IPv6 /64), Kanal und Stunde; weitere erhalten `429`. |

## Webhooks

| Variable | Standard | Bedeutung |
|---|---|---|
| `YUVA_WEBHOOK_ALLOW_PRIVATE` | `false` | Erlaubt Webhooks, Loopback- und private Adressen zu erreichen. Nur für die Entwicklung. |

## Web Push

| Variable | Standard | Bedeutung |
|---|---|---|
| `YUVA_VAPID_PUBLIC_KEY` | nicht gesetzt | Öffentlicher VAPID-Schlüssel. Setzen Sie beide Schlüssel oder keinen; `yuva vapid-keys` gibt ein Paar aus. |
| `YUVA_VAPID_PRIVATE_KEY` | nicht gesetzt | Privater VAPID-Schlüssel. |
| `YUVA_VAPID_SUBJECT` | `mailto:` + Adresse aus `YUVA_SMTP_FROM`, sonst `YUVA_PUBLIC_URL`, wenn `https` | Kontakt für Push-Dienste: `mailto:…` oder `https://…`. |

## Build

Die Version, die der Server meldet (`GET /v1/version`), wird beim Build mit dem Docker-Build-Argument
`VERSION` festgelegt. Sie ist keine Umgebungsvariable.

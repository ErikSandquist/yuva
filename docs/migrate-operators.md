# Move to Yuva for operators

How to run [Move to Yuva](migrate.md) on your own server: the forwarding domain, the settings, what to
check to trust that the old mailbox is only read, and the limits for a server shared by many workspaces.
IMAP mailboxes are the only source built so far.

## The forwarding domain

Moving a mailbox works without one: teams switch by changing MX records. With a forwarding domain every
e-mail channel also gets a [forwarding address](email.md#forwarding-address) `<token>@<forwarding domain>`,
which a team can forward its old mailbox to.

1. Pick a name no other mail server handles, such as `forward.example.com`, and set it on the server:

   ```sh
   YUVA_FORWARDING_DOMAIN=forward.example.com
   ```

2. Send all mail for that domain to Yuva, the same way as a channel's own domain (the three ways under
   [Inbound](email.md#inbound)). The token is the part before the `@`, so every address of the domain must
   reach Yuva: with the [Cloudflare Email Worker](../edge/README.md), enable Email Routing on the domain and
   point its catch-all rule at the Worker. The Worker needs nothing new: it signs the message for
   `/ingress/email` like any other, and Yuva finds the channel from the token. Unknown tokens are refused.
3. Restart, open **Settings → the inbox → Channels → E-mail** and check that the channel shows its address.

A forwarding address that leaks can be replaced by rotating it; mail to the old one is refused from then
on.

## Registering the Microsoft app

Outlook.com and Microsoft 365 mailboxes connect with "Sign in with Microsoft" ([Outlook.com](migrate-outlook.md),
[Microsoft 365](migrate-microsoft365.md)). Without an app of its own, your server makes every team register
one; with one, teams only press the button. To give the server one:

1. In the [Microsoft Entra admin center](https://entra.microsoft.com) register an application under **App
   registrations → New registration**. As the supported account types choose **Any Entra ID Tenant + Personal
   Microsoft accounts**, so that work, school and personal mailboxes all work. Microsoft's page:
   [Register an application](https://learn.microsoft.com/en-us/entra/identity-platform/quickstart-register-app).
2. Add a **Web** redirect URI: your public URL followed by `/migrations/sign-in/callback`, for example
   `https://yuva.example.com/migrations/sign-in/callback`.
3. Add a client secret under **Certificates & secrets**. Note when it expires and replace it before: from
   then on sign-ins fail.
4. Under **API permissions** add the delegated permission **IMAP.AccessAsUser.All** of **Office 365
   Exchange Online** and **offline_access** of Microsoft Graph. Yuva asks for exactly these two and refuses
   a sign-in that grants more.
5. Set the app's ID and secret and restart:

   ```sh
   YUVA_MICROSOFT_CLIENT_ID=<Application (client) ID>
   YUVA_MICROSOFT_CLIENT_SECRET=<secret value>
   ```

The mailbox owner signs in to this app and consents; a Microsoft 365 organization that lets only admins approve
apps (or only apps of verified publishers) is shown a link for its admin
([how](migrate-microsoft365.md#3-when-microsoft-asks-for-an-admin)). For an app limited to one organization or
to personal accounts, also set `YUVA_MICROSOFT_TENANT`. Both settings are in the
[configuration reference](configuration.md#moving-to-yuva); the secret is sealed under the master key like a
mailbox's credentials.

## Settings

All are in the [configuration reference](configuration.md#moving-to-yuva). What they limit:

| Setting | Limits |
|---|---|
| `YUVA_MIGRATION_SOURCES` | Which sources teams can start; a request for another is `422 hosted_limit`. |
| `YUVA_MIGRATION_WORKERS` | Migrations copied at once per process. Set `0` on processes that should not copy. |
| `YUVA_MIGRATION_MAX_ACTIVE` | Migrations of one workspace that scan, copy or sync at the same time. |
| `YUVA_MIGRATION_MAX_BYTES` | Bytes one workspace imports in a month; the copy waits for the next month when it is reached. |
| `YUVA_MIGRATION_MAX_MESSAGE_BYTES` | One message; larger ones are skipped and counted. |
| `YUVA_MIGRATION_SYNC_MAX_DAYS` | How long a migration keeps reading the old mailbox after the copy before it stops and deletes its credentials. |
| `YUVA_MIGRATION_AUDIT_DAYS` | How long a migration's activity is kept after it ended. |
| `YUVA_FORWARDING_DOMAIN` | Whether channels have forwarding addresses. |
| `YUVA_MIGRATION_ALLOW_PRIVATE` | Whether mail servers on private networks are allowed (below). |

Each connection test is limited to 10 per migration and 30 per workspace an hour, and three failed
sign-ins in a row stop tests for 15 minutes, so the server cannot be used to guess passwords at a provider.

## Private addresses

Yuva resolves a mail server's name once and refuses it when any address is on a private, loopback or
local network, as it does for webhooks and SMTP hosts. Link-local and cloud metadata addresses are never
allowed. Set `YUVA_MIGRATION_ALLOW_PRIVATE=true` to move from a server on your own network or to try
Move to Yuva against a local test server; the server logs a warning at start. On a shared server leave it
off.

## Checking that the old mailbox is only read

The promise rests on three layers ([what Yuva does](migrate.md#what-yuva-does-and-does-not-do-with-your-mailbox)).
You can check each one yourself in a checkout of the repository.

- **The guard.** In `api/`, `go test ./...` runs `TestIMAPGuardRefusesWrites` (every command that writes,
  in every spelling, is refused before a byte reaches the socket) and `TestOnlyImapreadImportsClient` (only
  the `imapread` package may use the IMAP client, and only with read-only opens and `PEEK` fetches). If one
  fails, the build fails. When the guard keeps a command from a mailbox at run time, the migration records
  `source_write_refused`.
- **A mailbox that logs everything.** The `migrate-test` profile of `deploy/compose.yaml` runs Dovecot with
  fixture mailboxes and writes every session to `<fixtures>/<user>/rawlog`:

  ```sh
  cd api && go run ./internal/migrate/testdata/gen -out /tmp/yuva-migrate-fixtures
  cd ../deploy && docker compose --profile migrate-test up -d dovecot
  export YUVA_TEST_IMAP_URL=imaps://:migrate-test-only@127.0.0.1:59993
  ```

  Run a migration of a fixture mailbox from a development server (set `YUVA_MIGRATION_ALLOW_PRIVATE`), then
  list the commands the server received. Only `CAPABILITY`, `NOOP`, `LOGOUT`, `STARTTLS`, `LOGIN`,
  `AUTHENTICATE`, `LIST`, `STATUS`, `EXAMINE`, `UNSELECT`, `SEARCH` and `FETCH` (also as `UID …`) may
  appear:

  ```sh
  cat /tmp/yuva-migrate-fixtures/<user>/rawlog/*.in | tr -d '\r' \
    | awk 'NF >= 3 { c = toupper($3); if (c == "UID") c = c " " toupper($4); print c }' | sort -u
  ```

- **The mailbox itself.** With `YUVA_TEST_IMAP_URL` set, `go test ./internal/migrate/imapread -run
  TestDovecotSourceUnchanged` migrates a fixture mailbox and compares every message, flag and counter
  before and after.

## Credentials and audit

A mailbox's user name and password are sealed under `YUVA_MASTER_KEY` the moment they are entered, and
only opened for a slice of work. They appear in no job argument, event, log or answer. Yuva deletes them
when the migration completes or is cancelled, when `YUVA_MIGRATION_SYNC_MAX_DAYS` pass, when they are
replaced and with the workspace. Back up the master key like any other ([install guide](install.md#master-key)):
without it a paused migration cannot continue.

Every action on a migration (who, from which client, what) is kept as its activity for
`YUVA_MIGRATION_AUDIT_DAYS` after it ends, and removed by the retention job. Logs name the migration and
workspace ids, states, error codes and counts, never subjects, addresses or bodies.

## Hosted limits

On a server that offers Move to Yuva to many workspaces, set the limits that fit your plans. A request over
one is refused with `422 hosted_limit`, and the problem's `limit` names the setting:

| `limit` | When |
|---|---|
| `YUVA_MIGRATION_SOURCES` | The workspace chose a source the server does not offer. |
| `YUVA_MIGRATION_MAX_ACTIVE` | The workspace starts another migration while it runs as many as allowed. |
| `YUVA_MIGRATION_MAX_BYTES` | The workspace imported as many bytes this month as allowed. The copy records a `hosted_limit` error, waits until the first of the next month (UTC) and continues by itself. |

## Capacity

A migration reads through one connection and copies at the speed of its source's budget: 1,500 MB a day
for Gmail and Google Workspace, 4 MB a second for the others. Slow sources, not your server, decide how long
a large mailbox takes; every process runs `YUVA_MIGRATION_WORKERS` migrations at the same time, so raise it
with the number of workspaces you expect to move at once. Imported mail uses the same storage as live mail, and
"Keep original messages" stores each message's source on top.

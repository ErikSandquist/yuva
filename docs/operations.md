# Operations

How to upgrade, back up, restore and watch a Yuva server. The examples use the Compose setup from
the [install guide](install.md) and run from its directory.

## Upgrades

1. Read the release notes of every version between yours and the new one.
2. [Back up](#backups) the database and the attachments.
3. Change the image tag in `compose.yaml` and start the new version:

   ```sh
   docker compose up -d
   ```

Migrations run on start; the server is ready when `/readyz` answers `200`. Run one server process
while upgrading. To go back, restore the backup from step 2.

## Backups

A full backup has three parts: the database, the attachments and the master key.

1. Dump the database. `pg_dump` takes a consistent snapshot while Yuva runs:

   ```sh
   docker compose exec -T db pg_dump -U yuva -d yuva -Fc > yuva-$(date +%F).dump
   ```

2. Then archive the attachments. With `YUVA_STORAGE=local`:

   ```sh
   docker run --rm -v yuva_attachments:/data:ro -v "$PWD":/backup alpine \
     tar czf /backup/yuva-attachments-$(date +%F).tgz -C /data .
   ```

   With S3, use the provider's versioning or replication, or copy the bucket with `rclone sync`.

3. Keep `YUVA_MASTER_KEY` apart from both ([Master key](install.md#master-key)).

Run steps 1 and 2 from cron, in that order, and copy the files off the host.

## Restore

Use the same `YUVA_MASTER_KEY` the backup was made with:

```sh
docker compose stop yuva
docker compose exec -T db sh -c 'dropdb -U yuva --force yuva && createdb -U yuva yuva'
docker compose exec -T db pg_restore -U yuva -d yuva --no-owner < yuva-2026-10-07.dump
docker run --rm -v yuva_attachments:/data -v "$PWD":/backup alpine \
  sh -c 'rm -rf /data/* && tar xzf /backup/yuva-attachments-2026-10-07.tgz -C /data'
docker compose start yuva
```

A backup from an older version is migrated forward when the server starts.

## Operator commands

Run them with `docker compose exec yuva /yuva <command>` (`-T` when piping). `<ws>` is a workspace id
or exact name.

```sh
docker compose exec -T yuva /yuva api-key create --workspace Example --name provisioning
```

Commands that print an id or a secret write only that to stdout.

| Command | What it does |
|---|---|
| `serve [--migrate=false]` | Runs the server (the default), after migrations unless `--migrate=false`. |
| `migrate up\|down\|status` | Applies pending migrations, rolls back the latest one, or lists them. |
| `bootstrap --email <address> --workspace <name> [--name <name>] [--locale en\|tr] [--allow-existing]` | Creates a workspace and its first owner. `--allow-existing` adds another workspace. |
| `api-key create --workspace <ws> --name <name> [--scope <scope>]... [--inbox <id>]...` | Creates an API key and prints the secret once. Default: every scope, every inbox. |
| `api-key list --workspace <ws>` | Lists keys, never their secrets. |
| `api-key revoke <id>` | Revokes a key. |
| `inbox create --workspace <ws> --name <name> [--slug <slug>] [--locale <tag>] [--timezone <zone>] [--mode live\|async] [--expected-reply-minutes <n>]` | Creates an inbox and prints its id. Defaults: slug from the name, `en`, `UTC`, `async`. |
| `inbox list --workspace <ws>` | Lists inboxes. |
| `channel create-email --workspace <ws> --inbox <id\|slug> --name <name> --address <address> [--display-name <name>] [--from-address <address>] [--smtp-host <host> [--smtp-port <n>] [--tls starttls\|tls\|none] [--smtp-username <name>] [--smtp-password-file <path\|->]]` | Creates an e-mail channel and prints its id. Password from a file, or stdin with `-`. |
| `channel create-chat --workspace <ws> --inbox <id\|slug> --name <name> --origin <origin>... [--allow-anonymous] [--ask-email-offline=false] [--greeting <text>]` | Creates a chat (web widget) channel and prints its id and public key (`yuva_pk_…`), separated by a tab. Repeat `--origin` for each site. |
| `channel list --workspace <ws> --inbox <id\|slug>` | Lists an inbox's channels, with a chat channel's public key and origins. |
| `workspace delete --workspace <ws> --yes` | Deletes a workspace ([below](#deleting-a-workspace-or-an-account)). Without `--yes`, a dry run. |
| `workspace broadcasts approve\|deny\|suspend --workspace <ws> [--reason <text>]` | Answers a workspace's broadcast approval, or suspends it ([below](#broadcasts-on-a-hosted-server)). |
| `workspace broadcasts limit --workspace <ws> --daily <n>` | Sets the workspace's recipients per UTC day; `0` removes the limit. |
| `workspace broadcasts show --workspace <ws>` | Prints its status, limit, 30-day rates and latest broadcasts. |
| `person delete --email <address> --yes` | Deletes a person's account. Refused while they are a workspace's only owner. |
| `ingest-email --to <address> [--from <address>]` | Delivers a raw message from stdin ([E-mail](email.md#any-mta-yuva-ingest-email)). |
| `vapid-keys` | Prints a new Web Push key pair. Needs no database. |

Everything else (chat and app channels, members, webhooks, identity secrets) is managed in the
panel or through `/v1`.

## Broadcasts on a hosted server

How [broadcasts](broadcasts.md) are limited is set by `YUVA_BROADCASTS` ([Configuration](configuration.md#broadcasts)).
With `on` there is nothing to do. With `approval`, which is the default when `YUVA_SIGNUP=open`, a
workspace created through sign-up cannot schedule broadcasts until you approve it, and every workspace
warms up from 500 recipients a day, doubling after each clean day. Workspaces made with
`yuva bootstrap` before or after are `allowed`, but still warm up.

Set `YUVA_OPERATOR_EMAIL` so you hear about it. The digest (at most one e-mail every 5 minutes) lists,
without mail content or recipients' addresses:

| Line | When |
|---|---|
| Broadcast approval requested | An owner asked: the workspace name, the person's address, the website, who sends and what, how subscribers agreed. |
| Workspace suspended from broadcasts | The server suspended it because of its 30-day complaint or bounce rate, with the rates. |
| Broadcast paused | A broadcast paused itself (bounce rate, complaint rate or sender error), with the broadcast id and the reason. |

A workspace is suspended when at least 0.3% of at least 1,000 broadcast mails of the last 30 days were
reported as spam, or at least 8% of at least 2,000 bounced. Its running broadcasts pause.

Answer with the CLI. A workspace is a name or an id, and `--reason` (up to 500 characters) is shown to
the workspace's owners:

```sh
docker compose exec -T yuva /yuva workspace broadcasts approve --workspace Example
docker compose exec -T yuva /yuva workspace broadcasts deny --workspace Example --reason "No subscribe form on the site"
docker compose exec -T yuva /yuva workspace broadcasts suspend --workspace Example --reason "Complaints"
docker compose exec -T yuva /yuva workspace broadcasts limit --workspace Example --daily 2000
docker compose exec -T yuva /yuva workspace broadcasts show --workspace Example
```

| Command | Effect |
|---|---|
| `approve` | The workspace may send. Also lifts a suspension; broadcasts that paused stay paused until a member resumes them. |
| `deny` | The request is answered with no; the workspace may ask again. |
| `suspend` | Refuses scheduling and resuming, and pauses the running broadcasts (reason `workspace_suspended`). |
| `limit --daily <n>` | The workspace's recipients per UTC day, across its broadcasts. The smallest of this, `YUVA_BROADCAST_DAILY_RECIPIENTS` and the warm-up applies. `0` removes this limit only. |
| `show` | The status and its reason, the daily limit and today's count, the last 30 days (sent, bounce and complaint rates), the sender domains it used and its latest 10 broadcasts. |

Only the operator lifts a suspension. Before approving, look at the website and how the subscribers
agreed.

## Deleting a workspace or an account

**A workspace** is deleted by an owner under **Settings → Workspace → Danger zone**, with
`DELETE /v1/workspace` and `{"name": "<exact name>"}`, or with `yuva workspace delete`. It stops
working at once; a background job then deletes its data and logs `workspace deletion` lines. People
keep their accounts.

**An account** is deleted by its person under **Settings → My profile**, with `DELETE /v1/me` and
`{"email": "<their address>"}`, or with `yuva person delete`. Their messages stay without an
author. A workspace's only owner must hand over ownership or delete the workspace first.

## Retention

Conversations, messages and files stay until their contact is deleted (in the panel,
`DELETE /v1/contacts/{contactId}` or `DELETE /v1/contacts/by-external-id`). An owner can set a
retention period under **Settings → Workspace**, or with `PATCH /v1/workspace` and
`{"retention_days": 90}`; `null` keeps everything (the default).

Yuva cleans up once an hour:

| Data | Kept |
|---|---|
| Events (realtime replay and `GET /v1/events`) | 7 days |
| Contact sessions | until they expire, 7 days after last use |
| Finished webhook deliveries | 7 days; the newest 100 per endpoint stay |
| Stale realtime connection records | 1 hour |
| Closed conversations, with messages and files (when `retention_days` is set) | that many days since their last change |
| Recipients of a finished broadcast (its counts stay) | 400 days after it finished |
| Daily sending totals behind the limits | 90 days |
| Original `.eml` of inbound mail (when `retention_days` is set) | that many days; the message itself stays |

## Monitoring

| Endpoint | Port | Answers |
|---|---|---|
| `GET /healthz` | 8080 | `200` while the process runs. Liveness check. |
| `GET /readyz` | 8080 | `200` when the database is reachable, else `503`. Readiness and uptime check. |
| `GET /v1/version` | 8080 | The running version. |
| `GET /metrics` | 9090 | Prometheus: `yuva_http_requests_total`, `yuva_http_request_seconds`, Go and process metrics. |

Logs are JSON lines on stderr. Lines worth an alert:

| Log message | Meaning |
|---|---|
| `mail not sent` | The server's SMTP account fails; sign-in codes do not arrive. |
| `email send failed, retrying` | A channel's SMTP account fails. |
| `YUVA_SMTP_HOST is not set` (and the same for `YUVA_INGRESS_SECRET`, `YUVA_VAPID_PUBLIC_KEY`) | A feature is off by configuration. |
| `not ready` | The database is unreachable. |

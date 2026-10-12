# Yuva threat model

Yuva is open-source customer messaging: one inbox for e-mail, live chat and in-app conversations,
with broadcasts (newsletters), feedback and discussions on documentation pages, moving mail and
help-desk history in from other services, and cookieless web analytics. One server hosts many
workspaces; hosted servers allow open sign-up, so a workspace's members must be treated as
potentially hostile to every other workspace.

Parts: the Go server (`api/`, which also serves the panel and the hosted scripts), the React panel
(`web/`), the browser SDK (`sdk/js/`, web components in Shadow DOM embedded on customers' sites), the
Go SDK (`sdk/go/`), the mobile SDKs (`sdk/swift/`, `sdk/kotlin/`, not built in this image) and a
Cloudflare Email Worker (`edge/`) that forwards inbound mail to the server. `openapi/openapi.yaml`
lists every operation with its scope; `docs/architecture.md` describes the design and its rules.

## Where untrusted input enters

- **Visitors on customers' sites**: `/client/v1/*` with a channel's public key (chat sessions,
  messages and attachments, feedback, discussions, subscribe forms) and
  `POST /client/v1/analytics/{key}`. Public keys are public by design; origins are checked against
  the channel's or property's domains. Signed-in contacts use identity tokens (HS256, signed by the
  customer's backend with an inbox secret).
- **Inbound e-mail from anyone**: raw MIME through `POST /ingress/email` (HMAC from the edge Worker),
  parsed in `api/internal/email`, sanitised and shown in the panel; delivery reports, ARF complaints
  and `POST /ingress/ses` (SNS signatures).
- **Members**: the panel (cookie sessions with Origin checks, sign-in codes, passkeys), API keys and
  OAuth tokens with scopes, the MCP endpoint `/mcp`, and the OAuth server (PKCE, dynamic client
  registration). Roles: owner, admin, agent; agents see only their inboxes.
- **Public pages with sealed tokens**: `/s/{token}` (confirm a subscription), `/m/{token}` (manage and
  RFC 8058 one-click unsubscribe), `/c/{token}` (click redirect), `/i/{token}` (e-mail images),
  `/r/{token}` (ratings).
- **Files**: attachments, design images (raster only, metadata stripped), CSV imports, migration
  export files uploaded in parts (mbox, Google Takeout, `.eml` and Maildir zips).
- **Connections to hosts a workspace chooses**: webhooks, SMTP accounts, IMAP servers and help-desk
  APIs for migrations, OAuth providers (Microsoft, Zendesk, Help Scout). Hosted servers refuse private
  and link-local addresses unless an operator allows them.

## Security properties that matter most

- **Workspace isolation.** Every table has `workspace_id` and every query filters by it; a request
  authenticated for one workspace must never read or change another's data. Agents must not reach
  inboxes they are not in; keys and tokens must not exceed their scopes or workspace.
- **Authentication.** Sign-in codes, passkeys, sessions, OAuth (members), API keys and contact
  identity tokens must not be bypassed, confused with each other or replayed.
- **Stored XSS.** E-mail HTML, chat and discussion text and designer output are shown in the panel and
  in the widget; script must never run in either.
- **SSRF.** Webhooks, SMTP and IMAP hosts, help-desk attachment fetches and OAuth endpoints must not
  reach internal networks on hosted servers.
- **Read-only migrations.** Moving to Yuva must never change or delete anything at the source:
  `api/internal/migrate/imapread` is the only importer of the IMAP client and a wire guard allows only
  read commands; help-desk transports send only `GET` (plus the token request). Any way to make a
  migration write to the source is in scope.
- **Secrets.** SMTP, IMAP and OAuth credentials and DKIM keys are sealed under the master key; they
  must never appear in answers, logs, events, webhooks or MCP results.
- **Consent and privacy.** Unsubscribes and opt-outs must hold; analytics must not store IP addresses
  (they are shortened before any use) and must drop events when DNT or GPC is set; customer text in
  MCP results is marked as untrusted content.

## Components that matter most / least

- Most: `api/internal/api` (handlers, authorization), `api/internal/email` (MIME parsing, sanitising,
  SMTP), `api/internal/mail` (rendering), `api/internal/migrate` (readers, guard, file readers),
  `api/internal/analytics`, `sdk/js`, `web/src`.
- Less: `edge/` (forwards mail with an HMAC), `templates/` (static e-mail designs).
- Out of scope: `site/` (static website), `examples/`, `deploy/` (packaging), the mobile SDKs.

## How to exercise it

- Start Postgres and point the tests at it, then run any test; the integration tests run the server
  in-process:

      export YUVA_TEST_DATABASE_URL="$(.oss-scanner/test-db.sh)"
      cd api && go test ./internal/api/ -run TestName

- Run the server: `yuva serve` with `YUVA_DATABASE_URL` (the same database works),
  `YUVA_PUBLIC_URL=http://localhost:8080`, `YUVA_MASTER_KEY` (32 bytes, base64) and
  `YUVA_STORAGE=local`; `yuva bootstrap --email owner@example.com --workspace demo` creates an owner;
  sign-in codes go to the log when no SMTP server is set. `deploy/compose.yaml` shows a working
  development configuration; `docs/configuration.md` lists every setting.
- IMAP migration integration tests need the Dovecot fixture from `deploy/compose.yaml`
  (`migrate-test` profile), which this image does not include; the wire-guard tests in
  `api/internal/migrate/imapread` run without it.

## How we rate severity

- Critical: unauthenticated access to another workspace's data, remote code execution, reading the
  master key or sealed credentials.
- High: cross-workspace reads or writes by a member of another workspace; privilege escalation (agent
  to admin or owner, a key or token beyond its scopes); stored XSS in the panel; SSRF to internal
  networks; any write or delete at a migration source; forging a contact's identity.
- Medium: XSS limited to a visitor's own widget; open redirects; consent or unsubscribe bypass;
  leaks that need a valid session; practical denial of service by one client.
- Low: missing limits on authenticated endpoints, verbose errors.

## Anything to leave alone

- The development defaults in `deploy/compose.yaml` (passwords, the ingress secret) are for local use.
- The operator CLI (`yuva workspace …`, `yuva api-key …`) runs on the server and is trusted.
- Public channel and analytics keys are public. The analytics collect endpoint answers `202` to events
  it drops (DNT, GPC, bots) and takes the visitor's address from a site proxy's `X-Forwarded-For` by
  design; a forged one only skews that property's own counts.

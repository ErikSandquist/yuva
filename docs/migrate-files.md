# Move from an export file

Bring the history of a mailbox into Yuva from a file you exported, without giving Yuva any access to
the account. This suits a very large mailbox, a program that has no IMAP, or an account Yuva cannot sign
in to. The overview is in [Move to Yuva](migrate.md).

Open **Settings → Move to Yuva → New migration**, enter the address the mail was sent to, open
**Or upload an export file**, choose **Google Takeout** or **Export file** and pick the file. Then choose what to bring, scan and copy as
in any other migration. The file is checked and read on the server; Yuva never changes it.

## Files Yuva reads

| File | Notes |
|---|---|
| Google Takeout `.mbox` | Unzip the Takeout archive first and upload the `.mbox` file from its Mail folder. Gmail's labels and threads come with it. |
| An `.mbox` file | `mboxrd` (what most programs write) or, under "mbox options", the older `mboxo`. |
| A `.zip` of `.eml` files | Its folders become labels. |
| A `.zip` of a Maildir | Its folders become labels. |
| A `.zip` of `.mbox` files | Each file becomes a label. The `.mbox` files must be stored uncompressed in the zip. |

Yuva does not read `.tgz` or `.gz` files, compressed `.mbox` files inside a zip, or Outlook's `.pst`
files. Unzip an archive and upload the `.mbox` inside it.

## Limits

| Limit | Value |
|---|---|
| Size of one upload part | 8 MiB |
| Size of one file | 20 GiB, set by the server's [`YUVA_MIGRATION_UPLOAD_MAX_BYTES`](configuration.md#moving-to-yuva) |
| Entries in a zip | 1,000,000 |
| Compression of a zip entry | at most 100:1 |

Messages larger than [`YUVA_MIGRATION_MAX_MESSAGE_BYTES`](configuration.md#moving-to-yuva) are skipped,
as in every migration. One file per migration; for another file, start a new migration.

## Upload and resume

The browser cuts the file into 8 MiB parts, sends several at once and retries a part that fails. Keep the
page open. If the connection drops, the upload goes on when it is back. If the page was closed or reloaded,
open the migration again and **pick the same file again**: Yuva compares what it already has and sends only
the missing parts. A different file is not mixed in; the page asks before it sends that one from the start.
When all parts have arrived, Yuva reads the file and tells you its format.

## When the file is refused

Yuva checks a zip before it reads anything from it. A refused file's uploaded parts are deleted; fix the
file and upload it again.

| Reason | What it means |
|---|---|
| `unknown_format` | It is neither an mbox nor a zip. |
| `empty` | There is no message in it. |
| `corrupt` | The zip cannot be read; it may be damaged or cut off. Make it again. |
| `path_traversal` | An entry has `..` or an absolute path in its name. |
| `symlink` | An entry is a symbolic link. Make the zip again without links. |
| `too_many_entries` | The zip has more than 1,000,000 entries. |
| `zip_bomb` | An entry is compressed more than 100:1. |
| `entry_too_large` | An entry is larger than the server's limit for one file. |
| `encrypted` | An entry has a password. |
| `unsupported_compression` | An entry is neither stored nor deflated. |
| `compressed_mbox` | An `.mbox` inside the zip is compressed. Unzip the archive and upload the `.mbox` itself. |

A file larger than the server's limit is refused before it is sent. If the server's limit is `0`, file
imports are off.

## The file is deleted

The uploaded parts, and so the file, are deleted when the migration completes, is cancelled or is
deleted. Yuva keeps only what it imported. The migration page says when the file was deleted.

## Get a file out of your mail

Each page below is the program's own help.

- **Google Takeout.** Open [Google Takeout](https://takeout.google.com), select only Mail, choose a
  one-time export as **Zip files**, and create it. Google emails you a link when the archive is ready,
  which can take from minutes to days; the link works for about 7 days and 5 downloads. Unzip the archive
  and upload the `.mbox` file from the Mail folder. Google's page:
  [Export your Google data](https://support.google.com/accounts/answer/3024190).
- **Apple Mail.** Select the mailboxes and choose **Mailbox > Export Mailbox**. Mail writes each one as an
  `.mbox` file. Apple's page:
  [Import or export mailboxes in Mail](https://support.apple.com/guide/mail/import-or-export-mailboxes-mlhlp1030/mac).
- **Thunderbird.** Thunderbird keeps each mail folder as an mbox file in your profile. Mozilla's page:
  [Profiles - Where Thunderbird stores user data](https://support.mozilla.org/en-US/kb/profiles-where-thunderbird-stores-user-data).
  Upload a folder's file.
- **Outlook.** Outlook exports to `.pst`, which Yuva does not read. Convert the `.pst` to `.mbox` or `.eml`
  files with a conversion tool of your choice, put the `.eml` files in a `.zip`, and upload that. Microsoft's
  page:
  [Export or back up email to an Outlook .pst file](https://support.microsoft.com/en-us/office/export-or-backup-email-contacts-and-calendar-to-an-outlook-pst-file-14252b52-3075-4e9b-be4e-ff9ef1068f91).
- **Proton Mail.** Proton's export tool writes `.eml` files. Put them in a `.zip` and upload it. Proton's
  page: [Export emails](https://proton.me/support/export-emails-import-export-app).

## Very large Gmail mailboxes

Start with a Takeout file, then connect the mailbox over IMAP for the newer mail and the switch: see
[Gmail](migrate-gmail.md#very-large-mailbox) and [Google Workspace](migrate-google-workspace.md#very-large-mailbox).

## From your backend

Owners, admins and API keys without an inbox limit that hold `migrations:manage`. Start a migration with a
`file` or `takeout` preset, then:

| Endpoint | Use |
|---|---|
| `GET /v1/migrations/{migrationId}/upload` | The parts the server has, to resume a cut upload. |
| `POST /v1/migrations/{migrationId}/upload/parts/{part}` | Send part `part` (from 0): a `file` of at most 8 MiB and its `sha256`, as `multipart/form-data`. Parts may arrive in any order and in parallel. |
| `POST /v1/migrations/{migrationId}/upload/complete` | Check the parts and read the file; the migration becomes `connected`. |

Errors worth handling: `413 part_too_large`, `422 checksum_mismatch` (the part is not stored),
`422 hosted_limit` (the file is over the server's limit), `422 upload_incomplete` (`missing_parts` names the
first missing parts), `422 file_refused` (`reason`, and the zip `entry` when one is to blame) and
`409 migration_state`. The rest of the flow is in [Move to Yuva](migrate.md#from-your-backend); full shapes are in the
[API contract](../openapi/openapi.yaml).

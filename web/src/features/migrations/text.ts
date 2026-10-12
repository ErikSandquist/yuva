import { useLingui } from "@lingui/react/macro"

import type {
  MigrationFolderKind,
  MigrationPresetId,
  MigrationState,
  MigrationTestCode,
  MigrationTestStep,
} from "@/lib/api"

export const DOCS = "https://github.com/productdevbook/yuva/blob/main/docs"

export const TEST_STEPS: MigrationTestStep["step"][] = ["resolve", "connect", "tls", "greeting", "sign_in", "folders"]

export const NEVER_READ: MigrationFolderKind[] = ["spam", "trash", "drafts", "other"]

export type Tone = "muted" | "success" | "warning" | "danger"

export const stateTones: Record<MigrationState, Tone> = {
  draft: "muted",
  connected: "muted",
  scanning: "warning",
  ready: "warning",
  copying: "warning",
  paused: "muted",
  needs_attention: "danger",
  syncing: "warning",
  completed: "success",
  cancelled: "muted",
  removing: "warning",
  removed: "muted",
}

export function guideUrl(slug: string) {
  return `${DOCS}/${slug}.md`
}

export function usePresetName() {
  const { t } = useLingui()
  const names: Partial<Record<MigrationPresetId, string>> = {
    microsoft: t`Outlook.com and Microsoft 365`,
    imap: t`Other IMAP server`,
  }
  return (id: MigrationPresetId, name: string) => names[id] ?? name
}

export function useMigrationText() {
  const { t } = useLingui()
  return {
    state: {
      draft: t({ message: "Draft", context: "migration state" }),
      connected: t`Connected`,
      scanning: t`Scanning`,
      ready: t`Ready`,
      copying: t`Copying`,
      paused: t`Paused`,
      needs_attention: t`Needs attention`,
      syncing: t`Syncing`,
      completed: t`Completed`,
      cancelled: t`Cancelled`,
      removing: t`Removing history`,
      removed: t`History removed`,
    } satisfies Record<MigrationState, string>,
    step: {
      resolve: t`Find the server`,
      connect: t`Connect`,
      tls: t`Encryption`,
      greeting: t`Server answers`,
      sign_in: t`Sign in`,
      folders: t`Read the folders`,
    } satisfies Record<MigrationTestStep["step"], string>,
    folderKind: {
      inbox: t`Inbox`,
      sent: t`Sent mail`,
      archive: t({ message: "Archive", context: "mail folder kind" }),
      all: t`All mail`,
      user: t`Your folder`,
      label: t`Label`,
      spam: t`Spam, never imported`,
      trash: t`Trash, never imported`,
      drafts: t`Drafts, never imported`,
      other: t`Never imported`,
    } satisfies Record<MigrationFolderKind, string>,
  }
}

export function useTestCodeText() {
  const { t } = useLingui()
  return (code: MigrationTestCode, provider: string, host: string) =>
    ({
      dns_failed: t`${host} does not resolve. Check the server's name.`,
      address_not_allowed: t`${host} is on a private or local network, which this server does not connect to.`,
      connect_failed: t`Nothing answers on that port of ${host}. Check the port and that IMAP is on.`,
      tls_failed: t`The server's certificate is not valid for ${host}. If it is your own server with its own certificate, you can trust it below.`,
      not_imap: t`The server answered, but not as an IMAP server. Check the host and port.`,
      starttls_unavailable: t`The server offers no STARTTLS. Use TLS on port 993 instead.`,
      auth_failed: t`${provider} did not accept the user name or password. Check both, or create a new app password.`,
      app_password_required: t`${provider} wants an app password, not your account's password. Create one with the steps above.`,
      imap_disabled: t`IMAP is off for this mailbox. Turn it on in ${provider}'s settings, then test again.`,
      basic_auth_disabled: t`${provider} accepts no passwords over IMAP. This mailbox needs “Sign in with Microsoft”, which comes later.`,
      web_login_required: t`${provider} wants you to sign in once in the browser. Sign in to your mail on the web, then test again.`,
      too_many_connections: t`The mailbox has too many connections open. Close a mail app or wait a few minutes, then test again.`,
      throttled: t`${provider} limits this account for now. Wait a while, then test again.`,
      no_folders: t`The mailbox lists no folders.`,
      examine_failed: t`The inbox could not be opened read-only.`,
      timeout: t`The test ran out of its 20 seconds. The server may be slow; try again.`,
      unexpected_answer: t`The server gave an answer Yuva did not expect. The details below show it.`,
    })[code]
}

export type Guide = { steps: string[]; link: string; linkLabel: string }

export function useGuides() {
  const { t } = useLingui()
  const google = [
    t`Turn on 2-Step Verification for the Google account, if it is not on yet.`,
    t`Open App passwords in the Google account, name the password “Yuva” and create it.`,
    t`Copy the 16-character password and paste it below. Gmail's IMAP is always on.`,
  ]
  return {
    gmail: {
      steps: google,
      link: "https://myaccount.google.com/apppasswords",
      linkLabel: t`Open App passwords`,
    },
    google_workspace: {
      steps: [
        ...google,
        t`No App passwords page? Your admin must allow 2-Step Verification and IMAP in the Google Admin console.`,
      ],
      link: "https://myaccount.google.com/apppasswords",
      linkLabel: t`Open App passwords`,
    },
    microsoft: {
      steps: [t`Outlook.com and Microsoft 365 accept no app passwords. Their sign-in comes in a later version.`],
      link: guideUrl("migrate-microsoft"),
      linkLabel: t`Read the guide`,
    },
    yandex: {
      steps: [
        t`In Yandex Mail settings, under Mail clients, allow access over IMAP with app passwords.`,
        t`In Yandex ID, open Security › App passwords and create a password for Mail, named “Yuva”.`,
        t`Copy it and paste it below.`,
      ],
      link: "https://id.yandex.com/security/app-passwords",
      linkLabel: t`Open App passwords`,
    },
    icloud: {
      steps: [
        t`Sign in to your Apple Account and open Sign-In and Security › App-Specific Passwords.`,
        t`Generate a password named “Yuva” and paste it below.`,
        t`The user name is your iCloud address without @icloud.com.`,
      ],
      link: "https://account.apple.com",
      linkLabel: t`Open your Apple Account`,
    },
    zoho: {
      steps: [
        t`In Zoho Mail, open Settings › Mail accounts › IMAP access and turn IMAP on.`,
        t`With two-factor sign-in, create an app password in Zoho Accounts › Security › App passwords; otherwise your password works.`,
        t`Pick the server of your data centre under Server settings.`,
      ],
      link: "https://accounts.zoho.com",
      linkLabel: t`Open Zoho Accounts`,
    },
    yahoo: {
      steps: [
        t`Open Account security in your Yahoo account.`,
        t`Choose “Generate app password”, name it “Yuva” and create it.`,
        t`Copy it and paste it below.`,
      ],
      link: "https://login.yahoo.com/account/security",
      linkLabel: t`Open Account security`,
    },
    fastmail: {
      steps: [
        t`In Fastmail, open Settings › Privacy & Security › Manage app passwords.`,
        t`Create a new app password with access to mail over IMAP, named “Yuva”.`,
        t`Copy it and paste it below.`,
      ],
      link: "https://app.fastmail.com/settings/security",
      linkLabel: t`Open Privacy & Security`,
    },
    gmx: {
      steps: [
        t`In GMX, open E-mail settings › POP3 & IMAP and allow access over IMAP.`,
        t`Use your GMX password, or an app password if two-factor sign-in is on.`,
        t`Pick the server for your address under Server settings: gmx.net, gmx.com or mail.com.`,
      ],
      link: guideUrl("migrate-gmx"),
      linkLabel: t`Read the guide`,
    },
    imap: {
      steps: [
        t`Find the IMAP server of your mail in your provider's help or your mail app's account settings: a host, port 993 with TLS or 143 with STARTTLS.`,
        t`Use the mailbox's user name and password, or an app password if your provider offers them.`,
      ],
      link: guideUrl("migrate-imap"),
      linkLabel: t`Read the guide`,
    },
  } satisfies Record<MigrationPresetId, Guide>
}

export function useItemErrorText() {
  const { t } = useLingui()
  const texts: Record<string, string> = {
    too_large: t`Larger than this server's limit for one message`,
    unparseable: t`The message could not be read`,
    gone: t`No longer in the mailbox`,
    failed: t`Failed five times`,
    blocked: t`From a blocked contact`,
    auth_failed: t`The mailbox refused the sign-in`,
    source_unreachable: t`The mail server could not be reached`,
    throttled: t`The provider asked to slow down`,
    folder_changed: t`The folder was recreated and read again`,
    hosted_limit: t`This server's limit was reached`,
    source_write_refused: t`Yuva held back a command that would have changed the mailbox`,
  }
  return (code: string) => texts[code]
}

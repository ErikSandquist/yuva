# What's new

A What's new changelog tells your customers what changed: short, dated entries labelled New, Improved or
Fixed, on a public page, in an Atom feed, in the [web widget](widget.md) with an unread count, and, when
you choose, mailed to your subscribers. It belongs to an inbox, so a customer sees that the bug they
reported is fixed in the place where they would have asked about it.

Each inbox has at most one changelog. The public pages are plain HTML rendered by your Yuva server,
without required JavaScript. Nothing here sends mail by itself: mailing an entry makes a
[broadcast](broadcasts.md) draft that you review and send as usual.

## 1. Create a changelog

In the panel: What's new in the sidebar (shortcut `g w`), then Create changelog on the inbox. Owners and
admins create and change changelogs; see [who may publish](#drafts-and-publishing) for entries.

| Setting (API field) | Meaning |
|---|---|
| Name (`name`) | Up to 100 characters. |
| Description (`description`) | Up to 1,000 characters, shown under the name on the home page. |
| Address (`slug`) | 3 to 48 characters: `a-z`, `0-9` and inner hyphens. Unique on the server; some words are reserved. |
| Languages (`locales`), main language (`default_locale`) | Up to 20 [languages](widget.md#languages) entries are written and published in; the main one is one of them. |
| Analytics (`analytics_property_id`) | Stored with the changelog. |
| Agents may publish (`agents_may_publish`) | Off by default. See [Drafts and publishing](#drafts-and-publishing). |
| Published (`published`) | Off by default. An unpublished changelog and all its entries answer `404` to everyone but your team, and the widget shows no What's new. |

The logo and accent colour of the pages are the inbox's branding. Renaming the address keeps the old one
reserved for the changelog for 90 days, and the public addresses under it redirect to the new one.

Over the API: `POST /v1/changelogs` with `inbox_id`, `name` and `slug`, then
`PATCH /v1/changelogs/{changelogId}` with `{"published": true}`.
`GET /v1/changelogs/{changelogId}/preview` returns a page as visitors would see it, also while the
changelog is unpublished; the panel shows it with Preview (shortcut `v` in the editor), drafts included.
`DELETE` removes the changelog with its entries and translations; broadcasts made from its entries stay.

## 2. Write and translate entries

An entry has one to three labels, an optional picture and one *translation* per language. Open What's
new, pick New entry, choose the labels and write a title and a Markdown text in the main language.

| Field | Limit |
|---|---|
| Labels (`labels`) | One to three of `new`, `improved` and `fixed`. |
| Title (`title`) | 3 to 200 characters. |
| Text (`body`) | Markdown, up to 50,000 characters. Headings, lists, tables, code, links and images; raw HTML is removed. |
| Address (`slug`) | Part of a translation's web address; made from the title, editable. A wrong slug in a URL redirects to the right one. |
| Picture (`image_id`) | One of your workspace's [e-mail images](designs.md#4-images). |

Images in the text stay only when you uploaded them to your workspace: upload from the editor, or
`POST /v1/changelogs/{changelogId}/images` (same types, sizes and re-encoding as e-mail images). An image
no entry, draft or design uses is deleted a week after its upload.

To translate, pick a language in the editor: the main-language text stays beside the translation, and
translations that are missing or out of date are marked. Yuva does not translate for you. A published
entry always keeps its main-language translation.

A workspace has at most `YUVA_CHANGELOG_MAX_ENTRIES` entries ([Configuration](configuration.md#whats-new));
beyond that, creating one answers `422 hosted_limit`.

Over the API, under `/v1/changelogs/{changelogId}`: `POST /entries` writes an entry with its first
translation; `GET /entries` filters by `state` (`draft`, `scheduled`, `published` or `changed`), `label`
and `locale`; `PATCH /entries/{entryId}` changes labels and picture; `PUT` and `DELETE` on
`/entries/{entryId}/translations/{locale}` write and remove a translation.

## Drafts and publishing

A translation has a working draft and, once published, a frozen published copy. Saving changes only the
draft; the public page changes when you publish.

- **Saving.** Every save carries the `base_version` the editor loaded. If someone saved in between, the
  save is refused with `409 version_conflict` and the answer holds the translation as it is now, so nobody
  overwrites another person's work. Other open editors are told that the entry changed.
- **Publishing.** Publish freezes every translation that has a draft, renders it and puts the entry on
  the web (`POST .../entries/{entryId}/publish`). It needs a text in the main language
  (`409 no_default_translation`). The first publish sets the date shown; publishing again keeps it. A
  translation added or changed later is published alone with `locale` in the request.
  Unpublish (`POST .../unpublish`) takes the entry off the web and the widget and returns it to a draft
  that keeps its date.
- **Who may publish.** Agents with access to the inbox write entries and translations. Owners and admins
  also publish, schedule, unpublish, change the labels or picture of a published entry, delete published
  entries and change the changelog's settings. Turn on Agents may publish (`agents_may_publish`) to let
  agents with access to the inbox publish too. Agents without the right use Submit draft, and an owner or
  admin publishes.
- **Keys.** An API key publishes only with the scope `changelog:publish`, for example a release pipeline
  that posts its entry. Without it, a key writes drafts.
- **Assistants never publish.** OAuth and [MCP](mcp.md#whats-new-from-an-assistant) tokens write drafts
  but cannot create or change a changelog, publish, schedule, unpublish or delete a published entry
  (`403 member_session_required`), whatever the settings are.

## Scheduling

Schedule an entry to go public at a time you pick, up to 365 days ahead, in your time zone
(`POST .../entries/{entryId}/schedule` with `publish_at`). A changelog holds at most 50 scheduled entries
(`409 too_many_scheduled`). Scheduling again moves the time; Unschedule (`POST .../unschedule`) returns the
entry to a draft.

The server publishes the entry itself at that time, as the person or key that scheduled it would have.
A scheduled entry goes public as it is, so only those who may publish can change it, save its
translations or delete them (`403 entry_scheduled`).

If the right is gone when the time comes, nothing goes public: the entry goes back to a draft with
`schedule_failed`, which the panel shows.

| `schedule_failed.reason` | Meaning |
|---|---|
| `lost_right` | The member who scheduled it may no longer publish, or is gone. |
| `key_revoked` | The API key that scheduled it was revoked. |
| `key_scope_lost` | The key no longer holds `changelog:publish` or can no longer see the inbox. |
| `no_default_translation` | The main-language text was removed. |

Fix the cause and schedule again; the next schedule or publish clears the failure.

## 3. Public pages and the feed

A published changelog is at `<your Yuva URL>/changelog/{slug}`, which redirects to its main language.
There is no redirect by browser language.

| Address | Shows |
|---|---|
| `/changelog/{slug}/{locale}/` | Entries newest first, 20 a page; `?label=new`, `improved` or `fixed` filters and `?before=` pages. |
| `/changelog/{slug}/{locale}/entries/{number}-{slug}` | One entry. A wrong slug redirects to the right one. |
| `/changelog/{slug}/{locale}/feed.atom` | The newest 50 entries in that language as an Atom feed, with each entry's text and labels. |
| `/changelog/{slug}/sitemap.xml` | The published pages. |

A page lists only the entries that have a published translation in the language of its address. Draft,
scheduled and unpublished entries, unknown languages and unpublished changelogs answer a plain `404`.
Pages carry canonical and `hreflang` links, Open Graph tags with the entry's picture and structured data
for search engines. They are cached for a minute and limited to 120 requests a minute per address.

## 4. In the widget

When the inbox has a published changelog with an entry, [Home](widget.md) shows a What's new tile with the
number of entries the visitor has not opened, up to "9+". The app opens inside the panel: entries newest
first with their labels, date, picture and text, and a link to the public page. It uses the
visitor's language when the changelog has it, else the main language.

- **Unread.** The newest date a visitor opened is kept in their browser only, per channel; Yuva stores
  nothing about it and anonymous visitors get it too. Without a mark, entries older than 30 days count as
  read. Opening the app marks everything shown as read.
- **Launcher badge.** Turn on Announce updates (`announce_updates`) on the chat channel and the launcher
  shows the number of unread entries, only while the visitor has no unread messages. Messages always win
  the badge. Off by default; the Home tile shows the count either way.
- **Live.** A publish reaches open widgets at once.

In code: `openApp("whats-new")` opens the app, and `openApp("whats-new", { entry: "<id>" })` opens one entry.
To build your own screen, the client has `whatsNew({ locale, cursor, limit })` and `whatsNewEntry(id)`, and
React sites have `useWhatsNew({ locale })` in `useyuva/react`; see the
[SDK README](../sdk/js/README.md). The endpoints behind them are
`GET /client/v1/channels/{channel_key}/whats-new` and `.../whats-new/{entryId}`: no session, no contact is
created, 120 reads a minute per address, cacheable for a minute. They answer `404 whats_new_off` without a
published changelog.

## 5. Mail an entry to subscribers

On a published entry, owners and admins choose Mail this to subscribers: pick up to 10 [lists](lists.md)
of the inbox, the sending channel and the language. Yuva builds a draft broadcast from the `product-update`
[design](designs.md): the title, labels and date, the text, your workspace's images, the entry's picture
at the top and a button to the entry's public page. A table or code block that an e-mail design cannot
hold becomes a line linking to the entry. The panel opens the draft; the entry lists its broadcasts
and their states.

Nothing is scheduled or sent by this. Review, test sends, consent, unsubscribe links, approval, pace and
sending are those of every [broadcast](broadcasts.md). Over the API:
`POST /v1/changelogs/{changelogId}/entries/{entryId}/broadcast` with `locale`, `list_ids` and optionally
`channel_id`; it needs the scopes `broadcasts:write` and `lists:read`, and answers `409 not_published`
for an entry that is not published. Assistants never mail an entry.

## Events and webhooks

Publishing (also when a schedule fires, and when a published translation is replaced), unpublishing and
deleting a published entry store an event for the members who see the inbox and can be sent to
[webhooks](webhooks.md) as `changelog.entry.published`, `changelog.entry.unpublished` and
`changelog.entry.deleted`.

## Reference

### Scopes

API keys made without naming scopes hold `changelog:read` and `changelog:write`; `changelog:publish` must
be named. No scope is added to keys made before they existed. See [Headless](headless.md#scopes).

| Scope | Allows |
|---|---|
| `changelog:read` | Read changelogs, entries and drafts, and preview. |
| `changelog:write` | Write entries and translations, upload images. |
| `changelog:publish` | Create, change and delete changelogs; publish, schedule, unpublish and delete published entries. OAuth clients are never offered it. |

A key limited to inboxes sees the changelogs of those inboxes; any other answers `404`.

### Limits

| Item | Limit |
|---|---|
| Entries per workspace | `YUVA_CHANGELOG_MAX_ENTRIES` ([Configuration](configuration.md#whats-new)). |
| Scheduled entries per changelog | 50. |
| Languages per changelog | 20. |
| Lists per mailed entry | 10. |
| Public pages and feeds | 120 requests a minute per address. |
| Preview | 60 renders a minute per caller. |

### Turn it off

`YUVA_WHATS_NEW=off` removes the feature: its endpoints and public pages answer `404` and the widget
shows no What's new.

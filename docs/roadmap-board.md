# Roadmap board

A roadmap board shows your customers what you are building and lets them say what they need: ideas in
stages, votes, and suggestions from readers. It belongs to an inbox, so the feedback your customers
already send can be linked to an idea instead of being answered again. When the idea moves, every linked
customer sees where it stands, and when it ships, one reply reaches all of them on their own channel.

A board can stay private, as a tool to sort feedback for your team, or be public: a plain HTML page on
your Yuva server without required JavaScript, and a Roadmap app in the [web widget](widget.md). Each
inbox has at most one board.

## 1. Create a board

In the panel: Roadmap in the sidebar (shortcut `g p`), then Create board on the inbox. Owners and
admins create and change boards; agents with access to the inbox work with ideas.

| Setting (API field) | Meaning |
|---|---|
| Name (`name`) | Up to 100 characters. |
| Description (`description`) | Up to 1,000 characters, shown under the name. |
| Address (`slug`) | 3 to 48 characters: `a-z`, `0-9` and inner hyphens. Unique on the server; some words are reserved. |
| Chat channel (`channel_id`) | A [chat channel](widget.md) of the same inbox. The public page embeds its chat, and ideas your team starts are filed on it. Required, and it cannot be deleted while the board exists. |
| Public (`public`) | Off by default. A private board answers `404` to everyone but your team. |
| Suggestions (`suggestions`) | `moderated` (default), `open` or `off`; see [Suggestions](#suggestions-and-votes). |
| Analytics (`analytics_property_id`) | Stored with the board. |

The page's logo and accent colour are the inbox's branding. Renaming the address keeps the old one: it
stays reserved for the board for 90 days and the public address under it redirects to the new one.
Turning off Public takes the board off the web at once and the widget loses its Roadmap tile.

Over the API: `POST /v1/roadmap-boards` with `inbox_id`, `channel_id`, `name` and `slug`, then
`PATCH /v1/roadmap-boards/{boardId}` with `{"public": true}`. `GET /v1/roadmap-boards/{boardId}/preview`
returns a page as visitors would see it, also while the board is private. `DELETE` removes the board
with its ideas, votes, replies and stage history; the feedback and the progress already set on it stay.

## 2. Ideas and stages

An idea is a thread with a title (3 to 200 characters) and a Markdown body (up to 20,000), and it sits in
one of five stages:

| Stage | Meaning | Progress set on linked feedback |
|---|---|---|
| `ideas` | Collected, not decided. New ideas start here. | none |
| `planned` | You will build it. | `planned` |
| `in_progress` | You are building it. | `in_progress` |
| `shipped` | Done. Takes no new votes. | `done` |
| `closed` | You will not build it. Takes no new votes and is left out of public lists. | `wont_do` |

Your team starts ideas in the panel, or from a piece of feedback (see [Linking feedback](#3-link-feedback)).
Agents with access to the inbox, owners and admins move the stage: on the board, drag a card or use its
stage menu (`m`, or `[` and `]` for the previous and next stage). The panel first says how many linked
feedback get which progress. You can add a comment; it is posted as your public reply on the idea.
Every move is kept in the idea's stage history, which also gives the public page its "Planned on" and
"Shipped on" dates. A move to the stage the idea is in changes nothing.

Over the API: `POST /v1/discussions/{discussionId}/stage` with `stage` and an optional `comment` (a
comment needs a member session), and `GET /v1/discussions/{discussionId}/stage-changes`. Ideas are
discussions of kind `idea`: list them with `GET /v1/discussions?kind=idea`, filtered by `board_id`,
`stage`, `sort` (`votes`, `new`, `activity`) and `q`; start one with `POST /v1/discussions` and
`kind: idea`.

## 3. Link feedback

Open a feedback conversation and choose Link to an idea (`b`): search the inbox's board, or pick New idea
from this feedback to start an idea with the title and text you write and link the conversation to it.
Nothing of the feedback becomes public. The feedback shows the idea as a chip.

- Only `feedback` conversations link, and only to an idea of a board of the same inbox.
- An idea has many linked feedback; a feedback has at most one idea. Linking again replaces the link.
  An idea takes at most 5,000 links.
- Moving a conversation to another inbox unlinks it from its idea.

Each card on the board shows how much feedback sits behind its idea. Over the API:
`PUT` and `DELETE /v1/discussions/{discussionId}/conversations/{conversationId}`;
`GET /v1/conversations?idea_id=` lists an idea's feedback and `idea=none` lists feedback without one.

### Progress follows the stage

When an idea moves to `planned`, `in_progress`, `shipped` or `closed`, Yuva sets the progress of every
linked feedback as in the table above. Each conversation gets a timeline event ("via idea …"), the
contact sees the new progress line, and [webhooks](webhooks.md) send `conversation.updated`. Feedback
that already has that progress is skipped. A newer move always wins over an older one still running. A
move back to `ideas` changes no progress. Linking feedback to an idea that is already past `ideas` sets
its progress at once; unlinking leaves it as it is. You can still set a feedback's progress by hand; it
holds until the idea's next move.

### Reply to linked

When an idea ships, the panel offers Reply to linked right away. It is the same function as in
[status pages](status-pages.md#7-link-conversations-to-an-incident): one reply, written once, goes to
every open linked conversation as you, on each conversation's own channel (e-mail, chat or app), and can
close them. It cannot be recalled once sent, and the panel reports per conversation whether it was sent,
skipped or failed. Over the API: `POST /v1/discussions/{discussionId}/linked-replies` answers `202`, and
`GET /v1/discussions/{discussionId}/linked-replies/{replyId}` reports the outcome. A key needs
`roadmap:write`, `conversations:read` and `messages:write` (and `conversations:write` to close), and the
workspace must let bots send ([Bots may send](headless.md#bots-may-send)).

## Suggestions and votes

Readers vote on ideas and suggest new ones. This needs a sign-in: **Sign in** opens a small window on
your Yuva server where the reader signs in with an e-mail code or a passkey, the same as for
[documentation pages](documentation-pages.md#what-readers-and-your-team-see). If your product has
accounts, give the widget an [identity token](identity.md) and its users vote and suggest at once.
Anonymous visitors only read. Voting also follows the idea: followers get an e-mail digest when it moves
to `planned`, `in_progress`, `shipped` or `closed`, or gets a reply.

| Mode | A reader's suggestion |
|---|---|
| `moderated` (default) | Waits for approval until one of their posts in the workspace was approved. They see "Waiting for approval". Identity-token users and your team publish at once. |
| `open` | Published at once, except authors you marked as spam. |
| `off` | No suggestions. Votes and replies still work. |

Pending ideas appear in the Roadmap screen's strip; owners and admins approve or hide them
(`PATCH /v1/discussion-posts/{postId}` with `state`). `GET /v1/discussion-posts?state=pending` is the
queue. A board holds at most 200 pending ideas (`429 suggestions_full`). Ideas on a private board send
no e-mail to their authors.

## Public pages

A public board is at `/roadmap/{slug}` on your server:

| Address | Page |
|---|---|
| `/roadmap/{slug}` | Columns Ideas, Planned, In progress and Shipped (the last 90 days; all of them at `?stage=shipped`), by votes. Each card has title, excerpt, votes and comments. |
| `/roadmap/{slug}/ideas/{id}` | An idea with its stage history and published replies. Authors show as name and initials only. |

The page reads complete without JavaScript. With it, Vote and Suggest an idea open the widget's Roadmap
app at that idea. A private board, an unknown address, and a pending or hidden idea answer a plain `404`.
Pages set a canonical link, Open Graph tags and structured data, are cached for a minute (a vote count may
lag that long) and answer 120 requests a minute per address. Page words exist in English, Turkish and
German; the page uses the inbox's language and `?lang=` picks another.

## In the web widget

When the inbox has a public board, the [widget](widget.md#what-visitors-see)'s home screen gets a Roadmap
tile. The app shows tabs by stage, ideas by top votes, an idea with its text, votes and replies, and
Suggest an idea, which lists similar ideas while the title is typed. Your page can open it:
`document.querySelector("yuva-chat").openApp("roadmap", { idea: "<idea id>" })`.

For your own interface, the [shadcn components](https://useyuva.com/shadcn) `use-yuva-roadmap`,
`yuva-roadmap`, `yuva-idea` and `yuva-idea-suggest` use `GET /client/v1/channels/{channel_key}/roadmap`
and the discussion endpoints with `kind=idea` of the [headless client](headless.md).

## From an assistant

With [MCP](mcp.md#roadmap-from-an-assistant) an assistant lists ideas, links feedback to them and, only
when you name the scope `roadmap:stage`, moves a stage after you confirm how many feedback change.

## API, webhooks and scopes

All of it is under `/v1/roadmap-boards` and the discussion endpoints, with the tag `roadmap` in the
[API reference](headless.md). [Webhooks](webhooks.md) send `discussion.stage_changed` with `discussion`,
`from`, `to` and `linked`, next to `discussion.created`, `discussion.post_created` and
`discussion.updated`. A key limited to inboxes sees only the boards of those inboxes; any other board or
idea answers `404`.

| Scope | Allows |
|---|---|
| `roadmap:read` | Boards, stage history, linked replies' outcomes. Given to keys made without named scopes. |
| `roadmap:write` | Create, change and delete boards, link and unlink feedback, Reply to linked. Given to keys made without named scopes. |
| `roadmap:stage` | Move an idea's stage. Never implied: name it. |

Reading and linking ideas also needs `conversations:read`. Existing keys gain none of these scopes.
`YUVA_ROADMAP=off` removes boards altogether ([Configuration](configuration.md#roadmap-board)).

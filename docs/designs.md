# E-mail designs and templates

A design is the look of a mail: blocks such as headings, text, images and buttons, and a theme with
colours and fonts. You build it once in the panel, start from one of the templates that ship with
Yuva, and use it for your broadcasts. The server turns it into mail HTML that works in the common
mail apps, with a plain-text part and a dark mode.

Owners and admins make designs. API keys and OAuth tokens need the `broadcasts:read` or
`broadcasts:write` scope. Assistants (agents) cannot use designs.

## 1. Start from a template

In the panel: Broadcasts, then the Designs tab. The gallery offers six starters: Newsletter, Product
update, Announcement, Welcome, Event and Digest, each in English, Turkish and German. Pick one, pick
the template language, and choose Use. Its images are copied into your workspace's images, and you can
change everything afterwards. New blank design starts from an empty mail with simple colours.

A design belongs to an inbox or to the whole workspace. The inbox's [sender identity](lists.md#1-set-the-sender-identity)
fills the footer.

Over the API: `POST /v1/email-designs` with `name` and `template: { "slug": "newsletter", "locale": "tr" }`.
`GET /v1/email-templates` lists the templates.

## 2. Edit the design

The designer opens full screen. Drag blocks from the palette (or the Layers outline), or press
Enter on one to add it after the selected block. Click a block to change its settings on the right.

| Block | Use |
|---|---|
| Section | A group of blocks on its own background. Sections do not nest. |
| Columns | Two or three columns that stack on phones. Column widths add up to 100. Columns do not nest. |
| Heading | Three sizes. |
| Text | Paragraphs, lists and links. |
| Image | An uploaded picture, with an optional link and alt text. |
| Button | A link that looks like a button. |
| Divider, Spacer | A thin line, empty room. |
| Social | Links to your profiles, as icons or text. |
| Footer text | Your own lines above the required footer. |

Headings, text, footer text and button labels are edited in place. Text keeps only what mail apps
handle well: bold, italic, underline, strikethrough, links, lists, line breaks and code. Links
start with `https://`, `http://`, `mailto:` or `tel:`.

Every mail ends with a footer you cannot remove: the inbox's legal name and postal address, why the
person gets the mail, and the unsubscribe and preference links. A warning says when the inbox has no
legal name or postal address yet.

A design holds up to 200 blocks.

### Merge fields

Merge fields put each person's own details into the mail. Insert one from the Insert menu in a text,
or type it. Add a fallback after `|` for people whose value is empty:

```
Hi {{ contact.first_name | "there" }},
```

| Field | Value |
|---|---|
| `contact.name` | The contact's name. |
| `contact.first_name` | The first word of the name. |
| `contact.email` | The e-mail address. |
| `contact.attributes.<key>` | An attribute of the contact, by its key. |
| `inbox.name` | The inbox's name. |
| `list.name` | The list the mail goes to. |
| `unsubscribe_url`, `preferences_url` | The person's own links. In links, merge fields may appear only after the `?`. |

A field Yuva does not know is refused. A fallback is at most 100 characters. Values never turn into
markup. Without a fallback the field can leave a gap such as "Hi ,", so the designer warns you.

## 3. Theme and dark mode

Open Theme (shortcut `T`) for colours, fonts, content width (480 to 720 pixels), corner radius and
direction (left to right, right to left, or automatic per text). Fonts are system stacks: system,
humanist, geometric, serif and monospace; mails load no web fonts.

Readers whose mail app is in dark mode get darker colours made from yours, with text kept at
4.5:1 contrast. Under Dark mode you can set any of those colours yourself. An image of a dark logo on
a transparent background can get a light plate behind it in dark mode.

## 4. Images

Open the image library from an image block (shortcut `I`), or drop files on it.

| | |
|---|---|
| Types | PNG, JPEG, GIF, WebP. SVG is refused because it can carry scripts. |
| Upload size | At most 10 MiB; 40 megapixels and 8,000 pixels on a side. |
| Stored as | Decoded and encoded again without EXIF, GPS or colour-profile data. Wider than 1,200 pixels it is scaled down to 1,200. WebP is stored as PNG, or JPEG when it has no transparency. |
| GIF | Kept as it is, animation included: at most 1,200 pixels wide and 5 MiB. |
| Same file twice | Answers with the image you already have. |
| Address | Every image has a public `/i/{token}` URL on your server: no cookies, cached for a year, nothing recorded per request. |

Give every image a description for screen readers, or mark it decorative. Together the images of a
workspace may take `YUVA_EMAIL_IMAGES_MAX_BYTES` ([Configuration](configuration.md)).

An image used by a design that is not archived cannot be deleted; the panel names the designs. An
image that went out in a broadcast asks for confirmation, because mail already delivered then shows
a broken image. Images that no design uses and no broadcast sent are deleted 7 days after upload.

## 5. Previews and checks

The preview comes from the server, so it is what recipients get. Switch between Edit and Desktop,
Phone and Dark with `1` to `4`. Checks shows what to fix:

| Warning | Meaning |
|---|---|
| Hard to read | Text under 4.5:1 contrast against its background, in light or dark colours. |
| Over 90 KB | Gmail cuts a mail off at about 102 KB of HTML and hides what follows, the unsubscribe link included. The designer warns at 90 KB and again over 102 KB. |
| Image missing, no description | An image block without an image, or without alt text. |
| No fallback | A merge field that can be empty for some people. |
| Sender details missing | The inbox has no legal name or postal address. |

A design over 500 KB of HTML is refused.

## 6. Saving, versions and conflicts

The designer saves 1.5 seconds after your last change; `Ctrl`/`Cmd` + `S` saves at once and keeps a
version. Undo and redo (`Ctrl`/`Cmd` + `Z`, with `Shift` for redo) go back 100 steps. Changes stay on
your device while you are offline and are saved when you are back.

Versions (shortcut `V`) keeps the newest 50: one at most every 10 minutes while you edit, one for each
manual save and one for each restore. Restoring saves that version as the newest one.

If someone else (or another tab of yours) saved first, a dialog offers three ways out: keep mine,
save mine as a copy, or load theirs. Your edit is never dropped, and the other version stays in the
versions. Whatever you pick, the other side's version is kept there as "Kept from a conflict", so no
edit is lost.

Download JSON saves the design's document as a file. Archive hides a design from the list; saving it
again brings it back. A document made by a newer Yuva can be viewed but not changed.

Shortcuts: `A` add a block, `T` theme, `I` image, `V` versions, `Ctrl`/`Cmd` + `D` duplicate the
block, `Backspace` delete it, `Alt` + `↑`/`↓` move it, `Esc` leaves the text and then the block.

## 7. The API

Everything the panel does is in the [API contract](../openapi/openapi.yaml). This renders a design
without saving it:

```sh
curl -X POST https://support.example.com/v1/email-designs/render \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "inbox_id": "INBOX-ID",
    "document": {
      "schema": 1,
      "theme": {
        "width": 600,
        "font": "system",
        "colors": {
          "page": "#f4f4f5", "card": "#ffffff", "text": "#18181b", "muted": "#52525b",
          "accent": "#1d4ed8", "link": "#1d4ed8", "border": "#e4e4e7"
        }
      },
      "blocks": [
        { "id": "h", "type": "heading", "level": 1, "text": "Hello {{ contact.first_name | \"there\" }}" },
        { "id": "b", "type": "button", "label": "Read more", "url": "https://example.com/news" }
      ]
    }
  }'
```

The answer has `html`, `text`, `size_bytes` and `warnings`. Pass `"dark": true` for the dark palette.
A save sends the `version` it started from as `base_version`; if the design moved on, the answer is
`409 version_conflict` with the current design in `design`. An invalid document answers
`422 invalid_design` with one entry per problem in `errors`.

## Reference

### Endpoints

| Endpoint | Use |
|---|---|
| `GET`, `POST /v1/email-designs` | List (`inbox_id`, `q`, `archived=true`) and create from a `document`, a `template` or nothing. |
| `GET`, `PUT`, `DELETE /v1/email-designs/{designId}` | Read, save (with `base_version`), archive. |
| `POST /v1/email-designs/{designId}/duplicate` | Copy a design. |
| `POST /v1/email-designs/render` | Render a document: HTML, text, size and warnings. At most 120 a minute. |
| `GET /v1/email-designs/{designId}/versions` | The kept versions. |
| `GET /v1/email-designs/{designId}/versions/{version}` | One version with its document. |
| `POST /v1/email-designs/{designId}/versions/{version}/restore` | Save a version as the newest. |
| `GET`, `POST /v1/email-images` | List and upload images (multipart, one `file`). |
| `DELETE /v1/email-images/{imageId}` | Delete an image (`force=true` for one that was sent). |
| `GET /v1/email-templates`, `GET /v1/email-templates/{slug}` | The gallery (`locale=en`, `tr` or `de`). |
| `GET /i/{token}` | An image, public. |
| `GET /email-templates/{slug}/{locale}.html`, `.webp` | A template's preview, public. |

### Scopes

| Scope | Meaning |
|---|---|
| `broadcasts:read` | Read designs, versions, images and templates; render. |
| `broadcasts:write` | Create, save, copy, archive and restore designs; upload and delete images. |

A key limited to inboxes reads the designs of its inboxes and the workspace-wide ones, and changes
only designs of its inboxes. Keys made before these scopes existed do not get them.

### Limits

| Limit | Value |
|---|---|
| Blocks in a design | 200 |
| Design document | 300 KiB of JSON |
| Rendered HTML | 90 KB warning, 102 KB clipped, 500 KB refused |
| Versions kept | 50 |
| Image upload | 10 MiB, 40 megapixels, 8,000 pixels a side |
| Image width after upload | 1,200 pixels |
| Merge field fallback | 100 characters |
| Link | 2,048 characters |
| Renders | 120 a minute per caller |
| Unused images | Deleted 7 days after upload |

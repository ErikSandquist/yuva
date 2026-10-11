# E-mail templates

The starter designs in the template gallery of Yuva's e-mail designer. They are MIT licensed (see
`LICENSE` and the row in `LICENSING.md`): anyone may use, change and share them in their own mails. This
guide is for adding a template; how members use the gallery is in
[docs/designs.md](../../docs/designs.md).

## Layout

One folder per template, named by its slug (lowercase letters, digits and single hyphens):

```
newsletter/
  meta.json
  design.en.json
  design.tr.json
  design.de.json
  assets/hero.jpg
  preview.en.html   preview.en.webp
  preview.tr.html   preview.tr.webp
  preview.de.html   preview.de.webp
```

`scripts/` holds the preview script and is not a template. `api/internal/gallery/data/` is a copy of
this folder that the server embeds; do not edit it by hand.

## meta.json

```json
{
  "slug": "newsletter",
  "order": 1,
  "category": "newsletter",
  "name": { "en": "Newsletter", "tr": "Bülten", "de": "Newsletter" },
  "description": { "en": "…", "tr": "…", "de": "…" },
  "author": "Your name",
  "added": "2026-10-11"
}
```

`slug` must equal the folder's name. `order` sorts the gallery (ties by slug). `name` and
`description` need `en`, `tr` and `de`. `category` is a short lowercase word; the existing ones are
`newsletter`, `product`, `announcement`, `onboarding`, `event` and `digest`.

## The design per locale

`design.<locale>.json` is an `EmailDesignDocument` (`schema: 1`), the same format the API takes; see
`EmailDesignDocument` in [openapi/openapi.yaml](../../openapi/openapi.yaml). `design.en.json` is
required, and the gallery tests require `design.tr.json` and `design.de.json` too: translate the text,
keep the structure and the block ids.

Rules the tests enforce:

- Images are image blocks with `asset` (a file name in `assets/`, lowercase with hyphens, ending in
  `.png`, `.jpg` or `.gif`), never `image_id`. Every image has `alt` text or is `decorative`.
- Every `contact.*` merge field has a fallback, as in `{{ contact.first_name | "there" }}`.
- Links go to `https://example.com` (social profiles may use `https://mastodon.example`), never to a
  real site.
- Text on the card and the muted colour on the page keep 4.5:1 contrast; the renderer warns about the
  rest. A template renders without any warning.
- The Turkish and German text is not the English text.
- The rendered HTML is at most 90 KB.

## Assets

Photos are JPEG, logos and graphics PNG; GIF is allowed. Each file is at most 300 KB and 1,200 pixels
wide, and a template's assets add up to at most 1 MB. Use images you may license under MIT (your own,
or generated); no stock photos with other terms. Strip metadata. Applying a template copies its assets
into the member's workspace.

## Previews

Each locale has two generated files: `preview.<locale>.html`, the design rendered with sample data and
the sample sender, and `preview.<locale>.webp`, a screenshot of it. The gallery shows the screenshot and
serves both publicly. From the repository root:

```sh
cd api && go build -o /tmp/yuva ./cmd/yuva && cd ..
/tmp/yuva templates render                 # writes every preview.<locale>.html; prints sizes and warnings
(cd templates/email/scripts && bun install && bun run previews [slug...])   # writes the .webp files
(cd api && go generate ./internal/gallery)  # copies this folder into the server
```

`yuva templates render` takes `--dir` for another folder and needs no database. The screenshot script
uses Playwright's Chromium (set `CHROME_PATH` to use another browser). Fonts differ between machines, so
CI does not compare the screenshots, but it does compare the HTML.

## Checks

In `api/`:

```sh
go generate ./... && go vet ./... && go test ./... && go build ./...
```

`go test ./internal/gallery` runs the checks above for every template, and fails when a
`preview.<locale>.html` is not what the design renders to now ("run yuva templates render and go
generate"), when the gallery's slug list changes without the test changing, or when `LICENSE` is not
copied. Adding a template means adding its slug to the list in `TestGalleryLoads`.

## Opening a pull request

Read [CONTRIBUTING.md](../../CONTRIBUTING.md) first and sign the CLA in your first pull request.
Contribute one template per pull request, with its three designs, its assets, the generated previews
and the copy in `api/internal/gallery/data/`. Say in the description where the images come from. The
template is published under the MIT license of this folder.

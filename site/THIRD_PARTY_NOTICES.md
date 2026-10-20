# Third-party notices

## Technology marks on the home page

The strip under the home page hero names the frameworks, languages and protocols Yuva has guides
or SDK support for. Their marks come from the `simple-icons` package (pinned in `package.json`),
which is released under CC0-1.0 (https://github.com/simple-icons/simple-icons). Only the icons the
strip uses are imported (`src/lib/stack.ts`): Next.js, React, Vue.js, Nuxt, shadcn/ui, Astro,
Docker, MinIO, Firebase, Swift, Kotlin, Go, OpenAPI Initiative and Model Context Protocol. They are drawn unmodified in a
single colour. Each mark is a trademark of its owner; it names compatibility and implies no
endorsement.

Nitro's mark is not in `simple-icons`; it is the outer shape of `docs/.docs/public/icon.svg` from the
Nitro repository (https://github.com/nitrojs/nitro), MIT License, Copyright (c) Pooya Parsa and Nitro
contributors, drawn in a single colour like the others (`src/lib/marks.ts`). h3's mark is
`docs/.docs/public/icon.svg` from the h3 repository (https://github.com/h3js/h3), MIT License, drawn
the same way; the `simple-icons` "H3" is Uber's geospatial index and is not used.

## Cloudflare

Cloudflare's trademark guidelines (https://www.cloudflare.com/trademark/) allow the name in text
only and require written permission for the logo, so the site shows no Cloudflare logo. The name
is used in text with the attribution statement on the Cloudflare e-mail page.

## Names shown without a mark

Amazon SES and Amazon S3 (AWS marks are not in `simple-icons` and AWS requires a licence for its
logos), Postmark (no mark in `simple-icons`), and APNs (Apple does not allow third parties to use the
Apple logo) are shown as names only.

## shadcn/ui components in the registry previews

`src/shadcn/ui/*.tsx` and `src/shadcn/utils.ts` are the shadcn/ui components (style `base-nova`) that the
previews under `/shadcn` render, written by the shadcn CLI from the shadcn/ui registry
(https://github.com/shadcn-ui/ui), MIT License, Copyright (c) 2023 shadcn. `checkbox.tsx` has its icon
placeholder replaced by the `lucide-react` check icon. The licence text is in
`examples/shadcn-next/THIRD_PARTY_NOTICES.md`.

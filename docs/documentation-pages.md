# Documentation pages

Add "Was this page helpful?" and a public discussion area to your documentation site. Your pages
stay where they are; two web components send ratings and private feedback to the Yuva inbox and
show the page's discussions: threads with replies, votes and an accepted answer, like GitHub
Discussions.

## 1. A chat channel

Use a chat channel, set up as for the [web widget](widget.md#1-create-a-chat-channel); one channel
can serve both.

- Add the docs site's origin to the channel's **allowed origins** (`https://docs.example.com`,
  plus your dev server, such as `http://localhost:3000`). On any other origin the elements stay
  empty.
- Give the inbox an [e-mail channel](email.md): replies to feedback reach readers by e-mail.
- Choose how discussions work with the channel's `discussions` setting: `moderated` (the default:
  a reader's posts wait for your team until one of them was approved), `open` (published at once)
  or `off` (no discussions; the element says so).

## 2. Add the elements

### Plain HTML

```html
<script src="https://support.example.com/yuva-docs.js" defer></script>

<yuva-page-feedback channel="yuva_pk_xxxxxxxxxxxxxxxx"></yuva-page-feedback>
<yuva-discussions channel="yuva_pk_xxxxxxxxxxxxxxxx"></yuva-discussions>
```

Put them in the template every docs page shares, after the article. Your Yuva server hosts
`/yuva-docs.js`.

npm has them as `useyuva/docs`, `useyuva/react` and `useyuva/nuxt` (`useyuva` 0.0.7 and later).

### Nuxt Content

Runnable example: [`examples/nuxt-content`](../examples/nuxt-content)
([how to run it](../examples/README.md#nuxt-content)). Add the module:

```ts
// nuxt.config.ts
export default defineNuxtConfig({
  modules: ["@nuxt/content", "useyuva/nuxt"],
  yuva: { server: "https://support.example.com", channel: "yuva_pk_xxxxxxxxxxxxxxxx" },
});
```

```vue
<!-- app/pages/[...slug].vue, after <ContentRenderer> -->
<YuvaPageFeedback :page-title="page.title" />
<YuvaDiscussions :page-title="page.title" />
```

`NUXT_PUBLIC_YUVA_SERVER` and `NUXT_PUBLIC_YUVA_CHANNEL` override `yuva` at runtime. The components
render on the server and load the elements in the browser, so `nuxt generate` works as well. Events
are `@rating`, `@feedback`, `@discussion`, `@post` and `@reader`. On a fresh site, `nuxt build` fails with "Nuxt Content
requires better-sqlite3": install it, or set `content.experimental.sqliteConnector: "native"` as the
example does. The page queries a `content` collection; define it as in the example's
[`content.config.ts`](../examples/nuxt-content/content.config.ts).

### Fumadocs

Runnable example: [`examples/fumadocs`](../examples/fumadocs)
([how to run it](../examples/README.md#fumadocs)). The docs page is a server component, so wrap
the React components in a client component:

```tsx
// components/yuva.tsx
"use client";
import { YuvaDiscussions, YuvaPageFeedback } from "useyuva/react";

const channel = process.env.NEXT_PUBLIC_YUVA_CHANNEL!;
const server = process.env.NEXT_PUBLIC_YUVA_SERVER;

export const PageDiscussions = ({ title }: { title: string }) =>
  <YuvaDiscussions channel={channel} server={server} pageTitle={title} />;
export const PageFeedback = ({ title }: { title: string }) =>
  <YuvaPageFeedback channel={channel} server={server} pageTitle={title} />;
```

```tsx
// app/docs/[[...slug]]/page.tsx, after <DocsBody>
<PageFeedback title={page.data.title} />
<PageDiscussions title={page.data.title} />
```

If you added Fumadocs' own `<Feedback>` component, replace it with `<PageFeedback />`, or keep it
and add only `<PageDiscussions />`. With `useyuva` linked by path from `sdk/js`, Next can load a
second copy of React; set `turbopack.root` as in the example's `next.config.mjs`.

### shadcn/ui

For React sites built on [shadcn/ui](https://ui.shadcn.com) that want the discussions as their own
code, in their own design system. Runnable example: [`examples/shadcn-next`](../examples/shadcn-next)
([how to run it](../examples/README.md#shadcn-next)).

```sh
npx shadcn@latest add https://useyuva.com/r/yuva-discussions.json https://useyuva.com/r/yuva-page-feedback.json
```

This copies the components into your project (`components/yuva-*.tsx`,
`hooks/use-yuva-discussions.tsx`, `lib/yuva-utils.ts`), adds the shadcn/ui primitives they use
(button, card, input, tabs, textarea, avatar, badge, separator, skeleton) and installs `useyuva`
and `lucide-react`. The smaller items are `use-yuva-discussions` (the provider and hooks only),
`yuva-discussion-list`, `yuva-discussion-thread` and `yuva-discussion-composer`; all of them are
listed in `https://useyuva.com/r/registry.json`.

```tsx
// app/layout.tsx
import { YuvaDiscussionsProvider } from "@/hooks/use-yuva-discussions";

<YuvaDiscussionsProvider
  server={process.env.NEXT_PUBLIC_YUVA_SERVER}
  channel={process.env.NEXT_PUBLIC_YUVA_CHANNEL}
>
  {children}
</YuvaDiscussionsProvider>
```

```tsx
// under the article
<YuvaPageFeedback />
<YuvaDiscussions />
```

`YuvaDiscussions` takes `page` and `pageTitle` (the URL without query and fragment and
`document.title` by default), `defaultSort`, `labels` and `className`; without a provider around it,
give it `server` and `channel` itself. The provider also takes `identityToken` (a function, as for
the elements), `locale` for dates, and `client` to share a `createYuvaClient()` you made.

- **Customize** by editing the files: they are yours now. Colours come from your theme tokens
  (`primary`, `muted`, `border`, `destructive` …), so dark mode follows your theme. Posts show the
  server's `body_html` inside the classes of `yuvaProseClassName` in `lib/yuva-utils.ts`; the
  composer's preview is made of React elements, and what is published is what the server renders.
- **Your own layout**: the hooks behind the components are `useDiscussions(page, sort)`,
  `useDiscussion(id)`, `useYuvaReader()` (call `signIn()` from a click), `useDiscussionMode()` and
  `usePageRating(page)`.
- **Labels** are English by default (`defaultYuvaDiscussionsLabels`). Pass `labels` to the provider,
  or to one component, to change or translate them; counts are functions. Turkish:

```tsx
import type { YuvaDiscussionsLabels } from "@/lib/yuva-utils";

const tr: Partial<YuvaDiscussionsLabels> = {
  heading: "Tartışmalar",
  sortNew: "Yeni",
  sortTop: "En çok oy alan",
  sortUnanswered: "Yanıtsız",
  newDiscussion: "Yeni tartışma",
  empty: "Bu sayfada henüz tartışma yok. İlkini siz başlatın.",
  replies: (count) => `${count} yanıt`,
  votes: (count) => `${count} oy`,
  answered: "Yanıtlandı",
  acceptedAnswer: "Kabul edilen yanıt",
  reply: "Yanıtla",
  signIn: "Giriş yap",
  signInHint: "Tartışma başlatmak, yanıtlamak ve oy vermek için giriş yapın.",
  write: "Yaz",
  preview: "Önizleme",
  startDiscussion: "Tartışmayı başlat",
  feedbackQuestion: "Bu sayfa işinize yaradı mı?",
  feedbackYes: "Evet",
  feedbackNo: "Hayır",
};

<YuvaDiscussionsProvider server={server} channel={channel} labels={tr} locale="tr">
```

The components use logical spacing (`ps-`, `border-s`, `text-start`), so they work under
`dir="rtl"`. Like the elements, they need `useyuva` 0.0.7 or later.

### Other generators

Load the script (or `useyuva/docs`) and put both elements under the article. `useyuva/docs` and
`useyuva/react` can be imported during server rendering; they register the elements in the browser
only. Checked with Starlight 0.42, VitePress 1.6 and Docusaurus 3.10.

- **Starlight**: [override `Footer`](https://starlight.astro.build/guides/overriding-components/): render the default `Footer`, then both elements and
  `<script>import "useyuva/docs";</script>`. Astro needs no setting for custom elements.
- **VitePress**: [wrap `Layout`](https://vitepress.dev/guide/extending-default-theme#layout-slots) and put both elements in the `doc-after` slot; import
  `useyuva/docs` in `.vitepress/theme/index.js` and set `vue: { template: { compilerOptions: { isCustomElement: (tag) => tag.startsWith("yuva-") } } }` in the config.
- **Docusaurus**: [wrap](https://docusaurus.io/docs/swizzling#wrapping) `DocItem/Footer` (`npm run swizzle @docusaurus/theme-classic DocItem/Footer -- --wrap --danger`;
  the CLI marks it unsafe) and render `YuvaPageFeedback` and `YuvaDiscussions` from `useyuva/react` after it.
- **MkDocs, Hugo, Jekyll and others**: as [plain HTML](#plain-html).

## What readers and your team see

- **Readers** rate a page Yes or No (an anonymous counter), then may write feedback, which stays
  private and takes an optional e-mail address for the reply.
- **Everyone** reads the page's discussions without signing in: threads sorted by New, Top or
  Unanswered, each with its opening post, the accepted answer pinned first, replies (one level of
  replies to replies), votes and an "Answered" badge. Authors are shown by name and initials only;
  your team's posts carry a "Team" badge.
- **Taking part** needs a sign-in. **Sign in** opens a small window on your Yuva server, where the
  reader signs in with an e-mail code or a passkey; the account works on every site that uses
  this Yuva server, and the window hands the page a token for this channel that lasts 24 hours.
  Readers start threads, reply, vote, follow a thread (e-mail when someone replies or an answer is
  accepted), mark an answer on their own thread, edit their posts for 30 minutes and delete them
  at any time. Posts are Markdown, with a preview; the page shows the HTML the server made of it,
  without scripts or raw HTML. **Sign out** ends the token on this site only.
- **Moderation**: on a `moderated` channel a reader's first posts show "Awaiting approval" to them
  and are hidden from everyone else until someone of your team approves one; after that they
  publish at once. Marking a post as spam hides it and makes that author's posts wait again. A
  locked thread takes no new replies, votes or answers from readers.
- **Your team** gets feedback in the inbox as `feedback` conversations, and works on discussions in
  the panel: threads per site and page, the queue of posts waiting for approval, replies as the
  team, accepting answers, locking and retitling threads. A private conversation can become a
  public thread with the text your team writes; nothing else of the conversation is published.
  The **Docs** view lists pages with their ratings, feedback, open and unanswered threads and
  pending posts over 7, 30 or 90 days.

## Good to know

- **A page is its URL without query and fragment.** Per-language URLs (`/en/docs/install`,
  `/de/docs/install`) are separate pages with their own ratings; pass one URL as `page` to count
  them together, and set `locale` to the page's language.
- **Your own sign-in**: if your site has accounts, give the elements an
  [identity token](identity.md) with `setIdentityToken` (in React, the `identityToken` prop). The
  feedback form then has no e-mail field and replies go to the address in the token, and
  `<yuva-discussions>` writes as that user without the sign-in window. Without **Anonymous
  visitors** on the channel, feedback needs a token; ratings never do.
- **Sign-in window**: browsers open it only from a click, which is how the element opens it. Pages
  that send `Cross-Origin-Opener-Policy: same-origin` cut the window off from the page; use
  `same-origin-allow-popups` there.
- **Content Security Policy**: allow the Yuva server in `script-src` (for `/yuva-docs.js`) and
  `connect-src`.
- **Elements stay empty**: the origin is not allowed or the key is wrong; see the browser console.
- **Links to a thread**: e-mails to followers and the reader's account page link to
  `<page>#yuva-discussion-<id>`. The element opens that thread and scrolls to it, also on a
  `hashchange`; going back to the list removes the fragment from the address.
- **Questions from before**: published answers became discussions on their pages, answered by the
  member who published them.

## Reference

### Attributes

| Attribute | Meaning |
|---|---|
| `channel` | The chat channel's public key (`yuva_pk_…`). Required. |
| `server` | Yuva server URL. Defaults to the origin `yuva-docs.js` was loaded from. |
| `locale` | One of the 47 [languages](widget.md#languages), such as `en`, `tr`, `de`, `pt-BR` or `ar`. Defaults to the browser's language; a language Yuva does not have falls back to English. |
| `dir` | `ltr` or `rtl`. Defaults to the locale's direction. |
| `identity-token` | A signed identity token. `setIdentityToken` is usually better. |
| `page` | The page's URL. Defaults to the current URL; client-side navigation is followed. |
| `page-title` | The page's title. Defaults to `document.title`. |

The React wrappers take the same as props (`channel`, `server`, `locale`, `dir`, `page`,
`pageTitle`, `identityToken`) plus `className` and `style`.

### Properties and methods

| Element | Member | Meaning |
|---|---|---|
| both | `channel`, `server`, `locale`, `page`, `pageTitle` | The attributes as properties. |
| both | `setIdentityToken(fn)` | A function returning a token, or `null` for a reader not signed in. |
| `yuva-page-feedback` | `rating` | `"up"`, `"down"` or `null` for this page in this browser. |
| `yuva-page-feedback` | `rate("up" \| "down")` | The same as pressing Yes or No. |
| `yuva-discussions` | `discussions` | The threads listed. |
| `yuva-discussions` | `reader` | The signed-in reader (`{ token, expires_at, reader: { name, initials } }`) or `null`. |
| `yuva-discussions` | `reload()` | Fetch the list again. |
| `yuva-discussions` | `open(id)` | Show a thread. |
| `yuva-discussions` | `signIn()`, `signOut()` | The same as the buttons; call `signIn()` from a click. |

### Events

They bubble out of the Shadow DOM, so you can listen on `document`.

| Event | Element | `detail` | React prop |
|---|---|---|---|
| `yuva-rating` | `yuva-page-feedback` | `{ page, rating, previous }` | `onRating` |
| `yuva-feedback` | `yuva-page-feedback` | `{ page, rating, conversation_id }` | `onFeedback` |
| `yuva-discussion` | `yuva-discussions` | `{ page, discussion_id, state }` when the reader started a thread | `onDiscussion` |
| `yuva-post` | `yuva-discussions` | `{ page, discussion_id, post_id, state }` when the reader replied | `onPost` |
| `yuva-reader` | `yuva-discussions` | `{ name, initials }` after sign-in, `null` after sign-out | `onReader` |

### CSS variables

The elements take `color` and `font` from the page and follow dark mode. The accent comes from the
channel's launcher colour; to use your own, set it with `!important`.

| Variable | Used for |
|---|---|
| `--yuva-accent` | Buttons, the selected rating and sort, votes, focus rings, links in posts, team avatars. |
| `--yuva-on-accent` | Text on the accent. |
| `--yuva-border` | Borders of buttons, fields, threads and posts. |
| `--yuva-soft` | The selected rating's background, hovered threads, code in posts. |
| `--yuva-danger` | Error messages. |

### Endpoints

| Endpoint | Used for |
|---|---|
| `POST /client/v1/channels/{channel_key}/page-ratings` | Count a rating. |
| `POST /client/v1/feedback` | Send feedback with `page_url`, `page_title`, `rating`. |
| `GET /client/v1/channels/{channel_key}/discussions?page=&sort=` | A page's threads (cached for a minute without a token). |
| `GET /client/v1/channels/{channel_key}/discussions/{discussionId}` | A thread with its replies. |
| `POST /client/v1/discussions`, `POST /client/v1/discussions/{discussionId}/posts` | Start a thread, reply. |
| `PATCH`, `DELETE /client/v1/discussion-posts/{postId}`, `PUT`, `DELETE …/vote` | Edit, delete, vote. |
| `PUT`, `DELETE /client/v1/discussions/{discussionId}/answer`, `…/follow` | Accept an answer, follow. |
| `POST /v1/account/reader-tokens`, `DELETE /client/v1/reader-token` | The sign-in window's token, sign out. |
| `GET /v1/discussions`, `GET /v1/discussion-posts?state=pending` and the rest of `/v1/discussions*` | Your team's view and moderation. |
| `POST /v1/conversations/{conversationId}/discussion` | Turn a conversation into a public thread. |
| `GET /v1/docs/sites`, `GET /v1/docs/pages`, `GET /v1/docs/summary`, `GET /v1/docs/page` | The Docs view. |
| `GET /v1/conversations?page=&kind=feedback` | A page's feedback. |

Details: [API contract](../openapi/openapi.yaml).

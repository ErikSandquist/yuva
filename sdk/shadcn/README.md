# Yuva shadcn registry

React components for documentation pages and mailing list sign-up built on [shadcn/ui](https://ui.shadcn.com) primitives and
the headless client from [`useyuva`](../js): discussions like GitHub Discussions and "Was this page
helpful?", and a subscribe form. `npx shadcn add` copies them into your project, so they are your code in your design
system.

```sh
npx shadcn@latest add https://useyuva.com/r/yuva-discussions.json
```

| Item | Type | What |
|---|---|---|
| `yuva-discussions` | block | The list with New, Top and Unanswered tabs, threads, the accepted answer, replies, votes, follow, accept, a composer with preview, reader sign-in. |
| `yuva-discussion-list` | component | A page's threads with sort tabs. |
| `yuva-discussion-thread` | component | One thread with its posts and the reply composer. |
| `yuva-discussion-composer` | component | Markdown composer with Write and Preview tabs, and the sign-in prompt. |
| `yuva-page-feedback` | component | Yes and No page rating. |
| `yuva-subscribe` | component | Mailing list sign-up form with an unticked consent box, or a switch per list for signed-in users. |
| `use-yuva-subscribe` | hook | `useYuvaSubscribe`: public lists, form subscribing and the signed-in user's subscriptions. |
| `use-yuva-discussions` | hook | `YuvaDiscussionsProvider` and the hooks, plus labels and helpers in `lib/yuva-utils.ts`. |

Usage, labels and customizing: [Documentation pages › shadcn/ui](../../docs/documentation-pages.md#shadcnui).

## Develop

The sources are in `registry/yuva/` and listed in `registry.json`. Imports use the shadcn aliases
(`@/components/ui/…`, `@/hooks/…`, `@/lib/…`), which `shadcn add` rewrites to the project's own.

```sh
bun install
bun run check   # oxfmt, oxlint, shadcn build into dist/r
```

The website builds the same registry into `site/public/r` (`bun run registry` in `site/`) and serves
it at `/r/<item>.json`. Links between items point at `https://useyuva.com/r/`; the site rewrites
them to its own address. `examples/shadcn-next` is installed from that build and type-checks the
result with `next build`.

## License

MIT

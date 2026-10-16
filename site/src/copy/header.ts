import type { I18n } from "@lingui/core"
import { msg } from "@lingui/core/macro"
import type { IconName } from "@/lib/icons"
import {
  ai,
  blog,
  brand,
  cloudflareEmail,
  contact,
  docsRoot,
  home,
  links,
  releases,
  templates,
  type Locale,
} from "@/lib/routes"

export type MenuItem = { icon: IconName; title: string; body: string; href: string }
export type MenuGroup = { id: string; label: string; cols: 1 | 2; items: MenuItem[] }

export function headerCopy(i18n: I18n, locale: Locale) {
  const docs = (rel: string) => `${docsRoot(locale)}${rel}`
  const page = (id: string) => `${home[locale]}#${id}`
  return {
    home: i18n._(msg`Yuva home`),
    logIn: i18n._(msg`Log in`),
    startFree: i18n._(msg`Start free`),
    sections: i18n._(msg`Sections`),
    menu: i18n._(msg`Menu`),
    logoMenu: i18n._(msg`Logo options`),
    copyLogo: i18n._(msg`Copy logo as SVG`),
    copyWordmark: i18n._(msg`Copy logo with name as SVG`),
    downloadLogo: i18n._(msg`Download logo`),
    brand: i18n._(msg`Brand assets`),
    copied: i18n._(msg`Copied`),
    groups: [
      {
        id: "product",
        label: i18n._(msg`Product`),
        cols: 2,
        items: [
          {
            icon: "inbox",
            title: i18n._(msg`Inbox and conversations`),
            body: i18n._(msg`Every channel in one list.`),
            href: page("inbox"),
          },
          {
            icon: "chat",
            title: i18n._(msg`Channels`),
            body: i18n._(msg`E-mail, live chat and in-app.`),
            href: page("channels"),
          },
          {
            icon: "send",
            title: i18n._(msg`Broadcasts`),
            body: i18n._(msg`Newsletters to your lists.`),
            href: page("broadcasts"),
          },
          {
            icon: "mail",
            title: i18n._(msg`E-mail templates`),
            body: i18n._(msg`Free designs to copy.`),
            href: templates[locale],
          },
          {
            icon: "chart",
            title: i18n._(msg`Analytics`),
            body: i18n._(msg`Visitors without cookies.`),
            href: page("analytics"),
          },
          {
            icon: "download",
            title: i18n._(msg`Moving in`),
            body: i18n._(msg`Bring your mail and history.`),
            href: page("moving-in"),
          },
          {
            icon: "bolt",
            title: i18n._(msg`AI assistants and MCP`),
            body: i18n._(msg`Claude drafts, you send.`),
            href: ai[locale],
          },
        ],
      },
      {
        id: "developers",
        label: i18n._(msg`Developers`),
        cols: 1,
        items: [
          {
            icon: "book",
            title: i18n._(msg`Documentation`),
            body: i18n._(msg`Guides for every part.`),
            href: docsRoot(locale),
          },
          {
            icon: "code",
            title: i18n._(msg`API reference`),
            body: i18n._(msg`Every endpoint, to try.`),
            href: docs("api/"),
          },
          {
            icon: "network",
            title: i18n._(msg`Webhooks and identity`),
            body: i18n._(msg`Signed events and user tokens.`),
            href: docs("webhooks/"),
          },
          {
            icon: "server",
            title: i18n._(msg`Self-hosting`),
            body: i18n._(msg`One binary and Postgres.`),
            href: docs("install/"),
          },
          { icon: "flag", title: "GitHub", body: i18n._(msg`The code, open source.`), href: links.repo },
        ],
      },
      {
        id: "resources",
        label: i18n._(msg`Resources`),
        cols: 1,
        items: [
          {
            icon: "note",
            title: i18n._(msg`Blog`),
            body: i18n._(msg`News and how Yuva is built.`),
            href: blog[locale],
          },
          {
            icon: "tag",
            title: i18n._(msg`Releases`),
            body: i18n._(msg`What changed in each version.`),
            href: releases[locale],
          },
          {
            icon: "flag",
            title: i18n._(msg`Roadmap`),
            body: i18n._(msg`What is built and what is next.`),
            href: docs("roadmap/"),
          },
          {
            icon: "download",
            title: i18n._(msg`Brand assets`),
            body: i18n._(msg`Logo and icon to download.`),
            href: brand[locale],
          },
          {
            icon: "network",
            title: i18n._(msg`Cloudflare e-mail`),
            body: i18n._(msg`Receive mail with no server.`),
            href: cloudflareEmail[locale],
          },
          {
            icon: "chat",
            title: i18n._(msg`Contact`),
            body: i18n._(msg`Reach the people behind Yuva.`),
            href: contact[locale],
          },
        ],
      },
    ] as MenuGroup[],
  }
}

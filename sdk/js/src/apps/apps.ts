import type { MessageDescriptor, Messages } from "@lingui/core"
import { messages as en } from "../locales/en/apps"
import { addStyles } from "../styles"
import { translate } from "../translate"
import type { AppContext, AppHandle, AppKind } from "../types"
import { el } from "../panel/helpers"
import { feedbackApp } from "./feedback"
import { helpApp } from "./help"
import { statusApp } from "./status"
import { styles } from "./styles"
import { subscribeApp } from "./subscribe"

export type Translate = (descriptor: MessageDescriptor) => string

const apps = { feedback: feedbackApp, help: helpApp, status: statusApp, subscribe: subscribeApp }

declare const YUVA_APPS_LOCALES: string | undefined

const loadMessages = (locale: string): Promise<Messages> =>
  typeof YUVA_APPS_LOCALES === "string"
    ? import(/* @vite-ignore */ new URL(localeFile(YUVA_APPS_LOCALES, locale), import.meta.url).href)
        .then((module: { messages: Messages }) => module.messages)
        .catch(() => en)
    : import("../catalogs/apps").then((module) => module.catalogs[locale] ?? en)

const localeFile = (map: string, locale: string): string =>
  (JSON.parse(map) as Record<string, string>)[locale] ?? "about:blank"

export async function mountApp(kind: AppKind, context: AppContext): Promise<AppHandle> {
  const messages = await loadMessages(context.locale)
  const t: Translate = (descriptor) => translate(messages, descriptor, en)
  const element = el("div", "app")
  const page = el("div", "page")
  const fallback = addStyles(context.root, styles)
  element.append(...(fallback ? [fallback] : []), page)
  const destroy = apps[kind](context, t, page)
  return { element, destroy }
}

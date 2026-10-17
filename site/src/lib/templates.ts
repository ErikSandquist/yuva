import fs from "node:fs"
import path from "node:path"
import type { Locale } from "./routes"

const root = path.resolve("../templates/email")
const icons = path.resolve("../api/internal/mail/design/icons")

type Text = Partial<Record<Locale, string>> & { en: string }
export interface Template {
  slug: string
  order: number
  category: string
  name: Text
  description: Text
  author: string
}

const isTemplate = (dir: string) => fs.existsSync(path.join(root, dir, "meta.json"))

export function templates(): Template[] {
  return fs
    .readdirSync(root)
    .filter(isTemplate)
    .map((dir) => JSON.parse(fs.readFileSync(path.join(root, dir, "meta.json"), "utf8")) as Template)
    .sort((a, b) => a.order - b.order || a.slug.localeCompare(b.slug))
}

export const previewHtml = (slug: string, locale: Locale) => `/template-files/${slug}/preview.${locale}.html`
export const previewImage = (slug: string, locale: Locale) => `/template-files/${slug}/preview.${locale}.webp`

function* walk(dir: string, base = ""): Generator<string> {
  for (const name of fs.readdirSync(dir)) {
    const rel = base ? `${base}/${name}` : name
    if (fs.statSync(path.join(dir, name)).isDirectory()) yield* walk(path.join(dir, name), rel)
    else yield rel
  }
}

const served = /^(preview\.[a-z]{2}\.(html|webp)|assets\/[^/]+)$/

export function previewFiles(): { path: string; file: string }[] {
  return templates().flatMap((t) =>
    [...walk(path.join(root, t.slug))]
      .filter((rel) => served.test(rel))
      .map((rel) => ({ path: `${t.slug}/${rel}`, file: path.join(root, t.slug, rel) }))
  )
}

export function socialIcons(): { name: string; file: string }[] {
  return fs
    .readdirSync(icons)
    .filter((n) => n.endsWith(".png"))
    .map((name) => ({ name, file: path.join(icons, name) }))
}

export const text = (value: Text, locale: Locale) => value[locale] ?? value.en
export const available = (t: Template, locale: Locale) =>
  fs.existsSync(path.join(root, t.slug, `preview.${locale}.html`)) &&
  fs.existsSync(path.join(root, t.slug, `preview.${locale}.webp`))
export const previewLocale = (t: Template, locale: Locale): Locale => (available(t, locale) ? locale : "en")

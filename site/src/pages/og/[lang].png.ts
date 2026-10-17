import { msg } from "@lingui/core/macro"
import type { APIRoute, GetStaticPaths } from "astro"
import { i18nFor } from "@/i18n"
import { ogSvg, renderPng } from "@/lib/graphics"
import { locales, type Locale } from "@/lib/routes"

const glyphs: Locale[] = [
  "en",
  "ca",
  "cs",
  "da",
  "de",
  "el",
  "es",
  "es-MX",
  "fi",
  "fr",
  "fr-CA",
  "hr",
  "hu",
  "id",
  "it",
  "ms",
  "nb",
  "nl",
  "pl",
  "pt-BR",
  "pt-PT",
  "ro",
  "ru",
  "sk",
  "sl",
  "sv",
  "tr",
  "uk",
  "vi",
]

export const getStaticPaths: GetStaticPaths = () => locales.map((lang) => ({ params: { lang } }))

export const GET: APIRoute = ({ params }) => {
  const lang = params.lang as Locale
  const i18n = i18nFor(glyphs.includes(lang) ? lang : "en")
  const svg = ogSvg(
    i18n._(msg`One inbox for every product you run.`),
    i18n._(msg`Open-source customer messaging`),
    i18n._(msg`E-mail · live chat · in-app · self-hosted`)
  )
  return new Response(renderPng(svg, 1200), { headers: { "Content-Type": "image/png" } })
}

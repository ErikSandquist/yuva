// @ts-check
import { lingui, linguiTransformerBabelPreset } from "@lingui/vite-plugin"
import babel from "@rolldown/plugin-babel"
import tailwindcss from "@tailwindcss/vite"
import react from "@astrojs/react"
import { satteri } from "@astrojs/markdown-satteri"
import { defineConfig } from "astro/config"
import { fileURLToPath } from "node:url"
import { callouts } from "./src/lib/callouts"
import { repoLinks } from "./src/lib/repo-links"
import { defaultLocale, locales, SITE } from "./src/lib/routes"

const shadcn = (rel) => fileURLToPath(new URL(`./src/shadcn/${rel}`, import.meta.url))

export default defineConfig({
  site: SITE,
  output: "static",
  i18n: {
    locales: [...locales],
    defaultLocale,
    routing: { prefixDefaultLocale: false },
  },
  build: { format: "directory", assets: "_site", inlineStylesheets: "always" },
  integrations: [react()],
  devToolbar: { enabled: false },
  markdown: {
    processor: satteri({ hastPlugins: [repoLinks, callouts] }),
    shikiConfig: { themes: { light: "github-light", dark: "github-dark" } },
  },
  vite: {
    resolve: {
      preserveSymlinks: true,
      alias: [
        { find: /^@\/components\/ui\/(.*)$/, replacement: `${shadcn("ui")}/$1` },
        { find: /^@\/components\/(yuva-.*)$/, replacement: `${shadcn("registry/components")}/$1` },
        { find: /^@\/hooks\/(.*)$/, replacement: `${shadcn("registry/hooks")}/$1` },
        { find: /^@\/lib\/utils$/, replacement: shadcn("utils.ts") },
        { find: /^@\/lib\/(yuva-.*)$/, replacement: `${shadcn("registry/lib")}/$1` },
      ],
    },
    plugins: [
      tailwindcss(),
      lingui(),
      babel({
        include: [/\.(?:[jt]sx?|[cm][jt]s|astro)(?:$|\?)/],
        presets: [linguiTransformerBabelPreset()],
      }),
    ],
    ssr: { external: ["@resvg/resvg-js"] },
  },
})

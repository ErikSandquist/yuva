import { defineConfig } from "@lingui/conf"
import { formatter } from "@lingui/format-po"
import { createSwcExtractor } from "@lingui/native-tools"

export default defineConfig({
  sourceLocale: "en",
  locales: ["en", "tr"],
  catalogs: [
    { path: "<rootDir>/src/locales/{locale}/loader", include: ["<rootDir>/src/element.ts"] },
    { path: "<rootDir>/src/locales/{locale}/panel", include: ["<rootDir>/src/panel"] },
    {
      path: "<rootDir>/src/locales/{locale}/docs",
      include: ["<rootDir>/src/docs"],
      exclude: ["<rootDir>/src/docs/thread.ts"],
    },
    { path: "<rootDir>/src/locales/{locale}/thread", include: ["<rootDir>/src/docs/thread.ts"] },
  ],
  format: formatter({ lineNumbers: false }),
  extractors: [createSwcExtractor()],
})

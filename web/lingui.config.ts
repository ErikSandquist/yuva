import { defineConfig } from "@lingui/conf"
import { formatter } from "@lingui/format-po"

import { localeCodes } from "./src/localeList"

export default defineConfig({
  sourceLocale: "en",
  locales: [...localeCodes],
  catalogs: [
    {
      path: "<rootDir>/src/locales/{locale}/messages",
      include: ["src"],
      exclude: [
        "src/features/designs",
        "src/features/analytics",
        "src/features/help",
        "src/features/roadmap",
        "src/features/changelog",
      ],
    },
    {
      path: "<rootDir>/src/locales/{locale}/analytics",
      include: ["src/features/analytics"],
    },
    {
      path: "<rootDir>/src/locales/{locale}/help",
      include: ["src/features/help"],
    },
    {
      path: "<rootDir>/src/locales/{locale}/roadmap",
      include: ["src/features/roadmap"],
    },
    {
      path: "<rootDir>/src/locales/{locale}/changelog",
      include: ["src/features/changelog"],
    },
    {
      path: "<rootDir>/src/locales/{locale}/designs",
      include: ["src/features/designs"],
    },
  ],
  format: formatter({ lineNumbers: false }),
})

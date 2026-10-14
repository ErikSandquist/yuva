import { defineConfig } from "nitro"

export default defineConfig({
  serverDir: "./server",
  modules: ["useyuva/nitro"],
  runtimeConfig: {
    yuva: { channel: "" },
  },
})

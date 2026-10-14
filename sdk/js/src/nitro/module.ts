import { resolve } from "node:path"
import type { NitroModule } from "nitro/types"
import {
  findServerFile,
  IDENTITY_ROUTE,
  runtimePath,
  SERVER_MODULE,
  serverModule,
  serverParts,
  serverRuntimeConfig,
  WEBHOOK_ROUTE,
  type YuvaServerOptions,
} from "./setup"

export interface YuvaNitroOptions extends YuvaServerOptions {
  server?: string
}

export function yuva(options: YuvaNitroOptions = {}): NitroModule {
  return {
    name: "useyuva",
    setup(nitro) {
      const config = serverRuntimeConfig(nitro.options.runtimeConfig.yuva, options)
      nitro.options.runtimeConfig.yuva = config
      const parts = serverParts(config, process.env, "NITRO_")
      if (!parts.identity && !parts.webhook && !parts.analytics) return

      const dir = nitro.options.serverDir
        ? resolve(nitro.options.rootDir, nitro.options.serverDir)
        : nitro.options.rootDir
      const server = findServerFile(dir)
      if (!server && parts.identity)
        nitro.logger.warn(`useyuva: no yuva.ts in ${dir}, so ${IDENTITY_ROUTE} answers 404`)
      nitro.options.virtual[SERVER_MODULE] = serverModule({
        nitro: 3,
        server,
        runtimeConfig: "nitro/runtime-config",
      })
      if (parts.identity)
        nitro.options.handlers.push({ route: IDENTITY_ROUTE, method: "GET", handler: runtimePath("identity") })
      if (parts.webhook)
        nitro.options.handlers.push({ route: WEBHOOK_ROUTE, method: "POST", handler: runtimePath("webhook") })
      if (parts.analytics) nitro.options.plugins.push(runtimePath("plugin"))
    },
  }
}

export default yuva()

import { rm } from "node:fs/promises"
import { linguiMacro } from "./lingui-plugin"
import pkg from "../package.json"

process.env.NODE_ENV = "production"
await rm("dist", { recursive: true, force: true })

function check(result: Bun.BuildOutput): void {
  if (result.success) return
  for (const log of result.logs) console.error(log)
  process.exit(1)
}

async function chunk(entry: string, name: string): Promise<string> {
  const result = await Bun.build({
    entrypoints: [entry],
    outdir: "dist/widget",
    format: "esm",
    target: "browser",
    minify: true,
    naming: `${name}-[hash].[ext]`,
    define: { "process.env.NODE_ENV": '"production"' },
    plugins: [linguiMacro],
  })
  check(result)
  return `widget/${result.outputs[0]!.path.split("/").pop()}`
}

const thread = await chunk("src/docs/thread.ts", "yuva-docs-thread")
const subscribeClient = await chunk("src/subscribe/client.ts", "yuva-subscribe-client")

const define = {
  "process.env.NODE_ENV": '"production"',
  YUVA_DOCS_THREAD: JSON.stringify(thread),
  YUVA_SUBSCRIBE_CLIENT: JSON.stringify(subscribeClient),
}

const scripts = [
  { entry: "src/yuva.ts", format: "iife" },
  { entry: "src/yuva-chat.ts", format: "esm" },
  { entry: "src/yuva-docs.ts", format: "iife" },
  { entry: "src/yuva-subscribe.ts", format: "iife" },
  { entry: "src/yuva-analytics.ts", format: "iife", lite: true },
  { entry: "src/yuva-analytics.ts", format: "iife", naming: "yuva-analytics-consent.js" },
] as const

for (const script of scripts) {
  check(
    await Bun.build({
      entrypoints: [script.entry],
      outdir: "dist",
      format: script.format,
      target: "browser",
      minify: true,
      naming: "naming" in script ? script.naming : undefined,
      define: "lite" in script ? { ...define, YUVA_ANALYTICS_LITE: "true" } : define,
      plugins: [linguiMacro],
    })
  )
}

const entries = [
  "src/index.ts",
  "src/chat.ts",
  "src/docs.ts",
  "src/subscribe.ts",
  "src/analytics.ts",
  "src/react.ts",
  "src/api.ts",
]
check(
  await Bun.build({
    entrypoints: entries,
    outdir: "dist/esm",
    root: "src",
    format: "esm",
    target: "node",
    splitting: true,
    minify: { syntax: true, whitespace: true },
    naming: { chunk: "[name]-[hash].js" },
    external: [...Object.keys(pkg.dependencies), ...Object.keys(pkg.peerDependencies)],
    define: { "process.env.NODE_ENV": '"production"' },
    plugins: [linguiMacro],
  })
)

// Bun drops module directives; Next.js needs this one to render the React components from server layouts.
await Bun.write("dist/esm/react.js", `"use client";\n${await Bun.file("dist/esm/react.js").text()}`)

const transpiler = new Bun.Transpiler({ loader: "ts", target: "node" })
for (const name of ["nuxt/module", "nuxt/runtime/components", "nuxt/runtime/analytics", "nuxt/runtime/plugin"]) {
  const code = transpiler.transformSync(await Bun.file(`src/${name}.ts`).text())
  await Bun.write(`dist/esm/${name}.js`, code.replace(/((?:from|import\()\s*")(\.\.?\/[^"]+?)(?<!\.js)"/g, '$1$2.js"'))
}

const tsc = Bun.spawnSync(["bunx", "tsc", "-p", "tsconfig.build.json"], { stdout: "inherit", stderr: "inherit" })
if (tsc.exitCode !== 0) process.exit(tsc.exitCode ?? 1)

// Node's ESM resolution (moduleResolution node16/nodenext) needs explicit extensions in declarations too.
for await (const path of new Bun.Glob("dist/types/**/*.d.ts").scan()) {
  const text = await Bun.file(path).text()
  await Bun.write(path, text.replace(/((?:from|import\()\s*")(\.\.?\/[^"]+?)(?<!\.js)"/g, '$1$2.js"'))
}

const budgets: Record<string, number> = {
  "yuva-analytics.js": 1536,
  "yuva-analytics-consent.js": 2048,
  "esm/analytics.js": 1920,
}
let over = false
for (const name of [
  "yuva.js",
  "yuva-chat.js",
  "yuva-docs.js",
  "yuva-subscribe.js",
  "yuva-analytics.js",
  "yuva-analytics-consent.js",
  thread,
  subscribeClient,
  "esm/index.js",
  "esm/chat.js",
  "esm/docs.js",
  "esm/subscribe.js",
  "esm/analytics.js",
  "esm/react.js",
  "esm/api.js",
  "esm/nuxt/module.js",
  "esm/nuxt/runtime/components.js",
  "esm/nuxt/runtime/analytics.js",
  "esm/nuxt/runtime/plugin.js",
]) {
  const bytes = await Bun.file(`dist/${name}`).bytes()
  const gzip = Bun.gzipSync(bytes, { level: 9 }).length
  const budget = budgets[name]
  console.log(`dist/${name}  ${bytes.length} B  gzip ${gzip} B${budget ? ` (budget ${budget} B)` : ""}`)
  if (budget && gzip > budget) over = true
}
if (over) {
  console.error("A script is over its gzip budget.")
  process.exit(1)
}

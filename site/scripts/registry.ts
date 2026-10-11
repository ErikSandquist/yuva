import { spawnSync } from "node:child_process"
import { readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { join } from "node:path"
import { SITE } from "../src/lib/routes"

const source = join(import.meta.dirname, "../../sdk/shadcn")
const out = join(import.meta.dirname, "../public/r")
const published = "https://useyuva.com/r/"

rmSync(out, { recursive: true, force: true })
const build = spawnSync(
  join(import.meta.dirname, "../node_modules/.bin/shadcn"),
  ["build", join(source, "registry.json"), "--cwd", source, "--output", out],
  { stdio: "inherit" }
)
if (build.status !== 0) process.exit(build.status ?? 1)

const base = new URL("/r/", SITE).toString()
const files = readdirSync(out).filter((name) => name.endsWith(".json"))
for (const name of files) {
  const path = join(out, name)
  writeFileSync(path, readFileSync(path, "utf8").replaceAll(published, base))
}
console.log(`registry: ${files.length} files in public/r for ${base}`)

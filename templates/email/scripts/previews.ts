// Screenshots every preview.<locale>.html (written by `yuva templates render`) to
// preview.<locale>.webp: the 600 px design in a 640 px window, wide enough for the desktop layout,
// at twice the density. Maintainers run it by hand; font rendering differs between machines, so CI
// does not compare the images.
import { readdirSync, existsSync } from "node:fs"
import { join, resolve } from "node:path"
import { chromium } from "playwright"
import sharp from "sharp"

const root = resolve(import.meta.dir, "..")
const icons = resolve(root, "../../api/internal/mail/design/icons")
const only = process.argv.slice(2)

const server = Bun.serve({
  port: 0,
  hostname: "127.0.0.1",
  fetch(req) {
    const path = decodeURIComponent(new URL(req.url).pathname)
    if (path.includes("..")) return new Response("not found", { status: 404 })
    const file = path.startsWith("/i/s/") ? join(icons, path.slice(5)) : join(root, path)
    return existsSync(file) ? new Response(Bun.file(file)) : new Response("not found", { status: 404 })
  },
})

const browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || undefined })
try {
  const page = await browser.newPage({ viewport: { width: 640, height: 800 }, deviceScaleFactor: 2 })
  for (const slug of readdirSync(root).filter((d) => existsSync(join(root, d, "meta.json")))) {
    if (only.length && !only.includes(slug)) continue
    for (const file of readdirSync(join(root, slug)).filter((f) => /^preview\.[a-z]{2}\.html$/.test(f))) {
      await page.goto(`http://127.0.0.1:${server.port}/${slug}/${file}`, { waitUntil: "networkidle" })
      const png = await page.screenshot({ fullPage: true })
      const out = join(root, slug, file.replace(/\.html$/, ".webp"))
      await sharp(png).webp({ quality: 80 }).toFile(out)
      console.log(out)
    }
  }
} finally {
  await browser.close()
  server.stop(true)
}

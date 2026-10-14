import { afterAll, beforeAll, describe, expect, test } from "bun:test"
import { GlobalRegistrator } from "@happy-dom/global-registrator"
import { rmSync } from "node:fs"
import { createServer } from "node:net"
import { join } from "node:path"

const dir = join(import.meta.dir, "next-app")
const bin = join(import.meta.dir, "../node_modules/.bin/next")
const records: Record<string, unknown>[] = []
let sink: Bun.Server<undefined>

const listen = () =>
  Bun.serve({
    port: 0,
    hostname: "127.0.0.1",
    async fetch(request) {
      if (new URL(request.url).pathname !== "/v1/analytics/properties/prop-1/requests") {
        return new Response(null, { status: 404 })
      }
      const body = new Uint8Array(await request.arrayBuffer())
      records.push(...JSON.parse(new TextDecoder().decode(Bun.gunzipSync(body))).records)
      return Response.json({ accepted: 1, errors: [] }, { status: 202 })
    },
  })

const freePort = () =>
  new Promise<number>((resolve) => {
    const server = createServer().listen(0, "127.0.0.1", () => {
      const { port } = server.address() as { port: number }
      server.close(() => resolve(port))
    })
  })

let app: Bun.Subprocess | undefined
let origin = ""

beforeAll(
  async () => {
    // Bun.serve takes only the runtime's own Response, which happy-dom replaces.
    GlobalRegistrator.unregister()
    sink = listen()
    const env = { ...process.env, NEXT_TELEMETRY_DISABLED: "1", YUVA_TEST_SERVER: sink.url.href }
    const build = Bun.spawn([bin, "build"], { cwd: dir, env, stdout: "pipe", stderr: "pipe" })
    if ((await build.exited) !== 0) throw new Error(await new Response(build.stderr).text())
    const port = await freePort()
    origin = `http://127.0.0.1:${port}`
    app = Bun.spawn([bin, "start", "-p", String(port), "-H", "127.0.0.1"], { cwd: dir, env, stdout: "ignore" })
    for (let i = 0; i < 300; i++) {
      if (
        await fetch(`${origin}/users/ready`).then(
          (res) => res.ok,
          () => false
        )
      )
        return
      await Bun.sleep(50)
    }
    throw new Error("next start did not answer")
  },
  { timeout: 120_000 }
)

afterAll(async () => {
  app?.kill()
  await app?.exited
  await sink?.stop(true)
  rmSync(join(dir, ".next"), { recursive: true, force: true })
  rmSync(join(dir, "next-env.d.ts"), { force: true })
  GlobalRegistrator.register()
})

describe("next", () => {
  test("a route handler records its route and flushes through after()", async () => {
    for (let i = 0; i < 100 && records.length === 0; i++) await Bun.sleep(20)
    records.length = 0
    const ok = await fetch(`${origin}/users/42?token=secret`, { headers: { "x-client": "acme" } })
    expect(await ok.json()).toEqual({ id: "42" })
    const missing = await fetch(`${origin}/users/missing`)
    expect(missing.status).toBe(404)
    for (let i = 0; i < 100 && records.length < 2; i++) await Bun.sleep(20)
    expect(records).toHaveLength(2)
    expect(records[0]).toMatchObject({ method: "GET", route: "/users/{id}", status: 200, consumer: "acme" })
    expect(records[1]).toMatchObject({ method: "GET", route: "/users/{id}", status: 404 })
    expect(JSON.stringify(records)).not.toContain("secret")
  })
})

const maxBackoff = 60_000

export class AnalyticsBatchError extends Error {
  readonly status: number
  readonly body: string
  readonly events: number

  constructor(events: number, status: number, body: string, cause?: unknown) {
    super(
      cause === undefined
        ? `analytics: ${events} events not sent: ${status} ${body}`
        : `analytics: ${events} events not sent: ${cause instanceof Error ? cause.message : String(cause)}`,
      cause === undefined ? undefined : { cause }
    )
    this.name = "AnalyticsBatchError"
    this.status = status
    this.body = body
    this.events = events
  }
}

type Timer = ReturnType<typeof setTimeout>

export function unref(timer: Timer): Timer {
  if (typeof timer === "object" && timer && "unref" in timer && typeof timer.unref === "function") timer.unref()
  else (globalThis as { Deno?: { unrefTimer?(id: unknown): void } }).Deno?.unrefTimer?.(timer)
  return timer
}

const sleep = (ms: number, held: boolean) =>
  new Promise<void>((resolve) => {
    const timer = setTimeout(resolve, ms)
    if (!held) unref(timer)
  })

async function gzip(text: string): Promise<Uint8Array<ArrayBuffer>> {
  const stream = new CompressionStream("gzip")
  const writer = stream.writable.getWriter()
  const written = writer.write(new TextEncoder().encode(text)).then(() => writer.close())
  const reader = stream.readable.getReader()
  const chunks: Uint8Array[] = []
  let length = 0
  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    chunks.push(value)
    length += value.length
  }
  await written
  const out = new Uint8Array(length)
  let offset = 0
  for (const chunk of chunks) {
    out.set(chunk, offset)
    offset += chunk.length
  }
  return out
}

export interface SenderConfig<T> {
  endpoint: string
  apiKey: string
  fetch?: typeof fetch
  key: "events" | "records"
  batch: number
  gzip: boolean
  flushInterval?: number
  queueSize: number
  maxRetries?: number
  minBackoff?: number
  full: string
  wire(item: T): unknown
  refused(item: T, detail: string): Error
  report(error: Error): void
}

export interface Sender<T> {
  push(item: T): void
  run(held: boolean): Promise<Error | undefined>
  stop(): void
}

export function createSender<T>(config: SenderConfig<T>): Sender<T> {
  const interval = config.flushInterval && config.flushInterval > 0 ? config.flushInterval : 5_000
  const maxRetries = config.maxRetries || 5
  const minBackoff = config.minBackoff && config.minBackoff > 0 ? config.minBackoff : 1_000

  let queue: T[] = []
  let timer: Timer | undefined
  let chain: Promise<unknown> = Promise.resolve()

  async function post(body: string | Uint8Array<ArrayBuffer>) {
    const headers: Record<string, string> = {
      Authorization: `Bearer ${config.apiKey}`,
      "Content-Type": "application/json",
    }
    if (config.gzip) headers["Content-Encoding"] = "gzip"
    const res = await (config.fetch ?? fetch)(config.endpoint, { method: "POST", headers, body })
    const raw = (await res.text()).slice(0, 64 << 10).trim()
    const seconds = Number(res.headers.get("retry-after"))
    return { status: res.status, raw, retryAfter: Number.isInteger(seconds) && seconds > 0 ? seconds * 1000 : 0 }
  }

  async function sendBatch(batch: T[], held: boolean): Promise<Error | undefined> {
    let body: string | Uint8Array<ArrayBuffer>
    try {
      const text = JSON.stringify({ [config.key]: batch.map(config.wire) })
      body = config.gzip ? await gzip(text) : text
    } catch (error) {
      return new AnalyticsBatchError(batch.length, 0, "", error)
    }
    let backoff = minBackoff
    for (let attempt = 0; ; attempt++) {
      const res = await post(body).catch((error: unknown) => ({ error }))
      if (!("error" in res)) {
        if (res.status === 202) {
          try {
            const result = JSON.parse(res.raw) as { errors?: { index: number; detail: string }[] }
            for (const { index, detail } of result.errors ?? []) {
              const item = batch[index]
              if (item) config.report(config.refused(item, detail))
            }
          } catch {}
          return undefined
        }
        if (res.status !== 429 && res.status < 500) return new AnalyticsBatchError(batch.length, res.status, res.raw)
      }
      if (attempt >= maxRetries) {
        return "error" in res
          ? new AnalyticsBatchError(batch.length, 0, "", res.error)
          : new AnalyticsBatchError(batch.length, res.status, res.raw)
      }
      await sleep(Math.max(backoff, "error" in res ? 0 : res.retryAfter), held)
      backoff = Math.min(backoff * 2, maxBackoff)
    }
  }

  async function sendAll(held: boolean): Promise<Error | undefined> {
    let last: Error | undefined
    for (;;) {
      const batch = queue.slice(0, config.batch)
      queue = queue.slice(batch.length)
      if (batch.length === 0) return last
      const error = await sendBatch(batch, held)
      if (error) {
        last = error
        config.report(error)
      }
    }
  }

  function run(held: boolean): Promise<Error | undefined> {
    const next = chain.then(() => sendAll(held))
    chain = next
    return next
  }

  return {
    push(item) {
      if (queue.length >= config.queueSize) {
        config.report(new Error(config.full))
        return
      }
      queue.push(item)
      if (queue.length >= config.batch) {
        void run(false)
      } else if (!timer) {
        timer = unref(
          setTimeout(() => {
            timer = undefined
            void run(false)
          }, interval)
        )
      }
    },
    run,
    stop() {
      if (timer) clearTimeout(timer)
      timer = undefined
    },
  }
}

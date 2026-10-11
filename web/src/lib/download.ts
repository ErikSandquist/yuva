import { unwrap } from "@/lib/api"

function fileName(disposition: string | null, fallback: string) {
  const star = disposition?.match(/filename\*=UTF-8''([^;]+)/i)?.[1]
  if (star) {
    try {
      return decodeURIComponent(star)
    } catch {
      return fallback
    }
  }
  return disposition?.match(/filename="?([^";]+)"?/i)?.[1] ?? fallback
}

export async function saveDownload(
  call: Promise<{ data?: unknown; error?: unknown; response: Response }>,
  fallback: string
) {
  const [blob, response] = await Promise.all([unwrap(call) as Promise<Blob>, call.then((r) => r.response)])
  const url = URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = url
  a.download = fileName(response.headers.get("Content-Disposition"), fallback)
  document.body.append(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

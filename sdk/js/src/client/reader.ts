import { ApiError, type ReaderProfile, type ReaderToken } from "./api"

export interface ReaderSession {
  token: string
  expires_at: string
  reader: ReaderProfile
}

export interface ReaderSignInOptions {
  width?: number
  height?: number
}

export function readerConnectUrl(server: string, channel: string, origin: string): string {
  const url = new URL("/account/connect", server)
  url.searchParams.set("channel", channel)
  url.searchParams.set("origin", origin)
  return url.href
}

function isReaderToken(value: unknown): value is ReaderToken {
  const data = value as Partial<ReaderToken> | null
  return (
    !!data &&
    typeof data.token === "string" &&
    data.token.startsWith("yuva_rd_") &&
    typeof data.expires_at === "string" &&
    !Number.isNaN(Date.parse(data.expires_at)) &&
    typeof data.channel_key === "string" &&
    typeof data.origin === "string" &&
    !!data.reader &&
    typeof data.reader.initials === "string"
  )
}

// window.open must run in the click handler itself, or browsers block the popup.
export function openReaderPopup(
  server: string,
  channel: string,
  { width = 460, height = 640 }: ReaderSignInOptions = {}
): Promise<ReaderSession> {
  const serverOrigin = new URL(server).origin
  const origin = location.origin
  const left = Math.max(0, (window.screenX ?? 0) + ((window.outerWidth ?? width) - width) / 2)
  const top = Math.max(0, (window.screenY ?? 0) + ((window.outerHeight ?? height) - height) / 2)
  const popup = window.open(
    readerConnectUrl(server, channel, origin),
    "yuva-reader",
    `popup=yes,width=${width},height=${height},left=${Math.round(left)},top=${Math.round(top)}`
  )
  if (!popup) return Promise.reject(new ApiError(0, "popup_blocked"))
  return new Promise((resolve, reject) => {
    let done = false
    const finish = () => {
      done = true
      window.removeEventListener("message", received)
      clearInterval(watch)
    }
    const received = (event: MessageEvent) => {
      if (done || event.origin !== serverOrigin || (event.source && event.source !== popup)) return
      const data: unknown = event.data
      if (!isReaderToken(data) || data.channel_key !== channel || data.origin !== origin) return
      finish()
      try {
        popup.close()
      } catch {}
      resolve({ token: data.token, expires_at: data.expires_at, reader: data.reader })
    }
    const watch = setInterval(() => {
      let closed = false
      try {
        closed = popup.closed
      } catch {}
      if (!closed) return
      clearInterval(watch)
      // The token message may still be queued behind the close.
      setTimeout(() => {
        if (done) return
        finish()
        reject(new ApiError(0, "sign_in_cancelled"))
      }, 300)
    }, 500)
    window.addEventListener("message", received)
  })
}

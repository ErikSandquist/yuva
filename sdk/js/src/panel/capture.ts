export const canCapture = () => typeof navigator !== "undefined" && !!navigator.mediaDevices?.getDisplayMedia

export async function captureScreen(hide?: HTMLElement): Promise<File | null> {
  let stream: MediaStream
  try {
    stream = await navigator.mediaDevices.getDisplayMedia({
      video: true,
      preferCurrentTab: true,
    } as DisplayMediaStreamOptions)
  } catch {
    return null
  }
  try {
    const video = document.createElement("video")
    video.muted = true
    video.srcObject = stream
    await video.play()
    if (hide) hide.style.visibility = "hidden"
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)))
    const canvas = document.createElement("canvas")
    canvas.width = video.videoWidth
    canvas.height = video.videoHeight
    canvas.getContext("2d")?.drawImage(video, 0, 0)
    const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/png"))
    return blob ? new File([blob], "screenshot.png", { type: "image/png" }) : null
  } catch {
    return null
  } finally {
    if (hide) hide.style.visibility = ""
    for (const track of stream.getTracks()) track.stop()
  }
}

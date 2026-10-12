import type { APIRoute } from "astro"
import fs from "node:fs"
import { previewFiles } from "@/lib/templates"

const types: Record<string, string> = {
  html: "text/html; charset=utf-8",
  webp: "image/webp",
  png: "image/png",
  jpg: "image/jpeg",
  gif: "image/gif",
}

export function getStaticPaths() {
  return previewFiles().map((f) => ({ params: { path: f.path }, props: { file: f.file } }))
}

export const GET: APIRoute = ({ props }) => {
  const file = props.file as string
  let body: Buffer | string = fs.readFileSync(file)
  const ext = file.split(".").pop()!
  if (ext === "html") body = body.toString("utf8").replace("<head>", '<head>\n<base target="_blank">')
  return new Response(body, { headers: { "Content-Type": types[ext] ?? "application/octet-stream" } })
}

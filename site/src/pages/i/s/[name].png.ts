import type { APIRoute } from "astro"
import fs from "node:fs"
import { socialIcons } from "@/lib/templates"

export function getStaticPaths() {
  return socialIcons().map((i) => ({ params: { name: i.name.replace(/\.png$/, "") }, props: { file: i.file } }))
}

export const GET: APIRoute = ({ props }) =>
  new Response(fs.readFileSync(props.file as string), { headers: { "Content-Type": "image/png" } })

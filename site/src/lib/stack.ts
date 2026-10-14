import {
  siAstro,
  siDocker,
  siExpress,
  siFastify,
  siFirebase,
  siGo,
  siHono,
  siKotlin,
  siMinio,
  siModelcontextprotocol,
  siNextdotjs,
  siNuxt,
  siOpenapiinitiative,
  siReact,
  siShadcnui,
  siSwift,
  siVuedotjs,
} from "simple-icons"
import { h3Mark, nitroMark, srvxMark } from "@/lib/marks"
import { ai, cloudflareEmail, doc, docsPath, type Locale } from "@/lib/routes"

export interface StackItem {
  name: string
  path?: string
  viewBox?: string
  href: Record<Locale, string>
}

const same = (href: string): Record<Locale, string> => ({ en: href, tr: href, de: href })
const docs = (rel: string) => docsPath(rel)

export const stack: StackItem[] = [
  { name: "Cloudflare", href: cloudflareEmail },
  { name: "Next.js", path: siNextdotjs.path, href: docs("analytics/") },
  { name: "React", path: siReact.path, href: docs("analytics/") },
  { name: "Vue", path: siVuedotjs.path, href: docs("analytics/") },
  { name: "Nuxt", path: siNuxt.path, href: docs("analytics/#nuxt") },
  { name: "Nitro", path: nitroMark, viewBox: "0 0 40 40", href: same(doc("examples/nitro")) },
  { name: "h3", path: h3Mark, viewBox: "0 0 264 264", href: same(doc("sdk/js/README.md#server")) },
  { name: "srvx", path: srvxMark, viewBox: "0 0 264 264", href: same(doc("sdk/js/README.md#srvx")) },
  { name: "Hono", path: siHono.path, href: same(doc("sdk/js/README.md#hono")) },
  { name: "Express", path: siExpress.path, href: same(doc("sdk/js/README.md#express")) },
  { name: "Fastify", path: siFastify.path, href: same(doc("sdk/js/README.md#fastify")) },
  { name: "shadcn/ui", path: siShadcnui.path, href: docs("documentation-pages/#shadcnui") },
  { name: "Astro", path: siAstro.path, href: docs("analytics/") },
  { name: "Swift", path: siSwift.path, href: docs("mobile/#2-ios") },
  { name: "Kotlin", path: siKotlin.path, href: docs("mobile/#3-android") },
  { name: "Go", path: siGo.path, href: docs("analytics-server/#go") },
  { name: "Amazon SES", href: docs("email/") },
  { name: "Postmark", href: docs("email/") },
  { name: "Amazon S3", href: docs("configuration/") },
  { name: "Cloudflare R2", href: docs("configuration/") },
  { name: "MinIO", path: siMinio.path, href: docs("configuration/") },
  { name: "APNs", href: docs("mobile/") },
  { name: "Firebase Cloud Messaging", path: siFirebase.path, href: docs("mobile/") },
  { name: "Docker", path: siDocker.path, href: docs("install/") },
  { name: "OpenAPI", path: siOpenapiinitiative.path, href: docs("api/") },
  { name: "MCP", path: siModelcontextprotocol.path, href: ai },
]

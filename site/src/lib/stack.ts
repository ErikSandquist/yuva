import {
  siAstro,
  siGo,
  siH3,
  siKotlin,
  siModelcontextprotocol,
  siNextdotjs,
  siNuxt,
  siOpenapiinitiative,
  siReact,
  siShadcnui,
  siSwift,
  siVuedotjs,
} from "simple-icons"
import { ai, cloudflareEmail, doc, docsPath, type Locale } from "@/lib/routes"

export interface StackItem {
  name: string
  path?: string
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
  { name: "Nitro", href: same(doc("examples/nitro")) },
  { name: "h3", path: siH3.path, href: same(doc("sdk/js/README.md#server")) },
  { name: "shadcn/ui", path: siShadcnui.path, href: docs("documentation-pages/#shadcnui") },
  { name: "Astro", path: siAstro.path, href: docs("analytics/") },
  { name: "Swift", path: siSwift.path, href: docs("mobile/#2-ios") },
  { name: "Kotlin", path: siKotlin.path, href: docs("mobile/#3-android") },
  { name: "Go", path: siGo.path, href: docs("analytics-server/#go") },
  { name: "OpenAPI", path: siOpenapiinitiative.path, href: docs("api/") },
  { name: "MCP", path: siModelcontextprotocol.path, href: ai },
]

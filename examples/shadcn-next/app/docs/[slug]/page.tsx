import type { Metadata } from "next"
import { notFound } from "next/navigation"
import { YuvaDiscussions } from "@/components/yuva-discussions"
import { YuvaPageFeedback } from "@/components/yuva-page-feedback"
import { Separator } from "@/components/ui/separator"
import { docs } from "@/lib/docs"

export function generateStaticParams() {
  return docs.map((doc) => ({ slug: doc.slug }))
}

export async function generateMetadata({ params }: PageProps<"/docs/[slug]">): Promise<Metadata> {
  const { slug } = await params
  const doc = docs.find((d) => d.slug === slug)
  return { title: doc?.title }
}

export default async function DocPage({ params }: PageProps<"/docs/[slug]">) {
  const { slug } = await params
  const doc = docs.find((d) => d.slug === slug)
  if (!doc) notFound()
  return (
    <article className="flex flex-col gap-6">
      <header className="flex flex-col gap-2">
        <h1 className="text-3xl font-semibold tracking-tight">{doc.title}</h1>
        <p className="text-muted-foreground">{doc.description}</p>
      </header>
      {doc.sections.map((section) => (
        <section key={section.heading} className="flex flex-col gap-2">
          <h2 className="text-xl font-semibold">{section.heading}</h2>
          <p className="leading-7">{section.body}</p>
        </section>
      ))}
      <Separator />
      <YuvaPageFeedback />
      <YuvaDiscussions />
    </article>
  )
}

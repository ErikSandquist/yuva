import Link from "next/link"
import { docs } from "@/lib/docs"

export default function Home() {
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-3xl font-semibold tracking-tight">Acme docs</h1>
      <ul className="flex flex-col gap-3">
        {docs.map((doc) => (
          <li key={doc.slug}>
            <Link href={`/docs/${doc.slug}`} className="font-medium underline-offset-4 hover:underline">
              {doc.title}
            </Link>
            <p className="text-sm text-muted-foreground">{doc.description}</p>
          </li>
        ))}
      </ul>
    </div>
  )
}

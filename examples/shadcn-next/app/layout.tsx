import type { Metadata } from "next"
import Link from "next/link"
import { Geist, Geist_Mono } from "next/font/google"
import { YuvaDiscussionsProvider } from "@/hooks/use-yuva-discussions"
import { docs } from "@/lib/docs"
import "./globals.css"

const geistSans = Geist({ variable: "--font-geist-sans", subsets: ["latin"] })
const geistMono = Geist_Mono({ variable: "--font-geist-mono", subsets: ["latin"] })

export const metadata: Metadata = {
  title: { default: "Acme docs", template: "%s · Acme docs" },
  description: "A documentation site with Yuva discussions built from the shadcn registry.",
}

const theme = `document.documentElement.classList.toggle("dark", matchMedia("(prefers-color-scheme: dark)").matches)`

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
      suppressHydrationWarning
    >
      <head>
        <script dangerouslySetInnerHTML={{ __html: theme }} />
      </head>
      <body className="flex min-h-full flex-col bg-background text-foreground">
        <YuvaDiscussionsProvider
          server={process.env.NEXT_PUBLIC_YUVA_SERVER}
          channel={process.env.NEXT_PUBLIC_YUVA_CHANNEL}
        >
          <div className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-8 px-4 py-8 md:flex-row md:px-6">
            <nav aria-label="Documentation" className="flex shrink-0 flex-col gap-1 md:w-48">
              <Link href="/" className="mb-2 font-semibold">
                Acme docs
              </Link>
              {docs.map((doc) => (
                <Link
                  key={doc.slug}
                  href={`/docs/${doc.slug}`}
                  className="rounded-md px-2 py-1 text-sm text-muted-foreground hover:bg-muted hover:text-foreground"
                >
                  {doc.title}
                </Link>
              ))}
              <Link
                href="/help"
                className="rounded-md px-2 py-1 text-sm text-muted-foreground hover:bg-muted hover:text-foreground"
              >
                Help center
              </Link>
              <Link
                href="/chat"
                className="rounded-md px-2 py-1 text-sm text-muted-foreground hover:bg-muted hover:text-foreground"
              >
                Chat
              </Link>
            </nav>
            <main className="min-w-0 flex-1">{children}</main>
          </div>
        </YuvaDiscussionsProvider>
      </body>
    </html>
  )
}

import { StrictMode, useRef, useState, type ReactNode } from "react"
import { createRoot } from "react-dom/client"
import { YuvaChat } from "@/components/yuva-chat"
import { YuvaDiscussions } from "@/components/yuva-discussions"
import { YuvaHelp } from "@/components/yuva-help"
import { YuvaRoadmap } from "@/components/yuva-roadmap"
import "./index.css"

const server = import.meta.env.VITE_YUVA_SERVER
const channel = import.meta.env.VITE_YUVA_CHANNEL

function Cross({ side }: { side: "left" | "right" }) {
  return (
    <svg aria-hidden="true" viewBox="0 0 13 13" className={`frame-cross frame-cross-${side}`}>
      <path d="M6.5 0v13M0 6.5h13" stroke="currentColor" />
    </svg>
  )
}

function Section({ index, title, children }: { index: string; title: string; children: ReactNode }) {
  return (
    <section className="frame-section">
      <Cross side="left" />
      <Cross side="right" />
      <div className="mx-auto flex w-full max-w-2xl flex-col gap-6 px-10 py-12">
        <p className="font-mono text-[11px] tracking-[0.08em] text-muted-foreground uppercase">
          {index} — {title}
        </p>
        {children}
      </div>
    </section>
  )
}

function App() {
  const [asked, setAsked] = useState<string | undefined>()
  const chat = useRef<HTMLDivElement>(null)
  return (
    <main className="frame">
      <div ref={chat}>
        <Section index="01" title="Chat">
          <YuvaChat server={server} channel={channel} variant="lined" helpArticleId={asked} />
        </Section>
      </div>
      <Section index="02" title="Help">
        <YuvaHelp
          server={server}
          channel={channel}
          onAsk={(articleId) => {
            setAsked(articleId)
            chat.current?.scrollIntoView({ behavior: "smooth" })
          }}
        />
      </Section>
      <Section index="03" title="Discussions">
        <YuvaDiscussions server={server} channel={channel} />
      </Section>
      <Section index="04" title="Roadmap">
        <YuvaRoadmap server={server} channel={channel} />
      </Section>
    </main>
  )
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>
)

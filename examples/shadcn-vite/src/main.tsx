import { StrictMode, type ReactNode } from "react"
import { createRoot } from "react-dom/client"
import { YuvaChat } from "@/components/yuva-chat"
import { YuvaDiscussions } from "@/components/yuva-discussions"
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

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <main className="frame">
      <Section index="01" title="Chat">
        <YuvaChat server={server} channel={channel} />
      </Section>
      <Section index="02" title="Discussions">
        <YuvaDiscussions server={server} channel={channel} />
      </Section>
    </main>
  </StrictMode>
)

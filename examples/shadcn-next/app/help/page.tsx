import type { Metadata } from "next"
import { HelpCenter } from "./help-center"

export const metadata: Metadata = { title: "Help center" }

export default function HelpPage() {
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-3xl font-semibold tracking-tight">Help center</h1>
      <HelpCenter server={process.env.NEXT_PUBLIC_YUVA_SERVER} channel={process.env.NEXT_PUBLIC_YUVA_CHANNEL} />
    </div>
  )
}

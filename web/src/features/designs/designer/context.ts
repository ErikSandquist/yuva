import { createContext, useContext } from "react"

import type { Block, Doc } from "@/features/designs/doc"
import type { EmailDesignIssue, EmailImage } from "@/lib/api"

export type DropTarget = { container: string; index: number }

export type Designer = {
  doc: Doc
  selected: string | null
  select: (id: string | null) => void
  setDoc: (fn: (d: Doc) => Doc, key?: string) => void
  update: (id: string, patch: Partial<Block> | ((b: Block) => Block), key?: string) => void
  remove: (id: string) => void
  duplicate: (id: string) => void
  nudge: (id: string, by: -1 | 1) => void
  images: Map<string, EmailImage>
  issues: Map<string, EmailDesignIssue[]>
  readOnly: boolean
  openLibrary: (blockId: string) => void
  drop: DropTarget | null
  dragging: string | null
  toolSlot: HTMLElement | null
}

export const DesignerContext = createContext<Designer | null>(null)

export function useDesigner() {
  const d = useContext(DesignerContext)
  if (!d) throw new Error("useDesigner outside the designer")
  return d
}

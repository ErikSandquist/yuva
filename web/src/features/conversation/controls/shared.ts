import { AlarmClockIcon, CheckCircle2Icon, CircleDashedIcon, CircleIcon } from "lucide-react"

import type { Conversation, ConversationStatus, ConversationUpdate, Priority } from "@/lib/api"

export type MenuName = "assign" | "status" | "priority" | "labels" | "move"

export type ControlProps = {
  conversation: Conversation
  update: (body: ConversationUpdate) => void
  openMenu: MenuName | null
  setOpenMenu: (m: MenuName | null) => void
}

export function menuProps(name: MenuName, p: Pick<ControlProps, "openMenu" | "setOpenMenu">) {
  return { open: p.openMenu === name, onOpenChange: (open: boolean) => p.setOpenMenu(open ? name : null) }
}

export const statusIcons: Record<ConversationStatus, React.ComponentType<{ className?: string }>> = {
  open: CircleIcon,
  pending: CircleDashedIcon,
  snoozed: AlarmClockIcon,
  closed: CheckCircle2Icon,
}

export const priorityClass: Record<Priority, string> = {
  urgent: "text-destructive",
  high: "text-brand",
  normal: "text-faint",
  low: "text-faint",
}

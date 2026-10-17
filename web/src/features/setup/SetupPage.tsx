import { Navigate, useParams } from "react-router"

import { AddChannelStep } from "@/features/setup/AddChannelStep"
import { ChannelStep } from "@/features/setup/ChannelStep"
import { InstallStep } from "@/features/setup/InstallStep"
import { NameStep } from "@/features/setup/NameStep"
import { NewsletterStep } from "@/features/setup/NewsletterStep"
import { SETUP_KINDS, type SetupKind } from "@/features/setup/setup"
import { SetupHub } from "@/features/setup/SetupHub"
import { StartStep } from "@/features/setup/StartStep"
import { WaitStep } from "@/features/setup/WaitStep"
import { useSession } from "@/lib/session"

type Flow = "hub" | "new" | "pick" | "add" | "start" | "newsletter" | "wait"

function isKind(kind: string | undefined): kind is SetupKind {
  return SETUP_KINDS.includes(kind as SetupKind)
}

export function SetupPage({ flow }: { flow?: Flow }) {
  const { canManage } = useSession()
  const { inboxId: param, channelId, kind, listId } = useParams()
  const inboxId = flow === "pick" || flow === "add" ? "new" : param
  if (!canManage) return <Navigate to="/" replace />
  if (flow === "hub") return <SetupHub />
  if (flow === "newsletter") return <NewsletterStep key={listId ?? ""} listId={listId} />
  if (flow === "start") return isKind(kind) ? <StartStep key={kind} kind={kind} /> : <Navigate to="/setup" replace />
  if (flow === "new") {
    if (kind !== undefined && !isKind(kind)) return <Navigate to="/setup/new" replace />
    return <NameStep key={kind ?? ""} kind={kind} />
  }
  if (inboxId && kind) {
    if (!isKind(kind)) return <Navigate to={`/setup/${inboxId}`} replace />
    return <AddChannelStep key={kind} inboxId={inboxId} kind={kind} />
  }
  if (inboxId && channelId)
    return flow === "wait" ? (
      <WaitStep inboxId={inboxId} channelId={channelId} />
    ) : (
      <InstallStep inboxId={inboxId} channelId={channelId} />
    )
  if (inboxId) return <ChannelStep inboxId={inboxId} />
  return <Navigate to="/setup" replace />
}

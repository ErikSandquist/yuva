"use client"

import * as React from "react"
import { ApiError, type IdentityTokenSource, type YuvaClient } from "useyuva"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { useYuvaSubscribe } from "@/hooks/use-yuva-subscribe"
import { cn } from "@/lib/utils"

export interface YuvaSubscribeLabels {
  lists: string
  email: string
  name: string
  nameOptional: string
  button: string
  sending: string
  consent: (lists: string, inbox: string) => string
  privacy: string
  checkInbox: string
  subscribed: string
  pending: string
  chooseList: string
  invalidEmail: string
  nameRequired: string
  consentRequired: string
  noEmail: string
  rateLimited: string
  offline: string
  generic: string
}

export const defaultYuvaSubscribeLabels: YuvaSubscribeLabels = {
  lists: "Lists",
  email: "E-mail address",
  name: "Your name",
  nameOptional: "Your name (optional)",
  button: "Subscribe",
  sending: "Subscribing…",
  consent: (lists, inbox) => `I want to receive e-mails about ${lists} from ${inbox}. I can unsubscribe at any time.`,
  privacy: "Privacy policy",
  checkInbox: "Almost done. Check your inbox and confirm your subscription.",
  subscribed: "Thanks! You are subscribed.",
  pending: "Check your inbox to confirm.",
  chooseList: "Choose at least one list.",
  invalidEmail: "Enter a valid e-mail address.",
  nameRequired: "Enter your name.",
  consentRequired: "Tick the box to agree.",
  noEmail: "Your account has no e-mail address yet.",
  rateLimited: "Too many attempts. Wait a moment and try again.",
  offline: "You seem to be offline.",
  generic: "Could not subscribe. Try again.",
}

export interface YuvaSubscribeProps {
  server?: string
  channel?: string
  client?: YuvaClient
  identityToken?: IdentityTokenSource | null
  lists?: string[]
  layout?: "stacked" | "inline"
  name?: "off" | "optional" | "required"
  consent?: string
  privacyUrl?: string
  locale?: string
  labels?: Partial<YuvaSubscribeLabels>
  onSubscribe?: (detail: { lists: string[]; confirmation: boolean }) => void
  className?: string
}

function errorText(error: unknown, labels: YuvaSubscribeLabels): string {
  const code = error instanceof ApiError ? error.code : ""
  if (code === "invalid_email") return labels.invalidEmail
  if (code === "no_email") return labels.noEmail
  if (code === "rate_limited") return labels.rateLimited
  if (code === "network") return labels.offline
  return labels.generic
}

export function YuvaSubscribe({
  server,
  channel,
  client,
  identityToken,
  lists: only,
  layout = "stacked",
  name: askName = "off",
  consent: customConsent,
  privacyUrl,
  locale,
  labels: overrides,
  onSubscribe,
  className,
}: YuvaSubscribeProps) {
  const labels = React.useMemo(() => ({ ...defaultYuvaSubscribeLabels, ...overrides }), [overrides])
  const id = React.useId()
  const { inboxName, lists, states, identified, loading, error, subscribe, setSubscribed } = useYuvaSubscribe({
    server,
    channel,
    client,
    identityToken,
    lists: only,
  })
  const [chosen, setChosen] = React.useState<string[]>([])
  const [email, setEmail] = React.useState("")
  const [name, setName] = React.useState("")
  const [website, setWebsite] = React.useState("")
  const [agreed, setAgreed] = React.useState(false)
  const [busy, setBusy] = React.useState<string | null>(null)
  const [done, setDone] = React.useState<boolean | null>(null)
  const [problem, setProblem] = React.useState<string | null>(null)
  const [invalid, setInvalid] = React.useState<{ email?: string; name?: string; consent?: string }>({})

  const selected = lists.length === 1 ? lists : lists.filter((list) => chosen.includes(list.id))
  const consentFor = (names: string[]) =>
    customConsent?.trim() ||
    labels.consent(new Intl.ListFormat(locale, { type: "conjunction" }).format(names), inboxName)

  if (loading || (!identified && lists.length === 0 && !error)) return null
  if (error) {
    return (
      <p role="alert" className={cn("text-sm text-destructive", className)}>
        {errorText(error, labels)}
      </p>
    )
  }

  if (identified) {
    return (
      <fieldset className={cn("grid gap-3", className)}>
        <legend className="mb-1 text-sm font-medium">{labels.lists}</legend>
        {lists.map((list) => {
          const state = states[list.id]
          const text = consentFor([list.name])
          return (
            <div key={list.id} className="flex items-start gap-3">
              <Switch
                id={`${id}-${list.id}`}
                checked={state === "subscribed" || state === "pending"}
                disabled={busy === list.id}
                onCheckedChange={(on: boolean) => {
                  setBusy(list.id)
                  setProblem(null)
                  setSubscribed(list.id, on, text)
                    .then((next) => {
                      if (on) onSubscribe?.({ lists: [list.id], confirmation: next === "pending" })
                    })
                    .catch((failure: unknown) => setProblem(errorText(failure, labels)))
                    .finally(() => setBusy(null))
                }}
              />
              <Label htmlFor={`${id}-${list.id}`} className="grid gap-0.5 font-normal">
                <span className="font-medium">{list.name}</span>
                {list.description ? <span className="text-muted-foreground">{list.description}</span> : null}
                <span className="text-xs text-muted-foreground">{text}</span>
                {state === "pending" ? <span className="text-xs text-muted-foreground">{labels.pending}</span> : null}
              </Label>
            </div>
          )
        })}
        {problem ? (
          <p role="alert" className="text-sm text-destructive">
            {problem}
          </p>
        ) : null}
      </fieldset>
    )
  }

  const consentText = consentFor((selected.length ? selected : lists).map((list) => list.name))

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    const next = {
      email: /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim()) ? undefined : labels.invalidEmail,
      name: askName === "required" && !name.trim() ? labels.nameRequired : undefined,
      consent: agreed ? undefined : labels.consentRequired,
    }
    setInvalid(next)
    setProblem(selected.length ? null : labels.chooseList)
    if (next.email || next.name || next.consent || !selected.length) return
    setBusy("form")
    try {
      const { confirmation } = await subscribe({
        email: email.trim(),
        name: askName === "off" ? undefined : name.trim(),
        listIds: selected.map((list) => list.id),
        consentText,
        locale,
        website,
      })
      setDone(confirmation)
      onSubscribe?.({ lists: selected.map((list) => list.id), confirmation })
    } catch (failure) {
      if (failure instanceof ApiError && failure.code === "invalid_email") setInvalid({ email: labels.invalidEmail })
      else setProblem(errorText(failure, labels))
    } finally {
      setBusy(null)
    }
  }

  if (done !== null) {
    return (
      <p role="status" className={cn("text-sm", className)}>
        {done ? labels.checkInbox : labels.subscribed}
      </p>
    )
  }

  const sending = busy === "form"
  return (
    <form noValidate onSubmit={submit} className={cn("grid gap-3", className)}>
      {lists.length > 1 ? (
        <fieldset className="grid gap-2">
          <legend className="mb-1 text-sm font-medium">{labels.lists}</legend>
          {lists.map((list) => (
            <div key={list.id} className="flex items-start gap-2">
              <Checkbox
                id={`${id}-${list.id}`}
                checked={chosen.includes(list.id)}
                disabled={sending}
                onCheckedChange={(on) =>
                  setChosen((current) =>
                    on === true ? [...current, list.id] : current.filter((value) => value !== list.id)
                  )
                }
              />
              <Label htmlFor={`${id}-${list.id}`} className="grid gap-0.5 font-normal">
                <span>{list.name}</span>
                {list.description ? <span className="text-muted-foreground">{list.description}</span> : null}
              </Label>
            </div>
          ))}
        </fieldset>
      ) : null}
      <div className={cn("flex gap-2", layout === "inline" ? "flex-wrap" : "flex-col")}>
        {askName !== "off" ? (
          <Input
            aria-label={askName === "required" ? labels.name : labels.nameOptional}
            placeholder={askName === "required" ? labels.name : labels.nameOptional}
            autoComplete="name"
            maxLength={200}
            value={name}
            disabled={sending}
            aria-invalid={!!invalid.name}
            aria-describedby={invalid.name ? `${id}-name-error` : undefined}
            onChange={(event) => setName(event.target.value)}
            className={layout === "inline" ? "min-w-44 flex-1" : undefined}
          />
        ) : null}
        <Input
          type="email"
          aria-label={labels.email}
          placeholder={labels.email}
          autoComplete="email"
          maxLength={320}
          value={email}
          disabled={sending}
          aria-invalid={!!invalid.email}
          aria-describedby={invalid.email ? `${id}-email-error` : undefined}
          onChange={(event) => setEmail(event.target.value)}
          className={layout === "inline" ? "min-w-44 flex-1" : undefined}
        />
        <Button type="submit" disabled={sending}>
          {sending ? labels.sending : labels.button}
        </Button>
      </div>
      {invalid.name ? (
        <p id={`${id}-name-error`} className="text-sm text-destructive">
          {invalid.name}
        </p>
      ) : null}
      {invalid.email ? (
        <p id={`${id}-email-error`} className="text-sm text-destructive">
          {invalid.email}
        </p>
      ) : null}
      <div className="flex items-start gap-2">
        <Checkbox
          id={`${id}-consent`}
          checked={agreed}
          disabled={sending}
          aria-invalid={!!invalid.consent}
          aria-describedby={invalid.consent ? `${id}-consent-error` : undefined}
          onCheckedChange={(on) => setAgreed(on === true)}
        />
        <Label htmlFor={`${id}-consent`} className="block text-sm leading-snug font-normal">
          {consentText}
          {privacyUrl ? (
            <>
              {" "}
              <a href={privacyUrl} target="_blank" rel="noopener noreferrer" className="underline underline-offset-4">
                {labels.privacy}
              </a>
            </>
          ) : null}
        </Label>
      </div>
      {invalid.consent ? (
        <p id={`${id}-consent-error`} className="text-sm text-destructive">
          {invalid.consent}
        </p>
      ) : null}
      <div aria-hidden="true" className="absolute -start-[10000px] size-px overflow-hidden">
        <input
          tabIndex={-1}
          autoComplete="off"
          name="website"
          value={website}
          onChange={(event) => setWebsite(event.target.value)}
        />
      </div>
      <p aria-live="polite" role="status" className="text-sm text-destructive">
        {problem}
      </p>
    </form>
  )
}

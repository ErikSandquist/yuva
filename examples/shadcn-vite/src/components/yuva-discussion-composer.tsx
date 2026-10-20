"use client"

import * as React from "react"
import { LogIn } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { markdownPreview } from "@/components/yuva-markdown-preview"
import { useYuvaLabels, useYuvaReader } from "@/hooks/use-yuva-discussions"
import { cn } from "@/lib/utils"
import { yuvaErrorMessage, yuvaProseClassName, type YuvaDiscussionsLabels } from "@/lib/yuva-utils"

export interface YuvaDiscussionComposerProps {
  withTitle?: boolean
  initialTitle?: string
  initialBody?: string
  submitLabel?: string
  placeholder?: string
  autoFocus?: boolean
  labels?: Partial<YuvaDiscussionsLabels>
  className?: string
  onTitleChange?(title: string): void
  onSubmit(input: { title: string; body: string }): Promise<unknown>
  onCancel?(): void
}

export function YuvaDiscussionComposer({
  withTitle = false,
  initialTitle = "",
  initialBody = "",
  submitLabel,
  placeholder,
  autoFocus,
  labels: overrides,
  className,
  onTitleChange,
  onSubmit,
  onCancel,
}: YuvaDiscussionComposerProps) {
  const labels = useYuvaLabels(overrides)
  const id = React.useId()
  const [title, setTitle] = React.useState(initialTitle)
  const [body, setBody] = React.useState(initialBody)
  const [tab, setTab] = React.useState("write")
  const [busy, setBusy] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)

  const valid = body.trim().length > 0 && (!withTitle || title.trim().length >= 3)

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!valid || busy) return
    setBusy(true)
    setError(null)
    try {
      await onSubmit({ title: title.trim(), body: body.trim() })
      setTitle("")
      onTitleChange?.("")
      setBody("")
      setTab("write")
    } catch (failure) {
      setError(yuvaErrorMessage(failure, labels))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} className={cn("flex flex-col gap-3", className)} aria-busy={busy}>
      {withTitle && (
        <div className="flex flex-col gap-1.5">
          <label htmlFor={`${id}-title`} className="text-sm font-medium">
            {labels.titleLabel}
          </label>
          <Input
            id={`${id}-title`}
            value={title}
            onChange={(event) => {
              setTitle(event.target.value)
              onTitleChange?.(event.target.value)
            }}
            placeholder={labels.titlePlaceholder}
            maxLength={200}
            minLength={3}
            required
            autoFocus={autoFocus}
          />
        </div>
      )}
      <Tabs value={tab} onValueChange={(value) => setTab(String(value))}>
        <TabsList aria-label={labels.bodyLabel}>
          <TabsTrigger value="write">{labels.write}</TabsTrigger>
          <TabsTrigger value="preview">{labels.preview}</TabsTrigger>
        </TabsList>
        <TabsContent value="write">
          <Textarea
            aria-label={labels.bodyLabel}
            value={body}
            onChange={(event) => setBody(event.target.value)}
            placeholder={placeholder ?? (withTitle ? labels.bodyPlaceholder : labels.replyPlaceholder)}
            maxLength={20000}
            rows={5}
            autoFocus={autoFocus && !withTitle}
            onKeyDown={(event) => {
              if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) void submit(event)
            }}
            className="min-h-28"
          />
        </TabsContent>
        <TabsContent value="preview">
          <div className={cn("min-h-28 rounded-md border px-3 py-2", yuvaProseClassName)}>
            {body.trim() ? markdownPreview(body) : <p className="text-muted-foreground">{labels.previewEmpty}</p>}
          </div>
        </TabsContent>
      </Tabs>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-xs text-muted-foreground">{labels.markdownHint}</span>
        <div className="flex gap-2">
          {onCancel && (
            <Button type="button" variant="ghost" onClick={onCancel} disabled={busy}>
              {labels.cancel}
            </Button>
          )}
          <Button type="submit" disabled={!valid || busy}>
            {submitLabel ?? (withTitle ? labels.startDiscussion : labels.postReply)}
          </Button>
        </div>
      </div>
    </form>
  )
}

export interface YuvaSignInProps {
  labels?: Partial<YuvaDiscussionsLabels>
  className?: string
}

export function YuvaSignIn({ labels: overrides, className }: YuvaSignInProps) {
  const labels = useYuvaLabels(overrides)
  const { signIn, ready } = useYuvaReader()
  const [error, setError] = React.useState<string | null>(null)
  return (
    <div
      className={cn(
        "flex flex-col items-start gap-3 rounded-lg border border-dashed p-4 sm:flex-row sm:items-center sm:justify-between",
        className
      )}
    >
      <p className="text-sm text-muted-foreground">{error ?? labels.signInHint}</p>
      <Button
        variant="outline"
        disabled={!ready}
        onClick={() => {
          setError(null)
          signIn().catch((failure) => setError(yuvaErrorMessage(failure, labels)))
        }}
      >
        <LogIn data-icon="inline-start" className="rtl:rotate-180" />
        {labels.signIn}
      </Button>
    </div>
  )
}

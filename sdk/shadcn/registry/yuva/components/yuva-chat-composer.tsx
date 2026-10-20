"use client"

import * as React from "react"
import { ArrowUpIcon, FileIcon, ImageIcon, PaperclipIcon, ScanIcon, XIcon } from "lucide-react"
import {
  Attachment,
  AttachmentAction,
  AttachmentActions,
  AttachmentContent,
  AttachmentDescription,
  AttachmentGroup,
  AttachmentMedia,
  AttachmentTitle,
} from "@/components/ui/attachment"
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupText,
  InputGroupTextarea,
} from "@/components/ui/input-group"
import { Kbd } from "@/components/ui/kbd"
import type { YuvaChatState } from "@/hooks/use-yuva-chat"
import { chatErrorText, defaultYuvaChatLabels, sizeLabel, type YuvaChatLabels } from "@/lib/yuva-chat-utils"
import { cn } from "@/lib/utils"

export interface YuvaChatComposerProps {
  chat: YuvaChatState
  labels?: Partial<YuvaChatLabels>
  locale?: string
  disabled?: boolean
  onCapture?: () => File | null | undefined | Promise<File | null | undefined>
  className?: string
}

export function YuvaChatComposer({
  chat,
  labels: custom,
  locale,
  disabled,
  onCapture,
  className,
}: YuvaChatComposerProps) {
  const labels = React.useMemo(() => ({ ...defaultYuvaChatLabels, ...custom }), [custom])
  const [body, setBody] = React.useState("")
  const [files, setFiles] = React.useState<File[]>([])
  const [error, setError] = React.useState<unknown>(null)
  const picker = React.useRef<HTMLInputElement>(null)
  const field = React.useRef<HTMLTextAreaElement>(null)
  const [capturing, setCapturing] = React.useState(false)
  const empty = body.trim() === "" && files.length === 0

  const submit = async () => {
    if (empty || disabled) return
    const input = { body, files }
    setBody("")
    setFiles([])
    setError(null)
    chat.setTyping(false)
    field.current?.focus()
    try {
      await chat.send(input)
    } catch (failure) {
      setBody(input.body)
      setFiles(input.files)
      setError(failure)
    }
  }

  const capture = async () => {
    if (!onCapture) return
    setCapturing(true)
    try {
      const shot = await onCapture()
      if (shot) setFiles((list) => [...list, shot])
    } catch (failure) {
      setError(failure)
    } finally {
      setCapturing(false)
    }
  }

  return (
    <form
      className={cn("flex flex-col gap-2", className)}
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      <InputGroup>
        <InputGroupTextarea
          ref={field}
          rows={2}
          value={body}
          disabled={disabled}
          placeholder={labels.placeholder}
          aria-label={labels.placeholder}
          className="max-h-40 min-h-14 [field-sizing:content]"
          onChange={(event) => {
            setBody(event.target.value)
            chat.setTyping(event.target.value.trim() !== "")
          }}
          onBlur={() => chat.setTyping(false)}
          onKeyDown={(event) => {
            if (event.key !== "Enter" || event.shiftKey || event.nativeEvent.isComposing) return
            event.preventDefault()
            void submit()
          }}
        />
        {files.length > 0 && (
          <InputGroupAddon align="block-start">
            <AttachmentGroup className="w-full">
              {files.map((file, index) => (
                <Attachment key={`${file.name}:${index}`} state="idle" size="xs">
                  <AttachmentMedia>{file.type.startsWith("image/") ? <ImageIcon /> : <FileIcon />}</AttachmentMedia>
                  <AttachmentContent>
                    <AttachmentTitle>{file.name}</AttachmentTitle>
                    <AttachmentDescription>{sizeLabel(file.size, locale)}</AttachmentDescription>
                  </AttachmentContent>
                  <AttachmentActions>
                    <AttachmentAction
                      type="button"
                      aria-label={`${labels.remove}: ${file.name}`}
                      onClick={() => setFiles((list) => list.filter((_, at) => at !== index))}
                    >
                      <XIcon />
                    </AttachmentAction>
                  </AttachmentActions>
                </Attachment>
              ))}
            </AttachmentGroup>
          </InputGroupAddon>
        )}
        <InputGroupAddon align="block-end">
          <input
            ref={picker}
            type="file"
            multiple
            hidden
            onChange={(event) => {
              const picked = Array.from(event.target.files ?? [])
              event.target.value = ""
              setFiles((list) => [...list, ...picked])
            }}
          />
          <InputGroupButton
            size="icon-sm"
            aria-label={labels.attach}
            disabled={disabled}
            onClick={() => picker.current?.click()}
          >
            <PaperclipIcon />
          </InputGroupButton>
          {onCapture && (
            <InputGroupButton
              size="icon-sm"
              aria-label={labels.screenshot}
              disabled={disabled || capturing}
              onClick={() => void capture()}
            >
              <ScanIcon />
            </InputGroupButton>
          )}
          <InputGroupText className="ms-auto hidden text-xs sm:flex">
            <Kbd>{labels.enterKey}</Kbd>
            {labels.toSend}
          </InputGroupText>
          <InputGroupButton
            type="submit"
            variant="default"
            size="icon-sm"
            className="max-sm:ms-auto"
            aria-label={labels.send}
            disabled={disabled || empty}
          >
            <ArrowUpIcon />
          </InputGroupButton>
        </InputGroupAddon>
      </InputGroup>
      {error !== null && (
        <p role="alert" className="text-xs text-destructive">
          {chatErrorText(error, labels)}
        </p>
      )}
    </form>
  )
}

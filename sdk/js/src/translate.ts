import type { MessageDescriptor, Messages } from "@lingui/core"

// Keeps @lingui/core out of the loader; only plain messages and simple placeholders (no plurals) belong here.
export function translate(messages: Messages, descriptor: MessageDescriptor, fallback: Messages): string {
  const entry = messages[descriptor.id] ?? fallback[descriptor.id]
  const values = (descriptor.values ?? {}) as Record<string, unknown>
  if (typeof entry === "string") return entry
  if (Array.isArray(entry)) {
    return entry
      .map((part) =>
        typeof part === "string" ? part : Array.isArray(part) ? String(values[part[0] as string] ?? "") : ""
      )
      .join("")
  }
  return descriptor.message ?? descriptor.id
}

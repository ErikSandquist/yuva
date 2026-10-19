export type BarTone = "ok" | "warn" | "partial" | "maint" | "bad"

export function bars(marks: Record<number, BarTone>, count = 90) {
  const paths: Record<BarTone, string> = { ok: "", warn: "", partial: "", maint: "", bad: "" }
  for (let i = 0; i < count; i++) paths[marks[i] ?? "ok"] += `M${i * 2} 0h1.4v8h-1.4Z`
  return paths
}
